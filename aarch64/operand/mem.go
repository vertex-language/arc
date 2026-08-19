package operand

import (
	"errors"
	"strconv"

	"github.com/vertex-language/arc/aarch64/reg"
)

// AddrForm is which addressing mode a memory operand uses.
//
// The scaled/unscaled distinction that isa/ carries is deliberately absent
// here. [x1, #8] is the same operand whether it lands in LDR or in LDUR; those
// are two mnemonics the caller names, and picking between them on the strength
// of whether the offset divides evenly is exactly the instruction selection
// this tree does not do.
type AddrForm uint8

const (
	AddrNone      AddrForm = iota
	AddrBase               // [Xn]
	AddrOffset             // [Xn, #imm]
	AddrRegOffset          // [Xn, Xm{, LSL #s}] / [Xn, Wm, SXTW #s]
	AddrPreIndex           // [Xn, #imm]!
	AddrPostIndex          // [Xn], #imm
)

func (f AddrForm) String() string {
	switch f {
	case AddrBase:
		return "[Xn]"
	case AddrOffset:
		return "[Xn, #imm]"
	case AddrRegOffset:
		return "[Xn, Rm]"
	case AddrPreIndex:
		return "[Xn, #imm]!"
	case AddrPostIndex:
		return "[Xn], #imm"
	}
	return "[?]"
}

// Disp is a displacement: a constant, or a symbol reference that is not a
// number yet. Exactly one of the two is in use, which Sym reports.
type Disp struct {
	Const int64
	Ref   AddrRef
	Sym   bool
}

// Mem is an address operand.
//
// The base is an Xsp because register 31 in a base slot is the stack pointer on
// every addressing form the architecture has. There is no form that reads a
// base as the zero register, so a Mem built from XZR is refused at construction
// rather than at encode time.
type Mem struct {
	Base  reg.Xsp
	Width Width
	Form  AddrForm
	Disp  Disp

	// Index, Ext and Amount describe AddrRegOffset. Index is a reg.X or a
	// reg.W; which one is legal follows from Ext.
	Index  reg.Reg
	Ext    Extend
	Amount uint8

	// bad records a construction error. Builder methods return a Mem rather
	// than an error so an address reads as one expression, the same reason the
	// assembler's own calls return nothing; Validate is where it surfaces.
	bad string
}

// baseReg is what can be the base of an address: a numbered X, or an Xsp for
// the stack pointer itself.
type baseReg interface{ reg.X | reg.Xsp }

// MemOf builds an address with no stated access width, leaving it to the form.
func MemOf[T baseReg](base T) Mem { return newMem(base, WidthNone) }

// Mem8 through Mem128 state the width the caller means, which makes a
// mismatched form a resolve-time error naming both widths rather than a
// silently different instruction.
func Mem8[T baseReg](base T) Mem   { return newMem(base, Width8) }
func Mem16[T baseReg](base T) Mem  { return newMem(base, Width16) }
func Mem32[T baseReg](base T) Mem  { return newMem(base, Width32) }
func Mem64[T baseReg](base T) Mem  { return newMem(base, Width64) }
func Mem128[T baseReg](base T) Mem { return newMem(base, Width128) }

func newMem[T baseReg](base T, w Width) Mem {
	m := Mem{Width: w, Form: AddrBase, Ext: ExtNone}
	switch v := any(base).(type) {
	case reg.X:
		sp, ok := v.WithSP()
		if !ok {
			m.bad = "xzr is not an address: register 31 in a base slot is sp"
			return m
		}
		m.Base = sp
	case reg.Xsp:
		m.Base = v
	}
	return m
}

// Off sets a displacement: [Xn, #imm].
//
// It takes an integer or an AddrRef, because both spellings appear in the same
// position in source and a caller should not have to pick a method name based
// on whether the offset happens to be symbolic. A bare Label or SymRef is
// refused: the low-twelve half of an address is a role the caller has to state,
// and guessing PageOff would be inventing the one thing a reader most needs to
// see written down.
func (m Mem) Off(v any) Mem {
	if m.bad != "" {
		return m
	}
	m.Form = AddrOffset
	switch d := v.(type) {
	case int:
		m.Disp = Disp{Const: int64(d)}
	case int64:
		m.Disp = Disp{Const: d}
	case Imm:
		m.Disp = Disp{Const: int64(d)}
	case AddrRef:
		m.Disp = Disp{Ref: d, Sym: true}
	case Label, SymRef:
		m.bad = "a symbolic offset needs a role: write PageOff(...) or GotPageOff(...)"
	default:
		m.bad = "offset is neither an integer nor an address reference"
	}
	return m
}

// Pre sets a pre-indexed displacement: [Xn, #imm]!, which writes the computed
// address back into the base.
func (m Mem) Pre(d int64) Mem {
	if m.bad != "" {
		return m
	}
	m.Form, m.Disp = AddrPreIndex, Disp{Const: d}
	return m
}

// Post sets a post-indexed displacement: [Xn], #imm, which uses the base and
// then writes back.
func (m Mem) Post(d int64) Mem {
	if m.bad != "" {
		return m
	}
	m.Form, m.Disp = AddrPostIndex, Disp{Const: d}
	return m
}

// Index sets a register offset: [Xn, Xm, LSL #s] or [Xn, Wm, SXTW #s].
//
// The index is a reg.Reg rather than a type parameter because a method cannot
// take one. Passing anything but an X or a W is caught by Validate; passing a
// non-register does not compile.
func (m Mem) Index(idx reg.Reg, ext Extend, amount uint8) Mem {
	if m.bad != "" {
		return m
	}
	m.Form, m.Index, m.Ext, m.Amount = AddrRegOffset, idx, ext, amount
	return m
}

// Sized restates the access width, for a caller that built an address before it
// knew which load it was for.
func (m Mem) Sized(w Width) Mem {
	if m.bad != "" {
		return m
	}
	m.Width = w
	return m
}

// Symbolic reports whether the displacement is a symbol reference, which is
// what tells the encoder to leave the field blank and record a fixup.
func (m Mem) Symbolic() bool { return m.Disp.Sym }

// Validate reports what has no encoding.
//
// Range checking is not here: whether an offset fits depends on the form's
// field and its scale, which this operand does not know. What is here is the
// shape — combinations of base, index and writeback that no A64 addressing mode
// expresses at all, and which would otherwise reach the encoder as a form
// mismatch with a confusing message.
func (m Mem) Validate() error {
	if m.bad != "" {
		return errors.New(m.bad)
	}
	if m.Width != WidthNone && !m.Width.Valid() {
		return errors.New("access width is not one the architecture has")
	}

	switch m.Form {
	case AddrBase:
		return nil

	case AddrOffset:
		if m.Disp.Sym {
			switch m.Disp.Ref.Role {
			case RolePageOff, RoleGotPageOff:
			default:
				return errors.New("only the page-offset half of an address can be a memory displacement")
			}
		}
		return nil

	case AddrPreIndex, AddrPostIndex:
		if m.Disp.Sym {
			return errors.New("a writeback address cannot take a symbolic offset: the field is nine unscaled bits and a relocation has nowhere to go")
		}
		return nil

	case AddrRegOffset:
		if m.Index == nil {
			return errors.New("register offset with no index register")
		}
		if !m.Ext.Valid() {
			return errors.New("register offset needs an extend: lsl, uxtw, sxtw or sxtx")
		}
		switch m.Index.(type) {
		case reg.X:
			if m.Ext.SourceIsW() {
				return errors.New("index is a 64-bit register but " + m.Ext.String() + " reads a 32-bit one")
			}
		case reg.W:
			if !m.Ext.SourceIsW() {
				return errors.New("index is a 32-bit register but " + m.Ext.String() + " reads a 64-bit one")
			}
		default:
			return errors.New("index register must be an X or a W")
		}
		// The shift is one bit in the encoding: either no shift, or exactly
		// the log of the access width. Anything else has no field to go in.
		if m.Amount != 0 {
			sc, ok := m.Width.Scale()
			if !ok {
				return errors.New("a shifted index needs a stated access width: write Mem64 rather than Mem")
			}
			if m.Amount != sc {
				return errors.New("index shift must be 0 or " + strconv.Itoa(int(sc)) +
					" for a " + m.Width.String() + " access")
			}
		}
		return nil
	}
	return errors.New("address has no form")
}

func (m Mem) String() string {
	b := "[" + m.Base.String()
	switch m.Form {
	case AddrBase:
		return b + "]"
	case AddrOffset:
		if m.Disp.Sym {
			return b + ", " + m.Disp.Ref.String() + "]"
		}
		if m.Disp.Const == 0 {
			return b + "]"
		}
		return b + ", #" + strconv.FormatInt(m.Disp.Const, 10) + "]"
	case AddrPreIndex:
		return b + ", #" + strconv.FormatInt(m.Disp.Const, 10) + "]!"
	case AddrPostIndex:
		return b + "], #" + strconv.FormatInt(m.Disp.Const, 10)
	case AddrRegOffset:
		s := b + ", " + m.Index.String()
		e := ExtendOp{Op: m.Ext, Amount: m.Amount}
		if m.Ext == ExtLSL && m.Amount == 0 {
			return s + "]"
		}
		if m.Ext == ExtLSL {
			return s + ", " + Shifted(LSL, m.Amount).String() + "]"
		}
		return s + ", " + e.String() + "]"
	}
	return b + ", ?]"
}