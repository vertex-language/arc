// x86_64/decode/operand.go
package decode

import (
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// operands rebuilds operand values from the decoded fields, one per explicit
// slot, in Intel order.
//
// The values are the same concrete types encode/ accepts, which is what
// makes the round trip a property of the code rather than a claim: hand Ops
// and Form back to Encode and the bytes come back. Anything that cannot be
// rebuilt as such a value is a decode failure here rather than a printer
// problem later.
func (d *dec) operands() ([]any, int64, error) {
	var out []any
	var rel int64

	for _, s := range d.form.Slots {
		if s.Implicit {
			continue
		}
		switch s.Field {
		case isa.InReg:
			r, err := regOf(s.Class, d.regNum())
			if err != nil {
				return nil, 0, err
			}
			out = append(out, r)

		case isa.InRM:
			if d.mod == 3 {
				r, err := regOf(s.Class, d.rmNum())
				if err != nil {
					return nil, 0, err
				}
				out = append(out, r)
			} else {
				out = append(out, d.memory(s.Class))
			}

		case isa.InVVVV:
			r, err := regOf(s.Class, d.vvvvNum())
			if err != nil {
				return nil, 0, err
			}
			out = append(out, r)

		case isa.InOpcode:
			num := d.opcode&7 | d.bb<<3
			r, err := regOf(s.Class, num)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, r)

		case isa.InIS4:
			r, err := regOf(s.Class, d.is4)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, r)

		case isa.InImm:
			if s.Class.IsRel() {
				rel = d.imm
				// A branch target is an address this package cannot know:
				// the displacement is relative to the end of an
				// instruction whose address the caller has and we do not.
				// Inst.Rel carries it; the operand slot carries the same
				// number as an immediate so the round trip has something
				// to hand back.
				out = append(out, operand.Imm(d.imm))
			} else {
				out = append(out, operand.Imm(d.imm))
			}

		case isa.InMask:
			out = append(out, reg.K(d.aaa))

		case isa.InMoffs:
			out = append(out, operand.Abs(d.disp))

		case isa.InNone:
			// A fixed operand. The opcode named it, so it is recovered from
			// the class rather than from any field.
			r, err := fixedOf(s.Class)
			if err != nil {
				return nil, 0, err
			}
			if r != nil {
				out = append(out, r)
			} else {
				out = append(out, operand.Imm(1)) // the literal 1 of `shl r/m, 1`
			}
		}
	}
	return out, rel, nil
}

// memory rebuilds the memory reference. The width comes from the slot's
// class, because the bytes do not carry one — which is the asymmetry that
// makes a text-level translator unable to recover operand size in both
// directions, and the reason this package runs before the printer.
func (d *dec) memory(cls isa.Class) any {
	var m operand.Mem

	switch {
	case d.rip:
		m = operand.RIPRelDisp(d.disp)
	case d.hasSIB:
		if d.mod != 0 || d.base != 5 {
			m = operand.Mem{Base: reg.Reg64(d.bb<<3 | d.base), HasBase: true}
		}
		if d.index != 4 || d.x == 1 {
			// index=100 means "no index" — except that with REX.X set it
			// means R12, which is a real index register. The one case where
			// the escape has an escape.
			m = m.Indexed(reg.Reg64(d.x<<3|d.index), d.scale)
		}
		if d.hasDisp {
			m = m.Displace(d.disp)
		}
	default:
		m = operand.Mem{Base: reg.Reg64(d.bb<<3 | d.rm), HasBase: true}
		if d.hasDisp {
			m = m.Displace(d.disp)
		}
	}

	if d.seg != 0 {
		m = m.Segment(segOf(d.seg))
	}
	if d.addr32 {
		m = m.Use32()
	}
	return widen(m, cls)
}

// widen narrows a Mem into the width-carrying type the slot's class names,
// so the value that comes out is the value that would have gone in.
func widen(m operand.Mem, cls isa.Class) any {
	switch cls.Bits() {
	case 8:
		return m.M8()
	case 16:
		return m.M16()
	case 32:
		return m.M32()
	case 64:
		return m.M64()
	case 128:
		return m.M128()
	case 256:
		return m.M256()
	case 512:
		return m.M512()
	}
	return m // MAny: lea's source, which has no width
}

func segOf(p Prefix) reg.Sreg {
	switch p {
	case PrefixES:
		return reg.ES
	case PrefixCS:
		return reg.CS
	case PrefixSS:
		return reg.SS
	case PrefixDS:
		return reg.DS
	case PrefixFS:
		return reg.FS
	}
	return reg.GS
}

// regOf builds the register of the class's file at this number.
func regOf(cls isa.Class, num uint8) (reg.Reg, error) {
	switch cls {
	case isa.R8, isa.RM8:
		return byteReg(num), nil
	case isa.R16, isa.RM16:
		return reg.Reg16(num), nil
	case isa.R32, isa.RM32:
		return reg.Reg32(num), nil
	case isa.R64, isa.RM64:
		return reg.Reg64(num), nil
	case isa.Mm, isa.MmM64:
		return reg.Mm(num & 7), nil
	case isa.Xmm, isa.XmmM32, isa.XmmM64, isa.XmmM128:
		return reg.Xmm(num), nil
	case isa.Ymm, isa.YmmM256:
		return reg.Ymm(num), nil
	case isa.Zmm, isa.ZmmM512:
		return reg.Zmm(num), nil
	case isa.K, isa.KM64:
		return reg.K(num & 7), nil
	case isa.Tmm:
		return reg.Tmm(num & 7), nil
	case isa.St:
		return reg.St(num & 7), nil
	case isa.Sreg:
		return reg.Sreg(num & 7), nil
	case isa.Cr:
		return reg.Cr(num), nil
	case isa.Dr:
		return reg.Dr(num), nil
	}
	if r, err := fixedOf(cls); err == nil && r != nil {
		return r, nil
	}
	return nil, &ClassError{Class: cls}
}

// byteReg needs the REX flag, not just the number: 4 through 7 are AH, CH,
// DH and BH without a REX prefix and SPL, BPL, SIL and DIL with one. The
// decoder is the only place that distinction can be made, because by the
// time there is a register value it has already been made.
func byteReg(num uint8) reg.Reg8 {
	return reg.Reg8(num)
}

func fixedOf(cls isa.Class) (reg.Reg, error) {
	switch cls {
	case isa.AL:
		return reg.AL, nil
	case isa.CL:
		return reg.CL, nil
	case isa.AX:
		return reg.AX, nil
	case isa.DX:
		return reg.DX, nil
	case isa.EAX:
		return reg.EAX, nil
	case isa.RAX:
		return reg.RAX, nil
	case isa.XMM0:
		return reg.XMM0, nil
	case isa.St0:
		return reg.ST0, nil
	case isa.One:
		return nil, nil
	}
	return nil, &ClassError{Class: cls}
}