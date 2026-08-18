package i386

import (
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// Re-exports of reg/ and operand/, so i386.EAX and i386/reg.EAX are the same
// value under two spellings and a caller building operands by hand never
// needs the second import. Emit's variadic and the typed helpers both take
// Operand; nothing outside reg/ and operand/ can satisfy it, which is what
// keeps a caller from passing this package something no i386 encoding
// exists for.

// Operand is anything an i386 instruction can take: a register, an
// immediate, a memory operand, a label, or a symbol reference.
type Operand = operand.Operand

// The general-purpose registers, three widths over eight architectural
// registers. R32 has no Parent — in 32-bit mode there is no wider register
// above it, which is the one fact that keeps this package from being a
// build tag on x86_64's.
const (
	EAX = reg.EAX
	ECX = reg.ECX
	EDX = reg.EDX
	EBX = reg.EBX
	ESP = reg.ESP
	EBP = reg.EBP
	ESI = reg.ESI
	EDI = reg.EDI

	AX = reg.AX
	CX = reg.CX
	DX = reg.DX
	BX = reg.BX
	SP = reg.SP
	BP = reg.BP
	SI = reg.SI
	DI = reg.DI

	AL = reg.AL
	CL = reg.CL
	DL = reg.DL
	BL = reg.BL
	AH = reg.AH
	CH = reg.CH
	DH = reg.DH
	BH = reg.BH
)

// Segment registers, x87, MMX, and the vector and mask files.
const (
	ES = reg.ES
	CS = reg.CS
	SS = reg.SS
	DS = reg.DS
	FS = reg.FS
	GS = reg.GS

	ST0, ST1, ST2, ST3, ST4, ST5, ST6, ST7 = reg.ST0, reg.ST1, reg.ST2, reg.ST3, reg.ST4, reg.ST5, reg.ST6, reg.ST7
	MM0, MM1, MM2, MM3, MM4, MM5, MM6, MM7 = reg.MM0, reg.MM1, reg.MM2, reg.MM3, reg.MM4, reg.MM5, reg.MM6, reg.MM7

	XMM0, XMM1, XMM2, XMM3, XMM4, XMM5, XMM6, XMM7 = reg.XMM0, reg.XMM1, reg.XMM2, reg.XMM3, reg.XMM4, reg.XMM5, reg.XMM6, reg.XMM7
	YMM0, YMM1, YMM2, YMM3, YMM4, YMM5, YMM6, YMM7 = reg.YMM0, reg.YMM1, reg.YMM2, reg.YMM3, reg.YMM4, reg.YMM5, reg.YMM6, reg.YMM7
	ZMM0, ZMM1, ZMM2, ZMM3, ZMM4, ZMM5, ZMM6, ZMM7 = reg.ZMM0, reg.ZMM1, reg.ZMM2, reg.ZMM3, reg.ZMM4, reg.ZMM5, reg.ZMM6, reg.ZMM7

	K0, K1, K2, K3, K4, K5, K6, K7 = reg.K0, reg.K1, reg.K2, reg.K3, reg.K4, reg.K5, reg.K6, reg.K7

	CR0, CR1, CR2, CR3, CR4, CR5, CR6, CR7 = reg.CR0, reg.CR1, reg.CR2, reg.CR3, reg.CR4, reg.CR5, reg.CR6, reg.CR7
	DR0, DR1, DR2, DR3, DR4, DR5, DR6, DR7 = reg.DR0, reg.DR1, reg.DR2, reg.DR3, reg.DR4, reg.DR5, reg.DR6, reg.DR7
)

// Imm is an immediate operand. Its width is the form's, not its own — see
// operand.Imm's own doc for why the type carries no width.
type Imm = operand.Imm

// The memory operand types, one per access width, plus their constructors.
// The width is a type, the same reason R32 and R8 are different types: a
// name that cannot distinguish two widths cannot be the name of an operand
// class.
type (
	M8   = operand.M8
	M16  = operand.M16
	M32  = operand.M32
	M64  = operand.M64
	M80  = operand.M80
	M128 = operand.M128
	M256 = operand.M256
	M512 = operand.M512
)

var (
	Mem8   = operand.Mem8
	Mem16  = operand.Mem16
	Mem32  = operand.Mem32
	Mem64  = operand.Mem64
	Mem80  = operand.Mem80
	Mem128 = operand.Mem128
	Mem256 = operand.Mem256
	Mem512 = operand.Mem512

	Abs8   = operand.Abs8
	Abs16  = operand.Abs16
	Abs32  = operand.Abs32
	Abs64  = operand.Abs64
	Abs80  = operand.Abs80
	Abs128 = operand.Abs128
	Abs256 = operand.Abs256
	Abs512 = operand.Abs512
)

// Label names an offset within one section, resolved at Serialize as a
// direct fixup with no relocation record.
type Label = operand.Label

// SymRef is a reference to a symbol, with the relocation kind that resolves
// it. Ref is the constructor, matching the spelling docs/builder.md uses at
// every call site.
type SymRef = operand.SymRef

var Ref = operand.Ref

// RelocKind identifies a relocation. The value space is this package's own
// — reloc.go declares the R_386_* and IMAGE_REL_I386_* constants — and
// operand/ carries the value without interpreting it, which is what lets a
// SymRef exist below the package that knows what R_386_PLT32 means.
type RelocKind = operand.RelocKind