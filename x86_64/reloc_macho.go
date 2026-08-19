// x86_64/reloc_macho.go
//
// The Mach-O relocation kinds, from <mach-o/x86_64/reloc.h>.
//
// `arc` does not add the leading underscore Mach-O tooling expects on C
// symbols. `_main` is a name you write, the same as it is in a .s file —
// which matters here because objectfile/macho *does* prepend one, so a name
// written with it arrives in the symbol table doubled.
package x86_64

import (
	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/text"
)

const (
	X86_64_RELOC_UNSIGNED RelocKind = relocMachO + 0
	X86_64_RELOC_SIGNED   RelocKind = relocMachO + 1
	X86_64_RELOC_BRANCH   RelocKind = relocMachO + 2
	X86_64_RELOC_GOT_LOAD RelocKind = relocMachO + 3
	X86_64_RELOC_GOT      RelocKind = relocMachO + 4

	// SUBTRACTOR is half a relocation: it names the symbol being subtracted
	// and must be immediately followed by an UNSIGNED naming the one being
	// added. A pair is how `.quad a - b` is recorded, which is the same
	// expression Assemble refuses to fold today — so the two gaps are the
	// same gap seen from opposite ends.
	X86_64_RELOC_SUBTRACTOR RelocKind = relocMachO + 5

	// The displaced SIGNED variants, for a pc-relative field followed by
	// one, two or four bytes of immediate. Mach-O encodes the tail in the
	// relocation type where ELF and COFF fold it into the addend, which is
	// the one place the three formats disagree about something other than
	// spelling.
	X86_64_RELOC_SIGNED_1 RelocKind = relocMachO + 6
	X86_64_RELOC_SIGNED_2 RelocKind = relocMachO + 7
	X86_64_RELOC_SIGNED_4 RelocKind = relocMachO + 8

	X86_64_RELOC_TLV RelocKind = relocMachO + 9
)

func init() {
	type row struct {
		kind   RelocKind
		name   string
		size   int
		pcrel  bool
		mapped bool
	}

	// Only four are mapped, and the two absences are the ones that hurt:
	// SIGNED is how a %rip-relative *data* reference is recorded on this
	// format, and objectfile/macho has no case for it. Until it does, `lea
	// rsi, [rip+msg]` assembles for ELF and COFF and is refused for Mach-O
	// with a message naming the gap — which is the right failure, but it is
	// a failure on an ordinary instruction rather than an exotic one.
	for _, r := range []row{
		{X86_64_RELOC_UNSIGNED, "X86_64_RELOC_UNSIGNED", 8, false, true},
		{X86_64_RELOC_SIGNED, "X86_64_RELOC_SIGNED", 4, true, false},
		{X86_64_RELOC_BRANCH, "X86_64_RELOC_BRANCH", 4, true, true},
		{X86_64_RELOC_GOT_LOAD, "X86_64_RELOC_GOT_LOAD", 4, true, true},
		{X86_64_RELOC_GOT, "X86_64_RELOC_GOT", 4, true, false},
		{X86_64_RELOC_SUBTRACTOR, "X86_64_RELOC_SUBTRACTOR", 8, false, false},
		{X86_64_RELOC_SIGNED_1, "X86_64_RELOC_SIGNED_1", 4, true, false},
		{X86_64_RELOC_SIGNED_2, "X86_64_RELOC_SIGNED_2", 4, true, false},
		{X86_64_RELOC_SIGNED_4, "X86_64_RELOC_SIGNED_4", 4, true, false},
		{X86_64_RELOC_TLV, "X86_64_RELOC_TLV", 4, true, true},
	} {
		registerReloc(r.kind, relocInfo{
			name:     r.name,
			platform: MachO,
			raw:      uint16(r.kind) & relocValueMask,
			size:     r.size,
			pcrel:    r.pcrel,
			mapped:   r.mapped,
		})
	}

	registerDefault(MachO, encode.UseBranch, X86_64_RELOC_BRANCH)
	registerDefault(MachO, encode.UsePCRel, X86_64_RELOC_SIGNED)
	registerDefault(MachO, encode.UseAbs, X86_64_RELOC_UNSIGNED)

	// UNSIGNED is the only absolute kind Mach-O has, at either width: the
	// length is in the relocation record's r_length field rather than in
	// the type, which is why one kind serves both sizes here and two are
	// needed everywhere else.
	registerAbs(MachO, 8, X86_64_RELOC_UNSIGNED)
	registerAbs(MachO, 4, X86_64_RELOC_UNSIGNED)

	registerModifier(MachO, text.ModPLT, X86_64_RELOC_BRANCH)
	registerModifier(MachO, text.ModGOT, X86_64_RELOC_GOT)
	registerModifier(MachO, text.ModGOTPCREL, X86_64_RELOC_GOT_LOAD)
	registerModifier(MachO, text.ModTLSGD, X86_64_RELOC_TLV)
}

// machoAddend converts a fixup's logical addend to the value
// objectfile/macho patches into the section bytes.
//
// This is where the three formats stop agreeing, and the difference is one
// word: which address the linker calls P.
//
// ELF and COFF both compute a pc-relative relocation from the start of the
// field, so the addend has to cancel the field's own width as well as the
// tail. Mach-O computes it from the *end* of the field — ld64 uses
// r_address plus the length — so the width is already accounted for and the
// addend cancels only the tail:
//
//	call puts              tail 0  →  addend  0   (ELF writes -4)
//	mov [rip+x], dword 5   tail 4  →  addend -4   (ELF writes -8)
//
// A shared conversion with a per-format offset would be one expression with
// a magic number in it, and this way each format states its own definition
// next to its own constants.
func machoAddend(f fixup) int64 {
	if !f.pcrel {
		return f.addend
	}
	return f.addend - int64(f.tail)
}