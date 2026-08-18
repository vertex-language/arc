// x86_64/feature/parse.go
package feature

import (
	"fmt"
	"strings"
)

var byName map[string]Feature
var levelByName = map[string]Level{
	"x86-64-v1": V1, "x86-64-v2": V2, "x86-64-v3": V3, "x86-64-v4": V4,
	// The underscore spellings GCC and LLVM also accept on -march.
	"x86_64_v1": V1, "x86_64_v2": V2, "x86_64_v3": V3, "x86_64_v4": V4,
	// v1 is the unadorned baseline under its original name.
	"x86-64": V1, "x86_64": V1,
}

// aliases are spellings that resolve to a canonical feature and are then
// discarded. They exist because the world writes sse4_1 and sse4a and abm;
// the canonical spelling is what comes back out.
var aliases = map[string]Feature{
	"sse4_1": SSE41, "sse4.1": SSE41, "sse41": SSE41,
	"sse4_2": SSE42, "sse4.2": SSE42, "sse42": SSE42,
	"abm":        LZCNT, // AMD's name for the LZCNT/POPCNT pair
	"lahf_lm":    LAHFSAHF,
	"lahf-sahf":  LAHFSAHF,
	"cx16":       CMPXCHG16B,
	"pclmul":     PCLMULQDQ,
	"amx_tile":   AMXTILE,
	"amx_int8":   AMXINT8,
	"amx_bf16":   AMXBF16,
}

func init() {
	byName = make(map[string]Feature, numFeatures*2)
	for f := MMX; f < numFeatures; f++ {
		byName[names[f]] = f
	}
	for k, v := range aliases {
		byName[k] = v
	}
}

// ParseFeature resolves one feature name. Aliases resolve here, at the
// boundary, and nothing downstream sees them.
func ParseFeature(s string) (Feature, error) {
	f, ok := byName[strings.ToLower(strings.TrimSpace(s))]
	if !ok {
		return Base, fmt.Errorf("unknown feature %q", s)
	}
	return f, nil
}

// ParseFeatures parses a feature-set spelling:
//
//	x86-64-v2+avx512f
//	x86-64-v3
//	sse2+aes+pclmulqdq
//	x86-64-v4-avx512vl
//
// A leading level sets the base; without one the set starts empty, not at
// Baseline, so "sse2" means sse2 and not "v1 plus sse2" — those differ, and
// silently widening the set would make a gating diagnostic unfalsifiable.
//
// A +name enables a feature and everything it requires. A -name disables it
// and everything that requires it.
func ParseFeatures(s string) (Set, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Baseline(), nil
	}

	toks := tokenize(s)
	if len(toks) == 0 {
		return Set{}, fmt.Errorf("empty feature set %q", s)
	}

	var set Set
	start := 0

	// A level, if present, must lead — it sets the base rather than adding
	// to one, so "avx2+x86-64-v2" is a contradiction, not a union.
	head := strings.ToLower(toks[0].name)
	if lvl, ok := levelByName[head]; ok {
		if toks[0].sub {
			return Set{}, fmt.Errorf("cannot subtract a level: %q", s)
		}
		set = lvl.Set()
		start = 1
	}

	for _, t := range toks[start:] {
		if _, isLevel := levelByName[strings.ToLower(t.name)]; isLevel {
			return Set{}, fmt.Errorf(
				"level %s must come first in %q", t.name, s)
		}
		f, err := ParseFeature(t.name)
		if err != nil {
			return Set{}, fmt.Errorf("in %q: %w", s, err)
		}
		if t.sub {
			set = set.Minus(f)
		} else {
			set = set.Plus(f)
		}
	}
	return set, nil
}

type token struct {
	name string
	sub  bool
}

// tokenize splits on + and -, treating a leading sign as belonging to the
// term that follows. Feature names contain '-' (amx-tile, lahf-sahf) and '.'
// (sse4.1), so the split is on separators that follow a complete name — which
// means longest-match against the name table, not a naive strings.Split.
func tokenize(s string) []token {
	var out []token
	i := 0
	sub := false
	for i < len(s) {
		// Consume the longest name that starts at i.
		best := -1
		for j := len(s); j > i; j-- {
			if _, ok := byName[strings.ToLower(s[i:j])]; ok {
				best = j
				break
			}
			if _, ok := levelByName[strings.ToLower(s[i:j])]; ok {
				best = j
				break
			}
		}
		if best < 0 {
			// Unknown term: hand the rest to ParseFeature so the error
			// names it rather than reporting a parse failure.
			end := i
			for end < len(s) && s[end] != '+' {
				end++
			}
			out = append(out, token{s[i:end], sub})
			i = end
			if i < len(s) {
				i++
			}
			sub = false
			continue
		}
		out = append(out, token{s[i:best], sub})
		i = best
		if i < len(s) {
			switch s[i] {
			case '+':
				sub = false
			case '-':
				sub = true
			default:
				out = append(out, token{s[i:], false})
				return out
			}
			i++
		}
	}
	return out
}