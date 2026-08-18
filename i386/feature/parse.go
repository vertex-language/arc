package feature

import (
	"errors"
	"fmt"
	"strings"
)

// aliases are alternate spellings accepted for extensions. They resolve here
// and nowhere below: past Parse only canonical values exist.
var aliases = map[string]Feature{
	"sse4":       SSE42, // GNU as spells both SSE4 sub-extensions this way
	"sse4a":      SSE42,
	"bmi":        BMI1,
	"pclmulqdq":  PCLMUL,
	"aesni":      AES,
	"abm":        LZCNT,
	"f16":        F16C,
	"sha_ni":     SHA,
	"avx512":     AVX512F,
	"avx512_f":   AVX512F,
}

// levelAliases are alternate spellings for the base levels.
var levelAliases = map[string]Level{
	"80386":      I386,
	"80486":      I486,
	"pentium":    I586,
	"i786":       I686,
	"pentiumpro": I686,
	"p6":         I686,
}

// inBaseline names spellings that are real x86 features but are part of a base
// level here, so that asking for them as extensions gets an answer rather than
// "unknown".
var inBaseline = map[string]Level{
	"cmov":      I686,
	"fcmov":     I686,
	"fcomi":     I686,
	"cx8":       I586,
	"cmpxchg8b": I586,
	"cpuid":     I586,
	"rdtsc":     I586,
	"bswap":     I486,
	"cmpxchg":   I486,
	"xadd":      I486,
	"fpu":       I386,
	"x87":       I386,
}

// notThirtyTwoBit names extensions that exist but require 64-bit mode, so that
// the diagnostic says why rather than claiming the name is unknown. Each entry
// is the reason.
var notThirtyTwoBit = map[string]string{
	"cx16":        "CMPXCHG16B requires 64-bit mode",
	"cmpxchg16b":  "CMPXCHG16B requires 64-bit mode",
	"sce":         "SYSCALL/SYSRET are 64-bit mode instructions",
	"syscall":     "SYSCALL/SYSRET are 64-bit mode instructions",
	"lahf_lm":     "LAHF/SAHF in 64-bit mode is the extension; in 32-bit mode they are baseline",
	"amx-tile":    "AMX requires 64-bit mode",
	"amx-int8":    "AMX requires 64-bit mode",
	"amx-bf16":    "AMX requires 64-bit mode",
	"uintr":       "user interrupts require 64-bit mode",
}

// withdrawn names extensions that shipped and were then removed from the
// architecture. arc encodes ratified, current extensions only, so these are
// errors with a reason rather than silent acceptance.
var withdrawn = map[string]string{
	"mpx":    "Intel MPX was removed from the architecture",
	"bnd":    "Intel MPX was removed from the architecture",
	"3dnow":  "AMD 3DNow! was removed from the architecture",
	"3dnowa": "AMD 3DNow! was removed from the architecture",
	"pcommit": "PCOMMIT was removed from the architecture",
}

// ErrUsage is returned for a feature string that names nothing this target
// has. cmd/arc renders it as a usage error; the string is the whole message.
var ErrUsage = errors.New("feature")

// Parse resolves a --features value against a starting set.
//
// Two forms, and they do not mix:
//
//	avx2,bmi2        set exactly — start from the level's bare set
//	+avx2,-sse4.2    adjust — start from base
//
// The first form may name a level, which becomes the base level of the result;
// naming two levels is an error. The second form may not name a level, because
// +i486 and -i486 have no meaning on a ladder.
func Parse(base Set, s string) (Set, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return base, nil
	}

	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '+' && false })
	// Split on ',' only; '+' is part of the adjust syntax handled per field.
	fields = strings.Split(s, ",")

	var adjust, exact bool
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if f[0] == '+' || f[0] == '-' {
			adjust = true
		} else {
			exact = true
		}
	}
	if adjust && exact {
		return base, fmt.Errorf("%w: cannot mix an exact set with +/- adjustments\n  note: use either \"avx2,bmi2\" or \"+avx2,-sse4.2\"", ErrUsage)
	}

	out := base
	var levelSet bool
	if exact {
		out = New(base.level)
	}

	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		sign := byte('+')
		name := f
		if f[0] == '+' || f[0] == '-' {
			sign, name = f[0], f[1:]
		}
		name = strings.ToLower(strings.TrimSpace(name))

		if l, ok := parseLevel(name); ok {
			if adjust {
				return base, fmt.Errorf("%w: %q is a base level, not an extension\n  note: levels are cumulative; write --features %s", ErrUsage, name, name)
			}
			if levelSet && l != out.level {
				return base, fmt.Errorf("%w: two base levels named: %s and %s", ErrUsage, out.level, l)
			}
			out.level, levelSet = l, true
			continue
		}

		ft, err := parseFeature(name)
		if err != nil {
			return base, err
		}
		if sign == '-' {
			out = out.Remove(ft)
		} else {
			out = out.Add(ft)
		}
	}
	return out, nil
}

func parseLevel(name string) (Level, bool) {
	for i, n := range levelNames {
		if name == n {
			return Level(i), true
		}
	}
	l, ok := levelAliases[name]
	return l, ok
}

func parseFeature(name string) (Feature, error) {
	for f := Feature(0); f < numFeatures; f++ {
		if name == featureNames[f] {
			return f, nil
		}
	}
	if f, ok := aliases[name]; ok {
		return f, nil
	}
	if l, ok := inBaseline[name]; ok {
		return 0, fmt.Errorf("%w: %q is part of the %s base level, not an extension\n  note: it is in the i686 baseline; --features %s to require less", ErrUsage, name, l, l)
	}
	if why, ok := notThirtyTwoBit[name]; ok {
		return 0, fmt.Errorf("%w: %q does not apply to i386\n  note: %s", ErrUsage, name, why)
	}
	if why, ok := withdrawn[name]; ok {
		return 0, fmt.Errorf("%w: %q is not accepted\n  note: %s\n  note: arc encodes current, ratified extensions only", ErrUsage, name, why)
	}
	if strings.HasPrefix(name, "x86-64") {
		return 0, fmt.Errorf("%w: %q is an x86-64 microarchitecture level and does not apply to i386\n  note: the levels are defined over the 64-bit baseline; v1 includes SCE and v2 requires CMPXCHG16B\n  note: i386 base levels are i386, i486, i586, i686", ErrUsage, name)
	}
	return 0, fmt.Errorf("%w: unknown extension %q for i386\n  note: --features help lists what this target has", ErrUsage, name)
}

// Help is the body of --features help: every level and every extension this
// target has, in canonical order.
func Help() string {
	var b strings.Builder
	b.WriteString("i386 base levels (cumulative, default " + Baseline.String() + "):\n")
	for i := range levelNames {
		l := Level(i)
		b.WriteString("  " + l.String() + "\t" + strings.Join(l.Adds(), " ") + "\n")
	}
	b.WriteString("\ni386 extensions:\n")
	for f := Feature(0); f < numFeatures; f++ {
		b.WriteString("  " + f.String())
		if r := requires[f]; len(r) > 0 {
			names := make([]string, len(r))
			for i, x := range r {
				names[i] = x.String()
			}
			b.WriteString("\trequires " + strings.Join(names, ", "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}