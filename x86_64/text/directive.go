// x86_64/text/directive.go
package text

import (
	"errors"
	"fmt"
	"strings"
)

// Kind is a directive's meaning, which is the arch's. The spelling is the
// dialect's: `.globl` and `global` are one Kind, `.quad` and `dq` are
// another, and a file read in either prints back in either.
//
// Absent on purpose: .if, .rept, .macro and everything like them. Those are
// a language, and a language needs an evaluator and an expander that this
// tree deliberately does not have. A parser meeting one produces ErrMacro,
// which names the reason rather than the token.
type Kind uint8

const (
	Unknown Kind = iota

	// Sections and symbols.
	Section
	Global
	Local
	Weak
	Extern
	Hidden
	Type
	Size
	Comm
	LComm
	Equ

	// Data.
	Byte
	Word
	Long
	Quad
	Ascii
	Asciz

	// Space.
	Align
	P2Align
	Fill
	Zero
	Org
)

var kindNames = [...]string{
	Unknown: "unknown",
	Section: "section", Global: "global", Local: "local", Weak: "weak",
	Extern: "extern", Hidden: "hidden", Type: "type", Size: "size",
	Comm: "comm", LComm: "lcomm", Equ: "equ",
	Byte: "byte", Word: "word", Long: "long", Quad: "quad",
	Ascii: "ascii", Asciz: "asciz",
	Align: "align", P2Align: "p2align", Fill: "fill", Zero: "zero", Org: "org",
}

func (k Kind) String() string {
	if int(k) >= len(kindNames) {
		return "?"
	}
	return kindNames[k]
}

// ErrMacro is a conditional-assembly or macro directive. It is not a gap:
// `arc` is not a macro expander, Go is the macro language, and a tree that
// could hold a .rept would need an expander to mean anything.
var ErrMacro = errors.New("macros and conditional assembly are not part of this assembler")

// Directive is one directive as written.
type Directive struct {
	Position Pos
	Kind     Kind

	// Raw is the directive as spelled, for a diagnostic that has to quote
	// it back. It is never read for meaning.
	Raw string

	// Args are the arguments, as expressions. A section name, a symbol
	// name and a type are all *Sym; a count and an alignment are
	// expressions that must reduce to constants.
	Args []Expr

	// Str is the string argument of .ascii, .asciz and .section's flags.
	// Strings are not expressions and pretending they were would put
	// escape-sequence handling in the evaluator.
	Str string

	Comment string
}

func (d *Directive) Pos() Pos { return d.Position }
func (*Directive) node()      {}

// DataWidth is the element width of a data directive in bytes, or zero if
// this is not one.
//
// The names are gas's and the widths are the architecture's, which is why
// Word is two bytes here and sixteen bits everywhere: `.word` predates the
// machine being 64-bit and neither assembler renamed it.
func (d *Directive) DataWidth() int {
	switch d.Kind {
	case Byte:
		return 1
	case Word:
		return 2
	case Long:
		return 4
	case Quad:
		return 8
	}
	return 0
}

// IsData reports whether the directive emits bytes.
func (d *Directive) IsData() bool {
	switch d.Kind {
	case Byte, Word, Long, Quad, Ascii, Asciz, Fill, Zero:
		return true
	}
	return false
}

// SectionName is the section a Section directive enters.
func (d *Directive) SectionName() string {
	if d.Kind != Section || len(d.Args) == 0 {
		return ""
	}
	if s, ok := d.Args[0].(*Sym); ok {
		return s.Name
	}
	return ""
}

// Symbols is every symbol name the directive names.
func (d *Directive) Symbols() []string {
	var out []string
	for _, a := range d.Args {
		if s, ok := a.(*Sym); ok {
			out = append(out, s.Name)
		}
	}
	if d.Kind == Section && len(out) > 0 {
		return out[1:] // the first argument is the section, not a symbol
	}
	return out
}

// Values reduces every argument, for a directive whose arguments are data.
//
// A value that is not constant comes back as a residue rather than an
// error: `.quad . - msg` is legal assembly and needs a fixup the same way an
// operand does. What is missing today is the backpatch that consumes the
// residue, not this analysis — so a caller that cannot yet handle a
// non-constant checks IsConst and refuses with a specific message rather
// than writing a wrong eight bytes.
func (d *Directive) Values(env Env) ([]Value, error) {
	out := make([]Value, 0, len(d.Args))
	for _, a := range d.Args {
		v, err := Reduce(a, env)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Const reduces argument n to a constant, which is what an alignment, a
// count or a fill size has to be. There is no relocation that would let a
// linker decide how many bytes a statement occupies.
func (d *Directive) Const(env Env, n int) (int64, error) {
	if n >= len(d.Args) {
		return 0, Errorf(d.Position, ".%s takes at least %d arguments", d.Kind, n+1)
	}
	v, err := Eval(d.Args[n], env)
	if err != nil {
		if errors.Is(err, ErrNotConstant) {
			return 0, Errorf(d.Position,
				".%s needs a constant: the size of a statement cannot depend on a symbol", d.Kind)
		}
		return 0, err
	}
	return v, nil
}

// Alignment is the byte alignment a Align or P2Align directive asks for.
//
// gas has both `.align` and `.p2align` and they disagree about what the
// number means — on this target `.align` is a byte count and `.p2align` is
// an exponent. Normalizing to bytes here means the printers convert and
// nothing downstream has to remember which it was reading.
func (d *Directive) Alignment(env Env) (int64, error) {
	n, err := d.Const(env, 0)
	if err != nil {
		return 0, err
	}
	switch d.Kind {
	case Align:
		if n <= 0 || n&(n-1) != 0 {
			return 0, Errorf(d.Position, "alignment must be a power of two (got %d)", n)
		}
		return n, nil
	case P2Align:
		if n < 0 || n > 31 {
			return 0, Errorf(d.Position, "p2align exponent out of range (got %d)", n)
		}
		return 1 << uint(n), nil
	}
	return 0, Errorf(d.Position, "%s is not an alignment directive", d.Kind)
}

// SymbolType is what a .type directive says a symbol is.
type SymbolType uint8

const (
	TypeNone SymbolType = iota
	TypeFunc
	TypeObject
	TypeTLS
)

func (t SymbolType) String() string {
	switch t {
	case TypeFunc:
		return "function"
	case TypeObject:
		return "object"
	case TypeTLS:
		return "tls_object"
	}
	return "notype"
}

// ParseSymbolType resolves the spellings both dialects accept. gas writes
// `@function` and `%function` depending on the target's comment character,
// and NASM writes `function`; all three are one thing.
func ParseSymbolType(s string) (SymbolType, error) {
	switch strings.TrimLeft(strings.ToLower(s), "@%#") {
	case "function", "func":
		return TypeFunc, nil
	case "object":
		return TypeObject, nil
	case "tls_object":
		return TypeTLS, nil
	case "notype":
		return TypeNone, nil
	}
	return TypeNone, fmt.Errorf("unknown symbol type %q", s)
}

// String is a diagnostic rendering.
func (d *Directive) String() string {
	var b strings.Builder
	b.WriteString("." + d.Kind.String())
	for n, a := range d.Args {
		if n == 0 {
			b.WriteString(" ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(exprString(a))
	}
	if d.Str != "" {
		if len(d.Args) > 0 {
			b.WriteString(", ")
		} else {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%q", d.Str)
	}
	return b.String()
}