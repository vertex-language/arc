// x86_64/decode/error.go
package decode

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/x86_64/isa"
)

var (
	ErrTruncated = errors.New("instruction ends mid-field")
	ErrTooLong   = errors.New("instruction exceeds the fifteen-byte limit")
)

// UnknownError is bytes that decode to no form.
//
// It reports how far it got, because the useful thing about an undecodable
// byte is where it is: a disassembler walking a section that hits data
// wants the offset, not a diagnosis.
type UnknownError struct {
	Bytes  []byte
	Offset int
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("no instruction at offset %d: %x", e.Offset, e.Bytes)
}

// ClassError is a slot whose class this package cannot turn back into an
// operand value. It is a table problem, not an input problem: a class added
// to isa/ without a case here decodes to nothing.
type ClassError struct{ Class isa.Class }

func (e *ClassError) Error() string {
	return fmt.Sprintf("no operand value for class %s", e.Class)
}