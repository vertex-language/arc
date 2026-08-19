package decode

import (
	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
)

// Rebuilding operand values from decoded fields.
//
// Every function here is the inverse of something in encode/, and the pairing
// is the round-trip guarantee: what comes out is the concrete type that went
// in, so encode.EncodeForm(f, inst.Ops, opts) returns the word this started
// from. Where the inverse is not exact — a field the encoder computed from the
// value rather than copying — the note says which direction is lossy and what
// is emitted instead.

// regOf rebuilds a register from a field value and a slot's class.
func regOf(c isa.Class, n uint64) (reg.Reg, bool) {
	switch c {
	case isa.ClassX:
		return reg.X(n), true
	case isa.ClassW:
		return reg.W(n), true
	case isa.ClassXsp:
		if n == 31 {
			return reg.SP, true
		}
		return reg.Xsp(n), true
	case isa.ClassWsp:
		if n == 31 {
			return reg.WSP, true
		}
		return reg.Wsp(n), true
	case isa.ClassV:
		return reg.V(n), true
	case isa.ClassQ:
		return reg.Q(n), true
	case isa.ClassD:
		return reg.D(n), true
	case isa.ClassS:
		return reg.S(n), true
	case isa.ClassH:
		return reg.H(n), true
	case isa.ClassB:
		return reg.B(n), true
	case isa.ClassZ:
		return reg.Z(n), true
	case isa.ClassP, isa.ClassPg:
		return reg.P(n), true
	}
	return nil, false
}

// immOf rebuilds an immediate, applying the inverse of the slot's rule.
//
// The word is needed as well as the field, because three of the rules put part
// of their value somewhere else: the LSL #12 bit of an ADD, the halfword
// position of a MOVZ, and the N bit of a logical immediate all live in fields
// the immediate slot does not name.
func immOf(f *isa.Form, s isa.Slot, word uint32) (any, any, error) {
	raw := s.Field.Get(word)
	bits := s.Field.Width()

	switch s.Imm {
	case isa.ImmNone, isa.ImmPlain:
		return operand.Imm(signed(raw, bits)), nil, nil

	case isa.ImmRaw32:
		return operand.Imm(raw), nil, nil

	case isa.ImmAddSub12:
		// The shift is a separate slot, so it decodes on its own pass and the
		// value stays as written. Emitting the folded constant instead would
		// print "add x0, x0, #4096" for a word that says "#1, lsl #12", and
		// re-encoding it would produce the same word only by accident of
		// FitsImm12 finding the same answer.
		return operand.Imm(raw), nil, nil

	case isa.ImmMoveWide:
		hw := fieldOf(f, isa.ClassShift)
		sh := uint8(hw.Get(word)) * 16
		if sh == 0 {
			return operand.Imm(raw), nil, nil
		}
		// An explicit shift rather than the folded value. movz x0, #0, lsl #16
		// is a real word whose folded value is zero, and encoding zero back
		// would put hw at 0 and produce a different word.
		return operand.Imm(raw), operand.Shifted(operand.LSL, sh), nil

	case isa.ImmLogical:
		n := uint8(raw >> 12 & 1)
		immr := uint8(raw >> 6 & 0x3f)
		imms := uint8(raw & 0x3f)
		v, ok := operand.DecodeBitmask(n, immr, imms, operand.Width(regWidth(f)))
		if !ok {
			return nil, nil, &ClassError{f, 0, s.Field, raw,
				"not a logical immediate: the run of ones is empty or complete"}
		}
		return operand.Imm(int64(v)), nil, nil

	case isa.ImmUnscaled:
		return operand.Imm(signed(raw, 9)), nil, nil

	case isa.ImmScaled:
		w := operand.Width(f.AccessBits())
		sc, _ := w.Scale()
		switch bits {
		case 12:
			return operand.Imm(int64(raw) << sc), nil, nil
		case 7:
			return operand.Imm(signed(raw, 7) << sc), nil, nil
		}
		return operand.Imm(int64(raw) << sc), nil, nil

	case isa.ImmBitPos:
		return operand.Imm(int64(raw)), nil, nil

	case isa.ImmBranch:
		return operand.Imm(signed(raw, bits) * 4), nil, nil

	case isa.ImmPage:
		return operand.Imm(signed(raw, bits) * 4096), nil, nil
	}
	return operand.Imm(int64(raw)), nil, nil
}

// memOf rebuilds a memory operand from a base slot and an optional offset slot.
//
// Two slots become one operand, which is why the caller advances its slot index
// by the returned count and its operand list by one — the same asymmetry
// encode/ walks in the other direction.
func memOf(f *isa.Form, base isa.Slot, off *isa.Slot, word uint32) (operand.Mem, int, error) {
	n := base.Field.Get(word)
	var m operand.Mem
	if n == 31 {
		m = memWidth(reg.SP, operand.Width(base.Class.AccessBits()))
	} else {
		m = memWidth(reg.Xsp(n), operand.Width(base.Class.AccessBits()))
	}

	if off == nil {
		return m, 1, nil
	}

	v, _, err := immOf(f, *off, word)
	if err != nil {
		return m, 0, err
	}
	d := int64(v.(operand.Imm))

	switch {
	case f.Attrs&isa.AttrPreIndex != 0:
		return m.Pre(d), 2, nil
	case f.Attrs&isa.AttrPostIndex != 0:
		return m.Post(d), 2, nil
	}
	if d == 0 {
		// [x1, #0] and [x1] are the same word. The bare form is what the
		// architecture's disassembly prints, and it re-encodes identically
		// because a zero offset is the field's zero.
		return m, 2, nil
	}
	return m.Off(d), 2, nil
}

func memWidth(base reg.Xsp, w operand.Width) operand.Mem {
	switch w {
	case operand.Width8:
		return operand.Mem8(base)
	case operand.Width16:
		return operand.Mem16(base)
	case operand.Width32:
		return operand.Mem32(base)
	case operand.Width64:
		return operand.Mem64(base)
	case operand.Width128:
		return operand.Mem128(base)
	}
	return operand.MemOf(base)
}

// shiftOf rebuilds a shift operand.
//
// The amount is not in this field. A shifted-register form puts the kind at
// 23:22 and the amount in its own immediate slot, so the amount is read from
// there and the two are recombined into the one operand a caller writes.
func shiftOf(f *isa.Form, s isa.Slot, word uint32) operand.ShiftOp {
	if s.Field.Width() == 1 {
		if s.Field.Get(word) == 1 {
			return operand.Shifted(operand.LSL, 12)
		}
		return operand.NoShift
	}
	op := operand.Shift(s.Field.Get(word))
	amt := uint8(0)
	if fld := fieldOf(f, isa.ClassImm); fld.N != 0 {
		amt = uint8(fld.Get(word))
	}
	return operand.Shifted(op, amt)
}

// extendOf rebuilds an extend operand, with its amount from the sibling field
// the encoder placed it in.
func extendOf(f *isa.Form, s isa.Slot, word uint32) operand.ExtendOp {
	op := operand.Extend(s.Field.Get(word))
	amt := uint8(0)
	if fld := fieldOf(f, isa.ClassImm); fld.N != 0 && f.Attrs&isa.AttrScaled == 0 {
		amt = uint8(fld.Get(word))
	}
	return operand.Extended(op, amt)
}

// sysOf rebuilds a system register. The instruction carries o0 rather than op0,
// so the high bit that reg.Sys packs is restored from the encoding's own
// definition of op0 as 2+o0.
func sysOf(v uint64) reg.Sys {
	return reg.Sys(uint16(v) | 1<<15)
}

// fieldOf finds the field of the first slot of a class, the read side of
// encode/'s siblingField.
func fieldOf(f *isa.Form, c isa.Class) isa.Field {
	for _, s := range f.Slots {
		if s.Class == c {
			return s.Field
		}
	}
	return isa.Field{}
}

// regWidth is the datasize of a form's register operands, encode/slotWidth's
// twin. Both read it off the first register slot; a form whose registers
// disagreed about width would not be a form, and the two must agree or the
// logical immediate decodes at the wrong element size.
func regWidth(f *isa.Form) uint16 {
	for _, s := range f.Slots {
		if s.Class.Reg() {
			return s.Class.Bits()
		}
	}
	return 64
}

func signed(v uint64, bits uint8) int64 {
	if bits == 0 || bits >= 64 {
		return int64(v)
	}
	if v&(1<<(bits-1)) != 0 {
		return int64(v) - (1 << bits)
	}
	return int64(v)
}