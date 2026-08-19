// x86_64/text/nasm/lex.go
//
// Package nasm is NASM syntax: Intel operand order, size as a keyword on the
// operand it qualifies, `wrt` modifiers, and NASM's own C-like expression
// precedence.
//
// It parses to and prints from text.Unit and imports the root package never.
// What it knows about the architecture it asks isa/, reg/ and operand/
// directly.
//
// The comment character is ';' and the preprocessor sigil is '%'. Neither is
// gas's, which is the smaller half of why this is not shared with text/gas.
// The larger half is that the two disagree about operand order, about where
// an operand size is written, about what '/' means, and about whether a
// bare symbol in a load is an address or a location.
package nasm

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/vertex-language/arc/x86_64/text"
)

type tokKind uint8

const (
	tEOF     tokKind = iota
	tNewline         // a statement separator: a newline
	tIdent
	tNum
	tString
	tPunct
	tDollar  // $, the location counter
	tHere    // $$, the start of the current section
	tPreproc // %define, %macro, %if — recognized only to be refused
	tComment
)

type token struct {
	kind tokKind
	text string
	num  int64
	base int

	// quote is the delimiter a string was written with. NASM has three and
	// they are not interchangeable: '' and "" are raw and `` takes C
	// escapes, so a formatter that reprinted one as another would change
	// what the bytes are.
	quote byte

	// bol marks the first token of a line. NASM decides label-ness by
	// position — an identifier that starts a line and is not a mnemonic is
	// a label, with or without a colon — so the lexer has to say where a
	// token sits and not only what it is.
	bol bool

	pos text.Pos
}

// lexer scans one file.
type lexer struct {
	src  []byte
	file string

	pos  int
	line int
	col  int

	// lineStart is true until a token has been returned on this line.
	lineStart bool
}

func newLexer(file string, src []byte) *lexer {
	return &lexer{src: src, file: file, line: 1, col: 1, lineStart: true}
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
	} else {
		l.col++
	}
	return c
}

// emit stamps a token with the line position it was scanned at and closes
// the line to further label interpretation.
func (l *lexer) emit(t token) token {
	t.bol = l.lineStart
	l.lineStart = false
	return t
}

// next scans one token.
func (l *lexer) next() (token, error) {
	for {
		l.skipSpace()
		if l.pos >= len(l.src) {
			return l.emit(token{kind: tEOF, pos: l.at()}), nil
		}

		p := l.at()
		c := l.peek()

		switch {
		case c == '\n':
			l.advance()
			t := token{kind: tNewline, text: "\n", pos: p, bol: l.lineStart}
			l.lineStart = true
			return t, nil

		case c == ';':
			return l.emit(l.lineComment(p)), nil

		case c == '%' && l.lineStart && isPreprocStart(l.peekAt(1)):
			// The preprocessor is a language: %if decides which half of a
			// file exists and %macro expands one line into many. Both need
			// an evaluator and an expander this tree deliberately has not
			// got, so they are scanned only well enough to be refused by
			// name rather than to fail on a token.
			return l.emit(l.preproc(p)), nil

		case c == '"' || c == '\'' || c == '`':
			t, err := l.str(p)
			if err != nil {
				return token{}, err
			}
			return l.emit(t), nil

		case c == '$' && l.peekAt(1) == '$':
			l.advance()
			l.advance()
			return l.emit(token{kind: tHere, text: "$$", pos: p}), nil

		case c == '$' && isIdentStart(l.peekAt(1)):
			// A leading '$' escapes a name that would otherwise be a
			// keyword: $eax is the symbol eax. The sigil is a spelling and
			// does not survive into the tree.
			l.advance()
			return l.emit(l.ident(p)), nil

		case c == '$' && isDigit(l.peekAt(1)):
			l.advance()
			t, err := l.number(p, true)
			if err != nil {
				return token{}, err
			}
			return l.emit(t), nil

		case c == '$':
			return l.emit(token{kind: tDollar, text: "$", pos: p}), l.skipOne()

		case isDigit(c):
			t, err := l.number(p, false)
			if err != nil {
				return token{}, err
			}
			return l.emit(t), nil

		case isIdentStart(c):
			return l.emit(l.ident(p)), nil
		}

		t, err := l.punct(p)
		if err != nil {
			return token{}, err
		}
		return l.emit(t), nil
	}
}

func (l *lexer) skipOne() error {
	l.advance()
	return nil
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == '\f' {
			l.advance()
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

// preproc scans a preprocessor line whole. Nothing reads it but the error
// that names it.
func (l *lexer) preproc(p text.Pos) token {
	start := l.pos
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
	return token{kind: tPreproc, text: string(l.src[start:l.pos]), pos: p}
}

func isPreprocStart(c byte) bool {
	return c == '%' || c == '_' || unicode.IsLetter(rune(c))
}

// str scans a string.
//
// NASM has three quote characters and only one of them takes escapes: '\n'
// and "\n" are two characters each, and only `\n` is a newline. Recording
// which quote was written is what lets the formatter print the string back
// as the same bytes rather than as the same characters.
func (l *lexer) str(p text.Pos) (token, error) {
	q := l.advance()
	var b strings.Builder
	for {
		if l.pos >= len(l.src) || l.peek() == '\n' {
			return token{}, text.Errorf(p, "unterminated string")
		}
		c := l.advance()
		if c == q {
			return token{kind: tString, text: b.String(), quote: q, pos: p}, nil
		}
		if q == '`' && c == '\\' {
			r, err := l.escape(p)
			if err != nil {
				return token{}, err
			}
			b.WriteByte(r)
			continue
		}
		b.WriteByte(c)
	}
}

// escape handles the backquoted form's escapes, which are C's plus octal,
// \x and \e.
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
	case 'e':
		return 27, nil
	case '0':
		if l.pos < len(l.src) && !isOctal(l.peek()) {
			return 0, nil
		}
	case '\\', '"', '\'', '`':
		return c, nil
	case 'x':
		v, n := 0, 0
		for l.pos < len(l.src) && isHex(l.peek()) && n < 2 {
			v = v*16 + hexVal(l.advance())
			n++
		}
		if n == 0 {
			return 0, text.Errorf(p, `\x needs at least one hex digit`)
		}
		return byte(v), nil
	}
	if isOctal(c) {
		v := int(c - '0')
		for i := 0; i < 2 && l.pos < len(l.src) && isOctal(l.peek()); i++ {
			v = v*8 + int(l.advance()-'0')
		}
		return byte(v), nil
	}
	return 0, text.Errorf(p, `unknown escape \%c`, c)
}

// number scans an integer in any of NASM's spellings.
//
// NASM writes the base as a prefix or as a suffix and accepts both for all
// four bases: 0x1f, $1f and 1fh are one number; 0b1010, 0y1010 and 1010b are
// another. The suffix forms are why a number cannot be scanned as "a run of
// digits" — 1fh begins with a digit and ends with two letters, and 0b1010 is
// not a zero followed by the symbol b1010.
//
// The base is kept so a formatter can print 0x10 back as 0x10 rather than as
// 16. The suffix spellings do not survive: they resolve here, at the
// boundary, and the canonical prefix form is what comes back out — which is
// a spelling change arc fmt makes on purpose, the same way an alias resolves
// and vanishes everywhere else in this tree.
func (l *lexer) number(p text.Pos, dollar bool) (token, error) {
	start := l.pos
	for l.pos < len(l.src) && (isAlnum(l.peek()) || l.peek() == '_') {
		l.advance()
	}
	lit := string(l.src[start:l.pos])

	digits, base, ok := classify(lit, dollar)
	if !ok {
		return token{}, text.Errorf(p, "malformed number %q", lit)
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

func classify(lit string, dollar bool) (string, int, bool) {
	s := strings.ReplaceAll(lit, "_", "")
	if s == "" {
		return "", 0, false
	}
	if dollar {
		return s, 16, valid(s, 16)
	}

	if len(s) > 2 && s[0] == '0' {
		rest := s[2:]
		switch lowerByte(s[1]) {
		case 'x', 'h':
			if valid(rest, 16) {
				return rest, 16, true
			}
		case 'b', 'y':
			if valid(rest, 2) {
				return rest, 2, true
			}
		case 'o', 'q':
			if valid(rest, 8) {
				return rest, 8, true
			}
		case 'd', 't':
			if valid(rest, 10) {
				return rest, 10, true
			}
		}
	}

	if len(s) > 1 {
		body := s[:len(s)-1]
		switch lowerByte(s[len(s)-1]) {
		case 'h', 'x':
			if valid(body, 16) {
				return body, 16, true
			}
		case 'b', 'y':
			if valid(body, 2) {
				return body, 2, true
			}
		case 'o', 'q':
			if valid(body, 8) {
				return body, 8, true
			}
		case 'd', 't':
			if valid(body, 10) {
				return body, 10, true
			}
		}
	}

	return s, 10, valid(s, 10)
}

func valid(s string, base int) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if digitVal(s[i]) >= base {
			return false
		}
	}
	return true
}

func digitVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 99
}

func (l *lexer) ident(p text.Pos) token {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peek()) {
		l.advance()
	}
	return token{kind: tIdent, text: string(l.src[start:l.pos]), pos: p}
}

// punct scans an operator, longest match first: '<', '<=', '<<', '<<<' and
// '<=>' all start the same way and mean five different things.
var puncts = []string{
	"<=>", "<<<", ">>>",
	"<<", ">>", "<=", ">=", "==", "!=", "<>", "&&", "||", "^^", "//", "%%",
	"+", "-", "*", "/", "%", "|", "&", "^", "~", "!", "?",
	"(", ")", "[", "]", "{", "}", ",", ":", "=", "<", ">",
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
func isOctal(c byte) bool { return c >= '0' && c <= '7' }

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isAlnum(c byte) bool {
	return isDigit(c) || unicode.IsLetter(rune(c))
}

func hexVal(c byte) int { return digitVal(c) }

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// Symbol names may hold letters, digits, '_', '.', '?', '@', '$', '#' and
// '~'. The leading dot is NASM's local-label convention rather than a
// directive marker — which is the opposite of gas, where a statement
// beginning with '.' is a directive and a name beginning with '.L' is local.
func isIdentStart(c byte) bool {
	return c == '_' || c == '.' || c == '?' || c == '@' || unicode.IsLetter(rune(c))
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '$' || c == '#' || c == '~'
}