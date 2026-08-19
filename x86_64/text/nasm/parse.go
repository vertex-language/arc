// x86_64/text/nasm/parse.go
package nasm

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
	"github.com/vertex-language/arc/x86_64/text"
)

// Parse reads a NASM source file into a dialect-neutral tree.
func Parse(name string, src []byte) (*text.Unit, error) {
	p := &parser{lex: newLexer(name, src), unit: &text.Unit{Name: name, Dialect: text.NASM}}
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

func (p *parser) atEnd() bool {
	return p.tok.kind == tNewline || p.tok.kind == tEOF || p.tok.kind == tComment
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

// prefixNames are the prefixes written as separate words ahead of the
// instruction they modify.
var prefixNames = map[string]text.Prefix{
	"lock": text.Lock,
	"rep":  text.Rep, "repe": text.Rep, "repz": text.Rep,
	"repne": text.RepNE, "repnz": text.RepNE,
	"bnd": text.Bnd,
}

// statement parses one statement.
//
// A NASM label needs no colon: an identifier that begins a line and is not a
// mnemonic, a prefix or a directive is a label definition. That rule is the
// reason the lexer marks the first token of a line — nothing else in this
// package cares where a token sits, and this cannot be answered without it.
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

	case tPreproc:
		return text.Wrap(p.tok.pos, text.ErrMacro)
	}

	p.flushBlanks()

	var label *text.Label
	if p.tok.kind == tIdent && p.tok.bol && p.isLabel() {
		label = &text.Label{Position: p.tok.pos, Name: p.tok.text}
		// A name beginning with '.' is local to the preceding label. It is
		// a fact about the name, which is why it is recorded on the label
		// rather than inferred later — and why it is not the directive
		// marker the same character is in gas.
		if strings.HasPrefix(label.Name, ".") {
			label.Local = true
		}
		if err := p.advance(); err != nil {
			return err
		}
		if p.isPunct(":") {
			if err := p.advance(); err != nil {
				return err
			}
		}
	}

	// `NAME equ 8` names the symbol on the left of the directive rather
	// than after it, which is the one place NASM puts an argument before
	// the keyword.
	if label != nil && p.tok.kind == tIdent && strings.EqualFold(p.tok.text, "equ") {
		pos := p.tok.pos
		if err := p.advance(); err != nil {
			return err
		}
		e, err := p.parseExpr(lowestPrec)
		if err != nil {
			return err
		}
		d := &text.Directive{Position: pos, Kind: text.Equ, Raw: "equ", Args: []text.Expr{
			&text.Sym{Position: label.Position, Name: label.Name}, e,
		}}
		p.unit.Add(d)
		return p.endOfStatement(d)
	}

	if label != nil {
		p.unit.Add(label)
	}

	switch {
	case p.atEnd():
		return p.endOfStatement(nil)
	case p.tok.kind == tPreproc:
		return text.Wrap(p.tok.pos, text.ErrMacro)
	case p.tok.kind != tIdent:
		return text.Errorf(p.tok.pos,
			"expected an instruction or directive, got %q", p.tok.text)
	}

	word := strings.ToLower(p.tok.text)
	if isDirectiveWord(word) {
		return p.directive(p.tok.pos, word)
	}
	return p.instruction()
}

// isLabel reports whether the identifier at the front of a line defines a
// label rather than beginning a statement. A trailing colon settles it; so
// does the name not being anything else.
func (p *parser) isLabel() bool {
	word := strings.ToLower(p.tok.text)
	if _, ok := prefixNames[word]; ok {
		return false
	}
	if isDirectiveWord(word) {
		return false
	}
	if _, _, err := resolveMnemonic(word); err == nil {
		return false
	}
	return true
}

func isDirectiveWord(w string) bool {
	if _, ok := nasmDirectives[w]; ok {
		return true
	}
	if _, ok := unsupportedDirectives[w]; ok {
		return true
	}
	return w == "times" || w == "bits"
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

// ccAliases are the condition-code spellings that name an existing encoding.
// isa/ declares sixteen conditions; these are the other sixteen names for
// them, and a table row per alias would be a row nothing distinguishes.
var ccAliases = map[string]string{
	"nae": "b", "c": "b",
	"nb": "ae", "nc": "ae",
	"z":  "e",
	"nz": "ne",
	"na":  "be",
	"nbe": "a",
	"pe": "p", "po": "np",
	"nge": "l", "nl": "ge",
	"ng": "le", "nle": "g",
}

var ccPrefixes = []string{"j", "set", "cmov"}

// resolveMnemonic folds a written mnemonic into the canonical name isa/
// knows.
//
// There is no suffix to strip: NASM states an operand size on the operand,
// not on the mnemonic, which is why this is four lines and gas's is forty.
// The size that gas recovers from a suffix, this recovers from the operand
// keyword — and the size neither states is the form's, which is why arc fmt
// resolves before it prints.
func resolveMnemonic(s string) (name string, size operand.Width, err error) {
	s = strings.ToLower(s)
	if isa.Forms(s) != nil {
		return s, operand.WidthNone, nil
	}
	if n, ok := resolveCC(s); ok {
		return n, operand.WidthNone, nil
	}
	return "", operand.WidthNone, text.Errorf(text.Pos{}, "unknown instruction %q", s)
}

func resolveCC(s string) (string, bool) {
	for _, pfx := range ccPrefixes {
		if !strings.HasPrefix(s, pfx) {
			continue
		}
		if alias, ok := ccAliases[s[len(pfx):]]; ok {
			if name := pfx + alias; isa.Forms(name) != nil {
				return name, true
			}
		}
	}
	return "", false
}

func (p *parser) instruction() error {
	pos := p.tok.pos
	inst := &text.Inst{Position: pos}

	for {
		word := strings.ToLower(p.tok.text)
		pfx, ok := prefixNames[word]
		if !ok {
			break
		}
		inst.Prefixes = append(inst.Prefixes, pfx)
		if err := p.advance(); err != nil {
			return err
		}
		if p.tok.kind != tIdent {
			return text.Errorf(p.tok.pos, "%s must prefix an instruction", word)
		}
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

	for !p.atEnd() {
		o, err := p.operand(name)
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

	// No reversal. NASM writes destination first and so does the tree,
	// which is the whole of the difference: gas reverses at both of its
	// edges and this reverses at neither.
	lift(inst)

	p.unit.Add(inst)
	return p.endOfStatement(inst)
}

// lift copies the EVEX decorations from the operands they were written on
// to the instruction they belong to. The syntax attaches a writemask to the
// destination; the encoding puts it in a field of the instruction, and
// text.Inst.Lower reads it from there.
func lift(i *text.Inst) {
	for _, o := range i.Operands {
		if o.Mask != 0 {
			i.Mask = o.Mask
		}
		i.Zero = i.Zero || o.Zero
		i.Broadcast = i.Broadcast || o.Broadcast
		i.SAE = i.SAE || o.SAE
		if o.Round != text.RoundNone {
			i.Round = o.Round
		}
	}
}

// sizeKeywords are NASM's operand-size words.
var sizeKeywords = map[string]operand.Width{
	"byte":    operand.W8,
	"word":    operand.W16,
	"dword":   operand.W32,
	"qword":   operand.W64,
	"oword":   operand.W128,
	"xmmword": operand.W128,
	"yword":   operand.W256,
	"ymmword": operand.W256,
	"zword":   operand.W512,
	"zmmword": operand.W512,
}

// operand parses one Intel-syntax operand.
func (p *parser) operand(mnemonic string) (*text.Operand, error) {
	pos := p.tok.pos
	size := operand.WidthNone

	for p.tok.kind == tIdent {
		word := strings.ToLower(p.tok.text)
		if word == "strict" {
			// `strict` forbids the encoder from narrowing an immediate.
			// The typed helper layer is how that is asked for in this tree
			// and there is no operand field for it, so it is refused rather
			// than accepted and ignored — accepting it would mean promising
			// an encoding this path cannot deliver.
			return nil, text.Errorf(p.tok.pos,
				"strict has no representation here; name the form through the typed API")
		}
		if word == "tword" {
			return nil, text.Errorf(p.tok.pos,
				"tword is 80 bits, which is not an operand width on this target")
		}
		w, ok := sizeKeywords[word]
		if !ok {
			break
		}
		size = w
		if err := p.advance(); err != nil {
			return nil, err
		}
	}

	if p.isPunct("[") {
		o, err := p.memory(pos, size)
		if err != nil {
			return nil, err
		}
		if text.IsBranch(mnemonic) {
			// `jmp [rbx]` is an indirect branch. NASM writes no marker and
			// gas writes a star, so the fact lives on the operand: gas's
			// printer needs it and NASM's drops it.
			o.Indirect = true
		}
		return o, p.decorations(o)
	}

	if p.tok.kind == tIdent {
		if r, ok := lookupRegister(p.tok.text); ok {
			rpos := p.tok.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			// A segment register and a colon override the reference that
			// follows rather than being an operand of their own.
			if s, isSeg := r.(reg.Sreg); isSeg && p.isPunct(":") {
				if err := p.advance(); err != nil {
					return nil, err
				}
				if !p.isPunct("[") {
					return nil, text.Errorf(p.tok.pos,
						"a segment override qualifies a memory reference")
				}
				o, err := p.memory(pos, size)
				if err != nil {
					return nil, err
				}
				o.Mem.Seg, o.Mem.HasSeg = s, true
				return o, p.decorations(o)
			}
			o := text.RegOp(rpos, r)
			if text.IsBranch(mnemonic) {
				o.Indirect = true
			}
			return o, p.decorations(o)
		}
	}

	e, err := p.parseExpr(lowestPrec)
	if err != nil {
		return nil, err
	}
	if text.IsBranch(mnemonic) {
		return text.TargetOp(pos, e), nil
	}
	// A bare symbol outside a branch is the symbol's address, not a load
	// through it: `mov rax, msg` is an immediate here and a load in gas.
	// Same three tokens, different instruction — which is why the two
	// parsers cannot share this line either.
	o := text.ImmOp(pos, e)
	o.Size = size
	return o, nil
}

// memory parses [seg:rel base + index*scale + disp].
//
// The bracket holds a sum of terms, and a term is either a register, a
// scaled register, or a displacement. That is why an operator looser than
// multiplication has to be parenthesized in here: '+' and '-' separate the
// terms, so [rbx+1<<3] is a diagnostic and [rbx+(1<<3)] is the way to write
// it. NASM is more permissive; this refuses rather than guesses.
func (p *parser) memory(pos text.Pos, size operand.Width) (*text.Operand, error) {
	if err := p.advance(); err != nil { // past '['
		return nil, err
	}

	var m text.MemRef

	if p.tok.kind == tIdent {
		switch strings.ToLower(p.tok.text) {
		case "rel":
			m.RIP = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		case "abs":
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
	}

	if p.tok.kind == tIdent {
		if r, ok := lookupRegister(p.tok.text); ok {
			if s, isSeg := r.(reg.Sreg); isSeg {
				if err := p.advance(); err != nil {
					return nil, err
				}
				if !p.isPunct(":") {
					return nil, text.Errorf(p.tok.pos,
						"a segment register inside [ ] must be followed by :")
				}
				m.Seg, m.HasSeg = s, true
				if err := p.advance(); err != nil {
					return nil, err
				}
			}
		}
	}

	var disp text.Expr
	first := true
	for !p.isPunct("]") {
		if p.tok.kind == tEOF || p.tok.kind == tNewline {
			return nil, text.Errorf(p.tok.pos, "expected ] in a memory reference")
		}
		neg := false
		switch {
		case p.isPunct("+"):
			if err := p.advance(); err != nil {
				return nil, err
			}
		case p.isPunct("-"):
			neg = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		default:
			if !first {
				return nil, text.Errorf(p.tok.pos,
					"expected + or - between terms of a memory reference")
			}
		}
		first = false
		if err := p.memTerm(&m, &disp, neg); err != nil {
			return nil, err
		}
	}
	if err := p.advance(); err != nil { // past ']'
		return nil, err
	}

	m.Disp = disp
	o := text.MemOp(pos, m, size)
	return o, o.Validate()
}

func (p *parser) memTerm(m *text.MemRef, disp *text.Expr, neg bool) error {
	pos := p.tok.pos

	// A register, optionally scaled.
	if p.tok.kind == tIdent {
		if r, ok := lookupRegister(p.tok.text); ok {
			if err := p.advance(); err != nil {
				return err
			}
			scale := int64(1)
			if p.isPunct("*") {
				if err := p.advance(); err != nil {
					return err
				}
				if p.tok.kind != tNum {
					return text.Errorf(p.tok.pos, "expected a scale")
				}
				scale = p.tok.num
				if err := p.advance(); err != nil {
					return err
				}
			}
			if neg {
				return text.Errorf(pos, "a register term cannot be subtracted")
			}
			return addReg(m, r, scale, pos)
		}
	}

	// A scale written first: [4*rcx].
	if p.tok.kind == tNum {
		save := *p.lex
		saveTok := p.tok
		n := p.tok.num
		if err := p.advance(); err != nil {
			return err
		}
		if p.isPunct("*") {
			if err := p.advance(); err != nil {
				return err
			}
			if p.tok.kind == tIdent {
				if r, ok := lookupRegister(p.tok.text); ok {
					if err := p.advance(); err != nil {
						return err
					}
					if neg {
						return text.Errorf(pos, "a register term cannot be subtracted")
					}
					return addReg(m, r, n, pos)
				}
			}
		}
		*p.lex = save
		p.tok = saveTok
	}

	e, err := p.parseExpr(1)
	if err != nil {
		return err
	}
	if neg {
		e = &text.Unary{Position: pos, Op: text.OpNeg, X: e}
	}
	if *disp == nil {
		*disp = e
	} else {
		*disp = &text.Binary{Position: pos, Op: text.OpAdd, X: *disp, Y: e}
	}
	return nil
}

// addReg places a register term in the base or the index.
//
// The first unscaled register is the base and anything after it is the
// index, which is what NASM does and what the encoding wants: [rax+rbx] has
// one of each and [rbx*1] has only an index, because a scale written at all
// names the index field.
func addReg(m *text.MemRef, r reg.Reg, scale int64, pos text.Pos) error {
	var r64 reg.Reg64
	switch v := r.(type) {
	case reg.Reg64:
		r64 = v
	case reg.Reg32:
		r64 = reg.Reg64(v)
		m.Addr32 = true
	default:
		return text.Errorf(pos, "%s cannot address memory", r.Name())
	}

	if m.RIP {
		return text.Errorf(pos, "rip-relative addressing takes no base or index")
	}
	switch {
	case scale == 1 && !m.HasBase:
		m.Base, m.HasBase = r64, true
	case !m.HasIndex:
		m.Index, m.HasIndex, m.Scale = r64, true, scale
	default:
		return text.Errorf(pos, "a memory reference has at most one index register")
	}
	return nil
}

// lookupRegister resolves a bare register name. NASM writes no sigil, so a
// register and a symbol are told apart by the name alone — which is safe
// because the names are reserved, and is why `$eax` exists as an escape.
func lookupRegister(name string) (reg.Reg, bool) {
	return reg.Lookup(strings.ToLower(name))
}

// decorations reads the {k1}{z}, {1toN}, {sae} and {rn-sae} braces that
// follow an operand.
func (p *parser) decorations(o *text.Operand) error {
	for p.isPunct("{") {
		if err := p.advance(); err != nil {
			return err
		}
		if err := p.readDecoration(o); err != nil {
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

func (p *parser) readDecoration(o *text.Operand) error {
	switch p.tok.kind {
	case tIdent:
		word := strings.ToLower(p.tok.text)
		if r, ok := lookupRegister(word); ok {
			k, isMask := r.(reg.K)
			if !isMask {
				return text.Errorf(p.tok.pos, "%s is not a mask register", r.Name())
			}
			o.Mask = k
			return p.advance()
		}
		switch word {
		case "z":
			o.Zero = true
		case "sae":
			o.SAE = true
		case "rn-sae":
			o.Round = text.RoundNearest
		case "rd-sae":
			o.Round = text.RoundDown
		case "ru-sae":
			o.Round = text.RoundUp
		case "rz-sae":
			o.Round = text.RoundZero
		default:
			return text.Errorf(p.tok.pos, "unknown decoration {%s}", p.tok.text)
		}
		return p.advance()

	case tNum:
		// {1to8}, {1to16} — a broadcast. The count is implied by the
		// form's element size and vector length, so it is checked against
		// the form rather than stored.
		o.Broadcast = true
		if err := p.advance(); err != nil {
			return err
		}
		if p.tok.kind == tIdent {
			return p.advance() // the "to16" half, when the lexer split it
		}
		return nil
	}
	return text.Errorf(p.tok.pos, "unknown decoration")
}