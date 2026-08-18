package isa

import (
	"strings"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/operand"
)

// Form is one encodable form of one instruction.
//
// A Form is data. It names the bytes, the fields, and the gate; it holds no
// method that produces an encoding, because producing an encoding is encode/'s
// job and it takes a Form and operand values.
type Form struct {
	// Mnemonic is lowercase and canonical, as the SDM spells it.
	Mnemonic string

	// Ops are the operands in source order — the order they are written in
	// Intel syntax, destination first. The GAS printer reverses them; that is
	// a spelling and lives in text/gas.
	Ops []Op

	// Opcode is the opcode bytes, escape prefixes included: {0x0f, 0xaf} for
	// IMUL r32, r/m32.
	Opcode []byte

	// Ext is the /digit opcode extension carried in ModRM.reg, or NoExt.
	Ext int8

	// OpSize16 marks a form that needs the 0x66 operand-size override in
	// 32-bit code. The prefix is a consequence of the form, not a separate
	// operand, so it lives here rather than being decided by the encoder.
	OpSize16 bool

	// Level is the base CPU level that introduced this form.
	Level feature.Level

	// Feat is the extension that gates this form, when one does.
	Feat    feature.Feature
	HasFeat bool

	// AliasOf names the form this is a vendor-documented alias of. The
	// encoder emits the target and arc explain names the target, so that a
	// listing says what the silicon does. Aliases arc would have to invent do
	// not exist.
	AliasOf string
}

// NoExt marks a form with no /digit extension.
const NoExt int8 = -1

// Enabled reports whether s permits this form.
func (f *Form) Enabled(s feature.Set) bool {
	if !s.AtLeast(f.Level) {
		return false
	}
	if f.HasFeat && !s.Has(f.Feat) {
		return false
	}
	return true
}

// Gate names the flag that would allow this form, for a diagnostic. It is
// empty when the form is in the baseline.
func (f *Form) Gate() string {
	if f.HasFeat {
		return f.Feat.String()
	}
	if f.Level > feature.Baseline {
		return f.Level.String()
	}
	return ""
}

// Matches reports whether ops can be this form.
func (f *Form) Matches(ops []operand.Operand) bool {
	if len(ops) != len(f.Ops) {
		return false
	}
	for i, o := range ops {
		if !f.Ops[i].Class.Matches(o) {
			return false
		}
	}
	return true
}

// Signature is the form as the SDM writes it: MOV r/m32, r32.
func (f *Form) Signature() string {
	var b strings.Builder
	b.WriteString(strings.ToUpper(f.Mnemonic))
	for i, o := range f.Ops {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteString(", ")
		}
		b.WriteString(o.Class.String())
	}
	return b.String()
}

// HelperName is the name of the generated typed helper for this form:
// mnemonic, then each operand class in order, '/' dropped. MovRM32R32,
// AddRM32Imm8S, AddEAXImm32.
//
// The obvious shorter scheme — MovRI, MovRM — cannot distinguish MOV r32,
// imm32 from MOV r/m32, imm32, and a name that cannot distinguish two forms
// cannot be the name of a form.
func (f *Form) HelperName() string {
	var b strings.Builder
	b.WriteString(strings.ToUpper(f.Mnemonic[:1]))
	b.WriteString(f.Mnemonic[1:])
	for _, o := range f.Ops {
		b.WriteString(helperNames[o.Class])
	}
	return b.String()
}

// Bytes is the opcode as arc isa prints it: hex, with /digit or /r.
func (f *Form) Bytes() string {
	var b strings.Builder
	for i, x := range f.Opcode {
		if i > 0 {
			b.WriteByte(' ')
		}
		const hex = "0123456789abcdef"
		b.WriteByte(hex[x>>4])
		b.WriteByte(hex[x&15])
	}
	switch {
	case f.Ext >= 0:
		b.WriteString(" /")
		b.WriteByte(byte('0' + f.Ext))
	case f.hasSlot(SlotReg):
		b.WriteString(" /r")
	case f.hasSlot(SlotOpcode):
		b.WriteString("+r")
	}
	return b.String()
}

func (f *Form) hasSlot(s Slot) bool {
	for _, o := range f.Ops {
		if o.Slot == s {
			return true
		}
	}
	return false
}