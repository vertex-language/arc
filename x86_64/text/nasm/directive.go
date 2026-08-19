// x86_64/text/nasm/directive.go
package nasm

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/text"
)

// nasmDirectives maps a NASM spelling to the neutral kind.
//
// Absent on purpose: everything beginning with '%'. Those are the
// preprocessor, which is a language rather than a syntax, and a parser that
// accepted %if would need an evaluator that decides which half of a file
// exists. They are recognized only well enough to refuse them by name.
var nasmDirectives = map[string]text.Kind{
	"section": text.Section,
	"segment": text.Section,

	"global": text.Global,
	"extern": text.Extern,
	"common": text.Comm,
	"static": text.Local,

	"db": text.Byte,
	"dw": text.Word,
	"dd": text.Long,
	"dq": text.Quad,

	"resb": text.Zero,
	"resw": text.Zero,
	"resd": text.Zero,
	"resq": text.Zero,

	"align":  text.Align,
	"alignb": text.Align,
	"org":    text.Org,
	"equ":    text.Equ,
}

// resWidths is the element width of each reservation directive, in bytes. A
// reservation is a byte count in the neutral tree, so resq 4 becomes a count
// of 4*8 — the multiplication is built as an expression rather than folded,
// because the count may be symbolic and folding it would need an Env this
// parser has not got.
var resWidths = map[string]int64{
	"resb": 1, "resw": 2, "resd": 4, "resq": 8,
}

// dataWidths is the element width of each data directive.
var dataWidths = map[string]int64{
	"db": 1, "dw": 2, "dd": 4, "dq": 8,
}

// unsupportedDirectives are NASM directives with no neutral meaning, each
// refused with the reason rather than with "unknown directive".
var unsupportedDirectives = map[string]string{
	"dt":  "an 80-bit datum has no width in this tree",
	"do":  "a 128-bit datum has no width in this tree",
	"dy":  "a 256-bit datum has no width in this tree",
	"dz":  "a 512-bit datum has no width in this tree",
	"rest": "an 80-bit reservation has no width in this tree",
	"cpu": "the feature set is --features, not a directive: a set that changed " +
		"halfway through an object would make a gating diagnostic unfalsifiable",
	"default": "`default rel` is encoding state rather than a spelling — it changes " +
		"the bytes of every later memory operand, and the tree carries no mode. " +
		"Write [rel sym] where you mean it",
	"absolute": "absolute sections are a layout feature; address assignment is linker/'s",
	"struc":    "struc is the preprocessor under another name",
	"istruc":   "istruc is the preprocessor under another name",
}

func (p *parser) directive(pos text.Pos, word string) error {
	if why, ok := unsupportedDirectives[word]; ok {
		return text.Errorf(pos, "%s: %s", word, why)
	}

	switch word {
	case "bits":
		return p.bits(pos)
	case "times":
		return p.times(pos)
	case "section", "segment":
		return p.section(pos, word)
	case "global", "extern", "static":
		return p.symbolList(pos, word)
	case "common":
		return p.common(pos)
	}

	if w, ok := resWidths[word]; ok {
		return p.reserve(pos, word, w)
	}
	if _, ok := dataWidths[word]; ok {
		return p.data(pos, word)
	}

	kind, ok := nasmDirectives[word]
	if !ok {
		return text.Errorf(pos, "unknown directive %s", word)
	}

	d := &text.Directive{Position: pos, Kind: kind, Raw: word}
	if err := p.advance(); err != nil {
		return err
	}
	for !p.atEnd() {
		e, err := p.parseExpr(lowestPrec)
		if err != nil {
			return err
		}
		d.Args = append(d.Args, e)
		if !p.isPunct(",") {
			break
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
	p.unit.Add(d)
	return p.endOfStatement(d)
}

// bits states the mode. This package is the 64-bit target's, so `bits 64`
// says what the import path already said and anything else is a different
// arch: `bits 32` is i386, which is a directory of its own.
func (p *parser) bits(pos text.Pos) error {
	if err := p.advance(); err != nil {
		return err
	}
	if p.tok.kind != tNum {
		return text.Errorf(p.tok.pos, "bits takes a mode")
	}
	n := p.tok.num
	if n != 64 {
		return text.Errorf(pos,
			"bits %d is not this target; 32-bit code is the i386 package", n)
	}
	if err := p.advance(); err != nil {
		return err
	}
	return p.endOfStatement(nil)
}

// section reads `section .text` and the flag words that may follow.
//
// The flags are carried as an opaque string because the two dialects spell
// them differently — NASM writes bare words and gas writes a quoted string
// and an @type — and neither vocabulary is the architecture's.
func (p *parser) section(pos text.Pos, raw string) error {
	if err := p.advance(); err != nil {
		return err
	}
	if p.tok.kind != tIdent {
		return text.Errorf(p.tok.pos, "expected a section name")
	}
	d := &text.Directive{Position: pos, Kind: text.Section, Raw: raw}
	d.Args = append(d.Args, &text.Sym{Position: p.tok.pos, Name: p.tok.text})
	if err := p.advance(); err != nil {
		return err
	}

	var flags []string
	for !p.atEnd() {
		if p.tok.kind != tIdent && p.tok.kind != tNum {
			return text.Errorf(p.tok.pos, "expected a section attribute")
		}
		flags = append(flags, p.tok.text)
		if err := p.advance(); err != nil {
			return err
		}
	}
	d.Str = strings.Join(flags, " ")

	p.unit.Add(d)
	return p.endOfStatement(d)
}

// symbolList reads global/extern/static, each of which takes names and, for
// global, an optional `:type` and size expression.
//
// NASM folds the type onto the global line; gas writes a separate .type. The
// neutral tree has the separate form, so this splits what NASM joined —
// which is the same operation the printer runs backwards.
func (p *parser) symbolList(pos text.Pos, word string) error {
	kind := nasmDirectives[word]
	if err := p.advance(); err != nil {
		return err
	}

	for {
		if p.tok.kind != tIdent {
			return text.Errorf(p.tok.pos, "expected a symbol name")
		}
		name := p.tok.text
		npos := p.tok.pos
		if err := p.advance(); err != nil {
			return err
		}

		d := &text.Directive{Position: pos, Kind: kind, Raw: word}
		d.Args = append(d.Args, &text.Sym{Position: npos, Name: name})
		p.unit.Add(d)

		if p.isPunct(":") {
			if err := p.advance(); err != nil {
				return err
			}
			if p.tok.kind != tIdent {
				return text.Errorf(p.tok.pos, "expected a symbol type after :")
			}
			if _, err := text.ParseSymbolType(p.tok.text); err != nil {
				return text.Wrap(p.tok.pos, err)
			}
			t := &text.Directive{Position: pos, Kind: text.Type, Raw: word}
			t.Args = append(t.Args,
				&text.Sym{Position: npos, Name: name},
				&text.Sym{Position: p.tok.pos, Name: strings.ToLower(p.tok.text)})
			p.unit.Add(t)
			if err := p.advance(); err != nil {
				return err
			}

			// A size expression may follow the type on the same line.
			if !p.atEnd() && !p.isPunct(",") {
				e, err := p.parseExpr(lowestPrec)
				if err != nil {
					return err
				}
				s := &text.Directive{Position: pos, Kind: text.Size, Raw: word}
				s.Args = append(s.Args, &text.Sym{Position: npos, Name: name}, e)
				p.unit.Add(s)
			}
		}

		if !p.isPunct(",") {
			break
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
	return p.endOfStatement(nil)
}

func (p *parser) common(pos text.Pos) error {
	if err := p.advance(); err != nil {
		return err
	}
	if p.tok.kind != tIdent {
		return text.Errorf(p.tok.pos, "expected a symbol name")
	}
	d := &text.Directive{Position: pos, Kind: text.Comm, Raw: "common"}
	d.Args = append(d.Args, &text.Sym{Position: p.tok.pos, Name: p.tok.text})
	if err := p.advance(); err != nil {
		return err
	}
	e, err := p.parseExpr(lowestPrec)
	if err != nil {
		return err
	}
	d.Args = append(d.Args, e)
	p.unit.Add(d)
	return p.endOfStatement(d)
}

// reserve turns resb/resw/resd/resq into a byte count.
func (p *parser) reserve(pos text.Pos, word string, width int64) error {
	if err := p.advance(); err != nil {
		return err
	}
	e, err := p.parseExpr(lowestPrec)
	if err != nil {
		return err
	}
	if width != 1 {
		e = &text.Binary{Position: pos, Op: text.OpMul, X: e,
			Y: &text.Num{Position: pos, Value: width, Base: 10}, Paren: true}
	}
	d := &text.Directive{Position: pos, Kind: text.Zero, Raw: word, Args: []text.Expr{e}}
	p.unit.Add(d)
	return p.endOfStatement(d)
}

// data reads a db/dw/dd/dq list.
//
// A string in the list is not an expression: `db "hi", 0` is three bytes,
// and the neutral tree holds a string in Str rather than in Args. So a list
// that mixes the two becomes more than one node — an Ascii for each string
// and a Byte for each run of expressions, in source order. The bytes are
// identical and the statement splits across lines when printed, which is a
// spelling change arc fmt is allowed to make.
func (p *parser) data(pos text.Pos, word string) error {
	kind := nasmDirectives[word]
	if err := p.advance(); err != nil {
		return err
	}

	var pending []text.Expr
	flush := func() {
		if len(pending) == 0 {
			return
		}
		p.unit.Add(&text.Directive{Position: pos, Kind: kind, Raw: word, Args: pending})
		pending = nil
	}

	for !p.atEnd() {
		if p.tok.kind == tString && len(p.tok.text) > 8 {
			if kind != text.Byte {
				return text.Errorf(p.tok.pos,
					"a string is a byte sequence; %s takes expressions", word)
			}
		}
		if p.tok.kind == tString && kind == text.Byte {
			flush()
			p.unit.Add(&text.Directive{Position: p.tok.pos, Kind: text.Ascii,
				Raw: word, Str: p.tok.text})
			if err := p.advance(); err != nil {
				return err
			}
		} else {
			e, err := p.parseExpr(lowestPrec)
			if err != nil {
				return err
			}
			pending = append(pending, e)
		}

		if !p.isPunct(",") {
			break
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
	flush()
	return p.endOfStatement(nil)
}

// times repeats what follows.
//
// Over a data directive it is exactly gas's .fill — a count, a width and a
// value — and folds to it. Over an instruction it is a repetition, which
// needs an expander, and an expander is a language. Refused by name.
func (p *parser) times(pos text.Pos) error {
	if err := p.advance(); err != nil {
		return err
	}
	count, err := p.parseExpr(lowestPrec)
	if err != nil {
		return err
	}
	if p.tok.kind != tIdent {
		return text.Errorf(p.tok.pos, "times takes a data directive")
	}
	word := strings.ToLower(p.tok.text)
	width, ok := dataWidths[word]
	if !ok {
		return text.Errorf(pos,
			"times before %s is a repetition, which needs a macro expander; "+
				"only times before db/dw/dd/dq is a fill", word)
	}
	if err := p.advance(); err != nil {
		return err
	}
	value, err := p.parseExpr(lowestPrec)
	if err != nil {
		return err
	}

	d := &text.Directive{Position: pos, Kind: text.Fill, Raw: "times", Args: []text.Expr{
		count,
		&text.Num{Position: pos, Value: width, Base: 10},
		value,
	}}
	p.unit.Add(d)
	return p.endOfStatement(d)
}

// ---- printing --------------------------------------------------------

// directive renders a neutral directive in NASM's spelling.
func (pr *printer) directive(d *text.Directive) (string, error) {
	switch d.Kind {
	case text.Section:
		s := "section " + d.SectionName()
		if d.Str != "" {
			s += " " + d.Str
		}
		return s, nil

	case text.Global:
		syms := d.Symbols()
		if len(syms) == 0 {
			return "", text.Errorf(d.Position, "global names no symbol")
		}
		s := "global " + syms[0]
		if t, ok := pr.typeOf[syms[0]]; ok {
			s += ":" + t
			if e, ok := pr.sizeOf[syms[0]]; ok {
				s += " " + printExpr(e)
			}
		}
		return s, nil

	case text.Extern:
		return "extern " + strings.Join(d.Symbols(), ", "), nil

	case text.Local:
		return "static " + strings.Join(d.Symbols(), ", "), nil

	case text.Comm:
		syms := d.Symbols()
		if len(syms) == 0 || len(d.Args) < 2 {
			return "", text.Errorf(d.Position, "common takes a symbol and a size")
		}
		return "common " + syms[0] + " " + printExpr(d.Args[1]), nil

	case text.Type, text.Size:
		// Merged onto the global line when there is one. Reaching here
		// means there is not: NASM writes a symbol's type as part of
		// `global`, so a type on a symbol this file does not export has
		// no NASM spelling at all.
		return "", text.Errorf(d.Position,
			"NASM writes a symbol's %s on its global directive; %s is not global here",
			d.Kind, strings.Join(d.Symbols(), ", "))

	case text.Weak, text.Hidden:
		return "", text.Errorf(d.Position, "NASM has no %s directive", d.Kind)

	case text.LComm:
		return "", text.Errorf(d.Position,
			"NASM has no lcomm; write a static label and a reservation in .bss")

	case text.Equ:
		syms := d.Symbols()
		if len(syms) == 0 || len(d.Args) < 2 {
			return "", text.Errorf(d.Position, "equ takes a name and a value")
		}
		return syms[0] + " equ " + printExpr(d.Args[1]), nil

	case text.Byte, text.Word, text.Long, text.Quad:
		return dataSpelling(d.Kind) + " " + printArgs(d.Args), nil

	case text.Ascii:
		return "db " + quote(d.Str), nil

	case text.Asciz:
		// NASM has no terminating-string directive; the zero is written.
		return "db " + quote(d.Str) + ", 0", nil

	case text.Align:
		return "align " + printArgs(d.Args), nil

	case text.P2Align:
		// NASM's align is a byte count and has no exponent form, so the
		// exponent is folded. It has to be constant to be folded, which it
		// has to be anyway: the size of a statement cannot depend on a
		// symbol.
		n, err := d.Alignment(nil)
		if err != nil {
			return "", err
		}
		return "align " + dec(n), nil

	case text.Fill:
		if len(d.Args) < 3 {
			return "", text.Errorf(d.Position, "fill takes a count, a width and a value")
		}
		w, err := text.Eval(d.Args[1], nil)
		if err != nil {
			return "", err
		}
		spell, ok := fillSpelling(w)
		if !ok {
			return "", text.Errorf(d.Position, "no NASM data directive is %d bytes wide", w)
		}
		return "times " + printExpr(d.Args[0]) + " " + spell + " " + printExpr(d.Args[2]), nil

	case text.Zero:
		// In a nobits section this is a reservation and in any other it is
		// zeroed data, and NASM refuses resb in one and db in the other.
		// The printer tracks the section it is inside for exactly this.
		if pr.nobits {
			return "resb " + printArgs(d.Args), nil
		}
		return "times " + printArgs(d.Args) + " db 0", nil

	case text.Org:
		return "org " + printArgs(d.Args), nil
	}
	return "", text.Errorf(d.Position, "no NASM spelling for %s", d.Kind)
}

func dataSpelling(k text.Kind) string {
	switch k {
	case text.Byte:
		return "db"
	case text.Word:
		return "dw"
	case text.Long:
		return "dd"
	case text.Quad:
		return "dq"
	}
	return "db"
}

func fillSpelling(width int64) (string, bool) {
	switch width {
	case 1:
		return "db", true
	case 2:
		return "dw", true
	case 4:
		return "dd", true
	case 8:
		return "dq", true
	}
	return "", false
}

func printArgs(args []text.Expr) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, printExpr(a))
	}
	return strings.Join(parts, ", ")
}