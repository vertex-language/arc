package feature

// A level is an architecture version: a name for a closed set of features, not
// a separate axis.
//
// The ladder is not a chain. Armv9-A is built on Armv8.5-A, not on Armv8.9-A,
// so Armv9A does not contain MOPS while Armv8_8A does, and neither level
// contains the other. Decompose has to search both chains rather than walk one.
//
// It is also not monotone in the way x86-64's levels are. CRC is optional at
// Armv8-A and mandatory from Armv8.1-A, so Armv8A.Plus(CRC) and Armv8_1A are
// different sets that overlap, and each prints as the thing the world calls it:
// "armv8-a+crc" and "armv8.1-a".
type Level uint8

const (
	LevelNone Level = iota

	Armv8A
	Armv8_1A
	Armv8_2A
	Armv8_3A
	Armv8_4A
	Armv8_5A
	Armv8_6A
	Armv8_7A
	Armv8_8A
	Armv8_9A

	Armv9A
	Armv9_1A
	Armv9_2A
	Armv9_3A
	Armv9_4A
	Armv9_5A

	levelCount
)

// levelSpec is a level stated the way the architecture states it: as a base
// plus what that version made mandatory. The closed set is derived.
var levelSpec = [levelCount]struct {
	name string
	base Level
	adds []Feature
}{
	Armv8A:   {"armv8-a", LevelNone, []Feature{FP, SIMD}},
	Armv8_1A: {"armv8.1-a", Armv8A, []Feature{CRC, LSE, RDMA}},
	Armv8_2A: {"armv8.2-a", Armv8_1A, nil},
	Armv8_3A: {"armv8.3-a", Armv8_2A, []Feature{PAuth, FCMA, JSCVT}},
	Armv8_4A: {"armv8.4-a", Armv8_3A, []Feature{FlagM, FP16FML, DotProd, RCPC2}},
	Armv8_5A: {"armv8.5-a", Armv8_4A, []Feature{SB, SSBS, PredRes, FRIntTS, FlagM2}},
	Armv8_6A: {"armv8.6-a", Armv8_5A, []Feature{BF16, I8MM}},
	Armv8_7A: {"armv8.7-a", Armv8_6A, []Feature{WFxT, XS}},
	Armv8_8A: {"armv8.8-a", Armv8_7A, []Feature{MOPS}},
	Armv8_9A: {"armv8.9-a", Armv8_8A, nil},

	// Armv9-A branches from Armv8.5-A, not from the top of the v8 chain.
	Armv9A:   {"armv9-a", Armv8_5A, []Feature{SVE, SVE2}},
	Armv9_1A: {"armv9.1-a", Armv9A, []Feature{BF16, I8MM}},
	Armv9_2A: {"armv9.2-a", Armv9_1A, []Feature{WFxT, XS}},
	Armv9_3A: {"armv9.3-a", Armv9_2A, []Feature{MOPS}},
	Armv9_4A: {"armv9.4-a", Armv9_3A, []Feature{SVE2p1}},
	Armv9_5A: {"armv9.5-a", Armv9_4A, []Feature{CPA, FAMINMAX, LUT}},
}

var levelSet = buildLevelSets()

func buildLevelSets() [levelCount]Set {
	var out [levelCount]Set
	var build func(Level) Set
	build = func(l Level) Set {
		if l == LevelNone {
			return Set{}
		}
		spec := levelSpec[l]
		return build(spec.base).Plus(spec.adds...)
	}
	for l := LevelNone + 1; l < levelCount; l++ {
		out[l] = build(l)
	}
	return out
}

// Valid reports whether l names a level.
func (l Level) Valid() bool { return l > LevelNone && l < levelCount }

// String is the canonical spelling: armv8.2-a, armv9-a.
func (l Level) String() string {
	if !l.Valid() {
		return "none"
	}
	return levelSpec[l].name
}

// Set is the closed set of features this version makes mandatory.
func (l Level) Set() Set {
	if !l.Valid() {
		return Set{}
	}
	return levelSet[l]
}

// Has reports whether the version makes a feature mandatory.
func (l Level) Has(f Feature) bool { return l.Set().Has(f) }

// Plus is the version plus extensions it leaves optional, closed.
func (l Level) Plus(fs ...Feature) Set { return l.Set().Plus(fs...) }

// Minus is the version with features removed, along with their dependents. The
// result no longer describes that version and will not print as it.
func (l Level) Minus(fs ...Feature) Set { return l.Set().Minus(fs...) }

// Levels returns every level in declaration order.
func Levels() []Level {
	out := make([]Level, 0, levelCount-1)
	for l := LevelNone + 1; l < levelCount; l++ {
		out = append(out, l)
	}
	return out
}

// Baseline is the default feature set: Armv8-A, floating point and Advanced
// SIMD and nothing above them.
func Baseline() Set { return Armv8A.Set() }

// Decompose names a set: the highest level it fully contains, plus the minimal
// list of extra features needed to reach the rest.
//
// Both String and GoExpr go through this, so a diagnostic's note line and a
// set's printed form can never disagree about what a set is called.
//
// ok is false when no level is fully contained — a set built from bare features
// with no base, which prints as its features alone.
func Decompose(s Set) (l Level, ok bool, extra []Feature) {
	best := LevelNone
	bestLen := -1
	for c := LevelNone + 1; c < levelCount; c++ {
		cs := levelSet[c]
		if !s.Contains(cs) {
			continue
		}
		// Ties go to the later declaration, which is the further-along
		// version; between two levels of equal size the more specific name is
		// the one a reader expects.
		if n := cs.Len(); n >= bestLen {
			best, bestLen = c, n
		}
	}

	rest := s
	if best != LevelNone {
		for _, f := range levelSet[best].Features() {
			rest = rest.clear(f)
		}
	}

	// Reduce what is left to generators: drop anything another remaining
	// feature already pulls in, so "sve2" does not print as "sve+sve2".
	var gens []Feature
	for _, f := range rest.Features() {
		implied := false
		for _, g := range rest.Features() {
			if g != f && NewSet(g).Has(f) {
				implied = true
				break
			}
		}
		if !implied {
			gens = append(gens, f)
		}
	}
	return best, best != LevelNone, gens
}

// String prints a set the way the world writes it: a version name followed by
// the extensions above it.
//
// Removals never appear. A set that drops something a version made mandatory is
// no longer that version, and prints as the highest version it does satisfy
// plus whatever it kept.
func (s Set) String() string {
	l, ok, extra := Decompose(s)
	if !ok && len(extra) == 0 {
		return "none"
	}
	out := ""
	if ok {
		out = l.String()
	}
	for _, f := range extra {
		out += "+" + f.String()
	}
	return out
}

// GoExpr prints the Go expression that builds this set, for the note line of a
// gating diagnostic.
func (s Set) GoExpr() string {
	l, ok, extra := Decompose(s)
	if !ok && len(extra) == 0 {
		return "aarch64.Set{}"
	}
	base := "aarch64.Baseline()"
	if ok {
		base = "aarch64." + goLevelName(l)
	}
	if len(extra) == 0 {
		return base
	}
	out := base + ".Plus("
	for i, f := range extra {
		if i > 0 {
			out += ", "
		}
		out += "aarch64." + goFeatureName(f)
	}
	return out + ")"
}