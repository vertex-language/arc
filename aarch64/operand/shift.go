package operand

import "strconv"

// Shift decorates a register operand: the second source of a shifted-register
// form, or the twelve-bit immediate of ADD and SUB.
//
// A shift is not a different instruction. add x0, x1, x2, lsl #3 is the same
// form as add x0, x1, x2 with a field filled in, which is why this is an
// operand type and not a mnemonic suffix.
type Shift uint8

const (
	LSL Shift = 0
	LSR Shift = 1
	ASR Shift = 2
	ROR Shift = 3

	shiftCount
)

func (s Shift) Valid() bool { return s < shiftCount }

func (s Shift) String() string {
	switch s {
	case LSL:
		return "lsl"
	case LSR:
		return "lsr"
	case ASR:
		return "asr"
	case ROR:
		return "ror"
	}
	return "?"
}

// ShiftOp is a shift with its amount, which is what fills a slot.
type ShiftOp struct {
	Op     Shift
	Amount uint8
}

// Shifted builds a shift operand.
func Shifted(op Shift, amount uint8) ShiftOp { return ShiftOp{Op: op, Amount: amount} }

// NoShift is the operand an omitted shift defaults to. It is LSL #0, which
// every form that takes a shift encodes as zero — the Default on the optional
// slot, stated here so a caller building operands at runtime can pass it
// explicitly rather than varying the argument count.
var NoShift = ShiftOp{Op: LSL, Amount: 0}

// Valid reports whether the amount is in range for a register of this width.
// Whether the *kind* is accepted is the form's: ROR is legal on the logical
// shifted-register forms and has no encoding on the arithmetic ones, and only
// isa/ knows which of the two a slot belongs to.
func (s ShiftOp) Valid(w Width) bool {
	if !s.Op.Valid() {
		return false
	}
	switch w {
	case Width32:
		return s.Amount < 32
	case Width64:
		return s.Amount < 64
	}
	return false
}

// ValidImm12 reports whether this shift is one of the two ADD and SUB accept on
// their immediate: LSL #0 or LSL #12, which is the Sh field's single bit.
func (s ShiftOp) ValidImm12() bool {
	return s.Op == LSL && (s.Amount == 0 || s.Amount == 12)
}

func (s ShiftOp) String() string {
	return s.Op.String() + " #" + strconv.Itoa(int(s.Amount))
}