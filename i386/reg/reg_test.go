package reg

import "testing"

// The high-byte registers are what makes i386 a package rather than a build
// tag on x86_64. Without a REX prefix, encoding numbers 4-7 of the byte file
// are AH, CH, DH and BH, and they are never displaced by SPL, BPL, SIL, DIL.
func TestHighByteRegisters(t *testing.T) {
	for _, c := range []struct {
		r    R8
		num  uint8
		par  R16
		high bool
	}{
		{AL, 0, AX, false},
		{CL, 1, CX, false},
		{DL, 2, DX, false},
		{BL, 3, BX, false},
		{AH, 4, AX, true},
		{CH, 5, CX, true},
		{DH, 6, DX, true},
		{BH, 7, BX, true},
	} {
		if got := c.r.Num(); got != c.num {
			t.Errorf("%s.Num() = %d, want %d", c.r, got, c.num)
		}
		if got := c.r.Parent(); got != c.par {
			t.Errorf("%s.Parent() = %s, want %s", c.r, got, c.par)
		}
		if got := c.r.High(); got != c.high {
			t.Errorf("%s.High() = %v, want %v", c.r, got, c.high)
		}
	}
}

// AL and AH are disjoint bytes of the same register. A model that carried only
// (parent, width) would get this wrong.
func TestOverlaps(t *testing.T) {
	for _, c := range []struct {
		a, b Reg
		want bool
	}{
		{AL, AH, false},
		{AL, AX, true},
		{AH, AX, true},
		{AL, EAX, true},
		{AH, EAX, true},
		{AX, EAX, true},
		{AL, BL, false},
		{AH, BH, false},
		{EAX, EBX, false},
		{AL, AL, true},

		// Different classes never overlap, whatever their numbers.
		{EAX, XMM0, false},
		{ST0, MM0, false},
		{CR0, DR0, false},
		{EAX, CR0, false},

		// The vector widths do share a root.
		{XMM3, YMM3, true},
		{XMM3, ZMM3, true},
		{YMM3, ZMM3, true},
		{XMM3, YMM4, false},
		{K0, ZMM0, false},
	} {
		if got := c.a.Overlaps(c.b); got != c.want {
			t.Errorf("%s.Overlaps(%s) = %v, want %v", c.a.Name(), c.b.Name(), got, c.want)
		}
		if got := c.b.Overlaps(c.a); got != c.want {
			t.Errorf("%s.Overlaps(%s) = %v, want %v (not symmetric)", c.b.Name(), c.a.Name(), got, c.want)
		}
	}
}

// Segment register encoding order is ES CS SS DS FS GS, which is neither
// alphabetical nor the order these are conventionally listed in.
func TestSegmentOrder(t *testing.T) {
	want := []Sreg{ES, CS, SS, DS, FS, GS}
	for i, r := range want {
		if r.Num() != uint8(i) {
			t.Errorf("%s.Num() = %d, want %d", r, r.Num(), i)
		}
	}
}

// Intel386 psABI v1.1, Table 2.14.
func TestDWARF(t *testing.T) {
	for _, c := range []struct {
		r    Reg
		num  int
		want bool
	}{
		{EAX, 0, true}, {ECX, 1, true}, {EDX, 2, true}, {EBX, 3, true},
		{ESP, 4, true}, {EBP, 5, true}, {ESI, 6, true}, {EDI, 7, true},
		{ST0, 11, true}, {ST7, 18, true},
		{XMM0, 21, true}, {XMM7, 28, true},
		{MM0, 29, true}, {MM7, 36, true},
		{ES, 40, true}, {CS, 41, true}, {SS, 42, true},
		{DS, 43, true}, {FS, 44, true}, {GS, 45, true},

		// The table assigns nothing to the narrow views or to AVX.
		{AX, noDWARF, false},
		{AL, noDWARF, false},
		{AH, noDWARF, false},
		{YMM0, noDWARF, false},
		{ZMM0, noDWARF, false},
		{K0, noDWARF, false},
		{CR0, noDWARF, false},
	} {
		got, ok := c.r.DWARF()
		if ok != c.want || got != c.num {
			t.Errorf("%s.DWARF() = %d, %v; want %d, %v", c.r.Name(), got, ok, c.num, c.want)
		}
	}
}

// Intel386 psABI v1.1, Table 2.3. Preservation is a property of the whole
// register, so the narrow views inherit it.
func TestSave(t *testing.T) {
	for _, c := range []struct {
		r    Reg
		want Save
	}{
		{EAX, CallerSaved}, {ECX, CallerSaved}, {EDX, CallerSaved},
		{EBX, CalleeSaved}, {ESP, CalleeSaved}, {EBP, CalleeSaved},
		{ESI, CalleeSaved}, {EDI, CalleeSaved},
		{AX, CallerSaved}, {AL, CallerSaved}, {AH, CallerSaved},
		{BX, CalleeSaved}, {BL, CalleeSaved}, {BH, CalleeSaved},
		{ST0, CallerSaved}, {XMM0, CallerSaved},
		{CS, SaveNone}, {CR0, SaveNone},
	} {
		if got := c.r.Save(); got != c.want {
			t.Errorf("%s.Save() = %v, want %v", c.r.Name(), got, c.want)
		}
	}
}

func TestLookup(t *testing.T) {
	for _, name := range []string{"eax", "ax", "al", "ah", "es", "gs", "st0", "st7", "mm0", "xmm0", "ymm7", "zmm0", "k7", "cr0", "dr7"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%q) failed", name)
		}
	}
	// Dialect spellings and registers this arch does not have.
	for _, name := range []string{"%eax", "st(0)", "db0", "rax", "r8d", "spl", "sil", "xmm8", "tr0", "eip", "eiz"} {
		if r, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q) = %s, want failure", name, r.Name())
		}
	}
}

func TestNumIsOrdinal(t *testing.T) {
	for _, r := range All() {
		s := r.spec()
		if s.num != r.Num() {
			t.Errorf("%s: spec num %d != Num() %d", r.Name(), s.num, r.Num())
		}
		if r.Num() > 7 {
			t.Errorf("%s: Num() = %d, no i386 encoding field is wider than 3 bits", r.Name(), r.Num())
		}
		if got := int(s.hi - s.lo); got != r.Bits() {
			t.Errorf("%s: extent is %d bits, Bits() = %d", r.Name(), got, r.Bits())
		}
	}
}