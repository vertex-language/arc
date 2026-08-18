// x86_64/text/gas/lex.go
//
// Package gas is GNU as syntax: AT&T operand order, size as a mnemonic
// suffix, @-modifiers, and gas's own expression precedence.
//
// It parses to and prints from text.Unit and imports the root package
// never. What it knows about the architecture it asks isa/, reg/ and
// operand/ directly.
package gas

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/vertex-language/arc/x86_64/text"
)

type tokKind uint8

const (
	tEOF tokKind = iota
	tNewline // a statement separator: a newline or a semicolon
	tIdent
	tNum
	tString
	tChar
	tPunct
	tPercent // %, the register sigil
	tDollar  // $, the immediate sigil
	tStar    // *, the indirect-branch sigil
	tAt      // @, the modifier sigil
	tComment // a whole-line or trailing comment, text only
)

type token struct {
	kind tokKind
	text string
	num  int64
	base int
	pos  text.Pos
}

// lexer scans one file.
//
// The comment character is '#', which is x86's; on other targets gas uses
// something else and the same source would lex differently. That is one of
// the several reasons this directory is a copy of i386's rather than a
// shared package: "the comment character" is not a fact about gas.
type lexer struct {
	src  []byte
	file string

	pos  int
	line int
	col  int

	// atLineStart tracks whether anything but whitespace has been seen on
	// this line, because a '#' in column one after a directive is a comment
	// and a '#' anywhere is also a comment — but "#APP" and "#NO_APP" are
	// gcc's inline-asm markers and are neither.
	atLineStart bool
}

func newLexer(file string, src []byte) *lexer {
	return &lexer{src: src, file: file, line: 1, col: 1, atLineStart: true}
}

func (l *lexer) at() text.Pos {
	return text.Pos{File: l.file, Line: l.line, Col: l.col}
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peekAt(n int) byte {
	if l.pos+n >= len(l.src) {
		return 0
	}
	return l.src[l.pos+n]
}

func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
		l.atLineStart = true
	} else {
		l.col++
	}
	return c
}

// next scans one token.
func (l *lexer) next() (token, error) {
	for {
		l.skipSpace()
		if l.pos >= len(l.src) {
			return token{kind: tEOF, pos: l.at()}, nil
		}

		p := l.at()
		c := l.peek()

		switch {
		case c == '\n' || c == ';':
			// A statement ends at a newline or a semicolon. Both are
			// statement separators and neither is discarded, because a
			// formatter that turned `a; b` into two lines would be making a
			// choice this package does not get to make.
			l.advance()
			return token{kind: tNewline, text: string(c), pos: p}, nil

		case c == '#':
			// gcc emits #APP and #NO_APP around inline asm. They are not
			// comments to gas and dropping them loses nothing, but treating
			// them as comments and printing them back is what keeps a
			// compiler-generated file byte-identical through arc fmt.
			return l.lineComment(p), nil

		case c == '/' && l.peekAt(1) == '*':
			if err := l.blockComment(); err != nil {
				return token{}, err
			}
			continue

		case c == '/' && l.peekAt(1) == '/':
			// Not gas syntax — '/' is division. A doubled slash is two
			// divisions and gas says so, so this is not folded into a
			// comment here either.
			l.advance()
			return token{kind: tPunct, text: "/", pos: p}, nil

		case c == '"':
			return l.str(p)

		case c == '\'':
			return l.charConst(p)

		case c == '%':
			l.advance()
			return token{kind: tPercent, text: "%", pos: p}, nil
		case c == '$':
			l.advance()
			return token{kind: tDollar, text: "$", pos: p}, nil
		case c == '*':
			l.advance()
			return token{kind: tStar, text: "*", pos: p}, nil
		case c == '@':
			l.advance()
			return token{kind: tAt, text: "@", pos: p}, nil

		case isDigit(c):
			return l.number(p)

		case isIdentStart(c):
			return l.ident(p), nil
		}

		return l.punct(p)
	}
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == '\f' {
			l.advance()
			l.atLineStart = false
			continue
		}
		if c == '\\' && l.peekAt(1) == '\n' {
			// A backslash-newline continues a statement. The joined line is
			// one statement and prints back as one.
			l.advance()
			l.advance()
			continue
		}
		return
	}
}

func (l *lexer) lineComment(p text.Pos) token {
	start := l.pos
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
	return token{kind: tComment, text: string(l.src[start:l.pos]), pos: p}
}

func (l *lexer) blockComment() error {
	p := l.at()
	l.advance()
	l.advance()
	for l.pos < len(l.src) {
		if l.peek() == '*' && l.peekAt(1) == '/' {
			l.advance()
			l.advance()
			return nil
		}
		l.advance()
	}
	return text.Errorf(p, "unterminated /* comment")
}

func (l *lexer) str(p text.Pos) (token, error) {
	l.advance()
	var b strings.Builder
	for {
		if l.pos >= len(l.src) {
			return token{}, text.Errorf(p, "unterminated string")
		}
		c := l.advance()
		switch c {
		case '"':
			return token{kind: tString, text: b.String(), pos: p}, nil
		case '\\':
			r, err := l.escape(p)
			if err != nil {
				return token{}, err
			}
			b.WriteByte(r)
		default:
			b.WriteByte(c)
		}
	}
}

// escape handles gas's string escapes, which are C's plus octal and \x.
func (l *lexer) escape(p text.Pos) (byte, error) {
	if l.pos >= len(l.src) {
		return 0, text.Errorf(p, "unterminated escape")
	}
	c := l.advance()
	switch c {
	case 'n':
		return '\n', nil
	case 't':
		return '\t', nil
	case 'r':
		return '\r', nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'v':
		return '\v', nil
	case 'a':
		return 7, nil
	case '\\', '"', '\'':
		return c, nil
	case 'x':
		v := 0
		n := 0
		for l.pos < len(l.src) && isHex(l.peek()) {
			v = v*16 + hexVal(l.advance())
			n++
		}
		if n == 0 {
			return 0, text.Errorf(p, `\x needs at least one hex digit`)
		}
		return byte(v), nil
	}
	if c >= '0' && c <= '7' {
		v := int(c - '0')
		for i := 0; i < 2 && l.pos < len(l.src) && l.peek() >= '0' && l.peek() <= '7'; i++ {
			v = v*8 + int(l.advance()-'0')
		}
		return byte(v), nil
	}
	return 0, text.Errorf(p, `unknown escape \%c`, c)
}

func (l *lexer) charConst(p text.Pos) (token, error) {
	// gas's character constant is 'a — one quote, no closing quote. A
	// closing quote is accepted too and most sources write one, so both are
	// taken and the token records which, so it prints back the same way.
	l.advance()
	if l.pos >= len(l.src) {
		return token{}, text.Errorf(p, "unterminated character constant")
	}
	var v byte
	if l.peek() == '\\' {
		l.advance()
		c, err := l.escape(p)
		if err != nil {
			return token{}, err
		}
		v = c
	} else {
		v = l.advance()
	}
	closed := ""
	if l.pos < len(l.src) && l.peek() == '\'' {
		l.advance()
		closed = "'"
	}
	return token{kind: tChar, num: int64(v), text: closed, pos: p}, nil
}

// number scans an integer.
//
// A digit followed by 'b' or 'f' is a numeric label reference, not a number:
// `1b` is "the nearest 1: above" and `1f` is "the nearest 1: below". They
// collide with hex digits, which is why they are only recognized for a
// single leading digit with no 0x prefix.
func (l *lexer) number(p text.Pos) (token, error) {
	start := l.pos

	if l.peek() == '0' && (l.peekAt(1) == 'x' || l.peekAt(1) == 'X') {
		l.advance()
		l.advance()
		for l.pos < len(l.src) && (isHex(l.peek()) || l.peek() == '_') {
			l.advance()
		}
		return l.parseNum(p, string(l.src[start:l.pos]), 16)
	}
	if l.peek() == '0' && (l.peekAt(1) == 'b' || l.peekAt(1) == 'B') &&
		(l.peekAt(2) == '0' || l.peekAt(2) == '1') {
		l.advance()
		l.advance()
		for l.pos < len(l.src) && (l.peek() == '0' || l.peek() == '1') {
			l.advance()
		}
		return l.parseNum(p, string(l.src[start:l.pos]), 2)
	}

	for l.pos < len(l.src) && isDigit(l.peek()) {
		l.advance()
	}
	lit := string(l.src[start:l.pos])

	if len(lit) == 1 && l.pos < len(l.src) {
		if c := l.peek(); c == 'b' || c == 'f' {
			// Only if what follows is not more of an identifier: `1b` is a
			// label reference and `1before` is not anything.
			if !isIdentPart(l.peekAt(1)) {
				l.advance()
				return token{kind: tIdent, text: lit + string(c), pos: p}, nil
			}
		}
	}

	base := 10
	if len(lit) > 1 && lit[0] == '0' {
		base = 8
	}
	return l.parseNum(p, lit, base)
}

func (l *lexer) parseNum(p text.Pos, lit string, base int) (token, error) {
	digits := strings.ReplaceAll(lit, "_", "")
	switch base {
	case 16:
		digits = digits[2:]
	case 2:
		digits = digits[2:]
	case 8:
		digits = digits[1:]
		if digits == "" {
			digits = "0"
		}
	}
	// Parsed as unsigned and reinterpreted, because 0xffffffffffffffff is a
	// number people write and is not a signed overflow to anyone but a
	// parser.
	v, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		return token{}, text.Errorf(p, "malformed number %q", lit)
	}
	return token{kind: tNum, num: int64(v), base: base, text: lit, pos: p}, nil
}

func (l *lexer) ident(p text.Pos) token {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peek()) {
		l.advance()
	}
	return token{kind: tIdent, text: string(l.src[start:l.pos]), pos: p}
}

// punct scans an operator, longest match first, because '<' and '<<' and
// '<>' and '<=' all start the same way and mean four different things.
var puncts = []string{
	"<<", ">>", "<>", "<=", ">=", "==", "!=", "&&", "||",
	"+", "-", "*", "/", "%", "|", "&", "^", "!", "~",
	"(", ")", ",", ":", "<", ">", "[", "]",
}

func (l *lexer) punct(p text.Pos) (token, error) {
	rest := l.src[l.pos:]
	for _, op := range puncts {
		if len(rest) >= len(op) && string(rest[:len(op)]) == op {
			for range op {
				l.advance()
			}
			return token{kind: tPunct, text: op, pos: p}, nil
		}
	}
	c := l.advance()
	return token{}, text.Errorf(p, "unexpected character %q", string(rune(c)))
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return int(c-'A') + 10
	}
}

// Symbol names may hold letters, digits, '_', '.' and '$'. The dot is why a
// directive and a symbol are told apart by position rather than by
// spelling: `.text` is a directive at the start of a statement and a
// perfectly good symbol name in an expression.
func isIdentStart(c byte) bool {
	return c == '_' || c == '.' || unicode.IsLetter(rune(c))
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '$'
}