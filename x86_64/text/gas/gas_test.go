// x86_64/text/gas/gas_test.go
package gas

import (
	"strings"
	"testing"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
	"github.com/vertex-language/arc/x86_64/text"
)

func parse(t *testing.T, src string) *text.Unit {
	t.Helper()
	u, err := Parse("t.s", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func inst(t *testing.T, line string) *text.Inst {
	t.Helper()
	i, err := ParseInst(line)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// Bitwise binds tighter than additive in gas and looser in NASM. This is the
// difference the neutral tree cannot normalize away, so it had better be
// parsed the way gas parses it.
func TestBitwiseAboveAdditive(t *testing.T) {
	i := inst(t, "mov $1|2+3, %eax")
	e := i.Operands[1].Expr
	b, ok := e.(*text.Binary)
	if !ok || b.Op != text.OpAdd {
		t.Fatalf("top of tree is %#v, want an addition", e)
	}
	// (1|2)+3 = 3+3 = 6. If it parsed as C does, it would be 1|(2+3) = 5.
	v, err := text.Eval(e, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != 6 {
		t.Errorf("1|2+3 = %d, want 6", v)
	}
}

// Comparisons share a rank with + and -, which surprises everyone.
func TestComparisonSharesAdditivePrecedence(t *testing.T) {
	i := inst(t, "mov $1 < 2 + 3, %eax")
	// (1 < 2) + 3 = -1 + 3 = 2.
	v, err := text.Eval(i.Operands[1].Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Errorf("1 < 2 + 3 = %d, want 2", v)
	}
}

// A comparison is -1 for true; && and || are 1.
func TestTruthValues(t *testing.T) {
	for _, c := range []struct {
		src  string
		want int64
	}{
		{"$1 == 1", -1},
		{"$1 != 1", 0},
		{"$1 && 2", 1},
		{"$0 || 0", 0},
	} {
		i := inst(t, "mov "+c.src+", %eax")
		v, err := text.Eval(i.Operands[1].Expr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if v != c.want {
			t.Errorf("%s = %d, want %d", c.src, v, c.want)
		}
	}
}

// gas's infix ! is bitwise or-not. It desugars, because a neutral tree
// should not carry one dialect's compound operator.
func TestInfixBangIsOrNot(t *testing.T) {
	i := inst(t, "mov $0x0f!0xf0, %eax")
	v, err := text.Eval(i.Operands[1].Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(0x0f | ^0xf0); v != want {
		t.Errorf("0x0f!0xf0 = %#x, want %#x", v, want)
	}
}

// AT&T is source-first and the tree is destination-first. The reversal
// happens once, at the parser's edge.
func TestOperandOrderReversesAtTheEdge(t *testing.T) {
	i := inst(t, "movq $60, %rax")
	if len(i.Operands) != 2 {
		t.Fatalf("got %d operands", len(i.Operands))
	}
	if i.Operands[0].Kind != text.KindReg || i.Operands[0].Reg != reg.Reg(reg.RAX) {
		t.Errorf("destination is %v, want rax", i.Operands[0])
	}
	if i.Operands[1].Kind != text.KindImm {
		t.Errorf("source is %v, want an immediate", i.Operands[1])
	}
}

// The suffix is a size, and stripping it needs the ISA table: `call` ends in
// 'l' and is not `cal` with a long suffix.
func TestSuffixStrippingIsTableDriven(t *testing.T) {
	cases := []struct {
		src  string
		name string
		size operand.Width
	}{
		{"movq $1, %rax", "mov", operand.W64},
		{"movl $1, %eax", "mov", operand.W32},
		{"movb $1, %al", "mov", operand.W8},
		{"call foo", "call", operand.WidthNone},
		{"mov %ax, %bx", "mov", operand.WidthNone},
		{"push %rax", "push", operand.WidthNone},
		{"pushq %rax", "push", operand.W64},
	}
	for _, c := range cases {
		i := inst(t, c.src)
		if i.Mnemonic != c.name || i.Size != c.size {
			t.Errorf("%q → %s/%v, want %s/%v", c.src, i.Mnemonic, i.Size, c.name, c.size)
		}
	}
}

// The AT&T conversion names, and the condition-code aliases, resolve at the
// boundary and do not survive into the tree.
func TestAliasesResolveAndVanish(t *testing.T) {
	cases := map[string]string{
		"cltq":     "cdqe",
		"cqto":     "cqo",
		"cltd":     "cdq",
		"jz foo":   "je",
		"jnz foo":  "jne",
		"setc %al": "setb",
		"jnae foo": "jb",
	}
	for src, want := range cases {
		i := inst(t, src)
		if i.Mnemonic != want {
			t.Errorf("%q → %s, want %s", src, i.Mnemonic, want)
		}
	}
}

// movsbl is two suffixes: from byte, to long.
func TestExtendTakesTwoSuffixes(t *testing.T) {
	i := inst(t, "movsbl %al, %edx")
	if i.Mnemonic != "movsx" {
		t.Errorf("mnemonic = %s, want movsx", i.Mnemonic)
	}
	if i.Size != operand.W32 {
		t.Errorf("size = %v, want the destination width", i.Size)
	}
}

func TestAddressingForms(t *testing.T) {
	cases := []struct {
		src   string
		check func(*testing.T, text.MemRef)
	}{
		{"mov (%rbx), %rax", func(t *testing.T, m text.MemRef) {
			if !m.HasBase || m.Base != reg.RBX || m.Disp != nil {
				t.Errorf("got %+v", m)
			}
		}},
		{"mov 8(%rbx), %rax", func(t *testing.T, m text.MemRef) {
			if v, _ := text.Eval(m.Disp, nil); v != 8 {
				t.Errorf("disp = %v", m.Disp)
			}
		}},
		{"mov (%rbx,%rcx,4), %rax", func(t *testing.T, m text.MemRef) {
			if !m.HasIndex || m.Index != reg.RCX || m.Scale != 4 {
				t.Errorf("got %+v", m)
			}
		}},
		{"mov (,%rcx,8), %rax", func(t *testing.T, m text.MemRef) {
			if m.HasBase || !m.HasIndex {
				t.Errorf("a baseless form has no base: %+v", m)
			}
		}},
		{"mov %fs:8(%rbx), %rax", func(t *testing.T, m text.MemRef) {
			if !m.HasSeg || m.Seg != reg.FS {
				t.Errorf("segment override lost: %+v", m)
			}
		}},
		{"lea msg(%rip), %rsi", func(t *testing.T, m text.MemRef) {
			if !m.RIP {
				t.Errorf("rip-relative not recognized: %+v", m)
			}
		}},
	}
	for _, c := range cases {
		i := inst(t, c.src)
		var m text.MemRef
		for _, o := range i.Operands {
			if o.Kind == text.KindMem {
				m = o.Mem
			}
		}
		t.Run(c.src, func(t *testing.T) { c.check(t, m) })
	}
}

// The star is the difference between an indirect branch and a direct one.
func TestIndirectBranch(t *testing.T) {
	i := inst(t, "jmp *%rax")
	if !i.Operands[0].Indirect {
		t.Error("the * must survive: jmp %rax is not an instruction")
	}
	d := inst(t, "jmp foo")
	if d.Operands[0].Kind != text.KindTarget {
		t.Errorf("a bare symbol after a branch is a target, got %v", d.Operands[0].Kind)
	}
}

// A bare symbol is a target for a branch and an absolute load otherwise.
// Which one is isa/'s fact, not the syntax's.
func TestBareSymbolDependsOnTheInstruction(t *testing.T) {
	if inst(t, "call puts").Operands[0].Kind != text.KindTarget {
		t.Error("call takes a target")
	}
	if inst(t, "mov msg, %eax").Operands[1].Kind != text.KindMem {
		t.Error("mov from a bare symbol is a load, not an immediate")
	}
}

func TestModifiersFold(t *testing.T) {
	i := inst(t, "call puts@PLT")
	s, ok := i.Operands[0].Expr.(*text.Sym)
	if !ok {
		t.Fatalf("operand is %T", i.Operands[0].Expr)
	}
	if s.Name != "puts" || s.Mod != text.ModPLT {
		t.Errorf("got %s/%v", s.Name, s.Mod)
	}
}

// Comments and blank lines survive, because a formatter that dropped them is
// a formatter nobody runs twice.
func TestCommentsAndBlanksSurvive(t *testing.T) {
	u := parse(t, "# leading\n\n\n\tret\t# trailing\n")
	var comments, blanks int
	for _, n := range u.Nodes {
		switch v := n.(type) {
		case *text.Comment:
			comments++
		case *text.Blank:
			if v.Lines < 1 {
				t.Error("a blank run must count its lines")
			}
			blanks++
		}
	}
	if comments != 1 || blanks != 1 {
		t.Errorf("comments=%d blanks=%d", comments, blanks)
	}
	if got := u.Insts()[0].Comment; got != "# trailing" {
		t.Errorf("trailing comment = %q", got)
	}
}

// The round trip, at the level this package can check on its own: parse,
// print, parse, and compare the trees. Byte identity against GNU as is the
// differential suite's job, because only it can run gas.
func TestRoundTrip(t *testing.T) {
	src := `	.section .text
	.globl _start
	.type _start, @function
_start:
	movq    $60, %rax
	xorq    %rdi, %rdi
	movq    $1, %rax
	leaq    msg(%rip), %rsi
	call    puts@PLT
	movq    %fs:8(%rbx), %rax
	movl    (%rbx,%rcx,4), %eax
	ret

	.section .rodata
msg:
	.ascii  "hello, silicon\n"
	.quad   0x1000
	.p2align 4
`
	u1 := parse(t, src)
	out, err := Print(u1)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := Parse("t.s", out)
	if err != nil {
		t.Fatalf("reprinting produced something unparsable:\n%s\n%v", out, err)
	}

	i1, i2 := u1.Insts(), u2.Insts()
	if len(i1) != len(i2) {
		t.Fatalf("%d instructions became %d", len(i1), len(i2))
	}
	for n := range i1 {
		if i1[n].String() != i2[n].String() {
			t.Errorf("instruction %d changed:\n  %s\n  %s", n, i1[n], i2[n])
		}
	}
}

// Macros are a language and languages are out. Refusing by name is more
// useful than a parse error on a token.
func TestMacrosAreRefusedByName(t *testing.T) {
	for _, src := range []string{".macro foo\n.endm\n", ".rept 4\nnop\n.endr\n", ".if 1\nnop\n.endif\n"} {
		_, err := Parse("t.s", []byte(src))
		if err == nil {
			t.Errorf("%q must be refused", src)
			continue
		}
		if !strings.Contains(err.Error(), "macro") {
			t.Errorf("%q → %v, want a message naming macros", src, err)
		}
	}
}

func TestNumericLabels(t *testing.T) {
	u := parse(t, "1:\n\tjmp 1b\n")
	l, ok := u.Nodes[0].(*text.Label)
	if !ok || !l.Numeric {
		t.Fatalf("first node is %#v", u.Nodes[0])
	}
	s := u.Insts()[0].Operands[0].Expr.(*text.Sym)
	if s.Name != "1" || !s.Backward {
		t.Errorf("1b → %+v", s)
	}
}

func TestNumberBasesSurvive(t *testing.T) {
	i := inst(t, "mov $0x10, %eax")
	n := i.Operands[1].Expr.(*text.Num)
	if n.Value != 16 || n.Base != 16 {
		t.Errorf("got %d base %d", n.Value, n.Base)
	}
	// Printing back keeps the base: rewriting 0x10 as 16 assembles the same
	// and reads differently, which is a change arc fmt must not make.
	if s := printExpr(n); s != "0x10" {
		t.Errorf("printed as %q", s)
	}
}