package gas

import "github.com/vertex-language/arc/i386/text"

// This dialect's expression grammar: the same tree as NASM's, bound
// differently.
//
// GNU as puts the bitwise operators ABOVE addition. Its levels, tightest
// first, are:
//
//	* / % << >>
//	| & ^ !
//	+ -
//	== != < <= > >=
//	&&
//	||
//
// NASM's order is the C-like one with | at the bottom, so `1|2+3` is 6 here
// and 5 there. That divergence is the reason text/ holds the tree and the
// arithmetic but no precedence table: one shared table would have to be one
// of the two and would silently mis-parse the other.
//
// The '!' infix operator is GNU as's bitwise or-not, not a logical negation.
// It sits at the bitwise level with | & ^, and it is the one operator in this
// grammar that no other assembler spells the same way.

const (
	precOr      = iota + 1 // ||
	precAnd                // &&
	precCompare            // == != < <= > >=
	precAdd                // + -
	precBit                // | & ^ !
	precMul                // * / % << >>
	precUnary
)

var binaryPrec = map[string]int{
	"||": precOr,
	"&&": precAnd,
	"==": precCompare, "!=": precCompare,
	"<": precCompare, "<=": precCompare, ">": precCompare, ">=": precCompare,
	"+": precAdd, "-": precAdd,
	"|": precBit, "&": precBit, "^": precBit, "!": precBit,
	"*": precMul, "/": precMul, "%": precMul, "<<": precMul, ">>": precMul,
}

var binaryOp = map[string]text.BinaryOp{
	"+": text.Add, "-": text.Sub, "*": text.Mul, "/": text.Div, "%": text.Mod,
	"<<": text.Shl, ">>": text.Shr,
	"&": text.And, "|": text.Or, "^": text.Xor,
	"==": text.Eq, "!=": text.Ne,
	"<": text.Lt, "<=": text.Le, ">": text.Gt, ">=": text.Ge,
	"&&": text.LAnd, "||": text.LOr,
}

// precOf is the printer's view of the same table.
func precOf(o text.BinaryOp) int {
	for s, op := range binaryOp {
		if op == o {
			return binaryPrec[s]
		}
	}
	return precOr
}

func spellingOf(o text.BinaryOp) string {
	for s, op := range binaryOp {
		if op == o {
			return s
		}
	}
	return "?"
}

// expr parses a full expression.
func (p *parser) expr() text.Expr { return p.binary(precOr) }

// binary is precedence climbing. Every operator here is left-associative,
// which is GNU as's rule: operations with equal precedence are performed left
// to right.
func (p *parser) binary(min int) text.Expr {
	x := p.unary()
	for {
		if p.tok.kind != tPunct {
			return x
		}
		prec, ok := binaryPrec[p.tok.str]
		if !ok || prec < min {
			return x
		}
		op, pos := binaryOp[p.tok.str], p.tok.pos
		p.advance()
		y := p.binary(prec + 1)
		x = &text.Binary{P: pos, Op: op, X: x, Y: y}
	}
}

func (p *parser) unary() text.Expr {
	pos := p.tok.pos
	if p.tok.kind == tPunct {
		switch p.tok.str {
		case "-":
			p.advance()
			return &text.Unary{P: pos, Op: text.Neg, X: p.unary()}
		case "~":
			p.advance()
			return &text.Unary{P: pos, Op: text.Not, X: p.unary()}
		case "+":
			p.advance()
			return p.unary()
		}
	}
	return p.primary()
}

func (p *parser) primary() text.Expr {
	pos := p.tok.pos

	switch {
	case p.tok.kind == tNumber:
		v := p.tok.num
		p.advance()
		return &text.Int{P: pos, Value: v}

	case p.tok.is("("):
		p.advance()
		x := p.binary(precOr)
		if !p.tok.is(")") {
			p.errorf(p.tok.pos, "expected ')', got %s", p.tok)
			return x
		}
		p.advance()
		return x

	case p.tok.kind == tIdent:
		name := p.tok.str
		p.advance()

		// The location counter. GNU as spells it '.', which the lexer scans
		// as an identifier because a name may begin with one.
		if name == "." {
			return &text.Here{P: pos}
		}

		mod := text.ModNone
		if p.tok.is("@") {
			p.advance()
			if p.tok.kind != tIdent {
				p.errorf(p.tok.pos, "expected a relocation modifier after '@'")
				return &text.SymExpr{P: pos, Name: name}
			}
			m, ok := text.ParseModifier(p.tok.str)
			if !ok {
				p.errs.Add(text.UnknownModifier(p.tok.pos, p.tok.str))
			}
			mod = m
			p.advance()
		}
		return &text.SymExpr{P: pos, Name: name, Mod: mod}
	}

	p.errorf(pos, "expected an expression, got %s", p.tok)
	p.advance()
	return &text.Int{P: pos}
}

// absolute parses an expression that must fold to a number now — an
// alignment, a repeat count, a .p2align exponent. Anything symbolic here is a
// diagnostic rather than a deferred relocation, because these values decide
// how many bytes a statement occupies and the layout cannot wait for a link.
func (p *parser) absolute(what string) (int64, bool) {
	pos := p.tok.pos
	x := p.expr()
	eq := p.unit.Equates()
	v, err := text.Eval(x, func(name string) (int64, bool) {
		c, ok := eq[name]
		return c, ok
	})
	if err != nil {
		p.errs.Add(err)
		return 0, false
	}
	if !v.IsAbs() {
		p.errorf(pos, "%s must be absolute, got %s", what, v).
			Note("the value decides how many bytes this statement occupies")
		return 0, false
	}
	return v.Const, true
}