// x86_64/isa/form.go
package isa

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/feature"
)

// NoExt is Form.Ext for a form whose ModRM.reg field holds a register rather
// than an opcode extension — the SDM's /r.
const NoExt int8 = -1

// Form is one declared encoding of one mnemonic.
//
// A Form is data. It never decides between itself and another form; Resolve
// does that, and encode/ turns the winner into bytes. Two forms of the same
// mnemonic that differ in nothing but opcode are two rows here, not one row
// with a condition in it.
type Form struct {
	Op    string // mnemonic, lowercase and canonical
	Slots []Slot // Intel order: destination first

	Enc    Enc
	Map    Map
	Pfx    Pfx
	Opcode byte
	Ext    int8 // /digit, or NoExt for /r
	W      WBit
	Len    VLen

	Imm   ImmW  // derived from Slots
	Attrs Attr  // HasModRM is derived; the rest are written
	Tuple Tuple // EVEX only
	Elem  int8  // broadcast element size in bytes, 4 or 8; zero if none

	// Need is the feature that gates this form. The zero value is
	// feature.Base, which every Set contains, so a base form needs no entry
	// and no nil check reaches encode/.
	Need feature.Feature

	index int // position in All(); Resolve's tie-break
}

// Enabled reports whether s permits this form.
func (f *Form) Enabled(s feature.Set) bool { return s.Has(f.Need) }

// Index is the form's position in All(). decode/ uses it as a stable
// identity; Resolve uses it to break a size tie toward the earlier row.
func (f *Form) Index() int { return f.index }

// Explicit is the slots a caller supplies, in order. It excludes implicit
// operands and nothing else — a fixed operand like AL is explicit, because
// the syntax names it even though the encoding does not carry it.
func (f *Form) Explicit() []Slot {
	out := make([]Slot, 0, len(f.Slots))
	for _, s := range f.Slots {
		if !s.Implicit {
			out = append(out, s)
		}
	}
	return out
}

// Arity is len(Explicit()).
func (f *Form) Arity() int {
	n := 0
	for _, s := range f.Slots {
		if !s.Implicit {
			n++
		}
	}
	return n
}

// MemSlot is the index into Slots of the slot that may hold memory, or -1.
// At most one slot can: there is one ModRM.rm field.
func (f *Form) MemSlot() int {
	for i, s := range f.Slots {
		if s.Class.AcceptsMem() {
			return i
		}
	}
	return -1
}

// finish derives what the table should not have to state and rejects what
// cannot be encoded. It runs at init, so a bad row is a panic at program
// start rather than a wrong byte at Serialize.
func (f *Form) finish() {
	if f.Op == "" {
		panic("isa: form with no mnemonic")
	}

	var sawReg, sawRM, sawOpcode, sawIS4, sawMoffs bool
	for _, s := range f.Slots {
		switch s.Field {
		case InReg:
			if sawReg {
				panic("isa: " + f.Op + ": two slots in ModRM.reg")
			}
			sawReg = true
		case InRM:
			if sawRM {
				panic("isa: " + f.Op + ": two slots in ModRM.rm")
			}
			sawRM = true
		case InOpcode:
			sawOpcode = true
		case InIS4:
			sawIS4 = true
		case InMoffs:
			sawMoffs = true
		case InImm:
			switch s.Class {
			case Imm8, Rel8:
				f.Imm = ImmB
			case Imm16:
				f.Imm = ImmWord
			case Imm32, Rel32:
				f.Imm = ImmD
			case Imm64:
				f.Imm = ImmQ
			default:
				panic("isa: " + f.Op + ": " + s.Class.String() + " is not an immediate")
			}
			if s.Class.IsRel() {
				f.Attrs |= Branch
			}
		}
		if s.Class.MemOnly() && s.Field != InRM && s.Field != InMoffs {
			panic("isa: " + f.Op + ": memory operand outside ModRM.rm")
		}
	}
	if sawIS4 && f.Imm == ImmNone {
		f.Imm = ImmB // the is4 byte is the immediate byte
	}

	if sawReg || sawRM || f.Ext != NoExt {
		f.Attrs |= HasModRM
	}
	if sawReg && f.Ext != NoExt {
		panic("isa: " + f.Op + ": /digit and a register both in ModRM.reg")
	}
	if sawOpcode {
		f.Attrs |= PlusReg
	}
	if sawMoffs && f.Attrs&HasModRM != 0 {
		panic("isa: " + f.Op + ": moffs and ModRM are the same bytes")
	}
	if f.Attrs&PlusReg != 0 && f.Attrs&HasModRM != 0 {
		panic("isa: " + f.Op + ": +r and ModRM both claim the register")
	}

	switch f.Enc {
	case EncLegacy:
		if f.Len != LNone {
			panic("isa: " + f.Op + ": a legacy form has no vector length")
		}
		if f.Tuple != TupleNone {
			panic("isa: " + f.Op + ": a legacy form has no tuple")
		}
	case EncVEX, EncEVEX:
		if f.Map == Map1 {
			panic("isa: " + f.Op + ": VEX and EVEX have no one-byte map")
		}
		if f.Len == LNone {
			panic("isa: " + f.Op + ": vector length not stated")
		}
		if f.Attrs&Data16 != 0 {
			panic("isa: " + f.Op + ": 66 is pp here, not an operand-size override")
		}
	}
	if f.Enc == EncEVEX && f.MemSlot() >= 0 && f.Tuple == TupleNone {
		// Without a tuple there is no N, and without N a disp8 form is
		// unencodable. Refusing here is the difference between a build
		// failure and a wrong displacement.
		panic("isa: " + f.Op + ": EVEX form takes memory but states no tuple")
	}
	if f.Attrs&Broadcast != 0 && f.Elem == 0 {
		panic("isa: " + f.Op + ": broadcast without an element size")
	}
	if f.Attrs&Zeroing != 0 && f.Attrs&Masked == 0 {
		panic("isa: " + f.Op + ": {z} without {k}")
	}
	if f.Attrs&RoundCtl != 0 {
		f.Attrs |= SAE
	}
}

// GoName is the generated helper's name: the mnemonic, then each explicit
// slot's class. MovR64Imm64 is MOV r64, imm64 and nothing else will quietly
// relax it to MOV r/m64, imm32.
func (f *Form) GoName() string {
	var b strings.Builder
	b.WriteString(strings.ToUpper(f.Op[:1]))
	b.WriteString(f.Op[1:])
	for _, s := range f.Slots {
		if s.Implicit {
			continue
		}
		b.WriteString(s.Class.GoName())
	}
	return b.String()
}

// String is the SDM instruction column: MOV r/m64, imm32.
func (f *Form) String() string {
	var parts []string
	for _, s := range f.Slots {
		if s.Implicit {
			continue
		}
		parts = append(parts, s.Class.String())
	}
	up := strings.ToUpper(f.Op)
	if len(parts) == 0 {
		return up
	}
	return up + " " + strings.Join(parts, ", ")
}

// Opcodes is the SDM opcode column: "REX.W + C7 /0 id",
// "EVEX.512.66.0F.W0 FE /r". This is what `arc isa` prints and what a
// differential test diffs against the manual, so it is spelled the manual's
// way rather than this package's.
func (f *Form) Opcodes() string {
	var b strings.Builder

	switch f.Enc {
	case EncLegacy:
		if f.Attrs&Data16 != 0 {
			b.WriteString("66 ")
		}
		if p := f.Pfx.Byte(); p != 0 {
			fmt.Fprintf(&b, "%02X ", p)
		}
		if f.W == W1 {
			b.WriteString("REX.W + ")
		}
		if m := f.Map.String(); m != "" {
			b.WriteString(strings.Join(splitPairs(m), " ") + " ")
		}
	case EncVEX, EncEVEX:
		b.WriteString(f.Enc.String())
		b.WriteString(".")
		b.WriteString(f.Len.String())
		b.WriteString(".")
		if f.Pfx != PfxNone {
			b.WriteString(f.Pfx.String() + ".")
		}
		b.WriteString(f.Map.String() + ".")
		b.WriteString(f.W.String())
		b.WriteString(" ")
	}

	fmt.Fprintf(&b, "%02X", f.Opcode)
	if f.Attrs&PlusReg != 0 {
		b.WriteString("+r")
	}
	if f.Ext != NoExt {
		fmt.Fprintf(&b, " /%d", f.Ext)
	} else if f.Attrs&HasModRM != 0 {
		b.WriteString(" /r")
	}
	if s := f.Imm.String(); s != "" {
		b.WriteString(" " + s)
	}
	return b.String()
}

func splitPairs(s string) []string {
	var out []string
	for i := 0; i+1 < len(s)+1; i += 2 {
		out = append(out, s[i:i+2])
	}
	return out
}