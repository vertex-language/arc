package text

// Node is one statement of a source file.
//
// The set is closed and small: a label, an instruction, a directive. There is
// no expression node and no block node, because assembly has neither — an
// expression is an operand's or a directive's argument and never a statement,
// and there is nothing that nests.
//
// A comment is not a node. It is trivia attached to the statement it precedes
// or follows, kept on the node rather than between nodes, so that a pass that
// reorders or drops statements cannot orphan one or silently move it onto
// something it was not about.
type Node interface {
	Pos() Pos
	node()
}

// Label is a name defined at the current position in the current section.
type Label struct {
	Name string
	P    Pos

	// Numeric marks gas's local numeric labels: 1:, 2:, referred to as 1f and
	// 1b. They are position references rather than symbols — the same name is
	// legitimately defined many times in one file — so they never reach a
	// symbol table and Unit.Defined excludes them.
	Numeric bool

	// Comment is trailing trivia: the text after the statement on its line.
	Comment string
}

func (l *Label) Pos() Pos { return l.P }
func (*Label) node()      {}

// Comment is a line that is only a comment. It is a node so that reprinting a
// file preserves the blank-line and comment structure a reader put there;
// nothing else looks at it.
type Comment struct {
	Text string
	P    Pos

	// Blank marks an empty line rather than a comment, which the printer
	// reproduces and everything else ignores.
	Blank bool
}

func (c *Comment) Pos() Pos { return c.P }
func (*Comment) node()      {}

// Nodes is a statement list, with the small queries a pass wants.
type Nodes []Node

// Insts returns every instruction node.
func (ns Nodes) Insts() []*Inst {
	var out []*Inst
	for _, n := range ns {
		if in, ok := n.(*Inst); ok {
			out = append(out, in)
		}
	}
	return out
}

// Directives returns every directive node of a kind, or all of them when kind
// is DirNone.
func (ns Nodes) Directives(kind DirKind) []*Directive {
	var out []*Directive
	for _, n := range ns {
		d, ok := n.(*Directive)
		if !ok {
			continue
		}
		if kind == DirNone || d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}