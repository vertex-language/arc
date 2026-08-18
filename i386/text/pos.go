// Package text is the i386 assembly-language layer: what a .s file denotes,
// independent of which syntax it was written in.
//
// Both dialects parse *to* the Unit this package declares and print *from*
// it, which is why it lives here and not in either subdirectory. arc build is
// gas.ParseFile then Assemble; arc fmt is gas.ParseFile then nasm.Print. That
// the two paths meet in one type is what makes the round trip a property of
// the code rather than a claim in a README.
//
// This package holds no grammar. It has no lexer, no precedence table, and no
// spelling of any directive — those are per-dialect and live in gas/ and
// nasm/. What it holds is everything the two dialects agree on: the node
// kinds, the expression tree, the arithmetic over that tree, and the
// directive semantics that are the arch's rather than the syntax's.
//
// It imports i386/reg and nothing else from the arch. A register is the one
// operand fully determined by its spelling, so it can be resolved at parse
// time; everything else — which form, which encoding, which relocation
// number — is resolved above, because resolving it here would need isa/ and a
// platform, and a parsed line has neither.
package text

import (
	"fmt"
	"sort"
	"strings"
)

// Pos is a position in a source file. Columns and lines are 1-based, and a
// column counts bytes rather than runes: this is the number an editor jumping
// to file:line:col wants, and the number GNU as prints.
type Pos struct {
	File string
	Line int
	Col  int
}

// IsValid reports whether the position names a line.
func (p Pos) IsValid() bool { return p.Line > 0 }

// String is the file:line:col prefix every diagnostic in arc starts with —
// the format every editor already parses.
func (p Pos) String() string {
	switch {
	case !p.IsValid():
		return "<input>"
	case p.Col > 0:
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d", p.File, p.Line)
}

// Error is one diagnostic, positioned.
//
// The level is not stored. Everything this package produces is an error;
// warnings would be a second severity nothing in arc emits, and a field with
// one value is a field that will grow one wrong one.
type Error struct {
	Pos   Pos
	Msg   string
	Notes []string
}

// Error renders file:line:col: error: message, with notes on following lines.
// A note names the flag or the spelling that would fix it.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Pos.String())
	b.WriteString(": error: ")
	b.WriteString(e.Msg)
	for _, n := range e.Notes {
		b.WriteString("\n  note: ")
		b.WriteString(n)
	}
	return b.String()
}

// Errorf builds a diagnostic at p.
func Errorf(p Pos, format string, args ...any) *Error {
	return &Error{Pos: p, Msg: fmt.Sprintf(format, args...)}
}

// Note appends a note and returns e, so a diagnostic reads as one expression.
func (e *Error) Note(format string, args ...any) *Error {
	e.Notes = append(e.Notes, fmt.Sprintf(format, args...))
	return e
}

// ErrorList is the errors from one parse, in source order.
//
// A parser reports as many as it can rather than stopping at the first,
// because a file with two typos should take one run to fix and not two. The
// list is sorted by position before it is returned, since a recovering parser
// may find a later error first.
type ErrorList []*Error

func (l ErrorList) Len() int      { return len(l) }
func (l ErrorList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func (l ErrorList) Less(i, j int) bool {
	a, b := l[i].Pos, l[j].Pos
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

// Sort orders the list by position.
func (l ErrorList) Sort() { sort.Sort(l) }

// Add appends err.
func (l *ErrorList) Add(err *Error) { *l = append(*l, err) }

// Err returns the list as an error, or nil when it is empty. The nil-interface
// trap is why this exists: a nil ErrorList is not a nil error.
func (l ErrorList) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

// Error renders every diagnostic, one per line.
func (l ErrorList) Error() string {
	switch len(l) {
	case 0:
		return "no errors"
	case 1:
		return l[0].Error()
	}
	var b strings.Builder
	for i, e := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Error())
	}
	return b.String()
}