package gas

import (
	"strings"

	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

// ParseFile parses one .s file.
//
// Errors are collected rather than fatal: a file with two typos should take
// one run to fix. The unit returned alongside a non-nil error is partial and
// is meant for a caller that wants to report more than the first problem, not
// for assembly.
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
//
// It shares the statement parser with ParseFile rather than having one of its
// own, because two parsers for one grammar is two places for the grammar to
// drift.
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

	// pending is the label written on this line, when the next statement
	// shares its line. It is how `msg: .ascii "hi"` prints back as one line.
	pendingLabel *text.Label
}

func (p *parser) advance() { p.tok = p.lex.next() }

func (p *parser) errorf(pos text.Pos, format string, args ...any) *text.Error {
	e := text.Errorf(pos, format, args...)
	p.errs.Add(e)
	return e
}

// skipLine discards to the end of the statement after an error, so one bad
// line does not cascade.
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

// statement parses one statement and everything on its line.
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

	// The trailing comment belongs to the last statement on the line, and the
	// leading trivia to the first.
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

// one parses a label, a directive, an assignment or an instruction. It
// returns false when it has already reported an error.
func (p *parser) one() bool {
	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a label, directive or instruction, got %s", p.tok)
		return false
	}

	word, pos := p.tok.str, p.tok.pos

	// A label. The name may begin with '.', which is why this is decided by
	// the colon and not by the first character.
	if p.peekIs(":") {
		p.advance()
		p.advance()
		return p.label(pos, word)
	}

	// GNU as's assignment: name = expr, the same as .set.
	if p.peekIs("=") {
		p.advance()
		p.advance()
		x := p.expr()
		p.unit.Add(&text.Equ{Common: text.Common{P: pos}, Name: word, Value: x})
		return true
	}

	if strings.HasPrefix(word, ".") {
		p.advance()
		return p.directive(pos, word)
	}

	p.advance()
	return p.instruction(pos, word)
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

func (p *parser) label(pos text.Pos, name string) bool {
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		p.errorf(pos, "numeric local label %q is not accepted", name).
			Note("1: with 1f and 1b needs a resolution pass keyed on occurrence").
			Note("NASM has no counterpart, so the round trip could not hold").
			Note("use a named local label; .L names work in both dialects")
		return false
	}

	l := &text.Label{
		Common: text.Common{P: pos},
		Name:   name,
		Local:  strings.HasPrefix(name, ".L"),
	}
	p.unit.Add(l)

	// Something else on this line attaches to the label.
	if p.tok.kind != tEOL && p.tok.kind != tEOF {
		l.Attached = true
		p.pendingLabel = l
		return p.one()
	}
	return true
}

func (p *parser) instruction(pos text.Pos, word string) bool {
	mnemonic, size, ok := splitMnemonic(word)

	// A prefix is written as a mnemonic in both dialects and modifies the
	// instruction that follows it on the same line.
	if pfx, is := prefixes[mnemonic]; is && size == text.WidthNone {
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

	if !ok {
		p.errs.Add(unknownMnemonic(pos, word))
		return false
	}

	inst := &text.Inst{Common: text.Common{P: pos}, Mnemonic: mnemonic, Size: size}
	branch := isBranch(mnemonic)

	for p.tok.kind != tEOL && p.tok.kind != tEOF {
		o := p.operand(branch)
		if o == nil {
			return false
		}
		inst.Ops = append(inst.Ops, o)
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}

	inst.Ops = reverse(mnemonic, inst.Ops)
	p.unit.Add(inst)
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

// operand parses one operand.
//
// The four shapes are a register, a $-immediate, a *-indirect, and everything
// else — which is a memory operand, because in this dialect a bare expression
// names an address. The exception is a branch target, where a bare expression
// is the displacement itself; that is the whole reason branch is a parameter.
func (p *parser) operand(branch bool) text.Operand {
	pos := p.tok.pos

	switch {
	case p.tok.kind == tReg:
		r := p.tok.reg
		p.advance()

		// A segment register followed by ':' is an override on the memory
		// operand that follows, not an operand of its own.
		if s, isSeg := r.(reg.Sreg); isSeg && p.tok.is(":") {
			p.advance()
			m := p.memory(pos)
			if m == nil {
				return nil
			}
			mem := m.(text.Mem)
			mem.Seg, mem.HasSeg = s, true
			return mem
		}
		return text.Reg{P: pos, R: r}

	case p.tok.is("$"):
		p.advance()
		return text.Imm{P: pos, X: p.expr()}

	case p.tok.is("*"):
		p.advance()
		inner := p.operand(false)
		if inner == nil {
			return nil
		}
		return text.Indirect{P: pos, X: inner}
	}

	if branch {
		return text.Imm{P: pos, X: p.expr()}
	}
	return p.memory(pos)
}

// memory parses disp(base, index, scale), with any part absent.
func (p *parser) memory(pos text.Pos) text.Operand {
	m := text.Mem{P: pos}

	if !p.tok.is("(") {
		m.Disp = p.expr()
	}

	if !p.tok.is("(") {
		return m
	}
	p.advance()

	if p.tok.kind == tReg {
		r, ok := p.gpr(p.tok)
		if !ok {
			return nil
		}
		m.Base, m.HasBase = r, true
		p.advance()
	}

	if p.tok.is(",") {
		p.advance()
		if p.tok.kind == tReg {
			r, ok := p.gpr(p.tok)
			if !ok {
				return nil
			}
			if r == reg.ESP {
				p.errorf(p.tok.pos, "esp cannot be an index register").
					Note("SIB.index=100b encodes \"no index\"")
				return nil
			}
			m.Index, m.HasIndex, m.Scale = r, true, 1
			p.advance()
		}
		if p.tok.is(",") {
			p.advance()
			if p.tok.kind != tNumber {
				p.errorf(p.tok.pos, "expected a scale, got %s", p.tok)
				return nil
			}
			switch p.tok.num {
			case 1, 2, 4, 8:
				m.Scale = uint8(p.tok.num)
			default:
				p.errorf(p.tok.pos, "scale %d is not 1, 2, 4 or 8", p.tok.num)
				return nil
			}
			p.advance()
		}
	}

	if !p.tok.is(")") {
		p.errorf(p.tok.pos, "expected ')' to close the address, got %s", p.tok)
		return nil
	}
	p.advance()

	if !m.HasBase && !m.HasIndex && m.Disp == nil {
		p.errorf(pos, "empty address ()")
		return nil
	}
	return m
}

func (p *parser) gpr(t token) (reg.R32, bool) {
	r, ok := t.reg.(reg.R32)
	if !ok {
		p.errorf(t.pos, "%%%s cannot appear in an address", t.reg.Name()).
			Note("an i386 effective address is built from 32-bit general registers")
		return 0, false
	}
	return r, true
}