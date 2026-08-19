// x86_64/error.go
package x86_64

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/text"
)

// The error categories this package reports. They are sentinels for
// errors.Is and carry no detail of their own — the detail is in the wrapped
// error, which came from isa/, encode/ or text/ and knows what went wrong
// without knowing where.
//
// A caller switching on these is asking a question with five answers: is
// this a feature I could enable, an instruction that does not exist in this
// shape, a relocation this format cannot record, a symbol nobody defined, or
// a platform that has no such thing.
var (
	// ErrFeature is a form that exists and matches, held back by the active
	// feature set. The diagnostic names the flag that would have allowed it.
	ErrFeature = errors.New("not in the active feature set")

	// ErrForm is a mnemonic with no encoding for these operands, or an
	// operand combination with no encoding at all.
	ErrForm = errors.New("no encoding for these operands")

	// ErrReloc is a relocation kind with no mapping in the chosen object
	// format — declared for completeness in reloc_*.go and refused at
	// Serialize rather than silently miscoded.
	ErrReloc = errors.New("relocation not available")

	// ErrUndefined is a fixup pointing at a symbol nothing defines and
	// nothing declares external.
	ErrUndefined = errors.New("undefined symbol")

	// ErrPlatform is a request this object format cannot express: a
	// cross-section reference on Flat, a Mach-O section name that is not a
	// segment,section pair, an unknown platform name.
	ErrPlatform = errors.New("not available on this platform")
)

// Error is a diagnostic with a place attached.
//
// Everything below this package produces errors that know what happened and
// not where: encode/ has no section, isa/ has no offset, and text/ has a
// line only when the input was text. This is where the two halves meet, and
// it is the only error type this package returns.
type Error struct {
	// Pos is the source position, when the work came from a file. It beats
	// Section and Offset for a reader, so it is printed instead of them.
	Pos text.Pos

	// Section and Offset locate the instruction in the object being built,
	// for the builder API, where there is no line to name.
	Section string
	Offset  int
	HasOff  bool

	// Err is the underlying diagnostic.
	Err error

	// Note is an extra line the reader can act on — for a gating error, the
	// Go expression that would have enabled the instruction.
	Note string
}

func (e *Error) Error() string {
	var b strings.Builder

	switch {
	case e.Pos.IsValid():
		b.WriteString(e.Pos.String() + ": ")
	case e.HasOff:
		// The object-file spelling of a location: `.text+0x1c`. Hex,
		// because every other tool that will print this offset — objdump,
		// readelf, a debugger — prints it in hex.
		fmt.Fprintf(&b, "%s+%#x: ", e.Section, e.Offset)
	case e.Section != "":
		b.WriteString(e.Section + ": ")
	}

	if e.Err != nil {
		b.WriteString(e.Err.Error())
	}
	if e.Note != "" {
		b.WriteString("\n  note: " + e.Note)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Is answers the category sentinels. The category is derived from the
// wrapped error rather than stored, so a new error type in isa/ or encode/
// classifies correctly the moment it is added to category() and cannot
// disagree with itself in the meantime.
func (e *Error) Is(target error) bool {
	c := category(e.Err)
	return c != nil && c == target
}

// category maps an underlying error to one of the five sentinels, or nil for
// something that is none of them.
func category(err error) error {
	if err == nil {
		return nil
	}

	var gate *isa.GateError
	if errors.As(err, &gate) {
		return ErrFeature
	}

	var form *isa.FormError
	var unknown *isa.UnknownError
	var count *encode.CountError
	var op *encode.OperandError
	var register *encode.RegisterError
	var rex *encode.RexConflictError
	var mod *encode.ModifierError
	var imm *encode.ImmediateError
	switch {
	case errors.As(err, &form),
		errors.As(err, &unknown),
		errors.As(err, &count),
		errors.As(err, &op),
		errors.As(err, &register),
		errors.As(err, &rex),
		errors.As(err, &mod),
		errors.As(err, &imm):
		return ErrForm
	}

	// encode/'s sentinel errors are all statements about an encoding that
	// does not exist, which is the same question ErrForm answers.
	switch {
	case errors.Is(err, encode.ErrNoRM),
		errors.Is(err, encode.ErrNoImmediate),
		errors.Is(err, encode.ErrRIPWithoutModRM),
		errors.Is(err, encode.ErrMoffsAddressing),
		errors.Is(err, encode.ErrZeroWithoutMask),
		errors.Is(err, encode.ErrBroadcastNeedsMemory),
		errors.Is(err, encode.ErrRoundNot512),
		errors.Is(err, encode.ErrRoundWithMemory):
		return ErrForm
	}

	// The operand's own rules, checked in operand/ where there is no line.
	switch {
	case errors.Is(err, operandErrScale),
		errors.Is(err, operandErrIndexRSP),
		errors.Is(err, operandErrRIPWithBase),
		errors.Is(err, operandErrRIPWithDisp):
		return ErrForm
	}

	// The sentinels this package raises itself are already categories.
	for _, c := range []error{ErrFeature, ErrForm, ErrReloc, ErrUndefined, ErrPlatform} {
		if errors.Is(err, c) {
			return c
		}
	}
	return nil
}

// at attaches a section and an offset to an error from below.
//
// This is the constructor asm.go and write.go use, and the only place a
// gating diagnostic gets its note line — built from the active set and the
// feature that was missing, so the message names something that compiles.
func at(section string, offset int, active FeatureSet, err error) error {
	if err == nil {
		return nil
	}
	e := &Error{Section: section, Offset: offset, HasOff: true, Err: err}

	var gate *isa.GateError
	if errors.As(err, &gate) {
		e.Note = "x86_64.WithFeatures(" + GoExpr(active.Plus(gate.Need)) + ")"
	}
	return e
}

// atPos attaches a source position instead, for work that came from text.
//
// A text.Error already carries a position and is returned unchanged: two
// positions on one message is one more than a reader can use, and the inner
// one is the specific one.
func atPos(pos text.Pos, active FeatureSet, err error) error {
	if err == nil {
		return nil
	}

	var te *text.Error
	if errors.As(err, &te) && te.Pos.IsValid() {
		pos = te.Pos
	}

	e := &Error{Pos: pos, Err: err}
	var gate *isa.GateError
	if errors.As(err, &gate) {
		e.Note = "--features " + active.Plus(gate.Need).String()
	}
	return e
}

// relocErr is a relocation kind the platform writer has no mapping for.
//
// It names the kind and the platform and says the mapping is missing rather
// than that the kind is wrong, because the kind is usually right: reloc_*.go
// declares the format's full set for completeness and only some of them are
// wired end to end. A caller who hits this has found a gap, not a mistake.
func relocErr(section string, offset int, kind RelocKind, p Platform) error {
	return &Error{
		Section: section, Offset: offset, HasOff: true,
		Err: fmt.Errorf("%w: %s has no %s mapping in this tree",
			ErrReloc, RelocName(kind), p),
	}
}

// undefinedErr is a fixup pointing at a name nothing defines.
func undefinedErr(section string, offset int, name string) error {
	return &Error{
		Section: section, Offset: offset, HasOff: true,
		Err: fmt.Errorf("%w: %s is referenced here and defined nowhere", ErrUndefined, name),
	}
}

// platformErr is something this object format cannot express.
func platformErr(p Platform, format string, args ...any) error {
	return &Error{Err: fmt.Errorf("%s: %w: %s", p, ErrPlatform, fmt.Sprintf(format, args...))}
}