package encode

import (
	"fmt"

	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// ModRM and SIB construction, Intel SDM Vol. 2, Tables 2-2 and 2-3.
//
// Four encodings in this table do not mean what their fields say, and every
// one of them is a place an assembler gets quietly wrong:
//
//   - mod=00, rm=101 is not [EBP]. It is disp32 with no base. In 32-bit mode
//     that is plain absolute addressing; 64-bit mode redefined the same
//     encoding as RIP-relative, which is why this file has no IP-relative
//     path and x86_64's does.
//   - Because that slot is taken, [EBP] with no displacement is not
//     encodable as written and comes out as mod=01 with an explicit zero
//     disp8. One byte longer, and not optional.
//   - rm=100 is not [ESP]. It means a SIB byte follows. ESP as a base is
//     therefore always a SIB encoding.
//   - SIB.index=100 means no index, which is why ESP has no index encoding at
//     all — rejected in operand/ at construction rather than here.
//   - SIB.base=101 with mod=00 means no base, and disp32 follows. This is how
//     an index-only address is written.

type encodedRM struct {
	bytes []byte

	// fixup describes the displacement field when it holds a symbol, with
	// Offset relative to the start of bytes.
	fixup   *Fixup
	fixupAt int
}

func modrm(rm operand.Operand, regField uint8, haveReg bool) (encodedRM, error) {
	if !haveReg {
		regField = 0
	}

	// A register r/m is mod=11 and nothing else.
	if r, ok := rm.(reg.Reg); ok {
		if _, isMem := rm.(operand.Memory); !isMem {
			return encodedRM{bytes: []byte{0xc0 | regField<<3 | r.Num()&7}}, nil
		}
	}

	m, ok := rm.(operand.Memory)
	if !ok {
		return encodedRM{}, fmt.Errorf("%w: %T cannot be an r/m operand", ErrEncode, rm)
	}
	if err := m.Err(); err != nil {
		return encodedRM{}, err
	}

	base, hasBase := m.Base()
	index, scale, hasIndex := m.IndexReg()
	disp := m.Displacement()
	sym, hasSym := m.Symbol()

	if hasSym {
		disp = sym.Addend()
	}

	var (
		out     []byte
		dispLen int
	)

	needSIB := hasIndex || (hasBase && base == reg.ESP) || (!hasBase && hasIndex)

	switch {
	// Absolute: no base, no index. mod=00, rm=101, disp32.
	case !hasBase && !hasIndex:
		out = append(out, 0x00|regField<<3|0x05)
		dispLen = 4

	// Index only: mod=00, rm=100, SIB with base=101, disp32.
	case !hasBase && hasIndex:
		out = append(out, 0x00|regField<<3|0x04)
		out = append(out, sib(scale, index.Num(), 0x05))
		dispLen = 4

	default:
		// A base is present. Choose the displacement size first, because the
		// EBP exception depends on it and the mod bits encode it.
		mod, dl := dispMode(disp, hasSym, base)

		if needSIB {
			out = append(out, mod|regField<<3|0x04)
			idx := uint8(0x04) // no index
			sc := uint8(1)
			if hasIndex {
				idx, sc = index.Num(), scale
			}
			out = append(out, sib(sc, idx, base.Num()))
		} else {
			out = append(out, mod|regField<<3|base.Num())
		}
		dispLen = dl
	}

	e := encodedRM{fixupAt: len(out)}

	if hasSym {
		if dispLen != 4 {
			// A symbol needs a full 32-bit field; dispMode forces mod=10 for
			// exactly this reason, so reaching here is a bug rather than a
			// user error.
			return encodedRM{}, fmt.Errorf("%w: symbol displacement sized %d bytes", ErrEncode, dispLen)
		}
		e.fixup = &Fixup{
			Kind:   FixupReloc,
			Size:   4,
			Name:   sym.Name(),
			Reloc:  sym.Kind(),
			Addend: sym.Addend(),
		}
		out = appendLE(out, 0, 4)
	} else {
		out = appendLE(out, uint64(uint32(disp)), dispLen)
	}

	e.bytes = out
	return e, nil
}

// dispMode picks the mod bits and displacement width for a based address.
func dispMode(disp int32, hasSym bool, base reg.R32) (mod uint8, length int) {
	switch {
	// A symbol is always a 32-bit field: its value is not known yet, so it
	// cannot be proved to fit in a byte.
	case hasSym:
		return 0x80, 4

	// [EBP] cannot be written with mod=00, because that slot is disp32.
	case disp == 0 && base != reg.EBP:
		return 0x00, 0

	case disp >= -128 && disp <= 127:
		return 0x40, 1

	default:
		return 0x80, 4
	}
}

// sib builds the scale-index-base byte. scale is the multiplier, not its log.
func sib(scale, index, base uint8) byte {
	var ss uint8
	switch scale {
	case 1, 0:
		ss = 0
	case 2:
		ss = 1
	case 4:
		ss = 2
	case 8:
		ss = 3
	}
	return ss<<6 | (index&7)<<3 | base&7
}