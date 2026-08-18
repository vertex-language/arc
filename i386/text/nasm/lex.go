// Package nasm is the NASM dialect of i386 assembly: Intel syntax, parsed to
// and printed from text.Unit.
//
// The reference implementation is NASM 3.02, the current documented version
// as of this writing. internal/difftest/nasm tests this directory against
// it; the guarantee is per-directory, same as gas's.
//
// Four things about NASM syntax are load-bearing here and are handled in one
// place each rather than everywhere:
//
//   - Operands are written and stored in the same order: destination first.
//     There is no reversal at the edges of this package, unlike gas — one
//     form table serves both dialects because isa/'s Ops are already in this
//     order.
//   - The operand size is never a mnemonic suffix. It is a keyword — BYTE,
//     WORD, DWORD, QWORD, TWORD, OWORD — written before an operand, most
//     often a bracketed memory one. That hint is threaded through
//     text.Inst.Size, the same field gas fills from its suffix, because nothing
//     else in the shared tree carries it.
//   - A bracketed expression is a memory operand; a bare one is an immediate,
//     a register, or — for a branch mnemonic — a displacement. There is no
//     sigil to tell an immediate from a branch target, so both are stored as
//     text.Imm, exactly as gas already does for a branch target.
//   - A relocation is spelled name WRT ..plt, a postfix keyword rather than a
//     suffix. It resolves to the same text.Modifier gas's @PLT resolves to.
//
// What this dialect does not accept, deliberately: the preprocessor (%if,
// %macro, %define — a language feature, and Go is arc's), ABSOLUTE (no
// hypothetical unaddressed section to point it at), COMMON (no merged-symbol
// model in text/'s Attr set), INCBIN (arc does not open files the command
// line did not name), BITS/USE16/USE32 (arc's i386 target is fixed 32-bit
// protected mode), floating-point Dx operands, DY/DZ/RESY/RESZ (text.Width
// has no 32- or 64-byte size), SEG (no segment base to take in a flat
// target), and the ?: ^^ <=> operators (no ternary node and no boolean-xor or
// three-way-compare BinaryOp exists to hold them).
package nasm

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386/text"
)

type kind uint8

const (
	tEOF kind = iota
	tEOL
	tIdent
	tNumber
	tString
	tPunct
)

// token is one lexical item.
//
// forced marks an identifier that was escaped with a leading '$' — "if some
// other module... defines a symbol called eax, you can refer to $eax". The
// flag only ever changes how *this* token is classified as it is consumed;
// it does not survive into the tree, since text.SymExpr has nowhere to keep
// it. A symbol that collides with a register name and is later folded into a
// decomposed effective address is the one case that escape cannot save —
// documented, not silently handled.
type token struct {
	kind   kind
	pos    text.Pos
	str    string
	num    int64
	forced bool
}

func (t token) is(s string) bool { return t.kind == tPunct && t.str == s }

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
	}
	return "'" + t.str + "'"
}

type lexer struct {
	src  []byte
	file string
	pos  int
	line int
	col  int

	errs *text.ErrorList

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

func (l *lexer) takeTrivia() text.Trivia {
	t := text.Trivia{Blanks: l.blanks, Before: l.before}
	l.blanks, l.before = 0, nil
	return t
}

func (l *lexer) takeComment() (string, bool) {
	s, ok := l.comment, l.hasCmt
	l.comment, l.hasCmt = "", false
	return s, ok
}

func (l *lexer) countBlank() { l.blanks++ }

// next returns the next token. Newlines and ';' both end a statement; NASM
// has no line-separator character the way gas's ';' doubles as one, so the
// only source of tEOL here is a real newline.
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
		case c == '\\' && l.at1() == '\n':
			// Line continuation: swallow both bytes and keep scanning as if
			// the statement never broke.
			l.advance()
			l.advance()
			continue
		case c == '\n':
			p := l.at()
			l.advance()
			return token{kind: tEOL, pos: p}
		case c == ';':
			l.lineComment()
			continue
		}
		break
	}

	p := l.at()
	c := l.peekByte()

	switch {
	case c == '$':
		return l.dollar(p)
	case c == '"' || c == '\'' || c == '`':
		return l.quoted(p, c)
	case c >= '0' && c <= '9':
		return l.number(p)
	case isIdentStart(c):
		return l.ident(p, false)
	}
	return l.punct(p)
}

// lineComment consumes ';' to end of line. NASM has no block comment in the
// mainline grammar, only the preprocessor's, which this package does not
// implement.
func (l *lexer) lineComment() {
	own := l.onlyWhitespaceBefore()
	l.advance() // ';'
	start := l.pos
	for l.pos < len(l.src) && l.peekByte() != '\n' {
		l.advance()
	}
	body := strings.TrimRight(string(l.src[start:l.pos]), " \t\r")

	if own {
		l.before = append(l.before, body)
		if l.pos < len(l.src) {
			l.advance()
		}
		return
	}
	l.comment, l.hasCmt = body, true
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

// dollar scans '$', '$$', or a $-escaped identifier. '$' alone is the
// assembly position of the current line, '$$' the start of the section, and
// '$' immediately before an identifier character forces that word to be read
// as a plain name rather than a register or reserved word.
func (l *lexer) dollar(p text.Pos) token {
	l.advance()
	if l.peekByte() == '$' {
		l.advance()
		return token{kind: tPunct, pos: p, str: "$$"}
	}
	if isIdentStart(l.peekByte()) {
		return l.ident(p, true)
	}
	return token{kind: tPunct, pos: p, str: "$"}
}

// ident scans a symbol, mnemonic, or directive word.
func (l *lexer) ident(p text.Pos, forced bool) token {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peekByte()) {
		l.advance()
	}
	return token{kind: tIdent, pos: p, str: string(l.src[start:l.pos]), forced: forced}
}

// number scans a numeric literal: a C-style radix prefix (0x 0h 0o 0q 0b 0y
// 0d 0t) or, failing that, a maximal alphanumeric run whose trailing letter
// names the radix (h/x d/t q/o b/y), defaulting to decimal. Underscores break
// up digit groups and are skipped.
func (l *lexer) number(p text.Pos) token {
	if l.peekByte() == '0' && isIdentPart(l.at1()) {
		switch lower(l.at1()) {
		case 'x', 'h':
			l.advance()
			l.advance()
			return l.digits(p, 16)
		case 'o', 'q':
			l.advance()
			l.advance()
			return l.digits(p, 8)
		case 'b', 'y':
			l.advance()
			l.advance()
			return l.digits(p, 2)
		case 'd', 't':
			l.advance()
			l.advance()
			return l.digits(p, 10)
		}
	}
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peekByte()) {
		l.advance()
	}
	return l.suffixed(p, string(l.src[start:l.pos]))
}

func (l *lexer) digits(p text.Pos, base int) token {
	var v int64
	for l.pos < len(l.src) {
		c := l.peekByte()
		if c == '_' {
			l.advance()
			continue
		}
		d, ok := digitVal(c, base)
		if !ok {
			break
		}
		v = v*int64(base) + int64(d)
		l.advance()
	}
	return token{kind: tNumber, pos: p, num: v}
}

func (l *lexer) suffixed(p text.Pos, word string) token {
	body, base := word, 10
	if n := len(body); n > 1 {
		switch lower(body[n-1]) {
		case 'h', 'x':
			base, body = 16, body[:n-1]
		case 'o', 'q':
			base, body = 8, body[:n-1]
		case 'b', 'y':
			base, body = 2, body[:n-1]
		case 'd', 't':
			base, body = 10, body[:n-1]
		}
	}
	var v int64
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '_' {
			continue
		}
		d, ok := digitVal(c, base)
		if !ok {
			l.errorf(p, "%q is not a number", word)
			return token{kind: tNumber, pos: p}
		}
		v = v*int64(base) + int64(d)
	}
	return token{kind: tNumber, pos: p, num: v}
}

// quoted scans a single-, double-, or back-quoted string. Only the backquote
// form interprets escapes; the other two are verbatim, which is what lets
// each surround a literal instance of the other's delimiter.
func (l *lexer) quoted(p text.Pos, q byte) token {
	l.advance()
	var b strings.Builder
	for {
		if l.pos >= len(l.src) || l.peekByte() == '\n' {
			l.errorf(p, "unterminated string")
			break
		}
		c := l.advance()
		if c == q {
			break
		}
		if q == '`' && c == '\\' {
			b.WriteByte(l.backquoteEscape(p))
			continue
		}
		b.WriteByte(c)
	}
	return token{kind: tString, pos: p, str: b.String()}
}

func (l *lexer) backquoteEscape(p text.Pos) byte {
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
	case 'e':
		return 27
	case '\\', '\'', '"', '`', '?':
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

// punct scans an operator or delimiter. Three-character operators are tried
// first, so <=> is a three-way compare and not < followed by =>.
func (l *lexer) punct(p text.Pos) token {
	if l.pos+2 < len(l.src) {
		switch string(l.src[l.pos : l.pos+3]) {
		case "<=>", "<<<", ">>>":
			s := string(l.src[l.pos : l.pos+3])
			l.advance()
			l.advance()
			l.advance()
			return token{kind: tPunct, pos: p, str: s}
		}
	}
	if l.pos+1 < len(l.src) {
		switch string(l.src[l.pos : l.pos+2]) {
		case "==", "!=", "<>", "<=", ">=", "&&", "||", "^^", "<<", ">>", "//", "%%":
			s := string(l.src[l.pos : l.pos+2])
			l.advance()
			l.advance()
			return token{kind: tPunct, pos: p, str: s}
		}
	}
	c := l.advance()
	return token{kind: tPunct, pos: p, str: string(c)}
}

// Identifier characters, per the manual's own list: letters, digits, and
// _ $ # @ ~ . ? — but only letters, '.', '_' and '?' may start one.
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '.' || c == '?'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') ||
		c == '$' || c == '#' || c == '@' || c == '~'
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