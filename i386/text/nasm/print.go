package nasm

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

// Printing is parsing's inverse at the semantic level, same standard gas
// holds itself to.
//
// The one structural simplification versus gas's printer: NASM operands are
// stored and written in the same order, so there is no reversal here, and
// NASM has no indirect-call sigil, so a gas-parsed text.Indirect prints as
// its bare inner operand — `call *%eax` becomes `call eax`, `call *foo`
// becomes `call [foo]` — which is exactly what makes a unit parsed from one
// dialect and printed in the other assemble to the same bytes.

const (
	indent   = "        "
	opCol    = 8
	maxBlank = 2
)

func Print(u *text.Unit) ([]byte, error) {
	p := &printer{}
	for _, n := range u.Nodes {
		p.node(n)
	}
	p.trivia(u.Tail)
	return []byte(p.b.String()), p.err
}

func PrintInst(i *text.Inst) string {
	p := &printer{}
	p.inst(i)
	return strings.TrimRight(strings.TrimPrefix(p.b.String(), indent), "\n")
}

type printer struct {
	b       strings.Builder
	err     error
	pending string
}

func (p *printer) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}

func (p *printer) node(n text.Node) {
	tv := *n.Trivia()

	switch d := n.(type) {
	case *text.Label:
		p.trivia(tv)
		if d.Attached {
			p.pending = d.Name + ":"
			return
		}
		p.line(d.Name+":", tv)

	case *text.SectionDecl:
		p.trivia(tv)
		p.line("section "+d.Name, tv)

	case *text.SymbolDecl:
		p.trivia(tv)
		p.line(p.symbol(d), tv)

	case *text.Equ:
		p.trivia(tv)
		p.line(d.Name+" equ "+p.expr(d.Value, precLOr), tv)

	case *text.Data:
		p.trivia(tv)
		p.data(d, tv)

	case *text.Fill:
		p.trivia(tv)
		p.line(p.fill(d), tv)

	case *text.Inst:
		p.trivia(tv)
		p.inst(d)
		p.comment(tv)

	default:
		p.fail("nasm: cannot print %T", n)
	}
}

func (p *printer) trivia(tv text.Trivia) {
	n := tv.Blanks
	if n > maxBlank {
		n = maxBlank
	}
	for i := 0; i < n; i++ {
		p.b.WriteByte('\n')
	}
	for _, c := range tv.Before {
		p.b.WriteString(";")
		if c != "" {
			p.b.WriteString(" " + c)
		}
		p.b.WriteByte('\n')
	}
}

func (p *printer) line(s string, tv text.Trivia) {
	p.open()
	p.b.WriteString(s)
	p.comment(tv)
}

func (p *printer) open() {
	if p.pending != "" {
		p.b.WriteString(p.pending)
		if len(p.pending) < opCol {
			p.b.WriteString(strings.Repeat(" ", opCol-len(p.pending)))
		} else {
			p.b.WriteString(" ")
		}
		p.pending = ""
		return
	}
	p.b.WriteString(indent)
}

func (p *printer) comment(tv text.Trivia) {
	if tv.HasLine {
		p.b.WriteString("   ;")
		if tv.Line != "" {
			p.b.WriteString(" " + tv.Line)
		}
	}
	p.b.WriteByte('\n')
}

func field(s string) string {
	if len(s) >= opCol {
		return s + " "
	}
	return s + strings.Repeat(" ", opCol-len(s))
}

func (p *printer) symbol(d *text.SymbolDecl) string {
	kw := "global"
	switch {
	case d.Attrs&text.AttrExtern != 0:
		kw = "extern"
	case d.Attrs&text.AttrLocal != 0:
		kw = "static"
	}
	s := kw + " " + strings.Join(d.Names, ", ")
	if d.Type != text.TypeNone && len(d.Names) == 1 {
		if d.Type == text.TypeFunc {
			s += ":function"
		} else {
			s += ":data"
		}
	}
	return s
}

func (p *printer) data(d *text.Data, tv text.Trivia) {
	name := dataDirective(d.Width)
	var parts []string
	for _, it := range d.Items {
		if it.IsStr {
			parts = append(parts, quote(it.Str))
		} else {
			parts = append(parts, p.expr(it.X, precLOr))
		}
	}
	p.open()
	p.b.WriteString(field(name) + strings.Join(parts, ", "))
	p.comment(tv)
}

func dataDirective(w text.Width) string {
	switch w {
	case text.Width8:
		return "db"
	case text.Width16:
		return "dw"
	case text.Width32:
		return "dd"
	case text.Width64:
		return "dq"
	case text.Width80:
		return "dt"
	case text.Width128:
		return "do"
	}
	return "db"
}

func resDirective(w text.Width) string {
	switch w {
	case text.Width8:
		return "resb"
	case text.Width16:
		return "resw"
	case text.Width32:
		return "resd"
	case text.Width64:
		return "resq"
	case text.Width80:
		return "rest"
	case text.Width128:
		return "reso"
	}
	return "resb"
}

func (p *printer) fill(d *text.Fill) string {
	if d.Value == nil {
		return field(resDirective(d.Size)) + p.expr(d.Count, precLOr)
	}
	return "times " + p.expr(d.Count, precMul+1) + " " +
		field(dataDirective(d.Size)) + p.expr(d.Value, precLOr)
}

func (p *printer) inst(in *text.Inst) {
	p.open()

	name := in.Mnemonic
	if in.Prefix != text.PrefixNone {
		name = in.Prefix.String() + " " + name
	}

	if len(in.Ops) == 0 {
		p.b.WriteString(strings.TrimRight(field(name), " "))
		return
	}

	// The size hint, if the instruction carries one, is written before the
	// first memory operand, or before the first immediate if there is no
	// memory operand — the best this printer can do without a per-operand
	// width field to consult.
	memIdx, immIdx := -1, -1
	for i, o := range in.Ops {
		switch o.(type) {
		case text.Mem:
			if memIdx < 0 {
				memIdx = i
			}
		case text.Imm:
			if immIdx < 0 {
				immIdx = i
			}
		}
	}
	hintAt := memIdx
	if hintAt < 0 {
		hintAt = immIdx
	}

	parts := make([]string, len(in.Ops))
	for i, o := range in.Ops {
		s := p.operand(o)
		if in.Size != text.WidthNone && i == hintAt {
			s = sizeWord(in.Size) + " " + s
		}
		parts[i] = s
	}
	p.b.WriteString(field(name) + strings.Join(parts, ", "))
}

func sizeWord(w text.Width) string {
	switch w {
	case text.Width8:
		return "byte"
	case text.Width16:
		return "word"
	case text.Width32:
		return "dword"
	case text.Width64:
		return "qword"
	case text.Width80:
		return "tword"
	case text.Width128:
		return "oword"
	}
	return ""
}

func (p *printer) operand(o text.Operand) string {
	switch v := o.(type) {
	case text.Reg:
		return v.R.Name()

	case text.Imm:
		return p.expr(v.X, precLOr)

	case text.Indirect:
		// No sigil in this syntax: whatever the inner operand is, printing
		// it bare is already the indirect form.
		return p.operand(v.X)

	case text.Mem:
		return p.mem(v)
	}
	p.fail("nasm: cannot print operand %T", o)
	return ""
}

func (p *printer) mem(m text.Mem) string {
	var b strings.Builder
	b.WriteByte('[')
	if m.HasSeg {
		b.WriteString(m.Seg.Name() + ":")
	}
	wrote := false
	if m.HasBase {
		b.WriteString(m.Base.Name())
		wrote = true
	}
	if m.HasIndex {
		if wrote {
			b.WriteByte('+')
		}
		b.WriteString(fmt.Sprintf("%s*%d", m.Index.Name(), m.Scale))
		wrote = true
	}
	if m.Disp != nil {
		s := p.expr(m.Disp, precAdd)
		if wrote && !strings.HasPrefix(s, "-") {
			b.WriteByte('+')
		}
		b.WriteString(s)
		wrote = true
	}
	if !wrote {
		b.WriteByte('0')
	}
	b.WriteByte(']')
	return b.String()
}

// nasmWrtSpelling is the print-side inverse of nasmWrtModifier.
var nasmWrtSpelling = map[text.Modifier]string{
	text.ModPLT: "..plt", text.ModGOT: "..got",
	text.ModGOTOFF: "..gotoff", text.ModGOTPC: "..gotpc",
}

func (p *printer) expr(e text.Expr, outer int) string {
	switch v := e.(type) {
	case *text.Int:
		return fmt.Sprint(v.Value)

	case *text.SymExpr:
		if v.Mod == text.ModNone {
			return v.Name
		}
		w, ok := nasmWrtSpelling[v.Mod]
		if !ok {
			p.fail("nasm: modifier %s has no WRT spelling", v.Mod)
			return v.Name
		}
		return v.Name + " wrt " + w

	case *text.Here:
		return "$"

	case *text.Start:
		return "$$"

	case *text.Unary:
		return v.Op.String() + p.expr(v.X, precUnary)

	case *text.Binary:
		prec := precOf(v.Op)
		s := p.expr(v.X, prec) + " " + spellingOf(v.Op) + " " + p.expr(v.Y, prec+1)
		if prec < outer {
			return "(" + s + ")"
		}
		return s
	}
	p.fail("nasm: cannot print expression %T", e)
	return ""
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			b.WriteString("', 39, '") // NASM verbatim strings have no escape;
			continue                  // splice the quote in as its own char.
		}
		b.WriteByte(c)
	}
	b.WriteByte('\'')
	return b.String()
}

var _ = reg.EAX // keep reg imported for the Sreg/R32 types referenced above