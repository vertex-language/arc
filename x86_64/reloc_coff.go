// x86_64/reloc_coff.go
//
// The COFF relocation kinds, from the PE/COFF specification's
// IMAGE_REL_AMD64_* table.
//
// Win64 dropped the cdecl decoration that i386 COFF carries: no leading
// underscore, no trailing @N. A symbol goes into the table spelled the way
// it was written, which is why write_coff.go mangles nothing and i386's
// does.
package x86_64

import (
	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/text"
)

const (
	IMAGE_REL_AMD64_ABSOLUTE RelocKind = relocCOFF + 0x00
	IMAGE_REL_AMD64_ADDR64   RelocKind = relocCOFF + 0x01
	IMAGE_REL_AMD64_ADDR32   RelocKind = relocCOFF + 0x02
	IMAGE_REL_AMD64_ADDR32NB RelocKind = relocCOFF + 0x03
	IMAGE_REL_AMD64_REL32    RelocKind = relocCOFF + 0x04

	// The displaced REL32 variants: the same pc-relative field, computed
	// from one to five bytes further on. They exist because a COFF REL32 is
	// defined against P+4 and an instruction whose displacement is followed
	// by an immediate needs P+5 through P+9 instead.
	//
	// This tree does not emit them, and does not need to: encode/ records a
	// Tail on every fixup and the addend conversion below folds it into the
	// value, which reaches the same displacement through the plain REL32.
	// They are declared because a decoder or a reader of somebody else's
	// object will meet them.
	IMAGE_REL_AMD64_REL32_1 RelocKind = relocCOFF + 0x05
	IMAGE_REL_AMD64_REL32_2 RelocKind = relocCOFF + 0x06
	IMAGE_REL_AMD64_REL32_3 RelocKind = relocCOFF + 0x07
	IMAGE_REL_AMD64_REL32_4 RelocKind = relocCOFF + 0x08
	IMAGE_REL_AMD64_REL32_5 RelocKind = relocCOFF + 0x09

	IMAGE_REL_AMD64_SECTION  RelocKind = relocCOFF + 0x0A
	IMAGE_REL_AMD64_SECREL   RelocKind = relocCOFF + 0x0B
	IMAGE_REL_AMD64_SECREL7  RelocKind = relocCOFF + 0x0C
	IMAGE_REL_AMD64_TOKEN    RelocKind = relocCOFF + 0x0D
	IMAGE_REL_AMD64_SREL32   RelocKind = relocCOFF + 0x0E
	IMAGE_REL_AMD64_PAIR     RelocKind = relocCOFF + 0x0F
	IMAGE_REL_AMD64_SSPAN32  RelocKind = relocCOFF + 0x10
)

func init() {
	type row struct {
		kind   RelocKind
		name   string
		size   int
		pcrel  bool
		mapped bool
	}

	for _, r := range []row{
		{IMAGE_REL_AMD64_ABSOLUTE, "IMAGE_REL_AMD64_ABSOLUTE", 0, false, false},
		{IMAGE_REL_AMD64_ADDR64, "IMAGE_REL_AMD64_ADDR64", 8, false, true},
		{IMAGE_REL_AMD64_ADDR32, "IMAGE_REL_AMD64_ADDR32", 4, false, true},
		{IMAGE_REL_AMD64_ADDR32NB, "IMAGE_REL_AMD64_ADDR32NB", 4, false, true},
		{IMAGE_REL_AMD64_REL32, "IMAGE_REL_AMD64_REL32", 4, true, true},
		{IMAGE_REL_AMD64_REL32_1, "IMAGE_REL_AMD64_REL32_1", 4, true, false},
		{IMAGE_REL_AMD64_REL32_2, "IMAGE_REL_AMD64_REL32_2", 4, true, false},
		{IMAGE_REL_AMD64_REL32_3, "IMAGE_REL_AMD64_REL32_3", 4, true, false},
		{IMAGE_REL_AMD64_REL32_4, "IMAGE_REL_AMD64_REL32_4", 4, true, false},
		{IMAGE_REL_AMD64_REL32_5, "IMAGE_REL_AMD64_REL32_5", 4, true, false},
		{IMAGE_REL_AMD64_SECTION, "IMAGE_REL_AMD64_SECTION", 2, false, false},
		{IMAGE_REL_AMD64_SECREL, "IMAGE_REL_AMD64_SECREL", 4, false, true},
		{IMAGE_REL_AMD64_SECREL7, "IMAGE_REL_AMD64_SECREL7", 1, false, false},
		{IMAGE_REL_AMD64_TOKEN, "IMAGE_REL_AMD64_TOKEN", 4, false, false},
		{IMAGE_REL_AMD64_SREL32, "IMAGE_REL_AMD64_SREL32", 4, false, false},
		{IMAGE_REL_AMD64_PAIR, "IMAGE_REL_AMD64_PAIR", 0, false, false},
		{IMAGE_REL_AMD64_SSPAN32, "IMAGE_REL_AMD64_SSPAN32", 4, false, false},
	} {
		registerReloc(r.kind, relocInfo{
			name:     r.name,
			platform: COFF,
			raw:      uint16(r.kind) & relocValueMask,
			size:     r.size,
			pcrel:    r.pcrel,
			mapped:   r.mapped,
		})
	}

	// COFF has no PLT and no separate branch relocation: a call and a
	// %rip-relative load are the same REL32, and whether the target needs a
	// thunk is the linker's problem rather than a fact recorded in the
	// object. Two Uses, one kind — which is exactly the sort of collapse a
	// portable relocation vocabulary would have to invent a name for.
	registerDefault(COFF, encode.UseBranch, IMAGE_REL_AMD64_REL32)
	registerDefault(COFF, encode.UsePCRel, IMAGE_REL_AMD64_REL32)
	registerDefault(COFF, encode.UseAbs, IMAGE_REL_AMD64_ADDR64)

	registerAbs(COFF, 8, IMAGE_REL_AMD64_ADDR64)
	registerAbs(COFF, 4, IMAGE_REL_AMD64_ADDR32)

	// The dialect modifiers that have a Win64 meaning.
	//
	// @PLT is one of them, and it maps to plain REL32 rather than being
	// refused: a source written for ELF and assembled for COFF is asking
	// for a call that may go through a stub, which is what REL32 already
	// permits. @GOT and the TLS modifiers have no COFF equivalent at all —
	// Windows reaches thread-local storage through a different mechanism
	// entirely — so they are absent, and RelocForModifier says so by name.
	registerModifier(COFF, text.ModPLT, IMAGE_REL_AMD64_REL32)
	registerModifier(COFF, text.ModTPOFF, IMAGE_REL_AMD64_SECREL)
}

// coffAddend converts a fixup's logical addend to the value objectfile/coff
// patches into the section bytes.
//
// COFF records no addend field: the linker reads it out of the instruction.
// The arithmetic works out to the same expression ELF uses, and it is worth
// spelling out why, because it looks like a coincidence and is not.
//
// A COFF REL32 is defined as S + field - (P + 4), where P is the start of
// the field — the +4 is intrinsic to the relocation type. What the
// instruction wants is S - (P + size + tail). Solving, field = -(size +
// tail) + 4, and objectfile/coff adds the 4 itself, so what it wants handed
// to it is -(size + tail): the ELF convention exactly, which is what its
// own doc comment says it speaks.
//
// The two agree here and stop agreeing on Mach-O, which is why each format
// gets its own function rather than a shared one with a flag.
func coffAddend(f fixup) int64 {
	if !f.pcrel {
		return f.addend
	}
	return f.addend - int64(f.size+f.tail)
}