package aarch64

import "github.com/vertex-language/arc/aarch64/feature"

// Re-export of feature/.
//
// These are type and constant aliases, not parallel definitions: aarch64.LSE
// and feature.LSE are the same value of the same type, so a helper taking a
// feature.Feature accepts either spelling and a caller never needs the second
// import.

// FeatureSet is a closed set of extensions. It is always closed under
// requirements in both directions: adding SVE2 pulls in SVE, and dropping FP16
// drops everything that needs it.
type FeatureSet = feature.Set

// Feature is one architectural extension.
type Feature = feature.Feature

// Level is an architecture version: a name for a closed set of features, not a
// separate axis.
type Level = feature.Level

// The architecture versions.
//
// The ladder is not a chain. Armv9-A is built on Armv8.5-A rather than on
// Armv8.9-A, so Armv9A does not contain MOPS while Armv8_8A does, and neither
// level contains the other.
const (
	Armv8A   = feature.Armv8A
	Armv8_1A = feature.Armv8_1A
	Armv8_2A = feature.Armv8_2A
	Armv8_3A = feature.Armv8_3A
	Armv8_4A = feature.Armv8_4A
	Armv8_5A = feature.Armv8_5A
	Armv8_6A = feature.Armv8_6A
	Armv8_7A = feature.Armv8_7A
	Armv8_8A = feature.Armv8_8A
	Armv8_9A = feature.Armv8_9A

	Armv9A   = feature.Armv9A
	Armv9_1A = feature.Armv9_1A
	Armv9_2A = feature.Armv9_2A
	Armv9_3A = feature.Armv9_3A
	Armv9_4A = feature.Armv9_4A
	Armv9_5A = feature.Armv9_5A
)

// The extensions.
//
// A version is shorthand for a closed set of these, and the two overlap rather
// than nesting: CRC is optional at Armv8-A and mandatory from Armv8.1-A, so
// Armv8A.Plus(CRC) and Armv8_1A are different sets that share most of their
// members and each prints as the thing the world calls it.
const (
	// Floating point and Advanced SIMD.
	FP       = feature.FP
	SIMD     = feature.SIMD
	FP16     = feature.FP16
	FP16FML  = feature.FP16FML
	BF16     = feature.BF16
	I8MM     = feature.I8MM
	DotProd  = feature.DotProd
	FCMA     = feature.FCMA
	JSCVT    = feature.JSCVT
	FRIntTS  = feature.FRIntTS
	FAMINMAX = feature.FAMINMAX
	LUT      = feature.LUT

	// Cryptography.
	AES  = feature.AES
	SHA2 = feature.SHA2
	SHA3 = feature.SHA3
	SM4  = feature.SM4

	// Atomics, ordering and memory.
	LSE    = feature.LSE
	LSE128 = feature.LSE128
	D128   = feature.D128
	RCPC   = feature.RCPC
	RCPC2  = feature.RCPC2
	RCPC3  = feature.RCPC3
	LS64   = feature.LS64
	MOPS   = feature.MOPS
	XS     = feature.XS
	THE    = feature.THE

	// Integer and flag manipulation.
	CRC    = feature.CRC
	RDMA   = feature.RDMA
	FlagM  = feature.FlagM
	FlagM2 = feature.FlagM2
	CSSC   = feature.CSSC
	HBC    = feature.HBC
	WFxT   = feature.WFxT

	// Security and control flow.
	PAuth   = feature.PAuth
	BTI     = feature.BTI
	MemTag  = feature.MemTag
	SB      = feature.SB
	SSBS    = feature.SSBS
	PredRes = feature.PredRes
	GCS     = feature.GCS
	CPA     = feature.CPA
	RNG     = feature.RNG
	TME     = feature.TME

	// Scalable Vector Extension.
	SVE         = feature.SVE
	SVE2        = feature.SVE2
	SVE2AES     = feature.SVE2AES
	SVE2SM4     = feature.SVE2SM4
	SVE2SHA3    = feature.SVE2SHA3
	SVE2BitPerm = feature.SVE2BitPerm
	SVE2p1      = feature.SVE2p1
	SVEB16B16   = feature.SVEB16B16
	F32MM       = feature.F32MM
	F64MM       = feature.F64MM

	// Scalable Matrix Extension.
	SME       = feature.SME
	SMEI16I64 = feature.SMEI16I64
	SMEF64F64 = feature.SMEF64F64
	SME2      = feature.SME2
	SME2p1    = feature.SME2p1
	SMEB16B16 = feature.SMEB16B16
	SMEF16F16 = feature.SMEF16F16

	// 8-bit floating point.
	FP8     = feature.FP8
	FP8FMA  = feature.FP8FMA
	FP8DOT4 = feature.FP8DOT4
	FP8DOT2 = feature.FP8DOT2

	// Profiling.
	Profile = feature.Profile
)

// NewFeatures builds a closed set from a list of extensions.
func NewFeatures(fs ...Feature) FeatureSet { return feature.NewSet(fs...) }

// ParseFeatures reads the +ext / +noext grammar the world writes:
//
//	armv8.2-a+sve+nofp16
//	armv9-a+sme2
//	+lse+crc
//
// Modifiers apply left to right, which is what makes "removals follow
// additions" work and what makes the rightmost of two conflicting modifiers the
// one that stands. Without a leading version the set starts empty rather than
// at Baseline, so "sve" means SVE and its requirements alone.
func ParseFeatures(spec string) (FeatureSet, error) { return feature.ParseFeatures(spec) }

// ParseFeature resolves one extension name, as written after a '+'.
func ParseFeature(name string) (Feature, error) { return feature.ParseFeature(name) }

// ParseLevel resolves an architecture version name.
func ParseLevel(name string) (Level, error) { return feature.ParseLevel(name) }

// MustParseFeatures is ParseFeatures for a constant spec in package-level
// initialization. It panics rather than returning an error.
func MustParseFeatures(spec string) FeatureSet { return feature.MustParseFeatures(spec) }

// Levels returns every architecture version, in declaration order.
func Levels() []Level { return feature.Levels() }

// Features returns every extension, in declaration order.
func Features() []Feature { return feature.All() }