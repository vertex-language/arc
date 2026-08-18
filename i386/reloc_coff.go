package i386

// IMAGE_REL_I386_*, the PE/COFF relocation types for the i386 machine.
//
// These share a Go type with reloc_elf.go's R_386_* constants, and the raw
// numeric spaces genuinely collide — R_386_NONE and IMAGE_REL_I386_ABSOLUTE
// are both specified as 0. coffRelocBase pushes every COFF value clear of
// every ELF one so relocTable's keys stay unique; the offset is invisible
// above this package and write_coff.go strips it back out with coffType
// before the byte reaches objectfile/coff, which only ever sees the true
// PE/COFF number.
const coffRelocBase RelocKind = 1 << 16

const (
	IMAGE_REL_I386_ABSOLUTE = coffRelocBase + 0x0000
	IMAGE_REL_I386_DIR16    = coffRelocBase + 0x0001
	IMAGE_REL_I386_REL16    = coffRelocBase + 0x0002
	IMAGE_REL_I386_DIR32    = coffRelocBase + 0x0006
	IMAGE_REL_I386_DIR32NB  = coffRelocBase + 0x0007
	IMAGE_REL_I386_SECTION  = coffRelocBase + 0x000A
	IMAGE_REL_I386_SECREL   = coffRelocBase + 0x000B
	IMAGE_REL_I386_REL32    = coffRelocBase + 0x0014
)

func init() {
	addReloc(IMAGE_REL_I386_ABSOLUTE, "IMAGE_REL_I386_ABSOLUTE", COFF, 0, false)
	addReloc(IMAGE_REL_I386_DIR16, "IMAGE_REL_I386_DIR16", COFF, 2, false)
	addReloc(IMAGE_REL_I386_REL16, "IMAGE_REL_I386_REL16", COFF, 2, true)
	addReloc(IMAGE_REL_I386_DIR32, "IMAGE_REL_I386_DIR32", COFF, 4, false)
	addReloc(IMAGE_REL_I386_DIR32NB, "IMAGE_REL_I386_DIR32NB", COFF, 4, false)
	addReloc(IMAGE_REL_I386_SECTION, "IMAGE_REL_I386_SECTION", COFF, 2, false)
	addReloc(IMAGE_REL_I386_SECREL, "IMAGE_REL_I386_SECREL", COFF, 4, false)
	addReloc(IMAGE_REL_I386_REL32, "IMAGE_REL_I386_REL32", COFF, 4, true)
}

// coffType strips coffRelocBase, returning the raw 16-bit PE/COFF relocation
// type write_coff.go writes into a relocation record. It is write_coff.go's
// to call, once that file exists — declared here, beside the constants it
// inverts, rather than there.
func coffType(k RelocKind) uint16 { return uint16(k - coffRelocBase) }