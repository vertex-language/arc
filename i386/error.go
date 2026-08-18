package i386

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386/feature"
)

// Error is one sticky error from a section: positioned, classified, and
// carrying whatever the classification needs to explain itself.
//
// codegen is a long run of calls that should not each be followed by three
// lines of error handling. Every error here is a programming error rather
// than a runtime condition, so a section keeps only the first and drops the
// rest; it surfaces at Err() or Serialize(). Nothing is silently encoded
// after a failure.
type Error struct {
	Section string       // ".text"
	Offset  uint32        // where it would have gone
	Op      string        // "vsetvli", "call", ".size" — whatever was being built
	Missing feature.Set    // non-empty for ErrFeature
	Active  feature.Set
	Err     error          // ErrFeature, ErrForm, ErrReloc, ErrUndefined, ErrPlatform

	msg   string
	notes []string
}

// The sentinels Error.Err wraps. errors.Is(err, ErrFeature) holds for a
// gated form, errors.Is(err, ErrForm) for an operand combination with no
// legal encoding, and so on — each is its own value so a caller can tell
// them apart without parsing the message.
var (
	// ErrFeature is a form gated behind an extension or base level not in
	// the active feature set.
	ErrFeature = fmt.Errorf("i386: requires a feature not in the active set")

	// ErrForm is an operand combination no declared form accepts.
	ErrForm = fmt.Errorf("i386: no form accepts these operands")

	// ErrReloc is a relocation kind that does not belong to the field it
	// landed in, or to the platform the section is being written for.
	ErrReloc = fmt.Errorf("i386: relocation does not fit")

	// ErrUndefined is a label with no definition anywhere in the section it
	// was referenced from.
	ErrUndefined = fmt.Errorf("i386: undefined label")

	// ErrPlatform is a call to the typed escape hatch — ELF, COFF — against
	// an Assembler built for a different platform.
	ErrPlatform = fmt.Errorf("i386: wrong platform for this call")
)

func (e *Error) Error() string {
	var b strings.Builder
	if e.Section != "" {
		fmt.Fprintf(&b, "%s+%#x: ", e.Section, e.Offset)
	}
	b.WriteString(e.msg)
	for _, n := range e.notes {
		b.WriteString("\n  note: ")
		b.WriteString(n)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Note appends a note and returns e, so a diagnostic reads as one expression
// at the call site that built it.
func (e *Error) Note(format string, args ...any) *Error {
	e.notes = append(e.notes, fmt.Sprintf(format, args...))
	return e
}

// featureErr builds the diagnostic Emit and the typed helpers give when a
// form is gated. active prints in canonical order, the same rule arc env
// follows: one spelling out, however the set was built.
func featureErr(section string, offset uint32, op string, missing, active feature.Set) *Error {
	e := &Error{
		Section: section, Offset: offset, Op: op,
		Missing: missing, Active: active, Err: ErrFeature,
		msg: fmt.Sprintf("%s requires %s, not in the active feature set", op, missing),
	}
	return e.Note("active: %s", active)
}

// formErr builds the diagnostic for an operand combination isa/ has no form
// for at all — as opposed to one it has a form for but that form is gated,
// which is featureErr's job.
func formErr(section string, offset uint32, op string) *Error {
	return &Error{
		Section: section, Offset: offset, Op: op, Err: ErrForm,
		msg: fmt.Sprintf("no form of %s accepts these operands", op),
	}
}

// relocErr builds the diagnostic for a relocation that does not belong where
// it landed — the wrong platform, or a field too narrow for the kind.
func relocErr(section string, offset uint32, msg string, notes ...string) *Error {
	e := &Error{Section: section, Offset: offset, Err: ErrReloc, msg: msg}
	for _, n := range notes {
		e.Note("%s", n)
	}
	return e
}

// undefinedErr builds the diagnostic for a label referenced but never
// defined in the section that referenced it.
func undefinedErr(section, name string) *Error {
	return &Error{
		Section: section, Err: ErrUndefined,
		msg: fmt.Sprintf("undefined label %q", name),
	}
}

// platformErr builds the diagnostic for a.ELF()/a.COFF()/a.Flat() called
// against the wrong target.
func platformErr(have, want string) *Error {
	return &Error{
		Err: ErrPlatform,
		msg: fmt.Sprintf("target is %s; this call needs %s", have, want),
	}
}