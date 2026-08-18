// x86_64/operand/imm.go
package operand

import "strconv"

// Imm is an immediate. It holds the value the user wrote; the field width is
// the encoder's choice, made against the form it resolved. An Imm does not
// know whether it will land in an imm8, an imm32 sign-extended to 64, or a
// full imm64 — asking it would mean asking before the form is known.
type Imm int64

// Uimm builds an Imm from an unsigned value. Values above 2^63-1 wrap to
// negative, which is the same bit pattern and the same bytes; the sign is a
// reading of the field, not a property of it.
func Uimm(v uint64) Imm { return Imm(int64(v)) }

// FitsInt8 reports whether the value survives an imm8 sign-extended to 64.
func (i Imm) FitsInt8() bool { return int64(i) >= -128 && int64(i) <= 127 }

// FitsInt16 reports whether the value survives an imm16 sign-extended.
func (i Imm) FitsInt16() bool { return int64(i) >= -32768 && int64(i) <= 32767 }

// FitsInt32 reports whether the value survives an imm32 sign-extended to 64.
// This is the test that decides between MOV r/m64, imm32 and MOV r64, imm64.
func (i Imm) FitsInt32() bool {
	return int64(i) >= -2147483648 && int64(i) <= 2147483647
}

// FitsUint32 reports whether the value survives an imm32 zero-extended, which
// is what a 32-bit destination gives you for free: a write to a 32-bit
// register clears the upper 32 bits of its parent.
func (i Imm) FitsUint32() bool {
	return uint64(i) <= 0xffffffff
}

// Narrowest is the smallest immediate width that holds the value under sign
// extension. encode/ uses it to break ties; the typed helper layer does not,
// because a caller who named MovR64Imm64 asked for imm64.
func (i Imm) Narrowest() Width {
	switch {
	case i.FitsInt8():
		return W8
	case i.FitsInt16():
		return W16
	case i.FitsInt32():
		return W32
	}
	return W64
}

func (i Imm) String() string { return strconv.FormatInt(int64(i), 10) }