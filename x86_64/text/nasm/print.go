// x86_64/text/nasm/print.go
package nasm

import (
	"strconv"
	"strings"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/text"
)

// Column layout. NASM's own examples indent with whitespace and put labels
// in column one; matching that is what makes `arc fmt --check` pass on
// hand-written sources rather than rewriting every file in the tree.
const (
	indent        = "\t"
	mnemonicWidth = 7
)

// printer is one Print in progress.
//
// It carries two pieces of state that a per-node printer could not have. The
// section, because `resb` is legal only in a nobits section and `db` only
// outside one, so the same neutral Zero has two spellings. And the symbol
// tables, because NASM writes a symbol's type and size on its global
// directive while the neutral tree holds them as separate statements — so
// printing has to join what parsing split.
type printer struct {
	section string
	nobits  bool

	global map[string]bool
	typeOf map[string]string
	sizeOf map[string]text.Expr
	merged map[*text.Directive]bool
}

func newPrinter(u *text.Unit) *printer {
	pr := &printer{
		global: map[string]bool{},
		typeOf: map[string]string{},
		sizeOf: map[string]text.Expr{},
		merged: map[*text.Directive]bool{},
	}

	for _, n := range u.Nodes {
		d, ok := n.(*text.Directive)
		if !ok || d.Kind != text.Global {
			continue
		}
		for _, s := range d.Symbols() {
			pr.global[s] = true
		}
	}

	for _, n := range u.Nodes {
		d, ok := n.(*text.Directive)
		if !ok {
			continue
		}
		syms := d.Symbols()
		if len(syms) == 0 || !pr.global[syms[0]] {
			continue
		}
		switch d.Kind {
		case text.Type:
			if len(syms) >= 2 {
				t, err := text.ParseSymbolType(syms[1])
				if err != nil {
					continue
				}
				pr.typeOf[syms[0]] = t.String()
				pr.merged[d] = true
			}
		case text.Size:
			if len(d.Args) >= 2 {
				pr.sizeOf[syms[0]] = d.Args[1]
				pr.merged[d] = true
			}
		}
	}
	return pr
}

// Print renders a unit as NASM source.
//
// Everything Print changes assembles to identical bytes. The changes it does
// make are whitespace, the parentheses NASM's precedence requires, the
// canonical spelling of a number's base, and the joining of a symbol's type
// onto its global directive.
func Print(u *text.Unit) ([]byte, error) {
	pr := newPrinter(u)
	var b strings.Builder

	for _, n := range u.Nodes {
		switch v := n.(type) {
		case *text.Label:
			if v.Numeric {
				return nil, text.Errorf(v.Position,
					"NASM has no numeric labels; %s: has no spelling here", v.Name)
			}
			b.WriteString(v.Name + ":")
			if v.Comment != "" {
				b.WriteString(" " + v.Comment)
			}
			b.WriteString("\n")

		case *text.Comment:
			b.WriteString(commentText(v.Text) + "\n")

		case *text.Blank:
			for i := 0; i < v.Lines; i++ {
				b.WriteString("\n")
			}

		case *text.Directive:
			if pr.merged[v] {
				continue
			}
			pr.enter(v)
			s, err := pr.directive(v)
			if err != nil {
				return nil, err
			}
			b.WriteString(indent + s)
			if v.Comment != "" {
				b.WriteString(" " + commentText(v.Comment))
			}
			b.WriteString("\n")

		case *text.Inst:
			s, err := PrintInst(v)
			if err != nil {
				return nil, err
			}
			b.WriteString(indent + s)
			if v.Comment != "" {
				b.WriteString(" " + commentText(v.Comment))
			}
			b.WriteString("\n")
		}
	}
	return []byte(b.String()), nil
}

// enter records the section a later reservation will be printed inside.
func (pr *printer) enter(d *text.Directive) {
	if d.Kind != text.Section {
		return
	}
	pr.section = d.SectionName()
	pr.nobits = pr.section == ".bss" ||
		strings.Contains(strings.ToLower(d.Str), "nobits")
}

// commentText converts a comment to NASM's comment character. gas writes '#'
// and NASM writes ';', and a comment that kept the other dialect's marker
// would be an instruction the next parser could not read.
func commentText(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, ";") {
		return s
	}
	return ";" + strings.TrimPrefix(s, "#")
}

// PrintInst renders one instruction in Intel syntax.
func PrintInst(i *text.Inst) (string, error) {
	var b strings.Builder

	for _, p := range i.Prefixes {
		b.WriteString(p.String() + " ")
	}
	b.WriteString(i.Mnemonic)

	if len(i.Operands) == 0 {
		return b.String(), nil
	}
	for n := len(i.Mnemonic); n < mnemonicWidth; n++ {
		b.WriteString(" ")
	}
	b.WriteString(" ")

	// The tree is already in Intel order, so there is nothing to reverse.
	size := sizeKeyword(i)
	for n, o := range i.Operands {
		if n > 0 {
			b.WriteString(", ")
		}
		s, err := printOperand(o, i, size)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

// sizeKeyword is the width NASM has to write on the memory operand, or
// WidthNone when an operand settles it on its own.
//
// This is gas's needsSuffix asking the same question and putting the answer
// somewhere else: `mov qword [rbx], 1` here is `movq $1, (%rbx)` there, and
// the width that neither source states is the form's. That is why arc fmt
// resolves before it prints — a text-level translator has to invent the size
// going one way and drop it going the other.
func sizeKeyword(i *text.Inst) operand.Width {
	sawMem := false
	for _, o := range i.Operands {
		switch o.Kind {
		case text.KindReg:
			return operand.WidthNone
		case text.KindMem:
			sawMem = true
		}
	}
	if !sawMem {
		return operand.WidthNone
	}
	if i.Size != operand.WidthNone {
		return i.Size
	}
	if i.Form != nil {
		return formWidth(i.Form)
	}
	return operand.WidthNone
}

func formWidth(f *isa.Form) operand.Width {
	for _, s := range f.Slots {
		if s.Implicit {
			continue
		}
		if b := s.Class.Bits(); b > 0 && b <= 512 {
			return operand.Width(b)
		}
	}
	return operand.WidthNone
}

func printOperand(o *text.Operand, i *text.Inst, size operand.Width) (string, error) {
	var s string

	switch o.Kind {
	case text.KindReg:
		s = o.Reg.Name()

	case text.KindImm:
		s = printExpr(o.Expr)

	case text.KindTarget:
		s = printExpr(o.Expr)

	case text.KindMem:
		w := o.Size
		if w == operand.WidthNone {
			w = size
		}
		if k := widthKeyword(w); k != "" {
			s = k + " " + printMem(o)
		} else {
			s = printMem(o)
		}

	default:
		return "", text.Errorf(o.Position, "cannot print operand")
	}

	// No star. An indirect branch is written the same as a direct one in
	// NASM and the difference is the operand's kind, which is why the flag
	// is on the operand and read only by the dialect that spells it.

	if o.Mask != 0 {
		s += "{" + o.Mask.Name() + "}"
		if o.Zero {
			s += "{z}"
		}
	}
	if o.Broadcast && i.Form != nil {
		s += "{1to" + strconv.Itoa(broadcastCount(i)) + "}"
	}
	if r := o.Round.String(); r != "" {
		s += "{" + r + "}"
	}
	if o.SAE {
		s += "{sae}"
	}
	return s, nil
}

func widthKeyword(w operand.Width) string {
	switch w {
	case operand.W8:
		return "byte"
	case operand.W16:
		return "word"
	case operand.W32:
		return "dword"
	case operand.W64:
		return "qword"
	case operand.W128:
		return "oword"
	case operand.W256:
		return "yword"
	case operand.W512:
		return "zword"
	}
	return ""
}

func printMem(o *text.Operand) string {
	m := o.Mem
	var b strings.Builder
	b.WriteString("[")

	if m.HasSeg {
		b.WriteString(m.Seg.Name() + ":")
	}
	if m.RIP {
		b.WriteString("rel ")
	}

	wrote := false
	if m.HasBase {
		b.WriteString(baseName(m))
		wrote = true
	}
	if m.HasIndex {
		if wrote {
			b.WriteString("+")
		}
		b.WriteString(indexName(m))
		if m.Scale > 1 {
			b.WriteString("*" + strconv.FormatInt(m.Scale, 10))
		}
		wrote = true
	}
	if m.Disp != nil {
		d := printExpr(m.Disp)
		switch {
		case !wrote:
			b.WriteString(d)
		case strings.HasPrefix(d, "-"):
			b.WriteString(d)
		default:
			b.WriteString("+" + d)
		}
		wrote = true
	}
	if !wrote {
		b.WriteString("0")
	}

	b.WriteString("]")
	return b.String()
}

func baseName(m text.MemRef) string {
	if m.Addr32 {
		return reg32Name(m.Base)
	}
	return m.Base.Name()
}

func indexName(m text.MemRef) string {
	if m.Addr32 {
		return reg32Name(m.Index)
	}
	return m.Index.Name()
}

func broadcastCount(i *text.Inst) int {
	f := i.Form
	if f == nil || f.Elem == 0 {
		return 0
	}
	bits := 128
	switch f.Len {
	case isa.L256:
		bits = 256
	case isa.L512:
		bits = 512
	}
	return bits / (int(f.Elem) * 8)
}

// quote renders a string in NASM's backquoted form, which is the only one of
// the three that takes escapes and therefore the only one that can hold
// every byte sequence.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('`')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '`':
			b.WriteString("\\`")
		default:
			if c < 0x20 || c >= 0x7f {
				b.WriteString(`\x` + strings.ToUpper(strconv.FormatUint(uint64(c), 16)))
				continue
			}
			b.WriteByte(c)
		}
	}
	b.WriteByte('`')
	return b.String()
}

func dec(v int64) string { return strconv.FormatInt(v, 10) }

func hexDigits(v uint64) string { return strconv.FormatUint(v, 16) }
func octDigits(v uint64) string { return strconv.FormatUint(v, 8) }
func binDigits(v uint64) string { return strconv.FormatUint(v, 2) }

func reg32Name(r interface{ Name() string }) string { return r.Name() }