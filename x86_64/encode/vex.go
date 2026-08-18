// x86_64/encode/vex.go
package encode

import "github.com/vertex-language/arc/x86_64/isa"

// mmmmm values. The same numbering serves VEX and EVEX, which is the one
// place the two prefixes agree without qualification.
func mapBits(m isa.Map) byte {
	switch m {
	case isa.Map0F:
		return 1
	case isa.Map0F38:
		return 2
	case isa.Map0F3A:
		return 3
	}
	return 0
}

// pp values, the two bits that replace a mandatory prefix byte.
func ppBits(p isa.Pfx) byte {
	switch p {
	case isa.Pfx66:
		return 1
	case isa.PfxF3:
		return 2
	case isa.PfxF2:
		return 3
	}
	return 0
}

func lBit(l isa.VLen) byte {
	if l == isa.L256 {
		return 1
	}
	return 0
}

// rxb returns the three extension bits in their uninverted form.
func (e *enc) rxb() (r, x, b byte) {
	r = e.regv.num() >> 3 & 1
	if e.rmv != nil {
		switch e.rmv.kind {
		case kReg:
			b = e.rmv.num() >> 3 & 1
		case kMem:
			if e.rmv.mem.HasBase {
				b = e.rmv.mem.Base.Num() >> 3 & 1
			}
			if e.rmv.mem.HasIndex {
				x = e.rmv.mem.Index.Num() >> 3 & 1
			}
		}
	}
	return
}

// vex emits the two- or three-byte VEX prefix.
//
// The two-byte form is not an optimization to be skipped: it is two bytes
// where the long form is three, and `arc` claims the shortest legal
// encoding. It is available only when X, B and W are all clear and the map
// is 0F, because those are the fields C5 has no room for.
func (e *enc) vex() {
	r, x, b := e.rxb()
	vv := byte(0)
	if e.vvvv != nil {
		vv = e.vvvv.num() & 15
	}
	l := lBit(e.f.Len)
	pp := ppBits(e.f.Pfx)
	w := byte(0)
	if e.f.W == isa.W1 {
		w = 1
	}

	if x == 0 && b == 0 && w == 0 && e.f.Map == isa.Map0F {
		e.emit(0xc5, ^r<<7|(^vv&15)<<3|l<<2|pp)
		return
	}
	e.emit(0xc4,
		^r<<7|^x<<6|^b<<5|mapBits(e.f.Map),
		w<<7|(^vv&15)<<3|l<<2|pp)
}

// evex emits the four-byte EVEX prefix.
//
// Two fields here have no VEX counterpart and are easy to lose: R' and V'
// are the fifth bits of the reg and vvvv fields, and X is the fifth bit of a
// register in r/m — which is why a register operand and a memory operand
// disagree about what X means, and why the SIB index steals it back when
// there is one.
func (e *enc) evex() {
	r := e.regv.num() >> 3 & 1
	rp := e.regv.num() >> 4 & 1

	var x, b byte
	if e.rmv != nil {
		switch e.rmv.kind {
		case kReg:
			// A register in r/m spans five bits: X is the high one.
			b = e.rmv.num() >> 3 & 1
			x = e.rmv.num() >> 4 & 1
		case kMem:
			if e.rmv.mem.HasBase {
				b = e.rmv.mem.Base.Num() >> 3 & 1
			}
			if e.rmv.mem.HasIndex {
				x = e.rmv.mem.Index.Num() >> 3 & 1
			}
		}
	}

	var vv, vp byte
	if e.vvvv != nil {
		vv = e.vvvv.num() & 15
		vp = e.vvvv.num() >> 4 & 1
	}

	w := byte(0)
	if e.f.W == isa.W1 {
		w = 1
	}

	// L'L is the vector length, unless embedded rounding has taken the
	// field over — at which point the length is 512 by definition and the
	// two bits name a rounding mode instead.
	var ll byte
	switch e.f.Len {
	case isa.L256:
		ll = 1
	case isa.L512:
		ll = 2
	}
	bb := byte(0)
	if e.opts.Broadcast {
		bb = 1
	}
	if e.opts.Round != RoundNone {
		ll = e.opts.Round.bits()
		bb = 1
	} else if e.opts.SAE {
		bb = 1
	}

	var z byte
	if e.opts.Zero {
		z = 1
	}
	var aaa byte
	if e.maskv != nil {
		aaa = e.maskv.num() & 7
	}

	e.emit(0x62,
		^r<<7|^x<<6|^b<<5|^rp<<4|mapBits(e.f.Map)&3,
		w<<7|(^vv&15)<<3|1<<2|ppBits(e.f.Pfx),
		z<<7|ll<<5|bb<<4|^vp<<3|aaa)
}