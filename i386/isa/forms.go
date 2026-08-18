package isa

import (
	"sort"
	"sync"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/operand"
)

var (
	byMnemonic     map[string][]*Form
	byMnemonicOnce sync.Once
)

// index builds byMnemonic from forms, once, on first use.
//
// This cannot run in an init() of its own: Go orders init() functions across
// a package's files alphabetically, and table_base.go's init() — which
// populates forms via buildALU, buildMov, and the rest — sorts after this
// file's. An init() here would see an empty forms slice and every lookup
// below would silently return nothing for every mnemonic, forever. Building
// the index lazily, on first call from outside the package, sidesteps the
// ordering question entirely: by the time any caller can reach Forms,
// Enabled, Mnemonics or Resolve, every table file's init() has already run
// and appended everything it has to forms.
func index() map[string][]*Form {
	byMnemonicOnce.Do(func() {
		byMnemonic = make(map[string][]*Form, len(forms))
		for _, f := range forms {
			byMnemonic[f.Mnemonic] = append(byMnemonic[f.Mnemonic], f)
		}
	})
	return byMnemonic
}

// Forms returns every declared form of a mnemonic, in table order, whatever
// the feature set. This is arc isa --all.
func Forms(mnemonic string) []*Form {
	return index()[mnemonic]
}

// Enabled returns the forms of a mnemonic that s permits, in table order.
// This is arc isa without --all.
func Enabled(mnemonic string, s feature.Set) []*Form {
	var out []*Form
	for _, f := range index()[mnemonic] {
		if f.Enabled(s) {
			out = append(out, f)
		}
	}
	return out
}

// Mnemonics returns every declared mnemonic, sorted. This is what shell
// completion and arc isa with no argument print.
func Mnemonics() []string {
	idx := index()
	out := make([]string, 0, len(idx))
	for m := range idx {
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
	for _, f := range index()[mnemonic] {
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