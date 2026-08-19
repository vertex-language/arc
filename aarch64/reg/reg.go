// Package reg enumerates the AArch64 architectural registers.
//
// Every value here is a physical register. There is no virtual register type
// and no allocator; a register is a number, a width, and a rule about which
// slots will accept it.
//
// The awkward parts of this file are the architecture's, not this package's:
// register number 31 names two different registers depending on the form that
// reads it, the SIMD file is one bank seen at six widths, and SVE's Z registers
// extend the same bank again without being the same registers for any purpose
// that matters here.
package reg

// File is a bank of registers. Two registers can only overlap if they are in
// the same file.
type File uint8

const (
	FileNone File = iota
	FileGPR       // X, W, Xsp, Wsp
	FileVec       // V, Q, D, S, H, B, Z
	FilePred      // P, FFR
	FileSys       // Sys
)

// Class is a register file seen at one width, which is what a form's slot
// actually names. ClassX and ClassXsp are the same thirty-one registers plus
// two different readings of number 31.
type Class uint8

const (
	ClassNone Class = iota
	ClassX          // 64-bit general purpose; 31 reads as XZR
	ClassW          // 32-bit view of the above; 31 reads as WZR
	ClassXsp        // 64-bit general purpose; 31 reads as SP
	ClassWsp        // 32-bit view of the above; 31 reads as WSP
	ClassV          // 128-bit SIMD register, no arrangement
	ClassVArr       // 128- or 64-bit SIMD register with an arrangement: v0.4s
	ClassVLane      // one element of a SIMD register: v2.s[1]
	ClassQ          // 128-bit scalar view
	ClassD          // 64-bit scalar view
	ClassS          // 32-bit scalar view
	ClassH          // 16-bit scalar view
	ClassB          // 8-bit scalar view
	ClassZ          // scalable vector
	ClassP          // scalable predicate
	ClassFFR        // the first fault register
	ClassSys        // a system register named by {op0,op1,CRn,CRm,op2}
)

// File reports which bank a class draws from.
func (c Class) File() File {
	switch c {
	case ClassX, ClassW, ClassXsp, ClassWsp:
		return FileGPR
	case ClassV, ClassVArr, ClassVLane, ClassQ, ClassD, ClassS, ClassH, ClassB, ClassZ:
		return FileVec
	case ClassP, ClassFFR:
		return FilePred
	case ClassSys:
		return FileSys
	}
	return FileNone
}

// Reg is what every register in this package satisfies.
//
// Save and DWARF are deliberately absent: Save needs a procedure-call variant
// the register cannot know (see save.go) and DWARF has to be able to answer
// "there is no number" (see dwarf.go). Both are package functions over one
// table rather than a method spread across nine types.
type Reg interface {
	// Num is the architectural encoding: 0-31 for the GPR, SIMD and Z files,
	// 0-15 for P, and the packed 16-bit {op0,op1,CRn,CRm,op2} id for Sys.
	Num() uint16

	// Bits is the width in bits, or 0 for the scalable registers (Z, P, FFR)
	// whose width is a runtime property of the implementation.
	Bits() uint16

	Class() Class
	String() string
}

// Scalable reports whether a register's width is only known at run time.
func Scalable(r Reg) bool { return r.Bits() == 0 }

// Overlaps reports whether writing one register can be observed by reading the
// other.
//
// Two decisions worth stating, because both have a defensible opposite:
//
//   - XZR and WZR overlap nothing, including each other. They are not storage;
//     a write is discarded and a read is zero, so no write is ever observable.
//   - SP and XZR do not overlap despite encoding identically. The encoding is
//     shared, the register is not, and a form reads exactly one of them.
func Overlaps(a, b Reg) bool {
	ca, cb := a.Class(), b.Class()
	if ca.File() != cb.File() || ca.File() == FileNone {
		return false
	}
	if ca == ClassFFR || cb == ClassFFR {
		return ca == cb
	}
	if ca.File() == FileGPR {
		if isZeroClass(ca) && a.Num() == 31 {
			return false
		}
		if isZeroClass(cb) && b.Num() == 31 {
			return false
		}
	}
	return a.Num() == b.Num()
}

func isZeroClass(c Class) bool { return c == ClassX || c == ClassW }

// Parent reports the wider register a narrow view writes into, if any.
//
// V does not report Z as its parent even though Z0 extends V0. The two are
// separate registers for every purpose this package serves — separate DWARF
// numbers, separate preservation rules, separate slots — and the aliasing they
// do have is exactly what Overlaps reports. Deriving one from the other would
// make DWARF and Save wrong in the same motion.
func Parent(r Reg) (Reg, bool) {
	switch v := r.(type) {
	case W:
		return X(v), true
	case Wsp:
		return Xsp(v), true
	case Q:
		return V(v), true
	case D:
		return V(v), true
	case S:
		return V(v), true
	case H:
		return V(v), true
	case B:
		return V(v), true
	}
	return nil, false
}