// Package encode turns a resolved form and a set of operand values into one
// 32-bit word and its fixups.
//
// It is a pure function. Nothing here reads a section, a symbol table, or an
// object format: a field whose value is an address that is not yet a number
// comes back as a Fixup and the field is left zero. Which relocation that
// becomes is the platform writer's answer, and the encoder does not know which
// formats exist.
//
// There are no prefixes and no displacement sizing. Every instruction is one
// word, so the work is bit-field placement, the immediates the architecture
// computes rather than copies, and deciding which of those two a slot is.
package encode

import (
	"fmt"

	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// CountError is the wrong number of operands for a form.
type CountError struct {
	Form *isa.Form
	Got  int
}

func (e *CountError) Error() string {
	if e.Form.Required() == e.Form.Arity() {
		return fmt.Sprintf("%s takes %d operands, got %d",
			e.Form.Mnem, e.Form.Required(), e.Got)
	}
	return fmt.Sprintf("%s takes %d to %d operands, got %d",
		e.Form.Mnem, e.Form.Required(), e.Form.Arity(), e.Got)
}

// OperandError is a value that cannot fill a slot of this class.
type OperandError struct {
	Form  *isa.Form
	Index int
	Class isa.Class
	Got   any
}

func (e *OperandError) Error() string {
	return fmt.Sprintf("%s operand %d: expected %s, got %T",
		e.Form.Mnem, e.Index+1, e.Class, e.Got)
}

// RegisterError is a register of the wrong file or the wrong reading of
// number 31.
type RegisterError struct {
	Form  *isa.Form
	Index int
	Class isa.Class
	Name  string
	Why   string
}

func (e *RegisterError) Error() string {
	return fmt.Sprintf("%s operand %d: %s is not a %s: %s",
		e.Form.Mnem, e.Index+1, e.Name, e.Class, e.Why)
}

// RangeError is a value with no encoding in its field. It names the field's
// shape rather than its bit positions, because the shape is what the caller
// can act on.
type RangeError struct {
	Form  *isa.Form
	Index int
	Value int64
	Want  string
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("%s operand %d: %d does not fit: %s",
		e.Form.Mnem, e.Index+1, e.Value, e.Want)
}

// BitmaskError is a constant that is not a logical immediate.
//
// It is separate from RangeError because the fix is different in kind. A value
// out of range gets smaller; a value that is not a rotated run of ones does not
// get more expressible, and the caller has to materialize it into a register
// instead.
type BitmaskError struct {
	Form  *isa.Form
	Index int
	Value uint64
}

func (e *BitmaskError) Error() string {
	switch e.Value {
	case 0:
		return fmt.Sprintf("%s: 0 is not a logical immediate; the encoding names "+
			"a run of ones and cannot name an empty one", e.Form.Mnem)
	case ^uint64(0):
		return fmt.Sprintf("%s: an all-ones immediate has no logical encoding; "+
			"the run of ones must be shorter than the register", e.Form.Mnem)
	}
	return fmt.Sprintf("%s operand %d: %#x is not a logical immediate — it is not "+
		"a rotated run of ones replicated to fill the register; materialize it "+
		"with movz/movk and use the register form",
		e.Form.Mnem, e.Index+1, e.Value)
}

// AddressError is a memory operand whose shape the form does not encode.
type AddressError struct {
	Form  *isa.Form
	Index int
	Addr  operand.Mem
	Why   string
}

func (e *AddressError) Error() string {
	return fmt.Sprintf("%s operand %d: %s: %s", e.Form.Mnem, e.Index+1, e.Addr, e.Why)
}

// UnsupportedError is a shape the architecture has and this encoder does not
// reach yet, named as such so a reader does not go looking for a mistake.
type UnsupportedError struct {
	Form *isa.Form
	What string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s: %s is not encodable here yet", e.Form.Mnem, e.What)
}