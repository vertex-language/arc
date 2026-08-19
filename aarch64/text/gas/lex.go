// Package gas parses and prints A64 assembly as GNU as and the LLVM
// integrated assembler accept it.
//
// There is one syntax on this architecture and this is it. NASM has no A64
// grammar to accept and inventing one would be inventing syntax, so --dialect
// on an aarch64 target is a usage error rather than a no-op.
//
// What this package owns is spelling: which characters begin a comment, how an
// expression's operators bind, how a register with an arrangement is written.
// What a directive means, what an expression reduces to, and what an operand
// lowers to are text/'s, because they are the architecture's answers and a
// printer that re-derived them from source could not promise that formatting
// changes nothing.
package gas

import (
	"strings"

	"github.com/vertex-language/arc/aarch64/text"
)

// Kind is a token's class.
type Kind uint8

const (
	EOF Kind = iota

	// EOL is a statement separator: a newline or a semicolon. The two are the
	// same token because the architecture's syntax says they are —
	// `mov x0, #1 ; ret` is two statements — and a parser that distinguished
	// them would have to say why.
	EOL

	Ident   // _start, .text, x0, v0.4s, b.eq
	Number  // 93, 0x5d, 0b1011
	String  // "hello\n"
	Local   // 1f, 2b — a numeric label reference
	Punct   // , [ ] { } ( ) : @ # ! and the operators
	Comment // // … or /* … */ or a full-line #
)

func (k Kind) String() string {
	switch k {
	case EOL:
		return "end of statement"
	case Ident:
		return "identifier"
	case Number:
		return "number"
	case String:
		return "string"
	case Local:
		return "local label reference"
	case Punct:
		return "punctuation"
	case Comment:
		return "comment"
	}
	return "end of file"
}

// Token is one lexeme.
type Token struct {
	Kind Kind
	Text string
	Num  int64
	Pos  text.Pos

	// Forward distinguishes 1f from 1b, for Local.
	Forward bool

	// Spaced reports whether whitespace preceded this token. The parser needs
	// it in exactly one place: `.` is the location counter, and `x0.4s` is a
	// register with an arrangement, so whether a dot is attached to what
	// precedes it changes what it means.
	Spaced bool
}

// Lexer produces tokens from source.
type Lexer struct {
	src  string
	file string
	off  int
	line int
	col  int

	// tok is the lookahead, filled by Next and read by Peek.
	tok   Token
	full  bool
	errs  []error
}

// NewLexer starts a lexer over src.
func NewLexer(file, src string) *Lexer {
	return &Lexer{src: src, file: file, line: 1, col: 1}
}

func (l *Lexer) pos() text.Pos {
	return text.Pos{File: l.file, Line: l.line, Col: l.col, Offset: l.off}
}

func (l *Lexer) peekByte(n int) byte {
	if l.off+n >= len(l.src) {
		return 0
	}
	return l.src[l.off+n]
}

func (l *Lexer) advance(n int) {
	for i := 0; i < n && l.off < len(l.src); i++ {
		if l.src[l.off] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.off++
	}
}

// Peek returns the next token without consuming it.
func (l *Lexer) Peek() Token {
	if !l.full {
		l.tok = l.scan()
		l.full = true
	}
	return l.tok
}

// Next consumes and returns the next token.
func (l *Lexer) Next() Token {
	t := l.Peek()
	l.full = false
	return t
}

// atLineStart reports whether only whitespace precedes the current offset on
// this line. It is what makes a leading '#' a comment and an interior one an
// immediate marker — the one place in this grammar where a character's meaning
// depends on its column.
func (l *Lexer) atLineStart() bool {
	for i := l.off - 1; i >= 0; i-- {
		switch l.src[i] {
		case '\n':
			return true
		case ' ', '\t', '\r':
			continue
		default:
			return false
		}
	}
	return true
}

func (l *Lexer) scan() Token {
	spaced := l.skipSpace()
	p := l.pos()

	if l.off >= len(l.src) {
		return Token{Kind: EOF, Pos: p, Spaced: spaced}
	}

	c := l.src[l.off]

	switch {
	case c == '\n' || c == ';':
		l.advance(1)
		return Token{Kind: EOL, Text: string(c), Pos: p, Spaced: spaced}

	case c == '/' && l.peekByte(1) == '/':
		return l.lineComment(p, spaced)

	case c == '/' && l.peekByte(1) == '*':
		return l.blockComment(p, spaced)

	case c == '#' && l.atLineStart():
		return l.lineComment(p, spaced)

	case c == '"':
		return l.string(p, spaced)

	case c == '\'':
		return l.char(p, spaced)

	case isDigit(c):
		return l.number(p, spaced)

	case isIdentStart(c):
		return l.ident(p, spaced)
	}

	// Multi-character operators first, longest match.
	for _, op := range []string{"<<", ">>", "&&", "||"} {
		if strings.HasPrefix(l.src[l.off:], op) {
			l.advance(len(op))
			return Token{Kind: Punct, Text: op, Pos: p, Spaced: spaced}
		}
	}

	l.advance(1)
	return Token{Kind: Punct, Text: string(c), Pos: p, Spaced: spaced}
}

// skipSpace consumes horizontal whitespace and line continuations, reporting
// whether any was found. A newline is not whitespace here: it is a statement
// separator and a token of its own.
func (l *Lexer) skipSpace() bool {
	found := false
	for l.off < len(l.src) {
		switch l.src[l.off] {
		case ' ', '\t', '\r':
			l.advance(1)
			found = true
		case '\\':
			if l.peekByte(1) == '\n' {
				l.advance(2)
				found = true
				continue
			}
			return found
		default:
			return found
		}
	}
	return found
}

func (l *Lexer) lineComment(p text.Pos, spaced bool) Token {
	start := l.off
	for l.off < len(l.src) && l.src[l.off] != '\n' {
		l.advance(1)
	}
	return Token{Kind: Comment, Text: l.src[start:l.off], Pos: p, Spaced: spaced}
}

func (l *Lexer) blockComment(p text.Pos, spaced bool) Token {
	start := l.off
	l.advance(2)
	for l.off < len(l.src) {
		if l.src[l.off] == '*' && l.peekByte(1) == '/' {
			l.advance(2)
			return Token{Kind: Comment, Text: l.src[start:l.off], Pos: p, Spaced: spaced}
		}
		l.advance(1)
	}
	l.errorf(p, "unterminated block comment")
	return Token{Kind: Comment, Text: l.src[start:l.off], Pos: p, Spaced: spaced}
}

// ident scans an identifier.
//
// The dot is an identifier character rather than punctuation, which is what
// makes `.text`, `b.eq` and `v0.4s` single tokens. Splitting them is the
// parser's job, because what a dot separates differs by position: a directive
// from nothing, a mnemonic from a condition, a register from an arrangement.
// A bare `.` is the location counter and falls out of the same rule.
func (l *Lexer) ident(p text.Pos, spaced bool) Token {
	start := l.off
	for l.off < len(l.src) && isIdentChar(l.src[l.off]) {
		l.advance(1)
	}
	return Token{Kind: Ident, Text: l.src[start:l.off], Pos: p, Spaced: spaced}
}

// number scans an integer, or a local label reference.
//
// A digit followed immediately by 'f' or 'b' is 1f or 1b — a reference to the
// nearest numeric label forward or backward. It has to be caught here because
// `1b` is otherwise a perfectly good hex-looking token and `2f` a perfectly
// good identifier suffix.
func (l *Lexer) number(p text.Pos, spaced bool) Token {
	start := l.off

	// Local label reference: a single digit, then f or b, then a non-identifier.
	if l.off+1 < len(l.src) && isDigit(l.src[l.off]) {
		if d := l.peekByte(1); d == 'f' || d == 'b' {
			if !isIdentChar(l.peekByte(2)) {
				digit := l.src[l.off]
				l.advance(2)
				return Token{
					Kind: Local, Text: string(digit),
					Forward: d == 'f', Pos: p, Spaced: spaced,
				}
			}
		}
	}

	base := 10
	if l.src[l.off] == '0' && l.off+1 < len(l.src) {
		switch l.peekByte(1) {
		case 'x', 'X':
			base = 16
			l.advance(2)
		case 'b', 'B':
			// 0b is binary unless what follows is not a binary digit, in which
			// case it was the digit zero and this is not our token.
			if d := l.peekByte(2); d == '0' || d == '1' {
				base = 2
				l.advance(2)
			}
		default:
			if isOctalDigit(l.peekByte(1)) {
				base = 8
				l.advance(1)
			}
		}
	}

	digits := l.off
	var v int64
	for l.off < len(l.src) {
		d, ok := digitVal(l.src[l.off], base)
		if !ok {
			break
		}
		v = v*int64(base) + int64(d)
		l.advance(1)
	}
	if l.off == digits && base != 10 {
		l.errorf(p, "malformed number")
	}
	return Token{Kind: Number, Text: l.src[start:l.off], Num: v, Pos: p, Spaced: spaced}
}

func (l *Lexer) string(p text.Pos, spaced bool) Token {
	l.advance(1)
	var b strings.Builder
	for l.off < len(l.src) && l.src[l.off] != '"' {
		if l.src[l.off] == '\\' {
			l.advance(1)
			if l.off >= len(l.src) {
				break
			}
			b.WriteByte(unescape(l.src[l.off]))
			l.advance(1)
			continue
		}
		if l.src[l.off] == '\n' {
			l.errorf(p, "unterminated string")
			break
		}
		b.WriteByte(l.src[l.off])
		l.advance(1)
	}
	if l.off < len(l.src) {
		l.advance(1)
	} else {
		l.errorf(p, "unterminated string")
	}
	return Token{Kind: String, Text: b.String(), Pos: p, Spaced: spaced}
}

// char scans a character literal. gas accepts both 'a' and 'a with no closing
// quote, and code in the wild uses the second, so the closing quote is
// optional rather than required.
func (l *Lexer) char(p text.Pos, spaced bool) Token {
	l.advance(1)
	if l.off >= len(l.src) {
		l.errorf(p, "empty character literal")
		return Token{Kind: Number, Pos: p, Spaced: spaced}
	}
	c := l.src[l.off]
	l.advance(1)
	if c == '\\' && l.off < len(l.src) {
		c = unescape(l.src[l.off])
		l.advance(1)
	}
	if l.off < len(l.src) && l.src[l.off] == '\'' {
		l.advance(1)
	}
	return Token{Kind: Number, Num: int64(c), Text: string(c), Pos: p, Spaced: spaced}
}

func (l *Lexer) errorf(p text.Pos, format string, args ...any) {
	l.errs = append(l.errs, &Error{Pos: p, Msg: sprintf(format, args...)})
}

// Errors reports lexical errors found so far.
func (l *Lexer) Errors() []error { return l.errs }

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isOctalDigit(c byte) bool { return c >= '0' && c <= '7' }

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '.' || c == '$'
}

func isIdentChar(c byte) bool { return isIdentStart(c) || isDigit(c) }

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

func unescape(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case '0':
		return 0
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'v':
		return '\v'
	}
	return c
}