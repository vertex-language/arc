// x86_64/isa/class.go
package isa

// Class is the operand class of one slot: the set of operands that slot
// accepts, spelled the way the SDM's instruction column spells it.
//
// A class is not a width and not a register file — it is both at once, plus
// whether memory is allowed in that position. RM64 and R64 differ only in
// that, and the difference is the whole reason MOV has four ways to move a
// register.
type Class uint8

const (
	ClassNone Class = iota

	// Fixed operands: written by the user but encoded nowhere, because the
	// opcode already names them.
	AL
	AX
	EAX
	RAX
	CL
	DX
	One  // the literal 1 in `shl r/m64, 1`
	XMM0 // the implicit third operand of the SSE4.1 blend forms

	// General-purpose registers, no memory.
	R8
	R16
	R32
	R64

	// General-purpose register or memory.
	RM8
	RM16
	RM32
	RM64

	// Memory only, width fixed by the class.
	M8
	M16
	M32
	M64
	M128
	M256
	M512
	MAny // memory of no particular width: lea, prefetch, lgdt

	// Absolute moffs operands, which take no ModRM at all.
	Moffs8
	Moffs32
	Moffs64

	// Immediates. The width here is the field width, not the value's.
	Imm8
	Imm16
	Imm32
	Imm64

	// Branch displacements.
	Rel8
	Rel32

	// Vector registers, with and without memory.
	Mm
	MmM64
	Xmm
	XmmM32
	XmmM64
	XmmM128
	Ymm
	YmmM256
	Zmm
	ZmmM512

	// Mask, tile, and the register files nothing else reaches.
	K
	KM64
	Tmm
	St
	St0
	Sreg
	Cr
	Dr

	numClasses
)

var classNames = [numClasses]string{
	AL: "AL", AX: "AX", EAX: "EAX", RAX: "RAX", CL: "CL", DX: "DX",
	One: "1", XMM0: "<XMM0>",

	R8: "r8", R16: "r16", R32: "r32", R64: "r64",
	RM8: "r/m8", RM16: "r/m16", RM32: "r/m32", RM64: "r/m64",

	M8: "m8", M16: "m16", M32: "m32", M64: "m64",
	M128: "m128", M256: "m256", M512: "m512", MAny: "m",

	Moffs8: "moffs8", Moffs32: "moffs32", Moffs64: "moffs64",

	Imm8: "imm8", Imm16: "imm16", Imm32: "imm32", Imm64: "imm64",
	Rel8: "rel8", Rel32: "rel32",

	Mm: "mm", MmM64: "mm/m64",
	Xmm: "xmm", XmmM32: "xmm/m32", XmmM64: "xmm/m64", XmmM128: "xmm/m128",
	Ymm: "ymm", YmmM256: "ymm/m256",
	Zmm: "zmm", ZmmM512: "zmm/m512",

	K: "k", KM64: "k/m64", Tmm: "tmm",
	St: "ST(i)", St0: "ST(0)", Sreg: "Sreg", Cr: "CR", Dr: "DR",
}

// goNames is the fragment this class contributes to a helper's Go name.
// MovR64Imm64 is R64 then Imm64; XorRM64R64 is RM64 then R64.
var classGoNames = [numClasses]string{
	AL: "AL", AX: "AX", EAX: "EAX", RAX: "RAX", CL: "CL", DX: "DX",
	One: "One", XMM0: "",

	R8: "R8", R16: "R16", R32: "R32", R64: "R64",
	RM8: "RM8", RM16: "RM16", RM32: "RM32", RM64: "RM64",

	M8: "M8", M16: "M16", M32: "M32", M64: "M64",
	M128: "M128", M256: "M256", M512: "M512", MAny: "M",

	Moffs8: "Moffs8", Moffs32: "Moffs32", Moffs64: "Moffs64",

	Imm8: "Imm8", Imm16: "Imm16", Imm32: "Imm32", Imm64: "Imm64",
	Rel8: "Rel8", Rel32: "Rel32",

	Mm: "Mm", MmM64: "MmM64",
	Xmm: "Xmm", XmmM32: "XmmM32", XmmM64: "XmmM64", XmmM128: "XmmM128",
	Ymm: "Ymm", YmmM256: "YmmM256",
	Zmm: "Zmm", ZmmM512: "ZmmM512",

	K: "K", KM64: "KM64", Tmm: "Tmm",
	St: "St", St0: "St0", Sreg: "Sreg", Cr: "Cr", Dr: "Dr",
}

func (c Class) String() string {
	if int(c) >= len(classNames) {
		return "class(?)"
	}
	return classNames[c]
}

// GoName is what this class contributes to a generated helper's name. It is
// empty for a class that never appears in one — an implicit operand, or the
// fixed XMM0 the blend forms read without naming.
func (c Class) GoName() string {
	if int(c) >= len(classGoNames) {
		return ""
	}
	return classGoNames[c]
}

// Bits is the width of the operand in bits, or zero where the class does not
// fix one (MAny, Sreg, St).
func (c Class) Bits() int {
	switch c {
	case AL, CL, R8, RM8, M8, Imm8, Rel8, Moffs8:
		return 8
	case AX, DX, R16, RM16, M16, Imm16, Sreg:
		return 16
	case EAX, R32, RM32, M32, Imm32, Rel32, Moffs32, XmmM32:
		return 32
	case RAX, R64, RM64, M64, Imm64, Moffs64, Mm, MmM64, XmmM64, K, KM64, Cr, Dr:
		return 64
	case Xmm, XmmM128, M128, XMM0:
		return 128
	case Ymm, YmmM256, M256:
		return 256
	case Zmm, ZmmM512, M512:
		return 512
	case Tmm:
		return 8192
	}
	return 0
}

// AcceptsMem reports whether a memory reference may fill this slot. It is the
// question ModRM.mod answers, and the only difference between R64 and RM64.
func (c Class) AcceptsMem() bool {
	switch c {
	case RM8, RM16, RM32, RM64,
		M8, M16, M32, M64, M128, M256, M512, MAny,
		MmM64, XmmM32, XmmM64, XmmM128, YmmM256, ZmmM512, KM64:
		return true
	}
	return false
}

// MemOnly reports whether the slot refuses a register. LEA's source is the
// clearest case: there is no register form of it at all.
func (c Class) MemOnly() bool {
	switch c {
	case M8, M16, M32, M64, M128, M256, M512, MAny,
		Moffs8, Moffs32, Moffs64:
		return true
	}
	return false
}

// IsImm reports whether the slot is filled from the immediate field.
func (c Class) IsImm() bool {
	switch c {
	case Imm8, Imm16, Imm32, Imm64:
		return true
	}
	return false
}

// IsRel reports whether the slot is a branch displacement.
func (c Class) IsRel() bool { return c == Rel8 || c == Rel32 }

// IsFixed reports whether the opcode already names the operand, so it
// occupies no encoding field. AL in `add al, imm8` is fixed but written; the
// RDX:RAX pair `div` clobbers is fixed and not written, which is Slot.Implicit
// and a different question.
func (c Class) IsFixed() bool {
	switch c {
	case AL, AX, EAX, RAX, CL, DX, One, XMM0, St0:
		return true
	}
	return false
}