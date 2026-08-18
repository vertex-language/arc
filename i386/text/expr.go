package text

import "fmt"

// The expression tree, and the arithmetic over it.
//
// Both dialects have an expression grammar and it is the same arithmetic over
// the same tree, which is why this file sits beside directive.go and the two
// surface syntaxes parse into it.
//
// What is NOT here is precedence, and the reason is that the two dialects do
// not agree. GNU as puts the bitwise operators above addition — its levels
// are * / % << >>, then | & ^, then + -, then the comparisons, then the
// logical operators. NASM uses the C-like order with | at the bottom. So:
//
//	1 | 2 + 3      is  (1|2)+3 = 6   in GAS
//	1 | 2 + 3      is  1|(2+3) = 5   in NASM
//
// One number, two syntaxes, two answers. A precedence table in this package
// would have to be one of them and would silently mis-parse the other, so
// each parser carries its own and hands this package a tree that is already
// bound. A printer parenthesises by comparing its own table against the tree,
// which is how `1|2+3` parsed from GAS prints as `(1|2)+3` in NASM — the same
// value, spelled so that NASM's binding produces it.
//
// There is no Paren node for the same reason: parentheses are a spelling of
// a shape the tree already has, and a node for them would give one expression
// two representations that print differently and mean the same.

// Expr is an expression.
type Expr interface {
	Pos() Pos
	expr()
}

// Int is a literal. Radix is not recorded: 0x10 and 16 are one number, and a
// formatter that preserved the base would be preserving a spelling that
// carries no information the value does not.
type Int struct {
	P     Pos
	Value int64
}

func (e *Int) Pos() Pos { return e.P }
func (*Int) expr()      {}

// SymExpr is a reference to a name, with the relocation modifier written on
// it. The modifier belongs here rather than on the operand because that is
// where the syntax puts it: `msg@GOTOFF(%ebx)` modifies the symbol, not the
// address.
type SymExpr struct {
	P    Pos
	Name string
	Mod  Modifier
}

func (e *SymExpr) Pos() Pos { return e.P }
func (*SymExpr) expr()      {}

// Here is the location counter: '.' in GAS, '$' in NASM.
//
// It is a term rather than a value because its value is not known until the
// assembler places the statement. `msg - . + 4` evaluates to a PCRelative
// value here, and the assembler finishes it with an offset this package does
// not have.
type Here struct{ P Pos }

func (e *Here) Pos() Pos { return e.P }
func (*Here) expr()      {}

// Start is the start of the current section: NASM's '$$'.
//
// GAS has no one-token spelling and writes the section symbol instead, so a
// GAS printer emits the section name. The node exists because `times
// 510-($-$$) db 0` is how every boot sector is written and dropping it would
// mean the flat-image example in the README could not be assembled.
type Start struct{ P Pos }

func (e *Start) Pos() Pos { return e.P }
func (*Start) expr()      {}

// Unary is a prefix operation. Both dialects have exactly these two and both
// require an absolute argument.
type Unary struct {
	P  Pos
	Op UnaryOp
	X  Expr
}

func (e *Unary) Pos() Pos { return e.P }
func (*Unary) expr()      {}

// Binary is an infix operation. The tree is already bound by the parser's own
// precedence; this node records no precedence of its own.
type Binary struct {
	P    Pos
	Op   BinaryOp
	X, Y Expr
}

func (e *Binary) Pos() Pos { return e.P }
func (*Binary) expr()      {}

// UnaryOp is a prefix operator.
type UnaryOp uint8

const (
	Neg UnaryOp = iota // -x, two's complement negation
	Not                // ~x, complementation
	LNot               // !x, logical negation
)

var unaryNames = [...]string{"-", "~", "!"}

func (o UnaryOp) String() string {
	if int(o) < len(unaryNames) {
		return unaryNames[o]
	}
	return "?"
}

// BinaryOp is an infix operator.
type BinaryOp uint8

const (
	Add BinaryOp = iota
	Sub
	Mul
	Div
	Mod
	Shl
	Shr
	And
	Or
	Xor
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
	LAnd
	LOr
)

var binaryNames = [...]string{
	"+", "-", "*", "/", "%", "<<", ">>", "&", "|", "^",
	"==", "!=", "<", "<=", ">", ">=", "&&", "||",
}

func (o BinaryOp) String() string {
	if int(o) < len(binaryNames) {
		return binaryNames[o]
	}
	return "?"
}

// Relocatable reports whether the operator may have a non-absolute operand.
//
// Only addition and subtraction may: apart from those, both arguments must be
// absolute and the result is absolute. This is GNU as's own rule and it is
// what makes the value model below finite — without it an expression could
// hold a symbol multiplied by another and there would be no relocation that
// means it.
func (o BinaryOp) Relocatable() bool { return o == Add || o == Sub }

// Term is one symbolic contribution to a value.
type Term struct {
	Name string
	Mod  Modifier
	Here bool
	Sign int // +1 or -1
}

// Value is the result of evaluating an expression: a constant plus a set of
// symbolic terms that did not cancel.
//
// The model is GNU as's. A value is absolute when nothing symbolic survives,
// and otherwise it is one of three shapes the psABI has a relocation for.
// Anything else — two positive symbols, a symbol times two — is an error,
// because there is no relocation that means it and producing bytes would mean
// guessing.
type Value struct {
	Const int64
	Terms []Term
}

// Kind classifies a value.
type Kind uint8

const (
	// Absolute is a number. It needs no relocation.
	Absolute Kind = iota

	// Relocatable is sym + const: one symbol, positive. This is .long msg.
	Relocatable

	// PCRelative is sym - . + const: a symbol against the location counter.
	// The assembler finishes it, because it knows where the field sits.
	PCRelative

	// Difference is symA - symB + const. It folds to a constant when both
	// are defined in the same section and is otherwise an error — you may
	// not subtract arguments from different sections.
	Difference

	// Invalid is anything else.
	Invalid
)

var kindNames = [...]string{"absolute", "relocatable", "pc-relative", "difference", "invalid"}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "?"
}

// Kind classifies v.
func (v Value) Kind() Kind {
	switch len(v.Terms) {
	case 0:
		return Absolute
	case 1:
		if v.Terms[0].Sign == 1 && !v.Terms[0].Here {
			return Relocatable
		}
	case 2:
		a, b := v.Terms[0], v.Terms[1]
		if a.Sign == -1 {
			a, b = b, a
		}
		if a.Sign != 1 || b.Sign != -1 || a.Here {
			return Invalid
		}
		if b.Here {
			return PCRelative
		}
		return Difference
	}
	return Invalid
}

// Sym returns the positive symbol term, for a Relocatable, PCRelative or
// Difference value.
func (v Value) Sym() (name string, mod Modifier, ok bool) {
	for _, t := range v.Terms {
		if t.Sign == 1 && !t.Here {
			return t.Name, t.Mod, true
		}
	}
	return "", ModNone, false
}

// Minus returns the negative symbol term of a Difference.
func (v Value) Minus() (name string, ok bool) {
	for _, t := range v.Terms {
		if t.Sign == -1 && !t.Here {
			return t.Name, true
		}
	}
	return "", false
}

// IsAbs reports whether the value is a plain number.
func (v Value) IsAbs() bool { return len(v.Terms) == 0 }

func (v Value) String() string {
	s := ""
	for _, t := range v.Terms {
		name := t.Name
		if t.Here {
			name = "."
		}
		if t.Mod != ModNone {
			name += "@" + t.Mod.String()
		}
		switch {
		case s == "" && t.Sign == 1:
			s = name
		case t.Sign == 1:
			s += "+" + name
		default:
			s += "-" + name
		}
	}
	switch {
	case s == "":
		return fmt.Sprintf("%d", v.Const)
	case v.Const == 0:
		return s
	}
	return fmt.Sprintf("%s%+d", s, v.Const)
}

// Lookup resolves a name to a constant. It answers for .equ and NASM's equ,
// and fails for anything whose value is an address — those become terms.
type Lookup func(name string) (int64, bool)

// Eval reduces an expression to a Value.
//
// Symbols that lookup resolves are folded to constants; the rest survive as
// terms. Evaluation is total over the tree: every operator is applied or the
// call returns a diagnostic, so a caller never has to re-walk to find out
// whether something was foldable.
func Eval(e Expr, lookup Lookup) (Value, *Error) {
	if lookup == nil {
		lookup = func(string) (int64, bool) { return 0, false }
	}
	return eval(e, lookup, 0)
}

// maxDepth bounds recursion. An expression this deep is not a program, and a
// parser bug that built a cycle should be a diagnostic rather than a stack
// overflow in a tool that runs in CI.
const maxDepth = 200

func eval(e Expr, lookup Lookup, depth int) (Value, *Error) {
	if depth > maxDepth {
		return Value{}, Errorf(e.Pos(), "expression nested more than %d deep", maxDepth)
	}

	switch e := e.(type) {
	case *Int:
		return Value{Const: e.Value}, nil

	case *SymExpr:
		if v, ok := lookup(e.Name); ok && e.Mod == ModNone {
			return Value{Const: v}, nil
		}
		return Value{Terms: []Term{{Name: e.Name, Mod: e.Mod, Sign: 1}}}, nil

	case *Here:
		return Value{Terms: []Term{{Here: true, Sign: 1}}}, nil

	case *Start:
		// The section start is a symbol whose name only the assembler knows,
		// so it survives as a term with an empty name and is resolved above.
		return Value{Terms: []Term{{Name: "", Sign: 1}}}, nil

	case *Unary:
		x, err := eval(e.X, lookup, depth+1)
		if err != nil {
			return Value{}, err
		}
		if !x.IsAbs() {
			return Value{}, Errorf(e.P, "%s requires an absolute value", e.Op).
				Note("only + and - accept a symbol; every other operator needs a number")
		}
		switch e.Op {
		case Neg:
			return Value{Const: -x.Const}, nil
		case Not:
			return Value{Const: ^x.Const}, nil
		case LNot:
			return Value{Const: b2i(x.Const == 0, 1)}, nil
		}
		return Value{}, Errorf(e.P, "unknown unary operator")

	case *Binary:
		x, err := eval(e.X, lookup, depth+1)
		if err != nil {
			return Value{}, err
		}
		y, err := eval(e.Y, lookup, depth+1)
		if err != nil {
			return Value{}, err
		}
		return apply(e, x, y)
	}

	return Value{}, Errorf(e.Pos(), "not an expression")
}

func apply(e *Binary, x, y Value) (Value, *Error) {
	if !e.Op.Relocatable() && (!x.IsAbs() || !y.IsAbs()) {
		return Value{}, Errorf(e.P, "%s requires absolute arguments", e.Op).
			Note("apart from + and -, both arguments must be absolute").
			Note("the non-absolute side is %s", nonAbs(x, y))
	}

	switch e.Op {
	case Add:
		return normalize(e.P, Value{Const: x.Const + y.Const,
			Terms: append(append([]Term{}, x.Terms...), y.Terms...)})

	case Sub:
		neg := make([]Term, len(y.Terms))
		for i, t := range y.Terms {
			t.Sign = -t.Sign
			neg[i] = t
		}
		return normalize(e.P, Value{Const: x.Const - y.Const,
			Terms: append(append([]Term{}, x.Terms...), neg...)})

	case Mul:
		return Value{Const: x.Const * y.Const}, nil

	case Div, Mod:
		if y.Const == 0 {
			return Value{}, Errorf(e.P, "division by zero")
		}
		if e.Op == Div {
			return Value{Const: x.Const / y.Const}, nil
		}
		return Value{Const: x.Const % y.Const}, nil

	case Shl:
		if err := checkShift(e.P, y.Const); err != nil {
			return Value{}, err
		}
		return Value{Const: x.Const << uint(y.Const)}, nil

	case Shr:
		if err := checkShift(e.P, y.Const); err != nil {
			return Value{}, err
		}
		return Value{Const: x.Const >> uint(y.Const)}, nil

	case And:
		return Value{Const: x.Const & y.Const}, nil
	case Or:
		return Value{Const: x.Const | y.Const}, nil
	case Xor:
		return Value{Const: x.Const ^ y.Const}, nil

	// A comparison yields -1 for true and 0 for false, and the comparison is
	// signed. This is GNU as's rule rather than C's, and it is load-bearing:
	// `.long (a==b)` writes 0xffffffff where C would write 1.
	case Eq:
		return Value{Const: b2i(x.Const == y.Const, -1)}, nil
	case Ne:
		return Value{Const: b2i(x.Const != y.Const, -1)}, nil
	case Lt:
		return Value{Const: b2i(x.Const < y.Const, -1)}, nil
	case Le:
		return Value{Const: b2i(x.Const <= y.Const, -1)}, nil
	case Gt:
		return Value{Const: b2i(x.Const > y.Const, -1)}, nil
	case Ge:
		return Value{Const: b2i(x.Const >= y.Const, -1)}, nil

	// The logical operators yield 1 for true, not -1. The asymmetry with the
	// comparisons is GNU as's and is preserved rather than tidied, because
	// tidying it would change what a program assembles to.
	case LAnd:
		return Value{Const: b2i(x.Const != 0 && y.Const != 0, 1)}, nil
	case LOr:
		return Value{Const: b2i(x.Const != 0 || y.Const != 0, 1)}, nil
	}

	return Value{}, Errorf(e.P, "unknown binary operator")
}

// normalize cancels opposed terms and rejects a shape no relocation means.
//
// This is where `msg - msg` becomes 0 and where `msg + msg` becomes a
// diagnostic. Doing it at every + and - rather than once at the end is what
// keeps `(end - start) / 8` working: the division sees an absolute value
// because the subtraction already cancelled.
func normalize(p Pos, v Value) (Value, *Error) {
	out := v.Terms[:0:0]
	for _, t := range v.Terms {
		merged := false
		for i := range out {
			if out[i].Name == t.Name && out[i].Mod == t.Mod && out[i].Here == t.Here {
				out[i].Sign += t.Sign
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, t)
		}
	}

	kept := out[:0]
	for _, t := range out {
		switch {
		case t.Sign == 0:
			// Cancelled. `. - .` is zero and needs no relocation.
		case t.Sign != 1 && t.Sign != -1:
			return Value{}, Errorf(p, "symbol %s appears %d times in one expression",
				display(t), t.Sign).
				Note("no relocation multiplies a symbol")
		default:
			kept = append(kept, t)
		}
	}
	v.Terms = kept

	if v.Kind() == Invalid {
		return Value{}, Errorf(p, "expression is neither absolute nor relocatable").
			Note("the shapes with a relocation are sym+n, sym-.+n and symA-symB+n").
			Note("you may not add together arguments from different sections")
	}
	return v, nil
}

func checkShift(p Pos, n int64) *Error {
	if n < 0 || n > 63 {
		return Errorf(p, "shift count %d is out of range", n).
			Note("the count must be 0 through 63")
	}
	return nil
}

func nonAbs(x, y Value) string {
	if !x.IsAbs() {
		return x.String()
	}
	return y.String()
}

func display(t Term) string {
	if t.Here {
		return "."
	}
	return t.Name
}

func b2i(b bool, t int64) int64 {
	if b {
		return t
	}
	return 0
}