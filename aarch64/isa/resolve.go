package isa

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/aarch64/feature"
)

// UnknownError is a mnemonic no form declares.
type UnknownError struct{ Mnem string }

func (e *UnknownError) Error() string {
	return fmt.Sprintf("unknown instruction %q", e.Mnem)
}

// FormError is a mnemonic that exists with no form accepting these operands.
type FormError struct {
	Mnem string
	Args []Arg
	// Near lists the forms that were closest, for the diagnostic.
	Near []*Form
}

func (e *FormError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "no form of %s takes ", e.Mnem)
	if len(e.Args) == 0 {
		b.WriteString("no operands")
	} else {
		for i, a := range e.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.Class.String())
		}
	}
	if len(e.Near) > 0 {
		b.WriteString("\n  candidates:")
		for _, f := range e.Near {
			b.WriteString("\n    ")
			b.WriteString(f.Signature())
		}
	}
	return b.String()
}

// GateError is a form that matches but that the active feature set holds back.
type GateError struct {
	Form   *Form
	Active feature.Set
}

func (e *GateError) Error() string {
	return fmt.Sprintf("%s requires %s, not in the active feature set\n  active: %s\n  note: aarch64.WithFeatures(%s)",
		e.Form.Mnem, e.Form.Gate, e.Active,
		e.Active.Plus(e.Form.Gate).GoExpr())
}

// Resolve finds the one form of a mnemonic that accepts these operands.
//
// There is no shortest-form search and no preference order. Every A64
// instruction is one word, so two forms of a mnemonic accepting the same
// operand classes would be an ambiguity with no tiebreak that means anything —
// and the table refuses to build if two such forms exist, so this function
// never has to choose.
//
// A form that matches but is gated returns a GateError rather than falling
// through to a FormError. Being told an instruction does not exist when it
// exists and is disabled sends a reader looking for a typo that is not there.
func Resolve(mnem string, args []Arg, set feature.Set) (*Form, error) {
	mnem = strings.ToLower(mnem)
	forms := byMnem[mnem]
	if len(forms) == 0 {
		return nil, &UnknownError{Mnem: mnem}
	}

	var gated []*Form
	var near []*Form

	for _, f := range forms {
		if !arityFits(f, len(args)) {
			continue
		}
		if !slotsMatch(f, args) {
			near = append(near, f)
			continue
		}
		if !f.Enabled(set) {
			gated = append(gated, f)
			continue
		}
		return f, nil
	}

	if len(gated) > 0 {
		return nil, &GateError{Form: gated[0], Active: set}
	}
	if len(near) == 0 {
		near = forms
	}
	if len(near) > 4 {
		near = near[:4]
	}
	return nil, &FormError{Mnem: mnem, Args: args, Near: near}
}

func arityFits(f *Form, n int) bool {
	return n >= f.Required() && n <= f.Arity()
}

func slotsMatch(f *Form, args []Arg) bool {
	for i, a := range args {
		if !f.Slots[i].Class.Match(a) {
			return false
		}
	}
	return true
}

// ResolveWord finds the form a word decodes to. It is the read side decode/
// builds its table from; the four-level lookup lives there, and this is the
// linear fallback the table generator checks itself against.
func ResolveWord(word uint32) (*Form, bool) {
	for _, f := range all {
		if f.Attrs&AttrAlias != 0 {
			continue
		}
		if word&f.Mask == f.Word {
			return f, true
		}
	}
	return nil, false
}