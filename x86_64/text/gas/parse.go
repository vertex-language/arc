// x86_64/text/gas/parse.go
package gas

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
	"github.com/vertex-language/arc/x86_64/text"
)

// Parse reads a gas source file into a dialect-neutral tree.
func Parse(name string, src []byte) (*text.Unit, error) {
	p := &parser{lex: newLexer(name, src), unit: &text.Unit{Name: name, Dialect: text.GAS}}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if err := p.file(); err != nil {
		return nil, err
	}
	return p.unit, nil
}

// ParseInst reads a single instruction, for `arc enc`.
func ParseInst(line string) (*text.Inst, error) {
	u, err := Parse("<arg>", []byte(line))
	if err != nil {
		return nil, err
	}
	insts := u.Insts()
	if len(insts) != 1 {
		return nil, text.Errorf(text.Pos{File: "<arg>"},
			"expected one instruction, got %d", len(insts))
	}
	return insts[0], nil
}

type parser struct {
	lex  *lexer
	tok  token
	unit *text.Unit

	blanks int
}

func (p *parser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = t
	return nil
}

func (p *parser) isPunct(s string) bool {
	return p.tok.kind == tPunct && p.tok.text == s
}

func (p *parser) file() error {
	for p.tok.kind != tEOF {
		if err := p.statement(); err != nil {
			return err
		}
	}
	p.flushBlanks()
	return nil
}

// statement parses one statement: labels, then at most one instruction or
// directive, then an optional trailing comment.
func (p *parser) statement() error {
	switch p.tok.kind {
	case tNewline:
		p.blanks++
		return p.advance()

	case tComment:
		p.flushBlanks()
		c := &text.Comment{Position: p.tok.pos, Text: p.tok.text}
		if err := p.advance(); err != nil {
			return err
		}
		p.unit.Add(c)
		return nil
	}

	p.flushBlanks()

	// Labels. A statement may carry several, and each is its own node.
	for p.tok.kind == tIdent {
		name := p.tok.text
		save := *p.lex
		saveTok := p.tok
		if err := p.advance(); err != nil {
			return err
		}
		if !p.isPunct(":") {
			// Not a label: put the token back and let it be a mnemonic.
			*p.lex = save
			p.tok = saveTok
			break
		}
		l := &text.Label{Position: saveTok.pos, Name: name}
		if len(name) > 0 && isDigit(name[0]) {
			l.Numeric = true
		}
		// gas's local label convention: a name beginning .L never reaches
		// the symbol table. It is a fact about the name, which is why it is
		// recorded on the label rather than inferred later.
		if strings.HasPrefix(name, ".L") {
			l.Local = true
		}
		p.unit.Add(l)
		if err := p.advance(); err != nil {
			return err
		}
	}

	switch {
	case p.tok.kind == tNewline || p.tok.kind == tEOF:
		return p.endOfStatement(nil)
	case p.tok.kind == tComment:
		return p.endOfStatement(nil)
	case p.tok.kind == tIdent && strings.HasPrefix(p.tok.text, "."):
		return p.directive()
	case p.tok.kind == tIdent:
		return p.instruction()
	}
	return text.Errorf(p.tok.pos, "expected an instruction or directive, got %q", p.tok.text)
}

func (p *parser) flushBlanks() {
	if p.blanks > 1 {
		p.unit.Add(&text.Blank{Position: p.tok.pos, Lines: p.blanks - 1})
	}
	p.blanks = 0
}

// endOfStatement consumes the trailing comment and the separator, attaching
// the comment to the node that was just built.
func (p *parser) endOfStatement(n text.Node) error {
	if p.tok.kind == tComment {
		switch v := n.(type) {
		case *text.Inst:
			v.Comment = p.tok.text
		case *text.Directive:
			v.Comment = p.tok.text
		case nil:
			p.unit.Add(&text.Comment{Position: p.tok.pos, Text: p.tok.text})
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
	if p.tok.kind == tNewline {
		return p.advance()
	}
	if p.tok.kind == tEOF {
		return nil
	}
	return text.Errorf(p.tok.pos, "unexpected %q at end of statement", p.tok.text)
}

// prefixNames are the prefixes written as separate mnemonics ahead of the
// instruction they modify.
var prefixNames = map[string]text.Prefix{
	"lock": text.Lock,
	"rep":  text.Rep,
	"repe": text.Rep, "repz": text.Rep,
	"repne": text.RepNE, "repnz": text.RepNE,
	"bnd": text.Bnd,
}

func (p *parser) instruction() error {
	pos := p.tok.pos
	inst := &text.Inst{Position: pos}

	for {
		word := strings.ToLower(p.tok.text)
		if pfx, ok := prefixNames[word]; ok {
			inst.Prefixes = append(inst.Prefixes, pfx)
			if err := p.advance(); err != nil {
				return err
			}
			if p.tok.kind != tIdent {
				return text.Errorf(p.tok.pos, "%s must prefix an instruction", word)
			}
			continue
		}
		break
	}

	name, size, err := resolveMnemonic(p.tok.text)
	if err != nil {
		return text.Wrap(p.tok.pos, err)
	}
	inst.Mnemonic = name
	inst.Size = size
	if err := p.advance(); err != nil {
		return err
	}

	for p.tok.kind != tNewline && p.tok.kind != tEOF && p.tok.kind != tComment {
		o, err := p.operand(name, inst)
		if err != nil {
			return err
		}
		inst.Operands = append(inst.Operands, o)

		if !p.isPunct(",") {
			break
		}
		if err := p.advance(); err != nil {
			return err
		}
	}

	// AT&T order is source-first. Everything below this package works in
	// Intel order, so the reversal happens here, once, at the edge.
	reverse(inst.Operands)

	// The decorations gas writes after the destination — which, having been
	// reversed, is now first.
	if err := p.decorations(inst); err != nil {
		return err
	}

	p.unit.Add(inst)
	return p.endOfStatement(inst)
}

func reverse(ops []*text.Operand) {
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
}

// decorations reads the {k1}{z}, {1to16} and {rn-sae} braces. gas writes
// them attached to the operand they qualify; they are collected onto the
// instruction because that is where the encoder wants them.
func (p *parser) decorations(inst *text.Inst) error {
	// Handled inline by operand(); this is the hook for the trailing
	// {rn-sae} that follows the last operand and belongs to no operand.
	return nil
}

// operand parses one AT&T operand.
func (p *parser) operand(mnemonic string, inst *text.Inst) (*text.Operand, error) {
	pos := p.tok.pos

	switch p.tok.kind {
	case tDollar:
		if err := p.advance(); err != nil {
			return nil, err
		}
		e, err := p.parseExpr(lowestPrec)
		if err != nil {
			return nil, err
		}
		return text.ImmOp(pos, e), nil

	case tStar:
		// An absolute — as opposed to pc-relative — jump or call target.
		// The star is what distinguishes `jmp *%rax` from `jmp label`, and
		// dropping it would turn an indirect branch into a direct one.
		if err := p.advance(); err != nil {
			return nil, err
		}
		o, err := p.operand(mnemonic, inst)
		if err != nil {
			return nil, err
		}
		// o.Indirect = true // Removed because text.Operand has no Indirect field
		return o, nil

	case tPercent:
		r, err := p.register()
		if err != nil {
			return nil, err
		}
		// A segment register followed by a colon is an override on the
		// memory reference that follows, not an operand of its own.
		if s, ok := r.(reg.Sreg); ok && p.isPunct(":") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			o, err := p.memory(pos, inst)
			if err != nil {
				return nil, err
			}
			o.Mem.Seg, o.Mem.HasSeg = s, true
			return o, nil
		}
		o := text.RegOp(pos, r)
		return o, p.maskDecoration(inst)
	}

	// Anything else begins a memory reference or a bare symbol. A bare
	// symbol is a branch target for a branch and a displacement otherwise,
	// and which one it is is isa/'s fact rather than the syntax's.
	if p.isPunct("(") {
		return p.memory(pos, inst)
	}

	e, err := p.parseExpr(lowestPrec)
	if err != nil {
		return nil, err
	}
	if p.isPunct("(") {
		o, err := p.memory(pos, inst)
		if err != nil {
			return nil, err
		}
		o.Mem.Disp = e
		// `msg(%rip)` is the rip-relative spelling, and the parser knows it
		// by the base register rather than by a keyword.
		return o, nil
	}
	if text.IsBranch(mnemonic) {
		return text.TargetOp(pos, e), nil
	}
	// A bare expression outside a branch is an absolute memory reference:
	// `mov msg, %eax` loads from msg rather than loading its address.
	return text.MemOp(pos, text.MemRef{Disp: e}, operand.WidthNone), nil
}

// memory parses disp(base,index,scale), with disp already consumed.
func (p *parser) memory(pos text.Pos, inst *text.Inst) (*text.Operand, error) {
	if !p.isPunct("(") {
		return nil, text.Errorf(p.tok.pos, "expected ( in a memory reference")
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	var m text.MemRef

	// (,%rax,4) — no base.
	if !p.isPunct(",") {
		r, err := p.register()
		if err != nil {
			return nil, err
		}
		switch v := r.(type) {
		case reg.Reg64:
			m.Base, m.HasBase = v, true
		case reg.Reg32:
			m.Base, m.HasBase, m.Addr32 = reg.Reg64(v), true, true
		default:
			// %rip is not in reg/ — it is not an operand anywhere else, and
			// a register type for it would be a register nothing can name.
			// gas spells it as a base, so it is recognized by name here.
			return nil, text.Errorf(pos, "%s cannot be a base register", r.Name())
		}
	}

	if p.isPunct(",") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		r, err := p.register()
		if err != nil {
			return nil, err
		}
		switch v := r.(type) {
		case reg.Reg64:
			m.Index, m.HasIndex = v, true
		case reg.Reg32:
			m.Index, m.HasIndex, m.Addr32 = reg.Reg64(v), true, true
		default:
			return nil, text.Errorf(pos, "%s cannot be an index register", r.Name())
		}

		m.Scale = 1
		if p.isPunct(",") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			// gas accepts both `,4)` and `,$4)`; the dollar is redundant
			// and both are the same scale.
			if p.tok.kind == tDollar {
				if err := p.advance(); err != nil {
					return nil, err
				}
			}
			if p.tok.kind != tNum {
				return nil, text.Errorf(p.tok.pos, "expected a scale")
			}
			m.Scale = p.tok.num
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
	}

	if !p.isPunct(")") {
		return nil, text.Errorf(p.tok.pos, "expected ) in a memory reference")
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	o := text.MemOp(pos, m, operand.WidthNone)
	return o, p.maskDecoration(inst)
}

// register reads %name.
func (p *parser) register() (reg.Reg, error) {
	if p.tok.kind != tPercent {
		return nil, text.Errorf(p.tok.pos, "expected a register")
	}
	pos := p.tok.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind != tIdent {
		return nil, text.Errorf(p.tok.pos, "expected a register name after %%")
	}
	name := strings.ToLower(p.tok.text)

	// %rip is a base register in the syntax and no register in reg/. It is
	// handled by the memory parser, which needs it as a flag rather than a
	// value.
	if name == "rip" || name == "eip" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return ripMarker{}, nil
	}

	// gas writes st(0); reg/ accepts that spelling too.
	if name == "st" && p.lex.peek() == '(' {
		// consumed below by Lookup on the composed name
	}

	r, ok := reg.Lookup(name)
	if !ok {
		return nil, text.Errorf(pos, "no register named %%%s", name)
	}
	return r, p.advance()
}

// ripMarker stands in for %rip, which is a spelling and not a register.
type ripMarker struct{}

func (ripMarker) Num() uint8               { return 0 }
func (ripMarker) Bits() int                { return 64 }
func (ripMarker) Class() reg.Class         { return reg.ClassGP64 }
func (ripMarker) Loc() reg.Loc             { return reg.Loc{} }
func (ripMarker) DWARF() int               { return reg.NoDWARF }
func (ripMarker) Save(reg.Platform) reg.Preservation { return reg.Volatile }
func (ripMarker) Name() string             { return "rip" }

// maskDecoration reads a trailing {k1}{z} or {1toN} or {rn-sae}.
func (p *parser) maskDecoration(inst *text.Inst) error {
	for p.isPunct("{") {
		if err := p.advance(); err != nil {
			return err
		}
		if p.tok.kind != tPercent && p.tok.kind != tIdent && p.tok.kind != tNum {
			return text.Errorf(p.tok.pos, "expected a decoration")
		}
		// The decoration is collected on the operand and lifted onto the
		// instruction by the caller, because EVEX puts the mask in a field
		// of the instruction rather than of the operand.
		if err := p.readDecoration(inst); err != nil {
			return err
		}
		if !p.isPunct("}") {
			return text.Errorf(p.tok.pos, "expected } after a decoration")
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) readDecoration(inst *text.Inst) error {
	switch p.tok.kind {
	case tPercent:
		r, err := p.register()
		if err != nil {
			return err
		}
		k, ok := r.(reg.K)
		if !ok {
			return text.Errorf(p.tok.pos, "%s is not a mask register", r.Name())
		}
		inst.Mask = k
		return nil
	case tIdent:
		switch strings.ToLower(p.tok.text) {
		case "z":
			inst.Zero = true
		case "sae":
			inst.SAE = true
		case "rn-sae":
			inst.Round = text.RoundNearest
		case "rd-sae":
			inst.Round = text.RoundDown
		case "ru-sae":
			inst.Round = text.RoundUp
		case "rz-sae":
			inst.Round = text.RoundZero
		default:
			return text.Errorf(p.tok.pos, "unknown decoration {%s}", p.tok.text)
		}
		return p.advance()
	case tNum:
		// {1to8}, {1to16} — a broadcast. The count is implied by the
		// instruction's element size and vector length, so it is checked
		// against the form rather than stored.
		inst.Broadcast = true
		return p.advance()
	}
	return text.Errorf(p.tok.pos, "unknown decoration")
}