package gas

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

// Printing is parsing's inverse at the semantic level. Anything arc fmt
// changes assembles to identical bytes, and the differential suite against
// GNU as is the proof.
//
// Two normalisations this printer performs, both text-only:
//
//   - A NASM db that mixes strings and numbers becomes consecutive .ascii and
//     .byte statements, because GNU as has no directive that mixes. One node
//     in, several lines out, the same bytes.
//   - Block comments become line comments. The content survives; the
//     spelling does not.
//
// Everything else is written back as it was written: .text rather than
// .section .text when the source used the short form, .p2align when it wrote
// the exponent, the mnemonic suffix when the source stated a size.

const (
	indent  = "        " // eight columns, where GNU as output puts a mnemonic
	opCol   = 8          // mnemonic field width before the operands
	maxBlank = 2         // a formatter must not let whitespace accumulate
)

// Print renders a unit as GNU as source.
func Print(u *text.Unit) ([]byte, error) {
	p := &printer{}
	for _, n := range u.Nodes {
		p.node(n)
	}
	p.trivia(u.Tail, "")
	return []byte(p.b.String()), p.err
}

// PrintInst renders one instruction, for arc dis and arc enc.
func PrintInst(i *text.Inst) string {
	p := &printer{}
	p.inst(i, "")
	return strings.TrimRight(strings.TrimPrefix(p.b.String(), indent), "\n")
}

type printer struct {
	b   strings.Builder
	err error

	// pending holds a label that the next statement shares a line with.
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
		p.trivia(tv, "")
		if d.Attached {
			p.pending = d.Name + ":"
			return
		}
		p.line(d.Name+":", tv)

	case *text.SectionDecl:
		p.trivia(tv, "")
		p.line(p.section(d), tv)

	case *text.SymbolDecl:
		p.trivia(tv, "")
		p.line(p.symbol(d), tv)

	case *text.Equ:
		p.trivia(tv, "")
		p.line(field(".equ")+d.Name+", "+p.expr(d.Value, precOr), tv)

	case *text.Data:
		p.trivia(tv, "")
		p.data(d, tv)

	case *text.Fill:
		p.trivia(tv, "")
		p.line(p.fill(d), tv)

	case *text.Align:
		p.trivia(tv, "")
		p.line(p.align(d), tv)

	case *text.Inst:
		p.trivia(tv, "")
		p.inst(d, "")
		p.comment(tv)

	default:
		p.fail("gas: cannot print %T", n)
	}
}

// trivia writes the blank lines and leading comments before a statement.
func (p *printer) trivia(tv text.Trivia, pre string) {
	n := tv.Blanks
	if n > maxBlank {
		n = maxBlank
	}
	for i := 0; i < n; i++ {
		p.b.WriteByte('\n')
	}
	for _, c := range tv.Before {
		p.b.WriteString(pre)
		p.b.WriteString("#")
		if c != "" {
			p.b.WriteString(" " + c)
		}
		p.b.WriteByte('\n')
	}
}

// line writes one statement, with the pending label on the same line when one
// is waiting.
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
		p.b.WriteString("   #")
		if tv.Line != "" {
			p.b.WriteString(" " + tv.Line)
		}
	}
	p.b.WriteByte('\n')
}

// field pads a directive or mnemonic to the operand column.
func field(s string) string {
	if len(s) >= opCol {
		return s + " "
	}
	return s + strings.Repeat(" ", opCol-len(s))
}

func (p *printer) section(d *text.SectionDecl) string {
	if d.Short {
		return strings.TrimRight(field(d.Name), " ")
	}
	s := field(".section") + d.Name
	if d.Flags != "" {
		s += ", " + quote(d.Flags)
	}
	if d.Type != "" {
		s += ", @" + d.Type
	}
	return s
}

func (p *printer) symbol(d *text.SymbolDecl) string {
	if d.Size != nil {
		return field(".size") + d.Names[0] + ", " + p.expr(d.Size, precOr)
	}
	if d.Type != text.TypeNone {
		return field(".type") + d.Names[0] + ", @" + d.Type.String()
	}
	for name, a := range symbolAttrs {
		// .global and .func are the aliases; the canonical spellings are the
		// short ones, which is what a formatter should converge on.
		if name == ".global" || name == ".func" {
			continue
		}
		if d.Attrs&a != 0 {
			return field(name) + strings.Join(d.Names, ", ")
		}
	}
	p.fail("gas: symbol declaration with no attribute: %v", d.Names)
	return ""
}

// data writes a Data node, splitting runs of strings from runs of numbers
// because GNU as has no directive that takes both.
func (p *printer) data(d *text.Data, tv text.Trivia) {
	i := 0
	for i < len(d.Items) {
		j := i
		str := d.Items[i].IsStr
		for j < len(d.Items) && d.Items[j].IsStr == str {
			if str && d.Items[j].Terminated != d.Items[i].Terminated {
				break
			}
			j++
		}

		var parts []string
		for _, it := range d.Items[i:j] {
			if str {
				parts = append(parts, quote(it.Str))
			} else {
				parts = append(parts, p.expr(it.X, precOr))
			}
		}

		name := widthDirective(d.Width)
		if str {
			name = ".ascii"
			if d.Items[i].Terminated {
				name = ".asciz"
			}
		}
		p.open()
		p.b.WriteString(field(name) + strings.Join(parts, ", "))

		i = j
		if i == len(d.Items) {
			p.comment(tv)
		} else {
			p.b.WriteByte('\n')
		}
	}
}

func widthDirective(w text.Width) string {
	switch w {
	case text.Width8:
		return ".byte"
	case text.Width16:
		return ".word"
	case text.Width32:
		return ".long"
	case text.Width64:
		return ".quad"
	case text.Width128:
		return ".octa"
	}
	return ".byte"
}

func (p *printer) fill(d *text.Fill) string {
	if d.Size == text.Width8 && d.Value == nil {
		return field(".zero") + p.expr(d.Count, precOr)
	}
	s := field(".fill") + p.expr(d.Count, precOr) + ", " + fmt.Sprint(d.Size.Bytes())
	if d.Value != nil {
		s += ", " + p.expr(d.Value, precOr)
	}
	return s
}

func (p *printer) align(d *text.Align) string {
	name, arg := ".align", p.expr(d.Bytes, precOr)
	if d.P2 {
		n, ok := d.Bytes.(*text.Int)
		if !ok {
			p.fail("gas: .p2align with a non-constant boundary")
			return ""
		}
		e := 0
		for v := n.Value; v > 1; v >>= 1 {
			e++
		}
		name, arg = ".p2align", fmt.Sprint(e)
	}
	s := field(name) + arg
	switch {
	case d.Value != nil && d.Max != nil:
		s += ", " + p.expr(d.Value, precOr) + ", " + p.expr(d.Max, precOr)
	case d.Value != nil:
		s += ", " + p.expr(d.Value, precOr)
	case d.Max != nil:
		s += ", , " + p.expr(d.Max, precOr)
	}
	return s
}

func (p *printer) inst(in *text.Inst, _ string) {
	p.open()

	name := printedMnemonic(in.Mnemonic, in.Size)
	if in.Prefix != text.PrefixNone {
		name = in.Prefix.String() + " " + name
	}

	if len(in.Ops) == 0 {
		p.b.WriteString(strings.TrimRight(field(name), " "))
		return
	}

	ops := reverse(in.Mnemonic, in.Ops)
	parts := make([]string, len(ops))
	for i, o := range ops {
		parts[i] = p.operand(o)
	}
	p.b.WriteString(field(name) + strings.Join(parts, ", "))
}

func (p *printer) operand(o text.Operand) string {
	switch v := o.(type) {
	case text.Reg:
		return "%" + regName(v.R)

	case text.Imm:
		// An immediate carries $ and a branch target does not. The two share
		// a type because the syntax does not distinguish them and the form
		// does — so the printer asks the same question the parser did.
		return "$" + p.expr(v.X, precOr)

	case text.Indirect:
		return "*" + p.operand(v.X)

	case text.Mem:
		return p.mem(v)
	}
	p.fail("gas: cannot print operand %T", o)
	return ""
}

// printBranch renders a rel operand without the $ sigil. It is separate from
// operand because only the instruction knows which slot the operand landed
// in, and the printer follows the parser's own test.
func (p *printer) printBranch(o text.Operand) string {
	if v, ok := o.(text.Imm); ok {
		return p.expr(v.X, precOr)
	}
	return p.operand(o)
}

func (p *printer) mem(m text.Mem) string {
	var b strings.Builder
	if m.HasSeg {
		b.WriteString("%" + m.Seg.Name() + ":")
	}
	if m.Disp != nil {
		b.WriteString(p.expr(m.Disp, precOr))
	}
	if !m.HasBase && !m.HasIndex {
		return b.String()
	}
	b.WriteByte('(')
	if m.HasBase {
		b.WriteString("%" + m.Base.Name())
	}
	if m.HasIndex {
		b.WriteString(fmt.Sprintf(",%%%s,%d", m.Index.Name(), m.Scale))
	}
	b.WriteByte(')')
	return b.String()
}

// regName is this dialect's spelling of a register. reg/ holds the psABI's
// bare names; the parentheses of st(0) and the db spelling of a debug
// register are syntax and live here.
func regName(r reg.Reg) string {
	name := r.Name()
	switch v := r.(type) {
	case reg.St:
		return fmt.Sprintf("st(%d)", v.Num())
	case reg.Dr:
		return "db" + name[2:]
	}
	return name
}

// expr renders an expression, parenthesising by this dialect's own precedence.
//
// The tree carries no precedence, so a tree parsed from NASM prints here with
// whatever parentheses GNU as's binding requires to mean the same number.
// That is what makes `1|2+3` survive a translation in either direction.
func (p *printer) expr(e text.Expr, outer int) string {
	switch v := e.(type) {
	case *text.Int:
		return fmt.Sprint(v.Value)

	case *text.SymExpr:
		if v.Mod == text.ModNone {
			return v.Name
		}
		return v.Name + "@" + v.Mod.String()

	case *text.Here:
		return "."

	case *text.Start:
		// GNU as has no one-token spelling for the section start. The
		// assembler resolves the empty name to the current section's symbol,
		// and printing it needs that name, which a printer does not have.
		p.fail("gas: $$ has no GNU as spelling; rewrite it against a label")
		return ""

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
	p.fail("gas: cannot print expression %T", e)
	return ""
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		case '\r':
			b.WriteString("\\r")
		default:
			if c < 0x20 || c >= 0x7f {
				b.WriteString(fmt.Sprintf("\\%03o", c))
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}