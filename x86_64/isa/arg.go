// x86_64/isa/arg.go
package isa

import (
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Arg is one operand as the caller has it, in the terms Resolve can match
// against a Slot. It is not the root package's Operand interface — nothing
// below the root sees that — and it is not an operand value either: encode/
// takes those. An Arg carries exactly what choosing between forms needs.
type Arg struct {
	Kind ArgKind

	Reg reg.Reg     // KindReg
	Mem operand.Mem // KindMem
	Imm operand.Imm // KindImm

	// Width pins an immediate to a field width. Zero means the encoder may
	// narrow: `add rax, 1` becomes the imm8 form. A symbolic immediate sets
	// it, because its value is not known here and narrowing an unknown is
	// how a fixup ends up one byte wide.
	Width operand.Width
}

type ArgKind uint8

const (
	KindReg ArgKind = iota
	KindMem
	KindImm
	KindRel // a branch or call target: matches rel8/rel32 and nothing else
)

func RegArg(r reg.Reg) Arg         { return Arg{Kind: KindReg, Reg: r} }
func MemArg(m operand.Mem) Arg     { return Arg{Kind: KindMem, Mem: m} }
func ImmArg(v operand.Imm) Arg     { return Arg{Kind: KindImm, Imm: v} }
func RelArg() Arg                  { return Arg{Kind: KindRel} }

// SymImmArg is an immediate whose value a fixup will supply. The width is the
// caller's, because only the caller knows what the relocation will write.
func SymImmArg(w operand.Width) Arg { return Arg{Kind: KindImm, Width: w} }

func (a Arg) String() string {
	switch a.Kind {
	case KindReg:
		return a.Reg.Name()
	case KindMem:
		return a.Mem.String()
	case KindImm:
		if a.Width != operand.WidthNone {
			return "imm:" + a.Width.String()
		}
		return a.Imm.String()
	}
	return "rel"
}

// Match reports whether a may fill a slot of class c.
func (c Class) Match(a Arg) bool {
	switch a.Kind {
	case KindReg:
		return c.matchReg(a.Reg)
	case KindMem:
		return c.matchMem(a.Mem)
	case KindImm:
		return c.matchImm(a)
	case KindRel:
		return c.IsRel()
	}
	return false
}

func (c Class) matchReg(r reg.Reg) bool {
	if c.MemOnly() || c.IsImm() || c.IsRel() {
		return false
	}
	switch c {
	case AL:
		return r == reg.Reg(reg.AL)
	case CL:
		return r == reg.Reg(reg.CL)
	case AX:
		return r == reg.Reg(reg.AX)
	case DX:
		return r == reg.Reg(reg.DX)
	case EAX:
		return r == reg.Reg(reg.EAX)
	case RAX:
		return r == reg.Reg(reg.RAX)
	case XMM0:
		return r == reg.Reg(reg.XMM0)
	case St0:
		return r == reg.Reg(reg.ST0)
	}
	switch cl := r.Class(); c {
	case R8, RM8:
		return cl == reg.ClassGP8
	case R16, RM16:
		return cl == reg.ClassGP16
	case R32, RM32:
		return cl == reg.ClassGP32
	case R64, RM64:
		return cl == reg.ClassGP64
	case Mm, MmM64:
		return cl == reg.ClassMm
	case Xmm, XmmM32, XmmM64, XmmM128:
		return cl == reg.ClassXmm
	case Ymm, YmmM256:
		return cl == reg.ClassYmm
	case Zmm, ZmmM512:
		return cl == reg.ClassZmm
	case K, KM64:
		return cl == reg.ClassK
	case Tmm:
		return cl == reg.ClassTmm
	case St:
		return cl == reg.ClassSt
	case Sreg:
		return cl == reg.ClassSreg
	case Cr:
		return cl == reg.ClassCr
	case Dr:
		return cl == reg.ClassDr
	}
	return false
}

func (c Class) matchMem(m operand.Mem) bool {
	if !c.AcceptsMem() {
		return false
	}
	if c == MAny {
		return true
	}
	// A width-agnostic reference — Abs, AbsSym, RIPRel — takes the class's
	// width. That is why `lea rsi, [rip+msg]` and `mov rax, [rip+x]` are the
	// same operand value in two different slots.
	if m.Width == operand.WidthNone {
		return true
	}
	return int(m.Width) == c.Bits()
}

func (c Class) matchImm(a Arg) bool {
	if !c.IsImm() {
		return false
	}
	if a.Width != operand.WidthNone {
		return int(a.Width) == c.Bits()
	}
	// The value's own narrowest width decides what it fits. Sign extension
	// is the rule everywhere except the imm64 forms, which have no
	// extension to do.
	switch c {
	case Imm8:
		return a.Imm.FitsInt8()
	case Imm16:
		return a.Imm.FitsInt16()
	case Imm32:
		return a.Imm.FitsInt32()
	case Imm64:
		return true
	}
	return false
}