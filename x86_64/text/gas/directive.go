// x86_64/text/gas/directive.go
package gas

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/text"
)

// gasDirectives maps a gas spelling to the neutral kind.
//
// Absent on purpose: .if, .ifdef, .else, .endif, .rept, .irp, .macro,
// .endm, .altmacro. Those are a language rather than a syntax, and a parser
// that accepted them would need an evaluator that decides which branch of a
// file exists — which is a different program from this one. They are
// recognized only well enough to refuse them by name.
var gasDirectives = map[string]text.Kind{
	".section": text.Section,
	".text":    text.Section,
	".data":    text.Section,
	".bss":     text.Section,
	".rodata":  text.Section,

	".globl":  text.Global,
	".global": text.Global,
	".local":  text.Local,
	".weak":   text.Weak,
	".extern": text.Extern,
	".hidden": text.Hidden,
	".type":   text.Type,
	".size":   text.Size,
	".comm":   text.Comm,
	".lcomm":  text.LComm,
	".equ":    text.Equ,
	".set":    text.Equ,

	".byte":  text.Byte,
	".word":  text.Word,
	".short": text.Word,
	".hword": text.Word,
	".long":  text.Long,
	".int":   text.Long,
	".quad":  text.Quad,
	".ascii": text.Ascii,
	".asciz": text.Asciz,
	".string": text.Asciz,

	".align":   text.Align,
	".balign":  text.Align,
	".p2align": text.P2Align,
	".fill":    text.Fill,
	".zero":    text.Zero,
	".skip":    text.Zero,
	".space":   text.Zero,
	".org":     text.Org,
}

var macroDirectives = map[string]bool{
	".if": true, ".ifdef": true, ".ifndef": true, ".ifeq": true, ".ifne": true,
	".else": true, ".elseif": true, ".endif": true,
	".rept": true, ".irp": true, ".irpc": true, ".endr": true,
	".macro": true, ".endm": true, ".altmacro": true, ".noaltmacro": true,
}

// shorthandSections are the directives that name a section by being
// themselves. `.text` is `.section .text` with the name folded into the
// spelling, and the tree holds the general form.
var shorthandSections = map[string]string{
	".text":   ".text",
	".data":   ".data",
	".bss":    ".bss",
	".rodata": ".rodata",
}

func (p *parser) directive() error {
	pos := p.tok.pos
	raw := strings.ToLower(p.tok.text)

	if macroDirectives[raw] {
		return text.Wrap(pos, text.ErrMacro)
	}

	kind, ok := gasDirectives[raw]
	if !ok {
		// A .cfi_* or .loc is debug information, which passes through as an
		// opaque payload rather than being understood. Anything else is an
		// unknown directive and saying so is more useful than ignoring it.
		if strings.HasPrefix(raw, ".cfi_") || raw == ".loc" || raw == ".file" {
			return p.opaque(pos, raw)
		}
		return text.Errorf(pos, "unknown directive %s", raw)
	}

	d := &text.Directive{Position: pos, Kind: kind, Raw: raw}
	if err := p.advance(); err != nil {
		return err
	}

	if name, ok := shorthandSections[raw]; ok {
		d.Args = append(d.Args, &text.Sym{Position: pos, Name: name})
		p.unit.Add(d)
		return p.endOfStatement(d)
	}

	for p.tok.kind != tNewline && p.tok.kind != tEOF && p.tok.kind != tComment {
		switch {
		case p.tok.kind == tString:
			d.Str = p.tok.text
			if err := p.advance(); err != nil {
				return err
			}
		case p.tok.kind == tAt && kind == text.Type:
			// .type name,@function — the @ is part of the type spelling and
			// not a relocation modifier.
			if err := p.advance(); err != nil {
				return err
			}
			if p.tok.kind != tIdent {
				return text.Errorf(p.tok.pos, "expected a symbol type after @")
			}
			d.Args = append(d.Args, &text.Sym{Position: p.tok.pos, Name: p.tok.text})
			if err := p.advance(); err != nil {
				return err
			}
		default:
			e, err := p.parseExpr(lowestPrec)
			if err != nil {
				return err
			}
			d.Args = append(d.Args, e)
		}

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

// opaque swallows a directive that is passed through untouched: debug
// payloads, which attach as bytes and are never interpreted.
func (p *parser) opaque(pos text.Pos, raw string) error {
	var b strings.Builder
	b.WriteString(raw)
	if err := p.advance(); err != nil {
		return err
	}
	for p.tok.kind != tNewline && p.tok.kind != tEOF && p.tok.kind != tComment {
		b.WriteString(" " + p.tok.text)
		if err := p.advance(); err != nil {
			return err
		}
	}
	c := &text.Comment{Position: pos, Text: b.String()}
	p.unit.Add(c)
	return p.endOfStatement(nil)
}

// printDirective is the inverse: the neutral kind back into gas's spelling.
func printDirective(d *text.Directive) string {
	name := gasSpelling(d)
	var b strings.Builder
	b.WriteString(name)

	args := d.Args
	if d.Kind == text.Section && name != ".section" {
		args = nil // the shorthand names the section by being itself
	}

	for i, a := range args {
		if i == 0 {
			b.WriteString(" ")
		} else {
			b.WriteString(", ")
		}
		if d.Kind == text.Type && i == 1 {
			b.WriteString("@" + strings.TrimLeft(printExpr(a), "@"))
			continue
		}
		b.WriteString(printExpr(a))
	}
	if d.Str != "" {
		if len(args) > 0 {
			b.WriteString(", ")
		} else {
			b.WriteString(" ")
		}
		b.WriteString(quote(d.Str))
	}
	return b.String()
}

func gasSpelling(d *text.Directive) string {
	if d.Kind == text.Section {
		if n := d.SectionName(); shorthandSections[n] != "" {
			return n
		}
		return ".section"
	}
	switch d.Kind {
	case text.Global:
		return ".globl"
	case text.Local:
		return ".local"
	case text.Weak:
		return ".weak"
	case text.Extern:
		return ".extern"
	case text.Hidden:
		return ".hidden"
	case text.Type:
		return ".type"
	case text.Size:
		return ".size"
	case text.Comm:
		return ".comm"
	case text.LComm:
		return ".lcomm"
	case text.Equ:
		return ".equ"
	case text.Byte:
		return ".byte"
	case text.Word:
		return ".word"
	case text.Long:
		return ".long"
	case text.Quad:
		return ".quad"
	case text.Ascii:
		return ".ascii"
	case text.Asciz:
		return ".asciz"
	case text.Align:
		return ".align"
	case text.P2Align:
		return ".p2align"
	case text.Fill:
		return ".fill"
	case text.Zero:
		return ".zero"
	case text.Org:
		return ".org"
	}
	return "." + d.Kind.String()
}