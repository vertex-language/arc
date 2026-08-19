// x86_64/text.go
//
// The text layer's entry points: parsing a source file into a
// dialect-neutral tree, and printing one back out.
//
// This is the only file in the tree that imports both text/gas and
// text/nasm. Neither imports the other, neither imports this package, and
// the dispatch below is the whole of what joins them — which is what makes
// "a dialect is a spelling, never a byte" checkable rather than aspirational.
package x86_64

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/text"
	"github.com/vertex-language/arc/x86_64/text/gas"
	"github.com/vertex-language/arc/x86_64/text/nasm"
)

// Unit is one assembly file as a tree. Both dialects parse to it and both
// print from it.
type Unit = text.Unit

// TextInst is one instruction as written, before a form is resolved.
//
// The name is not Inst, because decode.Inst is already Inst here and the
// two are different things: one is what a source says and one is what bytes
// mean. A single name for both would be a name that answers "does this have
// a Form yet" differently depending on where it came from.
type TextInst = text.Inst

// ParseFile reads a source file into a tree.
func ParseFile(name string, src []byte, d Dialect) (*Unit, error) {
	switch d {
	case GAS:
		u, err := gas.Parse(name, src)
		return u, wrapText(err)
	case NASM:
		u, err := nasm.Parse(name, src)
		return u, wrapText(err)
	}
	return nil, fmt.Errorf("%w: %s", ErrForm, dialectError(d))
}

// ParseInst reads a single instruction, for `arc enc`.
func ParseInst(line string, d Dialect) (*TextInst, error) {
	switch d {
	case GAS:
		i, err := gas.ParseInst(line)
		return i, wrapText(err)
	case NASM:
		i, err := nasm.ParseInst(line)
		return i, wrapText(err)
	}
	return nil, fmt.Errorf("%w: %s", ErrForm, dialectError(d))
}

// Print renders a unit as source.
//
// Passing DialectNone prints the unit in the dialect it was parsed from,
// which is what `arc fmt` without --dialect does. A unit built
// programmatically has no origin dialect and one must be named.
func Print(u *Unit, d Dialect) ([]byte, error) {
	if d == text.DialectNone {
		d = u.Dialect
	}
	switch d {
	case GAS:
		b, err := gas.Print(u)
		return b, wrapText(err)
	case NASM:
		b, err := nasm.Print(u)
		return b, wrapText(err)
	}
	return nil, fmt.Errorf("%w: %s", ErrForm, dialectError(d))
}

// PrintInst renders one instruction.
//
// A printer may need the resolved form: gas writes an operand size as a
// mnemonic suffix and NASM as an operand keyword, and going from one to the
// other means knowing a width that the source may not have stated. Resolve
// the instruction first — Assemble does, and so does `arc fmt` — or accept
// that an instruction whose operands do not settle the size prints without
// one.
func PrintInst(i *TextInst, d Dialect) (string, error) {
	switch d {
	case GAS:
		s, err := gas.PrintInst(i)
		return s, wrapText(err)
	case NASM:
		s, err := nasm.PrintInst(i)
		return s, wrapText(err)
	}
	return "", fmt.Errorf("%w: %s", ErrForm, dialectError(d))
}

// Format parses and reprints in one call, which is `arc fmt`'s whole job
// when no resolution is needed.
//
// Everything it changes assembles to identical bytes. What it changes is
// whitespace, the parentheses the target dialect's precedence requires, and
// the canonical spelling of things that have one — a number's base prefix,
// a condition-code alias, an @-modifier become `wrt`.
func Format(name string, src []byte, from, to Dialect) ([]byte, error) {
	u, err := ParseFile(name, src, from)
	if err != nil {
		return nil, err
	}
	if errs := u.Validate(); len(errs) > 0 {
		return nil, errs[0]
	}
	return Print(u, to)
}

// ---- resolution --------------------------------------------------------

// Resolved is a unit whose instructions have been matched to forms.
//
// Printing across dialects needs this and printing within one does not,
// which is why it is a separate step rather than part of Print: resolving
// requires a feature set, and a feature set is a thing the caller has and
// the tree does not.
type Resolved struct {
	Unit     *Unit
	Features FeatureSet
}

// ResolveUnit attaches a form to every instruction in the unit.
//
// This is what `arc fmt --dialect` runs before printing. The tree is
// mutated in place — Inst.Form is documented as "cached by whoever resolved
// it" and this is that — so a caller who wants the unresolved tree back
// should parse it again.
//
// An instruction whose operands do not resolve is an error naming the line.
// That is stricter than a formatter has to be, and it is the right
// strictness for a translator: an instruction nobody can encode is one
// whose operand size nobody can recover, and printing it into the other
// dialect would produce a line that means something else or nothing.
func ResolveUnit(u *Unit, f FeatureSet) (*Resolved, error) {
	for _, i := range u.Insts() {
		if i.Form != nil {
			continue
		}
		ops, err := i.Lower(nil)
		if err != nil {
			return nil, atPos(i.Position, f, err)
		}
		args, err := encode.Args(ops...)
		if err != nil {
			return nil, atPos(i.Position, f, err)
		}
		form, err := isa.Resolve(f, i.Mnemonic, args...)
		if err != nil {
			return nil, atPos(i.Position, f, err)
		}
		i.Form = form
	}
	return &Resolved{Unit: u, Features: f}, nil
}

// Print renders a resolved unit, with every operand size recoverable.
func (r *Resolved) Print(d Dialect) ([]byte, error) { return Print(r.Unit, d) }

// Translate is Format with a resolution step in the middle: parse, resolve,
// print.
//
// This is the call that makes `arc fmt --dialect nasm` byte-identical
// rather than approximately right. A text-level translator cannot do it:
// going from NASM's `mov qword [rbx], 1` to gas's `movq $1, (%rbx)` means
// knowing the width, and the only thing that knows it is the form the
// encoder resolved.
func Translate(name string, src []byte, from, to Dialect, f FeatureSet) ([]byte, error) {
	u, err := ParseFile(name, src, from)
	if err != nil {
		return nil, err
	}
	if errs := u.Validate(); len(errs) > 0 {
		return nil, errs[0]
	}
	r, err := ResolveUnit(u, f)
	if err != nil {
		return nil, err
	}
	return r.Print(to)
}

// ---- helpers -----------------------------------------------------------

// wrapText attaches this package's error vocabulary to a text/ diagnostic,
// which already carries a position.
func wrapText(err error) error {
	if err == nil {
		return nil
	}
	var te *text.Error
	if errors.As(err, &te) {
		return &Error{Pos: te.Pos, Err: te.Err, Note: ""}
	}
	return &Error{Err: err}
}

// dialectError names what was asked for, since a zero Dialect is the
// commonest way to reach one of these and prints as "none".
func dialectError(d Dialect) string {
	if d == text.DialectNone {
		return "no dialect named; a unit built programmatically has no origin syntax to print back"
	}
	return fmt.Errorf("no dialect %d", uint8(d)).Error()
}