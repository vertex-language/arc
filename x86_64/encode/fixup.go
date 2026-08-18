// x86_64/encode/fixup.go
package encode

import "github.com/vertex-language/arc/x86_64/operand"

// Fixup is a field this instruction left blank because its value is an
// address that is not yet a number.
//
// It is not a relocation. A relocation is a format's record with a format's
// kind and a format's addend convention, and this package knows about no
// format — it does not import the root, where the R_X86_64_*,
// IMAGE_REL_AMD64_* and X86_64_RELOC_* constants live. What it produces is
// everything the platform writer needs to build one.
type Fixup struct {
	// Offset is the field's position from the first byte of the
	// instruction. The section adds its own base.
	Offset int

	// Size is the field width in bytes: 1, 2, 4 or 8.
	Size int

	// Tail is how many bytes of this instruction follow the field.
	//
	// This is what a caller would otherwise have to know to write
	// Addend: -4. A call's displacement ends the instruction, so Tail is
	// zero; the displacement of `mov dword [rip+x], 5` is followed by four
	// bytes of immediate, so Tail is four and the raw ELF addend is -8. The
	// encoder knows because it placed the field.
	Tail int

	// PCRel is whether the field is relative to the end of the instruction.
	PCRel bool

	// Use is what the field is for, when the caller named no kind. The
	// platform writer maps it: a branch is R_X86_64_PLT32 under ELF,
	// IMAGE_REL_AMD64_REL32 under COFF and X86_64_RELOC_BRANCH under
	// Mach-O, and that mapping is the writer's because the constants are.
	Use Use

	// Kind is the relocation kind the caller asked for, or RelocNone.
	// Naming one overrides Use.
	Kind operand.RelocKind

	// Target is the label or symbol the field points at.
	Target operand.Target

	// Addend is the logical addend: the offset from the symbol the caller
	// meant, never adjusted for the width or position of the field.
	Addend int64
}

// Use is what a field is for, in terms that do not name a format.
type Use uint8

const (
	// UseAbs: the field holds the address itself.
	UseAbs Use = iota
	// UsePCRel: the field holds the distance from the end of the
	// instruction to the address — a %rip-relative data reference.
	UsePCRel
	// UseBranch: the same arithmetic as UsePCRel, but the target is code
	// and may be reached through a stub the linker inserts.
	UseBranch
)

func (u Use) String() string {
	switch u {
	case UsePCRel:
		return "pcrel"
	case UseBranch:
		return "branch"
	}
	return "abs"
}

// fixup records a fixup at the current end of the buffer, which is where the
// field is about to be written.
func (e *enc) fixup(f Fixup) {
	f.Offset = len(e.buf)
	e.fix = append(e.fix, f)
}