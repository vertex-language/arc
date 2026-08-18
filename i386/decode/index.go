package decode

import (
	"fmt"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
)

// The opcode maps of Intel SDM Vol. 2, Appendix A: the one-byte map, the
// two-byte map reached through 0F, and the two three-byte maps reached
// through 0F 38 and 0F 3A.
//
// None of them is written out here. They are built at init from isa.All(),
// which is the same table Emit resolves against and arc isa prints — so a
// byte decodes to a form arc build could have emitted, and adding
// table_sse.go populates the 0F 38 map without a line changing in this file.
// A hand-written second opcode map would be the one place in the tree where
// the decoder could quietly disagree with the encoder about what a byte means.
var (
	map1    [256][]*entry
	map0F   [256][]*entry
	map0F38 [256][]*entry
	map0F3A [256][]*entry
)

// entry is a form with the questions selection asks answered in advance.
type entry struct {
	form *isa.Form

	// base is the opcode byte the form is declared at. For a +r form the
	// entry is registered at base through base+7 and the register number is
	// the difference.
	base  byte
	plusR bool

	// modrm reports whether the form has a ModRM byte at all.
	modrm bool

	// ext is the /digit, or isa.NoExt.
	ext int8

	regSlot int
	rmSlot  int
	opSlot  int

	// sysMove marks the control- and debug-register moves. The SDM states
	// that their mod field is ignored and treated as 11b, so mod=00 is not a
	// memory operand there and not an error either — it is bytes real silicon
	// executes, and refusing them would be a decoder disagreeing with the
	// processor about what it just ran.
	sysMove bool
}

var entries []*entry

func init() {
	for _, f := range isa.All() {
		// An alias is never decoded to. jz and je are one encoding and the
		// listing should say what the silicon does; the same rule that makes
		// arc explain name MOVZ rather than MOV.
		if f.AliasOf != "" {
			continue
		}
		entries = append(entries, newEntry(f))
	}

	// Two passes, and the order is the selection rule: an exact opcode match
	// outranks an opcode-plus-register match. That is why 0x90 is NOP and
	// 0x91 is XCHG EAX, ECX, even though XCHG's +r range covers both.
	for _, e := range entries {
		if !e.plusR {
			register(e)
		}
	}
	for _, e := range entries {
		if e.plusR {
			register(e)
		}
	}
}

func newEntry(f *isa.Form) *entry {
	e := &entry{form: f, ext: f.Ext, regSlot: -1, rmSlot: -1, opSlot: -1}
	for i, o := range f.Ops {
		switch o.Slot {
		case isa.SlotReg:
			e.regSlot = i
		case isa.SlotRM:
			e.rmSlot = i
		case isa.SlotOpcode:
			e.opSlot, e.plusR = i, true
		}
		if o.Class == isa.Cr || o.Class == isa.Dr {
			e.sysMove = true
		}
	}
	e.modrm = e.regSlot >= 0 || e.rmSlot >= 0 || e.ext >= 0
	return e
}

func register(e *entry) {
	m, base := mapFor(e.form)
	e.base = base

	if !e.plusR {
		m[base] = append(m[base], e)
		return
	}
	if int(base)+7 > 0xff {
		panic("i386/decode: +r form at " + e.form.Signature() + " runs off the end of its map")
	}
	for n := 0; n < 8; n++ {
		m[base+byte(n)] = append(m[base+byte(n)], e)
	}
}

func mapFor(f *isa.Form) (*[256][]*entry, byte) {
	switch {
	case len(f.Opcode) == 1:
		return &map1, f.Opcode[0]
	case len(f.Opcode) == 2 && f.Opcode[0] == 0x0f:
		return &map0F, f.Opcode[1]
	case len(f.Opcode) == 3 && f.Opcode[0] == 0x0f && f.Opcode[1] == 0x38:
		return &map0F38, f.Opcode[2]
	case len(f.Opcode) == 3 && f.Opcode[0] == 0x0f && f.Opcode[1] == 0x3a:
		return &map0F3A, f.Opcode[2]
	}
	panic("i386/decode: " + f.Signature() + " has an opcode in no SDM map")
}

// lookup picks the form for an opcode byte.
//
// Candidates come from the bucket in rank order — exact before +r, table order
// within each — and the first one every test admits wins. The tests are the
// encoding's, not preferences: a /digit must match ModRM.reg, a form declared
// with the operand-size override must have seen 0x66, a class that is
// register-only requires mod=11, and a class that is memory-only forbids it.
//
// Selection is the only place gating happens. A bucket whose surviving
// candidates are all gated is an error naming the flag, not an unknown
// opcode: the byte means something, it just does not mean it on this target.
func lookup(m *[256][]*entry, op byte, s feature.Set, opSize16 bool, modrm byte, haveModRM bool) (*entry, uint8, error) {
	bucket := m[op]
	if len(bucket) == 0 {
		return nil, 0, fmt.Errorf("%w %#02x", ErrUnknown, op)
	}

	var (
		gated     *isa.Form
		truncated bool
		why       string
	)

	for _, e := range bucket {
		f := e.form

		if f.OpSize16 != opSize16 {
			if opSize16 {
				why = fmt.Sprintf("%s has no 16-bit form declared", f.Signature())
			} else {
				why = fmt.Sprintf("%s needs the 0x66 operand-size override", f.Signature())
			}
			continue
		}

		if e.modrm && !haveModRM {
			truncated = true
			continue
		}

		if e.ext >= 0 && int8((modrm>>3)&7) != e.ext {
			why = fmt.Sprintf("no /%d in the group at %#02x", (modrm>>3)&7, op)
			continue
		}

		if e.rmSlot >= 0 {
			c := f.Ops[e.rmSlot].Class
			mod := modrm >> 6
			switch {
			case c == isa.M && mod == 3:
				why = fmt.Sprintf("%s takes a memory operand; mod=11", f.Signature())
				continue
			case registerOnly(c) && mod != 3 && !e.sysMove:
				why = fmt.Sprintf("%s takes a register operand; mod=%02b", f.Signature(), mod)
				continue
			}
		}

		if !f.Enabled(s) {
			gated = f
			continue
		}

		return e, op - e.base, nil
	}

	switch {
	case truncated:
		return nil, 0, ErrTruncated
	case gated != nil:
		return nil, 0, fmt.Errorf("%w: %s requires %s, not in the active feature set\n"+
			"  active: %s\n  note: --features %s",
			ErrDecode, gated.Mnemonic, gated.Gate(), s, gated.Gate())
	case why != "":
		return nil, 0, fmt.Errorf("%w %#02x: %s", ErrUnknown, op, why)
	}
	return nil, 0, fmt.Errorf("%w %#02x", ErrUnknown, op)
}

// registerOnly reports whether a class in a ModRM.rm slot can only be a
// register. The r/m classes are not among them: that is what r/m means.
func registerOnly(c isa.Class) bool {
	switch c {
	case isa.R8, isa.R16, isa.R32, isa.Sreg, isa.St, isa.Mm,
		isa.Xmm, isa.Ymm, isa.Zmm, isa.Cr, isa.Dr:
		return true
	}
	return false
}

// opcodeNote is the encoding suffix arc explain prints beside the opcode, the
// same one arc isa prints in the Bytes column.
func (e *entry) opcodeNote(plusReg uint8) string {
	switch {
	case e.ext >= 0:
		return fmt.Sprintf("/%d", e.ext)
	case e.plusR:
		if r, err := regOperand(e.form.Ops[e.opSlot].Class, plusReg); err == nil {
			if n, ok := r.(interface{ Name() string }); ok {
				return fmt.Sprintf("+r → %s", n.Name())
			}
		}
		return "+r"
	case e.regSlot >= 0:
		return "/r"
	}
	return ""
}