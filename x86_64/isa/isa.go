// x86_64/isa/isa.go
//
// Package isa is the form table: every declared encoding of every mnemonic
// this target has, its operand slots, its opcode bytes, and the feature that
// gates it.
//
// A Form is one row of the SDM's opcode column together with one row of its
// Instruction Operand Encoding table. Nothing here decides anything: encode/
// turns a Form and operand values into bytes, decode/ builds its opcode maps
// from All(), text/ prints from Slots, and the generator that writes
// helpers_*_gen.go reads Form.GoName and Form.Need and nothing else.
//
// This package imports reg/ and operand/ because a Form's slots are declared
// in terms of what a register and a memory reference are. It does not import
// the root package, and the root's Operand interface is not visible here —
// Resolve takes an Arg, which the root builds from its own operand values.
package isa

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vertex-language/arc/x86_64/feature"
)

// forms is every declared form, in table order. Order is load-bearing:
// Resolve breaks size ties by it, so a form declared earlier wins.
var forms []*Form

// byMnemonic indexes forms by mnemonic, preserving table order within each.
var byMnemonic map[string][]*Form

// The tables are registered from one place rather than from an init per
// file, because Go orders per-file init by filename and the table order is
// part of the contract. table_avx.go must not sort ahead of table_base.go.
func init() {
	register(baseForms())
	register(sseForms())
	register(avxForms())
	register(avx512Forms())

	byMnemonic = make(map[string][]*Form, 512)
	for _, f := range forms {
		byMnemonic[f.Op] = append(byMnemonic[f.Op], f)
	}
}

func register(fs []*Form) {
	for _, f := range fs {
		f.finish() // derives what can be derived, panics on what cannot be encoded
		f.index = len(forms)
		forms = append(forms, f)
	}
}

// All is every form in table order. decode/ builds its opcode maps from this
// and reads nothing else.
func All() []*Form { return forms }

// Forms is every declared encoding of one mnemonic, in table order,
// regardless of what is enabled. Nil for a mnemonic this target has no form
// for; that is the caller's "unknown mnemonic".
func Forms(mnemonic string) []*Form { return byMnemonic[mnemonic] }

// Mnemonics is every mnemonic with at least one form, sorted. This is what
// `arc isa` lists.
func Mnemonics() []string {
	out := make([]string, 0, len(byMnemonic))
	for m := range byMnemonic {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Enabled is every form encodable under s, in table order.
func Enabled(s feature.Set) []*Form {
	var out []*Form
	for _, f := range forms {
		if f.Enabled(s) {
			out = append(out, f)
		}
	}
	return out
}

// Count is len(All()), for the generator's benefit and for a test that
// notices when a table file silently stops being registered.
func Count() int { return len(forms) }

func init() {
	// Every mnemonic is lowercase here. gas and nasm disagree about case and
	// both fold before they reach this package; a mixed-case entry would be
	// unreachable rather than wrong, which is worse.
	for _, f := range forms {
		if f.Op != strings.ToLower(f.Op) {
			panic(fmt.Sprintf("isa: mnemonic %q is not lowercase", f.Op))
		}
	}
}