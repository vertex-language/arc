package operand

import (
	"errors"
	"testing"

	"github.com/vertex-language/arc/i386/reg"
)

// SIB.index=100b encodes "no index", so ESP has no index encoding at all.
func TestESPCannotBeIndex(t *testing.T) {
	m := Mem32(reg.EAX).Index(reg.ESP, 4)
	if !errors.Is(m.Err(), ErrOperand) {
		t.Fatal("esp accepted as an index")
	}
	// ESP is fine as a base; it only forces a SIB byte.
	if err := Mem32(reg.ESP).Disp(8).Err(); err != nil {
		t.Errorf("esp rejected as a base: %v", err)
	}
	// Every other register indexes.
	for _, r := range []reg.R32{reg.EAX, reg.ECX, reg.EDX, reg.EBX, reg.EBP, reg.ESI, reg.EDI} {
		if err := Mem32(reg.EAX).Index(r, 1).Err(); err != nil {
			t.Errorf("%s rejected as an index: %v", r, err)
		}
	}
}

func TestScale(t *testing.T) {
	for _, s := range []uint8{1, 2, 4, 8} {
		if err := Mem32(reg.EAX).Index(reg.ECX, s).Err(); err != nil {
			t.Errorf("scale %d rejected: %v", s, err)
		}
	}
	for _, s := range []uint8{0, 3, 5, 16} {
		if err := Mem32(reg.EAX).Index(reg.ECX, s).Err(); err == nil {
			t.Errorf("scale %d accepted", s)
		}
	}
}

// SS when the base is ESP or EBP, DS otherwise.
func TestDefaultSegment(t *testing.T) {
	for _, c := range []struct {
		base reg.R32
		want reg.Sreg
	}{
		{reg.ESP, reg.SS},
		{reg.EBP, reg.SS},
		{reg.EAX, reg.DS},
		{reg.ESI, reg.DS},
		{reg.EDI, reg.DS},
	} {
		if got := Mem32(c.base).DefaultSeg(); got != c.want {
			t.Errorf("base %s: default segment %s, want %s", c.base, got, c.want)
		}
	}
	// No base is DS.
	if got := Abs32().DefaultSeg(); got != reg.DS {
		t.Errorf("absolute: default segment %s, want ds", got)
	}
}

// The width is a type, so a wrong-width operand cannot reach a helper. These
// assignments are the compile-time check, written out.
func TestWidthIsAType(t *testing.T) {
	var _ RM32 = reg.EAX
	var _ RM32 = Mem32(reg.EAX)
	var _ RM8 = reg.AL
	var _ RM8 = Mem8(reg.EAX)
	var _ RM128 = reg.XMM0
	var _ RM128 = Mem128(reg.EAX)

	// var _ RM32 = reg.AL          // cannot use reg.AL as RM32
	// var _ RM32 = Mem8(reg.EAX)   // cannot use M8 as RM32

	for _, c := range []struct {
		op   Operand
		bits int
	}{
		{Mem8(reg.EAX), 8},
		{Mem16(reg.EAX), 16},
		{Mem32(reg.EAX), 32},
		{Mem64(reg.EAX), 64},
		{Mem80(reg.EAX), 80},
		{Mem128(reg.EAX), 128},
		{Mem256(reg.EAX), 256},
		{Mem512(reg.EAX), 512},
		{reg.EAX, 32},
		{reg.AH, 8},
	} {
		if got := c.op.Bits(); got != c.bits {
			t.Errorf("Bits() = %d, want %d", got, c.bits)
		}
	}
}

// Imm, Label and SymRef have no width: an immediate takes the form's, and a
// name has none at all.
func TestWidthlessOperands(t *testing.T) {
	for _, op := range []Operand{NewImm(60), NewLabel("loop"), Ref("puts", RelocKind(4))} {
		if got := op.Bits(); got != 0 {
			t.Errorf("%T.Bits() = %d, want 0", op, got)
		}
	}
}

// Addends are logical. The field-position correction is the assembler's.
func TestAddend(t *testing.T) {
	r := Ref("buf", RelocKind(1)).Plus(16)
	if r.Name() != "buf" || r.Addend() != 16 {
		t.Errorf("Ref = %s, addend %d", r.Name(), r.Addend())
	}
	if got := r.String(); got != "buf+16" {
		t.Errorf("String() = %q, want %q", got, "buf+16")
	}
	if got := Ref("buf", 1).Plus(-4).String(); got != "buf-4" {
		t.Errorf("String() = %q, want %q", got, "buf-4")
	}
}

func TestMemString(t *testing.T) {
	for _, c := range []struct {
		m    interface{ String() string }
		want string
	}{
		{Mem32(reg.EAX), "(%eax)"},
		{Mem32(reg.EAX).Disp(8), "8(%eax)"},
		{Mem32(reg.EAX).Index(reg.ECX, 4), "(%eax,%ecx,4)"},
		{Mem32(reg.EAX).Index(reg.ECX, 4).Disp(-16), "-16(%eax,%ecx,4)"},
		{Abs32().Disp(0x1234), "4660"},
		{Mem32(reg.EBX).Sym(Ref("msg", 9)), "msg(%ebx)"},
		{Mem32(reg.EAX).Segment(reg.GS), "%gs:(%eax)"},
	} {
		if got := c.m.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}