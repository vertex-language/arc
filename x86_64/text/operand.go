// x86_64/text/operand.go
package text

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Operand is one operand as written.
//
// It is not an operand/ value and does not become one until its expressions
// have values. `mov rax, [rbx + count*4]` has a displacement that is a
// symbol times a constant, and no type in operand/ can hold that.
type Operand struct {
	Position Pos
	Kind     OperandKind

	// Reg is the register, for KindReg.
	Reg reg.Reg

	// Expr is the immediate or branch target, for KindImm and KindTarget.
	Expr Expr

	// Mem is the memory reference, for KindMem.
	Mem MemRef

	// Size is the operand size the source stated: gas's mnemonic suffix,
	// NASM's BYTE/WORD/DWORD/QWORD keyword. WidthNone means unstated, which
	// is legal whenever another operand settles it.
	//
	// This is the field a text-level translator cannot recover in both
	// directions, and the reason `arc fmt` resolves the form before it
	// prints: gas puts the size on the mnemonic and NASM puts it on the
	// operand, so going one way you have to invent it and going the other
	// you have to drop it.
	Size operand.Width
}

// OperandKind is what the operand is.
type OperandKind uint8

const (
	KindReg OperandKind = iota
	KindImm
	KindMem

	// KindTarget is a branch or call destination. It is a separate kind
	// from KindImm because the two are the same syntax in NASM and
	// different relocations in every object format: `call foo` is
	// pc-relative and `mov rax, foo` is not.
	KindTarget
)

// MemRef is a memory reference as written: registers by value, everything
// else still an expression.
type MemRef struct {
	Seg    reg.Sreg
	HasSeg bool

	Base    reg.Reg64
	HasBase bool

	Index    reg.Reg64
	Scale    int64
	HasIndex bool

	// Disp is the displacement expression, or nil.
	Disp Expr

	// RIP marks %rip-relative addressing: gas's `msg(%rip)` and NASM's
	// `[rel msg]`.
	RIP bool

	// Addr32 marks a 32-bit address size, from gas's `(%ebx)` or NASM's
	// 32-bit register names inside brackets.
	Addr32 bool
}

// RegOp builds a register operand.
func RegOp(p Pos, r reg.Reg) *Operand {
	return &Operand{Position: p, Kind: KindReg, Reg: r}
}

// ImmOp builds an immediate operand.
func ImmOp(p Pos, e Expr) *Operand {
	return &Operand{Position: p, Kind: KindImm, Expr: e}
}

// TargetOp builds a branch target.
func TargetOp(p Pos, e Expr) *Operand {
	return &Operand{Position: p, Kind: KindTarget, Expr: e}
}

// MemOp builds a memory operand.
func MemOp(p Pos, m MemRef, size operand.Width) *Operand {
	return &Operand{Position: p, Kind: KindMem, Mem: m, Size: size}
}

// Validate is the checking the operand can do on its own: a scale that has
// no encoding, an index that cannot be one, a rip-relative reference with a
// base.
//
// The same rules live in operand/, on the values this becomes. They are
// checked twice on purpose: here the diagnostic can name a line, and there
// it protects a caller who never had a line.
func (o *Operand) Validate() error {
	if o.Kind != KindMem {
		return nil
	}
	m := o.Mem
	if m.RIP && (m.HasBase || m.HasIndex) {
		return Errorf(o.Position, "rip-relative addressing takes no base or index")
	}
	if m.HasIndex {
		switch m.Scale {
		case 1, 2, 4, 8:
		default:
			return Errorf(o.Position, "scale must be 1, 2, 4 or 8 (got %d)", m.Scale)
		}
		if m.Index == reg.RSP {
			return Errorf(o.Position, "rsp cannot be an index register")
		}
	}
	return nil
}

// Lower turns a text operand into the operand/ value encode/ takes.
//
// A displacement that is not a constant comes back as a SymRef with the
// residue's addend, which is what a fixup needs; a displacement that is
// neither constant nor a single relocatable symbol is refused here, because
// there is no relocation that would express it.
func (o *Operand) Lower(env Env) (any, error) {
	switch o.Kind {
	case KindReg:
		return o.Reg, nil

	case KindImm, KindTarget:
		v, err := Reduce(o.Expr, env)
		if err != nil {
			return nil, err
		}
		if v.IsConst() {
			return operand.Imm(v.Const), nil
		}
		if v.Sub != "" || v.Add == "" {
			return nil, Errorf(o.Position,
				"%s is not an address a relocation can name", v)
		}
		return operand.SymRef{Name: v.Add, Kind: v.Reloc, Addend: v.Const}, nil

	case KindMem:
		return o.lowerMem(env)
	}
	return nil, Errorf(o.Position, "unknown operand kind")
}

func (o *Operand) lowerMem(env Env) (any, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	m := operand.Mem{
		Base: o.Mem.Base, HasBase: o.Mem.HasBase,
		Index: o.Mem.Index, HasIndex: o.Mem.HasIndex,
		Scale: uint8(o.Mem.Scale),
		RIP:   o.Mem.RIP, Addr32: o.Mem.Addr32,
	}
	if o.Mem.HasSeg {
		m = m.Segment(o.Mem.Seg)
	}

	if o.Mem.Disp != nil {
		v, err := Reduce(o.Mem.Disp, env)
		if err != nil {
			return nil, err
		}
		switch {
		case v.IsConst():
			if v.Const < -2147483648 || v.Const > 4294967295 {
				return nil, Errorf(o.Position,
					"displacement %d does not fit 32 bits", v.Const)
			}
			m = m.Displace(int32(v.Const))
		case v.Add != "" && v.Sub == "":
			m = m.WithSym(operand.SymRef{Name: v.Add, Kind: v.Reloc, Addend: v.Const})
		default:
			return nil, Errorf(o.Position,
				"%s is not a displacement a relocation can name", v)
		}
	}

	// The width comes from Size, which the syntax stated or the assembler
	// filled in from the form. Without one the reference stays unsized, and
	// isa/ decides: lea takes it as-is and mov does not.
	switch o.Size {
	case operand.W8:
		return m.M8(), nil
	case operand.W16:
		return m.M16(), nil
	case operand.W32:
		return m.M32(), nil
	case operand.W64:
		return m.M64(), nil
	case operand.W128:
		return m.M128(), nil
	case operand.W256:
		return m.M256(), nil
	case operand.W512:
		return m.M512(), nil
	}
	return m, nil
}

// String is a diagnostic rendering in a neutral notation belonging to
// neither dialect, so nobody mistakes it for output.
func (o *Operand) String() string {
	switch o.Kind {
	case KindReg:
		return o.Reg.Name()
	case KindImm:
		return "$" + exprString(o.Expr)
	case KindTarget:
		return exprString(o.Expr)
	case KindMem:
		var parts []string
		if o.Mem.RIP {
			parts = append(parts, "rip")
		}
		if o.Mem.HasBase {
			parts = append(parts, o.Mem.Base.Name())
		}
		if o.Mem.HasIndex {
			parts = append(parts, fmt.Sprintf("%s*%d", o.Mem.Index.Name(), o.Mem.Scale))
		}
		if o.Mem.Disp != nil {
			parts = append(parts, exprString(o.Mem.Disp))
		}
		s := "[" + strings.Join(parts, "+") + "]"
		if o.Mem.HasSeg {
			s = o.Mem.Seg.Name() + ":" + s
		}
		if o.Size != operand.WidthNone {
			s = o.Size.String() + " " + s
		}
		return s
	}
	return "?"
}

func exprString(e Expr) string {
	switch x := e.(type) {
	case nil:
		return ""
	case *Num:
		return fmt.Sprint(x.Value)
	case *Sym:
		return x.Name
	case *Dot:
		return "."
	case *Here:
		return "$$"
	case *Unary:
		return x.Op.String() + exprString(x.X)
	case *Binary:
		s := exprString(x.X) + x.Op.String() + exprString(x.Y)
		if x.Paren {
			return "(" + s + ")"
		}
		return s
	}
	return "?"
}