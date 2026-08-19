package gas

import "github.com/vertex-language/arc/aarch64/text"

// Expression parsing, at gas's precedence.
//
// gas has three tiers where C has ten, and they are not C's three. The
// multiplicative operators and both shifts bind tightest; the bitwise
// operators come next; addition and subtraction come last. So `a | b + c`
// parses as `(a | b) + c` here and as `a | (b + c)` in C, and a parser written
// from C habit produces the wrong tree for correct source without ever
// failing.
//
// Two further quirks are real and are the reason this table is written out
// rather than borrowed. `<` and `>` are accepted as single-character synonyms
// for `<<` and `>>`; there are no comparison operators to collide with them.
// And `!` is an infix operator meaning bitwise or-not, not a prefix negation —
// prefix has only `-` and `~`.

type prec uint8

const (
	precNone prec = iota
	precAdd       // + -
	precBit       // | & ^ !
	precMul       // * / % << >>
)

func precOf(op string) (text.Op, prec) {
	switch op {
	case "+":
		return text.OpAdd, precAdd
	case "-":
		return text.OpSub, precAdd

	case "|":
		return text.OpOr, precBit
	case "&":
		return text.OpAnd, precBit
	case "^":
		return text.OpXor, precBit
	case "!":
		return text.OpOrNot, precBit

	case "*":
		return text.OpMul, precMul
	case "/":
		return text.OpDiv, precMul
	case "%":
		return text.OpMod, precMul
	case "<<", "<":
		return text.OpShl, precMul
	case ">>", ">":
		return text.OpShr, precMul
	}
	return text.OpNone, precNone
}

// parseExpr reads a full expression.
func (p *Parser) parseExpr() text.Expr {
	return p.parseBinary(precAdd)
}

// parseBinary is precedence climbing. Every tier is left-associative, which is
// gas's rule for operators of equal precedence and the reason this loops rather
// than recursing on the right.
func (p *Parser) parseBinary(min prec) text.Expr {
	left := p.parseUnary()
	if left == nil {
		return nil
	}
	for {
		t := p.lex.Peek()
		if t.Kind != Punct {
			return left
		}
		op, pr := precOf(t.Text)
		if pr < min || pr == precNone {
			return left
		}
		p.lex.Next()
		right := p.parseBinary(pr + 1)
		if right == nil {
			p.errorf(t.Pos, "missing operand after %s", t.Text)
			return left
		}
		left = &text.Binary{Op: op, L: left, R: right, P: t.Pos}
	}
}

func (p *Parser) parseUnary() text.Expr {
	t := p.lex.Peek()
	if t.Kind == Punct {
		switch t.Text {
		case "-":
			p.lex.Next()
			return &text.Unary{Op: text.OpNeg, X: p.parseUnary(), P: t.Pos}
		case "~":
			p.lex.Next()
			return &text.Unary{Op: text.OpNot, X: p.parseUnary(), P: t.Pos}
		case "+":
			p.lex.Next()
			return p.parseUnary()
		}
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() text.Expr {
	t := p.lex.Peek()

	switch t.Kind {
	case Number:
		p.lex.Next()
		return &text.Num{Value: t.Num, P: t.Pos}

	case Local:
		p.lex.Next()
		return &text.Sym{Name: t.Text, Local: true, Forward: t.Forward, P: t.Pos}

	case Ident:
		p.lex.Next()
		if t.Text == "." {
			return &text.Dot{P: t.Pos}
		}
		return &text.Sym{Name: t.Text, P: t.Pos}

	case Punct:
		if t.Text == "(" {
			p.lex.Next()
			inner := p.parseExpr()
			if !p.expectPunct(")") {
				return inner
			}
			// The grouping is kept rather than discarded. Structure alone
			// would let `(a + b) * c` and `a + b * c` print identically once
			// precedence is reapplied — correct arithmetic, and a formatting
			// change nobody asked for.
			return &text.Paren{X: inner, P: t.Pos}
		}
	}

	p.errorf(t.Pos, "expected an expression, found %s", describe(t))
	return nil
}

// printExpr writes an expression back out, re-parenthesizing from structure.
//
// A Paren node prints its parentheses because it was written. Everything else
// gets parentheses only where the tree's shape demands them at gas's
// precedence, which is the property that makes printing the inverse of parsing
// rather than merely compatible with it.
func printExpr(b *builder, e text.Expr) {
	printExprPrec(b, e, precAdd)
}

func printExprPrec(b *builder, e text.Expr, min prec) {
	switch x := e.(type) {
	case *text.Num:
		b.num(x.Value)

	case *text.Sym:
		if x.Local {
			b.str(x.Name)
			if x.Forward {
				b.str("f")
			} else {
				b.str("b")
			}
			return
		}
		b.str(x.Name)

	case *text.Dot:
		b.str(".")

	case *text.Paren:
		b.str("(")
		printExprPrec(b, x.X, precAdd)
		b.str(")")

	case *text.Unary:
		b.str(x.Op.String())
		printExprPrec(b, x.X, precMul)

	case *text.Binary:
		_, pr := precOf(x.Op.String())
		wrap := pr < min
		if wrap {
			b.str("(")
		}
		printExprPrec(b, x.L, pr)
		b.str(" " + x.Op.String() + " ")
		// The right operand binds one tier tighter, which is what makes
		// `a - (b - c)` keep its parentheses and `a - b - c` lose them.
		printExprPrec(b, x.R, pr+1)
		if wrap {
			b.str(")")
		}
	}
}