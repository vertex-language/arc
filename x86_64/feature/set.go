// x86_64/feature/set.go
package feature

import "strings"

// Set is a set of enabled features. It is a value: comparable with ==, usable
// as a map key, and cheap to copy. Every Set contains Base.
//
// A Set is always closed under requirements. There is no way to construct one
// that holds AVX512BW without AVX512F, because a set that could would make
// every "requires" check in encode/ a two-step question.
type Set struct {
	bits uint64
}

func init() {
	// A Set is one word. If the vocabulary outgrows it this must become
	// [2]uint64, and the change is mechanical — but it must be noticed.
	if numFeatures > 64 {
		panic("feature: vocabulary exceeds one word; widen Set.bits")
	}
}

func bit(f Feature) uint64 { return 1 << uint(f) }

// Has reports whether f is enabled. Base is always enabled.
func (s Set) Has(f Feature) bool {
	return f == Base || s.bits&bit(f) != 0
}

// Contains reports whether every feature of t is enabled in s.
func (s Set) Contains(t Set) bool { return s.bits&t.bits == t.bits }

// Plus returns s with each feature and everything it requires enabled.
func (s Set) Plus(fs ...Feature) Set {
	for _, f := range fs {
		s.bits |= closure(f)
	}
	return s
}

// Minus returns s with each feature disabled, along with everything that
// requires it. Dropping AVX512F drops AVX512BW with it; the alternative is a
// set that claims an encoding is available when its prefix is not.
func (s Set) Minus(fs ...Feature) Set {
	for _, f := range fs {
		for g := MMX; g < numFeatures; g++ {
			if g == f || closure(g)&bit(f) != 0 {
				s.bits &^= bit(g)
			}
		}
	}
	return s
}

// Union is every feature enabled in either set.
func (s Set) Union(t Set) Set { return Set{s.bits | t.bits} }

// Features lists every enabled feature in declaration order, including those
// pulled in by closure.
func (s Set) Features() []Feature {
	var out []Feature
	for f := MMX; f < numFeatures; f++ {
		if s.bits&bit(f) != 0 {
			out = append(out, f)
		}
	}
	return out
}

// Empty is the set with nothing above Base. It is not Baseline — Baseline is
// V1, which has SSE2. Empty exists for callers building a set from scratch.
func Empty() Set { return Set{} }

// closure is f plus everything f requires, transitively. Computed on demand
// and memoised; the graph is a few dozen edges deep at most.
var closureCache [numFeatures]uint64

func closure(f Feature) uint64 {
	if f == Base || int(f) >= int(numFeatures) {
		return 0
	}
	if c := closureCache[f]; c != 0 {
		return c
	}
	c := bit(f)
	for _, r := range requires[f] {
		c |= closure(r)
	}
	closureCache[f] = c
	return c
}

// Decompose splits s into the highest level it fully contains and the minimal
// set of features that must be named to reach the rest. Redundant features are
// dropped: a set holding AVX512F and AVX512BW decomposes to AVX512BW alone,
// because naming it implies the other.
//
// Both String and a gating diagnostic's note line are built from this, so they
// cannot disagree about what the active set is called.
func (s Set) Decompose() (Level, []Feature) {
	lvl := LevelNone
	for l := V1; l <= V4; l++ {
		if s.Contains(l.Set()) {
			lvl = l
		}
	}

	rest := s.bits &^ lvl.Set().bits
	var extras []Feature
	for f := MMX; f < numFeatures; f++ {
		if rest&bit(f) == 0 {
			continue
		}
		// Skip f if another feature still in rest already implies it.
		implied := false
		for g := MMX; g < numFeatures; g++ {
			if g != f && rest&bit(g) != 0 && closure(g)&bit(f) != 0 {
				implied = true
				break
			}
		}
		if !implied {
			extras = append(extras, f)
		}
	}
	return lvl, extras
}

// String is the canonical spelling, and round-trips through ParseFeatures.
//
//	x86-64-v1
//	x86-64-v1+avx512f
//	sse2+aes           (no level fully contained)
func (s Set) String() string {
	lvl, extras := s.Decompose()

	var b strings.Builder
	if lvl != LevelNone {
		b.WriteString(lvl.String())
	}
	for i, f := range extras {
		if b.Len() > 0 || i > 0 {
			b.WriteByte('+')
		}
		b.WriteString(f.Name())
	}
	if b.Len() == 0 {
		return "base"
	}
	return b.String()
}

// GoExpr renders s as the Go expression that would build it, unqualified. The
// caller prefixes each identifier with its own package name:
//
//	V1.Plus(AVX512F)  ->  x86_64.V1.Plus(x86_64.AVX512F)
//
// This is what the note line of a gating diagnostic prints, so the message
// names something the user can paste back.
func (s Set) GoExpr() string {
	lvl, extras := s.Decompose()

	var b strings.Builder
	if lvl != LevelNone {
		b.WriteString(lvl.GoName())
	} else {
		b.WriteString("Empty()")
	}
	if len(extras) > 0 {
		b.WriteString(".Plus(")
		for i, f := range extras {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.GoName())
		}
		b.WriteByte(')')
	}
	return b.String()
}