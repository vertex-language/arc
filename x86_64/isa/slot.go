// x86_64/isa/slot.go
package isa

// Slot is one operand of one form: what it accepts, whether the instruction
// reads or writes it, and which encoding field carries it.
//
// This is the SDM's Instruction Operand Encoding table, one row per slot, in
// Intel operand order — destination first. text/gas reverses at its edges;
// nothing below text/ ever sees AT&T order.
type Slot struct {
	Class Class
	Role  Role
	Field Field

	// Implicit marks an operand the instruction touches but the syntax does
	// not name: the RDX:RAX pair DIV reads and writes, the RCX that LOOP
	// decrements. Resolve does not match an argument against it and GoName
	// does not spell it, but encode/ and a future register-pressure caller
	// need it declared.
	Implicit bool
}

// Role is how the instruction touches the operand. It is not used to pick an
// encoding; it exists because a caller reasoning about clobbers should not
// have to consult a manual, and because Explain prints it.
type Role uint8

const (
	Read Role = iota
	Write
	ReadWrite
)

func (r Role) String() string {
	switch r {
	case Write:
		return "w"
	case ReadWrite:
		return "r/w"
	}
	return "r"
}

// Field is the encoding field the operand lands in.
type Field uint8

const (
	// InNone is an operand with no field: fixed by the opcode, or implicit.
	InNone Field = iota
	InReg               // ModRM.reg, extended by REX.R / VEX.R / EVEX.R'R
	InRM                // ModRM.rm, extended by REX.B / EVEX.X, or a memory form
	InVVVV              // VEX.vvvv / EVEX.vvvv, the non-destructive source
	InOpcode            // the low three opcode bits, +rb/+rw/+rd/+ro
	InImm               // the immediate field, also where rel8/rel32 lands
	InIS4               // imm8[7:4], the fourth operand of VBLENDVPS and friends
	InMask              // EVEX.aaa, the writemask
	InMoffs             // the moffs field, which displaces ModRM entirely
)

func (f Field) String() string {
	switch f {
	case InReg:
		return "ModRM:reg"
	case InRM:
		return "ModRM:r/m"
	case InVVVV:
		return "vvvv"
	case InOpcode:
		return "opcode"
	case InImm:
		return "imm"
	case InIS4:
		return "imm8[7:4]"
	case InMask:
		return "EVEX.aaa"
	case InMoffs:
		return "moffs"
	}
	return "-"
}

// S declares an explicit slot.
func S(c Class, r Role, f Field) Slot { return Slot{Class: c, Role: r, Field: f} }

// Imp declares an implicit operand: touched, never written in source.
func Imp(c Class, r Role) Slot {
	return Slot{Class: c, Role: r, Field: InNone, Implicit: true}
}