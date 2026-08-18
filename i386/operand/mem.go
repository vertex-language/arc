package operand

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/i386/reg"
)

// i386 effective addresses, Intel SDM Vol. 2, Tables 2-2 and 2-3.
//
//	[base]
//	[base + disp]
//	[base + index*scale]
//	[base + index*scale + disp]
//	[index*scale + disp]              no base
//	[disp32]                          absolute
//
// Three encoding facts constrain what can be built here, and this package
// rejects at construction what the encoder could not emit:
//
//   - SIB.index = 100b means "no index", so ESP can never be an index. It can
//     be a base.
//   - scale is two bits: 1, 2, 4 or 8.
//   - the address size is 32 bits. 16-bit effective addresses exist in 32-bit
//     code behind the 0x67 address-size override, but they are a different
//     ModRM table with no SIB and no scaling, and arc's i386 is protected mode
//     with no .code16. Absent rather than half-supported.
//
// There is no instruction-pointer-relative form. In protected mode the
// mod=00 rm=101 encoding is a plain disp32 and only the displacement is used;
// 64-bit mode redefined that same encoding as RIP-relative. This is why i386
// position-independent code goes through R_386_GOTPC and R_386_GOTOFF with the
// GOT pointer materialised in EBX, where x86-64 has R_X86_64_GOTPCREL and a
// single addressing mode. There is deliberately no RIPRel constructor to find.

// ErrOperand is the sentinel for an operand that cannot be encoded. A memory
// operand carries its own error rather than returning one, so that a chain of
// builders stays a chain; the assembler reports it at the call that used it.
var ErrOperand = errors.New("operand")

type mem struct {
	reg.Seal

	width uint16

	base     reg.R32
	hasBase  bool
	index    reg.R32
	hasIndex bool
	scale    uint8

	disp int32

	ref    SymRef
	hasRef bool

	seg    reg.Sreg
	hasSeg bool

	err error
}

func based(width uint16, base reg.R32) mem {
	return mem{width: width, base: base, hasBase: true}
}

func absolute(width uint16) mem { return mem{width: width} }

func (m mem) disp_(d int32) mem {
	m.disp = d
	return m
}

func (m mem) index_(r reg.R32, scale uint8) mem {
	if r == reg.ESP {
		m.err = fmt.Errorf("%w: esp cannot be an index register\n  note: SIB.index=100b encodes \"no index\"", ErrOperand)
		return m
	}
	switch scale {
	case 1, 2, 4, 8:
	default:
		m.err = fmt.Errorf("%w: scale %d is not 1, 2, 4 or 8", ErrOperand, scale)
		return m
	}
	m.index, m.scale, m.hasIndex = r, scale, true
	return m
}

func (m mem) sym_(r SymRef) mem {
	m.ref, m.hasRef = r, true
	return m
}

func (m mem) seg_(s reg.Sreg) mem {
	m.seg, m.hasSeg = s, true
	return m
}

// Accessors. encode/ builds ModRM and SIB from these; this package classifies
// nothing, because the classification is the encoder's table.
//
// These are named apart from the builders of the same shape (Disp, Index,
// Sym on M8..M512 in width.go) rather than overloaded onto them: a method
// defined directly on M8 shadows a promoted method of the same name from the
// embedded mem struct regardless of signature, so "Disp" cannot be both
// "set the displacement and return M8" and "read the displacement back" at
// once. Base has no builder counterpart and needs no such split; Seg already
// followed this pattern opposite Segment, and IndexReg/Displacement/Symbol
// follow it now too.

func (m mem) Bits() int  { return int(m.width) }
func (m mem) Err() error { return m.err }

func (m mem) Base() (reg.R32, bool)            { return m.base, m.hasBase }
func (m mem) IndexReg() (reg.R32, uint8, bool) { return m.index, m.scale, m.hasIndex }
func (m mem) Displacement() int32              { return m.disp }
func (m mem) Symbol() (SymRef, bool)           { return m.ref, m.hasRef }
func (m mem) Seg() (reg.Sreg, bool)            { return m.seg, m.hasSeg }

// DefaultSeg is the segment the processor uses when no override prefix is
// present: SS when the base is ESP or EBP, DS otherwise. An override that
// names the default is still emitted if asked for, because it changes bytes.
func (m mem) DefaultSeg() reg.Sreg {
	if m.hasBase && (m.base == reg.ESP || m.base == reg.EBP) {
		return reg.SS
	}
	return reg.DS
}

func (m mem) String() string {
	s := ""
	if m.hasSeg {
		s += "%" + m.seg.Name() + ":"
	}
	if m.hasRef {
		s += m.ref.String()
	} else if m.disp != 0 || (!m.hasBase && !m.hasIndex) {
		s += fmt.Sprintf("%d", m.disp)
	}
	if !m.hasBase && !m.hasIndex {
		return s
	}
	s += "("
	if m.hasBase {
		s += "%" + m.base.Name()
	}
	if m.hasIndex {
		s += fmt.Sprintf(",%%%s,%d", m.index.Name(), m.scale)
	}
	return s + ")"
}