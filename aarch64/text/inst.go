package text

import (
	"github.com/vertex-language/arc/aarch64/feature"
	"github.com/vertex-language/arc/aarch64/isa"
)

// Inst is one instruction as written.
type Inst struct {
	// Mnem is the mnemonic, lower case, as written. It is what the caller
	// named and is never rewritten: an alias stays the alias, because Emit
	// picks an encoding of the instruction you named and a tree that quietly
	// replaced cmp with subs would have made that choice on the caller's
	// behalf before the encoder was reached.
	Mnem string

	// Ops are the operands in source order.
	Ops []*Operand

	// Form is the encoding this instruction resolved to, or nil.
	//
	// It is on the tree rather than computed at use because two consumers need
	// the same answer: the encoder, and anything printing the instruction back
	// out. On x86_64 that second consumer is load-bearing — a width neither
	// dialect states outright has to come from somewhere — and here it is
	// merely cheaper, since one syntax means printing never needs a fact the
	// source does not carry. Storing it anyway keeps the two trees the same
	// shape and keeps a resolve from happening twice with two feature sets.
	Form *isa.Form

	P       Pos
	Comment string
}

func (in *Inst) Pos() Pos { return in.P }
func (*Inst) node()       {}

// Args builds the isa.Arg list that Resolve matches against.
//
// It needs an Env because a slot's acceptance can depend on an immediate's
// value, and an immediate can be an expression. An expression that does not
// evaluate contributes a zero, which is right: a symbolic operand is a label or
// a reference, and no form's acceptance turns on which address it is.
func (in *Inst) Args(env Env) ([]isa.Arg, error) {
	out := make([]isa.Arg, 0, len(in.Ops))
	for _, o := range in.Ops {
		a, err := argOf(o, env)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func argOf(o *Operand, env Env) (isa.Arg, error) {
	switch o.Kind {
	case OpReg:
		v, err := o.lowerReg()
		if err != nil {
			return isa.Arg{}, err
		}
		return isa.ArgOf(v.(interface {
			Num() uint16
			Bits() uint16
			Class() reg2Class
			String() string
		})), nil
	}
	return isa.Arg{}, nil
}

// Resolve fills Form.
//
// It is separate from Lower because the two answer to different owners: the
// form is the architecture's, chosen from the operand classes and the active
// feature set, and the values are the source's. A caller wanting the resolved
// tree without encoding it — arc fmt does — stops here.
func (in *Inst) Resolve(set feature.Set, env Env) error {
	if in.Form != nil {
		return nil
	}
	args, err := in.Args(env)
	if err != nil {
		return err
	}
	f, err := isa.Resolve(in.Mnem, args, set)
	if err != nil {
		return err
	}
	in.Form = f
	return nil
}

// Lower turns every operand into the values the encoder accepts.
//
// The result is []any rather than a typed list because the encoder's own входной
// vocabulary is a closed type switch over a dozen concrete types, and naming
// that union here would mean declaring an interface every one of them has to
// satisfy — which is the loose Operand interface the x86_64 README argues buys
// nothing toward exhaustiveness. The type switch in encode/ is still what
// actually decides.
func (in *Inst) Lower(env Env) ([]any, error) {
	out := make([]any, 0, len(in.Ops))
	for _, o := range in.Ops {
		v, err := o.Lower(env)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}