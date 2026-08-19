package reg

// Z is a scalable vector register. Z0 extends V0: the low 128 bits of Z0 are
// V0. Its width is VG x 64 bits, where VG is a runtime property, so Bits
// reports 0.
type Z uint8

// P is a scalable predicate register, one bit per byte of a Z register.
type P uint8

// Ffr is the first fault register, which has the size and format of a predicate
// and records the fault status of a first-fault or non-fault vector load. There
// is one, so it has one value.
type Ffr struct{}

// FFR is the first fault register.
var FFR Ffr

const (
	Z0, Z1, Z2, Z3, Z4, Z5, Z6, Z7 Z = 0, 1, 2, 3, 4, 5, 6, 7
	Z8, Z9, Z10, Z11, Z12, Z13, Z14, Z15 Z = 8, 9, 10, 11, 12, 13, 14, 15
	Z16, Z17, Z18, Z19, Z20, Z21, Z22, Z23 Z = 16, 17, 18, 19, 20, 21, 22, 23
	Z24, Z25, Z26, Z27, Z28, Z29, Z30, Z31 Z = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	P0, P1, P2, P3, P4, P5, P6, P7 P = 0, 1, 2, 3, 4, 5, 6, 7
	P8, P9, P10, P11, P12, P13, P14, P15 P = 8, 9, 10, 11, 12, 13, 14, 15
)

func (r Z) Num() uint16    { return uint16(r) }
func (r Z) Bits() uint16   { return 0 }
func (r Z) Class() Class   { return ClassZ }
func (r P) Num() uint16    { return uint16(r) }
func (r P) Bits() uint16   { return 0 }
func (r P) Class() Class   { return ClassP }
func (r Ffr) Num() uint16  { return 0 }
func (r Ffr) Bits() uint16 { return 0 }
func (r Ffr) Class() Class { return ClassFFR }

// Governing reports whether this predicate can be used as the governing
// predicate of a predicated instruction. Most such forms encode the predicate
// in three bits and so reach P0-P7 only; the wider merging and zeroing forms
// that reach all sixteen are the exception rather than the rule.
//
// Which of the two a given form is remains isa/'s answer. This reports the
// property of the register that makes the question worth asking.
func (r P) Governing() bool { return r <= 7 }