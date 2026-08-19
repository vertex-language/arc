package operand

import "math/bits"

// The logical immediate.
//
// AND, ORR, EOR and their flag-setting forms do not take a constant; they take
// an N:immr:imms triple that names one, and the set of constants nameable that
// way is exactly the rotations of a run of ones replicated to fill the
// register. 0xff00ff00ff00ff00 is expressible and 0xff00ff00ff00ff01 is not,
// and no amount of field widening changes that.
//
// So "is this constant even expressible" is a first-class question here with a
// real answer, rather than a range check. A caller that wants a constant this
// cannot express has to materialize it — MOVZ into a scratch register and then
// the register form — which is a decision about which instructions to emit and
// therefore the caller's, not this package's.
//
// The search below is the standard one: find the smallest element size the
// value replicates at, check that the element is a rotated contiguous run of
// ones, and derive the triple from the run's length and rotation.

// EncodeBitmask computes the N, immr and imms fields that name v as a logical
// immediate at the given datasize. ok is false when v has no such encoding.
//
// At Width32 the value is taken as a 32-bit pattern and its high half ignored;
// the architecture encodes a 32-bit logical immediate as a 64-bit pattern that
// repeats, which is why N is always zero there.
func EncodeBitmask(v uint64, w Width) (n, immr, imms uint8, ok bool) {
	switch w {
	case Width32:
		v &= 0xffffffff
		v |= v << 32
	case Width64:
	default:
		return 0, 0, 0, false
	}

	// All zeros and all ones are the two patterns the encoding cannot name:
	// the run of ones must be neither empty nor complete. Both are also the
	// two a caller most plausibly writes, so the diagnostic encode/ builds on
	// top of this should say so by name.
	if v == 0 || v == ^uint64(0) {
		return 0, 0, 0, false
	}

	// The smallest element size the value is a replication of.
	size := 64
	for {
		size /= 2
		mask := ones(uint(size))
		if v&mask != (v>>uint(size))&mask {
			size *= 2
			break
		}
		if size <= 2 {
			break
		}
	}

	mask := ones(uint(size))
	elem := v & mask

	var i, cto int
	if isShiftedMask(elem) {
		i = bits.TrailingZeros64(elem)
		cto = bits.TrailingZeros64(^(elem >> uint(i)))
	} else {
		// The run wraps the element boundary. Fill the bits above the element
		// so the complement is a single run, and measure that instead.
		elem |= ^mask
		if !isShiftedMask(^elem) {
			return 0, 0, 0, false
		}
		clo := bits.LeadingZeros64(^elem)
		i = 64 - clo
		cto = clo + bits.TrailingZeros64(^elem) - (64 - size)
	}

	immr = uint8((size - i) & (size - 1))

	// imms is the run length minus one, below a unary prefix of zeros that
	// encodes the element size. N is the bit that prefix overflows into, which
	// is why only a 64-bit element sets it.
	nimms := (^(size - 1) << 1) | (cto - 1)
	n = uint8(((nimms >> 6) & 1) ^ 1)
	imms = uint8(nimms & 0x3f)
	return n, immr, imms, true
}

// BitmaskEncodable reports whether v can be a logical immediate at this width.
func BitmaskEncodable(v uint64, w Width) bool {
	_, _, _, ok := EncodeBitmask(v, w)
	return ok
}

// DecodeBitmask is the inverse, for decode/ and for the differential suite,
// which checks that every triple this package emits reads back as the constant
// it was asked for.
func DecodeBitmask(n, immr, imms uint8, w Width) (uint64, bool) {
	var datasize int
	switch w {
	case Width32:
		datasize = 32
	case Width64:
		datasize = 64
	default:
		return 0, false
	}

	// len is the position of the highest set bit of N:NOT(imms), seven bits
	// wide. It is the log of the element size.
	x := uint32(n&1)<<6 | uint32(^imms&0x3f)
	if x == 0 {
		return 0, false
	}
	length := 31 - bits.LeadingZeros32(x)
	if length < 1 || datasize < 1<<length {
		return 0, false
	}

	levels := uint8(1<<length) - 1
	s := imms & levels
	r := immr & levels
	if s == levels {
		return 0, false // a complete run: not a logical immediate
	}

	esize := 1 << length
	elem := ror(ones(uint(s)+1), uint(r), uint(esize))

	out := uint64(0)
	for i := 0; i < datasize; i += esize {
		out |= elem << uint(i)
	}
	if datasize == 32 {
		out &= 0xffffffff
	}
	return out, true
}

// ones is a run of n set bits at the bottom of a word. n of 64 is the whole
// word: Go defines an over-wide shift as zero, so the subtraction wraps to the
// answer we want rather than being undefined the way C's is.
func ones(n uint) uint64 { return (uint64(1) << n) - 1 }

// ror rotates the low size bits of v right by r.
func ror(v uint64, r, size uint) uint64 {
	if r == 0 {
		return v & ones(size)
	}
	m := ones(size)
	v &= m
	return ((v >> r) | (v << (size - r))) & m
}

// isMask reports whether v is a run of ones ending at bit zero.
func isMask(v uint64) bool { return v != 0 && (v+1)&v == 0 }

// isShiftedMask reports whether v is a single contiguous run of ones anywhere
// in the word.
func isShiftedMask(v uint64) bool { return v != 0 && isMask(v+(v&-v)) }