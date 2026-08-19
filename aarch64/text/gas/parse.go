package gas

import (
	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
	"github.com/vertex-language/arc/aarch64/text"
)

// Parser reads A64 assembly into a text.Unit.
type Parser struct {
	lex     *Parser0
	aliases *aliases
	errs    ErrorList
}

// Parser0 is the lexer, named so the field can be `lex` without shadowing.
type Parser0 = Lexer

// Parse reads a source file.
//
// Errors are collected rather than returned at the first failure: a file with
// two typos should report two. A statement that fails to parse is skipped to
// the next separator and the unit continues, so one bad line does not cascade.
func Parse(file, src string) (*text.Unit, error) {
	p := &Parser{lex: NewLexer(file, src), aliases: newAliases()}
	u := &text.Unit{File: file}

	for {
		t := p.lex.Peek()
		if t.Kind == EOF {
			break
		}
		if n := p.parseStatement(); n != nil {
			u.Nodes = append(u.Nodes, n)
		}
		if len(p.errs) >= maxErrors {
			break
		}
	}

	for _, e := range p.lex.Errors() {
		if pe, ok := e.(*Error); ok {
			p.errs = append(p.errs, pe)
		}
	}
	return u, p.errs.Err()
}

// parseStatement reads one statement.
//
// The dispatch is on the first token and on what follows it, because A64
// assembly's statement forms are not distinguished by their first token alone:
// an identifier begins a label, an instruction, or a .req — and which of the
// three is decided by the token after it.
func (p *Parser) parseStatement() text.Node {
	t := p.lex.Peek()

	switch t.Kind {
	case EOL:
		p.lex.Next()
		return &text.Comment{Blank: true, P: t.Pos}

	case Comment:
		p.lex.Next()
		p.acceptEOL()
		return &text.Comment{Text: t.Text, P: t.Pos}

	case Number:
		// A numeric label: 1:, 2:. They are position references rather than
		// symbols, and the same digit may appear many times in one file.
		p.lex.Next()
		if !p.expectPunct(":") {
			return nil
		}
		return &text.Label{Name: t.Text, Numeric: true, P: t.Pos,
			Comment: p.finishStatement()}

	case Ident:
		p.lex.Next()
		next := p.lex.Peek()

		if next.Kind == Punct && next.Text == ":" {
			p.lex.Next()
			return &text.Label{Name: t.Text, P: t.Pos, Comment: p.finishStatement()}
		}

		// `foo .req w0` puts the alias name where a label goes, which is the
		// one statement form in this grammar that begins with a name meaning
		// neither a label nor a mnemonic.
		if next.Kind == Ident && lowerEq(next.Text, ".req") {
			return p.parseReq(t)
		}

		if t.Text[0] == '.' {
			if _, known := lookupDirective(t.Text); known {
				return p.parseDirective(t)
			}
		}
		return p.parseInst(t)
	}

	p.errorf(t.Pos, "expected a label, instruction or directive, found %s", describe(t))
	p.skipStatement()
	return nil
}

// parseReq reads `name .req register` and records the alias.
//
// The alias takes effect immediately, mid-file, which is why the table is
// parser state: a register name written on the next line resolves through it,
// and a parser that collected .req into a pass would have to know all of them
// before reading any of them.
func (p *Parser) parseReq(name Token) text.Node {
	d := &text.Directive{Kind: text.DirReq, Spelling: ".req", Name: name.Text, P: name.Pos}
	p.lex.Next() // .req

	t := p.lex.Peek()
	if t.Kind != Ident {
		p.errorf(t.Pos, ".req needs a register name")
		p.skipStatement()
		return nil
	}
	p.lex.Next()

	r, ok := p.aliases.lookup(t.Text)
	if !ok {
		p.errorf(t.Pos, "%s is not a register", t.Text)
		p.skipStatement()
		return nil
	}
	p.aliases.define(name.Text, r)
	d.Flags = []string{t.Text}
	d.Comment = p.finishStatement()
	return d
}

// parseInst reads an instruction.
func (p *Parser) parseInst(tok Token) text.Node {
	mnem, cond, hasCond := splitMnemonic(tok.Text)
	in := &text.Inst{Mnem: mnem, P: tok.Pos}

	if hasCond {
		// The table spells this form's operands condition-first, so the
		// condition carried in the mnemonic becomes operand zero.
		in.Ops = append(in.Ops, &text.Operand{
			Kind: text.OpCond, Cond: cond, P: tok.Pos, Text: tok.Text,
		})
	}

	for {
		t := p.lex.Peek()
		if t.Kind == EOL || t.Kind == EOF || t.Kind == Comment {
			break
		}
		o := p.parseOperand()
		if o == nil {
			p.skipStatement()
			return nil
		}
		in.Ops = append(in.Ops, o)
		if !p.acceptPunct(",") {
			break
		}
	}

	in.Comment = p.finishStatement()
	return in
}

// parseOperand reads one operand.
func (p *Parser) parseOperand() *text.Operand {
	t := p.lex.Peek()
	o := &text.Operand{P: t.Pos}

	switch {
	case t.Kind == Punct && t.Text == "[":
		return p.parseMem()

	case t.Kind == Punct && t.Text == "{":
		// A register list: { v0.16b, v1.16b }. The table declares no form
		// taking one yet, so this is recognized and refused rather than
		// parsed into a shape nothing consumes.
		p.errorf(t.Pos, "register lists are not encodable yet: no form in the table takes one")
		return nil

	case t.Kind == Punct && (t.Text == "#" || t.Text == ":"):
		return p.parseImmOrTarget()

	case t.Kind == Ident:
		return p.parseIdentOperand()

	case t.Kind == Number || t.Kind == Local:
		// A bare number in operand position is an immediate; gas makes the '#'
		// optional and code in the wild omits it.
		o.Kind = text.OpImm
		o.Expr = p.parseExpr()
		if o.Expr == nil {
			return nil
		}
		if t.Kind == Local {
			o.Kind = text.OpTarget
		}
		return o
	}

	p.errorf(t.Pos, "expected an operand, found %s", describe(t))
	return nil
}

// parseImmOrTarget reads a '#'-prefixed immediate or a modified address.
//
// The two share a prefix because gas writes `#:lo12:foo` — a hash, then a
// modifier, then a symbol — and the hash says only "an immediate follows",
// which a modified address technically is.
func (p *Parser) parseImmOrTarget() *text.Operand {
	t := p.lex.Peek()
	o := &text.Operand{P: t.Pos}

	if t.Text == "#" {
		p.lex.Next()
	}

	if mod, ok := p.parseModifier(); ok {
		o.Kind = text.OpTarget
		o.Mod = mod
		o.Expr = p.parseExpr()
		if o.Expr == nil {
			return nil
		}
		return o
	}

	o.Kind = text.OpImm
	o.Expr = p.parseExpr()
	if o.Expr == nil {
		return nil
	}
	return o
}

// parseModifier reads `:name:` if one is present.
func (p *Parser) parseModifier() (text.Modifier, bool) {
	t := p.lex.Peek()
	if t.Kind != Punct || t.Text != ":" {
		return text.ModNone, false
	}
	p.lex.Next()

	name := p.lex.Peek()
	if name.Kind != Ident {
		p.errorf(name.Pos, "expected a relocation modifier name after ':'")
		return text.ModNone, false
	}
	p.lex.Next()
	p.expectPunct(":")

	m, ok, note := modifier(name.Text)
	if !ok {
		if note != "" {
			p.noteErrorf(name.Pos, note, "the %s modifier has no wired relocation", name.Text)
		} else {
			p.errorf(name.Pos, "unknown relocation modifier :%s:", name.Text)
		}
		return text.ModNone, false
	}
	return m, true
}

// parseIdentOperand reads an operand that begins with a name: a register, a
// shift, an extend, a condition, a barrier option, or a symbol.
//
// The order of these tests is the vocabulary's, not a preference. A register
// name is checked first because the alias table can make an arbitrary
// identifier one; the small enumerations follow; a name that is none of them is
// a symbol, which is the only open-ended case.
func (p *Parser) parseIdentOperand() *text.Operand {
	t := p.lex.Next()
	o := &text.Operand{P: t.Pos, Text: t.Text}

	base, suffix := splitRegSuffix(t.Text)

	if r, ok := p.aliases.lookup(base); ok {
		o.Kind = text.OpReg
		o.Reg = r
		if suffix != "" {
			if !p.applyVectorSuffix(o, r, suffix, t.Pos) {
				return nil
			}
		}
		return o
	}

	if s, ok := shiftOp(t.Text); ok {
		o.Kind = text.OpShift
		o.Shift = s
		o.Amount = p.parseOptionalAmount()
		return o
	}

	if e, ok := extendOp(t.Text); ok {
		o.Kind = text.OpExtend
		o.Extend = e
		o.Amount = p.parseOptionalAmount()
		return o
	}

	if c, ok := operand.LookupCond(t.Text); ok {
		o.Kind = text.OpCond
		o.Cond = c
		return o
	}

	if b, ok := operand.LookupBarrier(t.Text); ok {
		o.Kind = text.OpBarrier
		o.Barrier = b
		return o
	}

	if pf, ok := operand.LookupPrfOp(t.Text); ok {
		o.Kind = text.OpPrfOp
		o.Prf = pf
		return o
	}

	// A symbol, possibly with an addend: `puts`, `msg + 8`.
	o.Kind = text.OpTarget
	o.Expr = p.continueExpr(&text.Sym{Name: t.Text, P: t.Pos})
	return o
}

// applyVectorSuffix reads the arrangement or lane after a vector register's
// dot: v0.4s, or v2.s[1] whose bracket is still ahead.
func (p *Parser) applyVectorSuffix(o *text.Operand, r reg.Reg, suffix string, pos text.Pos) bool {
	v, isV := r.(reg.V)
	if !isV {
		p.errorf(pos, "%s is not a vector register and takes no arrangement", r)
		return false
	}
	_ = v

	if a, ok := arrangement(suffix); ok {
		o.Arr = a
		return true
	}

	e, ok := element(suffix)
	if !ok {
		p.errorf(pos, "%s is not an arrangement or element width", suffix)
		return false
	}
	if !p.expectPunct("[") {
		return false
	}
	idx := p.lex.Peek()
	if idx.Kind != Number {
		p.errorf(idx.Pos, "expected a lane index")
		return false
	}
	p.lex.Next()
	if !p.expectPunct("]") {
		return false
	}
	o.Elem = e
	o.Lane = int(idx.Num)
	o.HasLane = true
	return true
}

// parseMem reads an address: everything between the brackets, plus a trailing
// `!` or `, #imm` that makes it a writeback form.
func (p *Parser) parseMem() *text.Operand {
	open := p.lex.Next() // [
	o := &text.Operand{Kind: text.OpMem, P: open.Pos}

	baseTok := p.lex.Peek()
	if baseTok.Kind != Ident {
		p.errorf(baseTok.Pos, "expected a base register")
		return nil
	}
	p.lex.Next()
	r, ok := p.aliases.lookup(baseTok.Text)
	if !ok {
		p.errorf(baseTok.Pos, "%s is not a register", baseTok.Text)
		return nil
	}
	o.Reg = r
	o.Mem.Form = operand.AddrBase

	if p.acceptPunct("]") {
		// [x1], #16 — post-indexed, the offset outside the brackets.
		if p.acceptPunct(",") {
			o.Mem.Form = operand.AddrPostIndex
			o.Mem.Disp = p.parseHashExpr()
			if o.Mem.Disp == nil {
				return nil
			}
		}
		return o
	}

	if !p.expectPunct(",") {
		return nil
	}

	next := p.lex.Peek()
	switch {
	case next.Kind == Ident && p.isRegisterName(next.Text):
		p.lex.Next()
		idx, _ := p.aliases.lookup(next.Text)
		o.Mem.Form = operand.AddrRegOffset
		o.Mem.Index = idx
		o.Mem.Ext = operand.ExtLSL
		if p.acceptPunct(",") {
			extTok := p.lex.Peek()
			if extTok.Kind != Ident {
				p.errorf(extTok.Pos, "expected lsl, uxtw, sxtw or sxtx")
				return nil
			}
			p.lex.Next()
			if s, isShift := shiftOp(extTok.Text); isShift {
				if s != operand.LSL {
					p.errorf(extTok.Pos, "only lsl decorates an index register")
					return nil
				}
				o.Mem.Ext = operand.ExtLSL
			} else if e, isExt := extendOp(extTok.Text); isExt {
				o.Mem.Ext = e
			} else {
				p.errorf(extTok.Pos, "%s is not an index modifier", extTok.Text)
				return nil
			}
			o.Mem.Amount = p.parseOptionalAmount()
		}

	default:
		o.Mem.Form = operand.AddrOffset
		if p.acceptPunct("#") {
			// nothing: the hash is optional and carries no meaning
		}
		if mod, hasMod := p.parseModifier(); hasMod {
			o.Mem.Mod = mod
		}
		o.Mem.Disp = p.parseExpr()
		if o.Mem.Disp == nil {
			return nil
		}
	}

	if !p.expectPunct("]") {
		return nil
	}

	// [x1, #-32]! — pre-indexed.
	if p.acceptPunct("!") {
		if o.Mem.Form != operand.AddrOffset {
			p.errorf(open.Pos, "writeback applies to an immediate offset only")
			return nil
		}
		o.Mem.Form = operand.AddrPreIndex
	}
	return o
}

// parseHashExpr reads an expression whose optional '#' has not been consumed.
func (p *Parser) parseHashExpr() text.Expr {
	p.acceptPunct("#")
	return p.parseExpr()
}

// parseOptionalAmount reads the `#3` of `lsl #3`, which the syntax may omit.
func (p *Parser) parseOptionalAmount() text.Expr {
	t := p.lex.Peek()
	if t.Kind == Punct && t.Text == "#" {
		p.lex.Next()
		return p.parseExpr()
	}
	if t.Kind == Number {
		return p.parseExpr()
	}
	return nil
}

// continueExpr resumes expression parsing with a first term already read,
// which is how `puts + 8` is recovered after the name turned out not to be a
// register.
func (p *Parser) continueExpr(first text.Expr) text.Expr {
	left := first
	for {
		t := p.lex.Peek()
		if t.Kind != Punct {
			return left
		}
		op, pr := precOf(t.Text)
		if pr == precNone {
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

func (p *Parser) isRegisterName(s string) bool {
	base, _ := splitRegSuffix(s)
	_, ok := p.aliases.lookup(base)
	return ok
}

// finishStatement consumes a trailing comment and the statement separator,
// returning the comment so it can ride on the node rather than float between
// nodes where a pass could orphan it.
func (p *Parser) finishStatement() string {
	comment := ""
	if t := p.lex.Peek(); t.Kind == Comment {
		p.lex.Next()
		comment = t.Text
	}
	t := p.lex.Peek()
	switch t.Kind {
	case EOL:
		p.lex.Next()
	case EOF:
	default:
		p.errorf(t.Pos, "unexpected %s after statement", describe(t))
		p.skipStatement()
	}
	return comment
}

func (p *Parser) skipStatement() {
	for {
		t := p.lex.Peek()
		if t.Kind == EOF {
			return
		}
		p.lex.Next()
		if t.Kind == EOL {
			return
		}
	}
}

func (p *Parser) acceptEOL() {
	if p.lex.Peek().Kind == EOL {
		p.lex.Next()
	}
}

func (p *Parser) acceptPunct(s string) bool {
	t := p.lex.Peek()
	if t.Kind == Punct && t.Text == s {
		p.lex.Next()
		return true
	}
	return false
}

func (p *Parser) expectPunct(s string) bool {
	if p.acceptPunct(s) {
		return true
	}
	t := p.lex.Peek()
	p.errorf(t.Pos, "expected %q, found %s", s, describe(t))
	return false
}

func (p *Parser) parseName() string {
	t := p.lex.Peek()
	if t.Kind != Ident && t.Kind != String {
		p.errorf(t.Pos, "expected a name, found %s", describe(t))
		return ""
	}
	p.lex.Next()
	return t.Text
}

func (p *Parser) errorf(pos text.Pos, format string, args ...any) {
	p.errs = append(p.errs, &Error{Pos: pos, Msg: sprintf(format, args...)})
}

func (p *Parser) noteErrorf(pos text.Pos, note, format string, args ...any) {
	p.errs = append(p.errs, &Error{Pos: pos, Msg: sprintf(format, args...), Note: note})
}

func describe(t Token) string {
	switch t.Kind {
	case EOF:
		return "end of file"
	case EOL:
		return "end of statement"
	}
	if t.Text == "" {
		return t.Kind.String()
	}
	return t.Kind.String() + " " + t.Text
}

func lowerEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 32
		}
		if y >= 'A' && y <= 'Z' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}