// x86_64/operand.go
//
// The re-export of reg/ and operand/, so a caller never needs the second
// import, and the one interface neither of them can declare.
//
// The register names are aliases of constants, not new constants: x86_64.RAX
// and reg.RAX are the same value of the same type, and a helper generated
// against reg.Reg64 takes either spelling. A parallel set of definitions
// would be a set that can drift by one.
package x86_64

import (
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Operand is something Emit accepts.
//
// This interface is declared here and nowhere below, because reg/ and
// operand/ are two packages and one of them would have to import the other
// to state it — operand/ already imports reg/, so an interface in operand/
// that reg/ implemented would close the cycle.
//
// It is deliberately loose: the only method every operand value has in
// common is String, so this admits any Stringer. What it buys is that the
// common mistakes — passing an int, a string, a *Section — are compile
// errors at the call rather than an OperandError at encode time. What it
// does not buy is exhaustiveness, and encode/'s type switch is still the
// thing that decides; a Stringer it has no case for comes back as an
// OperandError naming its Go type.
type Operand interface {
	String() string
}

// Reg is any physical register, at any width.
type Reg = reg.Reg

// The register types, one per width and file. They are named Reg8 through
// Reg64 rather than R8 through R64 because R8 is a register — %r8 — and a
// name cannot be both a type and a constant in one package.
type (
	Reg8  = reg.Reg8
	Reg16 = reg.Reg16
	Reg32 = reg.Reg32
	Reg64 = reg.Reg64
	Sreg  = reg.Sreg
	St    = reg.St
	Mm    = reg.Mm
	Xmm   = reg.Xmm
	Ymm   = reg.Ymm
	Zmm   = reg.Zmm
	K     = reg.K
	Tmm   = reg.Tmm
	Cr    = reg.Cr
	Dr    = reg.Dr
)

// Class is the operand class a register belongs to.
type Class = reg.Class

// Preservation is whether a register survives a call. It is three-valued
// because Win64 preserves XMM6–XMM15 but not the upper half of YMM6–YMM15.
type Preservation = reg.Preservation

const (
	Volatile     = reg.Volatile
	Preserved    = reg.Preserved
	PreservedLow = reg.PreservedLow
)

// NoDWARF is DWARF's answer for a register the psABI assigns no number to.
const NoDWARF = reg.NoDWARF

// ReturnAddress is the DWARF column for the return address. It is not a
// register; the value lives at 0(%rsp).
const ReturnAddress = reg.ReturnAddress

// ---- general-purpose registers -----------------------------------------

const (
	RAX Reg64 = reg.RAX
	RCX Reg64 = reg.RCX
	RDX Reg64 = reg.RDX
	RBX Reg64 = reg.RBX
	RSP Reg64 = reg.RSP
	RBP Reg64 = reg.RBP
	RSI Reg64 = reg.RSI
	RDI Reg64 = reg.RDI
	R8  Reg64 = reg.R8
	R9  Reg64 = reg.R9
	R10 Reg64 = reg.R10
	R11 Reg64 = reg.R11
	R12 Reg64 = reg.R12
	R13 Reg64 = reg.R13
	R14 Reg64 = reg.R14
	R15 Reg64 = reg.R15
)

const (
	EAX  Reg32 = reg.EAX
	ECX  Reg32 = reg.ECX
	EDX  Reg32 = reg.EDX
	EBX  Reg32 = reg.EBX
	ESP  Reg32 = reg.ESP
	EBP  Reg32 = reg.EBP
	ESI  Reg32 = reg.ESI
	EDI  Reg32 = reg.EDI
	R8D  Reg32 = reg.R8D
	R9D  Reg32 = reg.R9D
	R10D Reg32 = reg.R10D
	R11D Reg32 = reg.R11D
	R12D Reg32 = reg.R12D
	R13D Reg32 = reg.R13D
	R14D Reg32 = reg.R14D
	R15D Reg32 = reg.R15D
)

const (
	AX   Reg16 = reg.AX
	CX   Reg16 = reg.CX
	DX   Reg16 = reg.DX
	BX   Reg16 = reg.BX
	SP   Reg16 = reg.SP
	BP   Reg16 = reg.BP
	SI   Reg16 = reg.SI
	DI   Reg16 = reg.DI
	R8W  Reg16 = reg.R8W
	R9W  Reg16 = reg.R9W
	R10W Reg16 = reg.R10W
	R11W Reg16 = reg.R11W
	R12W Reg16 = reg.R12W
	R13W Reg16 = reg.R13W
	R14W Reg16 = reg.R14W
	R15W Reg16 = reg.R15W
)

// The byte registers. SPL, BPL, SIL and DIL exist only under REX; AH, CH, DH
// and BH exist only without one, because they are the same four encodings.
// Naming one of each in a single instruction is a RexConflictError and not a
// preference — the byte would select the other register.
const (
	AL   Reg8 = reg.AL
	CL   Reg8 = reg.CL
	DL   Reg8 = reg.DL
	BL   Reg8 = reg.BL
	SPL  Reg8 = reg.SPL
	BPL  Reg8 = reg.BPL
	SIL  Reg8 = reg.SIL
	DIL  Reg8 = reg.DIL
	R8B  Reg8 = reg.R8B
	R9B  Reg8 = reg.R9B
	R10B Reg8 = reg.R10B
	R11B Reg8 = reg.R11B
	R12B Reg8 = reg.R12B
	R13B Reg8 = reg.R13B
	R14B Reg8 = reg.R14B
	R15B Reg8 = reg.R15B
	AH   Reg8 = reg.AH
	CH   Reg8 = reg.CH
	DH   Reg8 = reg.DH
	BH   Reg8 = reg.BH
)

// ---- segment, x87, MMX -------------------------------------------------

// In 64-bit mode only FS and GS carry a nonzero base; the others are ignored
// for address computation and their overrides are accepted and inert.
const (
	ES Sreg = reg.ES
	CS Sreg = reg.CS
	SS Sreg = reg.SS
	DS Sreg = reg.DS
	FS Sreg = reg.FS
	GS Sreg = reg.GS
)

const (
	ST0 St = reg.ST0
	ST1 St = reg.ST1
	ST2 St = reg.ST2
	ST3 St = reg.ST3
	ST4 St = reg.ST4
	ST5 St = reg.ST5
	ST6 St = reg.ST6
	ST7 St = reg.ST7
)

const (
	MM0 Mm = reg.MM0
	MM1 Mm = reg.MM1
	MM2 Mm = reg.MM2
	MM3 Mm = reg.MM3
	MM4 Mm = reg.MM4
	MM5 Mm = reg.MM5
	MM6 Mm = reg.MM6
	MM7 Mm = reg.MM7
)

// ---- vector ------------------------------------------------------------

// XMM16–31, YMM16–31 and ZMM16–31 are reachable only through EVEX: no REX
// and no VEX prefix has a bit that names them, so a legacy or VEX form
// naming one has no encoding at all and is a RegisterError rather than a
// truncation to register 0.
const (
	XMM0, YMM0, ZMM0    = reg.XMM0, reg.YMM0, reg.ZMM0
	XMM1, YMM1, ZMM1    = reg.XMM1, reg.YMM1, reg.ZMM1
	XMM2, YMM2, ZMM2    = reg.XMM2, reg.YMM2, reg.ZMM2
	XMM3, YMM3, ZMM3    = reg.XMM3, reg.YMM3, reg.ZMM3
	XMM4, YMM4, ZMM4    = reg.XMM4, reg.YMM4, reg.ZMM4
	XMM5, YMM5, ZMM5    = reg.XMM5, reg.YMM5, reg.ZMM5
	XMM6, YMM6, ZMM6    = reg.XMM6, reg.YMM6, reg.ZMM6
	XMM7, YMM7, ZMM7    = reg.XMM7, reg.YMM7, reg.ZMM7
	XMM8, YMM8, ZMM8    = reg.XMM8, reg.YMM8, reg.ZMM8
	XMM9, YMM9, ZMM9    = reg.XMM9, reg.YMM9, reg.ZMM9
	XMM10, YMM10, ZMM10 = reg.XMM10, reg.YMM10, reg.ZMM10
	XMM11, YMM11, ZMM11 = reg.XMM11, reg.YMM11, reg.ZMM11
	XMM12, YMM12, ZMM12 = reg.XMM12, reg.YMM12, reg.ZMM12
	XMM13, YMM13, ZMM13 = reg.XMM13, reg.YMM13, reg.ZMM13
	XMM14, YMM14, ZMM14 = reg.XMM14, reg.YMM14, reg.ZMM14
	XMM15, YMM15, ZMM15 = reg.XMM15, reg.YMM15, reg.ZMM15
	XMM16, YMM16, ZMM16 = reg.XMM16, reg.YMM16, reg.ZMM16
	XMM17, YMM17, ZMM17 = reg.XMM17, reg.YMM17, reg.ZMM17
	XMM18, YMM18, ZMM18 = reg.XMM18, reg.YMM18, reg.ZMM18
	XMM19, YMM19, ZMM19 = reg.XMM19, reg.YMM19, reg.ZMM19
	XMM20, YMM20, ZMM20 = reg.XMM20, reg.YMM20, reg.ZMM20
	XMM21, YMM21, ZMM21 = reg.XMM21, reg.YMM21, reg.ZMM21
	XMM22, YMM22, ZMM22 = reg.XMM22, reg.YMM22, reg.ZMM22
	XMM23, YMM23, ZMM23 = reg.XMM23, reg.YMM23, reg.ZMM23
	XMM24, YMM24, ZMM24 = reg.XMM24, reg.YMM24, reg.ZMM24
	XMM25, YMM25, ZMM25 = reg.XMM25, reg.YMM25, reg.ZMM25
	XMM26, YMM26, ZMM26 = reg.XMM26, reg.YMM26, reg.ZMM26
	XMM27, YMM27, ZMM27 = reg.XMM27, reg.YMM27, reg.ZMM27
	XMM28, YMM28, ZMM28 = reg.XMM28, reg.YMM28, reg.ZMM28
	XMM29, YMM29, ZMM29 = reg.XMM29, reg.YMM29, reg.ZMM29
	XMM30, YMM30, ZMM30 = reg.XMM30, reg.YMM30, reg.ZMM30
	XMM31, YMM31, ZMM31 = reg.XMM31, reg.YMM31, reg.ZMM31
)

// K0 is legal as a source and means "no mask" as a writemask. isa/ gates
// that distinction; the register itself is just K0.
const (
	K0 K = reg.K0
	K1 K = reg.K1
	K2 K = reg.K2
	K3 K = reg.K3
	K4 K = reg.K4
	K5 K = reg.K5
	K6 K = reg.K6
	K7 K = reg.K7
)

// AMX tiles. The shape is set at run time by LDTILECFG, so nothing about a
// tile's width is checkable here.
const (
	TMM0 Tmm = reg.TMM0
	TMM1 Tmm = reg.TMM1
	TMM2 Tmm = reg.TMM2
	TMM3 Tmm = reg.TMM3
	TMM4 Tmm = reg.TMM4
	TMM5 Tmm = reg.TMM5
	TMM6 Tmm = reg.TMM6
	TMM7 Tmm = reg.TMM7
)

// CR8 through CR15 and DR8 through DR15 are reachable only through REX.R.
const (
	CR0  Cr = reg.CR0
	CR1  Cr = reg.CR1
	CR2  Cr = reg.CR2
	CR3  Cr = reg.CR3
	CR4  Cr = reg.CR4
	CR5  Cr = reg.CR5
	CR6  Cr = reg.CR6
	CR7  Cr = reg.CR7
	CR8  Cr = reg.CR8
	CR9  Cr = reg.CR9
	CR10 Cr = reg.CR10
	CR11 Cr = reg.CR11
	CR12 Cr = reg.CR12
	CR13 Cr = reg.CR13
	CR14 Cr = reg.CR14
	CR15 Cr = reg.CR15
)

const (
	DR0  Dr = reg.DR0
	DR1  Dr = reg.DR1
	DR2  Dr = reg.DR2
	DR3  Dr = reg.DR3
	DR4  Dr = reg.DR4
	DR5  Dr = reg.DR5
	DR6  Dr = reg.DR6
	DR7  Dr = reg.DR7
	DR8  Dr = reg.DR8
	DR9  Dr = reg.DR9
	DR10 Dr = reg.DR10
	DR11 Dr = reg.DR11
	DR12 Dr = reg.DR12
	DR13 Dr = reg.DR13
	DR14 Dr = reg.DR14
	DR15 Dr = reg.DR15
)

// LookupReg resolves a bare register name. The caller has already stripped
// any dialect sigil; case folding is the lexer's job, because gas and nasm
// disagree about it.
func LookupReg(name string) (Reg, bool) { return reg.Lookup(name) }

// Overlaps reports whether writing a can be observed by reading b. AL and AH
// do not overlap each other; both overlap RAX. XMM4 overlaps ZMM4.
func Overlaps(a, b Reg) bool { return reg.Overlaps(a, b) }

// ---- operands ----------------------------------------------------------

// Width is an operand width in bits. Zero means unspecified, which is a
// state lea genuinely occupies: it computes an address and never loads
// through it.
type Width = operand.Width

const (
	WidthNone = operand.WidthNone
	W8        = operand.W8
	W16       = operand.W16
	W32       = operand.W32
	W64       = operand.W64
	W128      = operand.W128
	W256      = operand.W256
	W512      = operand.W512
)

// Imm is an immediate. It holds the value the caller wrote; the field width
// is the encoder's choice, made against the form it resolved.
type Imm = operand.Imm

// Uimm builds an immediate from an unsigned value. Values above 2^63-1 wrap
// to negative, which is the same bit pattern and the same bytes.
func Uimm(v uint64) Imm { return operand.Uimm(v) }

// Mem is a memory reference with no width of its own, which is what Abs,
// AbsSym and RIPRel produce. The width-carrying types are M8 through M512.
type Mem = operand.Mem

type (
	M8   = operand.M8
	M16  = operand.M16
	M32  = operand.M32
	M64  = operand.M64
	M128 = operand.M128
	M256 = operand.M256
	M512 = operand.M512
)

// The memory constructors, one per width. The width names the access, so
// Mem64(RBX) is the operand in `mov rax, [rbx]` and Mem8(RBX) is the one in
// `mov al, [rbx]`.
func Mem8(base Reg64) M8     { return operand.Mem8(base) }
func Mem16(base Reg64) M16   { return operand.Mem16(base) }
func Mem32(base Reg64) M32   { return operand.Mem32(base) }
func Mem64(base Reg64) M64   { return operand.Mem64(base) }
func Mem128(base Reg64) M128 { return operand.Mem128(base) }
func Mem256(base Reg64) M256 { return operand.Mem256(base) }
func Mem512(base Reg64) M512 { return operand.Mem512(base) }

// Abs is an absolute reference with no base or index. The displacement is a
// disp32 sign-extended to 64 bits — this target has no 64-bit displacement
// outside the MOV moffs forms, so an address above 2GB goes through a
// register.
func Abs(disp int32) Mem { return operand.Abs(disp) }

// AbsSym is an absolute reference to a symbol, which becomes a fixup.
func AbsSym(t Target) Mem { return operand.AbsSym(t) }

// RIPRel is a %rip-relative reference to a label or symbol. This is how
// position-independent code reaches static data here: the displacement
// resolves against the end of the instruction, and the encoder knows where
// that is because it placed the field.
func RIPRel(t Target) Mem { return operand.RIPRel(t) }

// RIPRelDisp is a %rip-relative reference to a constant offset. Rare outside
// hand-written code and almost never what you want, since the offset is from
// the next instruction and moves when anything before it changes size.
func RIPRelDisp(disp int32) Mem { return operand.RIPRelDisp(disp) }

// Target is something a displacement can point at: a label defined in this
// unit, or a symbol resolved later.
type Target = operand.Target

// Label is a reference to a label defined somewhere in this unit. Whether it
// resolves without a relocation depends on where it lands — same section is
// folded at Serialize, another section needs a fixup, and on Flat the latter
// is refused.
type Label = operand.Label

// SymRef is a reference to a symbol, with the relocation kind that should
// record it. The symbol need not be defined in this unit.
type SymRef = operand.SymRef

// Ref builds a symbol reference. Pass RelocNone to let the encoder choose
// the kind from the form: a call site gets PLT32, a %rip-relative load gets
// PC32.
func Ref(name string, kind RelocKind) SymRef { return operand.Ref(name, kind) }