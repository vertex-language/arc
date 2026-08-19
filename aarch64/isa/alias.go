package isa

import "fmt"

// The architecture's own alias relation.
//
// CMP is SUBS with XZR as its destination. MOV is ORR with WZR, or ADD with an
// immediate of zero, depending on the operands. LSL is UBFM. RET is RET X30.
// Each of these is in the ARM ARM's own table with a stated equivalence and a
// stated preferred disassembly, and each is one-to-one: the alias determines
// the word and the word determines whether the alias is preferred.
//
// That one-to-one property is what makes this an alias relation and not
// instruction selection. Emit picks an encoding of the instruction the caller
// named; when the caller names CMP it gets the SUBS word, which is the same
// instruction under another name, not a different instruction chosen for them.
//
// The assembler-invented layer — ldr x0, =value, a MOVZ/MOVK chain for a wide
// constant — is not here and is not an alias. One mnemonic becoming three
// instructions is selection, and a literal pool is layout.
type aliasOf struct {
	// of is the mnemonic this form aliases.
	of string

	// fixed lists the fields the alias pins and the values it pins them to.
	// These are what make the alias narrower than what it aliases.
	fixed []fixedField

	// preferred reports whether a word that matches this form should be
	// disassembled as the alias rather than as the underlying instruction.
	// The ARM ARM states this per alias; nil means always preferred.
	preferred func(word uint32) bool
}

type fixedField struct {
	field Field
	value uint64
}

// Alias reports whether this form is an alias, and of what.
func (f *Form) Alias() (string, bool) {
	if f.alias == nil {
		return "", false
	}
	return f.alias.of, true
}

// Preferred reports whether a word matching this alias form should be printed
// as the alias. It is what makes arc dis print cmp where objdump prints cmp.
func (f *Form) Preferred(word uint32) bool {
	if f.alias == nil {
		return false
	}
	if f.alias.preferred == nil {
		return true
	}
	return f.alias.preferred(word)
}

// AliasesOf returns every alias form of a mnemonic, which is what the decoder
// walks to apply the preferred-disassembly rule.
func AliasesOf(mnem string) []*Form {
	var out []*Form
	for _, f := range all {
		if f.alias != nil && f.alias.of == mnem {
			out = append(out, f)
		}
	}
	return out
}

// checkAliases verifies that every alias names a mnemonic that exists and that
// an alias form's fixed fields really are fixed in its own word and mask.
//
// An alias whose fixed fields were not in its mask would decode to the
// underlying instruction as well as to itself, which is the ambiguity the
// one-to-one requirement exists to prevent.
func checkAliases() {
	for _, f := range all {
		a := f.alias
		if a == nil {
			continue
		}
		if len(byMnem[a.of]) == 0 {
			panic(fmt.Sprintf("isa: %s aliases %s, which no form declares", f.Mnem, a.of))
		}
		for _, ff := range a.fixed {
			if ff.field.Mask()&f.Mask != ff.field.Mask() {
				panic(fmt.Sprintf("isa: %s pins %s but does not fix it in its mask",
					f.Mnem, ff.field))
			}
			if ff.field.Get(f.Word) != ff.value {
				panic(fmt.Sprintf("isa: %s pins %s to %d but its base word holds %d",
					f.Mnem, ff.field, ff.value, ff.field.Get(f.Word)))
			}
		}
	}
}