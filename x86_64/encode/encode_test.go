// x86_64/encode/encode_test.go
package encode

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/vertex-language/arc/x86_64/feature"
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// enc resolves and encodes the way the root's Emit does, so a test states an
// instruction and gets the bytes an assembler would write.
func enc(t *testing.T, mnemonic string, ops ...any) ([]byte, []Fixup) {
	t.Helper()
	args, err := Args(ops...)
	if err != nil {
		t.Fatal(err)
	}
	f, err := isa.Resolve(feature.V4.Plus(feature.AMXTILE), mnemonic, args...)
	if err != nil {
		t.Fatal(err)
	}
	b, fx, err := Encode(f, ops...)
	if err != nil {
		t.Fatal(err)
	}
	return b, fx
}

func want(t *testing.T, got []byte, s string) {
	t.Helper()
	exp, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, exp) {
		t.Errorf("got %x, want %s", got, s)
	}
}

func TestExitSixty(t *testing.T) {
	// The three instructions from the README, byte for byte.
	b, _ := enc(t, "mov", reg.RAX, operand.Imm(60))
	want(t, b, "48 c7 c0 3c 00 00 00")

	b, _ = enc(t, "xor", reg.RDI, reg.RDI)
	want(t, b, "48 31 ff")

	b, _ = enc(t, "syscall")
	want(t, b, "0f 05")
}

// The imm64 form is ten bytes and Resolve will not pick it for a value that
// fits in four. Reaching it means naming it, which is what the typed helper
// layer is for — so the test names the form instead of the mnemonic.
func TestMovImm64IsReachableOnlyByName(t *testing.T) {
	var form *isa.Form
	for _, f := range isa.Forms("mov") {
		if f.GoName() == "MovR64Imm64" {
			form = f
		}
	}
	if form == nil {
		t.Fatal("no MovR64Imm64 in the table")
	}
	b, _, err := Encode(form, reg.RAX, operand.Imm(60))
	if err != nil {
		t.Fatal(err)
	}
	want(t, b, "48 b8 3c 00 00 00 00 00 00 00")
}

// Every addressing form that has a special case, and the special case.
func TestAddressingForms(t *testing.T) {
	cases := []struct {
		name string
		mem  operand.M64
		want string
	}{
		{"plain base", operand.Mem64(reg.RBX), "48 8b 03"},
		{"disp8", operand.Mem64(reg.RBX).Disp(8), "48 8b 43 08"},
		{"disp32", operand.Mem64(reg.RBX).Disp(0x1000), "48 8b 83 00 10 00 00"},
		// RBP encodes as 5, and mod=00 rm=5 is disp32-with-no-base, so
		// [rbp] has to spend a zero disp8.
		{"rbp", operand.Mem64(reg.RBP), "48 8b 45 00"},
		{"r13", operand.Mem64(reg.R13), "49 8b 45 00"},
		// RSP encodes as 4, which is the escape to SIB, so [rsp] needs one
		// even with no index.
		{"rsp", operand.Mem64(reg.RSP), "48 8b 04 24"},
		{"r12", operand.Mem64(reg.R12), "49 8b 04 24"},
		{"index", operand.Mem64(reg.RBX).Index(reg.RCX, 4), "48 8b 04 8b"},
		// R12 as an index is fine: REX.X distinguishes it from "no index".
		{"r12 index", operand.Mem64(reg.RBX).Index(reg.R12, 8), "4a 8b 04 e3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, _ := enc(t, "mov", reg.RAX, c.mem)
			want(t, b, c.want)
		})
	}
}

// A %rip-relative load leaves a four-byte hole and a fixup whose tail is
// zero, because the displacement ends the instruction.
func TestRIPRelativeFixup(t *testing.T) {
	m := operand.RIPRel(operand.Label("msg")).M64()
	b, fx := enc(t, "mov", reg.RAX, m)
	want(t, b, "48 8b 05 00 00 00 00")

	if len(fx) != 1 {
		t.Fatalf("got %d fixups, want 1", len(fx))
	}
	f := fx[0]
	if f.Offset != 3 || f.Size != 4 || f.Tail != 0 {
		t.Errorf("fixup = %+v, want offset 3 size 4 tail 0", f)
	}
	if !f.PCRel || f.Use != UsePCRel {
		t.Errorf("a rip-relative load must be pc-relative")
	}
}

// The case the whole Tail field exists for. Four bytes of immediate follow
// the displacement, so the ELF addend is -8 and nobody wrote it.
func TestTailCountsWhatFollowsTheField(t *testing.T) {
	m := operand.RIPRel(operand.Label("x")).M32()
	b, fx := enc(t, "mov", m, operand.Imm(5))
	want(t, b, "c7 05 00 00 00 00 05 00 00 00")

	if len(fx) != 1 {
		t.Fatalf("got %d fixups, want 1", len(fx))
	}
	if fx[0].Tail != 4 {
		t.Errorf("Tail = %d, want 4 — the immediate follows the displacement", fx[0].Tail)
	}
	// The logical addend is untouched. write_elf.go subtracts Size+Tail.
	if fx[0].Addend != 0 {
		t.Errorf("Addend = %d, want 0", fx[0].Addend)
	}
}

func TestCallIsABranchFixup(t *testing.T) {
	b, fx := enc(t, "call", operand.Ref("puts", 0))
	want(t, b, "e8 00 00 00 00")
	if len(fx) != 1 || fx[0].Use != UseBranch || fx[0].Tail != 0 {
		t.Errorf("fixup = %+v, want a branch with tail 0", fx)
	}
}

// A segment override is a prefix on the memory operand, not on the
// instruction, and it lands ahead of REX.
func TestSegmentOverride(t *testing.T) {
	b, _ := enc(t, "mov", reg.RAX, operand.Mem64(reg.RBX).Segment(reg.FS).Disp(8))
	want(t, b, "64 48 8b 43 08")
}

// VEX folds to two bytes when X, B and W are clear and the map is 0F. That
// is one byte saved on most of AVX, so it is not optional.
func TestVEXTwoByteForm(t *testing.T) {
	b, _ := enc(t, "vaddps", reg.YMM0, reg.YMM1, reg.YMM2)
	want(t, b, "c5 f4 58 c2")

	// An extended register forces the three-byte form through B.
	b, _ = enc(t, "vaddps", reg.YMM0, reg.YMM1, reg.YMM9)
	want(t, b, "c4 c1 74 58 c1")
}

func TestEVEXPrefix(t *testing.T) {
	b, _ := enc(t, "vpaddd", reg.ZMM0, reg.K1, reg.ZMM1, reg.ZMM2)
	want(t, b, "62 f1 75 49 fe c2")
}

// Compressed displacement: the disp8 byte holds disp/N, so a full-vector
// 512-bit form addresses ±8128 bytes in one byte and refuses anything that
// is not a multiple of 64.
func TestCompressedDisplacement(t *testing.T) {
	m := operand.Mem512(reg.RAX).Disp(64)
	b, _ := enc(t, "vpaddd", reg.ZMM0, reg.K1, reg.ZMM1, m)
	want(t, b, "62 f1 75 49 fe 40 01")

	// Not a multiple of N: the encoding falls back to disp32 rather than
	// rounding, which would address the wrong cache line quietly.
	m = operand.Mem512(reg.RAX).Disp(8)
	b, _ = enc(t, "vpaddd", reg.ZMM0, reg.K1, reg.ZMM1, m)
	want(t, b, "62 f1 75 49 fe 80 08 00 00 00")
}

// AH and SPL are the same encoding, and REX is what chooses. An instruction
// that needs REX for another reason cannot also name AH.
func TestAHIsUnreachableUnderREX(t *testing.T) {
	args, err := Args(reg.AH, reg.R9B)
	if err != nil {
		t.Fatal(err)
	}
	f, err := isa.Resolve(feature.Baseline(), "mov", args...)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Encode(f, reg.AH, reg.R9B)
	var rc *RexConflictError
	if !errors.As(err, &rc) {
		t.Fatalf("got %v, want a RexConflictError", err)
	}
}

// SPL requires REX even when nothing else does, so the empty 0x40 prefix is
// emitted rather than optimized away.
func TestSPLForcesAnEmptyREX(t *testing.T) {
	b, _ := enc(t, "mov", reg.SPL, reg.AL)
	want(t, b, "40 88 c4")
}

// XMM16 has no encoding without EVEX, and a VEX form naming one is an error
// rather than a truncation to XMM0.
func TestExtendedVectorRegistersNeedEVEX(t *testing.T) {
	var form *isa.Form
	for _, f := range isa.Forms("vaddps") {
		if f.Enc == isa.EncVEX && f.Len == isa.L256 {
			form = f
			break
		}
	}
	_, _, err := Encode(form, reg.YMM16, reg.YMM1, reg.YMM2)
	var re *RegisterError
	if !errors.As(err, &re) {
		t.Fatalf("got %v, want a RegisterError", err)
	}
}

func TestNops(t *testing.T) {
	for n := 1; n <= MaxNop; n++ {
		if got := len(Nop(n)); got != n {
			t.Errorf("Nop(%d) is %d bytes", n, got)
		}
	}
	want(t, Nops(5), "0f 1f 44 00 00")
	if got := len(Nops(23)); got != 23 {
		t.Errorf("Nops(23) is %d bytes", got)
	}
	if Nop(0) != nil || Nop(10) != nil {
		t.Error("Nop is defined only over 1..MaxNop")
	}
}

// Nothing survives the call: the returned slices are the caller's.
func TestOutputIsNotShared(t *testing.T) {
	a, _ := enc(t, "ret")
	b, _ := enc(t, "ret")
	a[0] = 0
	if b[0] != 0xc3 {
		t.Error("two encodings share a buffer")
	}
}