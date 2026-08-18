// x86_64/decode/table.go
package decode

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64/isa"
)

// key is everything that selects a form before the ModRM byte is read. The
// map is keyed on it rather than on the opcode alone because the same
// opcode byte means different instructions under different prefixes — 0F 10
// is MOVUPS, MOVUPD, MOVSS or MOVSD depending on nothing but pp.
type key struct {
	enc    isa.Enc
	m      isa.Map
	pfx    isa.Pfx
	opcode byte
}

// bucket is every form sharing a key, in table order. The list is short —
// usually one to four rows — and is walked linearly, discriminating on the
// fields that only ModRM and the operand widths can settle: the /digit, W,
// L, and whether r/m is a register or memory.
type bucket []*isa.Form

var opmap map[key]bucket

// plusMask marks opcodes whose low three bits are a register rather than
// part of the opcode. A lookup has to mask them off before it can hit, and
// it can only know to do that from the table.
var plusMask map[key]bool

func init() {
	opmap = make(map[key]bucket, 2048)
	plusMask = make(map[key]bool, 64)

	for _, f := range isa.All() {
		k := key{enc: f.Enc, m: f.Map, pfx: f.Pfx, opcode: f.Opcode}
		if f.Attrs&isa.PlusReg != 0 {
			// The eight opcodes B8..BF are one row. Register the base and
			// remember that the low bits are an operand.
			k.opcode = f.Opcode &^ 7
			plusMask[k] = true
		}
		opmap[k] = append(opmap[k], f)
	}

	// A legacy encoding distinguishes the operand-size override from a
	// mandatory 66 by which field the table put it in. If a form ever
	// claimed both, no decoder could tell them apart and this is where the
	// contradiction surfaces.
	for _, f := range isa.All() {
		if f.Attrs&isa.Data16 != 0 && f.Pfx == isa.Pfx66 {
			panic(fmt.Sprintf("decode: %s claims 66 as both operand size and mandatory prefix", f))
		}
	}
}

// match narrows the bucket to one form.
func (d *dec) match() error {
	k := key{enc: d.enc, m: d.mapOf(), pfx: d.pfxOf(), opcode: d.opcode}

	forms, ok := opmap[k]
	if !ok {
		masked := k
		masked.opcode = d.opcode &^ 7
		if plusMask[masked] {
			forms, ok = opmap[masked]
		}
	}
	if !ok || len(forms) == 0 {
		return &UnknownError{Bytes: d.b[:min(d.pos, len(d.b))], Offset: d.pos - 1}
	}

	// Some rows need the ModRM byte to discriminate, so peek without
	// consuming: /digit is in reg, and register-versus-memory is in mod.
	// Reading it here and again in modrm() is deliberate — the alternative
	// is a decoder whose position depends on which candidate it is testing.
	modrm, hasModRM := d.peek()

	var best *isa.Form
	for _, f := range forms {
		if !d.fits(f, modrm, hasModRM) {
			continue
		}
		if best == nil {
			best = f
		}
	}
	if best == nil {
		return &UnknownError{Bytes: d.b[:min(d.pos, len(d.b))], Offset: d.pos - 1}
	}
	d.form = best
	return nil
}

func (d *dec) fits(f *isa.Form, modrm byte, hasModRM bool) bool {
	// Operand size. In a legacy encoding REX.W and the 66 prefix are two
	// separate questions and both have to agree with the row.
	if f.Enc == isa.EncLegacy {
		w := d.rex>>3&1 == 1
		if (f.W == isa.W1) != w {
			return false
		}
		if (f.Attrs&isa.Data16 != 0) != d.data16 {
			return false
		}
	} else {
		if f.W != isa.WIG && (f.W == isa.W1) != (d.vw == 1) {
			return false
		}
		if !d.lenFits(f) {
			return false
		}
	}

	if f.Attrs&isa.HasModRM != 0 {
		if !hasModRM {
			return false
		}
		if f.Ext != isa.NoExt && byte(f.Ext) != modrm>>3&7 {
			return false
		}
		// A memory-only slot cannot have mod=11, and a register-only r/m
		// cannot have anything else. This is what separates the two halves
		// of the 0F 1F group and the FF group's memory and register rows.
		if i := f.MemSlot(); i >= 0 {
			cls := f.Slots[i].Class
			isMem := modrm>>6 != 3
			if cls.MemOnly() && !isMem {
				return false
			}
			if !cls.AcceptsMem() && isMem {
				return false
			}
		}
	}
	return true
}

// lenFits compares the row's vector length against L'L. Rounding control
// borrows the field, so a 512-bit form with EVEX.b set over a register
// operand matches any L'L — the bits are a mode, not a length.
func (d *dec) lenFits(f *isa.Form) bool {
	if f.Len == isa.LIG {
		return true
	}
	if f.Len == isa.LZ {
		return d.ll == 0
	}
	if d.enc == isa.EncEVEX && d.bcst && f.Attrs&isa.RoundCtl != 0 {
		if modrm, ok := d.peek(); ok && modrm>>6 == 3 {
			return f.Len == isa.L512
		}
	}
	want := byte(0)
	switch f.Len {
	case isa.L256:
		want = 1
	case isa.L512:
		want = 2
	}
	return d.ll == want
}

func (d *dec) mapOf() isa.Map {
	if d.enc == isa.EncLegacy {
		return d.legacyMap
	}
	return d.vmap
}

func (d *dec) pfxOf() isa.Pfx {
	if d.enc == isa.EncLegacy {
		return d.legacyPfx
	}
	return d.vpp
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}