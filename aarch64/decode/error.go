// Package decode is the inverse machine: a word back to the form that encodes
// it and the operand values that fill it.
//
// It is built from isa.All() and nothing else. There is no second table and no
// hand-written opcode map, because a decoder with its own table disagrees with
// the encoder eventually and the disagreement is a wrong disassembly of correct
// bytes — the hardest kind to notice.
//
// Decode's operand values are the exact concrete types encode.Encode accepts.
// Handing Inst.Form and Inst.Ops to encode.EncodeForm returns the word this
// package started from, which is the round trip the differential suite checks
// on every instruction it generates.
package decode

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/aarch64/isa"
)

// ErrTruncated is fewer than four bytes.
var ErrTruncated = errors.New("truncated instruction: fewer than 4 bytes")

// ErrUnaligned is a length that is not a multiple of four.
//
// It is an error rather than a truncated guess. Every A64 instruction is one
// word, so a buffer that does not divide by four is not a sequence of
// instructions with a tail — it is a buffer the caller sliced wrong, and
// decoding the part that fits would hide that.
var ErrUnaligned = errors.New("unaligned length: not a multiple of 4 bytes")

// UnknownError is a word no form in the table matches.
type UnknownError struct{ Word uint32 }

func (e *UnknownError) Error() string {
	return fmt.Sprintf("no form decodes %#08x", e.Word)
}

// ClassError is a field whose value names no operand: a system register in the
// SYS encoding space where an MRS was decoded, a condition code that is the
// never-taken encoding, a barrier option with no name.
//
// It is separate from UnknownError because the word did match a form. The
// instruction exists and one of its fields holds something this package cannot
// turn back into an operand, which is a different thing to go looking for.
type ClassError struct {
	Form  *isa.Form
	Index int
	Field isa.Field
	Value uint64
	Why   string
}

func (e *ClassError) Error() string {
	return fmt.Sprintf("%s operand %d: field %s holds %#x: %s",
		e.Form.Mnem, e.Index+1, e.Field, e.Value, e.Why)
}

// AmbiguousError is two non-alias forms matching one word. It is returned by
// the table's self-check rather than by Decode, which takes the more specific
// of the two; see table.go.
type AmbiguousError struct {
	Word uint32
	A, B *isa.Form
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("word %#08x matches both %s and %s",
		e.Word, e.A.Signature(), e.B.Signature())
}