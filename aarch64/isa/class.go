// Package isa is the AArch64 instruction table: every declared encoding of
// every mnemonic, its operand classes, its base word, and the feature that
// gates it.
//
// Resolve matches operand classes to exactly one form. Because the instruction
// width is fixed there is no shortest-form search and no tie to break: a
// mnemonic whose forms could both accept the same operands is a table-time
// error, caught when the table is built rather than answered arbitrarily at
// encode time. That is the whole difference between this package and x86_64's.
package isa

import "github.com/vertex-language/arc/aarch64/reg"

// Class is what one operand slot accepts: a register file, a width, and — for
// memory — the access size, in one value.
//
// The access size is here rather than on the form because it is what the
// relocation depends on. Under ELF the low-12 half of an address reference is
// ADD_ABS_LO12_NC in an add and LDST8/16/32/64/128_ABS_LO12_NC in a load,
// chosen by the width the immediate is scaled by. The caller does not know that
// width; the class does, and the encoder copies it onto the Fixup.
type Class uint8

const (
	ClassNone Class = iota

	// General purpose. The Xsp and Wsp classes read register 31 as the stack
	// pointer; the X and W classes read it as the zero register. No form
	// accepts both readings, which is why they are separate classes rather
	// than one class with a flag.
	ClassX
	ClassW
	ClassXsp
	ClassWsp

	// SIMD and floating point, scalar views.
	ClassV // 128-bit, no arrangement stated
	ClassQ
	ClassD
	ClassS
	ClassH
	ClassB

	// SIMD vector views.
	ClassVArr  // v0.16b — register with an arrangement
	ClassVLane // v2.s[1] — one element

	// Scalable vector and predicate.
	ClassZ
	ClassP  // any predicate, P0-P15
	ClassPg // governing predicate, P0-P7 only

	// Memory operands, by access width. A slot takes one of these or takes no
	// memory at all; there is no "register or memory" class on this
	// architecture, because no A64 form has one.
	ClassMem8
	ClassMem16
	ClassMem32
	ClassMem64
	ClassMem128

	// Everything that is not a register or an address.
	ClassImm     // an immediate; the field's own predicate decides the range
	ClassLabel   // a branch or address target
	ClassCond    // EQ, NE, …
	ClassShift   // LSL/LSR/ASR/ROR decorating a register operand
	ClassExtend  // UXTB…SXTX decorating a register operand
	ClassSys     // a system register
	ClassPrfOp   // PLDL1KEEP and the rest
	ClassBarrier // SY, ISH, LD, ST …

	classCount
)

var classInfo = [classCount]struct {
	name string
	file reg.File
	bits uint16
	mem  bool
}{
	ClassX:   {"Xn", reg.FileGPR, 64, false},
	ClassW:   {"Wn", reg.FileGPR, 32, false},
	ClassXsp: {"Xn|SP", reg.FileGPR, 64, false},
	ClassWsp: {"Wn|WSP", reg.FileGPR, 32, false},

	ClassV: {"Vn", reg.FileVec, 128, false},
	ClassQ: {"Qn", reg.FileVec, 128, false},
	ClassD: {"Dn", reg.FileVec, 64, false},
	ClassS: {"Sn", reg.FileVec, 32, false},
	ClassH: {"Hn", reg.FileVec, 16, false},
	ClassB: {"Bn", reg.FileVec, 8, false},

	ClassVArr:  {"Vn.T", reg.FileVec, 0, false},
	ClassVLane: {"Vn.T[i]", reg.FileVec, 0, false},

	ClassZ:  {"Zn", reg.FileVec, 0, false},
	ClassP:  {"Pn", reg.FilePred, 0, false},
	ClassPg: {"Pg", reg.FilePred, 0, false},

	ClassMem8:   {"[addr]", reg.FileNone, 8, true},
	ClassMem16:  {"[addr]", reg.FileNone, 16, true},
	ClassMem32:  {"[addr]", reg.FileNone, 32, true},
	ClassMem64:  {"[addr]", reg.FileNone, 64, true},
	ClassMem128: {"[addr]", reg.FileNone, 128, true},

	ClassImm:     {"#imm", reg.FileNone, 0, false},
	ClassLabel:   {"label", reg.FileNone, 0, false},
	ClassCond:    {"cond", reg.FileNone, 0, false},
	ClassShift:   {"shift", reg.FileNone, 0, false},
	ClassExtend:  {"extend", reg.FileNone, 0, false},
	ClassSys:     {"sysreg", reg.FileNone, 64, false},
	ClassPrfOp:   {"prfop", reg.FileNone, 0, false},
	ClassBarrier: {"option", reg.FileNone, 0, false},
}

func (c Class) String() string {
	if c >= classCount {
		return "?"
	}
	return classInfo[c].name
}

// File is the register bank this class draws from, or reg.FileNone.
func (c Class) File() reg.File {
	if c >= classCount {
		return reg.FileNone
	}
	return classInfo[c].file
}

// Bits is the width, or 0 where the width is scalable or stated by an
// arrangement rather than the class.
func (c Class) Bits() uint16 {
	if c >= classCount {
		return 0
	}
	return classInfo[c].bits
}

// Mem reports whether this class is an address rather than a value.
func (c Class) Mem() bool {
	if c >= classCount {
		return false
	}
	return classInfo[c].mem
}

// Reg reports whether the class names a register.
func (c Class) Reg() bool { return c.File() != reg.FileNone }

// AccessBits is the width of a memory access, for the class of a memory slot.
// It is what selects between the five LDST_ABS_LO12_NC relocations, and is 0
// for every class that is not an address.
func (c Class) AccessBits() uint16 {
	if !c.Mem() {
		return 0
	}
	return c.Bits()
}

// memClass maps an access width to its class.
func memClass(bits uint16) Class {
	switch bits {
	case 8:
		return ClassMem8
	case 16:
		return ClassMem16
	case 32:
		return ClassMem32
	case 64:
		return ClassMem64
	case 128:
		return ClassMem128
	}
	return ClassNone
}