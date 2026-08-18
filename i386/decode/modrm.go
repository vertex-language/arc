package decode

import (
	"fmt"

	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// ModRM and SIB decoding, Intel SDM Vol. 2, Tables 2-2 and 2-3.
//
// This is encode/modrm.go read backwards, and the four encodings that do not
// mean what their fields say are the four places a decoder gets it wrong
// rather than an assembler:
//
//   - mod=00, rm=101 is not [EBP]. It is disp32 with no base — plain absolute
//     addressing in 32-bit mode. The same encoding in 64-bit mode is
//     RIP-relative, which is why x86_64's decoder has a case this one does
//     not.
//   - rm=100 is not [ESP]. It means a SIB byte follows, at every mod but 11.
//   - SIB.index=100 means no index, so a SIB byte can encode ESP as a base
//     and can never encode it as an index.
//   - SIB.base=101 with mod=00 means no base, and a disp32 follows. With
//     mod=01 or mod=10 the same field is EBP, and the displacement is the
//     one the mod bits already called for.

// addr is a decoded effective address, in the terms operand/ builds one from.
type addr struct {
	base     reg.R32
	hasBase  bool
	index    reg.R32
	scale    uint8
	hasIndex bool
	disp     int32
	seg      reg.Sreg
	hasSeg   bool
}

// rm consumes the ModRM byte and whatever it announces — a SIB byte, a
// displacement, or neither — and returns the r/m operand and the reg field.
func (w *walker) rm(rmClass isa.Class, e *entry, p Prefixes) (operand.Operand, uint8, error) {
	off := w.pos
	b, err := w.u8()
	if err != nil {
		return nil, 0, err
	}
	mod, regField, rm := b>>6, (b>>3)&7, b&7

	regNote := fmt.Sprintf("→ reg %d", regField)
	if e.ext >= 0 {
		regNote = fmt.Sprintf("/%d", e.ext)
	} else if e.regSlot >= 0 {
		if r, err := regOperand(e.form.Ops[e.regSlot].Class, regField); err == nil {
			regNote = "→ " + name(r)
		}
	}

	// A register r/m, either because mod says so or because the class admits
	// nothing else. The second case is the control- and debug-register moves,
	// whose mod field the SDM says is ignored.
	if mod == 3 || (registerOnly(rmClass) && e.sysMove) {
		note := ""
		var op operand.Operand
		if e.rmSlot >= 0 {
			op, err = regOperand(rmClass, rm)
			if err != nil {
				return nil, 0, err
			}
			note = "→ " + name(op)
		}
		if mod != 3 {
			note += "; mod ignored for this form"
		}
		w.span(FieldModRM, off, 1, "ModRM", modrmValue(mod, regField, rm), note,
			BitField{Name: "mod", Hi: 7, Lo: 6, Value: uint32(mod), Note: "register"},
			BitField{Name: "reg", Hi: 5, Lo: 3, Value: uint32(regField), Note: regNote},
			BitField{Name: "rm", Hi: 2, Lo: 0, Value: uint32(rm), Note: note},
		)
		return op, regField, nil
	}

	if rmClass == isa.ClassNone {
		return nil, 0, fmt.Errorf("%w: %s has a ModRM byte and no r/m operand",
			ErrDecode, e.form.Signature())
	}

	var (
		a       addr
		sibOff  = -1
		sibByte byte
		dispOff = -1
		dispLen int
		modNote string
	)
	a.seg, a.hasSeg = p.Seg, p.HasSeg

	switch {
	case mod == 0 && rm == 5:
		// The slot that is not [EBP].
		dispLen, modNote = 4, "disp32, no base"

	case rm == 4:
		sibOff = w.pos
		sibByte, err = w.u8()
		if err != nil {
			return nil, 0, err
		}
		ss, index, base := sibByte>>6, (sibByte>>3)&7, sibByte&7

		if index != 4 {
			a.index, a.scale, a.hasIndex = reg.R32(index), 1<<ss, true
		}
		if base == 5 && mod == 0 {
			dispLen, modNote = 4, "SIB, no base, disp32"
		} else {
			a.base, a.hasBase = reg.R32(base), true
			dispLen, modNote = modDisp(mod), "SIB"
		}

	default:
		a.base, a.hasBase = reg.R32(rm), true
		dispLen, modNote = modDisp(mod), "base"
	}

	if dispLen > 0 {
		dispOff = w.pos
		raw, err := w.le(dispLen)
		if err != nil {
			return nil, 0, err
		}
		if dispLen == 1 {
			a.disp = int32(int8(raw))
		} else {
			a.disp = int32(raw)
		}
	}

	op, err := memOperand(rmClass, a)
	if err != nil {
		return nil, 0, err
	}
	if err := op.Err(); err != nil {
		return nil, 0, err
	}

	// The notes below name fields; they are not a spelling of the operand.
	// Printing the address is a dialect's job, and this package has none.
	w.span(FieldModRM, off, 1, "ModRM", modrmValue(mod, regField, rm), modNote,
		BitField{Name: "mod", Hi: 7, Lo: 6, Value: uint32(mod), Note: modNote},
		BitField{Name: "reg", Hi: 5, Lo: 3, Value: uint32(regField), Note: regNote},
		BitField{Name: "rm", Hi: 2, Lo: 0, Value: uint32(rm), Note: rmNote(rm, mod)},
	)
	if sibOff >= 0 {
		ss, index, base := sibByte>>6, (sibByte>>3)&7, sibByte&7
		w.span(FieldSIB, sibOff, 1,
			"SIB",
			fmt.Sprintf("scale=%02b index=%03b base=%03b", ss, index, base),
			sibNote(a),
			BitField{Name: "scale", Hi: 7, Lo: 6, Value: uint32(ss), Note: fmt.Sprintf("×%d", 1<<ss)},
			BitField{Name: "index", Hi: 5, Lo: 3, Value: uint32(index), Note: indexNote(a, index)},
			BitField{Name: "base", Hi: 2, Lo: 0, Value: uint32(base), Note: baseNote(a, base, mod)},
		)
	}
	if dispOff >= 0 {
		w.span(FieldDisp, dispOff, dispLen,
			fmt.Sprintf("disp%d", dispLen*8),
			fmt.Sprintf("%d", a.disp),
			hexN(uint32(a.disp), dispLen))
	}

	return op, regField, nil
}

// modDisp is the displacement width the mod bits call for, outside the two
// slots that mean something else.
func modDisp(mod byte) int {
	switch mod {
	case 1:
		return 1
	case 2:
		return 4
	}
	return 0
}

func modrmValue(mod, reg, rm byte) string {
	return fmt.Sprintf("mod=%02b reg=%03b rm=%03b", mod, reg, rm)
}

func rmNote(rm, mod byte) string {
	switch {
	case rm == 4:
		return "SIB follows"
	case rm == 5 && mod == 0:
		return "disp32, no base"
	}
	return "base " + reg.R32(rm).Name()
}

func sibNote(a addr) string {
	s := "no base"
	if a.hasBase {
		s = "base " + a.base.Name()
	}
	if a.hasIndex {
		s += fmt.Sprintf(", index %s ×%d", a.index.Name(), a.scale)
	} else {
		s += ", no index"
	}
	return s
}

func indexNote(a addr, index byte) string {
	if !a.hasIndex {
		return "no index"
	}
	return "→ " + reg.R32(index).Name()
}

func baseNote(a addr, base, mod byte) string {
	if !a.hasBase && mod == 0 {
		return "no base, disp32"
	}
	return "→ " + reg.R32(base).Name()
}

func name(o operand.Operand) string {
	if n, ok := o.(interface{ Name() string }); ok {
		return n.Name()
	}
	return "?"
}

// memOperand builds the memory operand of the width the class calls for.
//
// Class M has no access width — LEA's operand is an address, not an access —
// and operand/ has no width-less memory type, so it is materialised at 32
// bits. Nothing reads that width back: the encoder ignores it for Class M and
// the form's other operand carries the size a printer needs.
func memOperand(c isa.Class, a addr) (operand.Memory, error) {
	switch c {
	case isa.RM8:
		return mem8(a), nil
	case isa.RM16:
		return mem16(a), nil
	case isa.RM32, isa.M:
		return mem32(a), nil
	case isa.RM64:
		return mem64(a), nil
	case isa.RM128:
		return mem128(a), nil
	}
	return nil, fmt.Errorf("%w: %s is not a memory class", ErrDecode, c)
}

// One constructor per width, because the width is a type. This is the same
// duplication width.go carries and for the same reason: a name that cannot
// distinguish two widths cannot be the name of an operand.

func mem8(a addr) operand.M8 {
	m := operand.Abs8()
	if a.hasBase {
		m = operand.Mem8(a.base)
	}
	if a.hasIndex {
		m = m.Index(a.index, a.scale)
	}
	m = m.Disp(a.disp)
	if a.hasSeg {
		m = m.Segment(a.seg)
	}
	return m
}

func mem16(a addr) operand.M16 {
	m := operand.Abs16()
	if a.hasBase {
		m = operand.Mem16(a.base)
	}
	if a.hasIndex {
		m = m.Index(a.index, a.scale)
	}
	m = m.Disp(a.disp)
	if a.hasSeg {
		m = m.Segment(a.seg)
	}
	return m
}

func mem32(a addr) operand.M32 {
	m := operand.Abs32()
	if a.hasBase {
		m = operand.Mem32(a.base)
	}
	if a.hasIndex {
		m = m.Index(a.index, a.scale)
	}
	m = m.Disp(a.disp)
	if a.hasSeg {
		m = m.Segment(a.seg)
	}
	return m
}

func mem64(a addr) operand.M64 {
	m := operand.Abs64()
	if a.hasBase {
		m = operand.Mem64(a.base)
	}
	if a.hasIndex {
		m = m.Index(a.index, a.scale)
	}
	m = m.Disp(a.disp)
	if a.hasSeg {
		m = m.Segment(a.seg)
	}
	return m
}

func mem128(a addr) operand.M128 {
	m := operand.Abs128()
	if a.hasBase {
		m = operand.Mem128(a.base)
	}
	if a.hasIndex {
		m = m.Index(a.index, a.scale)
	}
	m = m.Disp(a.disp)
	if a.hasSeg {
		m = m.Segment(a.seg)
	}
	return m
}

// regOperand is the register a class and an encoding number name.
//
// The r/m classes appear here because a mod=11 r/m is a register: RM64's
// register inhabitant is an MMX register and RM128's is an XMM register, which
// is the class's own answer and not a decoder special case.
func regOperand(c isa.Class, n uint8) (operand.Operand, error) {
	switch c {
	case isa.R8, isa.RM8:
		return reg.R8(n), nil
	case isa.R16, isa.RM16:
		return reg.R16(n), nil
	case isa.R32, isa.RM32:
		return reg.R32(n), nil
	case isa.Sreg:
		if n > 5 {
			return nil, fmt.Errorf("%w: no segment register %d\n"+
				"  note: the six are es cs ss ds fs gs", ErrDecode, n)
		}
		return reg.Sreg(n), nil
	case isa.St:
		return reg.St(n), nil
	case isa.Mm, isa.RM64:
		return reg.Mm(n), nil
	case isa.Xmm, isa.RM128:
		return reg.Xmm(n), nil
	case isa.Ymm:
		return reg.Ymm(n), nil
	case isa.Zmm:
		return reg.Zmm(n), nil
	case isa.Cr:
		return reg.Cr(n), nil
	case isa.Dr:
		return reg.Dr(n), nil
	}
	return nil, fmt.Errorf("%w: %s is not a register class", ErrDecode, c)
}

// fixedOperand is the one value a fixed class names. These occupy no encoding
// field, which is what makes ADD EAX, imm32 a distinct form.
func fixedOperand(c isa.Class) (operand.Operand, error) {
	switch c {
	case isa.AL:
		return reg.AL, nil
	case isa.AX:
		return reg.AX, nil
	case isa.EAX:
		return reg.EAX, nil
	case isa.CL:
		return reg.CL, nil
	case isa.DX:
		return reg.DX, nil
	case isa.One:
		return operand.NewImm(1), nil
	}
	return nil, fmt.Errorf("%w: %s is not a fixed operand class", ErrDecode, c)
}