// x86_64/text/nasm/expr.go
package nasm

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/text"
)

// NASM's precedence is C's, which is the whole difference from gas and the
// reason text.Binary carries a Paren bit:
//
//	1  * / // % %%            highest
//	2  + -
//	3  << <<< >> >>>
//	4  == = != <> < > <= >=
//	5  &
//	6  ^
//	7  |
//	8  &&
//	9  ^^
//	10 ||                     lowest
//
// Bitwise below additive means `a|b+c` is `a|(b+c)` here and `(a|b)+c` in
// gas. That is not a difference this package can normalize away, and a tree
// parsed from one dialect prints back into the other with parentheses that
// were not in the source. The parentheses are a spelling change; the bytes
// are the same, which is the guarantee.
//
// The division operators are the other trap. NASM's '/' is unsigned and its
// '//' is signed; gas's '/' is signed and it has no unsigned spelling. Two
// operators in the neutral tree rather than one with a flag, because a flag
// would print back into the wrong dialect.
func precedence(op string) int {
	switch op {
	case "*", "/", "//", "%", "%%":
		return 1
	case "+", "-":
		return 2
	case "<<", "<<<", ">>", ">>>":
		return 3
	case "==", "=", "!=", "<>", "<", ">", "<=", ">=":
		return 4
	case "&":
		return 5
	case "^":
		return 6
	case "|":
		return 7
	case "&&":
		return 8
	case "^^":
		return 9
	case "||":
		return 10
	}
	return 0
}

const lowestPrec = 10

var binaryOps = map[string]text.Op{
	"*": text.OpMul, "/": text.OpUDiv, "//": text.OpDiv,
	"%": text.OpUMod, "%%": text.OpMod,
	"+": text.OpAdd, "-": text.OpSub,
	"<<": text.OpShl, "<<<": text.OpShl,
	">>": text.OpShr, ">>>": text.OpSar,
	"&": text.OpAnd, "^": text.OpXor, "|": text.OpOr,
	"==": text.OpEq, "=": text.OpEq, "!=": text.OpNe, "<>": text.OpNe,
	"<": text.OpLt, ">": text.OpGt, "<=": text.OpLe, ">=": text.OpGe,
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

		// ^^ is a logical exclusive-or and <=> is a three-way comparison.
		// Neither has a node in the neutral tree and neither should get
		// one: a node no other dialect can print is a node that makes the
		// round trip conditional. Refused by name rather than desugared,
		// because the obvious desugaring of ^^ produces -1 where NASM
		// produces 1.
		switch op {
		case "^^":
			return nil, text.Errorf(pos,
				"^^ has no dialect-neutral spelling; write (!!a) != (!!b)")
		case "<=>":
			return nil, text.Errorf(pos,
				"<=> has no dialect-neutral spelling")
		}

		if err := p.advance(); err != nil {
			return nil, err
		}

		// Equal precedence is left-associative, so the right operand is
		// parsed at one rank tighter.
		y, err := p.parseExpr(r - 1)
		if err != nil {
			return nil, err
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
		case "!":
			// NASM's '!' is unary logical negation and nothing else.
			// gas's is an infix bitwise or-not. Same character, different
			// arity, different operation — which is why neither parser can
			// be written from a memory of the other.
			if err := p.advance(); err != nil {
				return nil, err
			}
			x, err := p.parseUnary()
			return &text.Unary{Position: pos, Op: text.OpLNot, X: x}, err
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

	case tString:
		// A string in an expression is a number: NASM packs its bytes
		// little-endian into as many as fit. 'abcd' is 0x64636261 and is a
		// perfectly ordinary immediate.
		if len(p.tok.text) > 8 {
			return nil, text.Errorf(pos,
				"a string constant in an expression holds at most eight bytes")
		}
		n := &text.Num{Position: pos, Value: packString(p.tok.text), Base: 16}
		return n, p.advance()

	case tDollar:
		return &text.Dot{Position: pos}, p.advance()

	case tHere:
		return &text.Here{Position: pos}, p.advance()

	case tIdent:
		name := p.tok.text
		if _, ok := lookupRegister(name); ok {
			return nil, text.Errorf(pos,
				"%s is a register, which is a term only inside [ ]", name)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		s := &text.Sym{Position: pos, Name: name}

		// `wrt ..plt` is NASM's spelling of gas's `@PLT`. It folds to the
		// neutral modifier here, at the boundary, and the spelling does not
		// survive — which is what lets the same tree print either way.
		if p.tok.kind == tIdent && strings.EqualFold(p.tok.text, "wrt") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.tok.kind != tIdent {
				return nil, text.Errorf(p.tok.pos, "expected a special symbol after wrt")
			}
			m, err := parseWRT(p.tok.text)
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

func packString(s string) int64 {
	var v uint64
	for i := len(s) - 1; i >= 0; i-- {
		v = v<<8 | uint64(s[i])
	}
	return int64(v)
}

// The special symbols NASM accepts after `wrt`. They are folded to the
// neutral vocabulary; the R_X86_64_* constants they eventually become are
// declared at the root, which this package may not import.
var wrtNames = map[string]text.Modifier{
	"..plt":      text.ModPLT,
	"..got":      text.ModGOT,
	"..gotpcrel": text.ModGOTPCREL,
	"..gotoff":   text.ModGOTOFF,
	"..gottpoff": text.ModTPOFF,
	"..tlsie":    text.ModTPOFF,
	"..tpoff":    text.ModTPOFF,
	"..dtpoff":   text.ModDTPOFF,
	"..tlsld":    text.ModTLSLD,
	"..tlsgd":    text.ModTLSGD,
	"..sym":      text.ModSize,
}

func parseWRT(s string) (text.Modifier, error) {
	if m, ok := wrtNames[strings.ToLower(s)]; ok {
		return m, nil
	}
	return text.ModNone, text.Errorf(text.Pos{}, "unknown special symbol %s", s)
}

func wrtName(m text.Modifier) string {
	switch m {
	case text.ModPLT:
		return "..plt"
	case text.ModGOT:
		return "..got"
	case text.ModGOTPCREL:
		return "..gotpcrel"
	case text.ModGOTOFF:
		return "..gotoff"
	case text.ModTPOFF:
		return "..gottpoff"
	case text.ModDTPOFF:
		return "..dtpoff"
	case text.ModTLSGD:
		return "..tlsgd"
	case text.ModTLSLD:
		return "..tlsld"
	case text.ModSize:
		return "..sym"
	}
	return ""
}

// printExpr renders an expression in NASM syntax, parenthesizing wherever
// NASM's ranks would otherwise re-associate it.
func printExpr(e text.Expr) string { return printExprAt(e, lowestPrec) }

func printExprAt(e text.Expr, outer int) string {
	switch x := e.(type) {
	case nil:
		return ""
	case *text.Num:
		return printNum(x)
	case *text.Sym:
		s := x.Name
		if x.Backward || x.Forward {
			// gas's 1b and 1f resolve against the nearest numeric label in
			// a direction. NASM has no numeric labels and no spelling for
			// the question, so a unit carrying one cannot be printed as
			// NASM without inventing a name — and inventing one would
			// change what the file means.
			return "<" + s + ">"
		}
		if m := wrtName(x.Mod); m != "" {
			s += " wrt " + m
		}
		return s
	case *text.Dot:
		return "$"
	case *text.Here:
		return "$$"
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
		return "0o" + octDigits(uint64(n.Value))
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
	case text.OpDiv:
		return "//"
	case text.OpUDiv:
		return "/"
	case text.OpMod:
		return "%%"
	case text.OpUMod:
		return "%"
	case text.OpShl:
		return "<<"
	case text.OpShr:
		return ">>"
	case text.OpSar:
		return ">>>"
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
	case text.OpLNot:
		return "!"
	}
	return "?"
}