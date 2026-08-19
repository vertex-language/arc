package decode

import "github.com/vertex-language/arc/aarch64/isa"

// The preferred-disassembly rule.
//
// The architecture defines aliases and defines, per alias, when a word that
// matches one should be printed as the alias rather than as what it aliases.
// That rule is the ARM ARM's, it is stated on the form in isa/, and applying it
// here is what makes arc dis print cmp where objdump prints cmp.
//
// It is a printing decision and nothing more. Inst.Form is always the
// underlying form, so re-encoding is unaffected: a word decoded as cmp encodes
// back through subs and returns the same word.

// preferredAlias returns the alias form this word should be printed as, or nil.
func preferredAlias(f *isa.Form, word uint32) *isa.Form {
	var best *isa.Form
	for _, a := range isa.AliasesOf(f.Mnem) {
		if word&a.Mask != a.Word {
			continue
		}
		if !a.Preferred(word) {
			continue
		}
		if best == nil {
			best = a
			continue
		}
		// Two aliases both claiming one word is a table bug rather than a
		// choice: the alias relation is one-to-one in both directions, and an
		// arbitrary tiebreak here would be the arbitrariness this package's
		// whole design refuses. Keep the first and let Conflicts report it.
		if a == best {
			continue
		}
	}
	return best
}

// Conflicts reports every word shape where two aliases of one mnemonic would
// both be preferred.
//
// It is a check rather than a panic, and it is a check rather than nothing,
// because the case exists in the table today: lsl and lsr both alias ubfm with
// the same base word and mask and no preferred predicate on either, so every
// UBFM word they match has two equally preferred spellings. The two differ only
// through the immr/imms pair encode/ computes, which is the argument for making
// them a lowering in encode/ rather than forms — the decision the isa/ TODO
// flagged and the one this function will keep reporting until it is made.
func Conflicts() [][2]*isa.Form {
	var out [][2]*isa.Form
	seen := map[string]bool{}

	for _, f := range isa.All() {
		if f.Attrs&isa.AttrAlias != 0 || f.Mask == 0 {
			continue
		}
		as := isa.AliasesOf(f.Mnem)
		for i, a := range as {
			for _, b := range as[i+1:] {
				// Overlapping encodings: some word matches both.
				common := a.Mask & b.Mask
				if a.Word&common != b.Word&common {
					continue
				}
				k := a.GoName() + "/" + b.GoName()
				if seen[k] {
					continue
				}
				seen[k] = true
				out = append(out, [2]*isa.Form{a, b})
			}
		}
	}
	return out
}