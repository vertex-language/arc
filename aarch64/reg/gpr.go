package reg

// X is a 64-bit general-purpose register. Number 31 is XZR.
type X uint8

// W is the 32-bit view of a general-purpose register. A write zero-extends
// into the corresponding X. Number 31 is WZR.
type W uint8

// Xsp is a 64-bit general-purpose register in a slot that reads number 31 as
// the stack pointer rather than the zero register.
type Xsp uint8

// Wsp is the 32-bit view of the above.
type Wsp uint8

const (
	X0 X = iota
	X1
	X2
	X3
	X4
	X5
	X6
	X7
	X8
	X9
	X10
	X11
	X12
	X13
	X14
	X15
	X16
	X17
	X18
	X19
	X20
	X21
	X22
	X23
	X24
	X25
	X26
	X27
	X28
	X29
	X30
	XZR
)

const (
	W0 W = iota
	W1
	W2
	W3
	W4
	W5
	W6
	W7
	W8
	W9
	W10
	W11
	W12
	W13
	W14
	W15
	W16
	W17
	W18
	W19
	W20
	W21
	W22
	W23
	W24
	W25
	W26
	W27
	W28
	W29
	W30
	WZR
)

// SP and WSP are the only two values of their types that have their own names.
// Every other Xsp is an X viewed through a slot that permits the stack pointer;
// see X.WithSP.
const (
	SP  Xsp = 31
	WSP Wsp = 31
)

// The AAPCS64 role names. The standard recommends that disassembly use the
// architectural names, so String never prints these, and Lookup does not accept
// them — a source file that wants to write "lr" can say so with .req, which is
// where a name that is not the architecture's belongs.
const (
	IP0 = X16 // first intra-procedure-call scratch register
	IP1 = X17 // second intra-procedure-call scratch register
	PR  = X18 // platform register, if the platform ABI claims it
	FP  = X29 // frame pointer
	LR  = X30 // link register
)

func (r X) Num() uint16     { return uint16(r) }
func (r X) Bits() uint16    { return 64 }
func (r X) Class() Class    { return ClassX }
func (r W) Num() uint16     { return uint16(r) }
func (r W) Bits() uint16    { return 32 }
func (r W) Class() Class    { return ClassW }
func (r Xsp) Num() uint16   { return uint16(r) }
func (r Xsp) Bits() uint16  { return 64 }
func (r Xsp) Class() Class  { return ClassXsp }
func (r Wsp) Num() uint16   { return uint16(r) }
func (r Wsp) Bits() uint16  { return 32 }
func (r Wsp) Class() Class  { return ClassWsp }

// Zero reports whether this is the zero register rather than a numbered one.
func (r X) Zero() bool { return r == XZR }
func (r W) Zero() bool { return r == WZR }

// WithSP converts a numbered register for use in a slot that reads 31 as the
// stack pointer. It fails for XZR, which is the one register that has no
// meaning in such a slot.
//
// This conversion exists because Go cannot give one value two types, and the
// architecture genuinely has two register-31s. The alternative — one type with
// a flag — would make Overlaps and Parent answer questions that have two
// different right answers, which is the reason this package exists separately
// from i386's and x86_64's at all.
func (r X) WithSP() (Xsp, bool) {
	if r == XZR {
		return 0, false
	}
	return Xsp(r), true
}

func (r W) WithSP() (Wsp, bool) {
	if r == WZR {
		return 0, false
	}
	return Wsp(r), true
}

// WithZR is the reverse: a numbered Xsp read as an ordinary register. SP has no
// such reading.
func (r Xsp) WithZR() (X, bool) {
	if r == SP {
		return 0, false
	}
	return X(r), true
}

func (r Wsp) WithZR() (W, bool) {
	if r == WSP {
		return 0, false
	}
	return W(r), true
}

// IsSP reports whether the value is the stack pointer rather than a numbered
// register.
func (r Xsp) IsSP() bool { return r == SP }
func (r Wsp) IsSP() bool { return r == WSP }