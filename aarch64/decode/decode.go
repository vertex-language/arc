package decode

import (
	"encoding/binary"

	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// Inst is one decoded instruction.
type Inst struct {
	// Word is the instruction, host-order.
	Word uint32

	// Form is the encoding this word matches. It is always the underlying
	// form, never an alias, because it is what encode.EncodeForm accepts and
	// an alias form's operand list is a different shape.
	Form *isa.Form

	// Ops are the operand values, in assembly-source order and in the concrete
	// types encode.Encode accepts.
	Ops []any

	// Alias is the alias form this word is preferably disassembled as, or nil.
	// Printing reads it; re-encoding ignores it.
	Alias *isa.Form
}

// Mnem is what a printer should write: the alias where one is preferred, the
// form's own mnemonic otherwise.
func (in Inst) Mnem() string {
	if in.Alias != nil {
		return in.Alias.Mnem
	}
	return in.Form.Mnem
}

// Len is four. It exists so a caller walking a buffer reads the same way it
// would on an architecture where it varies.
func (in Inst) Len() int { return 4 }

// Decode reads one instruction from the front of b.
//
// It reads what the architecture decodes, not what this assembler emits. There
// is no canonicalization pass and no re-encoding: a word that names a form is
// that form's, and arc dis prints it.
func Decode(b []byte) (Inst, error) {
	if len(b) < 4 {
		return Inst{}, ErrTruncated
	}
	return DecodeWord(binary.LittleEndian.Uint32(b[:4]))
}

// DecodeAll reads every instruction in b.
//
// A length that is not a multiple of four is an error before anything is
// decoded, rather than a partial result with a truncated tail.
func DecodeAll(b []byte) ([]Inst, error) {
	if len(b)%4 != 0 {
		return nil, ErrUnaligned
	}
	out := make([]Inst, 0, len(b)/4)
	for i := 0; i < len(b); i += 4 {
		in, err := Decode(b[i:])
		if err != nil {
			return out, err
		}
		out = append(out, in)
	}
	return out, nil
}

// DecodeWord decodes an instruction already read as a word.
func DecodeWord(word uint32) (Inst, error) {
	f, ok := get().find(word)
	if !ok {
		return Inst{}, &UnknownError{Word: word}
	}

	ops, err := operands(f, word)
	if err != nil {
		return Inst{}, err
	}

	return Inst{
		Word:  word,
		Form:  f,
		Ops:   ops,
		Alias: preferredAlias(f, word),
	}, nil
}

// operands walks the form's slots and rebuilds a value for each.
//
// Slots and operands are not in step, the same way they are not in encode/: a
// memory operand fills two slots, and a slot whose value the syntax omits fills
// none. The two indices are walked separately rather than zipped.
func operands(f *isa.Form, word uint32) ([]any, error) {
	var ops []any

	for si := 0; si < len(f.Slots); si++ {
		s := f.Slots[si]

		switch {
		case s.Class.Mem():
			var off *isa.Slot
			if si+1 < len(f.Slots) && f.Slots[si+1].Role == isa.RoleOffset {
				off = &f.Slots[si+1]
			}
			m, used, err := memOf(f, s, off, word)
			if err != nil {
				return nil, err
			}
			ops = append(ops, m)
			si += used - 1

		case s.Class.Reg():
			n := s.Field.Get(word)
			if s.Class == isa.ClassSys {
				sr := sysOf(n)
				if !sr.Movable() {
					return nil, &ClassError{f, len(ops), s.Field, n,
						"op0 below 2 is the sys and sysl space, not a register mrs can reach"}
				}
				ops = append(ops, sr)
				break
			}
			// An optional register whose field holds its default is what the
			// syntax omits: ret's x30 prints as "ret", not "ret x30".
			if s.Optional && n == s.Default {
				break
			}
			r, ok := regOf(s.Class, n)
			if !ok {
				return nil, &ClassError{f, len(ops), s.Field, n,
					"no register of this class has that number"}
			}
			ops = append(ops, r)

		case s.Class == isa.ClassImm:
			// An immediate that is a shift's amount is not an operand of its
			// own: it is read back out by shiftOf and printed as part of the
			// shift. Only a slot the syntax writes on its own becomes an op.
			if amountSlot(f, si) {
				break
			}
			v, extra, err := immOf(f, s, word)
			if err != nil {
				return nil, err
			}
			ops = append(ops, v)
			if extra != nil {
				ops = append(ops, extra)
			}

		case s.Class == isa.ClassLabel:
			v, _, err := immOf(f, s, word)
			if err != nil {
				return nil, err
			}
			ops = append(ops, v)

		case s.Class == isa.ClassShift:
			// A move-wide's hw field is consumed by the immediate that
			// computed it; emitting it again would double the operand.
			if immKindOf(f) == isa.ImmMoveWide {
				break
			}
			sh := shiftOf(f, s, word)
			if s.Optional && sh == operand.NoShift {
				break
			}
			ops = append(ops, sh)

		case s.Class == isa.ClassExtend:
			e := extendOf(f, s, word)
			if s.Optional && uint64(e.Op) == s.Default && e.Amount == 0 {
				break
			}
			ops = append(ops, e)

		case s.Class == isa.ClassCond:
			c := operand.Cond(s.Field.Get(word))
			if !c.Valid() {
				return nil, &ClassError{f, len(ops), s.Field, s.Field.Get(word),
					"not a condition code"}
			}
			ops = append(ops, c)

		case s.Class == isa.ClassBarrier:
			bar := operand.Barrier(s.Field.Get(word))
			if s.Optional && uint64(bar) == s.Default {
				break
			}
			ops = append(ops, bar)

		case s.Class == isa.ClassPrfOp:
			ops = append(ops, operand.PrfOp(s.Field.Get(word)))
		}
	}
	return ops, nil
}

// amountSlot reports whether an immediate slot holds a shift or extend amount
// rather than an operand of its own.
func amountSlot(f *isa.Form, si int) bool {
	for _, s := range f.Slots {
		switch s.Class {
		case isa.ClassShift:
			if s.Field.Width() == 2 && f.Slots[si].Imm == isa.ImmPlain {
				return true
			}
		case isa.ClassExtend:
			if f.Attrs&isa.AttrScaled == 0 && f.Slots[si].Imm == isa.ImmPlain {
				return true
			}
		}
	}
	return false
}

func immKindOf(f *isa.Form) isa.ImmKind {
	for _, s := range f.Slots {
		if s.Class == isa.ClassImm && s.Imm != isa.ImmNone {
			return s.Imm
		}
	}
	return isa.ImmNone
}