package isa

import (
	"testing"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// The opcodes that appear in the documentation and in arc explain output.
func TestKnownEncodings(t *testing.T) {
	for _, c := range []struct {
		mn   string
		sig  string
		want string
	}{
		{"mov", "MOV r/m32, r32", "89 /r"},
		{"mov", "MOV r32, imm32", "b8+r"},
		{"mov", "MOV r/m32, imm32", "c7 /0"},
		{"add", "ADD r/m32, imm8", "83 /0"},
		{"add", "ADD EAX, imm32", "05"},
		{"cmp", "CMP r/m32, r32", "39 /r"},
		{"xor", "XOR r/m32, r32", "31 /r"},
		{"lea", "LEA r32, m", "8d /r"},
		{"imul", "IMUL r32, r/m32", "0f af /r"},
		{"call", "CALL rel32", "e8"},
		{"jne", "JNE rel8", "75"},
		{"jne", "JNE rel32", "0f 85"},
		{"sete", "SETE r/m8", "0f 94 /r"},
		{"cmove", "CMOVE r32, r/m32", "0f 44 /r"},
		{"bswap", "BSWAP r32", "0f c8+r"},
		{"ret", "RET", "c3"},
	} {
		var got *Form
		for _, f := range Forms(c.mn) {
			if f.Signature() == c.sig {
				got = f
				break
			}
		}
		if got == nil {
			t.Errorf("no form %q", c.sig)
			continue
		}
		if b := got.Bytes(); b != c.want {
			t.Errorf("%s = %q, want %q", c.sig, b, c.want)
		}
	}
}

// The ALU group's regularity is the encoding's, not a convenience: the nth
// operator's base opcode is 8n and its group-1 digit is n.
func TestALUPattern(t *testing.T) {
	for n, a := range alu {
		var rmReg *Form
		for _, f := range Forms(a.name) {
			if f.Signature() == "ADD r/m32, r32" || f.Signature() == a.upperSig() {
				rmReg = f
				break
			}
		}
		_ = rmReg
		var found bool
		for _, f := range Forms(a.name) {
			if len(f.Opcode) == 1 && f.Opcode[0] == byte(n*8)+1 {
				found = true
			}
			if len(f.Opcode) == 1 && f.Opcode[0] == 0x83 && f.Ext != int8(n) {
				t.Errorf("%s: group-1 digit is %d, want %d", a.name, f.Ext, n)
			}
		}
		if !found {
			t.Errorf("%s: no form at opcode %#02x", a.name, n*8+1)
		}
	}
}

// A name that cannot distinguish two forms cannot be the name of a form.
func TestHelperNamesAreUnique(t *testing.T) {
	seen := make(map[string]*Form)
	for _, f := range All() {
		n := f.HelperName()
		if prev, dup := seen[n]; dup {
			t.Errorf("helper name %s is shared by %q and %q", n, prev.Signature(), f.Signature())
		}
		seen[n] = f
	}
}

// The imm8 and imm32 forms of a group-1 operator are both legal for a small
// constant. Resolve returns both; choosing is the caller's.
func TestResolveReturnsAllCandidates(t *testing.T) {
	ops := []operand.Operand{reg.EAX, operand.Imm(1)}
	match, gated := Resolve("add", feature.Default(), ops)
	if len(gated) != 0 {
		t.Errorf("nothing should be gated at baseline, got %d", len(gated))
	}
	want := map[string]bool{
		"ADD r/m32, imm8":  false,
		"ADD r/m32, imm32": false,
		"ADD EAX, imm32":   false,
	}
	for _, f := range match {
		if _, ok := want[f.Signature()]; !ok {
			t.Errorf("unexpected candidate %q", f.Signature())
			continue
		}
		want[f.Signature()] = true
	}
	for sig, seen := range want {
		if !seen {
			t.Errorf("missing candidate %q", sig)
		}
	}

	// A constant too wide for a byte drops the short form.
	match, _ = Resolve("add", feature.Default(), []operand.Operand{reg.EAX, operand.Imm(300)})
	for _, f := range match {
		if f.Ops[len(f.Ops)-1].Class == Imm8S {
			t.Error("imm8 form matched a value that does not fit")
		}
	}
}

// A fixed operand names one register and has no field to hold another.
func TestFixedOperands(t *testing.T) {
	match, _ := Resolve("add", feature.Default(), []operand.Operand{reg.ECX, operand.Imm(300)})
	for _, f := range match {
		if f.Signature() == "ADD EAX, imm32" {
			t.Error("the EAX short form matched ECX")
		}
	}
	if !EAX.Matches(reg.EAX) || EAX.Matches(reg.ECX) {
		t.Error("EAX class matches the wrong things")
	}
	if !One.Matches(operand.Imm(1)) || One.Matches(operand.Imm(2)) {
		t.Error("One class matches the wrong things")
	}
}

// CMOVcc is what the i686 level means. Below it the form is gated, and the
// diagnostic names the flag that would allow it.
func TestLevelGating(t *testing.T) {
	below := feature.New(feature.I586)
	match, gated := Resolve("cmove", below, []operand.Operand{reg.EAX, reg.ECX})
	if len(match) != 0 {
		t.Error("cmove resolved below i686")
	}
	if len(gated) != 1 {
		t.Fatalf("cmove gated forms = %d, want 1", len(gated))
	}
	if g := gated[0].Gate(); g != "i686" {
		t.Errorf("gate = %q, want %q", g, "i686")
	}
	// And at baseline it is simply available, with no gate to name.
	match, _ = Resolve("cmove", feature.Default(), []operand.Operand{reg.EAX, reg.ECX})
	if len(match) != 1 {
		t.Errorf("cmove at baseline: %d forms, want 1", len(match))
	}
	if g := match[0].Gate(); g != "" {
		t.Errorf("baseline form has gate %q", g)
	}
}

// An alias emits its target, so a listing says what the silicon does.
func TestAliases(t *testing.T) {
	var jz *Form
	for _, f := range Forms("jz") {
		if f.Ops[0].Class == Rel8 {
			jz = f
		}
	}
	if jz == nil {
		t.Fatal("no jz rel8")
	}
	if jz.AliasOf != "je" {
		t.Errorf("jz.AliasOf = %q, want je", jz.AliasOf)
	}
	var je *Form
	for _, f := range Forms("je") {
		if f.Ops[0].Class == Rel8 {
			je = f
		}
	}
	if je.Opcode[0] != jz.Opcode[0] {
		t.Error("jz and je encode differently")
	}
	if je.AliasOf != "" {
		t.Error("je should not be an alias")
	}
}

// Opcode 0x82 is a working duplicate of 0x80 in 32-bit mode and would give
// Emit two candidates of equal cost. It is deliberately absent.
func TestNoOpcode82(t *testing.T) {
	for _, f := range All() {
		if len(f.Opcode) == 1 && f.Opcode[0] == 0x82 {
			t.Errorf("opcode 0x82 declared by %q", f.Signature())
		}
	}
}

// Every form must be reachable and internally consistent.
func TestTableIntegrity(t *testing.T) {
	for _, f := range All() {
		if f.Mnemonic == "" || len(f.Opcode) == 0 {
			t.Errorf("form with no mnemonic or no opcode: %+v", f)
		}
		if f.Ext > 7 {
			t.Errorf("%s: /digit %d out of range", f.Signature(), f.Ext)
		}
		// A form cannot both carry a /digit and use ModRM.reg for an operand.
		if f.Ext >= 0 && f.hasSlot(SlotReg) {
			t.Errorf("%s: has /%d and a reg operand", f.Signature(), f.Ext)
		}
		if f.AliasOf != "" && len(Forms(f.AliasOf)) == 0 {
			t.Errorf("%s: alias of unknown mnemonic %q", f.Signature(), f.AliasOf)
		}
	}
}