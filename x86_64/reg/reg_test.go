// x86_64/reg/reg_test.go
package reg

import "testing"

// The DWARF table is not the encoding table. This test exists because the
// two agree on RAX and RBX, so a derived implementation looks correct until
// it is asked about anything else.
func TestDWARFIsNotNum(t *testing.T) {
	for _, c := range []struct {
		r     Reg64
		num   uint8
		dwarf int
	}{
		{RAX, 0, 0},
		{RCX, 1, 2},
		{RDX, 2, 1},
		{RBX, 3, 3},
		{RSP, 4, 7},
		{RBP, 5, 6},
		{RSI, 6, 4},
		{RDI, 7, 5},
		{R15, 15, 15},
	} {
		if got := c.r.Num(); got != c.num {
			t.Errorf("%s.Num() = %d, want %d", c.r, got, c.num)
		}
		if got := c.r.DWARF(); got != c.dwarf {
			t.Errorf("%s.DWARF() = %d, want %d", c.r, got, c.dwarf)
		}
	}
}

func TestByteRegisterCollision(t *testing.T) {
	// AH and SPL are both encoding 4 and cannot coexist.
	if AH.Num() != SPL.Num() {
		t.Fatalf("AH=%d SPL=%d, expected both 4", AH.Num(), SPL.Num())
	}
	if !AH.RexForbidden() || AH.RexRequired() {
		t.Error("AH must forbid REX and not require it")
	}
	if !SPL.RexRequired() || SPL.RexForbidden() {
		t.Error("SPL must require REX and not forbid it")
	}
	// AH lives in RAX, not in RSP, despite sharing SPL's number.
	if AH.Parent() != RAX {
		t.Errorf("AH.Parent() = %s, want rax", AH.Parent())
	}
	if AH.DWARF() != RAX.DWARF() {
		t.Error("AH must answer RAX's DWARF number")
	}
}

func TestOverlaps(t *testing.T) {
	cases := []struct {
		a, b Reg
		want bool
	}{
		{AL, AH, false},  // disjoint bit ranges in the same entry
		{AL, RAX, true},
		{AH, RAX, true},
		{AH, EAX, true},
		{AL, RBX, false},
		{XMM4, ZMM4, true},
		{YMM4, XMM4, true},
		{XMM4, XMM5, false},
		{MM2, ST2, true},
		{MM2, ST3, false},
		{ST0, XMM0, false}, // different files, same index
	}
	for _, c := range cases {
		if got := Overlaps(c.a, c.b); got != c.want {
			t.Errorf("Overlaps(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSaveDependsOnPlatformAndWidth(t *testing.T) {
	if RSI.Save(ELF) != Volatile {
		t.Error("RSI is caller-saved under System V")
	}
	if RSI.Save(COFF) != Preserved {
		t.Error("RSI is callee-saved under Win64")
	}
	if XMM6.Save(ELF) != Volatile {
		t.Error("every vector register is caller-saved under System V")
	}
	if XMM6.Save(COFF) != Preserved {
		t.Error("XMM6 is callee-saved under Win64")
	}
	if YMM6.Save(COFF) != PreservedLow {
		t.Error("YMM6's upper half is caller-saved under Win64")
	}
}