// x86_64/feature/feature.go
//
// Package feature is the extension vocabulary this target gates encodings on,
// and the microarchitecture levels that name closed sets of them.
//
// The vocabulary is instruction-set extensions only. The psABI's level
// definitions also list CMOV, CX8, FPU, FXSR, OSFXSR, SCE and OSXSAVE; those
// report CPU identification or that the OS has enabled a state-save area, not
// that an encoding exists. An encoder gates on "may I emit these bytes", so
// they are absent here.
//
// Unratified extensions are absent too, and not declared-and-refused the way
// an unmapped relocation is: a relocation kind that exists in the format but
// has no mapping is a gap, whereas an unratified extension has no bytes to
// name yet. AVX512ER and AVX512PF shipped on Knights Landing only and are
// gone from every current part; they are excluded on the same footing.
package feature

// Feature is one instruction-set extension.
type Feature uint8

// Base is the unextended 64-bit instruction set: everything long mode gives
// you before any extension. Every Set contains it, so isa/ can gate every
// form on a Feature and never carry a nil.
const Base Feature = 0

const (
	// Vector, in dependency order.
	MMX Feature = iota + 1
	SSE
	SSE2
	SSE3
	SSSE3
	SSE41
	SSE42
	AVX
	AVX2

	// Scalar extensions above the baseline.
	POPCNT
	CMPXCHG16B
	LAHFSAHF
	LZCNT
	MOVBE
	BMI1
	BMI2
	ADX
	F16C
	FMA
	FSGSBASE
	RDRAND
	RDSEED

	// Crypto.
	AES
	VAES
	PCLMULQDQ
	VPCLMULQDQ
	SHA

	// AVX-512. Each is closed under its own requirements; see requires.
	AVX512F
	AVX512CD
	AVX512BW
	AVX512DQ
	AVX512VL
	AVX512IFMA
	AVX512VBMI
	AVX512VBMI2
	AVX512VNNI
	AVX512BITALG
	AVX512VPOPCNTDQ
	AVX512BF16
	AVX512FP16

	// AMX.
	AMXTILE
	AMXINT8
	AMXBF16

	numFeatures
)

// requires is the immediate requirement of each feature. Closure is computed,
// not written out: a table of full closures would be a table that can disagree
// with itself.
var requires = [numFeatures][]Feature{
	MMX: nil,
	SSE: nil,

	SSE2:  {SSE},
	SSE3:  {SSE2},
	SSSE3: {SSE3},
	SSE41: {SSSE3},
	SSE42: {SSE41},
	AVX:   {SSE42},
	AVX2:  {AVX},

	POPCNT:     nil,
	CMPXCHG16B: nil,
	LAHFSAHF:   nil,
	LZCNT:      nil,
	MOVBE:      nil,
	BMI1:       nil,
	BMI2:       nil, // architecturally independent of BMI1
	ADX:        nil,
	FSGSBASE:   nil,
	RDRAND:     nil,
	RDSEED:     nil,

	F16C: {AVX},
	FMA:  {AVX},

	AES:        {SSE2},
	PCLMULQDQ:  {SSE2},
	SHA:        {SSE2},
	VAES:       {AES, AVX},
	VPCLMULQDQ: {PCLMULQDQ, AVX},

	AVX512F:         {AVX2},
	AVX512CD:        {AVX512F},
	AVX512BW:        {AVX512F},
	AVX512DQ:        {AVX512F},
	AVX512VL:        {AVX512F},
	AVX512IFMA:      {AVX512F},
	AVX512VNNI:      {AVX512F},
	AVX512VPOPCNTDQ: {AVX512F},
	AVX512BF16:      {AVX512F},
	AVX512VBMI:      {AVX512BW},
	AVX512VBMI2:     {AVX512BW},
	AVX512BITALG:    {AVX512BW},
	AVX512FP16:      {AVX512BW},

	AMXTILE:  nil,
	AMXINT8:  {AMXTILE},
	AMXBF16:  {AMXTILE},
}

var names = [numFeatures]string{
	Base: "base",

	MMX:   "mmx",
	SSE:   "sse",
	SSE2:  "sse2",
	SSE3:  "sse3",
	SSSE3: "ssse3",
	SSE41: "sse4.1",
	SSE42: "sse4.2",
	AVX:   "avx",
	AVX2:  "avx2",

	POPCNT:     "popcnt",
	CMPXCHG16B: "cmpxchg16b",
	LAHFSAHF:   "lahf-sahf",
	LZCNT:      "lzcnt",
	MOVBE:      "movbe",
	BMI1:       "bmi1",
	BMI2:       "bmi2",
	ADX:        "adx",
	F16C:       "f16c",
	FMA:        "fma",
	FSGSBASE:   "fsgsbase",
	RDRAND:     "rdrand",
	RDSEED:     "rdseed",

	AES:        "aes",
	VAES:       "vaes",
	PCLMULQDQ:  "pclmulqdq",
	VPCLMULQDQ: "vpclmulqdq",
	SHA:        "sha",

	AVX512F:         "avx512f",
	AVX512CD:        "avx512cd",
	AVX512BW:        "avx512bw",
	AVX512DQ:        "avx512dq",
	AVX512VL:        "avx512vl",
	AVX512IFMA:      "avx512ifma",
	AVX512VBMI:      "avx512vbmi",
	AVX512VBMI2:     "avx512vbmi2",
	AVX512VNNI:      "avx512vnni",
	AVX512BITALG:    "avx512bitalg",
	AVX512VPOPCNTDQ: "avx512vpopcntdq",
	AVX512BF16:      "avx512bf16",
	AVX512FP16:      "avx512fp16",

	AMXTILE: "amx-tile",
	AMXINT8: "amx-int8",
	AMXBF16: "amx-bf16",
}

// goNames is the Go constant each feature is spelled as, for the note line in
// a gating diagnostic. It is the identifier without the package qualifier;
// the caller prepends its own, because this package does not know whether it
// is being read through x86_64 or through feature.
var goNames = [numFeatures]string{
	MMX: "MMX", SSE: "SSE", SSE2: "SSE2", SSE3: "SSE3", SSSE3: "SSSE3",
	SSE41: "SSE41", SSE42: "SSE42", AVX: "AVX", AVX2: "AVX2",
	POPCNT: "POPCNT", CMPXCHG16B: "CMPXCHG16B", LAHFSAHF: "LAHFSAHF",
	LZCNT: "LZCNT", MOVBE: "MOVBE", BMI1: "BMI1", BMI2: "BMI2", ADX: "ADX",
	F16C: "F16C", FMA: "FMA", FSGSBASE: "FSGSBASE", RDRAND: "RDRAND",
	RDSEED: "RDSEED", AES: "AES", VAES: "VAES", PCLMULQDQ: "PCLMULQDQ",
	VPCLMULQDQ: "VPCLMULQDQ", SHA: "SHA",
	AVX512F: "AVX512F", AVX512CD: "AVX512CD", AVX512BW: "AVX512BW",
	AVX512DQ: "AVX512DQ", AVX512VL: "AVX512VL", AVX512IFMA: "AVX512IFMA",
	AVX512VBMI: "AVX512VBMI", AVX512VBMI2: "AVX512VBMI2",
	AVX512VNNI: "AVX512VNNI", AVX512BITALG: "AVX512BITALG",
	AVX512VPOPCNTDQ: "AVX512VPOPCNTDQ", AVX512BF16: "AVX512BF16",
	AVX512FP16: "AVX512FP16",
	AMXTILE: "AMXTILE", AMXINT8: "AMXINT8", AMXBF16: "AMXBF16",
}

// Name is the canonical spelling, the one ParseFeature accepts and diagnostics
// print.
func (f Feature) Name() string {
	if int(f) >= len(names) {
		return "feature(?)"
	}
	return names[f]
}

func (f Feature) String() string { return f.Name() }

// GoName is the exported Go identifier for this feature, unqualified.
func (f Feature) GoName() string {
	if int(f) >= len(goNames) {
		return ""
	}
	return goNames[f]
}

// Requires is what this feature immediately depends on. The transitive
// closure is what Set.Plus adds; this is the edge list it walks.
func (f Feature) Requires() []Feature {
	if int(f) >= len(requires) {
		return nil
	}
	return requires[f]
}

// All is every feature this package knows, in declaration order. The
// generator that writes helpers_*_gen.go reads this to decide which file a
// form's helper lands in.
func All() []Feature {
	out := make([]Feature, 0, numFeatures-1)
	for f := MMX; f < numFeatures; f++ {
		out = append(out, f)
	}
	return out
}