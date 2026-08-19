package text

import (
	"errors"
	"fmt"
)

// Expr is an expression: a directive's argument, an operand's immediate, or a
// memory displacement.
//
// The tree is not evaluated at parse time. `. - msg` is a legal expression
// whose value nothing knows until layout, and an evaluator that failed on it
// would refuse correct source; one that guessed would write wrong bytes. So
// parsing produces a tree, Eval answers when the answer is a number, and Reduce
// answers when it is not.
type Expr interface {
	Pos() Pos
	expr()
}

// Num is an integer literal.
type Num struct {
	Value int64
	P     Pos
}

func (n *Num) Pos() Pos { return n.P }
func (*Num) expr()      {}

// Sym is a symbol reference by name.
type Sym struct {
	Name string
	P    Pos

	// Local marks a reference to a numeric label: 1f is forward, 1b is
	// backward. Name holds the digits and Forward the direction, because the
	// two are different places and the name alone does not say which.
	Local   bool
	Forward bool
}

func (s *Sym) Pos() Pos { return s.P }
func (*Sym) expr()      {}

// Dot is the current location counter, `.`.
//
// It is a distinct node rather than a symbol named "." because its value
// depends on where in the section the expression appears, which no symbol table
// records and only the assembler walking statements knows.
type Dot struct{ P Pos }

func (d *Dot) Pos() Pos { return d.P }
func (*Dot) expr()      {}

// Op is a binary or unary operator.
//
// The set is gas's. Precedence is not here: it is a property of the syntax, and
// gas puts bitwise operators above additive ones where C puts them below. The
// parser applies it and the printer re-parenthesizes from it, both in text/gas,
// so the two cannot disagree with each other and neither can disagree with this
// tree, which stores structure rather than spelling.
type Op uint8

const (
	OpNone Op = iota

	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpShl
	OpShr
	OpAnd
	OpOr
	OpXor

	OpNeg // unary -
	OpNot // unary ~
	OpLogNot

	opCount
)

var opName = [opCount]string{
	OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpMod: "%",
	OpShl: "<<", OpShr: ">>", OpAnd: "&", OpOr: "|", OpXor: "^",
	OpNeg: "-", OpNot: "~", OpLogNot: "!",
}

func (o Op) String() string {
	if o >= opCount {
		return "?"
	}
	return opName[o]
}

// Unary reports whether the operator takes one operand.
func (o Op) Unary() bool { return o == OpNeg || o == OpNot || o == OpLogNot }

// Binary is an expression with two operands.
type Binary struct {
	Op   Op
	L, R Expr
	P    Pos
}

func (b *Binary) Pos() Pos { return b.P }
func (*Binary) expr()      {}

// Unary is an expression with one.
type Unary struct {
	Op Op
	X  Expr
	P  Pos
}

func (u *Unary) Pos() Pos { return u.P }
func (*Unary) expr()      {}

// Paren preserves an explicit grouping.
//
// It exists for the printer. Structure alone would let `(a + b) * c` and
// `a + b * c` print identically once precedence is reapplied — which is correct
// arithmetic and a formatting change a reader did not ask for. Eval and Reduce
// see through it.
type Paren struct {
	X Expr
	P Pos
}

func (p *Paren) Pos() Pos { return p.P }
func (*Paren) expr()      {}

// Env is what an expression needs that the tree does not carry: the values of
// symbols an earlier .equ defined, and where `.` currently is.
//
// It is an interface because the two callers want different things behind it.
// A resolver walking statements has a real location counter; a caller asking
// whether a constant expression is constant has neither, and passing nil is the
// spelling for that.
//
// This is the Env the arch README says Assemble runs without. Until it carries
// one, .equ, .comm and .lcomm are refused rather than half-supported.
type Env interface {
	// Value reports the value of a symbol, and whether it has one here. A
	// symbol defined as a label has no value until layout, so this reports
	// false for it — absent, not zero.
	Value(name string) (int64, bool)

	// Dot is the current location counter, and whether one exists.
	Dot() (int64, bool)
}

// ErrSymbolic is returned by Eval for an expression whose value depends on a
// symbol. It is not a failure: Reduce is the function that answers this case,
// and a caller that can consume a relocation calls that instead.
var ErrSymbolic = errors.New("expression is not a constant")

// Eval computes a constant value.
//
// env may be nil, which means no symbol has a value and there is no location
// counter — the right environment for asking whether something is constant on
// its own terms.
func Eval(e Expr, env Env) (int64, error) {
	switch x := e.(type) {
	case *Num:
		return x.Value, nil

	case *Paren:
		return Eval(x.X, env)

	case *Sym:
		if env != nil {
			if v, ok := env.Value(x.Name); ok {
				return v, nil
			}
		}
		return 0, fmt.Errorf("%w: %s", ErrSymbolic, x.Name)

	case *Dot:
		if env != nil {
			if v, ok := env.Dot(); ok {
				return v, nil
			}
		}
		return 0, fmt.Errorf("%w: .", ErrSymbolic)

	case *Unary:
		v, err := Eval(x.X, env)
		if err != nil {
			return 0, err
		}
		switch x.Op {
		case OpNeg:
			return -v, nil
		case OpNot:
			return ^v, nil
		case OpLogNot:
			if v == 0 {
				return 1, nil
			}
			return 0, nil
		}
		return 0, fmt.Errorf("unknown unary operator %s", x.Op)

	case *Binary:
		l, err := Eval(x.L, env)
		if err != nil {
			return 0, err
		}
		r, err := Eval(x.R, env)
		if err != nil {
			return 0, err
		}
		switch x.Op {
		case OpAdd:
			return l + r, nil
		case OpSub:
			return l - r, nil
		case OpMul:
			return l * r, nil
		case OpDiv:
			if r == 0 {
				return 0, errors.New("division by zero")
			}
			return l / r, nil
		case OpMod:
			if r == 0 {
				return 0, errors.New("division by zero")
			}
			return l % r, nil
		case OpShl:
			return l << uint(r), nil
		case OpShr:
			return l >> uint(r), nil
		case OpAnd:
			return l & r, nil
		case OpOr:
			return l | r, nil
		case OpXor:
			return l ^ r, nil
		}
		return 0, fmt.Errorf("unknown operator %s", x.Op)
	}
	return 0, errors.New("expression has no form")
}

// Value is what an expression reduces to when it is not a constant.
//
// The shape is not general, and the narrowness is the point: it is exactly what
// a relocation can express. A record names one symbol, optionally subtracts a
// second, and carries an addend — so a constant, at most one symbol added and
// at most one subtracted is the whole vocabulary, and an expression that
// reduces outside it has no encoding rather than an inconvenient one.
type Value struct {
	Const int64
	Plus  string
	Minus string

	// PlusDot and MinusDot mark `.` appearing where a symbol would. It is not a
	// symbol and cannot be spelled as one, but it occupies the same slot: `. -
	// msg` is a section-relative distance, which is the common case this whole
	// type exists for.
	PlusDot  bool
	MinusDot bool
}

// Constant reports whether the value is a plain number.
func (v Value) Constant() bool {
	return v.Plus == "" && v.Minus == "" && !v.PlusDot && !v.MinusDot
}

// Simple reports whether the value is a constant plus a single symbol, which is
// the shape a data fixup can consume directly.
func (v Value) Simple() bool {
	return v.Minus == "" && !v.MinusDot && !v.PlusDot && v.Plus != ""
}

// Difference reports whether the value is the distance between two places,
// which needs a paired relocation record on the formats that have one and is
// refused on the formats that do not.
func (v Value) Difference() bool {
	return (v.Plus != "" || v.PlusDot) && (v.Minus != "" || v.MinusDot)
}

func (v Value) String() string {
	s := ""
	switch {
	case v.PlusDot:
		s = "."
	case v.Plus != "":
		s = v.Plus
	}
	switch {
	case v.MinusDot:
		s += " - ."
	case v.Minus != "":
		s += " - " + v.Minus
	}
	if v.Const != 0 || s == "" {
		if s != "" && v.Const > 0 {
			s += " + "
		} else if s != "" {
			s += " - "
		}
		c := v.Const
		if s != "" && c < 0 {
			c = -c
		}
		s += fmt.Sprint(c)
	}
	return s
}

// Reduce computes the symbolic residue of an expression.
//
// It succeeds where Eval fails and produces the same answer where Eval
// succeeds. What it refuses is an expression whose residue does not fit a
// relocation: two symbols added, a symbol multiplied, a symbol shifted. Those
// are legal arithmetic with no way to record them, and the error says which
// rather than reporting the expression as merely non-constant.
func Reduce(e Expr, env Env) (Value, error) {
	switch x := e.(type) {
	case *Num:
		return Value{Const: x.Value}, nil

	case *Paren:
		return Reduce(x.X, env)

	case *Sym:
		if env != nil {
			if v, ok := env.Value(x.Name); ok {
				return Value{Const: v}, nil
			}
		}
		return Value{Plus: x.Name}, nil

	case *Dot:
		if env != nil {
			if v, ok := env.Dot(); ok {
				return Value{Const: v}, nil
			}
		}
		return Value{PlusDot: true}, nil

	case *Unary:
		v, err := Reduce(x.X, env)
		if err != nil {
			return Value{}, err
		}
		switch x.Op {
		case OpNeg:
			return v.negate()
		case OpNot, OpLogNot:
			if !v.Constant() {
				return Value{}, fmt.Errorf("cannot apply %s to a symbolic value", x.Op)
			}
			n, err := Eval(x, env)
			return Value{Const: n}, err
		}
		return Value{}, fmt.Errorf("unknown unary operator %s", x.Op)

	case *Binary:
		l, err := Reduce(x.L, env)
		if err != nil {
			return Value{}, err
		}
		r, err := Reduce(x.R, env)
		if err != nil {
			return Value{}, err
		}
		switch x.Op {
		case OpAdd:
			return l.add(r)
		case OpSub:
			neg, err := r.negate()
			if err != nil {
				return Value{}, err
			}
			return l.add(neg)
		}
		// Everything else needs both sides to be numbers. A symbol times two
		// is not something any relocation records, and there is no partial
		// answer worth returning.
		if !l.Constant() || !r.Constant() {
			return Value{}, fmt.Errorf("%s needs constant operands; a symbolic value "+
				"can only be added to or subtracted from", x.Op)
		}
		n, err := Eval(x, env)
		return Value{Const: n}, err
	}
	return Value{}, errors.New("expression has no form")
}

// add combines two residues, refusing the shapes a relocation cannot hold.
func (v Value) add(o Value) (Value, error) {
	out := Value{Const: v.Const + o.Const}

	plus := 0
	if v.Plus != "" || v.PlusDot {
		plus++
	}
	if o.Plus != "" || o.PlusDot {
		plus++
	}
	minus := 0
	if v.Minus != "" || v.MinusDot {
		minus++
	}
	if o.Minus != "" || o.MinusDot {
		minus++
	}

	// A symbol on one side cancelling one on the other is the case worth
	// getting right: msg + (x - msg) is a constant, and refusing it because
	// two symbols were added would be refusing arithmetic that works out.
	if v.Plus != "" && v.Plus == o.Minus {
		v.Plus, o.Minus = "", ""
		plus, minus = plus-1, minus-1
	}
	if o.Plus != "" && o.Plus == v.Minus {
		o.Plus, v.Minus = "", ""
		plus, minus = plus-1, minus-1
	}
	if v.PlusDot && o.MinusDot {
		v.PlusDot, o.MinusDot = false, false
		plus, minus = plus-1, minus-1
	}
	if o.PlusDot && v.MinusDot {
		o.PlusDot, v.MinusDot = false, false
		plus, minus = plus-1, minus-1
	}

	if plus > 1 {
		return Value{}, errors.New("two symbols added: a relocation names one symbol " +
			"and subtracts at most one other")
	}
	if minus > 1 {
		return Value{}, errors.New("two symbols subtracted: a relocation subtracts at most one")
	}

	out.Plus, out.PlusDot = pick(v.Plus, v.PlusDot, o.Plus, o.PlusDot)
	out.Minus, out.MinusDot = pick(v.Minus, v.MinusDot, o.Minus, o.MinusDot)
	return out, nil
}

func (v Value) negate() (Value, error) {
	if v.Plus != "" && v.Minus != "" {
		return Value{}, errors.New("negating a symbol difference has no relocation")
	}
	return Value{
		Const:    -v.Const,
		Plus:     v.Minus,
		PlusDot:  v.MinusDot,
		Minus:    v.Plus,
		MinusDot: v.PlusDot,
	}, nil
}

func pick(a string, aDot bool, b string, bDot bool) (string, bool) {
	switch {
	case a != "":
		return a, false
	case aDot:
		return "", true
	case b != "":
		return b, false
	case bDot:
		return "", true
	}
	return "", false
}

// Symbols lists every symbol an expression names, for Unit.Referenced.
func Symbols(e Expr) []string {
	var out []string
	Walk(e, func(x Expr) {
		if s, ok := x.(*Sym); ok && !s.Local {
			out = append(out, s.Name)
		}
	})
	return out
}

// Walk calls f on every node of an expression, parents first.
func Walk(e Expr, f func(Expr)) {
	if e == nil {
		return
	}
	f(e)
	switch x := e.(type) {
	case *Paren:
		Walk(x.X, f)
	case *Unary:
		Walk(x.X, f)
	case *Binary:
		Walk(x.L, f)
		Walk(x.R, f)
	}
}