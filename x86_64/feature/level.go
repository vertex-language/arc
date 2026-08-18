// x86_64/feature/level.go
package feature

// Level is a microarchitecture level: shorthand for a closed set of features,
// not a separate axis. Every level is a Set and nothing more, which is why
// gating happens against features and levels only ever appear in spelling.
//
// The levels are cumulative — v1 ⊂ v2 ⊂ v3 ⊂ v4.
type Level uint8

const (
	// LevelNone is "no level fully contained", the answer Decompose gives
	// for a set like sse2+aes. It is not a level you can ask for.
	LevelNone Level = iota
	V1
	V2
	V3
	V4
)

// levelAdds is what each level contributes over the one below it, exactly as
// the psABI states it. Closure fills in the rest: naming SSE2 at v1 pulls in
// SSE, and naming AVX2 at v3 pulls in AVX and everything under it.
//
// The psABI also lists CMOV, CX8, FPU, FXSR, OSFXSR and SCE at v1 and OSXSAVE
// at v3. Those are identification and OS-enablement bits rather than
// encodings, so they are not in this vocabulary and not listed here.
var levelAdds = [5][]Feature{
	V1: {MMX, SSE2},
	V2: {SSE42, POPCNT, CMPXCHG16B, LAHFSAHF},
	V3: {AVX2, BMI1, BMI2, F16C, FMA, LZCNT, MOVBE},
	V4: {AVX512F, AVX512BW, AVX512CD, AVX512DQ, AVX512VL},
}

var levelSets [5]Set

func init() {
	var acc Set
	for l := V1; l <= V4; l++ {
		acc = acc.Plus(levelAdds[l]...)
		levelSets[l] = acc
	}
}

// Set is the closed feature set this level names.
func (l Level) Set() Set {
	if l == LevelNone || int(l) >= len(levelSets) {
		return Set{}
	}
	return levelSets[l]
}

// Plus is the level's set with extra features enabled, closed as usual.
//
//	V1.Plus(AVX512F)
func (l Level) Plus(fs ...Feature) Set { return l.Set().Plus(fs...) }

// Has reports whether the level includes f.
func (l Level) Has(f Feature) bool { return l.Set().Has(f) }

func (l Level) String() string {
	switch l {
	case V1:
		return "x86-64-v1"
	case V2:
		return "x86-64-v2"
	case V3:
		return "x86-64-v3"
	case V4:
		return "x86-64-v4"
	}
	return "none"
}

// GoName is the exported Go identifier, unqualified.
func (l Level) GoName() string {
	switch l {
	case V1:
		return "V1"
	case V2:
		return "V2"
	case V3:
		return "V3"
	case V4:
		return "V4"
	}
	return ""
}

// Baseline is the default feature set: V1, which is SSE2 and nothing above it.
func Baseline() Set { return V1.Set() }