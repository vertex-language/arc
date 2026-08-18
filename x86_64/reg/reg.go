// x86_64/reg/reg.go
//
// Package reg is every physical register the AMD64 architecture has, at every
// width, with the facts encode/, decode/, isa/ and text/ need about them.
//
// Types are named Reg8/Reg16/Reg32/Reg64 rather than R8/R16/R32/R64 because
// R8 is a register (%r8) and R16..R31 are registers under APX. A name cannot
// be both a type and a constant in one package.
package reg

// Class is the operand class a register belongs to. isa/ declares each form's
// operand slots in these terms.
type Class uint8

const (
	ClassGP8 Class = iota
	ClassGP16
	ClassGP32
	ClassGP64
	ClassSreg
	ClassSt
	ClassMm
	ClassXmm
	ClassYmm
	ClassZmm
	ClassK
	ClassTmm
	ClassCr
	ClassDr
)

// File is a physical register file. Two registers can only overlap if they
// name the same index in the same file.
type File uint8

const (
	FileGPR File = iota
	FileVec
	FileX87 // ST(i), and MM(i) aliased onto its low 64 bits
	FileMask
	FileTile
	FileSeg
	FileCtrl
	FileDebug
)

// Loc locates a register as a bit range within one entry of one file. Hi is
// exclusive. AL is {FileGPR, 0, 0, 8}; AH is {FileGPR, 0, 8, 16}; ZMM3 is
// {FileVec, 3, 0, 512}.
type Loc struct {
	File  File
	Index uint8
	Lo    uint16
	Hi    uint16
}

// NoDWARF is DWARF's answer for registers the psABI assigns no number to.
const NoDWARF = -1

// Reg is what every register in this package answers. It exists so encode/,
// decode/ and text/ can hold a register without knowing its width; every
// call that emits bytes takes a concrete type.
type Reg interface {
	// Num is the architectural register number, 0–31. This is the value
	// that goes into ModRM.reg, ModRM.rm, SIB.index, SIB.base or the low
	// opcode bits, spread across REX/VEX/EVEX extension fields by encode/.
	Num() uint8

	// Bits is the width in bits.
	Bits() int

	Class() Class
	Loc() Loc

	// DWARF is the psABI DWARF register number, or NoDWARF. It is NOT
	// derived from Num — the two orderings differ (see dwarf.go).
	DWARF() int

	// Save is whether the register survives a call on the given platform.
	Save(Platform) Preservation

	Name() string
}

// Platform is the object format, which fixes the calling convention: ELF and
// Mach-O are System V, COFF is Win64. It mirrors the root package's Platform
// rather than importing it — nothing imports an arch package.
type Platform uint8

const (
	ELF Platform = iota
	COFF
	MachO
	Flat // no OS, no linker; SysV rules apply by default
)

func (p Platform) win64() bool { return p == COFF }

// Preservation is whether a register survives a call. It is three-valued
// because Win64 preserves XMM6–XMM15 but not the upper half of YMM6–YMM15:
// the same physical register answers differently at different widths.
type Preservation uint8

const (
	Volatile     Preservation = iota // caller-saved
	Preserved                        // callee-saved
	PreservedLow                     // low 128 bits callee-saved, above volatile
)

func (p Preservation) String() string {
	switch p {
	case Preserved:
		return "callee-saved"
	case PreservedLow:
		return "callee-saved below 128 bits"
	default:
		return "caller-saved"
	}
}

// Overlaps reports whether writing a can be observed by reading b. AL and AH
// do not overlap each other; both overlap RAX. XMM4 overlaps ZMM4. MM2
// overlaps ST2.
func Overlaps(a, b Reg) bool {
	x, y := a.Loc(), b.Loc()
	return x.File == y.File && x.Index == y.Index && x.Lo < y.Hi && y.Lo < x.Hi
}