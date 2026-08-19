package operand

import "strconv"

// Extend decorates the second source of an extended-register form, or the
// index register of a register-offset address: it names the width the register
// is read at and whether the read is signed, before the shift is applied.
//
// The encoding is the three-bit option field, so these values are the field's
// own and not an arbitrary enumeration.
type Extend uint8

const (
	UXTB Extend = 0
	UXTH Extend = 1
	UXTW Extend = 2
	UXTX Extend = 3
	SXTB Extend = 4
	SXTH Extend = 5
	SXTW Extend = 6
	SXTX Extend = 7

	extendCount

	// ExtNone is "not stated". It is not zero, because zero is UXTB.
	ExtNone Extend = 0xff
)

// ExtLSL is how assembly spells UXTX when the index register is already
// sixty-four bits wide: [x1, x2, lsl #3] and [x1, x2, uxtx #3] are the same
// word, and the architecture's own disassembly prefers the former.
const ExtLSL = UXTX

func (e Extend) Valid() bool { return e < extendCount }

// Signed reports whether the source register is sign-extended.
func (e Extend) Signed() bool { return e.Valid() && e >= SXTB }

// Bits is the width the source register is read at.
func (e Extend) Bits() Width {
	switch e & 3 {
	case 0:
		return Width8
	case 1:
		return Width16
	case 2:
		return Width32
	}
	return Width64
}

// SourceIsW reports whether the extend reads a 32-bit register. It is what
// decides whether an index operand must be a W or an X, and the mismatch is
// the most common thing to get wrong when writing an address by hand.
func (e Extend) SourceIsW() bool { return e.Valid() && e.Bits() <= Width32 }

func (e Extend) String() string {
	switch e {
	case UXTB:
		return "uxtb"
	case UXTH:
		return "uxth"
	case UXTW:
		return "uxtw"
	case UXTX:
		return "uxtx"
	case SXTB:
		return "sxtb"
	case SXTH:
		return "sxth"
	case SXTW:
		return "sxtw"
	case SXTX:
		return "sxtx"
	}
	return "?"
}

// ExtendOp is an extend with its shift amount, which is what fills a slot.
type ExtendOp struct {
	Op     Extend
	Amount uint8
}

// Extended builds an extend operand.
func Extended(op Extend, amount uint8) ExtendOp { return ExtendOp{Op: op, Amount: amount} }

// Valid reports whether the amount has an encoding. The extended-register
// forms carry three bits and accept nothing above four.
func (e ExtendOp) Valid() bool { return e.Op.Valid() && e.Amount <= 4 }

func (e ExtendOp) String() string {
	if e.Amount == 0 {
		return e.Op.String()
	}
	return e.Op.String() + " #" + strconv.Itoa(int(e.Amount))
}