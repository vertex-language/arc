// x86_64/text/nasm/nasm_test.go
package nasm

import (
	"strings"
	"testing"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
	"github.com/vertex-language/arc/x86_64/text"
)

func parse(t *testing.T, src string) *text.Unit {
	t.Helper()
	u, err := Parse("t.asm", []byte(src))
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

// Bitwise binds looser than additive in NASM and tighter in gas. This is the
// difference the neutral tree cannot normalize away.
func TestBitwiseBelowAdditive(t *testing.T) {
	i := inst(t, "mov eax, 1|2+3")
	v, err := text.Eval(i.Operands[1].Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 1|(2+3) = 5. Under gas's ranks it would be (1|2)+3 = 6.
	if v != 5 {
		t.Errorf("1|2+3 = %d, want 5", v)
	}
}

// '/' is unsigned here and '//' is signed. gas has only the signed one, and
// one operator with a flag would print back into the wrong dialect.
func TestDivisionOperators(t *testing.T) {
	s, err := text.Eval(inst(t, "mov eax, -8 // 2").Operands[1].Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s != -4 {
		t.Errorf("-8 // 2 = %d, want -4", s)
	}
	u, err := text.Eval(inst(t, "mov eax, -8 / 2").Operands[1].Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if u != int64(uint64(0xfffffffffffffff8)/2) {
		t.Errorf("-8 / 2 = %d", u)
	}
}

// Intel order is destination-first and so is the tree. Nothing reverses.
func TestOperandOrderIsUnchanged(t *testing.T) {
	i := inst(t, "mov rax, 60")
	if i.Operands[0].Kind != text.KindReg || i.Operands[0].Reg != reg.Reg(reg.RAX) {
		t.Errorf("destination is %v, want rax", i.Operands[0])
	}
	if i.Operands[1].Kind != text.KindImm {
		t.Errorf("source is %v, want an immediate", i.Operands[1])
	}
}

// The size is a keyword on the operand, not a suffix on the mnemonic.
func TestSizeKeyword(t *testing.T) {
	i := inst(t, "mov qword [rbx], 1")
	if i.Operands[0].Size != operand.W64 {
		t.Errorf("size = %v, want qword", i.Operands[0].Size)
	}
	if i.Mnemonic != "mov" {
		t.Errorf("mnemonic = %q", i.Mnemonic)
	}
}

// A bare symbol is an address here and a load in gas. Same three tokens,
// two different instructions.
func TestBareSymbolIsAnImmediate(t *testing.T) {
	if inst(t, "mov rax, msg").Operands[1].Kind != text.KindImm {
		t.Error("mov from a bare symbol is the symbol's address")
	}
	if inst(t, "mov rax, [msg]").Operands[1].Kind != text.KindMem {
		t.Error("brackets are the load")
	}
	if inst(t, "call puts").Operands[0].Kind != text.KindTarget {
		t.Error("call takes a target")
	}
}

// NASM writes no star, so an indirect branch is known by the operand's kind.
// gas's printer needs the flag; this one drops it.
func TestIndirectBranchIsMarked(t *testing.T) {
	if !inst(t, "jmp rax").Operands[0].Indirect {
		t.Error("jmp through a register is indirect")
	}
	if inst(t, "jmp foo").Operands[0].Indirect {
		t.Error("jmp to a label is not")
	}
}

func TestAddressingForms(t *testing.T) {
	cases := []struct {
		src   string
		check func(*testing.T, text.MemRef)
	}{
		{"mov rax, [rbx]", func(t *testing.T, m text.MemRef) {
			if !m.HasBase || m.Base != reg.RBX || m.Disp != nil {
				t.Errorf("got %+v", m)
			}
		}},
		{"mov rax, [rbx+8]", func(t *testing.T, m text.MemRef) {
			if v, _ := text.Eval(m.Disp, nil); v != 8 {
				t.Errorf("disp = %v", m.Disp)
			}
		}},
		{"mov rax, [rbx-8]", func(t *testing.T, m text.MemRef) {
			if v, _ := text.Eval(m.Disp, nil); v != -8 {
				t.Errorf("disp = %v", m.Disp)
			}
		}},
		{"mov rax, [rbx+rcx*4]", func(t *testing.T, m text.MemRef) {
			if !m.HasIndex || m.Index != reg.RCX || m.Scale != 4 {
				t.Errorf("got %+v", m)
			}
		}},
		{"mov rax, [4*rcx]", func(t *testing.T, m text.MemRef) {
			if m.HasBase || !m.HasIndex || m.Scale != 4 {
				t.Errorf("a scale written first is still an index: %+v", m)
			}
		}},
		{"mov rax, [fs:rbx+8]", func(t *testing.T, m text.MemRef) {
			if !m.HasSeg || m.Seg != reg.FS {
				t.Errorf("segment override lost: %+v", m)
			}
		}},
		{"lea rsi, [rel msg]", func(t *testing.T, m text.MemRef) {
			if !m.RIP {
				t.Errorf("rel not recognized: %+v", m)
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

// All six number spellings are the same number, and the base survives so a
// formatter does not rewrite 0x10 as 16.
func TestNumberSpellings(t *testing.T) {
	for _, src := range []string{"0x1f", "$1f", "1fh", "0h1f"} {
		i := inst(t, "mov eax, "+src)
		n, ok := i.Operands[1].Expr.(*text.Num)
		if !ok || n.Value != 31 {
			t.Errorf("%s → %v", src, i.Operands[1].Expr)
			continue
		}
		if n.Base != 16 {
			t.Errorf("%s lost its base", src)
		}
	}
	for _, src := range []string{"0b1010", "1010b", "0y1010"} {
		i := inst(t, "mov eax, "+src)
		if n := i.Operands[1].Expr.(*text.Num); n.Value != 10 {
			t.Errorf("%s = %d", src, n.Value)
		}
	}
}

// Only the backquoted string takes escapes. '\n' is two characters.
func TestOnlyBackquotesEscape(t *testing.T) {
	u := parse(t, "\tdb 'a\\nb'\n")
	d := u.Nodes[0].(*text.Directive)
	if d.Str != `a\nb` {
		t.Errorf("single quotes are raw: got %q", d.Str)
	}
	u = parse(t, "\tdb `a\\nb`\n")
	d = u.Nodes[0].(*text.Directive)
	if d.Str != "a\nb" {
		t.Errorf("backquotes escape: got %q", d.Str)
	}
}

// A label needs no colon, and a mnemonic at the start of a line is not one.
func TestLabelsWithoutColons(t *testing.T) {
	u := parse(t, "_start\n\tret\n")
	if _, ok := u.Nodes[0].(*text.Label); !ok {
		t.Fatalf("first node is %#v", u.Nodes[0])
	}
	if len(u.Insts()) != 1 {
		t.Errorf("got %d instructions", len(u.Insts()))
	}
	u = parse(t, "ret\n")
	if len(u.Insts()) != 1 || len(u.Defined()) != 0 {
		t.Error("a mnemonic in column one is an instruction")
	}
}

// The condition-code aliases resolve at the boundary and do not survive.
func TestAliasesResolveAndVanish(t *testing.T) {
	for src, want := range map[string]string{
		"jz foo":   "je",
		"jnz foo":  "jne",
		"jnae foo": "jb",
		"setc al":  "setb",
	} {
		if got := inst(t, src).Mnemonic; got != want {
			t.Errorf("%q → %s, want %s", src, got, want)
		}
	}
}

// `times` over data is a fill; over an instruction it is a repetition, which
// needs an expander.
func TestTimes(t *testing.T) {
	u := parse(t, "\ttimes 64 db 0\n")
	d := u.Nodes[0].(*text.Directive)
	if d.Kind != text.Fill || len(d.Args) != 3 {
		t.Fatalf("got %v", d)
	}
	if _, err := Parse("t.asm", []byte("\ttimes 4 nop\n")); err == nil {
		t.Error("times before an instruction must be refused")
	}
}

// The preprocessor is a language and languages are out.
func TestPreprocessorIsRefusedByName(t *testing.T) {
	for _, src := range []string{"%macro foo 0\n%endmacro\n", "%if 1\nnop\n%endif\n", "%define X 1\n"} {
		_, err := Parse("t.asm", []byte(src))
		if err == nil {
			t.Errorf("%q must be refused", src)
			continue
		}
		if !strings.Contains(err.Error(), "macro") {
			t.Errorf("%q → %v, want a message naming macros", src, err)
		}
	}
}

// `default rel` changes the bytes of every later memory operand and the tree
// carries no mode, so it is refused rather than accepted and ignored.
func TestDefaultRelIsRefused(t *testing.T) {
	_, err := Parse("t.asm", []byte("\tdefault rel\n"))
	if err == nil || !strings.Contains(err.Error(), "rel") {
		t.Errorf("got %v", err)
	}
}

// wrt folds to the neutral modifier, the same one gas's @PLT folds to.
func TestWRTFolds(t *testing.T) {
	i := inst(t, "call puts wrt ..plt")
	s, ok := i.Operands[0].Expr.(*text.Sym)
	if !ok {
		t.Fatalf("operand is %T", i.Operands[0].Expr)
	}
	if s.Name != "puts" || s.Mod != text.ModPLT {
		t.Errorf("got %s/%v", s.Name, s.Mod)
	}
}

// The round trip, at the level this package can check on its own: parse,
// print, parse, compare. Byte identity against NASM is the differential
// suite's job, because only it can run NASM.
func TestRoundTrip(t *testing.T) {
	src := `	section .text
	global _start:function
_start:
	mov     rax, 60
	xor     rdi, rdi
	lea     rsi, [rel msg]
	call    puts wrt ..plt
	mov     rax, [fs:rbx+8]
	mov     eax, [rbx+rcx*4]
	mov     qword [rbx], 1
	ret

	section .rodata
msg:
	db      ` + "`hello, silicon\\n`" + `
	dq      0x1000
	align   16
`
	u1 := parse(t, src)
	out, err := Print(u1)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := Parse("t.asm", out)
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

// A reservation is a byte count in the tree and comes back as resb in a
// nobits section.
func TestReservationRoundTrips(t *testing.T) {
	u := parse(t, "\tsection .bss\nbuf:\n\tresq 4\n")
	out, err := Print(u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "resb") {
		t.Errorf("a .bss reservation prints as resb:\n%s", out)
	}
}