package reg

// V is a 128-bit SIMD and floating-point register with no arrangement stated.
// Q, D, S, H and B are the scalar views of the same bank: q1, d1 and s1 all
// name the same entry, unlike AArch32, where narrow views packed several to a
// register.
type (
	V uint8
	Q uint8
	D uint8
	S uint8
	H uint8
	B uint8
)

const (
	V0, V1, V2, V3, V4, V5, V6, V7 V = 0, 1, 2, 3, 4, 5, 6, 7
	V8, V9, V10, V11, V12, V13, V14, V15 V = 8, 9, 10, 11, 12, 13, 14, 15
	V16, V17, V18, V19, V20, V21, V22, V23 V = 16, 17, 18, 19, 20, 21, 22, 23
	V24, V25, V26, V27, V28, V29, V30, V31 V = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7 Q = 0, 1, 2, 3, 4, 5, 6, 7
	Q8, Q9, Q10, Q11, Q12, Q13, Q14, Q15 Q = 8, 9, 10, 11, 12, 13, 14, 15
	Q16, Q17, Q18, Q19, Q20, Q21, Q22, Q23 Q = 16, 17, 18, 19, 20, 21, 22, 23
	Q24, Q25, Q26, Q27, Q28, Q29, Q30, Q31 Q = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	D0, D1, D2, D3, D4, D5, D6, D7 D = 0, 1, 2, 3, 4, 5, 6, 7
	D8, D9, D10, D11, D12, D13, D14, D15 D = 8, 9, 10, 11, 12, 13, 14, 15
	D16, D17, D18, D19, D20, D21, D22, D23 D = 16, 17, 18, 19, 20, 21, 22, 23
	D24, D25, D26, D27, D28, D29, D30, D31 D = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	S0, S1, S2, S3, S4, S5, S6, S7 S = 0, 1, 2, 3, 4, 5, 6, 7
	S8, S9, S10, S11, S12, S13, S14, S15 S = 8, 9, 10, 11, 12, 13, 14, 15
	S16, S17, S18, S19, S20, S21, S22, S23 S = 16, 17, 18, 19, 20, 21, 22, 23
	S24, S25, S26, S27, S28, S29, S30, S31 S = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	H0, H1, H2, H3, H4, H5, H6, H7 H = 0, 1, 2, 3, 4, 5, 6, 7
	H8, H9, H10, H11, H12, H13, H14, H15 H = 8, 9, 10, 11, 12, 13, 14, 15
	H16, H17, H18, H19, H20, H21, H22, H23 H = 16, 17, 18, 19, 20, 21, 22, 23
	H24, H25, H26, H27, H28, H29, H30, H31 H = 24, 25, 26, 27, 28, 29, 30, 31
)

const (
	B0, B1, B2, B3, B4, B5, B6, B7 B = 0, 1, 2, 3, 4, 5, 6, 7
	B8, B9, B10, B11, B12, B13, B14, B15 B = 8, 9, 10, 11, 12, 13, 14, 15
	B16, B17, B18, B19, B20, B21, B22, B23 B = 16, 17, 18, 19, 20, 21, 22, 23
	B24, B25, B26, B27, B28, B29, B30, B31 B = 24, 25, 26, 27, 28, 29, 30, 31
)

func (r V) Num() uint16  { return uint16(r) }
func (r V) Bits() uint16 { return 128 }
func (r V) Class() Class { return ClassV }
func (r Q) Num() uint16  { return uint16(r) }
func (r Q) Bits() uint16 { return 128 }
func (r Q) Class() Class { return ClassQ }
func (r D) Num() uint16  { return uint16(r) }
func (r D) Bits() uint16 { return 64 }
func (r D) Class() Class { return ClassD }
func (r S) Num() uint16  { return uint16(r) }
func (r S) Bits() uint16 { return 32 }
func (r S) Class() Class { return ClassS }
func (r H) Num() uint16  { return uint16(r) }
func (r H) Bits() uint16 { return 16 }
func (r H) Class() Class { return ClassH }
func (r B) Num() uint16  { return uint16(r) }
func (r B) Bits() uint16 { return 8 }
func (r B) Class() Class { return ClassB }

// Elem is an element width inside a SIMD register.
type Elem uint8

const (
	ElemNone Elem = iota
	ElemB         // 8-bit
	ElemH         // 16-bit
	ElemS         // 32-bit
	ElemD         // 64-bit
)

// Bits is the width of one element, or 0 for ElemNone.
func (e Elem) Bits() uint16 {
	switch e {
	case ElemB:
		return 8
	case ElemH:
		return 16
	case ElemS:
		return 32
	case ElemD:
		return 64
	}
	return 0
}

// Arrangement is an element width together with a count, which is what the
// architecture writes after the dot: v0.4s.
//
// These are spelled V4S rather than 4S because a Go identifier cannot start
// with a digit, and V16B rather than B16 because B16 is already the scalar
// register b16. See the note in the package documentation.
type Arrangement uint8

const (
	ArrNone Arrangement = iota
	V8B
	V16B
	V4H
	V8H
	V2S
	V4S
	V1D
	V2D
)

var arrTable = [...]struct {
	elem  Elem
	lanes uint8
}{
	V8B:  {ElemB, 8},
	V16B: {ElemB, 16},
	V4H:  {ElemH, 4},
	V8H:  {ElemH, 8},
	V2S:  {ElemS, 2},
	V4S:  {ElemS, 4},
	V1D:  {ElemD, 1},
	V2D:  {ElemD, 2},
}

// Elem is the width of one lane.
func (a Arrangement) Elem() Elem {
	if int(a) >= len(arrTable) {
		return ElemNone
	}
	return arrTable[a].elem
}

// Lanes is the number of elements.
func (a Arrangement) Lanes() uint8 {
	if int(a) >= len(arrTable) {
		return 0
	}
	return arrTable[a].lanes
}

// Bits is the total width the arrangement occupies: 64 for the short forms,
// 128 for the long ones. This is the Q bit of most SIMD encodings.
func (a Arrangement) Bits() uint16 {
	return uint16(a.Elem().Bits()) * uint16(a.Lanes())
}

// Vec is a SIMD register with an arrangement: v0.16b.
type Vec struct {
	R V
	A Arrangement
}

// Arr decorates a register with an arrangement.
func (r V) Arr(a Arrangement) Vec { return Vec{R: r, A: a} }

func (v Vec) Num() uint16  { return uint16(v.R) }
func (v Vec) Bits() uint16 { return v.A.Bits() }
func (v Vec) Class() Class { return ClassVArr }

// VLane is one element of a SIMD register: v2.s[1].
type VLane struct {
	R     V
	E     Elem
	Index uint8
}

// Lane names one element of a register.
func (r V) Lane(e Elem, index uint8) VLane {
	return VLane{R: r, E: e, Index: index}
}

func (l VLane) Num() uint16  { return uint16(l.R) }
func (l VLane) Bits() uint16 { return l.E.Bits() }
func (l VLane) Class() Class { return ClassVLane }

// Valid reports whether the index is inside a 128-bit register at this element
// width.
func (l VLane) Valid() bool {
	if l.E == ElemNone {
		return false
	}
	return uint16(l.Index)*l.E.Bits() < 128
}

// ByElementEncodable reports whether this lane can be the "by element" operand
// of an instruction that reads one lane and broadcasts it. At 16-bit elements
// the source is restricted to V0-V15, because the form has no bit left for the
// high half of the register number.
//
// Whether a given form has that restriction is isa/'s question. Whether this
// particular register survives it is this package's, because the answer is a
// property of the register number.
func (l VLane) ByElementEncodable() bool {
	return l.E != ElemH || l.R <= 15
}