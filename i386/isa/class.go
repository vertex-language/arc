// Package isa is the i386 instruction table: every form, its operand classes,
// its opcode bytes, and the level or extension that gates it.
//
// This is the one table. The generated helpers in the arch root are generated
// from it, Emit resolves against it, arc isa prints it, and arc enc --all
// enumerates it. That is the whole of "arc isa cannot describe an instruction
// arc build won't encode": the filter and the gate are the same values from
// the same package.
//
// isa does not encode. It says which form a mnemonic and a list of operands
// resolve to, and what fields that form has; turning a form plus operand
// values into bytes is encode/. Nothing here holds an instruction.
package isa

import (
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// Class is an operand class: the kind of operand a form's slot accepts.
//
// The names are the SDM's, as they appear in an instruction's Operand/
// Instruction column — r/m32, imm8, rel32, CL. A helper's name is built from
// these, so they are also the vocabulary of the generated API.
type Class uint8

const (
	ClassNone Class = iota

	// Registers.
	R8
	R16
	R32
	Sreg
	St
	Mm
	Xmm
	Ymm
	Zmm
	Cr
	Dr

	// Register or memory, by access width.
	RM8
	RM16
	RM32
	RM64
	RM128

	// Memory with no access width of its own: LEA's operand, and the far
	// pointer loads. The address is the operand.
	M

	// Immediates. Imm8S is a sign-extended byte — the imm8 of ADD r/m32,
	// imm8, which is a different form from ADD r/m32, imm32 and four bytes
	// shorter. Keeping them apart is the whole reason the helper names are
	// what they are.
	Imm8
	Imm8S
	Imm16
	Imm32

	// Branch displacements.
	Rel8
	Rel32

	// Fixed operands. These appear in the instruction's syntax but occupy no
	// encoding field, which is what makes ADD EAX, imm32 a distinct six-byte
	// form from ADD r/m32, imm32.
	AL
	AX
	EAX
	CL
	DX
	One // the literal 1 of SHL r/m32, 1
)

var classNames = map[Class]string{
	R8: "r8", R16: "r16", R32: "r32",
	Sreg: "Sreg", St: "ST(i)", Mm: "mm", Xmm: "xmm", Ymm: "ymm", Zmm: "zmm",
	Cr: "CR", Dr: "DR",
	RM8: "r/m8", RM16: "r/m16", RM32: "r/m32", RM64: "r/m64", RM128: "r/m128",
	M:     "m",
	Imm8:  "imm8", Imm8S: "imm8", Imm16: "imm16", Imm32: "imm32",
	Rel8: "rel8", Rel32: "rel32",
	AL: "AL", AX: "AX", EAX: "EAX", CL: "CL", DX: "DX", One: "1",
}

func (c Class) String() string {
	if n, ok := classNames[c]; ok {
		return n
	}
	return "?"
}

// helperName is the class as it appears in a generated helper's name:
// MovR32Imm32, AddRM32Imm8, AddEAXImm32. Imm8S and Imm8 differ as forms but
// share a spelling here, so the sign-extended form carries the S.
var helperNames = map[Class]string{
	R8: "R8", R16: "R16", R32: "R32",
	Sreg: "Sreg", St: "St", Mm: "Mm", Xmm: "Xmm", Ymm: "Ymm", Zmm: "Zmm",
	Cr: "Cr", Dr: "Dr",
	RM8: "RM8", RM16: "RM16", RM32: "RM32", RM64: "RM64", RM128: "RM128",
	M:     "M",
	Imm8:  "Imm8", Imm8S: "Imm8S", Imm16: "Imm16", Imm32: "Imm32",
	Rel8: "Rel8", Rel32: "Rel32",
	AL: "AL", AX: "AX", EAX: "EAX", CL: "CL", DX: "DX", One: "One",
}

// Matches reports whether o can be this class.
//
// For the immediate classes this is a question about the value, not only the
// type: Imm8S matches an Imm that fits in a signed byte. That is what lets
// Emit pick the four-byte ADD over the six-byte one, and it is why the typed
// helpers exist for callers who need the choice made differently.
func (c Class) Matches(o operand.Operand) bool {
	switch c {
	case R8:
		_, ok := o.(reg.R8)
		return ok
	case R16:
		_, ok := o.(reg.R16)
		return ok
	case R32:
		_, ok := o.(reg.R32)
		return ok
	case Sreg:
		_, ok := o.(reg.Sreg)
		return ok
	case St:
		_, ok := o.(reg.St)
		return ok
	case Mm:
		_, ok := o.(reg.Mm)
		return ok
	case Xmm:
		_, ok := o.(reg.Xmm)
		return ok
	case Cr:
		_, ok := o.(reg.Cr)
		return ok
	case Dr:
		_, ok := o.(reg.Dr)
		return ok

	case RM8:
		_, ok := o.(operand.RM8)
		return ok
	case RM16:
		_, ok := o.(operand.RM16)
		return ok
	case RM32:
		_, ok := o.(operand.RM32)
		return ok
	case RM64:
		_, ok := o.(operand.RM64)
		return ok
	case RM128:
		_, ok := o.(operand.RM128)
		return ok

	case M:
		_, ok := o.(operand.Memory)
		return ok

	case Imm8:
		v, ok := o.(operand.Imm)
		return ok && v.Int64() >= -128 && v.Int64() <= 255
	case Imm8S:
		v, ok := o.(operand.Imm)
		return ok && v.Int64() >= -128 && v.Int64() <= 127
	case Imm16:
		v, ok := o.(operand.Imm)
		return ok && v.Int64() >= -32768 && v.Int64() <= 65535
	case Imm32:
		v, ok := o.(operand.Imm)
		return ok && v.Int64() >= -2147483648 && v.Int64() <= 4294967295

	case Rel8, Rel32:
		switch o.(type) {
		case operand.Label, operand.SymRef:
			return true
		}
		return false

	// A fixed operand matches only that exact register. AL is not r8 here:
	// a form that names AL has no field to put another register in.
	case AL:
		return o == operand.Operand(reg.AL)
	case AX:
		return o == operand.Operand(reg.AX)
	case EAX:
		return o == operand.Operand(reg.EAX)
	case CL:
		return o == operand.Operand(reg.CL)
	case DX:
		return o == operand.Operand(reg.DX)
	case One:
		v, ok := o.(operand.Imm)
		return ok && v.Int64() == 1
	}
	return false
}

// Slot is where a form puts an operand in the encoding.
type Slot uint8

const (
	SlotNone   Slot = iota
	SlotReg         // ModRM.reg
	SlotRM          // ModRM.rm, with SIB and displacement as needed
	SlotOpcode      // added to the last opcode byte: +rb, +rw, +rd
	SlotImm         // immediate bytes following
	SlotRel         // a displacement relative to the end of the instruction
	SlotFixed       // named in the syntax, encoded nowhere
)

// Op is one operand of a form: what it accepts and where it goes.
type Op struct {
	Class Class
	Slot  Slot
}