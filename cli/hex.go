// cli/hex.go
//
// The two conversions dis, explain and enc share.
package cli

import (
	"encoding/hex"
	"fmt"
)

// decodeHex reads bytes written the way every tool that prints them writes
// them: run together, space-separated, comma-separated, 0x-prefixed, or any
// mixture, because a caller pasting from objdump and one pasting from a Go
// literal are both right.
//
// A character that is none of those is named rather than skipped. Silently
// dropping it would decode a typo into a shorter instruction that assembles
// fine and is not the one that was asked about.
func decodeHex(s string) ([]byte, error) {
	digits := make([]byte, 0, len(s))

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' || c == '_':
		case (c == 'x' || c == 'X') && len(digits) > 0 && digits[len(digits)-1] == '0':
			// A 0x prefix, which means the 0 already appended was not a
			// digit after all.
			digits = digits[:len(digits)-1]
		case isHexDigit(c):
			digits = append(digits, c)
		default:
			return nil, fmt.Errorf("%q is not a hex digit", string(rune(c)))
		}
	}

	if len(digits) == 0 {
		return nil, fmt.Errorf("no bytes given")
	}
	if len(digits)%2 != 0 {
		return nil, fmt.Errorf("%d hex digits is half a byte short of %d",
			len(digits), len(digits)/2+1)
	}
	return hex.DecodeString(string(digits))
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// hexBytes is one instruction's encoding, space-separated: the spelling
// `arc dis` takes back.
func hexBytes(b []byte) string {
	out := make([]byte, 0, 3*len(b))
	const digits = "0123456789abcdef"
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}