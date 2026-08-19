package isa

import "github.com/vertex-language/arc/aarch64/reg"

// AddrForm is which of the addressing forms a memory operand uses. Resolve
// needs it because LDR has a different form for each, and they are different
// encodings rather than different immediate ranges.
type AddrForm uint8

const (
	AddrNone      AddrForm = iota
	AddrBase               // [Xn]
	AddrOffset             // [Xn, #imm] — scaled unsigned
	AddrUnscaled           // [Xn, #imm] — unscaled signed, LDUR
	AddrRegOffset          // [Xn, Xm{, LSL #s}] / [Xn, Wm, SXTW #s]
	AddrPreIndex           // [Xn, #imm]!
	AddrPostIndex          // [Xn], #imm
	AddrLiteral            // a PC-relative literal
)

// Arg is one operand as Resolve sees it: enough to decide which form accepts
// it, and nothing more. The concrete operand value stays with the caller;
// encode/ is what lowers it.
type Arg struct {
	Class Class

	// Num is the register number, for the classes that name a register. It
	// matters because register 31 is two registers and the class alone does
	// not say which one an argument is.
	Num uint16

	// Imm is an immediate's value, for slots whose acceptance depends on it.
	Imm int64

	// Arr and Elem describe a vector operand.
	Arr  reg.Arrangement
	Elem reg.Elem

	// Addr is the addressing form of a memory operand.
	Addr AddrForm

	// AccessBits is the width a memory operand is accessed at, when the caller
	// stated it (Mem64 rather than Mem). Zero means the form decides.
	AccessBits uint16
}

// ArgOf builds an Arg from a register.
func ArgOf(r reg.Reg) Arg {
	a := Arg{Num: r.Num()}
	switch v := r.(type) {
	case reg.X:
		a.Class = ClassX
	case reg.W:
		a.Class = ClassW
	case reg.Xsp:
		a.Class = ClassXsp
	case reg.Wsp:
		a.Class = ClassWsp
	case reg.V:
		a.Class = ClassV
	case reg.Q:
		a.Class = ClassQ
	case reg.D:
		a.Class = ClassD
	case reg.S:
		a.Class = ClassS
	case reg.H:
		a.Class = ClassH
	case reg.B:
		a.Class = ClassB
	case reg.Vec:
		a.Class, a.Arr = ClassVArr, v.A
	case reg.VLane:
		a.Class, a.Elem = ClassVLane, v.E
	case reg.Z:
		a.Class = ClassZ
	case reg.P:
		a.Class = ClassP
	case reg.Sys:
		a.Class = ClassSys
	}
	return a
}

// ImmArg builds an immediate argument.
func ImmArg(v int64) Arg { return Arg{Class: ClassImm, Imm: v} }

// MemArg builds a memory argument.
func MemArg(form AddrForm, accessBits uint16) Arg {
	return Arg{Class: memClass(accessBits), Addr: form, AccessBits: accessBits}
}

// LabelArg builds a branch or address target.
func LabelArg() Arg { return Arg{Class: ClassLabel} }

// Match reports whether a slot's class accepts an argument.
//
// Two asymmetries are deliberate.
//
// An Xsp slot accepts a numbered X: "add x0, x1, #1" is legal and the parser
// has no reason to have produced an Xsp for x1. It does not accept XZR, which
// is a different register that happens to share an encoding.
//
// An X slot does not accept SP. Register 31 in such a slot is the zero
// register, and a caller who wrote SP meant the other one.
func (c Class) Match(a Arg) bool {
	switch c {
	case ClassXsp:
		return a.Class == ClassXsp ||
			(a.Class == ClassX && a.Num != 31)
	case ClassWsp:
		return a.Class == ClassWsp ||
			(a.Class == ClassW && a.Num != 31)
	case ClassX, ClassW:
		return a.Class == c
	case ClassPg:
		return a.Class == ClassP && a.Num <= 7
	case ClassMem8, ClassMem16, ClassMem32, ClassMem64, ClassMem128:
		// A caller that stated a width must match it; one that did not takes
		// the form's.
		if a.AccessBits != 0 && a.AccessBits != c.AccessBits() {
			return false
		}
		return a.Class.Mem()
	case ClassLabel:
		// A branch target may arrive as a label or as a bare immediate
		// displacement.
		return a.Class == ClassLabel || a.Class == ClassImm
	}
	return a.Class == c
}