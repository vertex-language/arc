package text

import (
	"errors"
	"fmt"

	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
)

// Operand is one operand as written, before it is a value.
//
// It is a struct with a kind rather than an interface with implementations,
// because a parser fills it field by field as it reads left to right and a
// printer reads it back the same way. An interface would mean building a value
// before knowing which one it is — an address is a register until the comma
// arrives, and a shift is a mnemonic until its operand does.
type Operand struct {
	Kind OperandKind
	P    Pos

	// Reg is the register, for OpReg and as the base of OpMem.
	Reg reg.Reg

	// Arr and Lane decorate a vector register: v0.4s is Arr, v2.s[1] is Lane.
	Arr  reg.Arrangement
	Elem reg.Elem
	Lane int
	HasLane bool

	// Expr is the immediate, displacement or target expression.
	Expr Expr

	// Mod is the address-role modifier on a symbolic operand.
	Mod Modifier

	// Mem describes an address. Base is in Reg.
	Mem MemRef

	// Shift and Extend decorate a register operand.
	Shift  operand.Shift
	Extend operand.Extend
	Amount Expr

	// Cond, Barrier and Prf are the small enumerations.
	Cond    operand.Cond
	Barrier operand.Barrier
	Prf     operand.PrfOp

	// Sys is a system register, named or generic.
	Sys reg.Sys

	// Text is the operand as written, kept for a diagnostic that wants to
	// quote the source rather than reprint the tree.
	Text string
}

// OperandKind is which of the above is in use.
type OperandKind uint8

const (
	OpNone OperandKind = iota

	OpReg     // x0, w1, v2.4s, v2.s[1], sp, xzr
	OpImm     // #93, #(1 << 3)
	OpMem     // [x1, #8]
	OpTarget  // a branch destination or an address: msg, :lo12:msg, 1f
	OpShift   // lsl #3
	OpExtend  // sxtw #2
	OpCond    // eq
	OpBarrier // ish
	OpPrfOp   // pldl1keep
	OpSys     // nzcv, s3_0_c0_c0_0

	opKindCount
)

func (k OperandKind) String() string {
	switch k {
	case OpReg:
		return "register"
	case OpImm:
		return "immediate"
	case OpMem:
		return "address"
	case OpTarget:
		return "target"
	case OpShift:
		return "shift"
	case OpExtend:
		return "extend"
	case OpCond:
		return "condition"
	case OpBarrier:
		return "barrier option"
	case OpPrfOp:
		return "prefetch operand"
	case OpSys:
		return "system register"
	}
	return "?"
}

// MemRef is an address as written.
//
// The addressing form is stated rather than inferred, because the syntax
// distinguishes shapes the fields alone do not: `[x1]` and `[x1, #0]` differ in
// source and not in encoding, and a printer that inferred the form would
// normalize one into the other — a formatting change a reader did not ask for.
type MemRef struct {
	Form operand.AddrForm

	// Disp is the displacement expression, or nil.
	Disp Expr

	// Mod is a modifier on a symbolic displacement: [x1, :lo12:msg].
	Mod Modifier

	// Index, Ext and Amount describe a register offset.
	Index  reg.Reg
	Ext    operand.Extend
	Amount Expr

	// Width is the access width, when the mnemonic's suffix or the register
	// implies one. Zero means the form decides.
	Width operand.Width
}

// Symbols lists the symbols this operand names.
func (o *Operand) Symbols() []string {
	var out []string
	if o.Expr != nil {
		out = append(out, Symbols(o.Expr)...)
	}
	if o.Mem.Disp != nil {
		out = append(out, Symbols(o.Mem.Disp)...)
	}
	return out
}

// Lower turns a written operand into the value the encoder accepts.
//
// This is the one place the source vocabulary meets the encoding vocabulary,
// and it is deliberately dumb: it evaluates expressions, applies modifiers, and
// builds the operand/ types. It chooses nothing. Whether the resulting value
// fits the field is encode/'s answer and whether the form accepts it at all is
// isa/'s, both of which have the form and neither of which this has.
func (o *Operand) Lower(env Env) (any, error) {
	switch o.Kind {
	case OpReg:
		return o.lowerReg()

	case OpSys:
		return o.Sys, nil

	case OpImm:
		v, err := Eval(o.Expr, env)
		if err != nil {
			return nil, err
		}
		return operand.Imm(v), nil

	case OpCond:
		return o.Cond, nil
	case OpBarrier:
		return o.Barrier, nil
	case OpPrfOp:
		return o.Prf, nil

	case OpShift:
		n, err := o.amount(env)
		if err != nil {
			return nil, err
		}
		return operand.Shifted(o.Shift, uint8(n)), nil

	case OpExtend:
		n, err := o.amount(env)
		if err != nil {
			return nil, err
		}
		return operand.Extended(o.Extend, uint8(n)), nil

	case OpTarget:
		return lowerTarget(o.Expr, o.Mod, env)

	case OpMem:
		return o.lowerMem(env)
	}
	return nil, fmt.Errorf("operand has no form")
}

func (o *Operand) lowerReg() (any, error) {
	v, isV := o.Reg.(reg.V)
	switch {
	case isV && o.HasLane:
		return v.Lane(o.Elem, uint8(o.Lane)), nil
	case isV && o.Arr != reg.ArrNone:
		return v.Arr(o.Arr), nil
	}
	return o.Reg, nil
}

func (o *Operand) amount(env Env) (int64, error) {
	if o.Amount == nil {
		return 0, nil
	}
	return Eval(o.Amount, env)
}

// lowerTarget builds a branch or address operand.
//
// A target reduces rather than evaluating, because the whole point of a branch
// destination is that it usually is not a number yet. A residue that is a plain
// constant becomes an immediate — a caller who wrote `b . + 8` meant a
// displacement — and one naming a symbol becomes a reference the encoder leaves
// blank.
func lowerTarget(e Expr, mod Modifier, env Env) (any, error) {
	v, err := Reduce(e, env)
	if err != nil {
		return nil, err
	}

	switch {
	case v.Constant():
		if mod.Valid() {
			return nil, fmt.Errorf("%s applied to a constant: the modifier names part of "+
				"a symbol's address and a number has no parts", mod.GAS())
		}
		return operand.Imm(v.Const), nil

	case v.Difference():
		return nil, errors.New("a symbol difference has no instruction encoding; " +
			"it is a data relocation, and on Mach-O a paired one")

	case v.PlusDot:
		return nil, errors.New("`.` in a branch target: write a label, since a " +
			"displacement from here is what the label already means")
	}

	var t operand.Target = operand.Label(v.Plus)
	if v.Const != 0 {
		t = operand.Sym(v.Plus).Plus(v.Const)
	}
	if mod.Valid() {
		return operand.AddrRef{T: t, Role: mod.Role()}, nil
	}
	return operand.Direct(t), nil
}

func (o *Operand) lowerMem(env Env) (any, error) {
	base, ok := o.Reg.(reg.Xsp)
	if !ok {
		x, isX := o.Reg.(reg.X)
		if !isX {
			return nil, errors.New("address base is not a 64-bit register")
		}
		sp, okSP := x.WithSP()
		if !okSP {
			return nil, errors.New("xzr is not an address: register 31 in a base slot is sp")
		}
		base = sp
	}

	m := memOf(base, o.Mem.Width)

	switch o.Mem.Form {
	case operand.AddrBase:
		return m, nil

	case operand.AddrRegOffset:
		n, err := evalOr(o.Mem.Amount, env, 0)
		if err != nil {
			return nil, err
		}
		return m.Index(o.Mem.Index, o.Mem.Ext, uint8(n)), nil

	case operand.AddrPreIndex, operand.AddrPostIndex:
		d, err := Eval(o.Mem.Disp, env)
		if err != nil {
			return nil, fmt.Errorf("writeback offset must be a constant: %w", err)
		}
		if o.Mem.Form == operand.AddrPreIndex {
			return m.Pre(d), nil
		}
		return m.Post(d), nil

	case operand.AddrOffset:
		if o.Mem.Mod.Valid() {
			t, err := lowerTarget(o.Mem.Disp, o.Mem.Mod, env)
			if err != nil {
				return nil, err
			}
			ref, ok := t.(operand.AddrRef)
			if !ok {
				return nil, errors.New("modified displacement did not reduce to an address reference")
			}
			return m.Off(ref), nil
		}
		d, err := Eval(o.Mem.Disp, env)
		if err != nil {
			return nil, err
		}
		if d == 0 {
			return m, nil
		}
		return m.Off(d), nil
	}
	return nil, errors.New("address has no form")
}

func memOf(base reg.Xsp, w operand.Width) operand.Mem {
	switch w {
	case operand.Width8:
		return operand.Mem8(base)
	case operand.Width16:
		return operand.Mem16(base)
	case operand.Width32:
		return operand.Mem32(base)
	case operand.Width64:
		return operand.Mem64(base)
	case operand.Width128:
		return operand.Mem128(base)
	}
	return operand.MemOf(base)
}

func evalOr(e Expr, env Env, def int64) (int64, error) {
	if e == nil {
		return def, nil
	}
	return Eval(e, env)
}