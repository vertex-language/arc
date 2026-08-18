// x86_64/text/pos.go
package text

import "fmt"

// Pos is a position in a source file: the file it came from, and a line and
// column within it.
//
// Every node carries one. A diagnostic without a position is a diagnostic
// the reader has to go looking for, and the whole point of gating a feature
// at encode time is that the message can name the line.
type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	if p.Line == 0 {
		return p.File
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// IsValid reports whether the position names a line.
func (p Pos) IsValid() bool { return p.Line > 0 }

// Error is a diagnostic with a position. Both dialects' parsers produce
// these, and so does everything in this package that can fail, so a caller
// formats one message shape rather than three.
type Error struct {
	Pos Pos
	Msg string
	Err error
}

func (e *Error) Error() string {
	if e.Pos.IsValid() {
		return e.Pos.String() + ": " + e.msg()
	}
	return e.msg()
}

func (e *Error) msg() string {
	if e.Err != nil && e.Msg == "" {
		return e.Err.Error()
	}
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

// Errorf builds a positioned diagnostic.
func Errorf(p Pos, format string, args ...any) *Error {
	return &Error{Pos: p, Msg: fmt.Sprintf(format, args...)}
}

// Wrap attaches a position to an error that has none. An error from isa/ or
// encode/ knows what went wrong and not where; this is where the two halves
// meet.
func Wrap(p Pos, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if ok := asError(err, &e); ok && e.Pos.IsValid() {
		return err
	}
	return &Error{Pos: p, Err: err}
}

func asError(err error, target **Error) bool {
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	return false
}