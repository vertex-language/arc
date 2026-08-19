// x86_64/feature.go
//
// The re-export of feature/, so a caller writing x86_64.V3 or
// x86_64.AVX512F never needs the second import.
//
// These are aliases and not definitions. A defined type would be a second
// type with the same shape, and every call into isa/ or encode/ would need
// a conversion — which is a conversion that compiles in both directions and
// therefore a conversion nobody would notice getting wrong.
package x86_64

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/feature"
)

// Feature is one instruction-set extension.
type Feature = feature.Feature

// FeatureSet is a closed set of features. It is a value: comparable with ==,
// usable as a map key, and always closed under requirements.
type FeatureSet = feature.Set

// Level is a microarchitecture level: shorthand for a closed set, never a
// separate axis. Gating happens against features; a level appears only in
// spelling.
type Level = feature.Level

const (
	V1 Level = feature.V1
	V2 Level = feature.V2
	V3 Level = feature.V3
	V4 Level = feature.V4
)

// Base is the unextended 64-bit instruction set. Every set contains it.
const Base Feature = feature.Base

// Vector, in dependency order.
const (
	MMX   Feature = feature.MMX
	SSE   Feature = feature.SSE
	SSE2  Feature = feature.SSE2
	SSE3  Feature = feature.SSE3
	SSSE3 Feature = feature.SSSE3
	SSE41 Feature = feature.SSE41
	SSE42 Feature = feature.SSE42
	AVX   Feature = feature.AVX
	AVX2  Feature = feature.AVX2
)

// Scalar extensions above the baseline.
const (
	POPCNT     Feature = feature.POPCNT
	CMPXCHG16B Feature = feature.CMPXCHG16B
	LAHFSAHF   Feature = feature.LAHFSAHF
	LZCNT      Feature = feature.LZCNT
	MOVBE      Feature = feature.MOVBE
	BMI1       Feature = feature.BMI1
	BMI2       Feature = feature.BMI2
	ADX        Feature = feature.ADX
	F16C       Feature = feature.F16C
	FMA        Feature = feature.FMA
	FSGSBASE   Feature = feature.FSGSBASE
	RDRAND     Feature = feature.RDRAND
	RDSEED     Feature = feature.RDSEED
)

// Crypto.
const (
	AES        Feature = feature.AES
	VAES       Feature = feature.VAES
	PCLMULQDQ  Feature = feature.PCLMULQDQ
	VPCLMULQDQ Feature = feature.VPCLMULQDQ
	SHA        Feature = feature.SHA
)

// AVX-512.
const (
	AVX512F         Feature = feature.AVX512F
	AVX512CD        Feature = feature.AVX512CD
	AVX512BW        Feature = feature.AVX512BW
	AVX512DQ        Feature = feature.AVX512DQ
	AVX512VL        Feature = feature.AVX512VL
	AVX512IFMA      Feature = feature.AVX512IFMA
	AVX512VBMI      Feature = feature.AVX512VBMI
	AVX512VBMI2     Feature = feature.AVX512VBMI2
	AVX512VNNI      Feature = feature.AVX512VNNI
	AVX512BITALG    Feature = feature.AVX512BITALG
	AVX512VPOPCNTDQ Feature = feature.AVX512VPOPCNTDQ
	AVX512BF16      Feature = feature.AVX512BF16
	AVX512FP16      Feature = feature.AVX512FP16
)

// AMX. Not part of any level: `arc build -t x86_64-elf --features x86-64-v4`
// does not get you a tile register.
const (
	AMXTILE Feature = feature.AMXTILE
	AMXINT8 Feature = feature.AMXINT8
	AMXBF16 Feature = feature.AMXBF16
)

// ParseFeatures parses a feature-set spelling: a leading level, then any
// number of +name and -name terms.
//
//	x86-64-v2+avx512f
//	sse2+aes+pclmulqdq
//	x86-64-v4-avx512vl
//
// Without a leading level the set starts empty rather than at Baseline, so
// "sse2" means sse2 and not "v1 plus sse2". Those differ, and silently
// widening the set is how a gating diagnostic stops being falsifiable.
func ParseFeatures(s string) (FeatureSet, error) { return feature.ParseFeatures(s) }

// ParseFeature resolves one feature name. Aliases — sse4_1, abm, cx16 —
// resolve here and do not survive into the canonical spelling.
func ParseFeature(s string) (Feature, error) { return feature.ParseFeature(s) }

// Empty is the set with nothing above Base. It is not Baseline: Baseline is
// V1, which has SSE2. Empty exists for a caller building a set from scratch.
func Empty() FeatureSet { return feature.Empty() }

// Features is every feature this target knows, in declaration order.
func Features() []Feature { return feature.All() }

// GoExpr renders a set as the Go expression that would build it, qualified
// with this package's name:
//
//	x86_64.V1.Plus(x86_64.AVX512F)
//
// This is the note line of a gating diagnostic. feature/ returns the
// unqualified form because it does not know whether it is being read
// through x86_64 or through feature, and the qualifier is prepended here —
// per identifier, from Decompose, rather than by patching the string, so a
// feature whose name contains a level's name cannot corrupt it.
func GoExpr(s FeatureSet) string {
	const pkg = "x86_64."

	lvl, extras := s.Decompose()

	var b strings.Builder
	if lvl == feature.LevelNone {
		b.WriteString(pkg + "Empty()")
	} else {
		b.WriteString(pkg + lvl.GoName())
	}
	if len(extras) > 0 {
		b.WriteString(".Plus(")
		for i, f := range extras {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(pkg + f.GoName())
		}
		b.WriteByte(')')
	}
	return b.String()
}