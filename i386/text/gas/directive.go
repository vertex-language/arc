package gas

import (
	"strings"

	"github.com/vertex-language/arc/i386/text"
)

// The directive spellings of this dialect.
//
// Only the spellings. What a directive means — that a word is two bytes on
// i386, which section names have a portable kind, how a p2align exponent
// becomes a boundary — is text/'s, because those are the arch's answers and
// do not change with the syntax.
//
// The set is what a .s file needs and no more. Absent, deliberately:
// .macro, .rept, .irp, .if — the language features that put MASM and HLASM
// out; .include and .incbin — arc does not open files the command line did
// not name; .float, .double and .single — the builder API declares seven data
// calls and none is a float; .cfi_* — unwind tables are a payload arc
// attaches with --debug-section and never generates.

// dataWidths are the directives that emit initialised numbers.
var dataWidths = map[string]text.Width{
	".byte":  text.Width8,
	".word":  text.WordWidth, // two bytes here, four in arm/text
	".short": text.WordWidth,
	".hword": text.WordWidth,
	".value": text.WordWidth,
	".long":  text.Width32,
	".int":   text.Width32,
	".quad":  text.Width64,
	".octa":  text.Width128,
	".2byte": text.Width16,
	".4byte": text.Width32,
	".8byte": text.Width64,
}

// shortSections are the one-word section directives.
var shortSections = map[string]struct {
	kind text.SectionKind
	name string
}{
	".text":   {text.SectionText, ".text"},
	".data":   {text.SectionData, ".data"},
	".rodata": {text.SectionROData, ".rodata"},
	".bss":    {text.SectionBSS, ".bss"},
}

var symbolAttrs = map[string]text.Attr{
	".globl":     text.AttrGlobal,
	".global":    text.AttrGlobal,
	".local":     text.AttrLocal,
	".weak":      text.AttrWeak,
	".hidden":    text.AttrHidden,
	".protected": text.AttrProtected,
	".internal":  text.AttrInternal,
	".extern":    text.AttrExtern,
}

var symbolTypes = map[string]text.SymbolType{
	"function":   text.TypeFunc,
	"func":       text.TypeFunc,
	"object":     text.TypeObject,
	"tls_object": text.TypeTLS,
}

// rejected names directives arc understands well enough to refuse by name,
// with the reason. An unknown directive gets a shorter message; these get the
// rule, because the rule is the answer and "unknown" would suggest it might
// land later.
var rejected = map[string]string{
	".macro":  "macros are a language feature; Go is arc's macro language",
	".endm":   "macros are a language feature; Go is arc's macro language",
	".rept":   "repetition needs an expander; use .fill for data",
	".endr":   "repetition needs an expander; use .fill for data",
	".irp":    "repetition needs an expander",
	".if":     "conditional assembly is a language feature and languages are out",
	".ifdef":  "conditional assembly is a language feature and languages are out",
	".else":   "conditional assembly is a language feature and languages are out",
	".endif":  "conditional assembly is a language feature and languages are out",
	".include": "arc does not open files the command line did not name",
	".incbin":  "arc does not open files the command line did not name",
	".float":   "arc emits .byte, .word, .long and .quad; there is no float directive",
	".double":  "arc emits .byte, .word, .long and .quad; there is no float directive",
	".single":  "arc emits .byte, .word, .long and .quad; there is no float directive",
	".intel_syntax": "a dialect is selected by --dialect, not from inside a file",
	".att_syntax":   "a dialect is selected by --dialect, not from inside a file",
	".code16":       "arc's i386 is protected mode; 16-bit code is a different addressing table",
	".arch":         "the target is selected by -t and --features, not from inside a file",
}

func (p *parser) directive(pos text.Pos, name string) bool {
	d := strings.ToLower(name)

	if why, no := rejected[d]; no {
		p.errorf(pos, "%s is not accepted", d).Note("%s", why)
		return false
	}

	switch {
	case dataWidths[d] != 0:
		return p.data(pos, dataWidths[d])
	}

	switch d {
	case ".ascii", ".asciz", ".string":
		return p.strings(pos, d != ".ascii")

	case ".section":
		return p.section(pos)

	case ".text", ".data", ".rodata", ".bss":
		s := shortSections[d]
		p.unit.Add(&text.SectionDecl{
			Common: text.Common{P: pos},
			Kind:   s.kind, Name: s.name, Short: true,
		})
		return true

	case ".zero", ".skip", ".space":
		return p.fill(pos, d)

	case ".fill":
		return p.fillFull(pos)

	case ".align", ".balign":
		return p.align(pos, false)

	case ".p2align":
		return p.align(pos, true)

	case ".equ", ".set":
		return p.equ(pos)

	case ".type":
		return p.symType(pos)

	case ".size":
		return p.symSize(pos)

	case ".comm", ".lcomm":
		p.errorf(pos, "%s is not accepted", d).
			Note("a common symbol has no section; declare it in .bss with .zero")
		return false
	}

	if attr, ok := symbolAttrs[d]; ok {
		return p.symAttr(pos, attr)
	}

	p.errorf(pos, "unknown directive %s", d).
		Note("arc accepts the directives that name sections, symbols and data")
	return false
}

func (p *parser) data(pos text.Pos, w text.Width) bool {
	if !text.DataWidth(w) {
		p.errorf(pos, "%s is not a data width", w)
		return false
	}
	n := &text.Data{Common: text.Common{P: pos}, Width: w}
	for {
		item := text.DataItem{Pos: p.tok.pos}
		if p.tok.kind == tString {
			item.Str, item.IsStr = p.tok.str, true
			p.advance()
		} else {
			item.X = p.expr()
		}
		n.Items = append(n.Items, item)
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}
	p.unit.Add(n)
	return true
}

func (p *parser) strings(pos text.Pos, terminated bool) bool {
	n := &text.Data{Common: text.Common{P: pos}, Width: text.Width8}
	for {
		if p.tok.kind != tString {
			p.errorf(p.tok.pos, "expected a string, got %s", p.tok)
			return false
		}
		n.Items = append(n.Items, text.DataItem{
			Pos: p.tok.pos, Str: p.tok.str, IsStr: true, Terminated: terminated,
		})
		p.advance()
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}
	p.unit.Add(n)
	return true
}

// section parses .section name[, "flags"[, @type]].
//
// Flags and type pass through verbatim. arc does not parse "ax" or @progbits,
// because the object layer takes flags as data and inventing one the source
// did not write is what "arc does not know what Linux is" forbids.
func (p *parser) section(pos text.Pos) bool {
	if p.tok.kind != tIdent && p.tok.kind != tString {
		p.errorf(p.tok.pos, "expected a section name, got %s", p.tok)
		return false
	}
	name := p.tok.str
	p.advance()

	d := &text.SectionDecl{Common: text.Common{P: pos}, Name: name}
	d.Kind, _ = text.StandardSection(name)

	if p.tok.is(",") {
		p.advance()
		if p.tok.kind != tString {
			p.errorf(p.tok.pos, "expected a quoted flag string, got %s", p.tok)
			return false
		}
		d.Flags = p.tok.str
		p.advance()
	}
	if p.tok.is(",") {
		p.advance()
		if p.tok.is("@") || p.tok.is("%") {
			p.advance()
		}
		if p.tok.kind != tIdent {
			p.errorf(p.tok.pos, "expected a section type, got %s", p.tok)
			return false
		}
		d.Type = p.tok.str
		p.advance()
	}

	p.unit.Add(d)
	return true
}

// fill parses .zero, .skip and .space: a count, and optionally a fill byte.
func (p *parser) fill(pos text.Pos, name string) bool {
	n := &text.Fill{Common: text.Common{P: pos}, Size: text.Width8}
	n.Count = p.expr()
	if p.tok.is(",") {
		p.advance()
		n.Value = p.expr()
	}
	p.unit.Add(n)
	return true
}

// fillFull parses .fill repeat[, size[, value]].
func (p *parser) fillFull(pos text.Pos) bool {
	n := &text.Fill{Common: text.Common{P: pos}, Size: text.Width8}
	n.Count = p.expr()

	if p.tok.is(",") {
		p.advance()
		sz, ok := p.absolute(".fill size")
		if !ok {
			return false
		}
		switch sz {
		case 1, 2, 4, 8:
			n.Size = text.Width(sz)
		default:
			p.errorf(pos, ".fill size %d is not 1, 2, 4 or 8", sz)
			return false
		}
	}
	if p.tok.is(",") {
		p.advance()
		n.Value = p.expr()
	}
	p.unit.Add(n)
	return true
}

// align parses .align, .balign and .p2align.
//
// The exponent form converts to a byte count at parse: two spellings of one
// boundary is a spelling, so the tree stores one value and the printer writes
// back whichever the source used. On x86 ELF, .align takes bytes.
func (p *parser) align(pos text.Pos, p2 bool) bool {
	n := &text.Align{Common: text.Common{P: pos}, P2: p2}

	v, ok := p.absolute("an alignment")
	if !ok {
		return false
	}
	if p2 {
		b, err := text.AlignBoundary(pos, v)
		if err != nil {
			p.errs.Add(err)
			return false
		}
		v = b
	}
	if err := text.CheckAlign(pos, v); err != nil {
		p.errs.Add(err)
		return false
	}
	n.Bytes = &text.Int{P: pos, Value: v}

	if p.tok.is(",") {
		p.advance()
		if !p.tok.is(",") {
			n.Value = p.expr()
		}
	}
	if p.tok.is(",") {
		p.advance()
		n.Max = p.expr()
	}
	p.unit.Add(n)
	return true
}

func (p *parser) equ(pos text.Pos) bool {
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a name, got %s", p.tok)
		return false
	}
	name := p.tok.str
	p.advance()
	if !p.tok.is(",") {
		p.errorf(p.tok.pos, "expected ',' after the name, got %s", p.tok)
		return false
	}
	p.advance()
	p.unit.Add(&text.Equ{Common: text.Common{P: pos}, Name: name, Value: p.expr()})
	return true
}

func (p *parser) symAttr(pos text.Pos, attr text.Attr) bool {
	d := &text.SymbolDecl{Common: text.Common{P: pos}, Attrs: attr}
	for {
		if p.tok.kind != tIdent {
			p.errorf(p.tok.pos, "expected a symbol name, got %s", p.tok)
			return false
		}
		d.Names = append(d.Names, p.tok.str)
		p.advance()
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}
	p.unit.Add(d)
	return true
}

func (p *parser) symType(pos text.Pos) bool {
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a symbol name, got %s", p.tok)
		return false
	}
	name := p.tok.str
	p.advance()
	if p.tok.is(",") {
		p.advance()
	}
	if p.tok.is("@") || p.tok.is("%") || p.tok.is("#") {
		p.advance()
	}
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a symbol type, got %s", p.tok)
		return false
	}
	t, ok := symbolTypes[strings.ToLower(p.tok.str)]
	if !ok {
		p.errorf(p.tok.pos, "unknown symbol type %q", p.tok.str).
			Note("arc records function, object and tls_object")
		return false
	}
	p.advance()
	p.unit.Add(&text.SymbolDecl{
		Common: text.Common{P: pos}, Names: []string{name}, Type: t,
	})
	return true
}

func (p *parser) symSize(pos text.Pos) bool {
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a symbol name, got %s", p.tok)
		return false
	}
	name := p.tok.str
	p.advance()
	if !p.tok.is(",") {
		p.errorf(p.tok.pos, "expected ',' after the name, got %s", p.tok)
		return false
	}
	p.advance()
	p.unit.Add(&text.SymbolDecl{
		Common: text.Common{P: pos}, Names: []string{name}, Size: p.expr(),
	})
	return true
}