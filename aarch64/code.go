package aarch64

import (
	"github.com/vertex-language/arc/aarch64/decode"
	"github.com/vertex-language/arc/aarch64/encode"
	"github.com/vertex-language/arc/aarch64/isa"
)

// Encoding and decoding without an object.
//
// Everything here forwards to isa/, encode/ or decode/ and adds nothing. The
// forwarding exists so a caller with one instruction and no section around it
// does not have to import three subpackages to ask one question, and so the
// answers a caller gets here are the same functions arc dis and arc explain
// call — a second implementation would eventually disagree with the first.

// Form is one declared encoding of one mnemonic.
type Form = isa.Form

// Arg is one operand as Resolve sees it.
type Arg = isa.Arg

// Class is what one operand slot accepts.
type Class = isa.Class

// Inst is one decoded instruction.
type Inst = decode.Inst

// Explanation is arc explain's field-by-field breakdown.
type Explanation = decode.Explanation

// Fixup is a field an encoding left blank because its value is an address that
// is not yet a number.
type Fixup = encode.Fixup

// Opts is what an encode call needs that is not an operand.
type Opts = encode.Opts

// Encode assembles one instruction with no section and no symbol table around
// it.
//
// The fixups come back rather than being resolved, because there is nothing
// here to resolve them against. A caller wanting them folded wants an
// Assembler.
func Encode(set FeatureSet, mnem string, ops ...any) (uint32, []Fixup, error) {
	return encode.Encode(set, mnem, ops...)
}

// EncodeWith is Encode with call context.
func EncodeWith(opts Opts, set FeatureSet, mnem string, ops ...any) (uint32, []Fixup, error) {
	return encode.EncodeWith(opts, set, mnem, ops...)
}

// EncodeForm encodes against a form the caller already resolved.
//
// It does not consult a feature set: the gate was checked when the form was
// resolved, and re-checking here would let the two answers disagree.
func EncodeForm(f *Form, ops []any, opts Opts) (uint32, []Fixup, error) {
	return encode.EncodeForm(f, ops, opts)
}

// Resolve finds the one form of a mnemonic that accepts these operands.
//
// There is no shortest-form search and no preference order. Every A64
// instruction is one word, so two forms accepting the same operand classes
// would be an ambiguity with no meaningful tiebreak — and the table refuses to
// build if two such forms exist, so this never has to choose.
func Resolve(mnem string, args []Arg, set FeatureSet) (*Form, error) {
	return isa.Resolve(mnem, args, set)
}

// Forms returns every declared encoding of a mnemonic.
func Forms(mnem string) []*Form { return isa.Forms(mnem) }

// Mnemonics returns every mnemonic the table declares, sorted.
func Mnemonics() []string { return isa.Mnemonics() }

// Enabled returns every form a feature set permits, which is what the helper
// generator iterates and what arc isa prints.
func Enabled(set FeatureSet) []*Form { return isa.Enabled(set) }

// Gates returns every feature that gates at least one form: the set of flags a
// diagnostic can ever name.
func Gates() []Feature { return isa.Gates() }

// Decode reads one instruction from the front of b.
//
// It reads what the architecture decodes, not what this assembler emits. There
// is no canonicalization and no re-encoding: a word that names a form is that
// form's, and arc dis prints it.
func Decode(b []byte) (Inst, error) { return decode.Decode(b) }

// DecodeAll reads every instruction in b. A length that is not a multiple of
// four is an error before anything is decoded, rather than a partial result
// with a truncated tail.
func DecodeAll(b []byte) ([]Inst, error) { return decode.DecodeAll(b) }

// DecodeWord decodes an instruction already read as a word.
func DecodeWord(w uint32) (Inst, error) { return decode.DecodeWord(w) }

// Explain is the field-by-field breakdown arc explain prints: one line naming a
// field, its contents, and what it does to the instruction — not "here are some
// bytes" but "here is why these are the bytes."
func Explain(b []byte) (Explanation, error) { return decode.Explain(b) }

// ExplainInst breaks down an instruction already decoded.
func ExplainInst(in Inst) Explanation { return decode.ExplainInst(in) }

// Nop is the one no-op word, d503201f.
//
// There is one, and Align needs no table. Every instruction is four bytes, so
// there is no question of where a decoder resumes inside padding — the
// multi-byte no-op tables x86_64 carries exist to answer exactly that question.
func Nop() uint32 { return encode.Nop() }