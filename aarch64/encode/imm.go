package encode

import (
	"strconv"

	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// The immediate rules.
//
// Which arithmetic a slot's immediate goes through cannot be derived from the
// field, because isa.Field is a value type and two unrelated fields of the same
// shape compare equal — Imm6 and Imms are both F(10, 6). It cannot be derived
// from the class either, since ClassImm is one class covering all of them. So
// the rule is stated on the slot, and this file is the switch over it.
//
// Each case is the architecture's arithmetic, not the encoder's bookkeeping,
// which is why the predicates themselves live in operand/ and are only called
// from here.

// immResult is a field value together with anything it computed for a sibling
// field — currently only ADD and SUB's LSL #12 bit.
//
// MOVZ's hw is not here. A move-wide's shift is written by the caller as an
// operand, and the shift slot encodes it directly; when the caller omits it,
// the halfword search finds the position and the shift slot is not reached at
// all, so the search returns the position and encodeForm places it. Routing
// both paths through one sibling mechanism would mean the omitted case and the
// written case disagreed about which field is authoritative.
type immResult struct {
	value uint64

	sibling      uint64
	hasSibling   bool
	siblingClass isa.Class
}

// encodeImm applies a slot's immediate rule.
//
// explicitShift reports whether the caller wrote the form's shift operand. It
// changes the rule rather than adding to it: movz x0, #1, lsl #16 states its
// own halfword position, and the search that would find one is wrong to run.
func encodeImm(f *isa.Form, i int, s isa.Slot, v val, explicitShift bool) (immResult, error) {
	bits := s.Field.Width()

	switch s.Imm {
	case isa.ImmPlain, isa.ImmNone:
		if fitsU(v.uimm, bits) {
			return immResult{value: v.uimm}, nil
		}
		if fitsS(v.imm, bits) {
			return immResult{value: truncS(v.imm, bits)}, nil
		}
		return immResult{}, &RangeError{f, i, v.imm, plural(bits) + " unsigned, or signed"}

	case isa.ImmRaw32:
		if v.uimm > 0xffffffff {
			return immResult{}, &RangeError{f, i, v.imm, "a 32-bit word"}
		}
		return immResult{value: v.uimm}, nil

	case isa.ImmAddSub12:
		if explicitShift {
			if !fitsU(v.uimm, 12) {
				return immResult{}, &RangeError{f, i, v.imm,
					"12 unsigned bits when the shift is written explicitly"}
			}
			return immResult{value: v.uimm}, nil
		}
		imm, shifted, ok := operand.FitsImm12(v.imm)
		if !ok {
			return immResult{}, &RangeError{f, i, v.imm,
				"12 unsigned bits, optionally shifted left by 12 " +
					"(so 0-4095, or a multiple of 4096 up to 16773120)"}
		}
		r := immResult{value: uint64(imm)}
		if shifted {
			r.sibling, r.hasSibling, r.siblingClass = 1, true, isa.ClassShift
		}
		return r, nil

	case isa.ImmMoveWide:
		w := operand.Width(FormWidth(f))
		if explicitShift {
			if !fitsU(v.uimm, 16) {
				return immResult{}, &RangeError{f, i, v.imm,
					"16 unsigned bits when the shift is written explicitly"}
			}
			return immResult{value: v.uimm}, nil
		}
		imm, hw, ok := operand.FitsImm16Shifted(v.uimm, w)
		if !ok {
			return immResult{}, &RangeError{f, i, v.imm,
				"one 16-bit halfword of a " + w.String() +
					"; a value spanning two needs a movz/movk pair, which is " +
					"two instructions and therefore the caller's to write"}
		}
		return immResult{
			value: uint64(imm), sibling: uint64(hw),
			hasSibling: true, siblingClass: isa.ClassShift,
		}, nil

	case isa.ImmLogical:
		w := operand.Width(FormWidth(f))
		n, immr, imms, ok := operand.EncodeBitmask(v.uimm, w)
		if !ok {
			return immResult{}, &BitmaskError{f, i, v.uimm}
		}
		// N:immr:imms, contiguous at 22:10, which is why the slot's field is
		// thirteen bits wide and not the six of imms alone.
		return immResult{value: uint64(n)<<12 | uint64(immr)<<6 | uint64(imms)}, nil

	case isa.ImmUnscaled:
		imm, ok := operand.FitsImm9(v.imm)
		if !ok {
			return immResult{}, &RangeError{f, i, v.imm, "9 signed bits, unscaled (-256 to 255)"}
		}
		return immResult{value: uint64(imm)}, nil

	case isa.ImmScaled:
		w := operand.Width(f.AccessBits())
		switch bits {
		case 12:
			imm, ok := operand.FitsUImm12Scaled(v.imm, w)
			if !ok {
				return immResult{}, &RangeError{f, i, v.imm,
					"a non-negative multiple of " + strconv.Itoa(int(w.Bytes())) +
						" up to " + strconv.Itoa(int(w.Bytes())*4095)}
			}
			return immResult{value: uint64(imm)}, nil
		case 7:
			imm, ok := operand.FitsImm7Scaled(v.imm, w)
			if !ok {
				return immResult{}, &RangeError{f, i, v.imm,
					"a multiple of " + strconv.Itoa(int(w.Bytes())) + " from " +
						strconv.Itoa(-64*int(w.Bytes())) + " to " +
						strconv.Itoa(63*int(w.Bytes()))}
			}
			return immResult{value: uint64(imm)}, nil
		}
		return immResult{}, &UnsupportedError{f, "a scaled immediate of " + plural(bits)}

	case isa.ImmBitPos:
		if v.imm < 0 || v.imm > 63 {
			return immResult{}, &RangeError{f, i, v.imm, "a bit number from 0 to 63"}
		}
		return immResult{value: uint64(v.imm)}, nil

	case isa.ImmBranch:
		// A bare displacement rather than a label: the caller stated the
		// distance in bytes and takes responsibility for it.
		imm, ok := operand.FitsBranch(v.imm, bits)
		if !ok {
			return immResult{}, &RangeError{f, i, v.imm,
				"a word-aligned displacement in " + plural(bits) + " signed"}
		}
		return immResult{value: uint64(imm)}, nil

	case isa.ImmPage:
		imm, ok := operand.FitsPage(v.imm)
		if !ok {
			return immResult{}, &RangeError{f, i, v.imm,
				"a page-aligned displacement in 21 signed bits"}
		}
		return immResult{value: uint64(imm)}, nil
	}

	return immResult{}, &UnsupportedError{f, "an immediate rule this encoder does not know"}
}

// FormWidth is the datasize of a form's register operands, which is what the
// move-wide and logical-immediate rules are computed against. It is read off
// the first register slot rather than stored, because a form whose registers
// disagreed about width would not be a form.
//
// It is exported because decode/ needs the identical answer: the two must agree
// or a logical immediate decodes at the wrong element size and the round trip
// silently produces a different constant.
func FormWidth(f *isa.Form) uint16 {
	for _, s := range f.Slots {
		if s.Class.Reg() {
			return s.Class.Bits()
		}
	}
	return 64
}

// immKindOf is the form's immediate rule, or ImmNone. The shift encoder reads
// it, because what a shift field holds depends on which immediate it belongs
// to rather than on the field's own width.
func immKindOf(f *isa.Form) isa.ImmKind {
	for _, s := range f.Slots {
		if s.Class == isa.ClassImm && s.Imm != isa.ImmNone {
			return s.Imm
		}
	}
	return isa.ImmNone
}

func plural(bits uint8) string {
	if bits == 1 {
		return "1 bit"
	}
	return strconv.Itoa(int(bits)) + " bits"
}