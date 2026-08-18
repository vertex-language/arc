package isa

import (
	"sort"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/operand"
)

var byMnemonic map[string][]*Form

func init() {
	byMnemonic = make(map[string][]*Form, len(forms))
	for _, f := range forms {
		byMnemonic[f.Mnemonic] = append(byMnemonic[f.Mnemonic], f)
	}
}

// Forms returns every declared form of a mnemonic, in table order, whatever
// the feature set. This is arc isa --all.
func Forms(mnemonic string) []*Form {
	return byMnemonic[mnemonic]
}

// Enabled returns the forms of a mnemonic that s permits, in table order.
// This is arc isa without --all.
func Enabled(mnemonic string, s feature.Set) []*Form {
	var out []*Form
	for _, f := range byMnemonic[mnemonic] {
		if f.Enabled(s) {
			out = append(out, f)
		}
	}
	return out
}

// Mnemonics returns every declared mnemonic, sorted. This is what shell
// completion and arc isa with no argument print.
func Mnemonics() []string {
	out := make([]string, 0, len(byMnemonic))
	for m := range byMnemonic {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// All returns every form in table order.
func All() []*Form { return forms }

// Resolve finds the form for a mnemonic and operands under a feature set.
//
// Candidates are the forms that match, in table order. Selection is the
// caller's: Emit takes the shortest and breaks ties by table order, and the
// typed helpers do not call this at all because they name their form already.
// Resolve returns the candidates rather than choosing, because the length of
// an encoding depends on the operands' addressing mode and that is encode/'s
// to compute.
func Resolve(mnemonic string, s feature.Set, ops []operand.Operand) (match []*Form, gated []*Form) {
	for _, f := range byMnemonic[mnemonic] {
		if !f.Matches(ops) {
			continue
		}
		if f.Enabled(s) {
			match = append(match, f)
		} else {
			gated = append(gated, f)
		}
	}
	return match, gated
}