// x86_64/encode/prefix.go
package encode

import (
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/reg"
)

var segmentPrefix = [6]byte{
	reg.ES: 0x26, reg.CS: 0x2e, reg.SS: 0x36,
	reg.DS: 0x3e, reg.FS: 0x64, reg.GS: 0x65,
}

// prefixes emits everything ahead of the opcode: the legacy prefix bytes,
// then REX, VEX or EVEX.
//
// The architecture accepts the legacy prefixes in any order. Byte-identity
// with GNU as does not, so the order below is fixed and the differential
// suite is what pins it: segment, operand size, address size, mandatory
// SIMD prefix, REX. A change here is a change the suite will notice.
func (e *enc) prefixes() {
	if seg, ok := e.segment(); ok {
		e.emit(seg)
	}

	switch e.f.Enc {
	case isa.EncLegacy:
		if e.f.Attrs&isa.Data16 != 0 {
			e.emit(0x66)
		}
		if e.address32() {
			e.emit(0x67)
		}
		// The mandatory prefix of an SSE form. It is the same byte as an
		// operand-size override when it is 0x66 and a different thing
		// entirely; conflating them is how a decoder mis-reads MOVDQA.
		if p := e.f.Pfx.Byte(); p != 0 {
			e.emit(p)
		}
		if rex, ok := e.rex(); ok {
			e.emit(rex)
		}
	case isa.EncVEX:
		if e.address32() {
			e.emit(0x67)
		}
		e.vex()
	case isa.EncEVEX:
		if e.address32() {
			e.emit(0x67)
		}
		e.evex()
	}
}

func (e *enc) segment() (byte, bool) {
	m := e.memOperand()
	if m == nil || !m.HasSeg {
		return 0, false
	}
	return segmentPrefix[m.Seg], true
}

func (e *enc) address32() bool {
	m := e.memOperand()
	return m != nil && m.Addr32
}

// rex builds the REX byte, or reports that none is needed. A REX of 0x40 —
// all four bits clear — is emitted only when a byte register requires it;
// otherwise it is a wasted byte and this target has enough of those.
func (e *enc) rex() (byte, bool) {
	var w, r, x, b byte

	if e.f.W == isa.W1 {
		w = 1
	}
	r = byte(e.regv.num() >> 3)

	if e.rmv != nil {
		switch e.rmv.kind {
		case kReg:
			b = e.rmv.num() >> 3
		case kMem:
			if e.rmv.mem.HasBase {
				b = e.rmv.mem.Base.Num() >> 3
			}
			if e.rmv.mem.HasIndex {
				x = e.rmv.mem.Index.Num() >> 3
			}
		}
	}
	if e.plusr != nil {
		b = e.plusr.num() >> 3
	}

	if w|r|x|b != 0 {
		return 0x40 | w<<3 | r<<2 | x<<1 | b, true
	}
	// SPL, BPL, SIL and DIL exist only under REX. Their encodings are AH,
	// CH, DH and BH without it, so the empty prefix is load-bearing.
	for _, v := range e.operands() {
		if v.kind != kReg {
			continue
		}
		if rr, ok := v.reg.(interface{ RexRequired() bool }); ok && rr.RexRequired() {
			return 0x40, true
		}
	}
	return 0, false
}

func (e *enc) needsREX() bool {
	_, ok := e.rex()
	return ok
}

// opcode emits the map escape and the opcode byte. VEX and EVEX carry the
// map in a field, so the escape bytes are legacy-only.
func (e *enc) opcode() {
	if e.f.Enc == isa.EncLegacy {
		e.emit(e.f.Map.Escape()...)
	}
	op := e.f.Opcode
	if e.f.Attrs&isa.PlusReg != 0 {
		op |= e.plusr.num() & 7
	}
	e.emit(op)
}