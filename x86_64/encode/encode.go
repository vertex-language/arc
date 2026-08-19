// x86_64/encode/encode.go
//
// Package encode turns a resolved form and operand values into bytes and
// fixups. It is a pure function: no state, no feature set, no section, no
// symbol table. isa.Resolve has already chosen the form and feature gating
// has already happened; nothing here can reach a different instruction than
// the one it was handed.
//
// This package imports isa/, operand/ and reg/, and switches on concrete
// operand types. It does not import the root package, which is why it names
// no relocation constants — see fixup.go for what it produces instead.
package encode

import (
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Opts are the encoding modifiers that are not operands: the EVEX bits a
// caller sets alongside the operands rather than in place of one.
//
// Masking is an operand — the form declares a slot in EVEX.aaa and the caller
// passes a reg.K — because a writemask names a register. Zeroing, broadcast
// and rounding are not: they are one bit each with no register behind them,
// and operand/ has no type that carries them.
type Opts struct {
	// Zero selects zeroing-masking. Legal only on a form with isa.Zeroing,
	// and only alongside a nonzero mask: {z} without {k} is #UD.
	Zero bool

	// Broadcast selects embedded broadcast, EVEX.b over a memory operand.
	// The form's Elem says how wide an element is; this says to use it.
	Broadcast bool

	// Round is embedded rounding control, EVEX.b over a register operand
	// with L'L reused as the mode. Legal only on a form with isa.RoundCtl,
	// and only at 512 bits — the length field is carrying the mode instead.
	Round RoundMode

	// SAE suppresses all exceptions without naming a rounding mode.
	SAE bool
}

// RoundMode is embedded rounding control.
type RoundMode uint8

const (
	RoundNone    RoundMode = iota
	RoundNearest           // {rn-sae}
	RoundDown              // {rd-sae}
	RoundUp                // {ru-sae}
	RoundZero              // {rz-sae}
)

func (r RoundMode) bits() byte { return byte(r - 1) }

// Encode is the whole job: form plus operands to bytes plus fixups.
//
// Operands are in Intel order — destination first — and correspond
// one-to-one with the form's explicit slots. An implicit operand is not
// passed; the form declared it and the opcode already names it.
func Encode(f *isa.Form, ops ...any) ([]byte, []Fixup, error) {
	return EncodeWith(f, Opts{}, ops...)
}

// EncodeWith is Encode with the EVEX modifiers set.
func EncodeWith(f *isa.Form, o Opts, ops ...any) ([]byte, []Fixup, error) {
	vs, err := lower(ops)
	if err != nil {
		return nil, nil, err
	}

	e := &enc{f: f, opts: o}
	if err := e.bind(vs); err != nil {
		return nil, nil, err
	}
	if err := e.check(); err != nil {
		return nil, nil, err
	}

	e.prefixes()
	e.opcode()
	if err := e.addressing(); err != nil {
		return nil, nil, err
	}
	if err := e.immediate(); err != nil {
		return nil, nil, err
	}
	e.close()

	return e.buf, e.fix, nil
}

// enc is one instruction under construction. It exists for the duration of
// one Encode call and is not reachable from outside it.
type enc struct {
	f    *isa.Form
	opts Opts

	buf []byte
	fix []Fixup

	// The operands bound to their encoding fields. A nil pointer means the
	// form has no slot there.
	rmv    *val // ModRM.rm — a register or a memory reference
	regv   *val // ModRM.reg
	vvvv   *val // VEX.vvvv / EVEX.vvvv
	plusr  *val // the low three opcode bits
	immv   *val // the immediate field
	is4v   *val // imm8[7:4]
	maskv  *val // EVEX.aaa
	moffsv *val

	// rmCls is the class of the slot that landed in ModRM.rm, which is what
	// the disp8*N scale factor is computed from for a Tuple1 form.
	rmCls isa.Class
}

// memOperand returns the memory operand from rmv or moffsv, if present.
func (e *enc) memOperand() *operand.Mem {
	if e.moffsv != nil && e.moffsv.kind == kMem {
		return &e.moffsv.mem
	}
	if e.rmv != nil && e.rmv.kind == kMem {
		return &e.rmv.mem
	}
	return nil
}

// bind walks the form's slots in order and puts each operand in the field
// the form says it belongs in. This is the only place operand order matters,
// and it matters exactly once.
func (e *enc) bind(vs []val) error {
	i := 0
	for _, s := range e.f.Slots {
		if s.Implicit {
			continue
		}
		if i >= len(vs) {
			return &CountError{Form: e.f, Got: len(vs), Want: e.f.Arity()}
		}
		v := &vs[i]
		i++

		switch s.Field {
		case isa.InRM:
			e.rmv, e.rmCls = v, s.Class
		case isa.InReg:
			e.regv = v
		case isa.InVVVV:
			e.vvvv = v
		case isa.InOpcode:
			e.plusr = v
		case isa.InImm:
			e.immv = v
		case isa.InIS4:
			e.is4v = v
		case isa.InMask:
			e.maskv = v
		case isa.InMoffs:
			e.moffsv = v
		case isa.InNone:
			// A fixed operand: the opcode already names it. It is checked
			// (isa.Resolve matched it) and then dropped, because there is
			// nowhere to put it.
		}
	}
	if i != len(vs) {
		return &CountError{Form: e.f, Got: len(vs), Want: e.f.Arity()}
	}
	return nil
}

// check is the legality the form cannot state and the operand cannot know on
// its own: the interactions between a register's number, the prefix that has
// to reach it, and the encoding the form chose.
func (e *enc) check() error {
	for _, v := range e.operands() {
		if v.kind == kMem {
			if err := v.mem.Validate(); err != nil {
				return err
			}
			if v.mem.RIP && e.f.Attrs&isa.PlusReg != 0 {
				return ErrRIPWithoutModRM
			}
		}
	}

	// XMM16–31, YMM16–31 and ZMM16–31 have no encoding without EVEX: there
	// is no prefix bit in a legacy or VEX encoding that reaches them.
	if e.f.Enc != isa.EncEVEX {
		for _, v := range e.operands() {
			if v.kind == kReg && v.reg.Num() >= 16 {
				return &RegisterError{Reg: v.reg, Enc: e.f.Enc}
			}
		}
	}

	// AH, CH, DH and BH occupy the encodings SPL, BPL, SIL and DIL take
	// under REX, so a REX prefix makes them unreachable. This is not a
	// preference: the byte would name a different register.
	if e.f.Enc == isa.EncLegacy && e.needsREX() {
		for _, v := range e.operands() {
			if v.kind != kReg {
				continue
			}
			if r, ok := v.reg.(interface{ RexForbidden() bool }); ok && r.RexForbidden() {
				return &RexConflictError{Reg: v.reg}
			}
		}
	}

	if e.opts.Zero && e.maskIsZero() {
		return ErrZeroWithoutMask
	}
	if e.opts.Zero && e.f.Attrs&isa.Zeroing == 0 {
		return &ModifierError{Form: e.f, What: "{z}"}
	}
	if e.opts.Broadcast {
		if e.f.Attrs&isa.Broadcast == 0 {
			return &ModifierError{Form: e.f, What: "{1toN}"}
		}
		if e.rmv == nil || e.rmv.kind != kMem {
			return ErrBroadcastNeedsMemory
		}
	}
	if e.opts.Round != RoundNone {
		if e.f.Attrs&isa.RoundCtl == 0 {
			return &ModifierError{Form: e.f, What: "{er}"}
		}
		if e.f.Len != isa.L512 {
			// L'L carries the rounding mode, so there is no length left to
			// state. Rounding exists only at the full width.
			return ErrRoundNot512
		}
		if e.rmv != nil && e.rmv.kind == kMem {
			return ErrRoundWithMemory
		}
	}
	if e.opts.SAE && e.f.Attrs&isa.SAE == 0 {
		return &ModifierError{Form: e.f, What: "{sae}"}
	}
	return nil
}

func (e *enc) operands() []*val {
	out := make([]*val, 0, 5)
	for _, v := range []*val{e.rmv, e.regv, e.vvvv, e.plusr, e.is4v, e.maskv, e.moffsv} {
		if v != nil {
			out = append(out, v)
		}
	}
	return out
}

func (e *enc) maskIsZero() bool {
	return e.maskv == nil || e.maskv.reg == reg.Reg(reg.K0)
}

// close records, for every fixup, how many bytes of this instruction follow
// the field it patches.
//
// This is the whole reason a caller never writes Addend: -4. The
// displacement of a call ends the instruction, so its tail is zero; the
// displacement of `mov dword [rip+x], 5` is followed by four bytes of
// immediate, so its tail is four and the raw ELF addend is -8. The
// assembler knows because it placed the field, and write_elf.go turns tail into the raw addend the format wants.
func (e *enc) close() {
	for i := range e.fix {
		f := &e.fix[i]
		f.Tail = len(e.buf) - (f.Offset + f.Size)
	}
}

func (e *enc) emit(b ...byte) { e.buf = append(e.buf, b...) }