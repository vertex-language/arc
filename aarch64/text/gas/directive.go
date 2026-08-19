package gas

import (
	"strings"

	"github.com/vertex-language/arc/aarch64/text"
)

// The directive table: spelling to meaning.
//
// The classification is text/'s DirKind and this is only the lookup, because
// what .globl does is the architecture's answer and how it is spelled is this
// package's. A spelling absent here is an error rather than a pass-through:
// silently ignoring a directive produces an object missing something the source
// asked for, which is the failure hardest to notice.
var dirKind = map[string]text.DirKind{
	// Sections. The four shorthands are directives in their own right and each
	// names the section it switches to.
	".section": text.DirSection,
	".text":    text.DirSection,
	".data":    text.DirSection,
	".bss":     text.DirSection,
	".rodata":  text.DirSection,
	".pushsection": text.DirSection,
	".popsection":  text.DirSection,
	".previous":    text.DirSection,

	// Alignment. .align on this target takes a byte count; .p2align takes an
	// exponent; .balign takes a byte count everywhere. text.Directive.Alignment
	// resolves the difference, which is why all three land on one kind.
	".align":   text.DirAlign,
	".balign":  text.DirAlign,
	".p2align": text.DirAlign,
	".even":    text.DirAlign,

	".org": text.DirOrg,

	// Symbols.
	".globl":   text.DirBinding,
	".global":  text.DirBinding,
	".weak":    text.DirBinding,
	".local":   text.DirBinding,
	".hidden":  text.DirBinding,
	".protected": text.DirBinding,
	".internal":  text.DirBinding,
	".type":    text.DirType,
	".size":    text.DirSize,
	".variant_pcs": text.DirVariantPCS,

	// Data. The widths are the architecture's: .word is four bytes and .xword
	// or .dword is eight, which is the opposite of x86's convention.
	".byte":   text.DirData,
	".hword":  text.DirData,
	".short":  text.DirData,
	".2byte":  text.DirData,
	".word":   text.DirData,
	".long":   text.DirData,
	".int":    text.DirData,
	".4byte":  text.DirData,
	".xword":  text.DirData,
	".dword":  text.DirData,
	".quad":   text.DirData,
	".8byte":  text.DirData,
	".ascii":  text.DirData,
	".asciz":  text.DirData,
	".string": text.DirData,

	".space": text.DirSpace,
	".skip":  text.DirSpace,
	".zero":  text.DirSpace,
	".fill":  text.DirSpace,

	".equ":   text.DirEqu,
	".set":   text.DirEqu,
	".equiv": text.DirEqu,
	".comm":  text.DirComm,
	".lcomm": text.DirComm,

	// Architecture state — the group that is this architecture's own.
	".arch":           text.DirArch,
	".arch_extension": text.DirArchExt,
	".cpu":            text.DirCPU,
	".req":            text.DirReq,
	".unreq":          text.DirUnreq,

	// The literal pool. Both are recognized and refused: placing a constant
	// into a pool means choosing where data lives, which is a layout engine.
	".ltorg": text.DirPool,
	".pool":  text.DirPool,

	// Opaque payloads: recorded, passed through, and never interpreted.
	".file":  text.DirOpaque,
	".loc":   text.DirOpaque,
	".ident": text.DirOpaque,
	".aeabi_subsection": text.DirOpaque,
	".aeabi_attribute":  text.DirOpaque,
}

// lookupDirective resolves a spelling, folding the .cfi_* family into one kind
// by prefix rather than listing forty names that all mean the same thing here.
func lookupDirective(name string) (text.DirKind, bool) {
	n := strings.ToLower(name)
	if k, ok := dirKind[n]; ok {
		return k, true
	}
	if strings.HasPrefix(n, ".cfi_") {
		return text.DirCFI, true
	}
	return text.DirNone, false
}

// impliedSection is the section a shorthand directive names, or "".
func impliedSection(spelling string) string {
	switch strings.ToLower(spelling) {
	case ".text":
		return ".text"
	case ".data":
		return ".data"
	case ".bss":
		return ".bss"
	case ".rodata":
		return ".rodata"
	}
	return ""
}

// binding is the symbol binding a .globl-family directive states.
func binding(spelling string) string { return strings.TrimPrefix(strings.ToLower(spelling), ".") }

// parseDirective reads a directive statement, the leading token already seen.
func (p *Parser) parseDirective(tok Token) *text.Directive {
	kind, known := lookupDirective(tok.Text)
	if !known {
		p.errorf(tok.Pos, "unknown directive %s", tok.Text)
		p.skipStatement()
		return nil
	}

	d := &text.Directive{Kind: kind, Spelling: tok.Text, P: tok.Pos}

	switch kind {
	case text.DirSection:
		p.parseSectionArgs(d)

	case text.DirBinding, text.DirVariantPCS, text.DirUnreq:
		d.Name = p.parseName()
		if kind == text.DirBinding {
			d.Flags = append(d.Flags, binding(tok.Text))
		}

	case text.DirType:
		d.Name = p.parseName()
		if p.acceptPunct(",") {
			d.Flags = append(d.Flags, p.parseTypeArg())
		}

	case text.DirSize:
		d.Name = p.parseName()
		if p.acceptPunct(",") {
			d.Args = append(d.Args, p.parseExpr())
		}

	case text.DirArch, text.DirCPU, text.DirArchExt:
		// The argument is a feature spec — armv8.2-a+sve+nofp16 — which the
		// lexer has already split on the '+' it does not know is significant.
		// Rejoining it here is cheaper than teaching the lexer a mode.
		d.Name = p.parseFeatureSpec()

	case text.DirEqu, text.DirComm:
		d.Name = p.parseName()
		for p.acceptPunct(",") {
			d.Args = append(d.Args, p.parseExpr())
		}

	case text.DirData, text.DirSpace, text.DirAlign, text.DirOrg:
		p.parseExprList(d)

	case text.DirPool:
		// No arguments, and refused at assemble time rather than here: the
		// directive is legal syntax, and a parse error would be the wrong
		// place to explain why it has no implementation.

	case text.DirCFI, text.DirOpaque:
		d.Flags = p.parseRawArgs()
	}

	d.Comment = p.finishStatement()
	return d
}

// parseSectionArgs reads .section's name and flags, or a shorthand's implied
// name.
func (p *Parser) parseSectionArgs(d *text.Directive) {
	if s := impliedSection(d.Spelling); s != "" {
		d.Name = s
		// .text 1 is a subsection number, which this tree has no model for.
		if p.lex.Peek().Kind == Number {
			t := p.lex.Next()
			p.errorf(t.Pos, "subsections are not supported")
		}
		return
	}
	d.Name = p.parseName()
	for p.acceptPunct(",") {
		t := p.lex.Next()
		d.Flags = append(d.Flags, t.Text)
	}
}

// parseTypeArg reads .type's second argument, which gas writes as @function,
// %function or "function" depending on the target's comment character.
func (p *Parser) parseTypeArg() string {
	if p.acceptPunct("@") || p.acceptPunct("%") {
		return p.parseName()
	}
	t := p.lex.Next()
	return t.Text
}

// parseFeatureSpec rejoins a +-separated feature spec.
func (p *Parser) parseFeatureSpec() string {
	var b strings.Builder
	for {
		t := p.lex.Peek()
		if t.Kind == EOL || t.Kind == EOF || t.Kind == Comment {
			break
		}
		if t.Spaced && b.Len() > 0 {
			break
		}
		p.lex.Next()
		b.WriteString(t.Text)
	}
	return b.String()
}

func (p *Parser) parseExprList(d *text.Directive) {
	for {
		t := p.lex.Peek()
		if t.Kind == EOL || t.Kind == EOF || t.Kind == Comment {
			return
		}
		if t.Kind == String {
			p.lex.Next()
			d.Strings = append(d.Strings, t.Text)
		} else {
			e := p.parseExpr()
			if e == nil {
				p.skipStatement()
				return
			}
			d.Args = append(d.Args, e)
		}
		if !p.acceptPunct(",") {
			return
		}
	}
}

// parseRawArgs consumes a directive's tail without interpreting it, which is
// what an opaque payload is: bytes that pass through untouched.
func (p *Parser) parseRawArgs() []string {
	var out []string
	for {
		t := p.lex.Peek()
		if t.Kind == EOL || t.Kind == EOF || t.Kind == Comment {
			return out
		}
		p.lex.Next()
		out = append(out, t.Text)
	}
}