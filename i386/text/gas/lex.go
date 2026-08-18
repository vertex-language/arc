// Package gas is the GNU as dialect of i386 assembly: AT&T syntax, parsed to
// and printed from text.Unit.
//
// The reference implementation is GNU as, and that is what bounds the
// round-trip claim. internal/difftest/gas tests this directory against it;
// the guarantee is per-directory and provable per-directory, which is why
// each dialect gets exactly one reference and why there is no att/ directory
// beside this one — att and intel are aliases resolved in cmd/arc/target.go
// and nowhere else.
//
// Four things about AT&T are load-bearing here and are handled in one place
// each rather than everywhere:
//
//   - Operands are written source-first and stored destination-first. The
//     reversal happens at the edges of this package and nowhere in the tree
//     above it, which is what lets one form table serve both dialects.
//   - The operand size is a mnemonic suffix. movl is not an instruction; it
//     is mov with Size 32, and the suffix is stripped against the ISA table
//     rather than against a list, so `call` does not lose its l.
//   - A bare expression is a memory operand, not an immediate. `mov foo,
//     %eax` loads through foo. An immediate carries $ and a branch target
//     carries nothing, which is why the parser asks isa/ whether a mnemonic
//     has a rel form before it decides what a bare name is.
//   - A relocation is spelled name@PLT. The @ is this dialect's sigil; the
//     word after it is the psABI's and resolves to text.Modifier, never to a
//     number.
package gas

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386/reg"
	"github.com/vertex-language/arc/i386/text"
)

// kind is a token kind.
type kind uint8

const (
	tEOF kind = iota
	tEOL
	tIdent
	tNumber
	tString
	tReg
	tPunct
)

// token is one lexical item.
type token struct {
	kind kind
	pos  text.Pos
	str  string // identifier, punctuation spelling, or decoded string body
	num  int64
	reg  reg.Reg
}

func (t token) is(p string) bool { return t.kind == tPunct && t.str == p }

func (t token) String() string {
	switch t.kind {
	case tEOF:
		return "end of file"
	case tEOL:
		return "end of line"
	case tIdent:
		return t.str
	case tNumber:
		return fmt.Sprintf("%d", t.num)
	case tString:
		return "a string"
	case tReg:
		return "%" + t.reg.Name()
	}
	return "'" + t.str + "'"
}

// lexer scans one source file.
//
// It also collects trivia, because arc fmt rewrites files in place and a
// formatter that dropped comments would be one nobody could run. Comment text
// is stored without the '#', since the introducer is a spelling and text/
// keeps none.
type lexer struct {
	src  []byte
	file string
	pos  int
	line int
	col  int

	errs *text.ErrorList

	// pending trivia, consumed by the parser when a statement begins.
	blanks  int
	before  []string
	comment string
	hasCmt  bool
}

func newLexer(file string, src []byte, errs *text.ErrorList) *lexer {
	return &lexer{src: src, file: file, line: 1, col: 1, errs: errs}
}

func (l *lexer) at() text.Pos { return text.Pos{File: l.file, Line: l.line, Col: l.col} }

func (l *lexer) errorf(p text.Pos, format string, args ...any) *text.Error {
	e := text.Errorf(p, format, args...)
	l.errs.Add(e)
	return e
}

func (l *lexer) peekByte() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) at1() byte {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

// takeTrivia hands the accumulated comments and blank lines to the parser and
// clears them.
func (l *lexer) takeTrivia() text.Trivia {
	t := text.Trivia{Blanks: l.blanks, Before: l.before}
	l.blanks, l.before = 0, nil
	return t
}

// takeComment hands over the trailing comment of the line just ended.
func (l *lexer) takeComment() (string, bool) {
	s, ok := l.comment, l.hasCmt
	l.comment, l.hasCmt = "", false
	return s, ok
}

// next returns the next token.
//
// Newlines are significant — a statement ends at one — so tEOL is a token
// rather than whitespace. The ';' separator produces one too: on x86 GNU as
// treats it as a line separator, which is how inline asm writes several
// statements in one string.
func (l *lexer) next() token {
	for {
		if l.pos >= len(l.src) {
			return token{kind: tEOF, pos: l.at()}
		}

		c := l.peekByte()
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			l.advance()
			continue

		case c == '\n':
			p := l.at()
			l.advance()
			return token{kind: tEOL, pos: p}

		case c == ';':
			p := l.at()
			l.advance()
			return token{kind: tEOL, pos: p}

		case c == '#':
			l.lineComment()
			continue

		case c == '/' && l.at1() == '*':
			l.blockComment()
			continue
		}
		break
	}

	p := l.at()
	c := l.peekByte()

	switch {
	case c == '%':
		return l.register(p)
	case c == '"':
		return l.stringLit(p)
	case c == '\'':
		return l.charLit(p)
	case c >= '0' && c <= '9':
		return l.number(p)
	case isIdentStart(c):
		return l.ident(p)
	}
	return l.punct(p)
}

// lineComment consumes '#' to end of line.
//
// Whether it is a trailing comment or a whole-line one depends on whether
// anything preceded it on the line, which the column tells us: a comment
// starting in column 1 after only whitespace is its own line.
func (l *lexer) lineComment() {
	own := l.onlyWhitespaceBefore()
	l.advance() // '#'
	start := l.pos
	for l.pos < len(l.src) && l.peekByte() != '\n' {
		l.advance()
	}
	body := strings.TrimRight(string(l.src[start:l.pos]), " \t\r")

	if own {
		l.before = append(l.before, body)
		if l.pos < len(l.src) {
			l.advance() // the newline belongs to the comment, not to a statement
		}
		return
	}
	l.comment, l.hasCmt = body, true
}

// blockComment consumes a C comment.
//
// GNU as accepts these and arc fmt normalises them to line comments: the
// content survives, the spelling does not. That is a text change and not a
// byte change, which is the only kind arc fmt is allowed to make.
func (l *lexer) blockComment() {
	own := l.onlyWhitespaceBefore()
	p := l.at()
	l.advance()
	l.advance()
	start := l.pos
	for {
		if l.pos >= len(l.src) {
			l.errorf(p, "unterminated block comment")
			break
		}
		if l.peekByte() == '*' && l.at1() == '/' {
			break
		}
		l.advance()
	}
	body := string(l.src[start:l.pos])
	if l.pos < len(l.src) {
		l.advance()
		l.advance()
	}

	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(strings.TrimLeft(lines[i], " \t"), " \t\r")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if own {
		l.before = append(l.before, lines...)
		return
	}
	l.comment, l.hasCmt = strings.Join(lines, " "), true
}

func (l *lexer) onlyWhitespaceBefore() bool {
	for i := l.pos - 1; i >= 0; i-- {
		switch l.src[i] {
		case ' ', '\t', '\r':
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

// countBlank is called by the parser when it consumes an empty line.
func (l *lexer) countBlank() { l.blanks++ }

// register scans %eax, %st(0), %gs.
func (l *lexer) register(p text.Pos) token {
	l.advance() // '%'
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peekByte()) {
		l.advance()
	}
	name := strings.ToLower(string(l.src[start:l.pos]))

	// %st(0) is this dialect's spelling of st0, and %st alone is %st(0).
	// reg/ declares neither, because the parentheses are syntax.
	if name == "st" {
		if l.peekByte() == '(' {
			save := l.pos
			l.advance()
			d := l.peekByte()
			if d >= '0' && d <= '7' && l.at1() == ')' {
				l.advance()
				l.advance()
				name = "st" + string(d)
			} else {
				l.pos = save
				name = "st0"
			}
		} else {
			name = "st0"
		}
	}

	// %db0 is what GNU as has always called a debug register. reg/ spells it
	// dr0, because that is the SDM's name; both arrive here.
	if len(name) == 3 && name[:2] == "db" && name[2] >= '0' && name[2] <= '7' {
		name = "dr" + name[2:]
	}

	r, ok := text.LookupRegister(name)
	if !ok {
		l.errorf(p, "unknown register %%%s", name).
			Note("arc regs --arch i386 lists what this target has")
		return token{kind: tReg, pos: p, reg: reg.EAX}
	}
	return token{kind: tReg, pos: p, reg: r}
}

// ident scans a symbol or directive name. A leading '.' is part of the name:
// .text is a directive and .L1 is a label, and only what follows tells them
// apart.
func (l *lexer) ident(p text.Pos) token {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peekByte()) {
		l.advance()
	}
	return token{kind: tIdent, pos: p, str: string(l.src[start:l.pos])}
}

// number scans an integer literal.
//
// Floats are rejected rather than parsed. The builder API declares seven data
// calls and none of them is a float, so accepting 0f3.14 here would make a
// thing .s files could say that Go could not.
func (l *lexer) number(p text.Pos) token {
	start := l.pos
	base := 10

	if l.peekByte() == '0' {
		switch lower(l.at1()) {
		case 'x':
			base = 16
			l.advance()
			l.advance()
			start = l.pos
		case 'b':
			base = 2
			l.advance()
			l.advance()
			start = l.pos
		case 'f', 'd', 'e':
			l.errorf(p, "floating-point literals are not accepted").
				Note("arc emits .byte, .word, .long and .quad; there is no float directive")
			for l.pos < len(l.src) && isNumPart(l.peekByte()) {
				l.advance()
			}
			return token{kind: tNumber, pos: p}
		default:
			if l.at1() >= '0' && l.at1() <= '7' {
				base = 8
				l.advance()
				start = l.pos
			}
		}
	}

	var v int64
	digits := 0
	for l.pos < len(l.src) {
		d, ok := digitVal(l.peekByte(), base)
		if !ok {
			break
		}
		v = v*int64(base) + int64(d)
		digits++
		l.advance()
	}
	if digits == 0 {
		v = 0
	}
	if l.pos < len(l.src) && isIdentPart(l.peekByte()) {
		l.errorf(p, "%q is not a number in base %d", string(l.src[start:l.pos+1]), base)
		for l.pos < len(l.src) && isIdentPart(l.peekByte()) {
			l.advance()
		}
	}
	return token{kind: tNumber, pos: p, num: v}
}

// stringLit scans a double-quoted string.
func (l *lexer) stringLit(p text.Pos) token {
	l.advance()
	var b strings.Builder
	for {
		if l.pos >= len(l.src) || l.peekByte() == '\n' {
			l.errorf(p, "unterminated string")
			break
		}
		c := l.advance()
		if c == '"' {
			break
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte(l.escape(p))
	}
	return token{kind: tString, pos: p, str: b.String()}
}

// charLit scans a character constant.
//
// GNU as accepts 'a with no closing quote, which is the older spelling and
// still the common one in hand-written source; 'a' is accepted too. Both are
// one byte and one number.
func (l *lexer) charLit(p text.Pos) token {
	l.advance()
	if l.pos >= len(l.src) {
		l.errorf(p, "unterminated character constant")
		return token{kind: tNumber, pos: p}
	}
	var v byte
	if l.peekByte() == '\\' {
		l.advance()
		v = l.escape(p)
	} else {
		v = l.advance()
	}
	if l.peekByte() == '\'' {
		l.advance()
	}
	return token{kind: tNumber, pos: p, num: int64(v)}
}

func (l *lexer) escape(p text.Pos) byte {
	if l.pos >= len(l.src) {
		l.errorf(p, "unterminated escape")
		return 0
	}
	c := l.advance()
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'f':
		return '\f'
	case 'b':
		return '\b'
	case 'v':
		return '\v'
	case 'a':
		return 7
	case '\\', '"', '\'':
		return c
	case 'x':
		var v int
		for l.pos < len(l.src) {
			d, ok := digitVal(l.peekByte(), 16)
			if !ok {
				break
			}
			v = v*16 + d
			l.advance()
		}
		return byte(v)
	}
	if c >= '0' && c <= '7' {
		v := int(c - '0')
		for i := 0; i < 2 && l.pos < len(l.src); i++ {
			d := l.peekByte()
			if d < '0' || d > '7' {
				break
			}
			v = v*8 + int(d-'0')
			l.advance()
		}
		return byte(v)
	}
	l.errorf(p, "unknown escape \\%c", c)
	return c
}

// punct scans an operator or delimiter. The two-character operators are
// matched first, so >> is a shift and not two comparisons.
func (l *lexer) punct(p text.Pos) token {
	if l.pos+1 < len(l.src) {
		two := string(l.src[l.pos : l.pos+2])
		switch two {
		case "<<", ">>", "==", "!=", "<=", ">=", "&&", "||", "<>":
			l.advance()
			l.advance()
			if two == "<>" {
				two = "!=" // GNU as spells inequality both ways
			}
			return token{kind: tPunct, pos: p, str: two}
		}
	}
	c := l.advance()
	return token{kind: tPunct, pos: p, str: string(c)}
}

// Identifier characters. GNU as allows $ and . inside a name, which is why a
// symbol may be spelled .L.str.1 and why the '.' of a directive is scanned as
// part of the word rather than as punctuation.
func isIdentStart(c byte) bool {
	return c == '_' || c == '.' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isNumPart(c byte) bool {
	return isIdentPart(c) || (c >= '0' && c <= '9') || c == '+' || c == '-'
}

func digitVal(c byte, base int) (int, bool) {
	var v int
	switch {
	case c >= '0' && c <= '9':
		v = int(c - '0')
	case c >= 'a' && c <= 'f':
		v = int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		v = int(c-'A') + 10
	default:
		return 0, false
	}
	if v >= base {
		return 0, false
	}
	return v, true
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}