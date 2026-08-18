// x86_64/text/gas/expr.go
package gas

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/text"
)

// gas has five precedence ranks. They are not C's, and the two places they
// differ are the two places a parser written from memory gets it wrong:
//
//   1  * / % << >>            highest
//   2  | & ^ !                bitwise, above additive
//   3  + - == <> != < > >= <= additive, sharing a rank with comparison
//   4  &&
//   5  ||                     lowest
//
// Bitwise above additive means `a|b+c` is `(a|b)+c` here and `a|(b+c)` in
// NASM. That is not a difference this package can normalize away: it is why
// text.Binary carries a Paren bit, and why a tree parsed from one dialect
// prints back into the other with parentheses that were not in the source.
//
// Comparisons sharing a rank with + and - means `a < b + c` is `(a < b) + c`,
// which surprises everyone and is what gas does.
func precedence(op string) int {
	switch op {
	case "*", "/", "%", "<<", ">>":
		return 1
	case "|", "&", "^", "!":
		return 2
	case "+", "-", "==", "<>", "!=", "<", ">", ">=", "<=":
		return 3
	case "&&":
		return 4
	case "||":
		return 5
	}
	return 0
}

const lowestPrec = 5

var binaryOps = map[string]text.Op{
	"*": text.OpMul, "/": text.OpDiv, "%": text.OpMod,
	"<<": text.OpShl, ">>": text.OpShr,
	"|": text.OpOr, "&": text.OpAnd, "^": text.OpXor,
	"+": text.OpAdd, "-": text.OpSub,
	"==": text.OpEq, "<>": text.OpNe, "!=": text.OpNe,
	"<": text.OpLt, ">": text.OpGt, ">=": text.OpGe, "<=": text.OpLe,
	"&&": text.OpLAnd, "||": text.OpLOr,
}

// parseExpr parses an expression at or below the given precedence rank.
func (p *parser) parseExpr(prec int) (text.Expr, error) {
	x, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		if p.tok.kind != tPunct {
			return x, nil
		}
		op := p.tok.text
		r := precedence(op)
		if r == 0 || r > prec {
			return x, nil
		}
		pos := p.tok.pos
		if err := p.advance(); err != nil {
			return nil, err
		}

		// Equal precedence is left-associative, so the right operand is
		// parsed at one rank tighter.
		y, err := p.parseExpr(r - 1)
		if err != nil {
			return nil, err
		}

		if op == "!" {
			// gas's infix '!' is bitwise or-not: a | ~b. There is no such
			// node in the neutral tree and there should not be — one
			// dialect's spelling of a compound operation is not an
			// architectural fact. It desugars, assembles identically, and
			// prints back as `a|~b`, which is the one place arc fmt changes
			// a gas source's spelling on purpose.
			x = &text.Binary{Position: pos, Op: text.OpOr, X: x,
				Y: &text.Unary{Position: pos, Op: text.OpNot, X: y}}
			continue
		}

		x = &text.Binary{Position: pos, Op: binaryOps[op], X: x, Y: y}
	}
}

func (p *parser) parseUnary() (text.Expr, error) {
	pos := p.tok.pos

	if p.tok.kind == tPunct {
		switch p.tok.text {
		case "-":
			if err := p.advance(); err != nil {
				return nil, err
			}
			x, err := p.parseUnary()
			return &text.Unary{Position: pos, Op: text.OpNeg, X: x}, err
		case "+":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return p.parseUnary()
		case "~":
			if err := p.advance(); err != nil {
				return nil, err
			}
			x, err := p.parseUnary()
			return &text.Unary{Position: pos, Op: text.OpNot, X: x}, err
		case "(":
			if err := p.advance(); err != nil {
				return nil, err
			}
			x, err := p.parseExpr(lowestPrec)
			if err != nil {
				return nil, err
			}
			if !p.isPunct(")") {
				return nil, text.Errorf(p.tok.pos, "expected ) in expression")
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			if b, ok := x.(*text.Binary); ok {
				b.Paren = true
			}
			return x, nil
		}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (text.Expr, error) {
	pos := p.tok.pos

	switch p.tok.kind {
	case tNum:
		n := &text.Num{Position: pos, Value: p.tok.num, Base: p.tok.base}
		return n, p.advance()

	case tChar:
		n := &text.Num{Position: pos, Value: p.tok.num, Base: 10}
		return n, p.advance()

	case tIdent:
		name := p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}

		// '.' alone is the location counter; '.foo' is a symbol.
		if name == "." {
			return &text.Dot{Position: pos}, nil
		}

		s := &text.Sym{Position: pos, Name: name}
		if n := len(name); n == 2 && isDigit(name[0]) {
			switch name[1] {
			case 'b':
				s.Name, s.Backward = name[:1], true
			case 'f':
				s.Name, s.Forward = name[:1], true
			}
		}

		if p.tok.kind == tAt {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.tok.kind != tIdent {
				return nil, text.Errorf(p.tok.pos, "expected a modifier after @")
			}
			m, err := parseModifier(p.tok.text)
			if err != nil {
				return nil, text.Wrap(p.tok.pos, err)
			}
			s.Mod = m
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		return s, nil
	}
	return nil, text.Errorf(pos, "expected an expression, got %q", p.tok.text)
}

// The @-modifiers gas accepts on a symbol in an operand. They are folded to
// the neutral vocabulary here, at the boundary, and the spelling does not
// survive — which is what lets the same tree print as NASM's `wrt ..plt`.
var modifiers = map[string]text.Modifier{
	"plt":      text.ModPLT,
	"got":      text.ModGOT,
	"gotpcrel": text.ModGOTPCREL,
	"gotoff":   text.ModGOTOFF,
	"tpoff":    text.ModTPOFF,
	"dtpoff":   text.ModDTPOFF,
	"tlsgd":    text.ModTLSGD,
	"tlsld":    text.ModTLSLD,
	"size":     text.ModSize,
}

func parseModifier(s string) (text.Modifier, error) {
	if m, ok := modifiers[strings.ToLower(s)]; ok {
		return m, nil
	}
	return text.ModNone, text.Errorf(text.Pos{}, "unknown modifier @%s", s)
}

func modifierName(m text.Modifier) string {
	switch m {
	case text.ModPLT:
		return "PLT"
	case text.ModGOT:
		return "GOT"
	case text.ModGOTPCREL:
		return "GOTPCREL"
	case text.ModGOTOFF:
		return "GOTOFF"
	case text.ModTPOFF:
		return "TPOFF"
	case text.ModDTPOFF:
		return "DTPOFF"
	case text.ModTLSGD:
		return "TLSGD"
	case text.ModTLSLD:
		return "TLSLD"
	case text.ModSize:
		return "SIZE"
	}
	return ""
}

// printExpr renders an expression in gas syntax, parenthesizing wherever
// gas's ranks would otherwise re-associate it.
func printExpr(e text.Expr) string {
	return printExprAt(e, lowestPrec)
}

func printExprAt(e text.Expr, outer int) string {
	switch x := e.(type) {
	case nil:
		return ""
	case *text.Num:
		return printNum(x)
	case *text.Sym:
		s := x.Name
		if x.Backward {
			s += "b"
		}
		if x.Forward {
			s += "f"
		}
		if m := modifierName(x.Mod); m != "" {
			s += "@" + m
		}
		return s
	case *text.Dot:
		return "."
	case *text.Here:
		// NASM's $$ is the start of the current section. gas has no
		// spelling for it, so a unit carrying one cannot be printed as gas
		// without inventing a symbol — and inventing one would change what
		// the file means.
		return "<$$>"
	case *text.Unary:
		return unaryOpName(x.Op) + printExprAt(x.X, 0)
	case *text.Binary:
		op := opName(x.Op)
		r := precedence(op)
		s := printExprAt(x.X, r) + op + printExprAt(x.Y, r-1)
		if r > outer || x.Paren {
			return "(" + s + ")"
		}
		return s
	}
	return "?"
}

func printNum(n *text.Num) string {
	switch n.Base {
	case 16:
		return "0x" + strings.ToLower(hexDigits(uint64(n.Value)))
	case 8:
		return "0" + octDigits(uint64(n.Value))
	case 2:
		return "0b" + binDigits(uint64(n.Value))
	}
	return dec(n.Value)
}

func opName(o text.Op) string {
	switch o {
	case text.OpAdd:
		return "+"
	case text.OpSub:
		return "-"
	case text.OpMul:
		return "*"
	case text.OpDiv, text.OpUDiv:
		// gas divides signed. A tree carrying NASM's unsigned division
		// prints as '/' here and means something different for exactly one
		// operand in a million — which is why it is refused rather than
		// printed.
		return "/"
	case text.OpMod, text.OpUMod:
		return "%"
	case text.OpShl:
		return "<<"
	case text.OpShr, text.OpSar:
		return ">>"
	case text.OpAnd:
		return "&"
	case text.OpOr:
		return "|"
	case text.OpXor:
		return "^"
	case text.OpEq:
		return "=="
	case text.OpNe:
		return "!="
	case text.OpLt:
		return "<"
	case text.OpLe:
		return "<="
	case text.OpGt:
		return ">"
	case text.OpGe:
		return ">="
	case text.OpLAnd:
		return "&&"
	case text.OpLOr:
		return "||"
	}
	return "?"
}

func unaryOpName(o text.Op) string {
	switch o {
	case text.OpNeg:
		return "-"
	case text.OpPos:
		return "+"
	case text.OpNot:
		return "~"
	}
	return "?"
}