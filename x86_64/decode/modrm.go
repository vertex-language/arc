// x86_64/decode/modrm.go
package decode

import (
	"encoding/binary"

	"github.com/vertex-language/arc/x86_64/isa"
)

// modrm reads the ModRM byte, the SIB byte if the addressing form calls for
// one, and the displacement.
func (d *dec) modrm() error {
	off := d.pos
	c, err := d.byteAt()
	if err != nil {
		return err
	}
	d.hasModRM = true
	d.mod = c >> 6
	d.regf = c >> 3 & 7
	d.rm = c & 7
	d.mark(off, 1, fieldModRM)

	if d.mod == 3 {
		return nil // a register: no SIB, no displacement
	}

	// rm=100 is the escape to SIB. It is not RSP; RSP is what you get when
	// the SIB's base says so.
	if d.rm == 4 {
		off := d.pos
		s, err := d.byteAt()
		if err != nil {
			return err
		}
		d.hasSIB = true
		d.scale = 1 << (s >> 6)
		d.index = s >> 3 & 7
		d.base = s & 7
		d.mark(off, 1, fieldSIB)
	}

	return d.displacement()
}

func (d *dec) displacement() error {
	switch {
	case d.mod == 0 && d.rm == 5:
		// mod=00 rm=101 is rip-relative in long mode. In 32-bit mode the
		// same encoding is an absolute disp32, which is one of the handful
		// of places i386 and x86_64 genuinely differ rather than merely
		// repeat each other.
		d.rip = true
		return d.disp32()

	case d.mod == 0 && d.hasSIB && d.base == 5:
		// No base: disp32 follows, with or without an index.
		return d.disp32()

	case d.mod == 1:
		off := d.pos
		c, err := d.byteAt()
		if err != nil {
			return err
		}
		// Under EVEX the byte is disp/N, so the displacement it stands for
		// is N times larger. Reading it as a plain signed byte is how a
		// disassembler prints an offset sixty-four times too small and
		// nobody notices until it is compared against objdump.
		d.disp = int32(int8(c)) * int32(d.dispScale())
		d.hasDisp = true
		d.mark(off, 1, fieldDisp)
		return nil

	case d.mod == 2:
		return d.disp32()
	}
	return nil
}

func (d *dec) disp32() error {
	off := d.pos
	if d.pos+4 > len(d.b) {
		return ErrTruncated
	}
	d.disp = int32(binary.LittleEndian.Uint32(d.b[d.pos:]))
	d.hasDisp = true
	d.pos += 4
	d.mark(off, 4, fieldDisp)
	return nil
}

// dispScale is EVEX's N, computed the same way encode/ computes it. The two
// implementations are separate because the packages do not import each
// other; they are checked against each other by the round-trip test, which
// is the only check that would catch them drifting anyway.
func (d *dec) dispScale() int {
	if d.enc != isa.EncEVEX || d.form == nil {
		return 1
	}
	vl := 0
	switch d.form.Len {
	case isa.L128:
		vl = 16
	case isa.L256:
		vl = 32
	case isa.L512:
		vl = 64
	}
	elem := int(d.form.Elem)
	if elem == 0 {
		elem = 4
	}
	bcst := d.bcst && d.mod != 3

	switch d.form.Tuple {
	case isa.TupleFull:
		if bcst {
			return elem
		}
		return vl
	case isa.TupleHalf:
		if bcst {
			return elem
		}
		return vl / 2
	case isa.TupleFullMem:
		return vl
	case isa.Tuple1Scalar:
		if i := d.form.MemSlot(); i >= 0 {
			if b := d.form.Slots[i].Class.Bits(); b > 0 && b <= 64 {
				return b / 8
			}
		}
		return elem
	case isa.Tuple1Fixed:
		return elem
	case isa.Tuple2:
		if d.form.W == isa.W1 {
			return 16
		}
		return 8
	case isa.Tuple4:
		if d.form.W == isa.W1 {
			return 32
		}
		return 16
	case isa.Tuple8:
		return 32
	case isa.TupleHalfMem:
		return vl / 2
	case isa.TupleQuarterMem:
		return vl / 4
	case isa.TupleEighthMem:
		return vl / 8
	case isa.TupleMem128:
		return 16
	case isa.TupleMOVDDUP:
		if vl == 16 {
			return 8
		}
		return vl / 2
	}
	return 1
}

// immediate reads the immediate or relative displacement field.
func (d *dec) immediate() error {
	n := d.form.Imm.Bytes()
	if n == 0 {
		return nil
	}
	off := d.pos
	if d.pos+n > len(d.b) {
		return ErrTruncated
	}
	raw := d.b[d.pos : d.pos+n]
	d.pos += n

	// The is4 byte carries a register in its high nibble and nothing in its
	// low one. It is the immediate field, so it is read here and split in
	// operands().
	if hasIS4(d.form) {
		d.is4 = raw[0] >> 4
		d.mark(off, 1, fieldIS4)
		return nil
	}

	switch n {
	case 1:
		d.imm = int64(int8(raw[0]))
	case 2:
		d.imm = int64(int16(binary.LittleEndian.Uint16(raw)))
	case 4:
		d.imm = int64(int32(binary.LittleEndian.Uint32(raw)))
	case 8:
		d.imm = int64(binary.LittleEndian.Uint64(raw))
	}
	d.hasImm = true

	k := fieldImm
	if d.form.Attrs&isa.Branch != 0 {
		k = fieldRel
	}
	d.mark(off, n, k)
	return nil
}

func hasIS4(f *isa.Form) bool {
	for _, s := range f.Slots {
		if s.Field == isa.InIS4 {
			return true
		}
	}
	return false
}