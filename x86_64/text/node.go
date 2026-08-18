// x86_64/text/node.go
package text

// Node is one statement of a unit: a label, an instruction, a directive, a
// comment, or a blank line.
//
// Comments and blank lines are nodes because `arc fmt` rewrites files people
// have to keep reading. A formatter that dropped them would be a formatter
// nobody ran twice, and the round-trip guarantee is about bytes assembled,
// not about bytes discarded.
type Node interface {
	Pos() Pos
	node()
}

// Label is a label definition: a name and a colon.
//
// Whether it becomes a symbol in the object file, and with what binding, is
// not settled here — a bare label is local until a .globl says otherwise,
// and the .globl may come after it. That reconciliation is the assembler's.
type Label struct {
	Position Pos
	Name     string

	// Local marks a label the syntax says is local to the file: gas's
	// .L-prefixed names and nasm's .-prefixed ones. It is a fact about the
	// name rather than about the directive that may or may not follow.
	Local bool

	// Numeric marks gas's numeric labels — the `1:` that `1b` and `1f`
	// refer to. They may be redefined, so they are not symbols and never
	// reach the object file.
	Numeric bool

	Comment string
}

func (l *Label) Pos() Pos { return l.Position }
func (*Label) node()      {}

// Comment is a whole-line comment. A trailing comment is a field on the node
// it trails, not a node of its own.
type Comment struct {
	Position Pos
	Text     string
}

func (c *Comment) Pos() Pos { return c.Position }
func (*Comment) node()      {}

// Blank is one or more empty lines. It carries a count so a formatter can
// collapse runs without deciding, here, how many is too many — that is a
// style question and this package has no opinions about style.
type Blank struct {
	Position Pos
	Lines    int
}

func (b *Blank) Pos() Pos { return b.Position }
func (*Blank) node()      {}

// Compile-time assurance that every node kind in this package satisfies the
// interface, so adding one and forgetting the method is a build failure.
var (
	_ Node = (*Label)(nil)
	_ Node = (*Comment)(nil)
	_ Node = (*Blank)(nil)
	_ Node = (*Inst)(nil)
	_ Node = (*Directive)(nil)
)