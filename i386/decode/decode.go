// Package decode turns bytes into instructions.
//
// This is a separate machine from encode/, and it shares exactly one thing
// with it: the form table. There is no second opcode map here written by
// hand — index.go builds the SDM's maps by walking isa.All(), so a form that
// arc build can emit is a form arc dis can read, and neither can drift from
// the other without the table moving underneath both.
//
// What makes it a separate machine is that nothing above is invertible by
// inspection. The encoder is told a form and lays out its fields; the decoder
// is told a byte and must work out which of several forms declared it, which
// prefixes are prefixes and which are part of an opcode, whether rm=100 is a
// register or an announcement that a SIB byte follows, and how many bytes the
// displacement it has not read yet will turn out to be. Those questions have
// no mirror in the builder API at all.
//
// It never sees text and has no notion of a dialect. arc dis is Decode then
// PrintInst, and the split is the enforcement of "a dialect is a spelling,
// never a byte": everything in this package is bytes, and the only thing that
// takes a dialect returns text. That is also why Inst has no String method.
// Printing an instruction is a dialect's job and this package cannot do it
// without picking one.
//
// decode/ exists as a package on i386 and x86_64 and nowhere else in the
// tree. On a fixed-width arch, decoding is bitfield unpacking against the
// form table and stays in asm.go; there is no separate machine to name.
package decode

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// ErrDecode is the sentinel for bytes that are not an instruction this target
// encodes. ErrTruncated and ErrUnknown wrap it, so errors.Is(err, ErrDecode)
// holds for every error this package returns.
var ErrDecode = errors.New("decode")

// ErrTruncated is returned when the instruction runs past the end of the
// input. It is distinct from ErrUnknown because a disassembler walking the
// tail of a section needs to tell "not an instruction" from "not all here".
var ErrTruncated = fmt.Errorf("%w: truncated instruction", ErrDecode)

// ErrUnknown is returned for an opcode no declared form claims.
var ErrUnknown = fmt.Errorf("%w: unknown opcode", ErrDecode)

// maxInstLen is the architectural limit. Anything longer is not a long
// instruction, it is a run of prefixes that will #GP on real silicon, and
// accepting it would let a bad byte stream consume a whole section.
const maxInstLen = 15

// Decode reads one instruction from the front of b.
//
// The feature set is the same one the encoder gates on, applied the same way:
// bytes whose only matching form is gated are an error naming the flag that
// would allow them, not a silent decode. This is what keeps arc dis -t
// i386-elf --features i486 from claiming a CMOVcc that would #UD.
func Decode(b []byte, s feature.Set) (Inst, error) {
	i, _, err := walk(b, s)
	return i, err
}

// walk is the one pass. Decode discards the field decomposition and Explain
// keeps it; there is no second traversal that could disagree with the first,
// for the same reason there is no size estimator beside the encoder.
func walk(b []byte, s feature.Set) (Inst, []Field, error) {
	w := &walker{b: b, s: s}

	p, err := w.prefixes()
	if err != nil {
		return Inst{}, nil, err
	}

	m, op, opStart, err := w.opcode()
	if err != nil {
		return Inst{}, nil, err
	}

	// The ModRM byte is peeked, not consumed: selection needs its reg field
	// to resolve a /digit and its mod field to tell a register r/m from a
	// memory one, and until a form is selected there is nothing that says
	// whether the byte is a ModRM at all.
	modrm, haveModRM := w.peek()

	e, plusReg, err := lookup(m, op, s, p.OpSize, modrm, haveModRM)
	if err != nil {
		return Inst{}, nil, err
	}
	f := e.form

	w.span(FieldOpcode, opStart, w.pos-opStart, "opcode", f.Signature(), e.opcodeNote(plusReg))

	// One ModRM byte serves both halves of the instruction, so it is read
	// once here and its two field numbers handed to the operand loop.
	var (
		rmOp   operand.Operand
		regNum uint8
	)
	if e.modrm {
		rmClass := isa.ClassNone
		if e.rmSlot >= 0 {
			rmClass = f.Ops[e.rmSlot].Class
		}
		rmOp, regNum, err = w.rm(rmClass, e, p)
		if err != nil {
			return Inst{}, nil, err
		}
	}

	inst := Inst{Form: f, Prefixes: p, Ops: make([]operand.Operand, len(f.Ops))}

	for i, o := range f.Ops {
		switch o.Slot {
		case isa.SlotRM:
			inst.Ops[i] = rmOp

		case isa.SlotReg:
			v, err := regOperand(o.Class, regNum)
			if err != nil {
				return Inst{}, nil, err
			}
			inst.Ops[i] = v

		case isa.SlotOpcode:
			v, err := regOperand(o.Class, plusReg)
			if err != nil {
				return Inst{}, nil, err
			}
			inst.Ops[i] = v

		case isa.SlotFixed:
			v, err := fixedOperand(o.Class)
			if err != nil {
				return Inst{}, nil, err
			}
			inst.Ops[i] = v

		case isa.SlotImm:
			v, err := w.imm(o.Class)
			if err != nil {
				return Inst{}, nil, err
			}
			inst.Ops[i] = v

		case isa.SlotRel:
			d, err := w.rel(o.Class)
			if err != nil {
				return Inst{}, nil, err
			}
			inst.Rel, inst.HasRel = d, true
			inst.Ops[i] = operand.Imm(d)
		}
	}

	if w.pos > maxInstLen {
		return Inst{}, nil, fmt.Errorf("%w: %d bytes; the architectural limit is %d",
			ErrDecode, w.pos, maxInstLen)
	}

	inst.Bytes = b[:w.pos]
	return inst, w.fields, nil
}

// walker is the cursor. Every read is bounds-checked and every consumed byte
// lands in exactly one field, which is the invariant Explain rests on.
type walker struct {
	b      []byte
	s      feature.Set
	pos    int
	fields []Field
}

func (w *walker) u8() (byte, error) {
	if w.pos >= len(w.b) {
		return 0, ErrTruncated
	}
	c := w.b[w.pos]
	w.pos++
	return c, nil
}

func (w *walker) peek() (byte, bool) {
	if w.pos >= len(w.b) {
		return 0, false
	}
	return w.b[w.pos], true
}

// le reads n little-endian bytes. Every multi-byte field on x86 is
// little-endian, including the ones that hold an address.
func (w *walker) le(n int) (uint32, error) {
	if w.pos+n > len(w.b) {
		return 0, ErrTruncated
	}
	var v uint32
	for i := 0; i < n; i++ {
		v |= uint32(w.b[w.pos+i]) << (8 * i)
	}
	w.pos += n
	return v, nil
}

func (w *walker) span(k FieldKind, off, n int, name, value, note string, bits ...BitField) {
	w.fields = append(w.fields, Field{
		Kind:   k,
		Name:   name,
		Offset: off,
		Len:    n,
		Bytes:  w.b[off : off+n],
		Value:  value,
		Note:   note,
		Bits:   bits,
	})
}

// prefixes consumes the legacy prefix bytes.
//
// The four groups may appear in any order and the processor accepts any
// permutation; arc emits one spelling and reads all of them. Repeats within a
// group are undefined on silicon, so the last one wins here rather than being
// rejected — a decoder that refused would refuse bytes that execute.
//
// When the SSE tranche lands, 0x66, 0xf2 and 0xf3 stop being only prefixes:
// they become mandatory prefixes that select an opcode. That is why they are
// recorded on Prefixes and handed to lookup rather than discarded here.
func (w *walker) prefixes() (Prefixes, error) {
	var p Prefixes
	for {
		c, ok := w.peek()
		if !ok {
			return p, ErrTruncated
		}

		off := w.pos
		var name, note string

		switch c {
		case 0xf0:
			p.Lock = true
			name, note = "lock", "bus lock"
		case 0xf2:
			p.RepNE, p.Rep = true, false
			name, note = "repne", "repeat while not equal"
		case 0xf3:
			p.Rep, p.RepNE = true, false
			name, note = "rep", "repeat"
		case 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65:
			p.Seg, p.HasSeg = segOf(c), true
			name, note = "segment", p.Seg.Name()+" override"
		case 0x66:
			p.OpSize = true
			name, note = "opsize", "16-bit operand size"
		case 0x67:
			// The address-size override selects the 16-bit ModRM table: no
			// SIB, no scaling, a different set of base and index registers.
			// operand/ does not model it, so decode does not invent it.
			return p, fmt.Errorf("%w: address-size override 0x67\n"+
				"  note: 0x67 selects the 16-bit addressing table, which arc's i386 does not model\n"+
				"  note: arc's i386 is protected mode; there is no .code16", ErrDecode)
		default:
			return p, nil
		}

		w.pos++
		if w.pos >= maxInstLen {
			return p, fmt.Errorf("%w: more than %d prefix bytes", ErrDecode, maxInstLen-1)
		}
		w.span(FieldPrefix, off, 1, name, hex(c), note)
	}
}

// opcode consumes the opcode bytes and names the map they came from.
func (w *walker) opcode() (m *[256][]*entry, op byte, start int, err error) {
	start = w.pos

	c, err := w.u8()
	if err != nil {
		return nil, 0, start, err
	}
	if c != 0x0f {
		return &map1, c, start, nil
	}

	c, err = w.u8()
	if err != nil {
		return nil, 0, start, err
	}
	switch c {
	case 0x38:
		c, err = w.u8()
		return &map0F38, c, start, err
	case 0x3a:
		c, err = w.u8()
		return &map0F3A, c, start, err
	}
	return &map0F, c, start, nil
}

// imm reads an immediate.
//
// Imm8S sign-extends and the rest do not: the sign-extended byte is a
// different form from the four-byte immediate and four bytes shorter, which
// is the whole reason the two classes exist.
func (w *walker) imm(c isa.Class) (operand.Imm, error) {
	n, err := immWidth(c)
	if err != nil {
		return 0, err
	}
	off := w.pos
	raw, err := w.le(n)
	if err != nil {
		return 0, err
	}

	var v operand.Imm
	switch {
	case c == isa.Imm8S:
		v = operand.Imm(int8(raw))
	case n == 1:
		v = operand.Imm(uint8(raw))
	case n == 2:
		v = operand.Imm(uint16(raw))
	default:
		v = operand.Imm(raw)
	}

	w.span(FieldImm, off, n, c.String(), v.String(), hexN(raw, n))
	return v, nil
}

// rel reads a branch displacement, which is always signed and always relative
// to the end of the instruction.
func (w *walker) rel(c isa.Class) (int32, error) {
	n := 4
	if c == isa.Rel8 {
		n = 1
	}
	off := w.pos
	raw, err := w.le(n)
	if err != nil {
		return 0, err
	}

	d := int32(raw)
	if n == 1 {
		d = int32(int8(raw))
	}

	w.span(FieldRel, off, n, c.String(), fmt.Sprintf("%+d", d),
		"relative to the end of the instruction")
	return d, nil
}

func immWidth(c isa.Class) (int, error) {
	switch c {
	case isa.Imm8, isa.Imm8S:
		return 1, nil
	case isa.Imm16:
		return 2, nil
	case isa.Imm32:
		return 4, nil
	}
	return 0, fmt.Errorf("%w: %s is not an immediate class", ErrDecode, c)
}

// segOf is the inverse of encode's segPrefix. The order of these bytes is not
// the order of the segment encoding numbers, which is why both directions are
// a switch and not arithmetic.
func segOf(c byte) reg.Sreg {
	switch c {
	case 0x26:
		return reg.ES
	case 0x2e:
		return reg.CS
	case 0x36:
		return reg.SS
	case 0x3e:
		return reg.DS
	case 0x64:
		return reg.FS
	case 0x65:
		return reg.GS
	}
	return reg.DS
}

func hex(b byte) string { return fmt.Sprintf("0x%02x", b) }

func hexN(v uint32, n int) string {
	switch n {
	case 1:
		return fmt.Sprintf("0x%02x", uint8(v))
	case 2:
		return fmt.Sprintf("0x%04x", uint16(v))
	}
	return fmt.Sprintf("0x%08x", v)
}