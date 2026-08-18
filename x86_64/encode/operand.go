// x86_64/encode/operand.go
package encode

import (
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// val is one lowered operand. The type switch that produces it is the only
// place this tree turns an operand value into something with a tag on it,
// and it is here rather than in the root because the root's Operand
// interface would then have to be visible to isa/ as well.
type val struct {
	kind kind

	reg  reg.Reg
	mem  operand.Mem
	imm  operand.Imm
	sym  operand.Target
	wide operand.Width // for a symbolic immediate: the field width the caller wants
}

type kind uint8

const (
	kReg kind = iota
	kMem
	kImm
	kTarget // a branch or call target
	kSymImm // a symbolic immediate: a symbol's address in an immediate field
)

// SymImm is a symbol's address in an immediate field, as distinct from a
// branch target in the same field. `call foo` and `mov rax, foo` both put a
// symbol where the immediate goes and mean different things: one is
// PC-relative and one is not, and no type below this can tell them apart.
type SymImm struct {
	Target operand.Target
	Width  operand.Width
}

// lower turns caller operand values into vals. The repetition across the
// memory widths is the same repetition as operand/mem_width.go and for the
// same reason: one type carrying a width field would put the check back at
// run time.
func lower(ops []any) ([]val, error) {
	out := make([]val, 0, len(ops))
	for _, o := range ops {
		switch v := o.(type) {
		case reg.Reg:
			out = append(out, val{kind: kReg, reg: v})

		case operand.Mem:
			out = append(out, val{kind: kMem, mem: v})
		case operand.M8:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M16:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M32:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M64:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M128:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M256:
			out = append(out, val{kind: kMem, mem: v.Mem})
		case operand.M512:
			out = append(out, val{kind: kMem, mem: v.Mem})

		case operand.Imm:
			out = append(out, val{kind: kImm, imm: v})

		case operand.Label:
			out = append(out, val{kind: kTarget, sym: v})
		case operand.SymRef:
			out = append(out, val{kind: kTarget, sym: v})

		case SymImm:
			out = append(out, val{kind: kSymImm, sym: v.Target, wide: v.Width})

		default:
			return nil, &OperandError{Value: o}
		}
	}
	return out, nil
}

// Args lowers operand values into the descriptors isa.Resolve matches
// against. The root calls this before Resolve and passes the same values to
// Encode afterward, so the type switch exists once rather than twice.
func Args(ops ...any) ([]isa.Arg, error) {
	vs, err := lower(ops)
	if err != nil {
		return nil, err
	}
	out := make([]isa.Arg, len(vs))
	for i, v := range vs {
		switch v.kind {
		case kReg:
			out[i] = isa.RegArg(v.reg)
		case kMem:
			out[i] = isa.MemArg(v.mem)
		case kImm:
			out[i] = isa.ImmArg(v.imm)
		case kTarget:
			out[i] = isa.RelArg()
		case kSymImm:
			out[i] = isa.SymImmArg(v.wide)
		}
	}
	return out, nil
}

// num is the register's architectural number, 0–31.
func (v *val) num() uint8 {
	if v == nil || v.kind != kReg {
		return 0
	}
	return v.reg.Num()
}