// x86_64/text/unit.go
package text

import (
	"fmt"
	"strings"
)

// Unit is one assembly file as a tree.
//
// Both dialects parse to this and both print from it, which is what makes
// the round trip a property of the code rather than a claim in a README.
// Nothing in a Unit records which dialect it came from except Dialect
// itself, and nothing reads Dialect except a printer told to keep the input
// dialect.
type Unit struct {
	// Name is the source file's name, for diagnostics.
	Name string

	// Dialect is the syntax this unit was parsed from, so `arc fmt` without
	// --dialect can print it back the way it came. It is metadata and never
	// changes what the unit means.
	Dialect Dialect

	Nodes []Node
}

// Dialect is a spelling, never a byte.
type Dialect uint8

const (
	// DialectNone is a unit built programmatically, which has no origin
	// syntax. Printing one requires naming a dialect.
	DialectNone Dialect = iota
	GAS
	NASM
)

func (d Dialect) String() string {
	switch d {
	case GAS:
		return "gas"
	case NASM:
		return "nasm"
	}
	return "none"
}

// Add appends a node.
func (u *Unit) Add(n Node) { u.Nodes = append(u.Nodes, n) }

// Insts is every instruction in source order.
func (u *Unit) Insts() []*Inst {
	var out []*Inst
	for _, n := range u.Nodes {
		if i, ok := n.(*Inst); ok {
			out = append(out, i)
		}
	}
	return out
}

// Sections is every section named by a directive, in the order they are
// first entered. Section order is creation order all the way down to
// objectfile/, so this is the order they will come out in.
func (u *Unit) Sections() []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range u.Nodes {
		d, ok := n.(*Directive)
		if !ok || d.Kind != Section {
			continue
		}
		name := d.SectionName()
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// Defined is every label defined in this unit, excluding gas's numeric
// labels, which are not symbols.
func (u *Unit) Defined() []string {
	var out []string
	for _, n := range u.Nodes {
		if l, ok := n.(*Label); ok && !l.Numeric {
			out = append(out, l.Name)
		}
	}
	return out
}

// Validate is the checking that is the unit's rather than any statement's:
// a label defined twice, a directive naming a symbol nothing defines and
// nothing declares external, a size or type attached to nothing.
//
// It is not the assembler's checking. Whether an instruction encodes is
// isa/ and encode/'s question and is not asked here, because a unit that
// fails to encode under one feature set may encode under another and the
// tree does not know which one it will meet.
func (u *Unit) Validate() []error {
	var errs []error

	defined := map[string]Pos{}
	for _, n := range u.Nodes {
		l, ok := n.(*Label)
		if !ok || l.Numeric {
			continue
		}
		if prev, dup := defined[l.Name]; dup {
			errs = append(errs, Errorf(l.Position,
				"%s redefined (first defined at %s)", l.Name, prev))
			continue
		}
		defined[l.Name] = l.Position
	}

	declared := map[string]bool{}
	for _, n := range u.Nodes {
		d, ok := n.(*Directive)
		if !ok {
			continue
		}
		switch d.Kind {
		case Extern, Comm, LComm:
			for _, s := range d.Symbols() {
				declared[s] = true
			}
		case Equ:
			if len(d.Args) > 0 {
				if s, ok := d.Args[0].(*Sym); ok {
					declared[s.Name] = true
				}
			}
		}
	}

	for _, n := range u.Nodes {
		d, ok := n.(*Directive)
		if !ok {
			continue
		}
		// .type and .size describe a symbol this file defines. Attaching
		// either to a name that is not defined here is a statement about
		// somebody else's symbol, which the object format has no way to
		// record.
		if d.Kind == Type || d.Kind == Size {
			for _, s := range d.Symbols() {
				if !defined[s].IsValid() && !declared[s] {
					errs = append(errs, Errorf(d.Position,
						"%s names %s, which this file does not define", d.Kind, s))
				}
			}
		}
	}
	return errs
}

// String is a diagnostic rendering, not a printer.
//
// The printers live in text/gas and text/nasm and produce assemblable
// source; this produces something readable in a test failure. Two renderings
// is one more than a package should have, so this one is deliberately not
// valid input to either parser.
func (u *Unit) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s (%s), %d nodes\n", u.Name, u.Dialect, len(u.Nodes))
	for _, n := range u.Nodes {
		switch v := n.(type) {
		case *Label:
			fmt.Fprintf(&b, "label   %s\n", v.Name)
		case *Inst:
			fmt.Fprintf(&b, "inst    %s\n", v)
		case *Directive:
			fmt.Fprintf(&b, "dir     %s\n", v)
		case *Comment:
			fmt.Fprintf(&b, "comment %s\n", v.Text)
		case *Blank:
			fmt.Fprintf(&b, "blank   %d\n", v.Lines)
		}
	}
	return b.String()
}