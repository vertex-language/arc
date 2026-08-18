// x86_64/encode/error.go
package encode

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/reg"
)

// These are this package's errors. The root wraps them into its own
// Error/ErrForm/ErrFeature vocabulary with a section and offset attached;
// nothing here knows where in a section it is being called from.
var (
	ErrNoRM                 = errors.New("form has a ModRM but no r/m operand")
	ErrNoImmediate          = errors.New("form has an immediate field but no immediate operand")
	ErrRIPWithoutModRM      = errors.New("rip-relative addressing needs a ModRM byte")
	ErrMoffsAddressing      = errors.New("a moffs operand is an absolute address with no base or index")
	ErrZeroWithoutMask      = errors.New("{z} requires a nonzero writemask")
	ErrBroadcastNeedsMemory = errors.New("{1toN} requires a memory operand")
	ErrRoundNot512          = errors.New("embedded rounding is only encodable at 512 bits")
	ErrRoundWithMemory      = errors.New("embedded rounding and a memory operand both need EVEX.b")
)

// CountError is the wrong number of operands for the form. It should not
// reach a user: isa.Resolve matched arity before this package ran, and a
// caller building a Form by hand is the only way here.
type CountError struct {
	Form      *isa.Form
	Got, Want int
}

func (e *CountError) Error() string {
	return fmt.Sprintf("%s takes %d operands, got %d", e.Form, e.Want, e.Got)
}

// OperandError is a value this package has no case for.
type OperandError struct{ Value any }

func (e *OperandError) Error() string {
	return fmt.Sprintf("%T is not an operand of this target", e.Value)
}

// RegisterError is a register with no encoding under the form's prefix.
type RegisterError struct {
	Reg reg.Reg
	Enc isa.Enc
}

func (e *RegisterError) Error() string {
	return fmt.Sprintf("%s has no %s encoding\n  note: registers 16 and above are reachable only through EVEX",
		e.Reg.Name(), e.Enc)
}

// RexConflictError is a byte register that a REX prefix would rename.
type RexConflictError struct{ Reg reg.Reg }

func (e *RexConflictError) Error() string {
	return fmt.Sprintf("%s is unreachable in an instruction that needs a REX prefix\n"+
		"  note: %s shares an encoding with spl/bpl/sil/dil, which REX selects instead",
		e.Reg.Name(), e.Reg.Name())
}

// ModifierError is an EVEX modifier the form does not accept.
type ModifierError struct {
	Form *isa.Form
	What string
}

func (e *ModifierError) Error() string {
	return fmt.Sprintf("%s does not take %s", e.Form, e.What)
}

// ImmediateError is a value too wide for the field the form declares.
type ImmediateError struct {
	Form  *isa.Form
	Value int64
	Size  int
}

func (e *ImmediateError) Error() string {
	return fmt.Sprintf("%d does not fit the %d-byte immediate of %s", e.Value, e.Size, e.Form)
}