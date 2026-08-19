// x86_64/reloc_elf.go
//
// The ELF relocation kinds, from the System V AMD64 psABI §4.4.
//
// The full set is declared and only some of it is wired end to end. That is
// deliberate: a caller who names R_X86_64_TLSGD gets a diagnostic saying the
// mapping is missing, which sends them here, rather than "unknown
// relocation", which would send them looking for a typo. The mapped ones are
// marked in the table below and nowhere else, so the claim and the code
// cannot drift.
package x86_64

import (
	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/text"
)

// The ELF kinds. The value is the psABI's number with this package's ELF tag
// in the high byte; RelocRaw strips it back off for the writer.
const (
	R_X86_64_NONE            RelocKind = relocELF + 0
	R_X86_64_64              RelocKind = relocELF + 1
	R_X86_64_PC32            RelocKind = relocELF + 2
	R_X86_64_GOT32           RelocKind = relocELF + 3
	R_X86_64_PLT32           RelocKind = relocELF + 4
	R_X86_64_COPY            RelocKind = relocELF + 5
	R_X86_64_GLOB_DAT        RelocKind = relocELF + 6
	R_X86_64_JUMP_SLOT       RelocKind = relocELF + 7
	R_X86_64_RELATIVE        RelocKind = relocELF + 8
	R_X86_64_GOTPCREL        RelocKind = relocELF + 9
	R_X86_64_32              RelocKind = relocELF + 10
	R_X86_64_32S             RelocKind = relocELF + 11
	R_X86_64_16              RelocKind = relocELF + 12
	R_X86_64_PC16            RelocKind = relocELF + 13
	R_X86_64_8               RelocKind = relocELF + 14
	R_X86_64_PC8             RelocKind = relocELF + 15
	R_X86_64_DTPMOD64        RelocKind = relocELF + 16
	R_X86_64_DTPOFF64        RelocKind = relocELF + 17
	R_X86_64_TPOFF64         RelocKind = relocELF + 18
	R_X86_64_TLSGD           RelocKind = relocELF + 19
	R_X86_64_TLSLD           RelocKind = relocELF + 20
	R_X86_64_DTPOFF32        RelocKind = relocELF + 21
	R_X86_64_GOTTPOFF        RelocKind = relocELF + 22
	R_X86_64_TPOFF32         RelocKind = relocELF + 23
	R_X86_64_PC64            RelocKind = relocELF + 24
	R_X86_64_GOTOFF64        RelocKind = relocELF + 25
	R_X86_64_GOTPC32         RelocKind = relocELF + 26
	R_X86_64_GOT64           RelocKind = relocELF + 27
	R_X86_64_GOTPCREL64      RelocKind = relocELF + 28
	R_X86_64_GOTPC64         RelocKind = relocELF + 29
	R_X86_64_GOTPLT64        RelocKind = relocELF + 30
	R_X86_64_PLTOFF64        RelocKind = relocELF + 31
	R_X86_64_SIZE32          RelocKind = relocELF + 32
	R_X86_64_SIZE64          RelocKind = relocELF + 33
	R_X86_64_GOTPC32_TLSDESC RelocKind = relocELF + 34
	R_X86_64_TLSDESC_CALL    RelocKind = relocELF + 35
	R_X86_64_TLSDESC         RelocKind = relocELF + 36
	R_X86_64_IRELATIVE       RelocKind = relocELF + 37
	R_X86_64_RELATIVE64      RelocKind = relocELF + 38

	// 39 and 40 were PC32_BND and PLT32_BND. They are withdrawn from the
	// psABI and no current toolchain emits them; absent rather than
	// declared-and-refused, on the same footing as an unratified extension.

	R_X86_64_GOTPCRELX     RelocKind = relocELF + 41
	R_X86_64_REX_GOTPCRELX RelocKind = relocELF + 42
)

// elfRelocs is the table. Mapped marks the four kinds objectfile/elf can
// write today; everything else is declared so that naming it produces a
// diagnostic about the gap rather than about the spelling.
//
// The four are not an arbitrary subset: R_X86_64_64 is how data points at a
// symbol, PC32 is how a %rip-relative load reaches one, PLT32 is how a call
// does, and GOTPCREL is how either reaches one through the global offset
// table. Between them they are every relocation a freestanding object needs,
// and the rest are dynamic-linker and thread-local business.
func init() {
	type row struct {
		kind   RelocKind
		name   string
		size   int
		pcrel  bool
		mapped bool
	}

	for _, r := range []row{
		{R_X86_64_NONE, "R_X86_64_NONE", 0, false, true},
		{R_X86_64_64, "R_X86_64_64", 8, false, true},
		{R_X86_64_PC32, "R_X86_64_PC32", 4, true, true},
		{R_X86_64_GOT32, "R_X86_64_GOT32", 4, false, false},
		{R_X86_64_PLT32, "R_X86_64_PLT32", 4, true, true},
		{R_X86_64_COPY, "R_X86_64_COPY", 0, false, false},
		{R_X86_64_GLOB_DAT, "R_X86_64_GLOB_DAT", 8, false, false},
		{R_X86_64_JUMP_SLOT, "R_X86_64_JUMP_SLOT", 8, false, false},
		{R_X86_64_RELATIVE, "R_X86_64_RELATIVE", 8, false, false},
		{R_X86_64_GOTPCREL, "R_X86_64_GOTPCREL", 4, true, true},
		{R_X86_64_32, "R_X86_64_32", 4, false, false},
		{R_X86_64_32S, "R_X86_64_32S", 4, false, false},
		{R_X86_64_16, "R_X86_64_16", 2, false, false},
		{R_X86_64_PC16, "R_X86_64_PC16", 2, true, false},
		{R_X86_64_8, "R_X86_64_8", 1, false, false},
		{R_X86_64_PC8, "R_X86_64_PC8", 1, true, false},
		{R_X86_64_DTPMOD64, "R_X86_64_DTPMOD64", 8, false, false},
		{R_X86_64_DTPOFF64, "R_X86_64_DTPOFF64", 8, false, false},
		{R_X86_64_TPOFF64, "R_X86_64_TPOFF64", 8, false, false},
		{R_X86_64_TLSGD, "R_X86_64_TLSGD", 4, true, false},
		{R_X86_64_TLSLD, "R_X86_64_TLSLD", 4, true, false},
		{R_X86_64_DTPOFF32, "R_X86_64_DTPOFF32", 4, false, false},
		{R_X86_64_GOTTPOFF, "R_X86_64_GOTTPOFF", 4, true, false},
		{R_X86_64_TPOFF32, "R_X86_64_TPOFF32", 4, false, false},
		{R_X86_64_PC64, "R_X86_64_PC64", 8, true, false},
		{R_X86_64_GOTOFF64, "R_X86_64_GOTOFF64", 8, false, false},
		{R_X86_64_GOTPC32, "R_X86_64_GOTPC32", 4, true, false},
		{R_X86_64_GOT64, "R_X86_64_GOT64", 8, false, false},
		{R_X86_64_GOTPCREL64, "R_X86_64_GOTPCREL64", 8, true, false},
		{R_X86_64_GOTPC64, "R_X86_64_GOTPC64", 8, true, false},
		{R_X86_64_GOTPLT64, "R_X86_64_GOTPLT64", 8, false, false},
		{R_X86_64_PLTOFF64, "R_X86_64_PLTOFF64", 8, false, false},
		{R_X86_64_SIZE32, "R_X86_64_SIZE32", 4, false, false},
		{R_X86_64_SIZE64, "R_X86_64_SIZE64", 8, false, false},
		{R_X86_64_GOTPC32_TLSDESC, "R_X86_64_GOTPC32_TLSDESC", 4, true, false},
		{R_X86_64_TLSDESC_CALL, "R_X86_64_TLSDESC_CALL", 0, false, false},
		{R_X86_64_TLSDESC, "R_X86_64_TLSDESC", 16, false, false},
		{R_X86_64_IRELATIVE, "R_X86_64_IRELATIVE", 8, false, false},
		{R_X86_64_RELATIVE64, "R_X86_64_RELATIVE64", 8, false, false},
		{R_X86_64_GOTPCRELX, "R_X86_64_GOTPCRELX", 4, true, false},
		{R_X86_64_REX_GOTPCRELX, "R_X86_64_REX_GOTPCRELX", 4, true, false},
	} {
		registerReloc(r.kind, relocInfo{
			name:     r.name,
			platform: ELF,
			raw:      uint16(r.kind) & relocValueMask,
			size:     r.size,
			pcrel:    r.pcrel,
			mapped:   r.mapped,
		})
	}

	// What a field is for, when the caller named no kind.
	//
	// A branch gets PLT32 rather than PC32, which looks like a choice and is
	// not one: the linker resolves PLT32 to a direct call when the target
	// turns out to be local, so PLT32 is PC32 plus permission to insert a
	// stub. Emitting PC32 for a call to an external symbol produces an
	// object that links only when the symbol happens not to need one.
	registerDefault(ELF, encode.UseBranch, R_X86_64_PLT32)
	registerDefault(ELF, encode.UsePCRel, R_X86_64_PC32)
	registerDefault(ELF, encode.UseAbs, R_X86_64_64)

	registerAbs(ELF, 8, R_X86_64_64)
	registerAbs(ELF, 4, R_X86_64_32)

	// The dialect modifiers. gas's @PLT and NASM's `wrt ..plt` both arrive
	// as ModPLT and become the same number here.
	registerModifier(ELF, text.ModPLT, R_X86_64_PLT32)
	registerModifier(ELF, text.ModGOT, R_X86_64_GOT32)
	registerModifier(ELF, text.ModGOTPCREL, R_X86_64_GOTPCREL)
	registerModifier(ELF, text.ModGOTOFF, R_X86_64_GOTOFF64)
	registerModifier(ELF, text.ModTPOFF, R_X86_64_GOTTPOFF)
	registerModifier(ELF, text.ModDTPOFF, R_X86_64_DTPOFF32)
	registerModifier(ELF, text.ModTLSGD, R_X86_64_TLSGD)
	registerModifier(ELF, text.ModTLSLD, R_X86_64_TLSLD)
	registerModifier(ELF, text.ModSize, R_X86_64_SIZE32)
}

// elfAddend converts a fixup's logical addend to the raw one ELF records.
//
// This is the whole reason a caller never writes `Addend: -4`. A
// pc-relative relocation is computed from the place it patches, and the
// place is the start of the field — but the displacement the instruction
// means is from the *end of the instruction*. The difference is the field's
// own width plus whatever follows it, which is exactly Fixup.Tail, and the
// encoder knows it because it placed the field.
//
//	call puts          displacement ends the instruction: tail 0, addend -4
//	mov dword [rip+x], 5   four bytes of immediate follow: tail 4, addend -8
//
// Nobody writes either number.
func elfAddend(f fixup) int64 {
	if !f.pcrel {
		return f.addend
	}
	return f.addend - int64(f.size+f.tail)
}