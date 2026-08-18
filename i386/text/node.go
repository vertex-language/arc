package text

// Node is one statement.
//
// The set is closed and small, and everything in it is something both
// dialects can write. What is absent is as deliberate as what is present:
//
//   - No macro, .rept, .irp, %macro or %rep. Go is the macro language.
//   - No .if, .else or %if. Conditional assembly is a language feature and
//     languages are out; it is also the reason MASM and HLASM have no
//     directory.
//   - No .include or incbin. arc is not a compiler driver and does not open
//     files the command line did not name.
//   - No .float, .double or dt. The builder API declares seven data calls and
//     none of them is a float, so a float literal would be a thing the text
//     layer could say and the Go API could not.
//   - No TIMES over an instruction. `times 510-($-$$) db 0` is a Fill and
//     round-trips as .fill; `times 4 nop` is a repeat of a statement, which
//     needs the expander this tree does not have.
type Node interface {
	Pos() Pos
	Trivia() *Trivia
	node()
}

// Trivia is everything on a line that is not the statement: the comments
// around it and the blank lines before it.
//
// It is kept because arc fmt rewrites files in place, and a formatter that
// dropped comments would be a formatter nobody could run. The round-trip
// guarantee is about bytes, but the useful version of it is about files.
//
// Comment text is stored without the introducer — '#' in GAS, ';' in NASM —
// because the introducer is a spelling and each printer supplies its own.
type Trivia struct {
	// Blanks is the number of blank lines before the statement, capped by the
	// printer so a file cannot accumulate whitespace across formattings.
	Blanks int

	// Before are whole-line comments immediately above the statement.
	Before []string

	// Line is a trailing comment on the statement's own line.
	Line string

	// HasLine distinguishes an empty trailing comment from none at all, since
	// `nop  #` is a line a formatter should not silently change.
	HasLine bool
}

// Common is the position and trivia every node carries.
type Common struct {
	P Pos
	T Trivia
}

func (c *Common) Pos() Pos        { return c.P }
func (c *Common) Trivia() *Trivia { return &c.T }

// Label is a name for the current offset.
//
// Whether it becomes a symbol is a SymbolDecl's business, not this node's: a
// bare label is a fixup target resolvable within one section and present in
// no symbol table, exactly as the builder API's bare Label is. Attaching an
// attribute is what makes it a symbol, and in text that attribute is a
// separate statement.
type Label struct {
	Common
	Name string

	// Attached records that the next statement was written on this label's
	// own line — `msg: .ascii "hi"`. Both dialects allow it and a formatter
	// that moved it would be reflowing code it was asked to normalise.
	Attached bool

	// Local marks a name the dialect treats as file-local by convention:
	// .L in GAS, a leading . in NASM. It is a naming convention rather than
	// a binding, which is why it does not set AttrLocal.
	Local bool
}

func (*Label) node() {}

// SectionDecl switches the current section.
//
// Kind is the portable meaning and Name is the spelling that was written.
// Both are kept: .text and .section .text are the same section and print
// differently, and a custom section has a name with no kind behind it.
//
// Flags and Type pass through verbatim. arc does not parse "ax" or @progbits,
// because the object layer takes flags as data and inventing a flag the
// source did not write is the sort of thing "arc does not know what Linux is"
// exists to forbid.
type SectionDecl struct {
	Common
	Kind  SectionKind
	Name  string
	Flags string
	Type  string

	// Short records that the source used a shorthand — .text rather than
	// .section .text — so the printer can write back what was written.
	Short bool
}

func (*SectionDecl) node() {}

// SymbolDecl attaches attributes to one or more names.
type SymbolDecl struct {
	Common
	Names []string
	Attrs Attr
	Type  SymbolType

	// Size is the .size expression, when the directive carried one. A .size
	// of `. - name` is the ordinary spelling and evaluates to a Difference,
	// which is why Eval models one.
	Size Expr
}

func (*SymbolDecl) node() {}

// Equ binds a name to an expression: .equ, .set, and NASM's `name equ expr`.
//
// This is not a macro. The right-hand side is an expression over the same
// tree everything else uses, evaluated once, and it cannot expand to a
// statement or to an operand list.
type Equ struct {
	Common
	Name  string
	Value Expr
}

func (*Equ) node() {}

// Data emits initialised bytes: .byte, .word, .long, .quad and their db, dw,
// dd, dq spellings, plus the string forms.
//
// One node covers numbers and strings because NASM's db takes both in one
// list — `db 'hello',13,10,'$'` is one statement — and splitting it would
// make a GAS printer emit three where the source had one.
type Data struct {
	Common
	Width Width
	Items []DataItem
}

func (*Data) node() {}

// DataItem is one element of a Data statement: an expression or a string.
//
// Terminated marks a string the directive terminates with a zero byte —
// .asciz and .string. GAS spells termination in the directive and NASM in the
// data (`db 'hi',0`), so the flag is on the item and each printer spells it
// its own way.
type DataItem struct {
	Pos Pos

	X Expr

	Str        string
	IsStr      bool
	Terminated bool
}

// Fill emits Count copies of a Value that is Size bytes wide.
//
// This is the shape .zero, .space, .skip, .fill, NASM's resb family, and
// `times N db V` all reduce to. They differ in which of the three the source
// stated and each printer picks the shortest spelling that says the same
// thing, which is a normalisation that changes text and not bytes.
//
// Value is nil for a zero fill, which is the common case and the only one
// resb can express.
type Fill struct {
	Common
	Count Expr
	Size  Width
	Value Expr
}

func (*Fill) node() {}

// Align advances to a boundary.
//
// Bytes is a byte count, not a power of two. GAS's .p2align and .balign and
// NASM's align differ in which they take, and the exponent form is converted
// at parse: two spellings of one boundary is a spelling, and a set with one
// meaning does not need two fields.
//
// Value is the fill byte when the source named one. When it did not, the
// arch's nop table fills a code section and zeros fill a data one — that
// table is the assembler's, because padding .text with 0x00 produces a
// listing that disassembles into garbage.
type Align struct {
	Common
	Bytes Expr
	Value Expr
	Max   Expr

	// P2 records that the source wrote the exponent form, so it prints back
	// as it was written.
	P2 bool
}

func (*Align) node() {}