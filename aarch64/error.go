package aarch64

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/aarch64/decode"
	"github.com/vertex-language/arc/aarch64/encode"
	"github.com/vertex-language/arc/aarch64/isa"
)

// The five error categories.
//
// They exist for errors.Is, so a caller can ask "was this a feature gate?"
// without matching on a concrete type from a subpackage it does not import.
//
// A category is derived from the wrapped error rather than stored on it. That
// is the whole design: a new error type added to isa/ or encode/ classifies
// correctly the moment it is named in the one switch below, and until then it
// classifies as nothing rather than as something wrong. A stored category could
// disagree with the error it sits on, and nothing would catch it.
var (
	// ErrFeature is a form that exists and matches, held back by the active
	// feature set.
	ErrFeature = errors.New("feature not enabled")

	// ErrForm is no encoding for these operands, or an operand with no
	// encoding at all.
	ErrForm = errors.New("no such form")

	// ErrReloc is a relocation kind with no mapping in this object format.
	ErrReloc = errors.New("relocation not mapped")

	// ErrUndefined is a fixup pointing at a name nothing defines.
	ErrUndefined = errors.New("undefined symbol")

	// ErrPlatform is something this format cannot express.
	ErrPlatform = errors.New("platform limitation")
)

// Error is a failure with the place it happened.
//
// Builder calls return nothing, so this is what Serialize hands back: the
// section and offset are the ones the failing statement would have been
// written at, which is the same place a per-call error would have named.
type Error struct {
	// Section is the section being written, or "" for a failure with no
	// section — a target-model or option error.
	Section string

	// Offset is the byte offset within that section.
	Offset int

	// Err is the underlying failure from isa/, encode/, or a writer.
	Err error
}

func (e *Error) Error() string {
	if e.Section == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s+%#x: %v", e.Section, e.Offset, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Is reports the category, computed rather than stored.
//
// Unwrap already reaches the concrete error, so this only has to answer for the
// five sentinels; anything else falls through to the wrapped error's own
// answer.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrFeature, ErrForm, ErrReloc, ErrUndefined, ErrPlatform:
		return Category(e.Err) == target
	}
	return false
}

// Category classifies an error into one of the five, or nil.
//
// This is the one switch. Everything that reports a category reads it from
// here, which is what keeps two answers from existing.
func Category(err error) error {
	if err == nil {
		return nil
	}

	// Unwrap first, so a wrapped error classifies the same as a bare one.
	var e *Error
	if errors.As(err, &e) {
		err = e.Err
	}

	switch err.(type) {
	// The gate is the one isa/ failure that is not about the operands: the
	// form exists and matches and the feature set held it back. Reporting it
	// as ErrForm would tell a reader the instruction does not exist when it
	// exists and is disabled, which sends them looking for a typo.
	case *isa.GateError:
		return ErrFeature

	case *isa.UnknownError, *isa.FormError:
		return ErrForm

	case *encode.CountError, *encode.OperandError, *encode.RegisterError,
		*encode.RangeError, *encode.BitmaskError, *encode.AddressError:
		return ErrForm

	// An encoding shape the architecture has and this encoder does not reach
	// yet is a form failure rather than a platform one: the format could hold
	// it, and the gap is here.
	case *encode.UnsupportedError:
		return ErrForm

	case *decode.UnknownError, *decode.ClassError:
		return ErrForm

	case *RelocError:
		return ErrReloc
	case *UndefinedError:
		return ErrUndefined
	case *PlatformError:
		return ErrPlatform
	}

	switch {
	case errors.Is(err, decode.ErrTruncated), errors.Is(err, decode.ErrUnaligned):
		return ErrForm
	}
	return nil
}

// RelocError is a relocation kind declared here with no mapping in objectfile/.
//
// It names the gap rather than saying "unknown relocation", which would send a
// reader looking for a spelling error that is not there. The kind is real, this
// package declares it, and the mapping below is what is missing.
type RelocError struct {
	Kind     RelocKind
	Platform Platform
	Symbol   string
}

func (e *RelocError) Error() string {
	s := fmt.Sprintf("%s has no mapping in the %s writer", RelocName(e.Kind), e.Platform)
	if e.Symbol != "" {
		s += " (reference to " + e.Symbol + ")"
	}
	return s
}

// UndefinedError is a fixup naming a symbol nothing in this object defines and
// nothing declares.
type UndefinedError struct {
	Symbol string
}

func (e *UndefinedError) Error() string {
	return fmt.Sprintf("%s is neither defined nor declared; "+
		"declare it with Declare if it is external", e.Symbol)
}

// PlatformError is something the chosen format cannot express.
//
// The message carries the reason rather than the fact, because the fixes
// differ: a flat image refusing a cross-section reference and a Mach-O object
// refusing a nonzero addend are both this, and knowing which one it is is the
// whole content of the diagnostic.
type PlatformError struct {
	Platform Platform
	What     string
	Why      string
}

func (e *PlatformError) Error() string {
	s := fmt.Sprintf("%s: %s", e.Platform, e.What)
	if e.Why != "" {
		s += ": " + e.Why
	}
	return s
}

// wrap attaches a section and offset to an error from below.
func wrap(section string, offset int, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err // already placed; the innermost placement is the true one
	}
	return &Error{Section: section, Offset: offset, Err: err}
}