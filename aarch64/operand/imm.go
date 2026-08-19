package operand

import "strconv"

// Imm is an integer operand. It is signed because assembly source writes
// negative constants and the field predicates below are what decide whether a
// given field can hold one; a caller writing an unsigned bit pattern wider than
// int64 wants the bitmask encoder, which takes a uint64.
type Imm int64

func (i Imm) String() string { return "#" + strconv.FormatInt(int64(i), 10) }

// Fits reports whether a value is representable in a field of n bits, read as
// signed or unsigned. It is the general case; the named predicates below are
// the fields whose rule is more than a width.
func Fits(v int64, bits uint8, signed bool) bool {
	if bits == 0 || bits > 63 {
		return false
	}
	if signed {
		lo := -(int64(1) << (bits - 1))
		hi := (int64(1) << (bits - 1)) - 1
		return v >= lo && v <= hi
	}
	return v >= 0 && v < (int64(1)<<bits)
}

// FitsImm12 is ADD/SUB (immediate): twelve unsigned bits, optionally shifted
// left by twelve. shift reports whether the LSL #12 form is needed, which is
// the Sh field's one bit.
//
// The two are not interchangeable at the boundary. 0x1000 fits both ways —
// unshifted it does not, shifted it is 1 — and the unshifted encoding simply
// has no room for it, so there is no choice to make and no shortest form to
// search for.
func FitsImm12(v int64) (imm uint16, shift bool, ok bool) {
	if v < 0 {
		return 0, false, false
	}
	if v <= 0xfff {
		return uint16(v), false, true
	}
	if v&0xfff == 0 && v>>12 <= 0xfff {
		return uint16(v >> 12), true, true
	}
	return 0, false, false
}

// FitsImm16Shifted is MOVZ/MOVN/MOVK: sixteen bits at one of four shifts, two
// of which exist only at 64 bits. hw is the field value, which is the shift
// divided by sixteen.
//
// A value needing more than one of these is not encodable, and this returns
// false rather than reporting the first chunk. Expanding it into a MOVZ/MOVK
// chain is one mnemonic becoming several, which is selection and not this
// package's to do — see the pseudo-instruction list in the arch README.
func FitsImm16Shifted(v uint64, w Width) (imm uint16, hw uint8, ok bool) {
	var slots uint8
	switch w {
	case Width32:
		if v>>32 != 0 {
			return 0, 0, false
		}
		slots = 2
	case Width64:
		slots = 4
	default:
		return 0, 0, false
	}
	for h := uint8(0); h < slots; h++ {
		sh := uint(h) * 16
		if v&^(uint64(0xffff)<<sh) == 0 {
			return uint16(v >> sh), h, true
		}
	}
	// Zero matches the first slot above, so reaching here means the value
	// spans more than one halfword.
	return 0, 0, false
}

// FitsImm9 is the unscaled signed nine-bit offset of LDUR, STUR and the
// writeback forms. It is not scaled by the access width, which is the entire
// difference between LDUR and LDR and the reason they are separate mnemonics
// rather than one with two immediate ranges.
func FitsImm9(v int64) (imm uint16, ok bool) {
	if v < -256 || v > 255 {
		return 0, false
	}
	return uint16(v) & 0x1ff, true
}

// FitsImm7Scaled is LDP/STP's signed seven-bit offset, in units of the access
// width. The width is the width of one of the pair's registers, not of both.
func FitsImm7Scaled(v int64, w Width) (imm uint8, ok bool) {
	sc, k := w.Scale()
	if !k {
		return 0, false
	}
	size := int64(1) << sc
	if v%size != 0 {
		return 0, false
	}
	n := v / size
	if n < -64 || n > 63 {
		return 0, false
	}
	return uint8(n) & 0x7f, true
}

// FitsUImm12Scaled is the unsigned twelve-bit offset of the scaled load and
// store forms, in units of the access width.
func FitsUImm12Scaled(v int64, w Width) (imm uint16, ok bool) {
	sc, k := w.Scale()
	if !k || v < 0 {
		return 0, false
	}
	size := int64(1) << sc
	if v%size != 0 {
		return 0, false
	}
	n := v / size
	if n > 0xfff {
		return 0, false
	}
	return uint16(n), true
}

// FitsBranch is a PC-relative displacement in bytes, as a field of n bits
// counting instructions. Every branch on this architecture is word-aligned, so
// a displacement that is not a multiple of four has no encoding at all.
func FitsBranch(delta int64, bits uint8) (imm uint32, ok bool) {
	if delta%4 != 0 {
		return 0, false
	}
	n := delta / 4
	if !Fits(n, bits, true) {
		return 0, false
	}
	return uint32(n) & ((1 << bits) - 1), true
}

// FitsPage is ADRP's twenty-one bit displacement, counted in 4KiB pages. The
// caller passes the byte distance between the two pages, already truncated to
// page granularity; whether that truncation is right is a fixup's business,
// since neither address is known here.
func FitsPage(delta int64) (imm uint32, ok bool) {
	if delta%4096 != 0 {
		return 0, false
	}
	n := delta / 4096
	if !Fits(n, 21, true) {
		return 0, false
	}
	return uint32(n) & 0x1fffff, true
}