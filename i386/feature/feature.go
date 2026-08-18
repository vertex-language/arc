// Package feature is the i386 feature vocabulary: what may be encoded, and
// under which flag.
//
// Two different things live here and are deliberately not flattened together.
//
// A Level is a point on the linear base-CPU ladder — i386, i486, i586, i686.
// Each level contains every instruction of the levels below it, and the
// instructions it adds have no CPUID feature bit because they predate CPUID.
// There is no way to have i686 without i586.
//
// A Feature is an orthogonal extension with a CPUID feature bit of its own.
// Extensions have requirements among themselves (SSE2 needs SSE) but are not
// a line, and they are independent of the level.
//
// This is the model GNU as uses for 32-bit x86 — -march=i686+sse4 — and it is
// the shape of the silicon rather than a choice.
//
// The x86-64 microarchitecture levels are not here. x86-64-v1 is defined as
// the common capability of the AMD K8 and Intel Prescott and includes SCE, the
// SYSCALL extension; v2 requires CMPXCHG16B. The whole series is anchored to
// the 64-bit baseline and has no 32-bit member, so it is rejected rather than
// approximated. See Parse.
package feature

// Level is a point on the base-CPU ladder. Levels are ordered and cumulative.
type Level uint8

const (
	I386 Level = iota
	I486
	I586
	I686
)

// Baseline is the level arc assembles for when nothing selects another, and
// the value in the BASELINE column of arc targets.
const Baseline = I686

var levelNames = [...]string{"i386", "i486", "i586", "i686"}

func (l Level) String() string {
	if int(l) < len(levelNames) {
		return levelNames[l]
	}
	return "?"
}

// Adds names the instructions a level contributes over the level below it.
// This is documentation for arc isa and diagnostics, not a dispatch table —
// isa/ gates on the Level itself.
func (l Level) Adds() []string {
	switch l {
	case I386:
		return []string{"the 32-bit base instruction set"}
	case I486:
		return []string{"bswap", "cmpxchg", "xadd", "invd", "wbinvd", "invlpg"}
	case I586:
		return []string{"cmpxchg8b", "cpuid", "rdtsc", "rdmsr", "wrmsr"}
	case I686:
		return []string{"cmov", "fcmov", "fcomi", "rdpmc"}
	}
	return nil
}

// Feature is an orthogonal extension.
//
// The order of this block is canonical order: it is the order every diagnostic
// and arc env prints a set in, and the order arc isa lists gates in. Input
// order is free; output order is not. Append only.
type Feature uint8

const (
	MMX Feature = iota
	FXSR
	SSE
	SSE2
	SSE3
	SSSE3
	SSE41
	SSE42
	POPCNT
	AES
	PCLMUL
	XSAVE
	AVX
	F16C
	FMA
	AVX2
	BMI1
	BMI2
	LZCNT
	MOVBE
	ADX
	RDRAND
	RDSEED
	SHA
	AVX512F
	AVX512CD
	AVX512VL
	AVX512BW
	AVX512DQ

	numFeatures
)

// Names are the CPUID-era spellings, lowercase, as LLVM -mattr and GNU as
// extension mnemonics spell them. Alternate spellings resolve in Parse.
var featureNames = [numFeatures]string{
	MMX:      "mmx",
	FXSR:     "fxsr",
	SSE:      "sse",
	SSE2:     "sse2",
	SSE3:     "sse3",
	SSSE3:    "ssse3",
	SSE41:    "sse4.1",
	SSE42:    "sse4.2",
	POPCNT:   "popcnt",
	AES:      "aes",
	PCLMUL:   "pclmul",
	XSAVE:    "xsave",
	AVX:      "avx",
	F16C:     "f16c",
	FMA:      "fma",
	AVX2:     "avx2",
	BMI1:     "bmi1",
	BMI2:     "bmi2",
	LZCNT:    "lzcnt",
	MOVBE:    "movbe",
	ADX:      "adx",
	RDRAND:   "rdrand",
	RDSEED:   "rdseed",
	SHA:      "sha",
	AVX512F:  "avx512f",
	AVX512CD: "avx512cd",
	AVX512VL: "avx512vl",
	AVX512BW: "avx512bw",
	AVX512DQ: "avx512dq",
}

func (f Feature) String() string {
	if f < numFeatures {
		return featureNames[f]
	}
	return "?"
}

// requires is what each extension directly depends on. Every edge is an ISA
// fact: SSE reuses the MMX register file for its integer operations and needs
// FXSR to save state; AVX-512 is defined as an extension of AVX2.
//
// Adding an extension adds its closure. Removing one removes everything whose
// closure contained it, because a set holding AVX2 but not AVX describes no
// silicon and would make a gating diagnostic unfalsifiable.
var requires = [numFeatures][]Feature{
	SSE:      {MMX, FXSR},
	SSE2:     {SSE},
	SSE3:     {SSE2},
	SSSE3:    {SSE3},
	SSE41:    {SSSE3},
	SSE42:    {SSE41},
	AES:      {SSE2},
	PCLMUL:   {SSE2},
	SHA:      {SSE2},
	AVX:      {SSE42, XSAVE},
	F16C:     {AVX},
	FMA:      {AVX},
	AVX2:     {AVX},
	AVX512F:  {AVX2},
	AVX512CD: {AVX512F},
	AVX512VL: {AVX512F},
	AVX512BW: {AVX512F},
	AVX512DQ: {AVX512F},
}

// requiredBy is the reverse of requires, computed once.
var requiredBy [numFeatures][]Feature

func init() {
	for f := Feature(0); f < numFeatures; f++ {
		for _, r := range requires[f] {
			requiredBy[r] = append(requiredBy[r], f)
		}
	}
}