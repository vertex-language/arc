// x86_64/encode/modrm.go
package encode

import (
	"encoding/binary"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// addressing emits ModRM, SIB and the displacement — or the moffs field,
// which replaces all three.
func (e *enc) addressing() error {
	if e.moffsv != nil {
		return e.moffs()
	}
	if e.f.Attrs&isa.HasModRM == 0 {
		return nil
	}

	regField := byte(0)
	switch {
	case e.regv != nil:
		regField = e.regv.num() & 7
	case e.f.Ext != isa.NoExt:
		regField = byte(e.f.Ext)
	}

	if e.rmv == nil {
		// A form with a /digit and no r/m operand does not exist; finish()
		// rejects it at init. Reaching here means the table changed.
		return ErrNoRM
	}

	if e.rmv.kind == kReg {
		e.emit(0xc0 | regField<<3 | e.rmv.num()&7)
		return nil
	}
	return e.memory(regField, e.rmv.mem)
}

func (e *enc) memory(regField byte, m operand.Mem) error {
	// RIP-relative: mod=00, rm=101, disp32 and nothing else. There is no
	// room for a base or an index in that encoding, which is why operand/
	// refuses one before we get here.
	if m.RIP {
		e.emit(0x00 | regField<<3 | 5)
		e.disp32(m)
		return nil
	}

	base, index := m.HasBase, m.HasIndex
	needSIB := m.NeedsSIB()

	switch {
	case !base:
		// No base: mod=00 with a SIB whose base field is 101, which means
		// disp32 with no base register. The index, if there is one, rides
		// along; if there is not, index=100 says so.
		e.emit(0x00 | regField<<3 | 4)
		e.emit(sib(m.Scale, indexField(m), 5))
		e.disp32(m)
		return nil

	case m.Sym != nil:
		// A symbolic displacement is always disp32: its value is not known
		// here, and a fixup that had to fit in a byte would be a fixup that
		// fails at link time instead of now.
		e.emit(0x80 | regField<<3 | rmField(m, needSIB))
		if needSIB {
			e.emit(sib(m.Scale, indexField(m), m.Base.Num()&7))
		}
		e.disp32(m)
		return nil
	}

	mod, dispLen := e.dispForm(m)
	e.emit(mod<<6 | regField<<3 | rmField(m, needSIB))
	if needSIB {
		e.emit(sib(m.Scale, indexField(m), m.Base.Num()&7))
	}

	switch dispLen {
	case 1:
		e.emit(byte(e.disp8Value(m)))
	case 4:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(m.Disp))
		e.emit(b[:]...)
	}
	_ = index
	return nil
}

func rmField(m operand.Mem, needSIB bool) byte {
	if needSIB {
		return 4
	}
	return m.Base.Num() & 7
}

func indexField(m operand.Mem) byte {
	if !m.HasIndex {
		return 4 // "no index" — the encoding RSP cannot escape, which is
		// why operand/ refuses RSP as an index and R12 is fine.
	}
	return m.Index.Num() & 7
}

func sib(scale uint8, index, base byte) byte {
	var ss byte
	switch scale {
	case 2:
		ss = 1
	case 4:
		ss = 2
	case 8:
		ss = 3
	}
	return ss<<6 | index<<3 | base
}

// dispForm picks mod and the displacement length.
func (e *enc) dispForm(m operand.Mem) (mod byte, length int) {
	// RBP and R13 encode as 5, and mod=00 with rm=5 means disp32-with-no-base.
	// So [rbp] has to spend a zero disp8 to be [rbp] at all.
	if m.NeedsZeroDisp8() {
		return 1, 1
	}
	if m.Disp == 0 {
		return 0, 0
	}
	if d, ok := e.disp8(m); ok {
		_ = d
		return 1, 1
	}
	return 2, 4
}

// disp8 reports whether the displacement fits a disp8, and what goes in it.
//
// For a legacy or VEX encoding that is a plain signed byte. For EVEX it is a
// compressed displacement: the byte is disp/N, so a displacement fits only
// when it is a multiple of N — and N depends on the tuple, the vector length
// and whether broadcast is on.
func (e *enc) disp8(m operand.Mem) (int8, bool) {
	n := e.dispScale()
	if n <= 1 {
		if m.Disp >= -128 && m.Disp <= 127 {
			return int8(m.Disp), true
		}
		return 0, false
	}
	if int(m.Disp)%n != 0 {
		return 0, false
	}
	q := int(m.Disp) / n
	if q < -128 || q > 127 {
		return 0, false
	}
	return int8(q), true
}

func (e *enc) disp8Value(m operand.Mem) int8 {
	d, _ := e.disp8(m)
	return d
}

// dispScale is EVEX's N. One is "no compression", which is every non-EVEX
// encoding.
//
// N is the number of bytes the instruction actually touches: the full vector
// for a form that reads all of it, one element for a broadcast or a scalar,
// a fraction for the forms that read a fraction. Getting it wrong does not
// fail — it silently addresses the wrong place, which is why the tuple is
// declared per form in isa/ and computed here rather than either one alone.
func (e *enc) dispScale() int {
	if e.f.Enc != isa.EncEVEX {
		return 1
	}
	vl := 0
	switch e.f.Len {
	case isa.L128:
		vl = 16
	case isa.L256:
		vl = 32
	case isa.L512:
		vl = 64
	}
	elem := int(e.f.Elem)
	if elem == 0 {
		elem = 4
	}

	switch e.f.Tuple {
	case isa.TupleFull:
		if e.opts.Broadcast {
			return elem
		}
		return vl
	case isa.TupleHalf:
		if e.opts.Broadcast {
			return elem
		}
		return vl / 2
	case isa.TupleFullMem:
		return vl
	case isa.Tuple1Scalar:
		// The scalar's own width, which the r/m slot's class states.
		if b := e.rmCls.Bits(); b > 0 && b <= 64 {
			return b / 8
		}
		return elem
	case isa.Tuple1Fixed:
		return elem
	case isa.Tuple2:
		if e.f.W == isa.W1 {
			return 16
		}
		return 8
	case isa.Tuple4:
		if e.f.W == isa.W1 {
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

// disp32 emits a four-byte displacement, recording a fixup when the
// displacement is a symbol rather than a number.
func (e *enc) disp32(m operand.Mem) {
	if m.Sym != nil {
		use := UseAbs
		if m.RIP {
			use = UsePCRel
		}
		e.fixup(Fixup{
			Size:   4,
			PCRel:  m.RIP,
			Use:    use,
			Kind:   m.Sym.Reloc(),
			Target: m.Sym,
			Addend: addendOf(m.Sym) + int64(m.Disp),
		})
	}
	var b [4]byte
	if m.Sym == nil {
		binary.LittleEndian.PutUint32(b[:], uint32(m.Disp))
	}
	e.emit(b[:]...)
}

func addendOf(t operand.Target) int64 {
	if s, ok := t.(operand.SymRef); ok {
		return s.Addend
	}
	return 0
}

// moffs emits the absolute address form, which has no ModRM at all: eight
// bytes of address and nothing to say about how to compute it.
func (e *enc) moffs() error {
	m := e.moffsv
	var b [8]byte
	switch m.kind {
	case kMem:
		if m.mem.HasBase || m.mem.HasIndex || m.mem.RIP {
			return ErrMoffsAddressing
		}
		if m.mem.Sym != nil {
			e.fixup(Fixup{
				Size:   8,
				Use:    UseAbs,
				Kind:   m.mem.Sym.Reloc(),
				Target: m.mem.Sym,
				Addend: addendOf(m.mem.Sym) + int64(m.mem.Disp),
			})
		} else {
			binary.LittleEndian.PutUint64(b[:], uint64(uint32(m.mem.Disp)))
		}
	default:
		return ErrMoffsAddressing
	}
	e.emit(b[:]...)
	return nil
}

var _ = reg.RAX