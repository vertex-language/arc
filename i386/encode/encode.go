// Package encode turns a resolved form and operand values into bytes.
//
// It is a pure function over resolved forms: it takes an *isa.Form and a list
// of operands and returns bytes and fixups. Nothing survives the call. It
// never sees text, never sees a dialect, never chooses a different
// instruction, and never reorders anything — the form was chosen before it was
// called, and this package only lays out the fields that form declares.
//
// encode/ exists as a package on i386 and x86_64 and nowhere else in the tree.
// On a fixed-width arch, encoding is bitfield packing against the form table
// and stays in asm.go; there is no separate machine to name. Here there is:
// prefix selection, ModRM and SIB construction, displacement sizing and
// immediate width are a machine of their own, and they have no mirror in the
// builder API.
package encode

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// ErrEncode is the sentinel for an operand combination this form cannot hold.
var ErrEncode = errors.New("encode")

// FixupKind distinguishes the two things that get patched later.
type FixupKind uint8

const (
	// FixupLabel is a reference to a name in the same section. It resolves at
	// Serialize as a direct patch and produces no relocation record.
	FixupLabel FixupKind = iota

	// FixupReloc is a reference that leaves the section. It produces a
	// relocation record carrying Kind.
	FixupReloc
)

// Fixup is a field this instruction left for the assembler to fill.
//
// Offset and Size locate the field within the instruction. Adjust is the
// field-position correction: for a PC-relative field the processor computes
// the target from the address of the *next* instruction, so the value written
// is target - (fieldAddress + Adjust'), and Adjust carries the -(len-offset)
// that accounts for it.
//
// This is the -4 that gets written by hand against objectfile/elf for a rel32.
// It is computed here because this is the code that placed the field and knows
// how many bytes follow it.
type Fixup struct {
	Kind   FixupKind
	Offset int
	Size   int
	PCRel  bool
	Adjust int32

	Name   string
	Reloc  operand.RelocKind
	Addend int32
}

// Inst is one encoded instruction.
type Inst struct {
	Bytes  []byte
	Fixups []Fixup
}

// Len is the encoded length in bytes. Emit encodes every candidate form and
// takes the shortest; there is no separate size estimator to disagree with the
// encoder.
func (i Inst) Len() int { return len(i.Bytes) }

// Encode lays out one form with the given operands.
//
// The form is assumed already resolved and already permitted by the feature
// set: isa.Resolve answered both questions, and re-asking them here would be a
// second place for the answer to live.
func Encode(f *isa.Form, ops []operand.Operand) (Inst, error) {
	if len(ops) != len(f.Ops) {
		return Inst{}, fmt.Errorf("%w: %s takes %d operands, got %d",
			ErrEncode, f.Signature(), len(f.Ops), len(ops))
	}

	var (
		out    []byte
		fixups []Fixup

		regField  uint8
		haveReg   bool
		rmOperand operand.Operand
		haveRM    bool

		opcodeAdd uint8
		haveAdd   bool

		imms []immField
		rels []relField
	)

	// Pass one: sort operands into the fields they occupy. Nothing is emitted
	// yet, because the prefixes depend on the memory operand and the ModRM
	// byte depends on both halves.
	for i, o := range ops {
		switch f.Ops[i].Slot {
		case isa.SlotReg:
			n, err := regNum(o)
			if err != nil {
				return Inst{}, err
			}
			regField, haveReg = n, true

		case isa.SlotRM:
			rmOperand, haveRM = o, true

		case isa.SlotOpcode:
			n, err := regNum(o)
			if err != nil {
				return Inst{}, err
			}
			opcodeAdd, haveAdd = n, true

		case isa.SlotImm:
			w, err := immWidth(f.Ops[i].Class)
			if err != nil {
				return Inst{}, err
			}
			v, ok := o.(operand.Imm)
			if !ok {
				return Inst{}, fmt.Errorf("%w: %s expects an immediate", ErrEncode, f.Signature())
			}
			imms = append(imms, immField{value: int64(v), width: w})

		case isa.SlotRel:
			w := 4
			if f.Ops[i].Class == isa.Rel8 {
				w = 1
			}
			rels = append(rels, relField{op: o, width: w})

		case isa.SlotFixed:
			// Named in the syntax, encoded nowhere.
		}
	}

	// A /digit occupies ModRM.reg, so a form cannot have both.
	if f.Ext >= 0 {
		regField, haveReg = uint8(f.Ext), true
	}

	// Prefixes. Group 2 (segment override) precedes group 3 (operand size);
	// the processor accepts any order but only one spelling comes out of arc.
	if haveRM {
		if m, ok := rmOperand.(operand.Memory); ok {
			if err := m.Err(); err != nil {
				return Inst{}, err
			}
			if s, ok := m.Seg(); ok {
				out = append(out, segPrefix(s))
			}
		}
	}
	if f.OpSize16 {
		out = append(out, 0x66)
	}

	// Opcode.
	opcode := append([]byte(nil), f.Opcode...)
	if haveAdd {
		opcode[len(opcode)-1] += opcodeAdd
	}
	out = append(out, opcode...)

	// ModRM, SIB and displacement.
	if haveRM {
		mrm, err := modrm(rmOperand, regField, haveReg)
		if err != nil {
			return Inst{}, err
		}
		out = append(out, mrm.bytes...)
		if mrm.fixup != nil {
			fx := *mrm.fixup
			fx.Offset += len(out) - len(mrm.bytes) + mrm.fixupAt
			fixups = append(fixups, fx)
		}
	} else if haveReg && f.Ext >= 0 {
		return Inst{}, fmt.Errorf("%w: %s has /%d but no r/m operand", ErrEncode, f.Signature(), f.Ext)
	}

	// Immediates.
	for _, im := range imms {
		out = appendLE(out, uint64(im.value), im.width)
	}

	// Branch displacements. The field is the last thing in the instruction
	// for every form that has one, which is why Adjust is always -width here;
	// it is computed from the layout rather than assumed, below.
	for _, rl := range rels {
		off := len(out)
		fx := Fixup{Offset: off, Size: rl.width, PCRel: true}
		switch v := rl.op.(type) {
		case operand.Label:
			fx.Kind = FixupLabel
			fx.Name = string(v)
		case operand.SymRef:
			fx.Kind = FixupReloc
			fx.Name = v.Name()
			fx.Reloc = v.Kind()
			fx.Addend = v.Addend()
		default:
			return Inst{}, fmt.Errorf("%w: %s expects a label or symbol", ErrEncode, f.Signature())
		}
		out = appendLE(out, 0, rl.width)
		fixups = append(fixups, fx)
	}

	// The field-position correction, computed now that the length is known.
	// A PC-relative field is resolved against the end of the instruction, so
	// the correction is the number of bytes that follow the field.
	for i := range fixups {
		if fixups[i].PCRel {
			fixups[i].Adjust = -int32(len(out) - fixups[i].Offset - fixups[i].Size)
			fixups[i].Adjust -= int32(fixups[i].Size)
		}
	}

	return Inst{Bytes: out, Fixups: fixups}, nil
}

type immField struct {
	value int64
	width int
}

type relField struct {
	op    operand.Operand
	width int
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
	return 0, fmt.Errorf("%w: %s is not an immediate class", ErrEncode, c)
}

func appendLE(b []byte, v uint64, width int) []byte {
	for i := 0; i < width; i++ {
		b = append(b, byte(v>>(8*i)))
	}
	return b
}

// regNum is the 0-7 encoding number of any register operand.
func regNum(o operand.Operand) (uint8, error) {
	if r, ok := o.(reg.Reg); ok {
		return r.Num(), nil
	}
	return 0, fmt.Errorf("%w: %T is not a register", ErrEncode, o)
}

// segPrefix is the override byte for a segment. The order of these bytes is
// not the order of the segment encoding numbers, which is why this is a switch
// and not arithmetic on Sreg.Num().
func segPrefix(s reg.Sreg) byte {
	switch s {
	case reg.ES:
		return 0x26
	case reg.CS:
		return 0x2e
	case reg.SS:
		return 0x36
	case reg.DS:
		return 0x3e
	case reg.FS:
		return 0x64
	case reg.GS:
		return 0x65
	}
	return 0
}