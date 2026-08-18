// x86_64/operand/operand_test.go
package operand

import (
	"errors"
	"testing"

	"github.com/vertex-language/arc/x86_64/reg"
)

// RSP is the one register with no index encoding: index field 4 means "no
// index" and there is no escape. R12 is also 4, but REX.X distinguishes it.
func TestIndexRSPIsRejectedButR12IsNot(t *testing.T) {
	if err := Mem64(reg.RAX).Index(reg.RSP, 1).Validate(); !errors.Is(err, ErrIndexRSP) {
		t.Errorf("rsp index: got %v, want ErrIndexRSP", err)
	}
	if err := Mem64(reg.RAX).Index(reg.R12, 8).Validate(); err != nil {
		t.Errorf("r12 index: %v", err)
	}
}

func TestScale(t *testing.T) {
	for _, s := range []uint8{1, 2, 4, 8} {
		if err := Mem64(reg.RAX).Index(reg.RCX, s).Validate(); err != nil {
			t.Errorf("scale %d: %v", s, err)
		}
	}
	for _, s := range []uint8{0, 3, 5, 16} {
		if err := Mem64(reg.RAX).Index(reg.RCX, s).Validate(); !errors.Is(err, ErrScale) {
			t.Errorf("scale %d: got %v, want ErrScale", s, err)
		}
	}
}

// mod=00 rm=101 is disp32-with-no-base, so [rbp] and [r13] have to spend a
// zero disp8. Nothing else does.
func TestZeroDispRequiredForRBPAndR13(t *testing.T) {
	for _, r := range []reg.Reg64{reg.RBP, reg.R13} {
		if !Mem64(r).Mem.NeedsZeroDisp8() {
			t.Errorf("[%s] must encode a zero disp8", r)
		}
	}
	for _, r := range []reg.Reg64{reg.RAX, reg.RBX, reg.R12, reg.R14} {
		if Mem64(r).Mem.NeedsZeroDisp8() {
			t.Errorf("[%s] must not spend a disp8", r)
		}
	}
	// With a real displacement the question does not arise.
	if Mem64(reg.RBP).Disp(8).Mem.NeedsZeroDisp8() {
		t.Error("[rbp+8] already has a displacement")
	}
}

// rm=100 is the escape to SIB, and RSP and R12 both encode as 4.
func TestSIBRequired(t *testing.T) {
	for _, r := range []reg.Reg64{reg.RSP, reg.R12} {
		if !Mem64(r).Mem.NeedsSIB() {
			t.Errorf("[%s] must use a SIB byte", r)
		}
	}
	if Mem64(reg.RAX).Mem.NeedsSIB() {
		t.Error("[rax] needs no SIB")
	}
	if !Mem64(reg.RAX).Index(reg.RCX, 4).Mem.NeedsSIB() {
		t.Error("an index always needs SIB")
	}
	if !Abs(0x1000).NeedsSIB() {
		t.Error("a baseless form needs SIB")
	}
	if RIPRelDisp(0).NeedsSIB() {
		t.Error("rip-relative has no SIB")
	}
}

func TestRIPExcludesBaseAndIndex(t *testing.T) {
	m := RIPRel(Label("msg"))
	m.HasBase, m.Base = true, reg.RBX
	if !errors.Is(m.Validate(), ErrRIPWithBase) {
		t.Error("rip with a base must be refused")
	}
}

func TestImmNarrowest(t *testing.T) {
	cases := []struct {
		v    int64
		want Width
	}{
		{0, W8}, {127, W8}, {-128, W8},
		{128, W16}, {-129, W16}, {32767, W16},
		{32768, W32}, {2147483647, W32}, {-2147483648, W32},
		{2147483648, W64}, {-2147483649, W64},
	}
	for _, c := range cases {
		if got := Imm(c.v).Narrowest(); got != c.want {
			t.Errorf("Imm(%d).Narrowest() = %v, want %v", c.v, got, c.want)
		}
	}
}

// The decision between MOV r/m64, imm32 and MOV r64, imm64 is exactly this
// predicate, and it is the reason the typed layer exists: a caller who wrote
// MovR64Imm64 gets imm64 whether or not the value fits in less.
func TestFitsInt32(t *testing.T) {
	if !Imm(60).FitsInt32() {
		t.Error("60 fits an imm32")
	}
	if Imm(1 << 40).FitsInt32() {
		t.Error("2^40 does not fit an imm32")
	}
	if !Imm(-1).FitsInt32() || Imm(-1).FitsUint32() {
		t.Error("-1 sign-extends from imm32 but does not zero-extend")
	}
}

func TestSymRefAddendIsLogical(t *testing.T) {
	// No -4. The displacement's position is the encoder's knowledge.
	r := Ref("puts", 0).At(16)
	if r.Addend != 16 {
		t.Errorf("Addend = %d, want 16", r.Addend)
	}
}

func TestStringRoundsToSomethingReadable(t *testing.T) {
	cases := []struct {
		m    Mem
		want string
	}{
		{Mem64(reg.RBX).Mem, "[rbx]"},
		{Mem64(reg.RBX).Disp(8).Mem, "[rbx+8]"},
		{Mem64(reg.RBX).Disp(-8).Mem, "[rbx-8]"},
		{Mem64(reg.RBX).Index(reg.RCX, 4).Mem, "[rbx+rcx*4]"},
		{Mem64(reg.RBX).Segment(reg.FS).Disp(8).Mem, "fs:[rbx+8]"},
		{RIPRel(Label("msg")), "[rip+msg]"},
		{Mem64(reg.RBX).Addr32().Mem, "[ebx]"},
		{Abs(0x1000), "[4096]"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestWidthIsInTheType(t *testing.T) {
	if Mem8(reg.RAX).Width != W8 || Mem512(reg.RAX).Width != W512 {
		t.Error("constructors must fix the width")
	}
	// A width-agnostic form narrows explicitly; lea keeps it unsized.
	if RIPRel(Label("msg")).Width != WidthNone {
		t.Error("rip-relative starts unsized")
	}
	if RIPRel(Label("msg")).M64().Width != W64 {
		t.Error("narrowing must set the width")
	}
}