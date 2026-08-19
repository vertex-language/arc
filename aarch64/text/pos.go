// Package text is the tree both a parser produces and a printer consumes, plus
// the expression evaluator and the directive semantics that belong to the
// architecture rather than to a syntax.
//
// There is one syntax on this architecture — A64 as GNU as and the LLVM
// integrated assembler accept it — so this package has one dialect below it
// rather than two. It is still separate from that dialect for the reason the
// x86_64 tree is: what a directive means, what an expression reduces to, and
// what an operand lowers to are the arch's answers, and a printer that had to
// re-derive them from source text could not guarantee that formatting changes
// nothing.
package text

import "strconv"

// Pos is a position in a source file.
//
// Line and Col are 1-based because that is what an editor and every other
// assembler's diagnostics use; Offset is 0-based because it indexes bytes. A
// zero Pos means "not from source" — a node the builder API produced, or one a
// pass synthesized — and IsValid reports that rather than pointing at line one.
type Pos struct {
	File   string
	Line   int
	Col    int
	Offset int
}

// IsValid reports whether the position names a place in a file.
func (p Pos) IsValid() bool { return p.Line > 0 }

// String is the file:line:col a diagnostic prefixes.
func (p Pos) String() string {
	if !p.IsValid() {
		return "<no position>"
	}
	s := p.File
	if s == "" {
		s = "<input>"
	}
	s += ":" + strconv.Itoa(p.Line)
	if p.Col > 0 {
		s += ":" + strconv.Itoa(p.Col)
	}
	return s
}

// Before reports whether p precedes q in the same file. Positions in different
// files are not ordered, which this reports as false in both directions rather
// than picking one.
func (p Pos) Before(q Pos) bool {
	return p.File == q.File && p.IsValid() && q.IsValid() && p.Offset < q.Offset
}