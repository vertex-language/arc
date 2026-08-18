package nasm

import (
	"strings"

	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

// ParseFile parses one NASM source file. Same contract as gas.ParseFile: a
// partial unit and every error the file produced, not just the first.
func ParseFile(name string, src []byte) (*text.Unit, error) {
	var errs text.ErrorList
	p := &parser{
		lex:  newLexer(name, src, &errs),
		errs: &errs,
		unit: &text.Unit{File: name},
	}
	p.advance()
	p.file()
	errs.Sort()
	return p.unit, errs.Err()
}

// ParseInst parses a single instruction, for arc enc and arc explain.
func ParseInst(s string) (*text.Inst, error) {
	var errs text.ErrorList
	p := &parser{
		lex:  newLexer("<argument>", []byte(s), &errs),
		errs: &errs,
		unit: &text.Unit{File: "<argument>"},
	}
	p.advance()
	p.statement(text.Trivia{})

	errs.Sort()
	if err := errs.Err(); err != nil {
		return nil, err
	}
	if len(p.unit.Nodes) != 1 {
		return nil, text.Errorf(text.Pos{File: "<argument>", Line: 1, Col: 1},
			"expected one instruction, got %d statements", len(p.unit.Nodes))
	}
	inst, ok := p.unit.Nodes[0].(*text.Inst)
	if !ok {
		return nil, text.Errorf(p.unit.Nodes[0].Pos(), "not an instruction")
	}
	return inst, nil
}

type parser struct {
	lex  *lexer
	tok  token
	errs *text.ErrorList
	unit *text.Unit

	pendingLabel *text.Label
}

func (p *parser) advance() { p.tok = p.lex.next() }

func (p *parser) errorf(pos text.Pos, format string, args ...any) *text.Error {
	e := text.Errorf(pos, format, args...)
	p.errs.Add(e)
	return e
}

func (p *parser) skipLine() {
	for p.tok.kind != tEOL && p.tok.kind != tEOF {
		p.advance()
	}
}

func (p *parser) file() {
	for p.tok.kind != tEOF {
		if p.tok.kind == tEOL {
			p.lex.countBlank()
			p.advance()
			continue
		}
		tv := p.lex.takeTrivia()
		p.statement(tv)
	}
	p.unit.Tail = p.lex.takeTrivia()
}

func (p *parser) statement(tv text.Trivia) {
	start := len(p.unit.Nodes)

	for {
		if p.tok.kind == tEOL || p.tok.kind == tEOF {
			break
		}
		if !p.one() {
			p.skipLine()
			break
		}
		if p.tok.kind != tEOL && p.tok.kind != tEOF {
			p.errorf(p.tok.pos, "unexpected %s after statement", p.tok)
			p.skipLine()
			break
		}
		break
	}

	if p.tok.kind == tEOL {
		p.advance()
	}
	if len(p.unit.Nodes) > start {
		*p.unit.Nodes[start].Trivia() = tv
		if c, ok := p.lex.takeComment(); ok {
			t := p.unit.Nodes[len(p.unit.Nodes)-1].Trivia()
			t.Line, t.HasLine = c, true
		}
	}
	p.pendingLabel = nil
}

// one parses a label, a directive, an EQU, or an instruction.
//
// The dispatch order matters: colon and EQU are checked first because they
// are unambiguous regardless of what the word spells; a known directive name
// beats a mnemonic lookup; and a word that is neither, alone on its line, is
// the orphan-label case the manual describes — accepted, the way NASM itself
// only warns rather than refuses.
func (p *parser) one() bool {
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a label, directive or instruction, got %s", p.tok)
		return false
	}

	word, pos := p.tok.str, p.tok.pos
	lw := strings.ToLower(word)

	if strings.HasPrefix(word, "%") {
		p.errorf(pos, "the preprocessor (%s) is not accepted", word).
			Note("Go is arc's macro language")
		return false
	}

	if p.peekIs(":") {
		p.advance()
		p.advance()
		return p.label(pos, word)
	}

	if p.peekIsFold("equ") {
		p.advance()
		p.advance()
		p.unit.Add(&text.Equ{Common: text.Common{P: pos}, Name: word, Value: p.expr()})
		return true
	}

	if directiveNames[lw] {
		p.advance()
		return p.directive(pos, word)
	}

	if _, ok := prefixes[lw]; ok {
		p.advance()
		return p.instruction(pos, word)
	}
	if len(isa.Forms(lw)) > 0 {
		p.advance()
		return p.instruction(pos, word)
	}

	p.advance()
	if p.tok.kind == tEOL || p.tok.kind == tEOF {
		return p.label(pos, word)
	}
	p.errs.Add(unknownMnemonic(pos, word))
	return false
}

func (p *parser) peekIs(s string) bool {
	save := *p.lex
	savedTok := p.tok
	p.advance()
	got := p.tok.is(s)
	*p.lex = save
	p.tok = savedTok
	return got
}

func (p *parser) peekIsFold(s string) bool {
	save := *p.lex
	savedTok := p.tok
	p.advance()
	got := p.tok.kind == tIdent && strings.EqualFold(p.tok.str, s)
	*p.lex = save
	p.tok = savedTok
	return got
}

// label handles both the colon and the bare-word form. A single leading '.'
// is local to the preceding non-local label; '..@' bypasses the mechanism
// entirely and is therefore, by this test, not local.
func (p *parser) label(pos text.Pos, name string) bool {
	l := &text.Label{
		Common: text.Common{P: pos},
		Name:   name,
		Local:  strings.HasPrefix(name, ".") && !strings.HasPrefix(name, ".."),
	}
	p.unit.Add(l)

	if p.tok.kind != tEOL && p.tok.kind != tEOF {
		l.Attached = true
		p.pendingLabel = l
		return p.one()
	}
	return true
}

var prefixes = map[string]text.Prefix{
	"lock":  text.PrefixLock,
	"rep":   text.PrefixRep,
	"repe":  text.PrefixRep,
	"repz":  text.PrefixRep,
	"repne": text.PrefixRepNE,
	"repnz": text.PrefixRepNE,
}

func unknownMnemonic(p text.Pos, word string) *text.Error {
	return text.Errorf(p, "unknown instruction %q", word).
		Note("arc isa lists every mnemonic this target encodes")
}

// sizeWidths are the keywords that force an operand's width — most often on
// a bracketed memory operand, occasionally on a bare immediate (`push dword
// 33`). The hint is threaded onto text.Inst.Size, the field gas fills from
// its mnemonic suffix; NASM has no suffix, so this is the only carrier.
var sizeWidths = map[string]text.Width{
	"byte": text.Width8, "word": text.Width16, "dword": text.Width32,
	"qword": text.Width64, "tword": text.Width80, "oword": text.Width128,
}

// sizeHints are consumed and dropped: NOSPLIT and STRICT are optimizer
// directives, and nothing in the shared tree records "do not optimize this
// operand's encoding."
var sizeHints = map[string]bool{"nosplit": true, "strict": true}

func (p *parser) instruction(pos text.Pos, word string) bool {
	lw := strings.ToLower(word)

	if pfx, ok := prefixes[lw]; ok {
		if p.tok.kind != tIdent {
			p.errorf(pos, "%s must prefix an instruction", word)
			return false
		}
		w2, p2 := p.tok.str, p.tok.pos
		p.advance()
		if !p.instruction(p2, w2) {
			return false
		}
		last := p.unit.Nodes[len(p.unit.Nodes)-1].(*text.Inst)
		last.Prefix = pfx
		last.P = pos
		return true
	}

	if len(isa.Forms(lw)) == 0 {
		p.errs.Add(unknownMnemonic(pos, word))
		return false
	}

	inst := &text.Inst{Common: text.Common{P: pos}, Mnemonic: lw}

	for p.tok.kind != tEOL && p.tok.kind != tEOF {
		o, hint := p.operand()
		if o == nil {
			return false
		}
		if hint != text.WidthNone && inst.Size == text.WidthNone {
			inst.Size = hint
		}
		inst.Ops = append(inst.Ops, o)
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}

	p.unit.Add(inst)
	return true
}

// operand parses one operand: a size hint (dropped or carried forward), then
// a bracketed memory reference, a bare register, or — everything else,
// including a branch target — an expression wrapped as text.Imm. There is no
// sigil disambiguating an immediate from a branch displacement in this
// syntax; both are the same shape here, exactly as they already are in
// gas's tree.
func (p *parser) operand() (text.Operand, text.Width) {
	pos := p.tok.pos
	var hint text.Width

	for p.tok.kind == tIdent {
		lw := strings.ToLower(p.tok.str)
		if sizeHints[lw] {
			p.advance()
			continue
		}
		if w, ok := sizeWidths[lw]; ok {
			hint = w
			p.advance()
		}
		break
	}

	if p.tok.is("[") {
		return p.bracketMemory(pos), hint
	}

	if p.tok.kind == tIdent && !p.tok.forced {
		if r, ok := text.LookupRegister(strings.ToLower(p.tok.str)); ok {
			p.advance()
			return text.Reg{P: pos, R: r}, hint
		}
	}

	return text.Imm{P: pos, X: p.expr()}, hint
}

// bracketMemory parses [ [seg:] [size/nosplit keywords] expr ] and hands the
// inner expression to decomposeAddress.
func (p *parser) bracketMemory(pos text.Pos) text.Operand {
	p.advance() // '['

	for p.tok.kind == tIdent {
		lw := strings.ToLower(p.tok.str)
		if sizeHints[lw] {
			p.advance()
			continue
		}
		if _, ok := sizeWidths[lw]; ok {
			p.advance()
			continue
		}
		break
	}

	var seg reg.Sreg
	hasSeg := false
	if p.tok.kind == tIdent && !p.tok.forced {
		if r, ok := text.LookupRegister(strings.ToLower(p.tok.str)); ok {
			if s, isSeg := r.(reg.Sreg); isSeg {
				save, savedTok := *p.lex, p.tok
				p.advance()
				if p.tok.is(":") {
					p.advance()
					seg, hasSeg = s, true
				} else {
					*p.lex, p.tok = save, savedTok
				}
			}
		}
	}

	x := p.expr()
	if !p.tok.is("]") {
		p.errorf(p.tok.pos, "expected ']' to close the address, got %s", p.tok)
		return nil
	}
	p.advance()

	m, ok := p.decomposeAddress(pos, x)
	if !ok {
		return nil
	}
	m.Seg, m.HasSeg = seg, hasSeg
	return m
}

// decomposeAddress pulls register terms out of a parsed expression, leaving
// everything else as the displacement. NASM writes [eax+ecx*4] as ordinary
// arithmetic; nothing in the syntax marks which term is the base and which
// is the scaled index, so this is the inverse of what the encoder's own
// modrm.go documents as the four special cases — done here once, at parse
// time, rather than deferred.
func (p *parser) decomposeAddress(pos text.Pos, e text.Expr) (text.Mem, bool) {
	m := text.Mem{P: pos}
	var dispTerms []text.Expr
	ok := true

	var walk func(x text.Expr, neg bool)
	walk = func(x text.Expr, neg bool) {
		switch v := x.(type) {
		case *text.Binary:
			switch v.Op {
			case text.Add:
				walk(v.X, neg)
				walk(v.Y, neg)
				return
			case text.Sub:
				walk(v.X, neg)
				walk(v.Y, !neg)
				return
			case text.Mul:
				if r, sc, is := regScale(v.X, v.Y); is {
					if !p.addIndexTerm(&m, pos, r, sc, neg) {
						ok = false
					}
					return
				}
				if r, sc, is := regScale(v.Y, v.X); is {
					if !p.addIndexTerm(&m, pos, r, sc, neg) {
						ok = false
					}
					return
				}
			}
		case *text.Unary:
			if v.Op == text.Neg {
				walk(v.X, !neg)
				return
			}
		case *text.SymExpr:
			if r, is := regFromSym(v.Name); is {
				if !p.addIndexTerm(&m, pos, r, 1, neg) {
					ok = false
				}
				return
			}
		}
		term := x
		if neg {
			term = &text.Unary{P: pos, Op: text.Neg, X: x}
		}
		dispTerms = append(dispTerms, term)
	}

	walk(e, false)
	if !ok {
		return text.Mem{}, false
	}

	if len(dispTerms) > 0 {
		d := dispTerms[0]
		for _, t := range dispTerms[1:] {
			d = &text.Binary{P: pos, Op: text.Add, X: d, Y: t}
		}
		m.Disp = d
	}
	return m, true
}

func regFromSym(name string) (reg.R32, bool) {
	r, ok := text.LookupRegister(strings.ToLower(name))
	if !ok {
		return 0, false
	}
	r32, ok := r.(reg.R32)
	return r32, ok
}

func regScale(a, b text.Expr) (reg.R32, uint8, bool) {
	sym, ok := a.(*text.SymExpr)
	if !ok {
		return 0, 0, false
	}
	r, ok := regFromSym(sym.Name)
	if !ok {
		return 0, 0, false
	}
	n, ok := b.(*text.Int)
	if !ok {
		return 0, 0, false
	}
	return r, uint8(n.Value), true
}

// addIndexTerm places a register term into the base or index slot. The first
// unscaled register claims base; a scaled term, or a second unscaled one,
// claims index; a third is an error, matching what operand/ can hold.
func (p *parser) addIndexTerm(m *text.Mem, pos text.Pos, r reg.R32, scale uint8, neg bool) bool {
	if neg {
		p.errorf(pos, "a register term in an effective address cannot be negated")
		return false
	}
	switch scale {
	case 1, 2, 4, 8:
	default:
		p.errorf(pos, "scale %d is not 1, 2, 4 or 8", scale)
		return false
	}

	if scale == 1 && !m.HasBase && !m.HasIndex {
		m.Base, m.HasBase = r, true
		return true
	}
	if !m.HasIndex {
		if r == reg.ESP {
			p.errorf(pos, "esp cannot be an index register").
				Note("SIB.index=100b encodes \"no index\"")
			return false
		}
		m.Index, m.Scale, m.HasIndex = r, scale, true
		return true
	}
	if !m.HasBase && scale == 1 {
		m.Base, m.HasBase = r, true
		return true
	}
	p.errorf(pos, "too many registers in an effective address").
		Note("an i386 address holds at most a base and a scaled index")
	return false
}