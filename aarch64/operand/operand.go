// Package operand is the AArch64 operand set: the values that fill a form's
// slots, before the encoder turns them into bits.
//
// Nothing here knows about the instruction table. An operand states what the
// caller wrote — a register, a constant, an address, a symbol and which half of
// it — and answers only the questions that are properties of the value itself:
// whether a constant is expressible in a field of a given shape, whether an
// addressing mode has an encoding at all. Which form accepts which operand is
// isa/'s question. Placing the bits is encode/'s.
//
// The width predicates live here rather than in encode/ because they are the
// architecture's arithmetic, not the encoder's bookkeeping: whether 0x1ff fits
// ADD's twelve bits is true or false before any form is resolved, and a caller
// building operands at runtime has a reason to ask before it emits.
package operand

// Width is an operand or memory-access width in bits.
//
// Zero is "not stated", which is a real answer: Mem(X1) leaves the access width
// to the form, and only Mem64(X1) insists on one. isa/'s Class.Match reads the
// difference.
type Width uint16

const (
	WidthNone Width = 0
	Width8    Width = 8
	Width16   Width = 16
	Width32   Width = 32
	Width64   Width = 64
	Width128  Width = 128
)

// Valid reports whether w is a width the architecture accesses memory at.
func (w Width) Valid() bool {
	switch w {
	case Width8, Width16, Width32, Width64, Width128:
		return true
	}
	return false
}

// Bytes is the width in bytes, or 0 if the width is not stated.
func (w Width) Bytes() uint16 { return uint16(w) / 8 }

// Scale is log2 of the byte size: the shift a scaled immediate offset is
// divided by, and the number LDR's unsigned imm12 and LDP's signed imm7 are
// both expressed in.
func (w Width) Scale() (uint8, bool) {
	switch w {
	case Width8:
		return 0, true
	case Width16:
		return 1, true
	case Width32:
		return 2, true
	case Width64:
		return 3, true
	case Width128:
		return 4, true
	}
	return 0, false
}

func (w Width) String() string {
	switch w {
	case Width8:
		return "byte"
	case Width16:
		return "halfword"
	case Width32:
		return "word"
	case Width64:
		return "doubleword"
	case Width128:
		return "quadword"
	}
	return "unsized"
}

// lower is ASCII lower-casing, for the Lookup functions. Assembly source is
// case-insensitive and register and condition names are ASCII, so this avoids
// pulling in strings for two callers.
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// itoa is strconv.Itoa for the small non-negative values this package prints.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}