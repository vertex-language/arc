// x86_64/feature/feature_test.go
package feature

import "testing"

func TestLevelsAreCumulative(t *testing.T) {
	for l := V1; l < V4; l++ {
		if !(l + 1).Set().Contains(l.Set()) {
			t.Errorf("%s is not a subset of %s", l, l+1)
		}
	}
}

func TestBaselineIsSSE2AndNothingAbove(t *testing.T) {
	b := Baseline()
	if !b.Has(SSE2) || !b.Has(SSE) || !b.Has(MMX) {
		t.Error("v1 must have MMX, SSE and SSE2")
	}
	for _, f := range []Feature{SSE3, SSE42, AVX, AVX2, POPCNT, AVX512F} {
		if b.Has(f) {
			t.Errorf("v1 must not have %s", f)
		}
	}
}

// The psABI states v3 as AVX2 and friends; SSE4.2 and POPCNT come from v2,
// and SSE/SSE2/SSE3/SSSE3/SSE4.1 come from closure. Nothing lists them twice.
func TestClosureFillsTheChain(t *testing.T) {
	v3 := V3.Set()
	for _, f := range []Feature{SSE, SSE2, SSE3, SSSE3, SSE41, SSE42, AVX, AVX2} {
		if !v3.Has(f) {
			t.Errorf("v3 must include %s by closure", f)
		}
	}
	if !V4.Set().Has(AVX512F) || !V4.Set().Has(AVX2) {
		t.Error("v4 must include AVX512F and, through it, AVX2")
	}
}

func TestPlusIsClosed(t *testing.T) {
	s := Empty().Plus(AVX512VBMI2)
	for _, f := range []Feature{AVX512BW, AVX512F, AVX2, AVX, SSE42, SSE2, SSE} {
		if !s.Has(f) {
			t.Errorf("AVX512VBMI2 must pull in %s", f)
		}
	}
}

// Removing a feature removes what depends on it. A set holding AVX512BW with
// AVX512F gone would claim an encoding whose prefix is unavailable.
func TestMinusClosesDownward(t *testing.T) {
	s := V4.Plus(AVX512VBMI2).Minus(AVX512F)
	for _, f := range []Feature{AVX512F, AVX512BW, AVX512VBMI2, AVX512VL, AVX512DQ, AVX512CD} {
		if s.Has(f) {
			t.Errorf("dropping AVX512F must drop %s", f)
		}
	}
	if !s.Has(AVX2) {
		t.Error("dropping AVX512F must not drop what it required")
	}
}

func TestCanonicalSpellingRoundTrips(t *testing.T) {
	cases := []string{
		"x86-64-v1",
		"x86-64-v3",
		"x86-64-v4",
		"x86-64-v1+avx512f",
		"x86-64-v3+avx512vbmi2",
	}
	for _, in := range cases {
		s, err := ParseFeatures(in)
		if err != nil {
			t.Fatalf("ParseFeatures(%q): %v", in, err)
		}
		if got := s.String(); got != in {
			t.Errorf("ParseFeatures(%q).String() = %q", in, got)
		}
		again, err := ParseFeatures(s.String())
		if err != nil || again != s {
			t.Errorf("%q did not round-trip", in)
		}
	}
}

// Decompose names the minimal generating set: avx512bw implies avx512f, so
// only avx512bw is printed.
func TestDecomposeDropsImplied(t *testing.T) {
	s := V1.Plus(AVX512F, AVX512BW)
	_, extras := s.Decompose()
	if len(extras) != 1 || extras[0] != AVX512BW {
		t.Errorf("extras = %v, want [avx512bw]", extras)
	}
	if got, want := s.String(), "x86-64-v1+avx512bw"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// The note line of a gating diagnostic is built from GoExpr, so it must name
// something that compiles.
func TestGoExpr(t *testing.T) {
	if got := V1.Plus(AVX512F).GoExpr(); got != "V1.Plus(AVX512F)" {
		t.Errorf("GoExpr = %q", got)
	}
	if got := V3.Set().GoExpr(); got != "V3" {
		t.Errorf("GoExpr = %q", got)
	}
}

func TestNamesWithSeparatorsParse(t *testing.T) {
	for _, in := range []string{"sse4.1", "amx-tile", "lahf-sahf", "amx-tile+amx-int8"} {
		if _, err := ParseFeatures(in); err != nil {
			t.Errorf("ParseFeatures(%q): %v", in, err)
		}
	}
}

func TestAliasesResolveAndVanish(t *testing.T) {
	s, err := ParseFeatures("sse4_1+cx16+abm")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Has(SSE41) || !s.Has(CMPXCHG16B) || !s.Has(LZCNT) {
		t.Error("aliases did not resolve")
	}
	if got := s.String(); got == "sse4_1+cx16+abm" {
		t.Error("aliases must not survive into canonical spelling")
	}
}

func TestBaseIsAlwaysPresent(t *testing.T) {
	if !Empty().Has(Base) {
		t.Error("Base must be in every set, including the empty one")
	}
}

func TestUnknownFeatureNamesItself(t *testing.T) {
	_, err := ParseFeatures("x86-64-v1+avx1024")
	if err == nil {
		t.Fatal("expected an error")
	}
	// The diagnostic has to name the bad token, not just fail.
	if !contains(err.Error(), "avx1024") {
		t.Errorf("error does not name the term: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}