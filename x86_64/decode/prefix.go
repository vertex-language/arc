// x86_64/decode/prefix.go
package decode

import "github.com/vertex-language/arc/x86_64/isa"

// legacyMap and legacyPfx are fields of dec, declared here because they are
// only meaningful for a legacy encoding.
type legacyState struct {
	legacyMap isa.Map
	legacyPfx isa.Pfx
}

// prefixes scans the legacy prefix bytes, then whichever of REX, VEX or EVEX
// follows.
//
// The architecture accepts the legacy prefixes in any order and any number,
// up to the fifteen-byte instruction limit. This scans them in any order
// because it has to; encode/ emits them in one order because byte identity
// with GNU as demands it. The asymmetry is real and the differential suite
// depends on both halves of it.
func (d *dec) prefixes() error {
	for {
		c, ok := d.peek()
		if !ok {
			return ErrTruncated
		}
		switch c {
		case 0xf0:
			d.lock = true
		case 0xf2, 0xf3:
			d.rep = Prefix(c)
		case 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65:
			d.seg = Prefix(c)
		case 0x66:
			d.data16 = true
		case 0x67:
			d.addr32 = true
		default:
			goto done
		}
		d.mark(d.pos, 1, fieldPrefix)
		d.pos++
		if d.pos > 14 {
			return ErrTooLong
		}
	}
done:
	c, ok := d.peek()
	if !ok {
		return ErrTruncated
	}

	switch {
	case c == 0x62:
		return d.evex()
	case c == 0xc4:
		return d.vex3()
	case c == 0xc5:
		return d.vex2()
	}

	d.enc = isa.EncLegacy
	if c >= 0x40 && c <= 0x4f {
		// REX must be the last prefix before the opcode. One that is not is
		// ignored by the silicon, and a decoder that honored it anyway
		// would disagree with the CPU about which registers an instruction
		// names.
		d.rex, d.hasRex = c, true
		d.r = c >> 2 & 1
		d.x = c >> 1 & 1
		d.bb = c & 1
		d.mark(d.pos, 1, fieldREX)
		d.pos++
	}

	// The mandatory SIMD prefix and the operand-size override are the same
	// bytes read differently. Which reading applies is settled by the
	// lookup: the table is consulted with the SIMD reading first, and the
	// operand-size reading is what is left when nothing matches.
	d.resolveLegacyPrefix()
	return nil
}

// resolveLegacyPrefix decides which of the scanned bytes was a mandatory
// prefix. It is a guess that match() confirms: 66 is Pfx66 if a form exists
// under it and an operand-size override otherwise, and the same for F2/F3.
func (d *dec) resolveLegacyPrefix() {
	switch {
	case d.rep == PrefixRep:
		d.legacyPfx = isa.PfxF3
	case d.rep == PrefixRepNE:
		d.legacyPfx = isa.PfxF2
	case d.data16:
		d.legacyPfx = isa.Pfx66
	default:
		d.legacyPfx = isa.PfxNone
	}
}

// opcodeByte reads the escape bytes and the opcode.
func (d *dec) opcodeByte() error {
	off := d.pos
	c, err := d.byteAt()
	if err != nil {
		return err
	}

	if d.enc == isa.EncLegacy {
		if c == 0x0f {
			c2, err := d.byteAt()
			if err != nil {
				return err
			}
			switch c2 {
			case 0x38:
				d.legacyMap = isa.Map0F38
				c, err = d.byteAt()
				if err != nil {
					return err
				}
			case 0x3a:
				d.legacyMap = isa.Map0F3A
				c, err = d.byteAt()
				if err != nil {
					return err
				}
			default:
				d.legacyMap = isa.Map0F
				c = c2
			}
			d.mark(off, d.pos-off-1, fieldEscape)
		} else {
			d.legacyMap = isa.Map1
		}
	}

	d.opcode = c
	d.mark(d.pos-1, 1, fieldOpcode)

	// A form with no matching row under the SIMD reading of the prefix gets
	// a second chance under the operand-size reading. Trying the SIMD
	// reading first is what makes MOVSD decode as MOVSD rather than as a
	// 16-bit MOVUPS with a stray F2.
	return nil
}

// retryAsOperandSize is called by match() when the SIMD reading found
// nothing: the prefix was an override after all.
func (d *dec) retryAsOperandSize() bool {
	if d.legacyPfx == isa.PfxNone {
		return false
	}
	d.legacyPfx = isa.PfxNone
	return true
}