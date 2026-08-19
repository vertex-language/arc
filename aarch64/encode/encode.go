package encode

import (
	"github.com/vertex-language/arc/aarch64/feature"
	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// Opts is what an encode call needs that is not an operand.
//
// x86_64's Opts carries broadcast, zeroing, rounding and SAE — one bit each,
// with no register behind them, which is what keeps them out of the operand
// list. A64 has no such modifier: a predicate is a register and goes in a slot,
// and a shift is an operand. So this carries call context only, and exists so
// the signature does not have to change when something does turn up.
type Opts struct {
	// Offset is the byte offset of this instruction within its section. It is
	// copied onto every Fixup and is otherwise unused; nothing here computes an
	// address.
	Offset int
}

// Encode resolves a mnemonic against a feature set and encodes it.
func Encode(set feature.Set, mnem string, ops ...any) (uint32, []Fixup, error) {
	return EncodeWith(Opts{}, set, mnem, ops...)
}

// EncodeWith is Encode with call context.
func EncodeWith(opts Opts, set feature.Set, mnem string, ops ...any) (uint32, []Fixup, error) {
	vals := make([]val, len(ops))
	args := make([]isa.Arg, len(ops))
	for i, o := range ops {
		vals[i] = lower(o)
		args[i] = vals[i].arg()
	}
	f, err := isa.Resolve(mnem, args, set)
	if err != nil {
		return 0, nil, err
	}
	return encodeForm(f, vals, opts)
}

// EncodeForm encodes against a form the caller already resolved, which is what
// the generated helpers and the text path's Inst.Form do.
//
// It does not consult a feature set. The gate was checked when the form was
// resolved, and re-checking here would let the two answers disagree.
func EncodeForm(f *isa.Form, ops []any, opts Opts) (uint32, []Fixup, error) {
	vals := make([]val, len(ops))
	for i, o := range ops {
		vals[i] = lower(o)
	}
	return encodeForm(f, vals, opts)
}

func encodeForm(f *isa.Form, ops []val, opts Opts) (uint32, []Fixup, error) {
	word := f.Word
	var fixups []Fixup

	// Slots and operands are not in step. A memory operand is one value
	// filling two slots — a base and an offset — so the two indices are walked
	// separately rather than zipped.
	oi := 0
	for si := 0; si < len(f.Slots); si++ {
		s := f.Slots[si]

		if oi >= len(ops) {
			if !s.Optional {
				return 0, nil, &CountError{Form: f, Got: len(ops)}
			}
			word = place(word, s.Field, s.Default)
			continue
		}
		v := ops[oi]

		switch {
		case s.Class.Mem():
			w, fx, consumed, err := encodeMem(f, oi, si, v, opts)
			if err != nil {
				return 0, nil, err
			}
			word = w(word)
			fixups = append(fixups, fx...)
			si += consumed - 1
			oi++

		case s.Class.Reg():
			if v.kind != valReg && v.kind != valSys {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			n, why := regNum(s.Class, v.reg)
			if why != "" {
				return 0, nil, &RegisterError{f, oi, s.Class, v.reg.String(), why}
			}
			word = place(word, s.Field, n)
			oi++

		case s.Class == isa.ClassImm:
			if v.kind != valImm {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			r, err := encodeImm(f, oi, s, v, hasShiftOperand(ops))
			if err != nil {
				return 0, nil, err
			}
			word = place(word, s.Field, r.value)
			if r.hasSibling {
				if fld, ok := siblingField(f, r.siblingClass); ok {
					word = place(word, fld, r.sibling)
				}
			}
			oi++

		case s.Class == isa.ClassLabel:
			w, fx, err := encodeTarget(f, oi, s, v, opts)
			if err != nil {
				return 0, nil, err
			}
			word = w(word)
			fixups = append(fixups, fx...)
			oi++

		case s.Class == isa.ClassShift:
			if v.kind != valShift {
				// An unwritten optional shift; the slot keeps its default and
				// this operand belongs to a later slot.
				if s.Optional {
					word = place(word, s.Field, s.Default)
					continue
				}
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			n, err := encodeShift(f, oi, s, v.shift)
			if err != nil {
				return 0, nil, err
			}
			word = place(word, s.Field, n)
			oi++

		case s.Class == isa.ClassExtend:
			if v.kind != valExtend {
				if s.Optional {
					word = place(word, s.Field, s.Default)
					continue
				}
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			if !v.ext.Valid() {
				return 0, nil, &RangeError{f, oi, int64(v.ext.Amount),
					"an extend shift of 0 to 4"}
			}
			word = place(word, s.Field, uint64(v.ext.Op))
			if fld, ok := siblingField(f, isa.ClassImm); ok && f.Attrs&isa.AttrScaled == 0 {
				word = place(word, fld, uint64(v.ext.Amount))
			}
			oi++

		case s.Class == isa.ClassCond:
			if v.kind != valCond {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			if !v.cond.Valid() {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			word = place(word, s.Field, uint64(v.cond))
			oi++

		case s.Class == isa.ClassBarrier:
			if v.kind != valBarrier {
				if s.Optional {
					word = place(word, s.Field, s.Default)
					continue
				}
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			if !v.bar.Valid() {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			word = place(word, s.Field, uint64(v.bar))
			oi++

		case s.Class == isa.ClassPrfOp:
			if v.kind != valPrfOp {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			if !v.prf.Valid() {
				return 0, nil, &OperandError{f, oi, s.Class, v.raw}
			}
			word = place(word, s.Field, uint64(v.prf))
			oi++

		default:
			return 0, nil, &OperandError{f, oi, s.Class, v.raw}
		}
	}

	if oi != len(ops) {
		return 0, nil, &CountError{Form: f, Got: len(ops)}
	}

	for i := range fixups {
		fixups[i].Offset = opts.Offset
	}
	return word, fixups, nil
}

// hasShiftOperand reports whether the caller wrote a shift, which changes the
// immediate rule rather than adding to it.
func hasShiftOperand(ops []val) bool {
	for _, v := range ops {
		if v.kind == valShift {
			return true
		}
	}
	return false
}

// encodeShift fills a shift slot.
//
// What the field holds depends on which immediate the shift belongs to, not on
// the field's own width, and the two happen to differ in width as well — which
// is why an earlier version of this function got it wrong in a way that still
// produced a plausible word. Hw and Shift are both two bits. Hw holds a
// halfword index and Shift holds a shift kind, and writing the kind into Hw
// encodes movz x0, #1, lsl #16 as lsl #0.
func encodeShift(f *isa.Form, i int, s isa.Slot, sh operand.ShiftOp) (uint64, error) {
	switch immKindOf(f) {
	case isa.ImmMoveWide:
		if sh.Op != operand.LSL {
			return 0, &OperandError{f, i, isa.ClassShift, sh}
		}
		if sh.Amount%16 != 0 {
			return 0, &RangeError{f, i, int64(sh.Amount),
				"a shift of 0, 16, 32 or 48: the field names a halfword, not a distance"}
		}
		hw := uint64(sh.Amount) / 16
		limit := uint64(4)
		if FormWidth(f) == 32 {
			limit = 2
		}
		if hw >= limit {
			return 0, &RangeError{f, i, int64(sh.Amount),
				"a shift inside a " + operand.Width(FormWidth(f)).String()}
		}
		return hw, nil

	case isa.ImmAddSub12:
		if !sh.ValidImm12() {
			return 0, &RangeError{f, i, int64(sh.Amount),
				"lsl #0 or lsl #12: the field is one bit"}
		}
		if sh.Amount == 12 {
			return 1, nil
		}
		return 0, nil
	}

	// A shifted-register form: the field is the shift kind and the amount
	// rides in the form's immediate slot.
	if !sh.Op.Valid() {
		return 0, &OperandError{f, i, isa.ClassShift, sh}
	}
	if fld, ok := siblingField(f, isa.ClassImm); ok {
		if !fitsU(uint64(sh.Amount), fld.Width()) {
			return 0, &RangeError{f, i, int64(sh.Amount),
				plural(fld.Width()) + " of shift amount"}
		}
	}
	return uint64(sh.Op), nil
}

// encodeMem fills a memory operand's slots and returns how many it consumed.
func encodeMem(f *isa.Form, oi, si int, v val, opts Opts) (func(uint32) uint32, []Fixup, int, error) {
	if v.kind != valMem {
		return nil, nil, 0, &OperandError{f, oi, f.Slots[si].Class, v.raw}
	}
	m := v.mem
	if err := m.Validate(); err != nil {
		return nil, nil, 0, &AddressError{f, oi, m, err.Error()}
	}

	base := f.Slots[si]
	var off *isa.Slot
	consumed := 1
	if si+1 < len(f.Slots) && f.Slots[si+1].Role == isa.RoleOffset {
		off = &f.Slots[si+1]
		consumed = 2
	}

	// The writeback forms are separate encodings, and a form that is not one
	// cannot express an address that asks for it.
	switch m.Form {
	case operand.AddrPreIndex:
		if f.Attrs&isa.AttrPreIndex == 0 {
			return nil, nil, 0, &AddressError{f, oi, m,
				"this form has no writeback; a pre-indexed address is a different encoding"}
		}
	case operand.AddrPostIndex:
		if f.Attrs&isa.AttrPostIndex == 0 {
			return nil, nil, 0, &AddressError{f, oi, m,
				"this form has no writeback; a post-indexed address is a different encoding"}
		}
	case operand.AddrRegOffset:
		return nil, nil, 0, &UnsupportedError{f,
			"a register-offset address: the table declares no Rm or option field for one"}
	}

	n, why := regNum(isa.ClassXsp, m.Base)
	if why != "" {
		return nil, nil, 0, &RegisterError{f, oi, isa.ClassXsp, m.Base.String(), why}
	}

	var fixups []Fixup
	apply := func(w uint32) uint32 { return place(w, base.Field, n) }

	if off == nil {
		// This form has no offset field at all. An address that states one is
		// refused whatever its value: accepting a zero displacement here would
		// mean [x1, #0] assembles and [x1, #8] does not, which reads as a range
		// problem when it is a form problem, and sends the caller looking for
		// the wrong fix.
		if m.Form != operand.AddrBase {
			return nil, nil, 0, &AddressError{f, oi, m,
				"this form takes a base register only; the offset forms are separate encodings"}
		}
		return apply, nil, consumed, nil
	}

	if m.Symbolic() {
		fixups = append(fixups, Fixup{
			Field:  off.Field,
			Target: m.Disp.Ref.T,
			Role:   m.Disp.Ref.Role,
			Kind:   m.Disp.Ref.Kind(),
			Addend: m.Disp.Ref.Addend(),
			Access: operand.Width(f.AccessBits()),
			Bits:   off.Field.Width(),
			Scale:  scaleOf(f),
		})
		return apply, fixups, consumed, nil
	}

	r, err := encodeImm(f, oi, *off, val{kind: valImm, imm: m.Disp.Const,
		uimm: uint64(m.Disp.Const)}, false)
	if err != nil {
		return nil, nil, 0, err
	}
	inner := apply
	apply = func(w uint32) uint32 { return place(inner(w), off.Field, r.value) }
	return apply, fixups, consumed, nil
}

func scaleOf(f *isa.Form) uint8 {
	if f.Attrs&isa.AttrScaled == 0 {
		return 0
	}
	sc, _ := operand.Width(f.AccessBits()).Scale()
	return sc
}

// encodeTarget fills a branch or address destination.
//
// A bare immediate is a displacement the caller computed and takes
// responsibility for. A reference is left blank and recorded, because the
// distance is not a number until something knows where both ends are.
func encodeTarget(f *isa.Form, i int, s isa.Slot, v val, opts Opts) (func(uint32) uint32, []Fixup, error) {
	switch v.kind {
	case valImm:
		r, err := encodeImm(f, i, s, v, false)
		if err != nil {
			return nil, nil, err
		}
		return func(w uint32) uint32 { return place(w, s.Field, r.value) }, nil, nil

	case valRef:
		role := v.ref.Role
		if role == operand.RoleDirect && s.Role == isa.RolePage {
			// adrp x0, msg with no modifier written: the form is the page
			// form, so the reference is to the page. This is the one place a
			// role is inferred, and it is inferred from the form rather than
			// from the spelling.
			role = operand.RolePage
		}
		fx := Fixup{
			Field:  s.Field,
			Target: v.ref.T,
			Role:   role,
			Kind:   v.ref.Kind(),
			Addend: v.ref.Addend(),
			Access: operand.Width(f.AccessBits()),
			Branch: f.Attrs&isa.AttrBranch != 0,
			Bits:   s.Field.Width(),
			Scale:  targetScale(role, f),
		}
		return func(w uint32) uint32 { return w }, []Fixup{fx}, nil
	}
	return nil, nil, &OperandError{f, i, isa.ClassLabel, v.raw}
}

func targetScale(role operand.AddrRole, f *isa.Form) uint8 {
	switch role {
	case operand.RolePage, operand.RoleGotPage:
		return 12 // 4KiB pages
	case operand.RolePageOff, operand.RoleGotPageOff:
		return scaleOf(f)
	}
	if f.Attrs&isa.AttrBranch != 0 {
		return 2 // word-aligned
	}
	return 0
}