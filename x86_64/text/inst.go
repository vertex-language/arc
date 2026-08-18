// x86_64/text/inst.go
package text

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Inst is one instruction as written.
//
// Operands are in Intel order — destination first — whatever the source
// dialect was. gas reverses at its edges, in its parser and its printer, and
// nothing between them ever sees AT&T order. Storing "as written" would mean
// every consumer had to ask which dialect it came from, which is the
// question this tree exists to stop anyone asking.
type Inst struct {
	Position Pos

	// Mnemonic is lowercase and canonical: the name isa/ knows. An alias —
	// gas's `jz`, its `cltq`, NASM's `jnae` — resolves at the parser's
	// boundary and does not survive into the tree, because two spellings of
	// one encoding would be two rows nothing distinguishes.
	Mnemonic string

	Operands []*Operand

	// Size is an operand size the source stated on the mnemonic rather than
	// on an operand: gas's `movq`. It means the same thing as a NASM
	// keyword on the memory operand, and printers put it wherever their
	// dialect puts it.
	Size operand.Width

	// Prefixes are the ones the source wrote: lock, rep, repne, and a
	// segment override written as a prefix rather than on the operand.
	Prefixes []Prefix

	// The EVEX decorations, as written. Mask is K0 for none, which is also
	// what "no mask" encodes as, so the two need no distinction.
	Mask      reg.K
	Zero      bool
	Broadcast bool
	Round     Round
	SAE       bool

	Comment string

	// Form is the encoding this instruction resolved to, cached by whoever
	// resolved it. It is nil in a freshly parsed unit.
	//
	// A printer needs it: gas spells a size as a mnemonic suffix and NASM
	// as an operand keyword, and going from one to the other means knowing
	// the operand size, which the source may not have stated. That is why
	// `arc fmt` resolves before it prints and why this field exists on a
	// tree that is otherwise pure syntax.
	Form *isa.Form
}

func (i *Inst) Pos() Pos { return i.Position }
func (*Inst) node()      {}

// Prefix is a prefix written in the source.
type Prefix uint8

const (
	Lock Prefix = iota
	Rep
	RepNE
	Bnd
)

func (p Prefix) String() string {
	switch p {
	case Lock:
		return "lock"
	case Rep:
		return "rep"
	case RepNE:
		return "repne"
	case Bnd:
		return "bnd"
	}
	return "?"
}

// Round is embedded rounding control.
type Round uint8

const (
	RoundNone Round = iota
	RoundNearest
	RoundDown
	RoundUp
	RoundZero
)

func (r Round) String() string {
	switch r {
	case RoundNearest:
		return "rn-sae"
	case RoundDown:
		return "rd-sae"
	case RoundUp:
		return "ru-sae"
	case RoundZero:
		return "rz-sae"
	}
	return ""
}

// IsBranch reports whether a mnemonic's forms take a branch displacement.
//
// Both parsers need this and neither should answer it: NASM writes `call
// foo` and `mov rax, foo` with the same operand syntax, so whether a bare
// symbol is a target or an immediate is a fact about the instruction. It is
// isa/'s fact, and this is where the parsers ask for it.
func IsBranch(mnemonic string) bool {
	for _, f := range isa.Forms(mnemonic) {
		if f.Attrs&isa.Branch != 0 {
			return true
		}
	}
	return false
}

// Known reports whether the mnemonic has any form at all, at any feature
// level. A parser uses it to tell a typo from a gated instruction: an
// unknown mnemonic is a parse error and a gated one is not.
func Known(mnemonic string) bool { return len(isa.Forms(mnemonic)) > 0 }

// OperandSize is the width the instruction operates at, from whatever stated
// it: the mnemonic suffix, an operand keyword, or a register operand that
// fixes it by being what it is.
//
// It returns WidthNone when nothing states one, which is a legal state and
// not an error — `ret` has no operand size and `mov [rbx], rax` states one
// twice, once redundantly.
func (i *Inst) OperandSize() operand.Width {
	if i.Size != operand.WidthNone {
		return i.Size
	}
	for _, o := range i.Operands {
		switch o.Kind {
		case KindReg:
			if b := o.Reg.Bits(); b > 0 {
				return operand.Width(b)
			}
		case KindMem:
			if o.Size != operand.WidthNone {
				return o.Size
			}
		}
	}
	return operand.WidthNone
}

// Sized returns a copy of the operands with any unsized memory reference
// given the width the instruction operates at.
//
// This is the step between parsing and resolving. A NASM source that wrote
// `mov qword [rbx], 1` states the size on the operand; a gas source that
// wrote `movq $1, (%rbx)` states it on the mnemonic; both mean an M64, and
// isa.Resolve only matches one of them until this has run.
func (i *Inst) Sized() []*Operand {
	w := i.OperandSize()
	if w == operand.WidthNone {
		return i.Operands
	}
	out := make([]*Operand, len(i.Operands))
	for n, o := range i.Operands {
		if o.Kind == KindMem && o.Size == operand.WidthNone {
			c := *o
			c.Size = w
			out[n] = &c
			continue
		}
		out[n] = o
	}
	return out
}

// Lower turns the instruction's operands into the values encode/ takes.
func (i *Inst) Lower(env Env) ([]any, error) {
	ops := i.Sized()
	out := make([]any, 0, len(ops))
	for _, o := range ops {
		v, err := o.Lower(env)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if i.Mask != reg.K0 {
		// The writemask is an operand of the form, in EVEX.aaa, and it sits
		// after the destination. The syntax writes it as a decoration and
		// the table declares it as a slot, so it is spliced in here.
		out = append(out[:1], append([]any{i.Mask}, out[1:]...)...)
	}
	return out, nil
}

// String is a diagnostic rendering, in Intel order with neutral punctuation.
func (i *Inst) String() string {
	var b strings.Builder
	for _, p := range i.Prefixes {
		b.WriteString(p.String() + " ")
	}
	b.WriteString(i.Mnemonic)
	for n, o := range i.Operands {
		if n == 0 {
			b.WriteString(" ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(o.String())
		if n == 0 && i.Mask != reg.K0 {
			b.WriteString("{" + i.Mask.Name() + "}")
			if i.Zero {
				b.WriteString("{z}")
			}
		}
	}
	if i.Broadcast {
		b.WriteString(" {1toN}")
	}
	if r := i.Round.String(); r != "" {
		b.WriteString(" {" + r + "}")
	}
	return b.String()
}