package feature

import (
	"strings"
	"testing"
)

// Adding an extension adds everything it requires. A set holding AVX2 without
// AVX describes no silicon.
func TestAddTakesClosure(t *testing.T) {
	s := Default().Add(AVX512BW)
	for _, f := range []Feature{AVX512BW, AVX512F, AVX2, AVX, XSAVE, SSE42, SSE41, SSSE3, SSE3, SSE2, SSE, FXSR, MMX} {
		if !s.Has(f) {
			t.Errorf("Add(AVX512BW): missing %s", f)
		}
	}
	// and nothing it does not require
	for _, f := range []Feature{AVX512DQ, AVX512CD, AVX512VL, FMA, F16C, SHA, BMI1} {
		if s.Has(f) {
			t.Errorf("Add(AVX512BW): unexpectedly has %s", f)
		}
	}
}

// Removing takes the reverse closure, or the set becomes incoherent.
func TestRemoveTakesReverseClosure(t *testing.T) {
	s := Default().Add(AVX512DQ, FMA, SHA).Remove(AVX)
	for _, f := range []Feature{AVX, AVX2, FMA, F16C, AVX512F, AVX512DQ} {
		if s.Has(f) {
			t.Errorf("Remove(AVX): still has %s", f)
		}
	}
	// SSE2 and what depends only on it survive.
	for _, f := range []Feature{SSE2, SSE, MMX, FXSR, SHA} {
		if !s.Has(f) {
			t.Errorf("Remove(AVX): lost %s", f)
		}
	}
}

// One spelling out, many in.
func TestCanonicalString(t *testing.T) {
	if got := Default().String(); got != "i686" {
		t.Errorf("Default() = %q, want %q", got, "i686")
	}
	if got := New(I386).String(); got != "i386" {
		t.Errorf("New(I386) = %q, want %q", got, "i386")
	}
	// Input order is free; output order is declaration order.
	a, err := Parse(Default(), "avx2,sse2,bmi2")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(Default(), "bmi2,avx2")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("order-dependent: %q vs %q", a, b)
	}
	want := "i686+mmx+fxsr+sse+sse2+sse3+ssse3+sse4.1+sse4.2+xsave+avx+avx2+bmi2"
	if got := a.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseExactVersusAdjust(t *testing.T) {
	// Exact: starts from the bare level, does not inherit base extensions.
	base := Default().Add(AVX2)
	got, err := Parse(base, "sse2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Has(AVX2) {
		t.Error("exact form inherited AVX2 from base")
	}
	if !got.Has(SSE2) {
		t.Error("exact form lost SSE2")
	}

	// Adjust: starts from base.
	got, err = Parse(base, "+bmi2,-avx")
	if err != nil {
		t.Fatal(err)
	}
	if got.Has(AVX2) || got.Has(AVX) {
		t.Error("-avx did not remove AVX2")
	}
	if !got.Has(BMI2) || !got.Has(SSE42) {
		t.Error("adjust form lost the wrong things")
	}
}

func TestParseRejections(t *testing.T) {
	for _, c := range []struct{ in, wantSubstr string }{
		{"avx2,+bmi2", "cannot mix"},
		{"+i486", "base level"},
		{"i486,i686", "two base levels"},
		{"x86-64-v2", "does not apply to i386"},
		{"x86-64-v3", "CMPXCHG16B"},
		{"cx16", "64-bit mode"},
		{"sce", "64-bit mode"},
		{"mpx", "removed from the architecture"},
		{"3dnow", "removed from the architecture"},
		{"cmov", "i686 base level"},
		{"cmpxchg8b", "i586 base level"},
		{"zvfbfmin", "unknown extension"},
	} {
		_, err := Parse(Default(), c.in)
		if err == nil {
			t.Errorf("Parse(%q) succeeded, want error", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSubstr) {
			t.Errorf("Parse(%q) error = %q, want it to mention %q", c.in, err, c.wantSubstr)
		}
	}
}

func TestLevels(t *testing.T) {
	s := New(I486)
	if s.AtLeast(I586) {
		t.Error("i486 should not satisfy i586")
	}
	if !s.AtLeast(I386) || !s.AtLeast(I486) {
		t.Error("i486 should satisfy i386 and i486")
	}
	if Baseline != I686 {
		t.Error("baseline must be i686, the value in the arc targets BASELINE column")
	}
}

func TestMissing(t *testing.T) {
	active := Default().Add(SSE2)
	want := Default().Add(AVX2)
	m := active.Missing(want)
	if m.Has(SSE2) {
		t.Error("Missing reported an active feature")
	}
	for _, f := range []Feature{AVX, AVX2, XSAVE, SSE42} {
		if !m.Has(f) {
			t.Errorf("Missing lost %s", f)
		}
	}
}

func TestZeroValueIsBare386(t *testing.T) {
	var s Set
	if s.Level() != I386 || !s.Empty() {
		t.Errorf("zero Set = %q, want a bare i386", s)
	}
}