// Package feature is the AArch64 extension vocabulary and the gating that goes
// with it.
//
// A Set is always closed under requirements. There is no way to build one
// holding SVE2 without SVE, and dropping a feature drops everything that
// depends on it. The closure is computed from the edges in this file; nothing
// tabulates a closed set by hand, because a hand-written closure and a
// dependency edge disagree eventually and the disagreement is silent.
package feature

// Feature is one architectural extension.
//
// The zero value is not a feature. Names are the ones GNU as and GCC accept
// after a '+', which is the spelling the world writes and the only one this
// package prints.
type Feature uint8

const (
	None Feature = iota

	// Floating point and Advanced SIMD.
	FP
	SIMD
	FP16
	FP16FML
	BF16
	I8MM
	DotProd
	FCMA
	JSCVT
	FRIntTS
	FAMINMAX
	LUT

	// Cryptography.
	AES
	SHA2
	SHA3
	SM4

	// Atomics, ordering and memory.
	LSE
	LSE128
	D128
	RCPC
	RCPC2
	RCPC3
	LS64
	MOPS
	XS
	THE

	// Integer and flag manipulation.
	CRC
	RDMA
	FlagM
	FlagM2
	CSSC
	HBC
	WFxT

	// Security and control flow.
	PAuth
	BTI
	MemTag
	SB
	SSBS
	PredRes
	GCS
	CPA
	RNG
	TME

	// Scalable Vector Extension.
	SVE
	SVE2
	SVE2AES
	SVE2SM4
	SVE2SHA3
	SVE2BitPerm
	SVE2p1
	SVEB16B16
	F32MM
	F64MM

	// Scalable Matrix Extension.
	SME
	SMEI16I64
	SMEF64F64
	SME2
	SME2p1
	SMEB16B16
	SMEF16F16

	// 8-bit floating point.
	FP8
	FP8FMA
	FP8DOT4
	FP8DOT2

	// Profiling.
	Profile

	// featureCount is one past the last feature. Adding a feature past 128
	// requires widening setWords below; the compile-time assertion at the
	// bottom of this file catches it.
	featureCount
)

// info is one row of the vocabulary: how the feature is spelled, and what it
// cannot exist without.
//
// Requirements are the architecture's, not a convenience. SVE2AES needs SVE2
// and AES because its instructions are defined in terms of both; a set holding
// it without either describes no processor.
var info = [featureCount]struct {
	name     string
	requires []Feature
}{
	FP:       {"fp", nil},
	SIMD:     {"simd", []Feature{FP}},
	FP16:     {"fp16", []Feature{FP}},
	FP16FML:  {"fp16fml", []Feature{FP16}},
	BF16:     {"bf16", []Feature{SIMD}},
	I8MM:     {"i8mm", []Feature{SIMD}},
	DotProd:  {"dotprod", []Feature{SIMD}},
	FCMA:     {"fcma", []Feature{SIMD}},
	JSCVT:    {"jscvt", []Feature{FP}},
	FRIntTS:  {"frintts", []Feature{FP}},
	FAMINMAX: {"faminmax", []Feature{SIMD}},
	LUT:      {"lut", []Feature{SIMD}},

	AES:  {"aes", []Feature{SIMD}},
	SHA2: {"sha2", []Feature{SIMD}},
	SHA3: {"sha3", []Feature{SIMD}},
	SM4:  {"sm4", []Feature{SIMD}},

	LSE:    {"lse", nil},
	LSE128: {"lse128", []Feature{LSE}},
	D128:   {"d128", []Feature{LSE128}},
	RCPC:   {"rcpc", nil},
	RCPC2:  {"rcpc2", []Feature{RCPC}},
	RCPC3:  {"rcpc3", []Feature{RCPC2}},
	LS64:   {"ls64", nil},
	MOPS:   {"mops", nil},
	XS:     {"xs", nil},
	THE:    {"the", nil},

	CRC:    {"crc", nil},
	RDMA:   {"rdma", []Feature{SIMD}},
	FlagM:  {"flagm", nil},
	FlagM2: {"flagm2", []Feature{FlagM}},
	CSSC:   {"cssc", nil},
	HBC:    {"hbc", nil},
	WFxT:   {"wfxt", nil},

	PAuth:   {"pauth", nil},
	BTI:     {"bti", nil},
	MemTag:  {"memtag", nil},
	SB:      {"sb", nil},
	SSBS:    {"ssbs", nil},
	PredRes: {"predres", nil},
	GCS:     {"gcs", nil},
	CPA:     {"cpa", nil},
	RNG:     {"rng", nil},
	TME:     {"tme", nil},

	SVE:         {"sve", []Feature{SIMD}},
	SVE2:        {"sve2", []Feature{SVE}},
	SVE2AES:     {"sve2-aes", []Feature{SVE2, AES}},
	SVE2SM4:     {"sve2-sm4", []Feature{SVE2, SM4}},
	SVE2SHA3:    {"sve2-sha3", []Feature{SVE2, SHA3}},
	SVE2BitPerm: {"sve2-bitperm", []Feature{SVE2}},
	SVE2p1:      {"sve2p1", []Feature{SVE2}},
	SVEB16B16:   {"sve-b16b16", []Feature{SVE2, BF16}},
	F32MM:       {"f32mm", []Feature{SVE}},
	F64MM:       {"f64mm", []Feature{SVE}},

	// SME requires BF16 architecturally and does not require SVE: the Z and P
	// registers it uses are SVE's, but a processor may implement SME with
	// streaming mode only. Treating SME as needing SVE would refuse a legal
	// target rather than a wrong instruction.
	SME:        {"sme", []Feature{BF16}},
	SMEI16I64:  {"sme-i16i64", []Feature{SME}},
	SMEF64F64:  {"sme-f64f64", []Feature{SME}},
	SME2:       {"sme2", []Feature{SME}},
	SME2p1:     {"sme2p1", []Feature{SME2}},
	SMEB16B16:  {"sme-b16b16", []Feature{SME2, SVEB16B16}},
	SMEF16F16:  {"sme-f16f16", []Feature{SME2}},

	FP8:     {"fp8", []Feature{SIMD}},
	FP8FMA:  {"fp8fma", []Feature{FP8}},
	FP8DOT4: {"fp8dot4", []Feature{FP8}},
	FP8DOT2: {"fp8dot2", []Feature{FP8}},

	Profile: {"profile", nil},
}

// Valid reports whether f names a feature.
func (f Feature) Valid() bool { return f > None && f < featureCount }

// String is the canonical spelling, the one that goes after a '+'.
func (f Feature) String() string {
	if !f.Valid() {
		return "none"
	}
	return info[f].name
}

// Requires lists the features this one directly depends on. The transitive
// closure is what Set.Plus applies; this is the edge list it walks.
func (f Feature) Requires() []Feature {
	if !f.Valid() {
		return nil
	}
	return info[f].requires
}

// All returns every feature in declaration order.
func All() []Feature {
	out := make([]Feature, 0, featureCount-1)
	for f := None + 1; f < featureCount; f++ {
		out = append(out, f)
	}
	return out
}

const setWords = 2

// Set is a closed set of features.
//
// It is a value, comparable with ==, and every operation returns a new one.
// A set that could be mutated in place would let a caller hold a reference to
// an encoder's feature set and change it midway through an object, which is
// the thing target.go's missing SetFeatures is there to prevent.
type Set struct {
	w [setWords]uint64
}

func (s Set) has(f Feature) bool {
	if !f.Valid() {
		return false
	}
	return s.w[f/64]&(1<<(f%64)) != 0
}

func (s Set) set(f Feature) Set {
	if f.Valid() {
		s.w[f/64] |= 1 << (f % 64)
	}
	return s
}

func (s Set) clear(f Feature) Set {
	if f.Valid() {
		s.w[f/64] &^= 1 << (f % 64)
	}
	return s
}

// Has reports whether the set enables a feature.
func (s Set) Has(f Feature) bool { return s.has(f) }

// Empty reports whether the set enables nothing.
func (s Set) Empty() bool {
	for _, w := range s.w {
		if w != 0 {
			return false
		}
	}
	return true
}

// Len is the number of features enabled, requirements included.
func (s Set) Len() int {
	n := 0
	for f := None + 1; f < featureCount; f++ {
		if s.has(f) {
			n++
		}
	}
	return n
}

// Features lists everything enabled, in declaration order.
func (s Set) Features() []Feature {
	var out []Feature
	for f := None + 1; f < featureCount; f++ {
		if s.has(f) {
			out = append(out, f)
		}
	}
	return out
}

// Contains reports whether every feature of other is enabled here.
func (s Set) Contains(other Set) bool {
	for i := range s.w {
		if other.w[i]&^s.w[i] != 0 {
			return false
		}
	}
	return true
}

// Union enables everything either set enables. Both are already closed, so the
// result is too.
func (s Set) Union(other Set) Set {
	for i := range s.w {
		s.w[i] |= other.w[i]
	}
	return s
}

// Plus enables features and everything they require.
func (s Set) Plus(fs ...Feature) Set {
	for _, f := range fs {
		s = s.addClosed(f)
	}
	return s
}

func (s Set) addClosed(f Feature) Set {
	if !f.Valid() || s.has(f) {
		return s
	}
	s = s.set(f)
	for _, r := range info[f].requires {
		s = s.addClosed(r)
	}
	return s
}

// Minus disables features and everything that requires them.
//
// The downward direction is the one people forget. Dropping FP16 has to drop
// FP16FML, because an FP16FML instruction is defined in terms of FP16 and a set
// holding one without the other would let the encoder emit it and the hardware
// refuse it.
func (s Set) Minus(fs ...Feature) Set {
	for _, f := range fs {
		s = s.removeClosed(f)
	}
	return s
}

func (s Set) removeClosed(f Feature) Set {
	if !f.Valid() {
		return s
	}
	s = s.clear(f)
	// Fixpoint rather than a reverse edge list: a dependent may itself have
	// dependents, and the table only stores edges upward.
	for changed := true; changed; {
		changed = false
		for g := None + 1; g < featureCount; g++ {
			if !s.has(g) {
				continue
			}
			for _, r := range info[g].requires {
				if !s.has(r) {
					s = s.clear(g)
					changed = true
					break
				}
			}
		}
	}
	return s
}

// NewSet builds a closed set from a list of features.
func NewSet(fs ...Feature) Set {
	var s Set
	return s.Plus(fs...)
}

// compile-time assertion that the bitset is wide enough.
var _ = [1]struct{}{}[featureCount/(setWords*64)]