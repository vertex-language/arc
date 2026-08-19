package isa

import (
	"sort"
	"strings"

	"github.com/vertex-language/arc/aarch64/feature"
)

var (
	all    []*Form
	byMnem = map[string][]*Form{}
)

// register adds forms to the table. Every table file calls it from an init
// function; nothing else may.
func register(forms ...*Form) {
	for _, f := range forms {
		f.finish()
		all = append(all, f)
		byMnem[f.Mnem] = append(byMnem[f.Mnem], f)
	}
}

// checkTable runs the cross-form checks that a single form cannot make about
// itself. It runs once, after every table file has registered.
//
// The check that matters is ambiguity. Two forms of one mnemonic with the same
// operand signature would make Resolve's answer depend on declaration order,
// which is exactly the arbitrary tiebreak this architecture does not need and
// this package refuses to have.
func checkTable() {
	seen := map[string]*Form{}
	for _, f := range all {
		sig := f.Signature()
		if prev, dup := seen[sig]; dup {
			panic("isa: ambiguous forms with identical signature: " +
				sig + " declared twice (" + prev.GoName() + ", " + f.GoName() + ")")
		}
		seen[sig] = f
	}

	names := map[string]*Form{}
	for _, f := range all {
		if prev, dup := names[f.name]; dup {
			panic("isa: two forms generate the helper name " + f.name +
				": " + prev.Signature() + " and " + f.Signature())
		}
		names[f.name] = f
	}

	checkAliases()
}

// All returns every form in the table, in declaration order.
func All() []*Form { return all }

// Forms returns the forms of one mnemonic.
func Forms(mnem string) []*Form { return byMnem[strings.ToLower(mnem)] }

// Mnemonics returns every mnemonic, sorted.
func Mnemonics() []string {
	out := make([]string, 0, len(byMnem))
	for m := range byMnem {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Enabled returns every form a feature set permits. It is what the helper
// generator iterates and what `arc isa` prints.
func Enabled(s feature.Set) []*Form {
	var out []*Form
	for _, f := range all {
		if f.Enabled(s) {
			out = append(out, f)
		}
	}
	return out
}

// Gates returns every feature that gates at least one form, which is the set of
// flags a diagnostic can ever name.
func Gates() []feature.Feature {
	seen := map[feature.Feature]bool{}
	var out []feature.Feature
	for _, f := range all {
		if f.Gate != feature.None && !seen[f.Gate] {
			seen[f.Gate] = true
			out = append(out, f.Gate)
		}
	}
	return out
}