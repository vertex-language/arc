package encode

import (
	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// Fixup is a field this encoder left blank because its value is an address that
// is not yet a number.
//
// It is not a relocation. A relocation is a format's record with a format's
// kind and a format's addend convention, and this package knows about no
// format. What a Fixup carries is everything the platform writer needs to pick
// one and nothing that presumes which it picks.
type Fixup struct {
	// Offset is the byte offset of the instruction within its section. This
	// package fills it from Opts and never computes it: there is no section
	// here.
	Offset int

	// Field is where in the word the value goes, so the writer can patch a
	// resolved value in place — which is what COFF wants and ELF does not.
	Field isa.Field

	// Target is the symbol or label the address names.
	Target operand.Target

	// Role is which part of the address this field wants: the whole thing, its
	// page, its offset within a page, or either of those through the GOT.
	//
	// The role is the portable part and the only part a caller states. One
	// address reference on this architecture is usually two fixups — adrp for
	// the page and add or a load for the offset — and the kind each becomes is
	// per format.
	Role operand.AddrRole

	// Kind is a relocation the caller insisted on, or operand.RelocNone. A
	// named kind is a request: it blocks folding, because resolving a PLT
	// reference to a direct branch answers a different question.
	Kind operand.RelocKind

	// Addend is the logical offset from the symbol. The writer converts it to
	// the format's convention; no caller writes a correction here.
	Addend int64

	// Access is the width a memory operand is accessed at, or WidthNone.
	//
	// This is the field that pays for the fixup/relocation distinction. Under
	// ELF the low-twelve half of an address is ADD_ABS_LO12_NC in an add and
	// one of five LDST*_ABS_LO12_NC kinds in a load, chosen by the width the
	// immediate is scaled by. The caller does not know that width and the role
	// does not carry it — the form does, and it is copied here.
	Access operand.Width

	// Branch marks a field that takes a branch relocation rather than a data
	// one, which is AttrBranch read off the form.
	Branch bool

	// Bits and Scale describe the field's arithmetic: how many bits it holds,
	// and the power of two the value is divided by before it is placed. A
	// branch is /4, ADRP is /4096, a scaled load offset is /access width.
	Bits  uint8
	Scale uint8
}

// Tail is the number of bytes of the instruction that follow this field.
//
// It is always zero on this architecture and this function is a constant. Every
// instruction is one word, the field is inside it, and every pc-relative
// relocation is defined against the address of that word — so there is no
// A - (size + tail) correction and the entire x86_64 addend table collapses
// into nothing. It exists as a method rather than as an absent concept so a
// platform writer shared in shape with x86_64's reads the same on both.
func (Fixup) Tail() int { return 0 }

// PCRel reports whether the field is relative to the instruction's own address,
// which is what decides whether a fixup can fold at Serialize.
func (f Fixup) PCRel() bool {
	switch f.Role {
	case operand.RolePage, operand.RoleGotPage:
		return true
	case operand.RolePageOff, operand.RoleGotPageOff:
		return false
	}
	return f.Branch
}

// Symbolic reports whether the target is a bare label, which is the only kind
// of target a fold can resolve.
func (f Fixup) Symbolic() bool {
	_, isLabel := f.Target.(operand.Label)
	return !isLabel
}