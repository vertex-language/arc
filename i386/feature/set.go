package feature

import "strings"

// Set is an i386 feature set: a base level and the extensions active over it.
//
// A Set is always closed under requires. There is no way to construct one that
// holds AVX2 without AVX, because every operation that adds takes the closure
// and every operation that removes takes the reverse closure.
//
// The zero Set is level i386 with no extensions — a bare 80386. It is a
// meaningful value, not an uninitialized one.
type Set struct {
	level Level
	bits  uint64
}

// New returns the set for a level with no extensions over it.
func New(l Level) Set { return Set{level: l} }

// Default is the set arc assembles for when --features selects nothing.
func Default() Set { return New(Baseline) }

// Level returns the base level.
func (s Set) Level() Level { return s.level }

// WithLevel returns s at level l, keeping its extensions.
func (s Set) WithLevel(l Level) Set { s.level = l; return s }

// Has reports whether f is active.
func (s Set) Has(f Feature) bool { return f < numFeatures && s.bits&(1<<f) != 0 }

// AtLeast reports whether the base level is l or higher.
func (s Set) AtLeast(l Level) bool { return s.level >= l }

// Add returns s with f and everything f requires.
func (s Set) Add(f ...Feature) Set {
	for _, x := range f {
		s.bits |= closure(x)
	}
	return s
}

// Remove returns s without f and without anything that requires f.
func (s Set) Remove(f ...Feature) Set {
	for _, x := range f {
		s.bits &^= reverseClosure(x)
	}
	return s
}

// Missing returns the features want has that s does not. The result is what
// a gating diagnostic names.
func (s Set) Missing(want Set) Set {
	return Set{level: want.level, bits: want.bits &^ s.bits}
}

// Empty reports whether no extension is active. It says nothing about level.
func (s Set) Empty() bool { return s.bits == 0 }

// Features returns the active extensions in canonical order.
func (s Set) Features() []Feature {
	var out []Feature
	for f := Feature(0); f < numFeatures; f++ {
		if s.Has(f) {
			out = append(out, f)
		}
	}
	return out
}

// String is the canonical spelling: the level, then each active extension in
// canonical order, joined by '+'. This is the one spelling out — arc env,
// every diagnostic, and arc targets all print this form regardless of how the
// set was spelled going in.
func (s Set) String() string {
	var b strings.Builder
	b.WriteString(s.level.String())
	for _, f := range s.Features() {
		b.WriteByte('+')
		b.WriteString(f.String())
	}
	return b.String()
}

func closure(f Feature) uint64 {
	if f >= numFeatures {
		return 0
	}
	bits := uint64(1) << f
	for _, r := range requires[f] {
		bits |= closure(r)
	}
	return bits
}

func reverseClosure(f Feature) uint64 {
	if f >= numFeatures {
		return 0
	}
	bits := uint64(1) << f
	for _, r := range requiredBy[f] {
		bits |= reverseClosure(r)
	}
	return bits
}