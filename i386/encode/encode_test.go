package encode

import (
	"bytes"
	"testing"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

func form(t *testing.T, mn, sig string) *isa.Form {
	t.Helper()
	for _, f := range isa.Forms(mn) {
		if f.Signature() == sig {
			return f
		}
	}
	t.Fatalf("no form %q", sig)
	return nil
}

func enc(t *testing.T, mn, sig string, ops ...operand.Operand) []byte {
	t.Helper()
	i, err := Encode(form(t, mn, sig), ops)
	if err != nil {
		t.Fatalf("%s: %v", sig, err)
	}
	return i.Bytes
}

func TestRegisterOperands(t *testing.T) {
	for _, c := range []struct {
		mn, sig string
		ops     []operand.Operand
		want    []byte
	}{
		{"mov", "MOV r/m32, r32", []operand.Operand{reg.EAX, reg.ECX}, []byte{0x89, 0xc8}},
		{"xor", "XOR r/m32, r32", []operand.Operand{reg.EDI, reg.EDI}, []byte{0x31, 0xff}},
		{"mov", "MOV r32, imm32", []operand.Operand{reg.EAX, operand.Imm(60)}, []byte{0xb8, 0x3c, 0, 0, 0}},
		{"mov", "MOV r/m32, imm32", []operand.Operand{reg.EAX, operand.Imm(60)}, []byte{0xc7, 0xc0, 0x3c, 0, 0, 0}},
		{"add", "ADD r/m32, imm8", []operand.Operand{reg.EAX, operand.Imm(1)}, []byte{0x83, 0xc0, 0x01}},
		{"add", "ADD EAX, imm32", []operand.Operand{reg.EAX, operand.Imm(1)}, []byte{0x05, 0x01, 0, 0, 0}},
		{"push", "PUSH r32", []operand.Operand{reg.EBP}, []byte{0x55}},
		{"pop", "POP r32", []operand.Operand{reg.EBP}, []byte{0x5d}},
		{"inc", "INC r32", []operand.Operand{reg.ECX}, []byte{0x41}},
		{"ret", "RET", nil, []byte{0xc3}},
		{"leave", "LEAVE", nil, []byte{0xc9}},
		{"not", "NOT r/m32", []operand.Operand{reg.EBX}, []byte{0xf7, 0xd3}},
	} {
		if got := enc(t, c.mn, c.sig, c.ops...); !bytes.Equal(got, c.want) {
			t.Errorf("%s = % x, want % x", c.sig, got, c.want)
		}
	}
}

// The four encodings whose fields do not mean what they say.
func TestAddressingSpecialCases(t *testing.T) {
	for _, c := range []struct {
		name string
		mem  operand.M32
		want []byte // the ModRM/SIB/disp part of MOV r32, r/m32 into EAX
	}{
		{"[eax]", operand.Mem32(reg.EAX), []byte{0x00}},
		{"[ecx]+8", operand.Mem32(reg.ECX).Disp(8), []byte{0x41, 0x08}},
		{"[ecx]+1000", operand.Mem32(reg.ECX).Disp(1000), []byte{0x81, 0xe8, 0x03, 0x00, 0x00}},

		// [EBP] is not encodable with mod=00 — that slot is disp32 — so it
		// comes out one byte longer with an explicit zero.
		{"[ebp]", operand.Mem32(reg.EBP), []byte{0x45, 0x00}},

		// ESP as a base is always a SIB encoding, because rm=100 means
		// "SIB follows", not "ESP".
		{"[esp]", operand.Mem32(reg.ESP), []byte{0x04, 0x24}},
		{"[esp]+8", operand.Mem32(reg.ESP).Disp(8), []byte{0x44, 0x24, 0x08}},

		// SIB.index=100 means no index; a plain base+index uses the real one.
		{"[eax+ecx*4]", operand.Mem32(reg.EAX).Index(reg.ECX, 4), []byte{0x04, 0x88}},
		{"[ebp+ecx*4]", operand.Mem32(reg.EBP).Index(reg.ECX, 4), []byte{0x44, 0x8d, 0x00}},

		// Index with no base: mod=00, SIB.base=101, disp32.
		{"[ecx*8+16]", operand.Abs32().Index(reg.ECX, 8).Disp(16), []byte{0x04, 0xcd, 0x10, 0, 0, 0}},

		// Absolute. On i386 this is mod=00 rm=101 with no SIB; the same
		// encoding is RIP-relative in 64-bit mode.
		{"[0x1234]", operand.Abs32().Disp(0x1234), []byte{0x05, 0x34, 0x12, 0, 0}},
	} {
		got := enc(t, "mov", "MOV r32, r/m32", reg.EAX, c.mem)
		want := append([]byte{0x8b}, c.want...)
		if !bytes.Equal(got, want) {
			t.Errorf("%s: % x, want % x", c.name, got, want)
		}
	}
}

func TestScaleEncoding(t *testing.T) {
	for _, c := range []struct {
		scale uint8
		ss    byte
	}{{1, 0x00}, {2, 0x40}, {4, 0x80}, {8, 0xc0}} {
		got := enc(t, "mov", "MOV r32, r/m32", reg.EAX,
			operand.Mem32(reg.EAX).Index(reg.ECX, c.scale))
		want := []byte{0x8b, 0x04, c.ss | 0x08}
		if !bytes.Equal(got, want) {
			t.Errorf("scale %d: % x, want % x", c.scale, got, want)
		}
	}
}

func TestSegmentOverride(t *testing.T) {
	got := enc(t, "mov", "MOV r32, r/m32", reg.EAX,
		operand.Mem32(reg.EAX).Segment(reg.GS))
	want := []byte{0x65, 0x8b, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("%%gs override: % x, want % x", got, want)
	}
}

// The field-position correction is computed, not written by hand.
func TestPCRelativeFixup(t *testing.T) {
	i, err := Encode(form(t, "call", "CALL rel32"),
		[]operand.Operand{operand.Ref("puts", operand.RelocKind(4))})
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Bytes) != 5 || i.Bytes[0] != 0xe8 {
		t.Fatalf("call = % x", i.Bytes)
	}
	if len(i.Fixups) != 1 {
		t.Fatalf("fixups = %d, want 1", len(i.Fixups))
	}
	f := i.Fixups[0]
	if f.Offset != 1 || f.Size != 4 || !f.PCRel {
		t.Errorf("fixup = %+v", f)
	}
	// The displacement ends the instruction, so the correction is -4 — the
	// value that would otherwise be typed by hand into an ELF addend.
	if f.Adjust != -4 {
		t.Errorf("Adjust = %d, want -4", f.Adjust)
	}
	if f.Kind != FixupReloc || f.Name != "puts" {
		t.Errorf("fixup names %q kind %v", f.Name, f.Kind)
	}
}

func TestLabelFixupIsNotARelocation(t *testing.T) {
	i, err := Encode(form(t, "jne", "JNE rel8"), []operand.Operand{operand.Label("retry")})
	if err != nil {
		t.Fatal(err)
	}
	if len(i.Fixups) != 1 || i.Fixups[0].Kind != FixupLabel {
		t.Fatalf("fixups = %+v", i.Fixups)
	}
	if i.Fixups[0].Adjust != -1 {
		t.Errorf("rel8 Adjust = %d, want -1", i.Fixups[0].Adjust)
	}
}

// A symbol in a displacement is always a 32-bit field, because its value
// cannot be proved to fit in a byte.
func TestSymbolDisplacement(t *testing.T) {
	i, err := Encode(form(t, "mov", "MOV r32, r/m32"),
		[]operand.Operand{reg.EAX, operand.Mem32(reg.EBX).Sym(operand.Ref("msg", 9))})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x8b, 0x83, 0, 0, 0, 0}
	if !bytes.Equal(i.Bytes, want) {
		t.Errorf("% x, want % x", i.Bytes, want)
	}
	if len(i.Fixups) != 1 || i.Fixups[0].Offset != 2 || i.Fixups[0].Size != 4 {
		t.Errorf("fixup = %+v", i.Fixups)
	}
	// A displacement is not PC-relative on i386; there is no such addressing
	// mode, which is why GOTOFF and GOTPC exist.
	if i.Fixups[0].PCRel {
		t.Error("memory displacement marked PC-relative")
	}
}

// The 0F 1F nop faults below i686. The table is chosen by level, not assumed.
func TestNopsByLevel(t *testing.T) {
	p6 := Nops(3, feature.Default())
	if !bytes.Equal(p6, []byte{0x0f, 0x1f, 0x00}) {
		t.Errorf("i686 3-byte nop = % x", p6)
	}
	old := Nops(3, feature.New(feature.I486))
	if !bytes.Equal(old, []byte{0x8d, 0x76, 0x00}) {
		t.Errorf("i486 3-byte nop = % x", old)
	}
	for _, s := range []feature.Set{feature.Default(), feature.New(feature.I386)} {
		for n := 1; n <= 64; n++ {
			if got := len(Nops(n, s)); got != n {
				t.Fatalf("%s: Nops(%d) produced %d bytes", s, n, got)
			}
		}
		if bytes.Contains(Nops(32, feature.New(feature.I386)), []byte{0x0f, 0x1f}) {
			t.Error("P6 nop emitted below i686")
		}
	}
}

// Identical calls produce identical bytes. There is no mode in which they
// don't.
func TestDeterministic(t *testing.T) {
	ops := []operand.Operand{reg.EAX, operand.Mem32(reg.EBX).Index(reg.ECX, 4).Disp(-16)}
	first := enc(t, "mov", "MOV r32, r/m32", ops...)
	for i := 0; i < 8; i++ {
		if got := enc(t, "mov", "MOV r32, r/m32", ops...); !bytes.Equal(got, first) {
			t.Fatalf("run %d differs: % x vs % x", i, got, first)
		}
	}
}