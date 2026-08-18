// x86_64/isa/isa_test.go
package isa

import (
	"errors"
	"strings"
	"testing"

	"github.com/vertex-language/arc/x86_64/feature"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// The generator names one method per form. Two forms with one name would
// silently drop an encoding from the typed layer, which is the one layer
// whose whole promise is that you get the bytes you asked for.
func TestGoNamesAreUnique(t *testing.T) {
	seen := map[string]*Form{}
	for _, f := range All() {
		n := f.GoName()
		if prev, ok := seen[n]; ok {
			t.Errorf("%s names both %q and %q", n, prev.Opcodes(), f.Opcodes())
			continue
		}
		seen[n] = f
	}
}

// `mov rax, 60` has two encodings and the seven-byte one wins. This is the
// example in the README, and the reason MovR64Imm64 exists as a separate
// entry point: Resolve will not give you the imm64 form for a value that
// fits in imm32.
func TestResolvePicksTheShorterMov(t *testing.T) {
	f, err := Resolve(feature.Baseline(), "mov",
		RegArg(reg.RAX), ImmArg(operand.Imm(60)))
	if err != nil {
		t.Fatal(err)
	}
	if f.Opcode != 0xc7 || f.Ext != 0 {
		t.Errorf("got %s (%s), want C7 /0 id", f, f.Opcodes())
	}

	// A value that does not fit leaves only one candidate.
	f, err = Resolve(feature.Baseline(), "mov",
		RegArg(reg.RAX), ImmArg(operand.Imm(1)<<40))
	if err != nil {
		t.Fatal(err)
	}
	if f.Opcode != 0xb8 || f.Imm != ImmQ {
		t.Errorf("got %s (%s), want B8+r io", f, f.Opcodes())
	}
}

// 83 /0 ib is four bytes shorter than 81 /0 id and both are legal.
func TestResolvePrefersSignExtendedImm8(t *testing.T) {
	f, err := Resolve(feature.Baseline(), "add",
		RegArg(reg.RAX), ImmArg(operand.Imm(1)))
	if err != nil {
		t.Fatal(err)
	}
	if f.Opcode != 0x83 {
		t.Errorf("got %s (%s), want 83 /0 ib", f, f.Opcodes())
	}
}

// A symbolic immediate has no value to narrow, and narrowing an unknown is
// how a four-byte fixup ends up in a one-byte field.
func TestSymbolicImmediateIsNotNarrowed(t *testing.T) {
	f, err := Resolve(feature.Baseline(), "mov",
		RegArg(reg.EAX), SymImmArg(operand.W32))
	if err != nil {
		t.Fatal(err)
	}
	if f.Imm != ImmD {
		t.Errorf("got %s, want an imm32 field", f.Opcodes())
	}
}

// The gating diagnostic has to name the feature that would have allowed the
// form, because the note line the root prints is built from it.
func TestGatingNamesTheFeature(t *testing.T) {
	_, err := Resolve(feature.Baseline(), "vpaddd",
		RegArg(reg.ZMM0), RegArg(reg.K1), RegArg(reg.ZMM1), RegArg(reg.ZMM2))
	var g *GateError
	if !errors.As(err, &g) {
		t.Fatalf("got %v, want a GateError", err)
	}
	if g.Need != feature.AVX512F {
		t.Errorf("Need = %s, want avx512f", g.Need)
	}
	if !strings.Contains(g.Error(), "avx512f") {
		t.Errorf("diagnostic does not name the feature: %v", g)
	}
}

// An unknown mnemonic and a known mnemonic with wrong operands are different
// errors, because they send the reader to different places.
func TestUnknownVersusNoForm(t *testing.T) {
	var u *UnknownError
	if _, err := Resolve(feature.Baseline(), "vaddq"); !errors.As(err, &u) {
		t.Errorf("got %v, want an UnknownError", err)
	}
	var f *FormError
	_, err := Resolve(feature.Baseline(), "lea", RegArg(reg.RAX), RegArg(reg.RBX))
	if !errors.As(err, &f) {
		t.Errorf("got %v, want a FormError", err)
	}
}

// LEA's source is memory that is never read, and there is no register form.
// The class is width-agnostic because the instruction computes an address.
func TestLeaTakesOnlyMemory(t *testing.T) {
	m := operand.RIPRel(operand.Label("msg"))
	if _, err := Resolve(feature.Baseline(), "lea", RegArg(reg.RSI), MemArg(m)); err != nil {
		t.Fatal(err)
	}
	if MAny.Match(RegArg(reg.RBX)) {
		t.Error("MAny must refuse a register")
	}
}

// A width-agnostic reference takes the slot's width. This is what makes one
// RIPRel value usable as both an address and a load.
func TestUnsizedMemoryMatchesAnyWidth(t *testing.T) {
	m := operand.RIPRel(operand.Label("msg"))
	for _, c := range []Class{RM8, RM32, RM64, XmmM128} {
		if !c.Match(MemArg(m)) {
			t.Errorf("%s must accept an unsized reference", c)
		}
	}
	// A sized one does not stretch.
	sized := operand.Mem64(reg.RBX).Mem
	if RM32.Match(MemArg(sized)) {
		t.Error("a 64-bit reference is not an r/m32")
	}
}

// Every EVEX form that can take memory needs a tuple, or its disp8 has no
// scale factor. finish() panics on the alternative; this proves the table
// currently satisfies it rather than that the check exists.
func TestEveryEVEXMemoryFormHasATuple(t *testing.T) {
	for _, f := range All() {
		if f.Enc == EncEVEX && f.MemSlot() >= 0 && f.Tuple == TupleNone {
			t.Errorf("%s (%s) states no tuple", f, f.Opcodes())
		}
	}
}

// The opcode column round-trips to something a reader can diff against the
// manual. This is the test that catches a transcription error in the table.
func TestOpcodeSpelling(t *testing.T) {
	cases := []struct{ want string }{
		{"REX.W + C7 /0 id"},
		{"EVEX.512.66.0F.W0 FE /r"},
		{"VEX.LZ.F2.0F38.W1 F6 /r"},
	}
	got := map[string]bool{}
	for _, f := range All() {
		got[f.Opcodes()] = true
	}
	for _, c := range cases {
		if !got[c.want] {
			t.Errorf("no form spells %q", c.want)
		}
	}
}

// Implicit operands are declared but never matched: `div rcx` takes one
// argument and touches three registers.
func TestImplicitOperandsAreNotArguments(t *testing.T) {
	f, err := Resolve(feature.Baseline(), "div", RegArg(reg.RCX))
	if err != nil {
		t.Fatal(err)
	}
	if f.Arity() != 1 {
		t.Errorf("Arity = %d, want 1", f.Arity())
	}
	if len(f.Slots) <= 1 {
		t.Error("div must declare the accumulator pair it clobbers")
	}
	if strings.Contains(f.GoName(), "RAX") {
		t.Errorf("GoName = %q; implicit operands must not be spelled", f.GoName())
	}
}

// A level is not a gate. Every form gates on a Feature, and V4's forms are
// reachable through the features V4 contains and not through V4 itself.
func TestEnabledGrowsWithTheSet(t *testing.T) {
	v1 := len(Enabled(feature.V1.Set()))
	v3 := len(Enabled(feature.V3.Set()))
	v4 := len(Enabled(feature.V4.Set()))
	if !(v1 < v3 && v3 < v4) {
		t.Errorf("enabled counts do not grow: v1=%d v3=%d v4=%d", v1, v3, v4)
	}
	if v4 == Count() {
		t.Error("v4 must not enable AMX, which is not part of any level")
	}
}