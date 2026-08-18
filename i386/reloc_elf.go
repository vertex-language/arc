package i386

// R_386_*, Intel386 psABI relocation types.
//
// The full psABI set is declared, not a portable subset — the arch hands
// objectfile/elf the number the specification assigned rather than
// selecting from a cross-arch enum. A few TLS sequence-point markers used
// only by linker relaxation (R_386_TLS_GD_PUSH/CALL/POP and their LDM
// counterparts) are omitted: arc's own encoder never emits them, since it
// does not perform the call-sequence relaxation they exist to mark.
const (
	R_386_NONE     = R_386_kind(0)
	R_386_32       = R_386_kind(1)
	R_386_PC32     = R_386_kind(2)
	R_386_GOT32    = R_386_kind(3)
	R_386_PLT32    = R_386_kind(4)
	R_386_COPY     = R_386_kind(5)
	R_386_GLOB_DAT = R_386_kind(6)
	R_386_JMP_SLOT = R_386_kind(7)
	R_386_RELATIVE = R_386_kind(8)
	R_386_GOTOFF   = R_386_kind(9)
	R_386_GOTPC    = R_386_kind(10)
	R_386_32PLT    = R_386_kind(11)

	R_386_TLS_TPOFF  = R_386_kind(14)
	R_386_TLS_IE     = R_386_kind(15)
	R_386_TLS_GOTIE  = R_386_kind(16)
	R_386_TLS_LE     = R_386_kind(17)
	R_386_TLS_GD     = R_386_kind(18)
	R_386_TLS_LDM    = R_386_kind(19)

	R_386_16   = R_386_kind(20)
	R_386_PC16 = R_386_kind(21)
	R_386_8    = R_386_kind(22)
	R_386_PC8  = R_386_kind(23)

	R_386_TLS_GD_32     = R_386_kind(24)
	R_386_TLS_LDM_32    = R_386_kind(28)
	R_386_TLS_LDO_32    = R_386_kind(32)
	R_386_TLS_IE_32     = R_386_kind(33)
	R_386_TLS_LE_32     = R_386_kind(34)
	R_386_TLS_DTPMOD32  = R_386_kind(35)
	R_386_TLS_DTPOFF32  = R_386_kind(36)
	R_386_TLS_TPOFF32   = R_386_kind(37)

	R_386_GOT32X = R_386_kind(43)
)

// R_386_kind exists only so the const block above can build RelocKind values
// through addReloc without repeating "RelocKind(addReloc(RelocKind(...)" at
// every line; it converts to RelocKind for free since both are uint32 under
// an alias.
type R_386_kind = RelocKind

func init() {
	addReloc(R_386_NONE, "R_386_NONE", ELF, 0, false)
	addReloc(R_386_32, "R_386_32", ELF, 4, false)
	addReloc(R_386_PC32, "R_386_PC32", ELF, 4, true)
	addReloc(R_386_GOT32, "R_386_GOT32", ELF, 4, false)
	addReloc(R_386_PLT32, "R_386_PLT32", ELF, 4, true)
	addReloc(R_386_COPY, "R_386_COPY", ELF, 0, false)
	addReloc(R_386_GLOB_DAT, "R_386_GLOB_DAT", ELF, 4, false)
	addReloc(R_386_JMP_SLOT, "R_386_JMP_SLOT", ELF, 4, false)
	addReloc(R_386_RELATIVE, "R_386_RELATIVE", ELF, 4, false)
	addReloc(R_386_GOTOFF, "R_386_GOTOFF", ELF, 4, false)
	addReloc(R_386_GOTPC, "R_386_GOTPC", ELF, 4, true)
	addReloc(R_386_32PLT, "R_386_32PLT", ELF, 4, false)

	addReloc(R_386_TLS_TPOFF, "R_386_TLS_TPOFF", ELF, 4, false)
	addReloc(R_386_TLS_IE, "R_386_TLS_IE", ELF, 4, false)
	addReloc(R_386_TLS_GOTIE, "R_386_TLS_GOTIE", ELF, 4, false)
	addReloc(R_386_TLS_LE, "R_386_TLS_LE", ELF, 4, false)
	addReloc(R_386_TLS_GD, "R_386_TLS_GD", ELF, 4, false)
	addReloc(R_386_TLS_LDM, "R_386_TLS_LDM", ELF, 4, false)

	addReloc(R_386_16, "R_386_16", ELF, 2, false)
	addReloc(R_386_PC16, "R_386_PC16", ELF, 2, true)
	addReloc(R_386_8, "R_386_8", ELF, 1, false)
	addReloc(R_386_PC8, "R_386_PC8", ELF, 1, true)

	addReloc(R_386_TLS_GD_32, "R_386_TLS_GD_32", ELF, 4, false)
	addReloc(R_386_TLS_LDM_32, "R_386_TLS_LDM_32", ELF, 4, false)
	addReloc(R_386_TLS_LDO_32, "R_386_TLS_LDO_32", ELF, 4, false)
	addReloc(R_386_TLS_IE_32, "R_386_TLS_IE_32", ELF, 4, false)
	addReloc(R_386_TLS_LE_32, "R_386_TLS_LE_32", ELF, 4, false)
	addReloc(R_386_TLS_DTPMOD32, "R_386_TLS_DTPMOD32", ELF, 4, false)
	addReloc(R_386_TLS_DTPOFF32, "R_386_TLS_DTPOFF32", ELF, 4, false)
	addReloc(R_386_TLS_TPOFF32, "R_386_TLS_TPOFF32", ELF, 4, false)

	addReloc(R_386_GOT32X, "R_386_GOT32X", ELF, 4, false)
}