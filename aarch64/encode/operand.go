package encode

import (
	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
)

// val is a caller's operand seen as this package needs it: one of a small
// closed set of shapes, with the original kept for the diagnostic.
type val struct {
	kind valKind
	raw  any

	reg   reg.Reg
	imm   int64
	uimm  uint64
	mem   operand.Mem
	ref   operand.AddrRef
	shift operand.ShiftOp
	ext   operand.ExtendOp
	cond  operand.Cond
	bar   operand.Barrier
	prf   operand.PrfOp
	sys   reg.Sys
}

type valKind uint8

const (
	valNone valKind = iota
	valReg
	valImm
	valMem
	valRef
	valShift
	valExtend
	valCond
	valBarrier
	valPrfOp
	valSys
)

// lower classifies one caller value.
//
// The accepted set is deliberately narrow. An int is accepted because assembly
// source writes constants and a caller building operands at runtime has one in
// hand; a string is not, because a bare name could be a label or a register and
// guessing which is how a typo becomes an object file.
func lower(v any) val {
	switch x := v.(type) {
	case operand.Imm:
		return val{kind: valImm, raw: v, imm: int64(x), uimm: uint64(x)}
	case int:
		return val{kind: valImm, raw: v, imm: int64(x), uimm: uint64(int64(x))}
	case int64:
		return val{kind: valImm, raw: v, imm: x, uimm: uint64(x)}
	case uint64:
		return val{kind: valImm, raw: v, imm: int64(x), uimm: x}

	case operand.Mem:
		return val{kind: valMem, raw: v, mem: x}
	case operand.AddrRef:
		return val{kind: valRef, raw: v, ref: x}
	case operand.Label:
		return val{kind: valRef, raw: v, ref: operand.Direct(x)}
	case operand.SymRef:
		return val{kind: valRef, raw: v, ref: operand.Direct(x)}

	case operand.ShiftOp:
		return val{kind: valShift, raw: v, shift: x}
	case operand.Shift:
		return val{kind: valShift, raw: v, shift: operand.Shifted(x, 0)}
	case operand.ExtendOp:
		return val{kind: valExtend, raw: v, ext: x}
	case operand.Extend:
		return val{kind: valExtend, raw: v, ext: operand.Extended(x, 0)}
	case operand.Cond:
		return val{kind: valCond, raw: v, cond: x}
	case operand.Barrier:
		return val{kind: valBarrier, raw: v, bar: x}
	case operand.PrfOp:
		return val{kind: valPrfOp, raw: v, prf: x}

	case reg.Sys:
		return val{kind: valSys, raw: v, sys: x, reg: x}
	case reg.Reg:
		return val{kind: valReg, raw: v, reg: x}
	}
	return val{kind: valNone, raw: v}
}

// arg builds the isa.Arg that Resolve matches against.
func (v val) arg() isa.Arg {
	switch v.kind {
	case valReg:
		return isa.ArgOf(v.reg)
	case valSys:
		return isa.ArgOf(v.sys)
	case valImm:
		return isa.ImmArg(v.imm)
	case valMem:
		return isa.MemArg(memForm(v.mem.Form), uint16(v.mem.Width))
	case valRef:
		return isa.LabelArg()
	case valShift:
		return isa.Arg{Class: isa.ClassShift}
	case valExtend:
		return isa.Arg{Class: isa.ClassExtend}
	case valCond:
		return isa.Arg{Class: isa.ClassCond}
	case valBarrier:
		return isa.Arg{Class: isa.ClassBarrier}
	case valPrfOp:
		return isa.Arg{Class: isa.ClassPrfOp}
	}
	return isa.Arg{}
}

// memForm maps the operand package's addressing form to the table's.
//
// They are separate enumerations because they answer different questions.
// operand.AddrForm is what the caller wrote; isa.AddrForm is what a form
// accepts, and it distinguishes scaled from unscaled — a distinction the caller
// never makes, because [x1, #8] is the same address in LDR and in LDUR and
// which one it lands in is the mnemonic the caller named.
func memForm(f operand.AddrForm) isa.AddrForm {
	switch f {
	case operand.AddrBase:
		return isa.AddrBase
	case operand.AddrOffset:
		return isa.AddrOffset
	case operand.AddrRegOffset:
		return isa.AddrRegOffset
	case operand.AddrPreIndex:
		return isa.AddrPreIndex
	case operand.AddrPostIndex:
		return isa.AddrPostIndex
	}
	return isa.AddrNone
}

// regNum checks a register against a slot's class and returns its field value.
//
// The two asymmetries are isa.Class.Match's, restated here because Match works
// on an Arg and this works on the register itself, and a caller reaching
// EncodeForm directly has not been through Resolve.
func regNum(c isa.Class, r reg.Reg) (uint64, string) {
	switch c {
	case isa.ClassX:
		switch v := r.(type) {
		case reg.X:
			return uint64(v), ""
		case reg.Xsp:
			if v.IsSP() {
				return 0, "this slot reads register 31 as the zero register"
			}
			return uint64(v), ""
		}
		return 0, "expected a 64-bit general-purpose register"

	case isa.ClassW:
		switch v := r.(type) {
		case reg.W:
			return uint64(v), ""
		case reg.Wsp:
			if v.IsSP() {
				return 0, "this slot reads register 31 as the zero register"
			}
			return uint64(v), ""
		}
		return 0, "expected a 32-bit general-purpose register"

	case isa.ClassXsp:
		switch v := r.(type) {
		case reg.Xsp:
			return uint64(v), ""
		case reg.X:
			if v == reg.XZR {
				return 0, "this slot reads register 31 as the stack pointer; " +
					"xzr is a different register that shares the encoding"
			}
			return uint64(v), ""
		}
		return 0, "expected a 64-bit general-purpose register or sp"

	case isa.ClassWsp:
		switch v := r.(type) {
		case reg.Wsp:
			return uint64(v), ""
		case reg.W:
			if v == reg.WZR {
				return 0, "this slot reads register 31 as the stack pointer; " +
					"wzr is a different register that shares the encoding"
			}
			return uint64(v), ""
		}
		return 0, "expected a 32-bit general-purpose register or wsp"

	case isa.ClassPg:
		p, ok := r.(reg.P)
		if !ok {
			return 0, "expected a predicate register"
		}
		if !p.Governing() {
			return 0, "a governing predicate is encoded in three bits and reaches p0-p7 only"
		}
		return uint64(p), ""

	case isa.ClassSys:
		s, ok := r.(reg.Sys)
		if !ok {
			return 0, "expected a system register"
		}
		if !s.Movable() {
			return 0, "op0 is below 2, which is the sys and sysl encoding space " +
				"rather than a register mrs and msr can reach"
		}
		// The instruction carries o0 rather than op0, so the high bit goes.
		return uint64(s.Num()) & 0x7fff, ""
	}

	// Every remaining register class is a plain number in its own file.
	if c.File() != r.Class().File() || c.File() == reg.FileNone {
		return 0, "wrong register file for this slot"
	}
	if c != r.Class() {
		return 0, "wrong width for this slot"
	}
	return uint64(r.Num()), ""
}