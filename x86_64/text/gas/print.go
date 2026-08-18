// x86_64/text/gas/print.go
package gas

import (
	"strconv"
	"strings"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/text"
)

// Column layout. gas sources are read by people and gcc's own output uses a
// tab; matching it is what makes `arc fmt --check` pass on a compiler's
// output rather than rewriting every file in the tree.
const (
	indent   = "\t"
	mnemonicWidth = 7
)

// Print renders a unit as gas source.
//
// Everything Print changes assembles to identical bytes. The changes it does
// make are whitespace, the parentheses gas's precedence requires, and the
// one desugaring of gas's infix '!'.
func Print(u *text.Unit) ([]byte, error) {
	var b strings.Builder
	for _, n := range u.Nodes {
		switch v := n.(type) {
		case *text.Label:
			b.WriteString(v.Name + ":")
			if v.Comment != "" {
				b.WriteString(" " + v.Comment)
			}
			b.WriteString("\n")

		case *text.Comment:
			b.WriteString(v.Text + "\n")

		case *text.Blank:
			for i := 0; i < v.Lines; i++ {
				b.WriteString("\n")
			}

		case *text.Directive:
			b.WriteString(indent + printDirective(v))
			if v.Comment != "" {
				b.WriteString(" " + v.Comment)
			}
			b.WriteString("\n")

		case *text.Inst:
			s, err := PrintInst(v)
			if err != nil {
				return nil, err
			}
			b.WriteString(indent + s)
			if v.Comment != "" {
				b.WriteString(" " + v.Comment)
			}
			b.WriteString("\n")
		}
	}
	return []byte(b.String()), nil
}

// PrintInst renders one instruction in AT&T syntax.
func PrintInst(i *text.Inst) (string, error) {
	var b strings.Builder

	for _, p := range i.Prefixes {
		b.WriteString(p.String() + " ")
	}

	m := printMnemonic(i)
	b.WriteString(m)

	if len(i.Operands) == 0 {
		return b.String(), nil
	}
	for n := len(m); n < mnemonicWidth; n++ {
		b.WriteString(" ")
	}
	b.WriteString(" ")

	// Back into AT&T order: the tree is Intel order and gas writes source
	// first. The reversal is symmetric with the parser's, which is what
	// makes the round trip hold rather than merely usually hold.
	ops := make([]*text.Operand, len(i.Operands))
	copy(ops, i.Operands)
	reverse(ops)

	for n, o := range ops {
		if n > 0 {
			b.WriteString(", ")
		}
		s, err := printOperand(o, i)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func printOperand(o *text.Operand, i *text.Inst) (string, error) {
	var s string

	switch o.Kind {
	case text.KindReg:
		s = "%" + o.Reg.Name()

	case text.KindImm:
		s = "$" + printExpr(o.Expr)

	case text.KindTarget:
		s = printExpr(o.Expr)

	case text.KindMem:
		s = printMem(o)

	default:
		return "", text.Errorf(o.Position, "cannot print operand")
	}

	if o.Indirect {
		// The star is not decoration: `jmp *%rax` is an indirect branch and
		// `jmp %rax` is not an instruction at all.
		s = "*" + s
	}

	// Decorations follow the operand they qualify. The destination is the
	// last operand in AT&T order, which is where the mask goes.
	if o.Mask != 0 {
		s += "{%" + o.Mask.Name() + "}"
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

func printMem(o *text.Operand) string {
	m := o.Mem
	var b strings.Builder

	if m.HasSeg {
		b.WriteString("%" + m.Seg.Name() + ":")
	}
	if m.Disp != nil {
		b.WriteString(printExpr(m.Disp))
	}

	if !m.HasBase && !m.HasIndex && !m.RIP {
		// An absolute reference with no registers has no parentheses.
		return b.String()
	}

	b.WriteString("(")
	switch {
	case m.RIP:
		b.WriteString("%rip")
	case m.HasBase:
		b.WriteString("%" + baseName(m))
	}
	if m.HasIndex {
		b.WriteString(",%" + indexName(m))
		if m.Scale != 0 {
			b.WriteString("," + strconv.FormatInt(m.Scale, 10))
		}
	}
	b.WriteString(")")
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
	case 2: // L256
		bits = 256
	case 3: // L512
		bits = 512
	}
	return bits / (int(f.Elem) * 8)
}

func quote(s string) string { return strconv.Quote(s) }
func dec(v int64) string    { return strconv.FormatInt(v, 10) }

func hexDigits(v uint64) string { return strconv.FormatUint(v, 16) }
func octDigits(v uint64) string { return strconv.FormatUint(v, 8) }
func binDigits(v uint64) string { return strconv.FormatUint(v, 2) }

func reg32Name(r interface{ Name() string }) string { return r.Name() }

var _ = operand.W64