// Package reg is the i386 register file.
//
// Every register here is a physical, architectural register of the Intel386
// architecture. There are no virtual registers and no relation to any other
// arch package: i386.EAX and x86_64.EAX are different registers that share a
// spelling.
//
// Three numberings appear in this package and they are not the same thing:
//
//   - Num is the encoding number: the 0-7 value carried by the ModRM reg or
//     r/m field, or added to an opcode byte by a +rb/+rw/+rd form. It is the
//     ordinal of the constant, so EAX is 0 and EDI is 7.
//   - DWARF is the number assigned by Intel386 psABI v1.1, Table 2.14. It
//     exists only for the registers that table names; byte and word registers
//     have none.
//   - Name is the psABI's own spelling, bare. Sigils and alternate spellings
//     are a dialect's business and live in i386/text.
package reg

// Class is the register file a register belongs to. Classes are separate
// encoding namespaces: Xmm(0) and Cr(0) are both encoding number 0.
type Class uint8

const (
	ClassGP Class = iota
	ClassSeg
	ClassX87
	ClassMMX
	ClassVec
	ClassMask
	ClassControl
	ClassDebug
)

var classNames = [...]string{"gp", "seg", "x87", "mmx", "vec", "mask", "cr", "dr"}

func (c Class) String() string {
	if int(c) < len(classNames) {
		return classNames[c]
	}
	return "?"
}

// Save is whether the psABI requires a function to preserve a register across
// a call. Intel386 psABI v1.1, Table 2.3.
type Save uint8

const (
	SaveNone Save = iota
	CallerSaved
	CalleeSaved
)

var saveNames = [...]string{"", "caller-saved", "callee-saved"}

func (s Save) String() string {
	if int(s) < len(saveNames) {
		return saveNames[s]
	}
	return "?"
}

// noDWARF is the sentinel for a register psABI Table 2.14 assigns no number.
const noDWARF = -1

// spec is the position of a register within its architectural root, as a
// half-open bit range. It is what Overlaps is computed from and nothing else.
type spec struct {
	name   string
	class  Class
	num    uint8
	root   uint8
	lo, hi uint16
}

// Reg is any i386 register. The interface is closed: spec is unexported, so
// only this package can declare a register.
type Reg interface {
	Name() string
	Num() uint8
	Bits() int
	Class() Class
	Save() Save
	Role() string
	DWARF() (int, bool)
	Overlaps(Reg) bool

	spec() spec
}

// overlaps reports whether two registers share any bit of one architectural
// register. Two registers of different classes never overlap.
//
// MMX and x87 are deliberately separate classes even though MMn aliases the
// mantissa of a physical x87 register: STn is relative to the x87 stack TOP,
// so the relation is not knowable without runtime state. A statically wrong
// answer would be worse than no answer, so this package does not model it.
// The same applies to the ClassVec relation between XMM and its wider views,
// which is modelled, because there the containment is static.
func overlaps(a, b Reg) bool {
	x, y := a.spec(), b.spec()
	return x.class == y.class && x.root == y.root && x.lo < y.hi && y.lo < x.hi
}