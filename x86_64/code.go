// x86_64/code.go
//
// The encoding and decoding surface, forwarded to isa/, encode/ and
// decode/. Nothing here decides anything: it exists so a caller who wants
// one instruction's bytes, or one instruction's meaning, does not have to
// know which of the three subpackages answers.
package x86_64

import (
	"errors"

	"github.com/vertex-language/arc/x86_64/decode"
	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/isa"
)

// Form is one declared encoding of one mnemonic: the SDM's opcode column
// together with its Instruction Operand Encoding table, as data.
type Form = isa.Form

// Slot is one operand of one form: what it accepts, whether the instruction
// reads or writes it, and which encoding field carries it.
type Slot = isa.Slot

// Fixup is a field an encoding left blank because its value is an address
// that is not yet a number. It is not a relocation — see reloc.go for what
// the platform writers turn it into.
type Fixup = encode.Fixup

// Opts are the EVEX modifiers a caller sets alongside the operands rather
// than in place of one: zeroing, broadcast, embedded rounding, SAE.
type Opts = encode.Opts

// RoundMode is embedded rounding control.
type RoundMode = encode.RoundMode

const (
	RoundNone    = encode.RoundNone
	RoundNearest = encode.RoundNearest
	RoundDown    = encode.RoundDown
	RoundUp      = encode.RoundUp
	RoundZero    = encode.RoundZero
)

// Inst is one decoded instruction, with operand values of the same concrete
// types Encode accepts. Handing Ops and Form back to EncodeForm returns the
// bytes it was decoded from.
type Inst = decode.Inst

// Explanation is a decoded instruction broken into its encoding fields, one
// line per field. This is what `arc explain` prints.
type Explanation = decode.Explanation

// ExplainField is one run of bytes of an encoding, with what it is and what
// it says.
//
// The name is not Field, because isa/ has a Field too — the encoding slot an
// operand lands in — and the two are different questions about the same
// bytes. Neither is aliased under a name that would let them be confused.
type ExplainField = decode.Field

// ---- the table ---------------------------------------------------------

// Forms is every declared encoding of one mnemonic, in table order,
// regardless of what the feature set enables. Nil for a mnemonic this target
// has no form for, which is a caller's "unknown mnemonic".
func Forms(mnemonic string) []*Form { return isa.Forms(mnemonic) }

// AllForms is every form this target has, in table order.
func AllForms() []*Form { return isa.All() }

// Mnemonics is every mnemonic with at least one form, sorted. This is what
// `arc isa` lists.
func Mnemonics() []string { return isa.Mnemonics() }

// EnabledForms is every form encodable under a feature set, in table order.
func EnabledForms(f FeatureSet) []*Form { return isa.Enabled(f) }

// ---- encoding ----------------------------------------------------------

// Resolve picks the encoding for a mnemonic and a set of operands: the
// shortest legal one, ties broken toward the earlier row of the table.
//
// It never picks a different instruction. A mnemonic with no form for these
// operands is an error and not a relaxation into one that fits.
func Resolve(f FeatureSet, mnemonic string, ops ...any) (*Form, error) {
	args, err := encode.Args(ops...)
	if err != nil {
		return nil, &Error{Err: err}
	}
	form, err := isa.Resolve(f, mnemonic, args...)
	if err != nil {
		return nil, &Error{Err: err, Note: gateNote(f, err)}
	}
	return form, nil
}

// Encode assembles one instruction to bytes and fixups, with no section and
// no symbol table around it.
//
// The fixups come back rather than being resolved, because there is nothing
// here for them to resolve against: a Fixup's offset is from the first byte
// of this instruction, and what it points at is a name. A caller wanting
// them resolved wants an Assembler.
func Encode(f FeatureSet, mnemonic string, ops ...any) ([]byte, []Fixup, error) {
	form, err := Resolve(f, mnemonic, ops...)
	if err != nil {
		return nil, nil, err
	}
	return EncodeForm(form, ops...)
}

// EncodeForm encodes a form that has already been chosen, skipping Resolve.
//
// This is what the typed helper layer does, and it is the only way to reach
// a form Resolve would never pick: MOV r64, imm64 is ten bytes and C7 /0 id
// is seven, so `mov rax, 60` gets the short one every time and the long one
// has to be named.
func EncodeForm(f *Form, ops ...any) ([]byte, []Fixup, error) {
	b, fx, err := encode.Encode(f, ops...)
	if err != nil {
		return nil, nil, &Error{Err: err}
	}
	return b, fx, nil
}

// EncodeFormWith is EncodeForm with the EVEX modifiers set.
func EncodeFormWith(f *Form, o Opts, ops ...any) ([]byte, []Fixup, error) {
	b, fx, err := encode.EncodeWith(f, o, ops...)
	if err != nil {
		return nil, nil, &Error{Err: err}
	}
	return b, fx, nil
}

// Nops is n bytes of code padding: the canonical multi-byte no-ops, as many
// nine-byte ones as fit and one shorter one for the remainder.
//
// These are the encodings Intel and AMD both document and the ones GNU as
// emits, which matters because a decoder walking a padded section has to
// find the same instruction boundaries either tool produces.
func Nops(n int) []byte { return encode.Nops(n) }

// ---- decoding ----------------------------------------------------------

// Decode reads one instruction from the front of b.
//
// It decodes what the architecture decodes and not what this assembler
// emits: a redundant prefix, a disp32 where a disp8 would have fit, an empty
// REX where none was needed. Re-encoding those produces the shorter form,
// which is a difference `arc fmt` is allowed to make and `arc dis` is not.
func Decode(b []byte) (*Inst, error) {
	in, err := decode.Decode(b)
	if err != nil {
		return nil, &Error{Err: err}
	}
	return in, nil
}

// DecodeAll decodes b until it is exhausted or an instruction fails,
// returning what it got and the error. A caller disassembling a section
// wants both: the bytes before a data island are still instructions.
func DecodeAll(b []byte) ([]*Inst, error) {
	insts, err := decode.DecodeAll(b)
	if err != nil {
		return insts, &Error{Err: err}
	}
	return insts, nil
}

// Explain decodes one instruction and breaks its encoding into fields.
//
// The output is the thing an assembler is for: not "here are some bytes" but
// "here is why these are the bytes." Every line names a field, its contents,
// and what it does to the instruction.
func Explain(b []byte) (*Explanation, error) {
	ex, err := decode.Explain(b)
	if err != nil {
		return nil, &Error{Err: err}
	}
	return ex, nil
}

// gateNote is the note line of a gating diagnostic: the Go expression that
// would have enabled the form. It is empty for anything else.
func gateNote(active FeatureSet, err error) string {
	var g *isa.GateError
	if !errors.As(err, &g) {
		return ""
	}
	return "x86_64.WithFeatures(" + GoExpr(active.Plus(g.Need)) + ")"
}