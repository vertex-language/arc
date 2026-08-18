package i386

import "github.com/vertex-language/arc/i386/feature"

// Re-exports of feature/, so a caller of this package never needs the
// second import. i386.FeatureSet and i386/feature.Set are the same value
// under two spellings — this file is the boundary that makes that true,
// not a copy of the type.
//
// feature/ is the sink of this package's subpackages: everything above may
// import it and it imports nothing back, which is what lets isa/ express a
// gate as a typed feature.Set rather than an opaque bitmask this package
// would have to interpret. That is the whole of "arc isa mov --features
// avx2 cannot drift from what arc build accepts" — filter and gate are the
// same type from the same package, just re-exported here for convenience.

// FeatureSet is a set of active extensions over a base CPU level.
type FeatureSet = feature.Set

// Level is a point on the base-CPU ladder: I386, I486, I586, I686.
type Level = feature.Level

const (
	I386 = feature.I386
	I486 = feature.I486
	I586 = feature.I586
	I686 = feature.I686
)

// The extensions. Declared here in feature/'s own canonical order — the
// order every diagnostic and arc env prints a set in — because re-exporting
// out of order would make this file a second, silently divergent listing.
const (
	MMX      = feature.MMX
	FXSR     = feature.FXSR
	SSE      = feature.SSE
	SSE2     = feature.SSE2
	SSE3     = feature.SSE3
	SSSE3    = feature.SSSE3
	SSE41    = feature.SSE41
	SSE42    = feature.SSE42
	POPCNT   = feature.POPCNT
	AES      = feature.AES
	PCLMUL   = feature.PCLMUL
	XSAVE    = feature.XSAVE
	AVX      = feature.AVX
	F16C     = feature.F16C
	FMA      = feature.FMA
	AVX2     = feature.AVX2
	BMI1     = feature.BMI1
	BMI2     = feature.BMI2
	LZCNT    = feature.LZCNT
	MOVBE    = feature.MOVBE
	ADX      = feature.ADX
	RDRAND   = feature.RDRAND
	RDSEED   = feature.RDSEED
	SHA      = feature.SHA
	AVX512F  = feature.AVX512F
	AVX512CD = feature.AVX512CD
	AVX512VL = feature.AVX512VL
	AVX512BW = feature.AVX512BW
	AVX512DQ = feature.AVX512DQ
)

// NewFeatureSet returns the set for a level with no extensions over it.
func NewFeatureSet(l Level) FeatureSet { return feature.New(l) }

// DefaultFeatures is the set arc assembles for when --features selects
// nothing: Baseline, no extensions.
func DefaultFeatures() FeatureSet { return feature.Default() }

// ParseFeatures resolves a --features value against a starting set — the
// exact syntax feature.Parse documents: an exact list, or +/- adjustments
// against base, never mixed.
func ParseFeatures(base FeatureSet, s string) (FeatureSet, error) {
	return feature.Parse(base, s)
}

// FeatureHelp is the body of --features help.
func FeatureHelp() string { return feature.Help() }