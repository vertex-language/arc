package gas

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/aarch64/text"
)

// Error is a parse or print failure at a position.
type Error struct {
	Pos text.Pos
	Msg string

	// Note is a second line of explanation, for the failures where naming the
	// problem is not enough to act on it.
	Note string
}

func (e *Error) Error() string {
	s := e.Pos.String() + ": " + e.Msg
	if e.Note != "" {
		s += "\n  note: " + e.Note
	}
	return s
}

// ErrorList is every failure from one parse, in source order.
//
// Parsing collects rather than stopping at the first error, because a source
// file with two typos should report two and not require two runs to find them.
// It stops at twenty, past which the later errors are usually consequences of
// the earlier ones and reporting them all buries the ones that matter.
type ErrorList []*Error

const maxErrors = 20

func (l ErrorList) Error() string {
	switch len(l) {
	case 0:
		return "no errors"
	case 1:
		return l[0].Error()
	}
	var b strings.Builder
	for i, e := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Error())
	}
	if len(l) >= maxErrors {
		b.WriteString("\n(too many errors)")
	}
	return b.String()
}

// Err returns the list as an error, or nil when it is empty.
func (l ErrorList) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}