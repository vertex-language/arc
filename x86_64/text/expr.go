// x86_64/text/expr.go
package text

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vertex-language/arc/x86_64/operand"
)

// Expr is an expression tree. It is precedence-free: gas puts bitwise
// operators above additive ones and NASM does not, and both disagreements
// are settled by their own parsers before anything reaches here. A tree
// built from one dialect and printed in the other comes out with whatever
// parentheses that dialect needs, which is a spelling change and not a
// meaning one.
type Expr interface {
	Pos() Pos
	expr()
}

// Num is an integer literal. Base is kept so a formatter can print 0x10 back
// as 0x10 rather than as 16 — a change that assembles identically and reads
// differently, which is exactly the kind of change `arc fmt` must not make.
type Num struct {
	Position Pos
	Value    int64
	Base     int
}

func (n *Num) Pos() Pos { return n.Position }
func (*Num) expr()      {}

// Sym is a reference to a symbol or label.
//
// Reloc is the relocation kind the source asked for: gas spells it
// `puts@PLT` and NASM spells it `puts wrt ..plt`, and both mean the same
// entry in the same table. Carrying the kind rather than the spelling is
// what lets one file be printed as the other.
type Sym struct {
	Position Pos
	Name     string
	Reloc    operand.RelocKind

	// Backward and Forward mark gas's numeric label references, `1b` and
	// `1f`. They resolve against the nearest matching numeric label in that
	// direction, which is a question about position rather than about
	// names, so it cannot be answered by a symbol table.
	Backward bool
	Forward  bool
}

func (s *Sym) Pos() Pos { return s.Position }
func (*Sym) expr()      {}

// Dot is the current location counter: `.` in gas, `$` in NASM.
//
// Its value is the offset of the statement it appears in, which is why
// `.long . - msg` needs the assembler and not this evaluator.
type Dot struct{ Position Pos }

func (d *Dot) Pos() Pos { return d.Position }
func (*Dot) expr()      {}

// Here is NASM's `$$`: the start of the current section. gas has no
// spelling for it and a unit containing one cannot be printed as gas
// without a section-relative subtraction the gas printer builds instead.
type Here struct{ Position Pos }

func (h *Here) Pos() Pos { return h.Position }
func (*Here) expr()      {}

// Op is a binary or unary operator.
type Op uint8

const (
	OpAdd Op = iota
	OpSub
	OpMul
	OpDiv  // signed division: gas's /, NASM's //
	OpUDiv // unsigned division: NASM's /
	OpMod  // signed remainder
	OpUMod
	OpShl
	OpShr  // logical
	OpSar  // arithmetic
	OpAnd
	OpOr
	OpXor
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	OpLAnd
	OpLOr

	OpNeg // unary
	OpPos
	OpNot // ~
	OpLNot
)

var opNames = [...]string{
	OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpUDiv: "/u",
	OpMod: "%", OpUMod: "%u", OpShl: "<<", OpShr: ">>", OpSar: ">>s",
	OpAnd: "&", OpOr: "|", OpXor: "^",
	OpEq: "==", OpNe: "!=", OpLt: "<", OpLe: "<=", OpGt: ">", OpGe: ">=",
	OpLAnd: "&&", OpLOr: "||",
	OpNeg: "-", OpPos: "+", OpNot: "~", OpLNot: "!",
}

func (o Op) String() string {
	if int(o) >= len(opNames) {
		return "?"
	}
	return opNames[o]
}

// Binary is an operator applied to two expressions.
type Binary struct {
	Position Pos
	Op       Op
	X, Y     Expr

	// Paren records that the source parenthesized this node. Both dialects
	// need it to print an expression back at the same precedence they read
	// it at, and dropping it is how a formatter changes `(a|b)+c` into
	// something that assembles differently under gas.
	Paren bool
}

func (b *Binary) Pos() Pos { return b.Position }
func (*Binary) expr()      {}

// Unary is an operator applied to one expression.
type Unary struct {
	Position Pos
	Op       Op
	X        Expr
}

func (u *Unary) Pos() Pos { return u.Position }
func (*Unary) expr()      {}

// Env is what an expression needs from outside itself to have a value.
//
// A nil Env is legal and means "constants only", which is what a parser
// checking a shift count or an alignment has. Anything symbolic then fails
// with ErrNotConstant, which is the answer, not a shortcoming.
type Env interface {
	// Lookup is the value of a symbol, and whether it has one. An absolute
	// symbol — a .equ — has one; a label does not until it has an address.
	Lookup(name string) (int64, bool)

	// Dot is the current location, and whether there is one.
	Dot() (int64, bool)

	// SectionStart is the start of the current section, for NASM's $$.
	SectionStart() (int64, bool)
}

var (
	// ErrNotConstant is an expression whose value depends on something the
	// Env cannot supply — usually an address that has not been assigned.
	ErrNotConstant = errors.New("expression is not a constant")

	// ErrDivideByZero is what it says. It is a parse-time error when the
	// divisor is a literal and an assemble-time one when it is not.
	ErrDivideByZero = errors.New("division by zero")
)

// Eval computes an expression's value, or reports why it has none.
//
// The arithmetic is 64-bit two's complement and wraps, because that is what
// both assemblers do and what the fields the value lands in hold. Range
// checking against a field belongs to encode/, which knows how wide the
// field is; this does not.
func Eval(e Expr, env Env) (int64, error) {
	switch x := e.(type) {
	case *Num:
		return x.Value, nil

	case *Sym:
		if env == nil {
			return 0, Wrap(x.Position, ErrNotConstant)
		}
		v, ok := env.Lookup(x.Name)
		if !ok {
			return 0, Errorf(x.Position, "%s has no value here", x.Name)
		}
		return v, nil

	case *Dot:
		if env == nil {
			return 0, Wrap(x.Position, ErrNotConstant)
		}
		v, ok := env.Dot()
		if !ok {
			return 0, Errorf(x.Position, "the location counter has no value here")
		}
		return v, nil

	case *Here:
		if env == nil {
			return 0, Wrap(x.Position, ErrNotConstant)
		}
		v, ok := env.SectionStart()
		if !ok {
			return 0, Errorf(x.Position, "no current section")
		}
		return v, nil

	case *Unary:
		v, err := Eval(x.X, env)
		if err != nil {
			return 0, err
		}
		switch x.Op {
		case OpNeg:
			return -v, nil
		case OpPos:
			return v, nil
		case OpNot:
			return ^v, nil
		case OpLNot:
			return b2i(v == 0), nil
		}
		return 0, Errorf(x.Position, "unary %s is not an operator here", x.Op)

	case *Binary:
		l, err := Eval(x.X, env)
		if err != nil {
			return 0, err
		}
		r, err := Eval(x.Y, env)
		if err != nil {
			return 0, err
		}
		return apply(x.Position, x.Op, l, r)
	}
	return 0, fmt.Errorf("unknown expression node %T", e)
}

func apply(p Pos, op Op, l, r int64) (int64, error) {
	switch op {
	case OpAdd:
		return l + r, nil
	case OpSub:
		return l - r, nil
	case OpMul:
		return l * r, nil

	case OpDiv, OpMod, OpUDiv, OpUMod:
		if r == 0 {
			return 0, Wrap(p, ErrDivideByZero)
		}
		// gas divides signed and NASM divides unsigned, which differ for
		// exactly one operand in a million and then differ by 2^64. Two
		// operators rather than one flag, because a tree with a flag on it
		// would print back into the wrong dialect.
		switch op {
		case OpDiv:
			return l / r, nil
		case OpMod:
			return l % r, nil
		case OpUDiv:
			return int64(uint64(l) / uint64(r)), nil
		default:
			return int64(uint64(l) % uint64(r)), nil
		}

	case OpShl:
		return l << uint64(r), nil
	case OpShr:
		return int64(uint64(l) >> uint64(r)), nil
	case OpSar:
		return l >> uint64(r), nil

	case OpAnd:
		return l & r, nil
	case OpOr:
		return l | r, nil
	case OpXor:
		return l ^ r, nil

	case OpEq:
		return b2i(l == r), nil
	case OpNe:
		return b2i(l != r), nil
	case OpLt:
		return b2i(l < r), nil
	case OpLe:
		return b2i(l <= r), nil
	case OpGt:
		return b2i(l > r), nil
	case OpGe:
		return b2i(l >= r), nil
	case OpLAnd:
		return b2i(l != 0 && r != 0), nil
	case OpLOr:
		return b2i(l != 0 || r != 0), nil
	}
	return 0, Errorf(p, "%s is not a binary operator", op)
}

// b2i is the truth value both assemblers agree on: -1 for true. NASM and gas
// both return all-ones rather than one, and a caller writing `x == 1 & mask`
// depends on it.
func b2i(b bool) int64 {
	if b {
		return -1
	}
	return 0
}

// Value is what an expression reduces to when it is not a constant: a
// constant part, plus at most one symbol added and at most one subtracted.
//
// This is the whole vocabulary a relocation has. `msg + 4` is a symbol and a
// constant; `. - msg` is one symbol subtracted from another and folds to a
// constant when both land in the same section. Anything with two symbols
// added, or a symbol multiplied, has no relocation that can express it and
// is refused here rather than miswritten later.
type Value struct {
	Const int64
	Add   string // the symbol whose address is added, or ""
	Sub   string // the symbol whose address is subtracted, or ""
	Reloc operand.RelocKind

	// Dot marks that the location counter took part, which the assembler
	// resolves against the statement's own offset.
	Dot bool
}

// IsConst reports whether the value needs no fixup.
func (v Value) IsConst() bool { return v.Add == "" && v.Sub == "" && !v.Dot }

func (v Value) String() string {
	var b strings.Builder
	if v.Dot {
		b.WriteString(".")
	}
	if v.Add != "" {
		if b.Len() > 0 {
			b.WriteString("+")
		}
		b.WriteString(v.Add)
	}
	if v.Sub != "" {
		b.WriteString("-" + v.Sub)
	}
	if v.Const != 0 || b.Len() == 0 {
		if b.Len() > 0 && v.Const > 0 {
			b.WriteString("+")
		}
		b.WriteString(strconv.FormatInt(v.Const, 10))
	}
	return b.String()
}

// Reduce evaluates as much of an expression as it can and reports the
// symbolic residue.
//
// This is the path `.quad . - msg` needs and that Assemble does not yet
// wire up: the residue is exactly a fixup's worth of information, and what
// is missing is the backpatch that consumes it, not the analysis that
// produces it.
func Reduce(e Expr, env Env) (Value, error) {
	switch x := e.(type) {
	case *Num:
		return Value{Const: x.Value}, nil

	case *Dot:
		if env != nil {
			if v, ok := env.Dot(); ok {
				return Value{Const: v}, nil
			}
		}
		return Value{Dot: true}, nil

	case *Sym:
		if env != nil {
			if v, ok := env.Lookup(x.Name); ok {
				return Value{Const: v}, nil
			}
		}
		return Value{Add: x.Name, Reloc: x.Reloc}, nil

	case *Here:
		if env != nil {
			if v, ok := env.SectionStart(); ok {
				return Value{Const: v}, nil
			}
		}
		return Value{}, Errorf(x.Position, "$$ has no value here")

	case *Unary:
		v, err := Reduce(x.X, env)
		if err != nil {
			return Value{}, err
		}
		switch x.Op {
		case OpPos:
			return v, nil
		case OpNeg:
			if !v.IsConst() {
				// Negating a symbol has no relocation. Subtracting one does,
				// which is why the tree keeps Add and Sub apart.
				if v.Add != "" && v.Sub == "" {
					return Value{Const: -v.Const, Sub: v.Add, Reloc: v.Reloc}, nil
				}
				return Value{}, Errorf(x.Position, "cannot negate a relocatable expression")
			}
			return Value{Const: -v.Const}, nil
		}
		if !v.IsConst() {
			return Value{}, Errorf(x.Position, "%s needs a constant operand", x.Op)
		}
		c, err := Eval(&Unary{Position: x.Position, Op: x.Op,
			X: &Num{Position: x.Position, Value: v.Const}}, nil)
		return Value{Const: c}, err

	case *Binary:
		l, err := Reduce(x.X, env)
		if err != nil {
			return Value{}, err
		}
		r, err := Reduce(x.Y, env)
		if err != nil {
			return Value{}, err
		}

		if l.IsConst() && r.IsConst() {
			c, err := apply(x.Position, x.Op, l.Const, r.Const)
			return Value{Const: c}, err
		}

		switch x.Op {
		case OpAdd:
			return addValues(x.Position, l, r)
		case OpSub:
			return subValues(x.Position, l, r)
		}
		return Value{}, Errorf(x.Position,
			"%s has no relocation: only addition and subtraction of symbols do", x.Op)
	}
	return Value{}, fmt.Errorf("unknown expression node %T", e)
}

func addValues(p Pos, l, r Value) (Value, error) {
	if l.Add != "" && r.Add != "" {
		return Value{}, Errorf(p, "cannot add two symbol addresses")
	}
	if l.Sub != "" && r.Sub != "" {
		return Value{}, Errorf(p, "cannot subtract two symbol addresses")
	}
	out := Value{
		Const: l.Const + r.Const,
		Add:   pick(l.Add, r.Add),
		Sub:   pick(l.Sub, r.Sub),
		Dot:   l.Dot != r.Dot,
		Reloc: pickReloc(l.Reloc, r.Reloc),
	}
	if l.Dot && r.Dot {
		return Value{}, Errorf(p, "the location counter cannot be added to itself")
	}
	return out, nil
}

func subValues(p Pos, l, r Value) (Value, error) {
	if r.Sub != "" {
		return Value{}, Errorf(p, "cannot subtract a symbol difference")
	}
	if l.Sub != "" && r.Add != "" {
		return Value{}, Errorf(p, "cannot subtract two symbol addresses")
	}
	out := Value{
		Const: l.Const - r.Const,
		Add:   l.Add,
		Sub:   pick(l.Sub, r.Add),
		Reloc: pickReloc(l.Reloc, r.Reloc),
	}
	switch {
	case l.Dot && r.Dot:
		out.Dot = false
	case l.Dot:
		out.Dot = true
	case r.Dot:
		if out.Sub != "" {
			return Value{}, Errorf(p, "cannot subtract both a symbol and the location counter")
		}
		return Value{}, Errorf(p, "cannot subtract the location counter from a symbol")
	}
	return out, nil
}

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func pickReloc(a, b operand.RelocKind) operand.RelocKind {
	if a != operand.RelocNone {
		return a
	}
	return b
}

// x86_64/text/expr.go — replace b2i with these two.

// cmp is the truth value of a comparison: -1 for true. Both assemblers
// agree, and a caller writing `(x == 1) & mask` depends on it.
func cmp(b bool) int64 {
	if b {
		return -1
	}
	return 0
}

// logical is the truth value of && and ||, which is 1 rather than -1.
// The asymmetry is gas's and it is deliberate on gas's part; a tree that
// evened it out would assemble differently from the assembler it claims to
// round-trip against.
func logical(b bool) int64 {
	if b {
		return 1
	}
	return 0
}