package nasm

import (
	"strings"

	"github.com/vertex-language/arc/i386/text"
)

// NASM's expression grammar: the same tree as gas's, bound the other way.
//
// NASM's precedence, tightest first, is the C-like order with '|' at the
// bottom:
//
//	* / // % %%
//	+ -
//	<< >> <<< >>>
//	&
//	^
//	|
//	= == != <> < <= > >=
//	&&
//	||
//
// (?: sits below ||, and ^^ between && and the comparisons; neither is
// implemented — see the package doc.) This is the inverse nesting of gas's
// table for bitwise-vs-additive, which is why `1|2+3` is 5 here and 6 there,
// and why the table lives in this package rather than in text/.
//
// Several NASM operators collapse onto one text.BinaryOp because the shared
// tree does not distinguish what NASM does: '/' (unsigned) and '//' (signed)
// both become text.Div, whose Eval performs Go's signed division; '%' and
// '%%' both become text.Mod for the same reason; '>>' (logical) and '>>>'
// (arithmetic) both become text.Shr, whose Eval is Go's arithmetic shift on a
// signed int64. Right-shifting a value NASM would treat as unsigned can
// therefore differ from NASM's own answer for operands whose sign bit is
// set — a real gap, and it is here rather than silently rounded away.

const (
	precLOr = iota + 1 // ||
	precLAnd
	precCompare // = == != <> < <= > >=
	precBitOr   // |
	precBitXor  // ^
	precBitAnd  // &
	precShift   // << >> <<< >>>
	precAdd     // + -
	precMul     // * / // % %%
	precUnary
)

var binaryPrec = map[string]int{
	"||": precLOr,
	"&&": precLAnd,
	"=": precCompare, "==": precCompare, "!=": precCompare, "<>": precCompare,
	"<": precCompare, "<=": precCompare, ">": precCompare, ">=": precCompare,
	"|": precBitOr,
	"^": precBitXor,
	"&": precBitAnd,
	"<<": precShift, "<<<": precShift, ">>": precShift, ">>>": precShift,
	"+": precAdd, "-": precAdd,
	"*": precMul, "/": precMul, "//": precMul, "%": precMul, "%%": precMul,
}

var binaryOp = map[string]text.BinaryOp{
	"||": text.LOr,
	"&&": text.LAnd,
	"=": text.Eq, "==": text.Eq, "!=": text.Ne, "<>": text.Ne,
	"<": text.Lt, "<=": text.Le, ">": text.Gt, ">=": text.Ge,
	"|": text.Or,
	"^": text.Xor,
	"&": text.And,
	"<<": text.Shl, "<<<": text.Shl, ">>": text.Shr, ">>>": text.Shr,
	"+": text.Add, "-": text.Sub,
	"*": text.Mul, "/": text.Div, "//": text.Div, "%": text.Mod, "%%": text.Mod,
}

// opPrec and opSpelling are the printer's view. They are separate, explicit
// tables rather than a reverse lookup over binaryOp, because several
// spellings collapse onto one operator there and a reverse map iteration
// would pick a spelling nondeterministically.
var opPrec = map[text.BinaryOp]int{
	text.LOr: precLOr, text.LAnd: precLAnd,
	text.Eq: precCompare, text.Ne: precCompare, text.Lt: precCompare,
	text.Le: precCompare, text.Gt: precCompare, text.Ge: precCompare,
	text.Or: precBitOr, text.Xor: precBitXor, text.And: precBitAnd,
	text.Shl: precShift, text.Shr: precShift,
	text.Add: precAdd, text.Sub: precAdd,
	text.Mul: precMul, text.Div: precMul, text.Mod: precMul,
}

var opSpelling = map[text.BinaryOp]string{
	text.LOr: "||", text.LAnd: "&&",
	text.Eq: "==", text.Ne: "!=", text.Lt: "<", text.Le: "<=", text.Gt: ">", text.Ge: ">=",
	text.Or: "|", text.Xor: "^", text.And: "&",
	text.Shl: "<<", text.Shr: ">>",
	text.Add: "+", text.Sub: "-",
	text.Mul: "*", text.Div: "/", text.Mod: "%",
}

func precOf(o text.BinaryOp) int {
	if p, ok := opPrec[o]; ok {
		return p
	}
	return precLOr
}

func spellingOf(o text.BinaryOp) string {
	if s, ok := opSpelling[o]; ok {
		return s
	}
	return "?"
}

// nasmWrtModifier maps a WRT keyword to the relocation it names. The
// spellings are NASM's own ("..plt" not "PLT"); only the four gas already
// supports have a WRT counterpart here.
var nasmWrtModifier = map[string]text.Modifier{
	"..plt":    text.ModPLT,
	"..got":    text.ModGOT,
	"..gotoff": text.ModGOTOFF,
	"..gotpc":  text.ModGOTPC,
}

func (p *parser) expr() text.Expr { return p.binary(precLOr) }

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
		case "!":
			p.advance()
			return &text.Unary{P: pos, Op: text.LNot, X: p.unary()}
		case "+":
			p.advance()
			return p.unary()
		}
	}
	if p.tok.kind == tIdent && !p.tok.forced && strings.EqualFold(p.tok.str, "seg") {
		p.advance()
		p.errorf(pos, "SEG is not accepted").
			Note("arc's i386 target is flat protected mode; there is no segment base to take")
		return p.unary()
	}
	return p.postfix()
}

// postfix applies WRT, which in NASM's grammar reads as a binary keyword
// operator rather than a suffix: `symbol wrt ..plt`.
func (p *parser) postfix() text.Expr {
	x := p.primary()
	for p.tok.kind == tIdent && !p.tok.forced && strings.EqualFold(p.tok.str, "wrt") {
		p.advance()
		if p.tok.kind != tIdent {
			p.errorf(p.tok.pos, "expected a WRT keyword, got %s", p.tok)
			break
		}
		mod, ok := nasmWrtModifier[strings.ToLower(p.tok.str)]
		if !ok {
			p.errs.Add(text.UnknownModifier(p.tok.pos, p.tok.str))
			p.advance()
			continue
		}
		p.advance()
		sym, isSym := x.(*text.SymExpr)
		if !isSym {
			p.errorf(x.Pos(), "WRT applies to a symbol")
			continue
		}
		sym.Mod = mod
	}
	return x
}

func (p *parser) primary() text.Expr {
	pos := p.tok.pos

	switch {
	case p.tok.kind == tNumber:
		v := p.tok.num
		p.advance()
		return &text.Int{P: pos, Value: v}

	case p.tok.kind == tString:
		s := p.tok.str
		p.advance()
		if len(s) > 8 {
			p.errorf(pos, "a string used as a value must be 8 bytes or fewer, got %d", len(s))
			return &text.Int{P: pos}
		}
		var v int64
		for i := 0; i < len(s); i++ {
			v |= int64(s[i]) << uint(8*i)
		}
		return &text.Int{P: pos, Value: v}

	case p.tok.is("$$"):
		p.advance()
		return &text.Start{P: pos}

	case p.tok.is("$"):
		p.advance()
		return &text.Here{P: pos}

	case p.tok.is("("):
		p.advance()
		x := p.binary(precLOr)
		if !p.tok.is(")") {
			p.errorf(p.tok.pos, "expected ')', got %s", p.tok)
			return x
		}
		p.advance()
		return x

	case p.tok.kind == tIdent:
		name := p.tok.str
		p.advance()
		return &text.SymExpr{P: pos, Name: name}
	}

	p.errorf(pos, "expected an expression, got %s", p.tok)
	p.advance()
	return &text.Int{P: pos}
}