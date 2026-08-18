// x86_64/decode/vex.go
package decode

import "github.com/vertex-language/arc/x86_64/isa"

func mapFromBits(b byte) (isa.Map, bool) {
	switch b {
	case 1:
		return isa.Map0F, true
	case 2:
		return isa.Map0F38, true
	case 3:
		return isa.Map0F3A, true
	}
	return isa.Map1, false
}

func pfxFromBits(b byte) isa.Pfx {
	switch b {
	case 1:
		return isa.Pfx66
	case 2:
		return isa.PfxF3
	case 3:
		return isa.PfxF2
	}
	return isa.PfxNone
}

// vex2 unwinds the two-byte VEX prefix. X, B and W are not present, which is
// exactly the condition under which the encoder chose this form; a decoder
// that inferred them from anything but zero would accept encodings the
// silicon rejects.
func (d *dec) vex2() error {
	off := d.pos
	d.pos++ // 0xc5
	c, err := d.byteAt()
	if err != nil {
		return err
	}

	d.enc = isa.EncVEX
	d.r = ^c >> 7 & 1
	d.vvvv = ^c >> 3 & 15
	d.ll = c >> 2 & 1
	d.vpp = pfxFromBits(c & 3)
	d.vmap = isa.Map0F
	d.vw = 0

	d.mark(off, 2, fieldVEX)
	return nil
}

func (d *dec) vex3() error {
	off := d.pos
	d.pos++ // 0xc4
	b1, err := d.byteAt()
	if err != nil {
		return err
	}
	b2, err := d.byteAt()
	if err != nil {
		return err
	}

	d.enc = isa.EncVEX
	d.r = ^b1 >> 7 & 1
	d.x = ^b1 >> 6 & 1
	d.bb = ^b1 >> 5 & 1

	m, ok := mapFromBits(b1 & 31)
	if !ok {
		return &UnknownError{Bytes: d.b[off:d.pos], Offset: off}
	}
	d.vmap = m

	d.vw = b2 >> 7 & 1
	d.vvvv = ^b2 >> 3 & 15
	d.ll = b2 >> 2 & 1
	d.vpp = pfxFromBits(b2 & 3)

	d.mark(off, 3, fieldVEX)
	return nil
}

// evex unwinds the four-byte EVEX prefix.
//
// Three fields have no VEX counterpart and every one of them is a place a
// decoder silently loses sixteen registers: R' is the fifth bit of reg, V'
// is the fifth bit of vvvv, and X is the fifth bit of r/m when r/m is a
// register rather than the index extension it is when r/m is memory.
func (d *dec) evex() error {
	off := d.pos
	d.pos++ // 0x62
	p0, err := d.byteAt()
	if err != nil {
		return err
	}
	p1, err := d.byteAt()
	if err != nil {
		return err
	}
	p2, err := d.byteAt()
	if err != nil {
		return err
	}

	d.enc = isa.EncEVEX
	d.r = ^p0 >> 7 & 1
	d.x = ^p0 >> 6 & 1
	d.bb = ^p0 >> 5 & 1
	d.rp = ^p0 >> 4 & 1

	m, ok := mapFromBits(p0 & 3)
	if !ok {
		return &UnknownError{Bytes: d.b[off:d.pos], Offset: off}
	}
	d.vmap = m

	if p1>>2&1 != 1 {
		// Bit 2 of the second payload byte is fixed at one. A zero there is
		// not an EVEX instruction, whatever else it might be.
		return &UnknownError{Bytes: d.b[off:d.pos], Offset: off}
	}
	d.vw = p1 >> 7 & 1
	d.vvvv = ^p1 >> 3 & 15
	d.vpp = pfxFromBits(p1 & 3)

	d.zero = p2>>7&1 == 1
	d.ll = p2 >> 5 & 3
	d.bcst = p2>>4&1 == 1
	d.vp = ^p2 >> 3 & 1
	d.aaa = p2 & 7

	d.mark(off, 4, fieldEVEX)
	return nil
}

// regNum assembles a five-bit register number from the ModRM field and
// whichever extension bits this encoding carries.
func (d *dec) regNum() uint8 {
	return uint8(d.rp<<4 | d.r<<3 | d.regf)
}

// rmNum is the same for the r/m field. Under EVEX the fifth bit is X, which
// is why a register operand and a memory operand read the same bit
// differently — and why swapping them costs sixteen registers rather than
// producing an obvious error.
func (d *dec) rmNum() uint8 {
	if d.enc == isa.EncEVEX && d.mod == 3 {
		return uint8(d.x<<4 | d.bb<<3 | d.rm)
	}
	return uint8(d.bb<<3 | d.rm)
}

func (d *dec) vvvvNum() uint8 {
	if d.enc == isa.EncEVEX {
		return uint8(d.vp<<4 | d.vvvv)
	}
	return d.vvvv
}