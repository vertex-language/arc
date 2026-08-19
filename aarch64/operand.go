package aarch64

import (
	"fmt"

	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
)

// Re-export of reg/ and operand/, so a caller never needs the second import.
//
// These are type and constant aliases, not parallel definitions: aarch64.X0 and
// reg.X0 are the same value of the same type, and a helper taking a reg.X
// accepts either spelling.
//
// The exceptions are the memory constructors below, which are generic and
// therefore wrappers rather than aliases — Go has no alias form for a generic
// function. They are one-line forwards with no logic, which is the closest a
// wrapper gets to being an alias, but they are not the same function value and
// a caller comparing them would find that out.

// Operand is what an encode call accepts in an operand position.
//
// It is declared here rather than in either subpackage, because stating it in
// operand/ would mean operand/ importing reg/'s implementers back, and
// operand/ already imports reg/.
//
// It is deliberately loose — any Stringer satisfies it — which turns the common
// mistakes (an int, a string, a *Section) into compile errors at the call site
// rather than an OperandError at encode time. It buys nothing toward
// exhaustiveness, and encode/'s type switch is still what actually decides.
type Operand interface{ fmt.Stringer }

// Register types.
type (
	// X is a 64-bit general-purpose register; number 31 is XZR.
	X = reg.X
	// W is the 32-bit view; a write zero-extends into the corresponding X.
	W = reg.W
	// Xsp reads register 31 as the stack pointer rather than the zero register.
	Xsp = reg.Xsp
	// Wsp is the 32-bit view of the above.
	Wsp = reg.Wsp

	// V is a 128-bit SIMD register with no arrangement; Q, D, S, H and B are
	// the scalar views of the same bank.
	V = reg.V
	Q = reg.Q
	D = reg.D
	S = reg.S
	H = reg.H
	B = reg.B

	// Vec is a SIMD register with an arrangement, VLane one of its elements.
	Vec   = reg.Vec
	VLane = reg.VLane

	// Z is a scalable vector register, P a scalable predicate.
	Z   = reg.Z
	P   = reg.P
	Ffr = reg.Ffr

	// Sys is a system register named by {op0, op1, CRn, CRm, op2}.
	Sys = reg.Sys

	// Reg is what every register satisfies.
	Reg = reg.Reg

	// Arrangement is an element width with a count: the 4s of v0.4s.
	Arrangement = reg.Arrangement
	// Elem is an element width inside a SIMD register.
	Elem = reg.Elem
)

// The general-purpose registers.
const (
	X0, X1, X2, X3, X4, X5, X6, X7 = reg.X0, reg.X1, reg.X2, reg.X3, reg.X4, reg.X5, reg.X6, reg.X7
	X8, X9, X10, X11, X12, X13, X14, X15 = reg.X8, reg.X9, reg.X10, reg.X11, reg.X12, reg.X13, reg.X14, reg.X15
	X16, X17, X18, X19, X20, X21, X22, X23 = reg.X16, reg.X17, reg.X18, reg.X19, reg.X20, reg.X21, reg.X22, reg.X23
	X24, X25, X26, X27, X28, X29, X30 = reg.X24, reg.X25, reg.X26, reg.X27, reg.X28, reg.X29, reg.X30

	W0, W1, W2, W3, W4, W5, W6, W7 = reg.W0, reg.W1, reg.W2, reg.W3, reg.W4, reg.W5, reg.W6, reg.W7
	W8, W9, W10, W11, W12, W13, W14, W15 = reg.W8, reg.W9, reg.W10, reg.W11, reg.W12, reg.W13, reg.W14, reg.W15
	W16, W17, W18, W19, W20, W21, W22, W23 = reg.W16, reg.W17, reg.W18, reg.W19, reg.W20, reg.W21, reg.W22, reg.W23
	W24, W25, W26, W27, W28, W29, W30 = reg.W24, reg.W25, reg.W26, reg.W27, reg.W28, reg.W29, reg.W30
)

// Register 31, which is two registers.
//
// XZR and SP are distinct values with distinct types even though both encode
// as 31. A form that reads register 31 as the stack pointer will not accept
// XZR, and the mismatch is a compile error at a typed call and a RegisterError
// at Emit. Rounding them into one value would make Overlaps and Parent answer
// questions that have two different right answers.
const (
	XZR = reg.XZR
	WZR = reg.WZR
	SP  = reg.SP
	WSP = reg.WSP
)

// The AAPCS64 role names.
//
// The standard recommends that disassembly use the architectural names, so
// nothing prints these. They are here because a caller writing Go finds LR
// clearer than X30, and the printed form is unaffected either way.
const (
	IP0 = reg.IP0
	IP1 = reg.IP1
	PR  = reg.PR
	FP_ = reg.FP // FP is taken by the feature; see the note below
	LR  = reg.LR
)

// The SIMD and floating-point registers.
const (
	V0, V1, V2, V3, V4, V5, V6, V7 = reg.V0, reg.V1, reg.V2, reg.V3, reg.V4, reg.V5, reg.V6, reg.V7
	V8, V9, V10, V11, V12, V13, V14, V15 = reg.V8, reg.V9, reg.V10, reg.V11, reg.V12, reg.V13, reg.V14, reg.V15
	V16, V17, V18, V19, V20, V21, V22, V23 = reg.V16, reg.V17, reg.V18, reg.V19, reg.V20, reg.V21, reg.V22, reg.V23
	V24, V25, V26, V27, V28, V29, V30, V31 = reg.V24, reg.V25, reg.V26, reg.V27, reg.V28, reg.V29, reg.V30, reg.V31

	Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7 = reg.Q0, reg.Q1, reg.Q2, reg.Q3, reg.Q4, reg.Q5, reg.Q6, reg.Q7
	D0, D1, D2, D3, D4, D5, D6, D7 = reg.D0, reg.D1, reg.D2, reg.D3, reg.D4, reg.D5, reg.D6, reg.D7
	S0, S1, S2, S3, S4, S5, S6, S7 = reg.S0, reg.S1, reg.S2, reg.S3, reg.S4, reg.S5, reg.S6, reg.S7
	H0, H1, H2, H3, H4, H5, H6, H7 = reg.H0, reg.H1, reg.H2, reg.H3, reg.H4, reg.H5, reg.H6, reg.H7
	B0, B1, B2, B3, B4, B5, B6, B7 = reg.B0, reg.B1, reg.B2, reg.B3, reg.B4, reg.B5, reg.B6, reg.B7

	Z0, Z1, Z2, Z3, Z4, Z5, Z6, Z7 = reg.Z0, reg.Z1, reg.Z2, reg.Z3, reg.Z4, reg.Z5, reg.Z6, reg.Z7
	P0, P1, P2, P3, P4, P5, P6, P7 = reg.P0, reg.P1, reg.P2, reg.P3, reg.P4, reg.P5, reg.P6, reg.P7
)

// FFR is the first fault register. It is a var below rather than a const
// because its type is a struct: there is one, so it has one value.
var FFR = reg.FFR

// The arrangements. They are spelled V4S rather than 4S because a Go
// identifier cannot start with a digit, and V16B rather than B16 because B16 is
// already the scalar register b16.
const (
	V8B  = reg.V8B
	V16B = reg.V16B
	V4H  = reg.V4H
	V8H  = reg.V8H
	V2S  = reg.V2S
	V4S  = reg.V4S
	V1D  = reg.V1D
	V2D  = reg.V2D
)

// The element widths, for lane operands: v2.s[1].
const (
	ElemB = reg.ElemB
	ElemH = reg.ElemH
	ElemS = reg.ElemS
	ElemD = reg.ElemD
)

// The named system registers. This is a starting set, not the architecture's
// full table; anything absent is reachable through NewSys and through the
// generic S3_0_c0_c0_0 spelling.
const (
	NZCV      = reg.NZCV
	DAIF      = reg.DAIF
	CurrentEL = reg.CurrentEL
	SPSel     = reg.SPSel
	FPCR      = reg.FPCR
	FPSR      = reg.FPSR

	TPIDR_EL0   = reg.TPIDR_EL0
	TPIDRRO_EL0 = reg.TPIDRRO_EL0
	TPIDR_EL1   = reg.TPIDR_EL1

	MIDR_EL1   = reg.MIDR_EL1
	MPIDR_EL1  = reg.MPIDR_EL1
	CTR_EL0    = reg.CTR_EL0
	CNTVCT_EL0 = reg.CNTVCT_EL0

	SCTLR_EL1 = reg.SCTLR_EL1
	TTBR0_EL1 = reg.TTBR0_EL1
	TTBR1_EL1 = reg.TTBR1_EL1
	TCR_EL1   = reg.TCR_EL1
	ESR_EL1   = reg.ESR_EL1
	FAR_EL1   = reg.FAR_EL1
	VBAR_EL1  = reg.VBAR_EL1
	ELR_EL1   = reg.ELR_EL1
	SPSR_EL1  = reg.SPSR_EL1
)

// NewSys builds a system register from its five fields.
var NewSys = reg.NewSys

// LookupReg resolves an architectural register name. It is the fixed table
// only: .req adds names and .unreq removes them, so a source file's set of
// valid register names is a moving target that only the text layer tracks.
var LookupReg = reg.Lookup

// Save reports how a register is preserved across a call under a procedure
// call variant. The variant is a parameter because SVE preservation depends on
// the interface of the function the registers appear in, not on the registers.
var Save = reg.Save

// DWARF reports the aadwarf64 register number, and whether one exists. XZR has
// none: it is not a location, so there is nothing for an unwinder to say.
var DWARF = reg.DWARF

// Overlaps reports whether writing one register can be observed by reading the
// other.
var Overlaps = reg.Overlaps

// Parent reports the wider register a narrow view writes into, if any.
var Parent = reg.Parent

// The procedure call variants and preservation rules.
type (
	Variant = reg.Variant
	Saved   = reg.Saved
)

const (
	Base    = reg.Base
	SVEArgs = reg.SVEArgs

	Caller      = reg.Caller
	Callee      = reg.Callee
	CalleeLow64 = reg.CalleeLow64
)

// Operand types.
type (
	// Imm is an integer operand. aarch64.Imm(93) is a conversion, not a call.
	Imm = operand.Imm

	// Mem is an address operand.
	Mem = operand.Mem

	// Width is an operand or memory-access width in bits.
	Width = operand.Width

	// Shift decorates a register operand or ADD's twelve-bit immediate.
	Shift   = operand.Shift
	ShiftOp = operand.ShiftOp

	// Extend names the width an index or source register is read at.
	Extend   = operand.Extend
	ExtendOp = operand.ExtendOp

	// Cond is a condition code, Barrier a barrier option, PrfOp a prefetch
	// operand.
	Cond    = operand.Cond
	Barrier = operand.Barrier
	PrfOp   = operand.PrfOp

	// Label is a name defined in this object. It carries no relocation kind,
	// which is what makes a pc-relative reference to a local one foldable.
	Label = operand.Label

	// SymRef is a reference to a symbol, with an addend and optionally a
	// relocation kind the caller insists on.
	SymRef = operand.SymRef

	// Target is what an address operand points at before it is a number.
	Target = operand.Target

	// AddrRef is a target with the role naming which half of its address this
	// operand wants.
	AddrRef  = operand.AddrRef
	AddrRole = operand.AddrRole
)

// The addressing forms.
const (
	AddrBase      = operand.AddrBase
	AddrOffset    = operand.AddrOffset
	AddrRegOffset = operand.AddrRegOffset
	AddrPreIndex  = operand.AddrPreIndex
	AddrPostIndex = operand.AddrPostIndex
)

// The shift and extend kinds.
const (
	LSL = operand.LSL
	LSR = operand.LSR
	ASR = operand.ASR
	ROR = operand.ROR

	UXTB = operand.UXTB
	UXTH = operand.UXTH
	UXTW = operand.UXTW
	UXTX = operand.UXTX
	SXTB = operand.SXTB
	SXTH = operand.SXTH
	SXTW = operand.SXTW
	SXTX = operand.SXTX
)

// The condition codes. HS and LO are the unsigned spellings of CS and CC and
// the same encoding; nothing prints them, because the architecture's preferred
// disassembly is CS and CC.
const (
	EQ = operand.EQ
	NE = operand.NE
	CS = operand.CS
	CC = operand.CC
	MI = operand.MI
	PL = operand.PL
	VS = operand.VS
	VC = operand.VC
	HI = operand.HI
	LS = operand.LS
	GE = operand.GE
	LT = operand.LT
	GT = operand.GT
	LE = operand.LE
	AL = operand.AL
	NV = operand.NV

	HS = operand.HS
	LO = operand.LO
)

// The barrier options.
const (
	OSHLD = operand.OSHLD
	OSHST = operand.OSHST
	OSH   = operand.OSH
	NSHLD = operand.NSHLD
	NSHST = operand.NSHST
	NSH   = operand.NSH
	ISHLD = operand.ISHLD
	ISHST = operand.ISHST
	ISH   = operand.ISH
	SY    = operand.SY
)

// The memory-access widths.
const (
	Width8   = operand.Width8
	Width16  = operand.Width16
	Width32  = operand.Width32
	Width64  = operand.Width64
	Width128 = operand.Width128
)

// Mem builds an address with no stated access width, leaving it to the form.
//
// These five are wrappers rather than aliases because Go has no alias form for
// a generic function. The constraint is written inline rather than exported
// from operand/, so the union stays stated in one place per signature and a
// caller reads what is accepted without following an import.
func MemOf[T X | Xsp](base T) Mem   { return operand.MemOf(base) }
func Mem8[T X | Xsp](base T) Mem    { return operand.Mem8(base) }
func Mem16[T X | Xsp](base T) Mem   { return operand.Mem16(base) }
func Mem32[T X | Xsp](base T) Mem   { return operand.Mem32(base) }
func Mem64[T X | Xsp](base T) Mem   { return operand.Mem64(base) }
func Mem128[T X | Xsp](base T) Mem  { return operand.Mem128(base) }

// Ref builds a symbol reference, optionally with the relocation kind the
// caller insists on. Stating a kind is a request rather than a hint: it blocks
// folding, because resolving a named PLT reference to a direct branch answers
// a different question than the one asked.
var Ref = operand.Sym

// The four address roles.
//
// Materializing an address here usually takes two instructions and therefore
// two references — adrp for the page, add or a load for the offset within it —
// and each needs its own record. The role is the portable part: the spelling is
// per platform and the relocation kind is per format, and the caller states
// neither.
var (
	Page       = operand.Page
	PageOff    = operand.PageOff
	GotPage    = operand.GotPage
	GotPageOff = operand.GotPageOff
	Direct     = operand.Direct
)

// Shifted and Extended build the decorating operands.
var (
	Shifted  = operand.Shifted
	Extended = operand.Extended
)

// NoShift is what an omitted shift defaults to: LSL #0, which every form
// taking a shift encodes as zero.
var NoShift = operand.NoShift

// The expressibility predicates, exported because "can this constant even be
// written" is a question worth asking before emitting rather than after.
var (
	// BitmaskEncodable reports whether a constant is a logical immediate: a
	// rotated run of ones replicated to fill the register. A constant that is
	// not has to be materialized with movz and movk, which is two instructions
	// and therefore the caller's to write.
	BitmaskEncodable = operand.BitmaskEncodable

	FitsImm12       = operand.FitsImm12
	FitsImm16Shifted = operand.FitsImm16Shifted
	FitsImm9        = operand.FitsImm9
	FitsImm7Scaled  = operand.FitsImm7Scaled
)