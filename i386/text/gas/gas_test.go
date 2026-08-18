package gas

import (
	"strings"
	"testing"

	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

func parse(t *testing.T, src string) *text.Unit {
	t.Helper()
	u, err := ParseFile("t.s", []byte(src))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return u
}

func printed(t *testing.T, u *text.Unit) string {
	t.Helper()
	b, err := Print(u)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	return string(b)
}

// The README's program, through parse and print and back. Print is parse's
// inverse at the semantic level, and the second parse producing the same tree
// is what that means operationally.
func TestRoundTrip(t *testing.T) {
	src := `# exit(60)
        .section .text
        .globl _start
        .type _start, @function
_start:
        mov     $60, %eax           # syscall number
        xor     %edi, %edi
        lea     msg(%ebx), %esi
        int     $0x80

        .section .rodata
msg:    .ascii "hello, silicon\n"
len:    .long   . - msg
`
	first := parse(t, src)
	out := printed(t, first)
	second := parse(t, out)

	if a, b := printed(t, first), printed(t, second); a != b {
		t.Errorf("not a fixed point:\n--- first ---\n%s\n--- second ---\n%s", a, b)
	}

	// Comments survive. A formatter that ate them would be one nobody runs.
	if !strings.Contains(out, "exit(60)") || !strings.Contains(out, "syscall number") {
		t.Errorf("comments lost:\n%s", out)
	}
	// So does the label written on the data's own line.
	if !strings.Contains(out, `msg:    .ascii "hello, silicon\n"`) &&
		!strings.Contains(out, `msg: .ascii "hello, silicon\n"`) {
		t.Errorf("attached label reflowed:\n%s", out)
	}
}

// Operands are stored in Intel order and reversed at both edges. One form
// table serves both dialects because of this and nothing else.
func TestOperandOrderIsReversed(t *testing.T) {
	u := parse(t, "\tmov $60, %eax\n")
	in := u.Nodes[0].(*text.Inst)

	if in.Mnemonic != "mov" || in.Size != text.Width32 && in.Size != text.WidthNone {
		t.Fatalf("mnemonic %q size %v", in.Mnemonic, in.Size)
	}
	if len(in.Ops) != 2 {
		t.Fatalf("%d operands", len(in.Ops))
	}
	if r, ok := in.Ops[0].(text.Reg); !ok || r.R != reg.Reg(reg.EAX) {
		t.Errorf("operand 0 = %v, want the destination %%eax", in.Ops[0])
	}
	if _, ok := in.Ops[1].(text.Imm); !ok {
		t.Errorf("operand 1 = %T, want the immediate", in.Ops[1])
	}

	if out := printed(t, u); !strings.Contains(out, "mov     $60, %eax") {
		t.Errorf("print did not restore GAS order:\n%s", out)
	}
}

// The table decides what a suffix is, not the spelling. A rule written as
// "strip a trailing l" would eat shl, call and sal.
func TestSuffixSplitting(t *testing.T) {
	for _, c := range []struct {
		in   string
		mn   string
		size text.Width
	}{
		{"movl %eax, %ebx", "mov", text.Width32},
		{"movb %al, %bl", "mov", text.Width8},
		{"movw %ax, %bx", "mov", text.Width16},
		{"mov %eax, %ebx", "mov", text.WidthNone},
		{"shl %eax", "shl", text.WidthNone},
		{"sal %eax", "sal", text.WidthNone},
		{"mul %ecx", "mul", text.WidthNone},
		{"call *%eax", "call", text.WidthNone},
		{"bswap %eax", "bswap", text.WidthNone},
		{"movzbl %al, %ebx", "movzx", text.Width8},
		{"movswl %ax, %ebx", "movsx", text.Width16},
	} {
		u := parse(t, "\t"+c.in+"\n")
		in := u.Nodes[0].(*text.Inst)
		if in.Mnemonic != c.mn || in.Size != c.size {
			t.Errorf("%q → %q/%v, want %q/%v", c.in, in.Mnemonic, in.Size, c.mn, c.size)
		}
		if out := printed(t, u); !strings.Contains(out, strings.Fields(c.in)[0]) {
			t.Errorf("%q printed back without its suffix:\n%s", c.in, out)
		}
	}
}

// GNU as's precedence is not C's: the bitwise operators sit above addition.
// This is the reason precedence lives in each dialect and not in text/.
func TestPrecedenceIsGASNotC(t *testing.T) {
	for _, c := range []struct {
		expr string
		want int64
	}{
		{"1|2+3", 6},   // (1|2)+3 — NASM would answer 5
		{"1^1+1", 1},   // (1^1)+1
		{"2*3+4", 10},  // * binds tighter than + in both
		{"1+2*3", 7},
		{"8>>1+1", 5},  // (8>>1)+1 — C would answer 8>>2 = 2
		{"1+2==3", -1}, // comparison is loosest of the four, true is -1
		{"(1|2)+3", 6},
		{"1|(2+3)", 7},
		{"7&3", 3},
		{"-5+2", -3},
		{"~0", -1},
		{"0xff!0xf0", -241}, // GNU as's or-not: 0xff | ~0xf0
	} {
		u := parse(t, "\t.long "+c.expr+"\n")
		d := u.Nodes[0].(*text.Data)
		v, err := text.Eval(d.Items[0].X, nil)
		if err != nil {
			t.Errorf("%s: %v", c.expr, err)
			continue
		}
		if !v.IsAbs() || v.Const != c.want {
			t.Errorf("%s = %s, want %d", c.expr, v, c.want)
		}
	}
}

// A tree carries no parentheses, so the printer computes them. Reprinting and
// reparsing must give the same number.
func TestParenthesesAreComputed(t *testing.T) {
	for _, src := range []string{"1|2+3", "1|(2+3)", "1-(2-3)", "(1+2)*3", "1+2*3"} {
		u := parse(t, "\t.long "+src+"\n")
		want := mustEval(t, u)

		again := parse(t, printed(t, u))
		if got := mustEval(t, again); got != want {
			t.Errorf("%s: %d after reprint, %d before\n%s", src, got, want, printed(t, u))
		}
	}
}

func mustEval(t *testing.T, u *text.Unit) int64 {
	t.Helper()
	v, err := text.Eval(u.Nodes[0].(*text.Data).Items[0].X, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v.Const
}

// Every addressing mode operand/ can build, and nothing it cannot.
func TestMemoryOperands(t *testing.T) {
	for _, c := range []struct {
		in    string
		check func(*testing.T, text.Mem)
	}{
		{"(%eax)", func(t *testing.T, m text.Mem) {
			if !m.HasBase || m.Base != reg.EAX || m.HasIndex || m.Disp != nil {
				t.Errorf("= %+v", m)
			}
		}},
		{"8(%ecx)", func(t *testing.T, m text.Mem) {
			if !m.HasBase || m.Base != reg.ECX || m.Disp == nil {
				t.Errorf("= %+v", m)
			}
		}},
		{"(%eax,%ecx,4)", func(t *testing.T, m text.Mem) {
			if !m.HasIndex || m.Index != reg.ECX || m.Scale != 4 {
				t.Errorf("= %+v", m)
			}
		}},
		{"(,%ecx,8)", func(t *testing.T, m text.Mem) {
			if m.HasBase || !m.HasIndex || m.Scale != 8 {
				t.Errorf("= %+v", m)
			}
		}},
		{"0x1234", func(t *testing.T, m text.Mem) {
			if m.HasBase || m.HasIndex || m.Disp == nil {
				t.Errorf("= %+v", m)
			}
		}},
		{"%gs:(%eax)", func(t *testing.T, m text.Mem) {
			if !m.HasSeg || m.Seg != reg.GS || !m.HasBase {
				t.Errorf("= %+v", m)
			}
		}},
		{"msg(%ebx)", func(t *testing.T, m text.Mem) {
			if !m.HasBase || m.Base != reg.EBX || m.Disp == nil {
				t.Errorf("= %+v", m)
			}
		}},
	} {
		u := parse(t, "\tmov "+c.in+", %eax\n")
		in := u.Nodes[0].(*text.Inst)
		m, ok := in.Ops[1].(text.Mem)
		if !ok {
			t.Errorf("%q parsed as %T", c.in, in.Ops[1])
			continue
		}
		c.check(t, m)

		if out := printed(t, u); !strings.Contains(out, c.in) {
			t.Errorf("%q printed as:\n%s", c.in, out)
		}
	}
}

// operand/ rejects at construction what the encoder could not emit, and the
// parser should not accept more than operand/ can hold.
func TestRejectedAddressing(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"mov (%eax,%esp,4), %ebx", "esp cannot be an index"},
		{"mov (%eax,%ecx,3), %ebx", "scale 3"},
		{"mov (%ax), %ebx", "cannot be a base"},
		{"mov %rax, %rbx", "unknown register"},
		{"frobnicate %eax", "unknown instruction"},
	} {
		_, err := ParseFile("t.s", []byte("\t"+c.in+"\n"))
		if err == nil {
			t.Errorf("%q accepted", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want it to mention %q", c.in, err, c.want)
		}
	}
}

// The modifier is a word here and a number in the arch root. That split is
// what lets the same line be an ELF relocation on i386-elf and a diagnostic
// on i386-coff.
func TestRelocationModifiers(t *testing.T) {
	u := parse(t, "\tcall puts@PLT\n\tmov msg@GOTOFF(%ebx), %eax\n")

	call := u.Nodes[0].(*text.Inst)
	m := call.Ops[0].(text.Mem)
	sym := m.Disp.(*text.SymExpr)
	if sym.Name != "puts" || sym.Mod != text.ModPLT {
		t.Errorf("call operand = %+v", sym)
	}

	mov := u.Nodes[1].(*text.Inst)
	src := mov.Ops[1].(text.Mem)
	if s := src.Disp.(*text.SymExpr); s.Mod != text.ModGOTOFF {
		t.Errorf("GOTOFF lost: %+v", s)
	}

	if out := printed(t, u); !strings.Contains(out, "puts@PLT") || !strings.Contains(out, "msg@GOTOFF(%ebx)") {
		t.Errorf("modifiers not printed:\n%s", out)
	}

	if _, err := ParseFile("t.s", []byte("\tcall puts@GOTPCREL\n")); err == nil {
		t.Error("GOTPCREL accepted; it is x86-64's and i386 has no PC-relative mode")
	}
}

// Radix survives, because arc fmt rewrites in place and .byte 0x55 must not
// come back as .byte 85.
func TestRadixSurvives(t *testing.T) {
	src := "\t.byte 0x55, 0xAA, 85, 0b1010, 'a'\n"
	out := printed(t, parse(t, src))
	for _, want := range []string{"0x55", "0xaa", "85", "0b1010", "'a'"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q lost:\n%s", want, out)
		}
	}
}

// Directives are recognised in order to be refused. A program using .macro is
// one arc will never assemble, and saying which rule refuses it is more useful
// than saying the name is unknown.
func TestExcludedDirectives(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{".macro foo", "macro expander"},
		{".rept 4", "macro expander"},
		{".if 1", "conditional assembly"},
		{".include \"x.s\"", "compiler driver"},
		{".float 1.0", "float literals"},
		{".code16", "protected mode"},
		{".intel_syntax noprefix", "--dialect"},
	} {
		_, err := ParseFile("t.s", []byte("\t"+c.in+"\n"))
		if err == nil {
			t.Errorf("%q accepted", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want it to mention %q", c.in, err, c.want)
		}
	}
}

// A parser that stopped at the first error would make a file with two typos
// take two runs to fix.
func TestMultipleDiagnostics(t *testing.T) {
	_, err := ParseFile("t.s", []byte("\tfrobnicate %eax\n\tmov %rax, %rbx\n"))
	if err == nil {
		t.Fatal("no error")
	}
	if n := strings.Count(err.Error(), "t.s:"); n < 2 {
		t.Errorf("reported %d diagnostics, want at least 2:\n%v", n, err)
	}
	if !strings.Contains(err.Error(), "t.s:1:") || !strings.Contains(err.Error(), "t.s:2:") {
		t.Errorf("diagnostics are not positioned:\n%v", err)
	}
}

// ParseInst is the same statement parser with the file removed.
func TestParseInst(t *testing.T) {
	in, err := ParseInst("movl $60, %eax")
	if err != nil {
		t.Fatal(err)
	}
	if in.Mnemonic != "mov" || in.Size != text.Width32 || len(in.Ops) != 2 {
		t.Errorf("= %+v", in)
	}
	if got := PrintInst(in); got != "movl    $60, %eax" {
		t.Errorf("PrintInst = %q", got)
	}

	for _, bad := range []string{"", ".globl x", "mov $1, %eax; ret"} {
		if _, err := ParseInst(bad); err == nil {
			t.Errorf("ParseInst(%q) accepted", bad)
		}
	}
}

// The boot-sector idiom, which is why Start exists at all.
func TestFillAndAlign(t *testing.T) {
	u := parse(t, "\t.zero 64\n\t.fill 32, 4, 0xff\n\t.align 16\n\t.p2align 4\n")

	if f := u.Nodes[0].(*text.Fill); f.Size != text.Width8 || f.Value != nil {
		t.Errorf(".zero = %+v", f)
	}
	if f := u.Nodes[1].(*text.Fill); f.Size != text.Width32 || f.Value == nil {
		t.Errorf(".fill = %+v", f)
	}

	// Two spellings of one boundary; the exponent converts at parse and the
	// form that was written is what prints back.
	a := u.Nodes[2].(*text.Align)
	b := u.Nodes[3].(*text.Align)
	if av, _ := text.Eval(a.Bytes, nil); av.Const != 16 {
		t.Errorf(".align 16 → %s", av)
	}
	if bv, _ := text.Eval(b.Bytes, nil); bv.Const != 16 {
		t.Errorf(".p2align 4 → %s, want 16 bytes", bv)
	}
	out := printed(t, u)
	if !strings.Contains(out, ".align   16") || !strings.Contains(out, ".p2align 4") {
		t.Errorf("align forms not preserved:\n%s", out)
	}
}