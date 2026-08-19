package feature

import (
	"fmt"
	"strings"
)

// ParseFeature resolves one extension name, as written after a '+'.
//
// It accepts the aliases the world writes and discards them: the canonical
// spelling is what comes back out and what String prints. "crypto" is rejected
// here because it names two features rather than one; ParseFeatures handles it.
func ParseFeature(name string) (Feature, error) {
	s := strings.ToLower(strings.TrimSpace(name))
	if f, ok := featureByName[s]; ok {
		return f, nil
	}
	if _, ok := groupByName[s]; ok {
		return None, fmt.Errorf("feature: %q names a group of extensions, not one", s)
	}
	return None, fmt.Errorf("feature: unknown extension %q", s)
}

// ParseLevel resolves an architecture version name.
func ParseLevel(name string) (Level, error) {
	s := strings.ToLower(strings.TrimSpace(name))
	if l, ok := levelByName[s]; ok {
		return l, nil
	}
	return LevelNone, fmt.Errorf("feature: unknown architecture %q", s)
}

// ParseFeatures reads the +ext / +noext grammar: an optional architecture
// version followed by additions and removals.
//
//	armv8.2-a+sve+nofp16
//	armv9-a+sme2
//	+lse+crc
//
// Modifiers apply left to right, which is what makes "removals follow
// additions" work and what makes the rightmost of two conflicting modifiers the
// one that stands. Each is closed as it is applied, so +sve2 pulls in sve and a
// later +nosve drops both.
//
// Without a leading version the set starts empty rather than at Baseline, so
// "sve" means SVE and its requirements alone.
func ParseFeatures(spec string) (Set, error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return Set{}, nil
	}

	parts := strings.Split(s, "+")
	var set Set

	// A leading empty part means the spec started with '+', so there is no
	// version to read.
	if parts[0] != "" {
		l, err := ParseLevel(parts[0])
		if err != nil {
			return Set{}, err
		}
		set = l.Set()
	}

	for _, p := range parts[1:] {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			return Set{}, fmt.Errorf("feature: empty extension in %q", spec)
		}

		remove := false
		if strings.HasPrefix(p, "no") {
			// "no" is a prefix, not a rule: nofp is a removal and none of the
			// real names begin with it, but checking the full name first costs
			// nothing and keeps a future extension spelled "nozzle" from being
			// read as a removal.
			if _, isName := featureByName[p]; !isName {
				if _, isGroup := groupByName[p]; !isGroup {
					remove, p = true, strings.TrimPrefix(p, "no")
				}
			}
		}

		if g, ok := groupByName[p]; ok {
			if remove {
				set = set.Minus(g...)
			} else {
				set = set.Plus(g...)
			}
			continue
		}

		f, err := ParseFeature(p)
		if err != nil {
			return Set{}, fmt.Errorf("feature: in %q: %w", spec, err)
		}
		if remove {
			set = set.Minus(f)
		} else {
			set = set.Plus(f)
		}
	}
	return set, nil
}

// MustParseFeatures is ParseFeatures for a constant spec in package-level
// initialization. It panics rather than returning an error.
func MustParseFeatures(spec string) Set {
	s, err := ParseFeatures(spec)
	if err != nil {
		panic(err)
	}
	return s
}

var featureByName = func() map[string]Feature {
	m := make(map[string]Feature, featureCount*2)
	for f := None + 1; f < featureCount; f++ {
		m[info[f].name] = f
	}
	// The aliases the world actually writes. Each maps to a canonical feature
	// and is then discarded; nothing round-trips back to an alias spelling.
	for alias, f := range map[string]Feature{
		"rdm":       RDMA,    // LLVM's spelling of gas's rdma
		"jsconv":    JSCVT,   // the ACLE spelling
		"mte":       MemTag,  // the architecture's own abbreviation
		"fhm":       FP16FML, // the HWCAP name
		"dot":       DotProd,
		"spe":       Profile,
		"sha512":    SHA3, // one extension, two instruction groups
		"sm3":       SM4,  // likewise
		"pmull":     AES,  // pmull ships with aes and has no separate gate
		"rcpc-imm":  RCPC2,
		"b16b16":    SVEB16B16,
		"sve-bf16":  BF16,
		"ssbs2":     SSBS,
	} {
		m[alias] = f
	}
	return m
}()

// Groups are names for more than one feature. They exist because the world
// writes them, not because the architecture has a gate by that name — nothing
// is encoded as "crypto", and no set prints back as one.
var groupByName = map[string][]Feature{
	"crypto": {AES, SHA2},
}

var levelByName = func() map[string]Level {
	m := make(map[string]Level, levelCount*2)
	for l := LevelNone + 1; l < levelCount; l++ {
		m[levelSpec[l].name] = l
	}
	// GCC and gas both accept the dotless spelling of the .0 versions.
	m["armv8"] = Armv8A
	m["armv9"] = Armv9A
	m["armv8.0-a"] = Armv8A
	m["armv9.0-a"] = Armv9A
	return m
}()

// goLevelName is the Go identifier for a level, for GoExpr.
func goLevelName(l Level) string {
	switch l {
	case Armv8A:
		return "Armv8A"
	case Armv9A:
		return "Armv9A"
	}
	// armv8.2-a -> Armv8_2A
	n := levelSpec[l].name
	n = strings.TrimPrefix(n, "armv")
	n = strings.TrimSuffix(n, "-a")
	return "Armv" + strings.Replace(n, ".", "_", 1) + "A"
}

// goFeatureName is the Go identifier for a feature, for GoExpr. The table is
// explicit because the identifier is not derivable from the spelling: sve2-aes
// is SVE2AES and fp16fml is FP16FML, and no rule produces both.
var goName = map[Feature]string{
	FP: "FP", SIMD: "SIMD", FP16: "FP16", FP16FML: "FP16FML", BF16: "BF16",
	I8MM: "I8MM", DotProd: "DotProd", FCMA: "FCMA", JSCVT: "JSCVT",
	FRIntTS: "FRIntTS", FAMINMAX: "FAMINMAX", LUT: "LUT",
	AES: "AES", SHA2: "SHA2", SHA3: "SHA3", SM4: "SM4",
	LSE: "LSE", LSE128: "LSE128", D128: "D128", RCPC: "RCPC", RCPC2: "RCPC2",
	RCPC3: "RCPC3", LS64: "LS64", MOPS: "MOPS", XS: "XS", THE: "THE",
	CRC: "CRC", RDMA: "RDMA", FlagM: "FlagM", FlagM2: "FlagM2", CSSC: "CSSC",
	HBC: "HBC", WFxT: "WFxT",
	PAuth: "PAuth", BTI: "BTI", MemTag: "MemTag", SB: "SB", SSBS: "SSBS",
	PredRes: "PredRes", GCS: "GCS", CPA: "CPA", RNG: "RNG", TME: "TME",
	SVE: "SVE", SVE2: "SVE2", SVE2AES: "SVE2AES", SVE2SM4: "SVE2SM4",
	SVE2SHA3: "SVE2SHA3", SVE2BitPerm: "SVE2BitPerm", SVE2p1: "SVE2p1",
	SVEB16B16: "SVEB16B16", F32MM: "F32MM", F64MM: "F64MM",
	SME: "SME", SMEI16I64: "SMEI16I64", SMEF64F64: "SMEF64F64", SME2: "SME2",
	SME2p1: "SME2p1", SMEB16B16: "SMEB16B16", SMEF16F16: "SMEF16F16",
	FP8: "FP8", FP8FMA: "FP8FMA", FP8DOT4: "FP8DOT4", FP8DOT2: "FP8DOT2",
	Profile: "Profile",
}

func goFeatureName(f Feature) string {
	if n, ok := goName[f]; ok {
		return n
	}
	return "None"
}