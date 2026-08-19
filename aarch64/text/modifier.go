package text

import "github.com/vertex-language/arc/aarch64/operand"

// Modifier is which half of an address an operand names.
//
// It stores the role and never the spelling, because the spelling varies by
// platform rather than by dialect. GNU as writes `adrp x0, :pg_hi21:foo` with
// the prefix optional and `add x0, x0, #:lo12:foo`; the Darwin assembler writes
// `foo@PAGE` and `foo@PAGEOFF` and rejects `:got:` and `:got_lo12:` outright.
// Both are the same four roles, so the parser reads either into a role and the
// printer picks the spelling from the target it is printing for.
//
// Treating this as a dialect would let a caller ask for @PAGEOFF in an ELF
// object, which is not a preference — it is a file no assembler on that
// platform will read back.
type Modifier uint8

const (
	ModNone Modifier = iota

	ModPage       // :pg_hi21: / @PAGE
	ModPageOff    // :lo12:    / @PAGEOFF
	ModGotPage    // :got:     / @GOTPAGE
	ModGotPageOff // :got_lo12:/ @GOTPAGEOFF

	modCount
)

// Valid reports whether m names a role.
func (m Modifier) Valid() bool { return m > ModNone && m < modCount }

// Role is the operand-level role this modifier names, which is what a fixup
// carries and what the platform writer turns into a relocation kind.
func (m Modifier) Role() operand.AddrRole {
	switch m {
	case ModPage:
		return operand.RolePage
	case ModPageOff:
		return operand.RolePageOff
	case ModGotPage:
		return operand.RoleGotPage
	case ModGotPageOff:
		return operand.RoleGotPageOff
	}
	return operand.RoleDirect
}

// GOT reports whether the modifier goes through the global offset table.
func (m Modifier) GOT() bool { return m == ModGotPage || m == ModGotPageOff }

// String is the neutral name, for diagnostics only. A printer must not use it;
// see GAS and MachO below.
func (m Modifier) String() string {
	switch m {
	case ModPage:
		return "page"
	case ModPageOff:
		return "pageoff"
	case ModGotPage:
		return "gotpage"
	case ModGotPageOff:
		return "gotpageoff"
	}
	return ""
}

// GAS is the ELF and COFF spelling: a prefix on the symbol.
func (m Modifier) GAS() string {
	switch m {
	case ModPage:
		return ":pg_hi21:"
	case ModPageOff:
		return ":lo12:"
	case ModGotPage:
		return ":got:"
	case ModGotPageOff:
		return ":got_lo12:"
	}
	return ""
}

// MachO is the Darwin spelling: a suffix on the symbol.
//
// The GOT roles have a spelling here, unlike the reverse direction where the
// Darwin assembler has no :got: to accept. That asymmetry is the platforms',
// and printing is where it shows up rather than parsing.
func (m Modifier) MachO() string {
	switch m {
	case ModPage:
		return "@PAGE"
	case ModPageOff:
		return "@PAGEOFF"
	case ModGotPage:
		return "@GOTPAGE"
	case ModGotPageOff:
		return "@GOTPAGEOFF"
	}
	return ""
}

// LookupModifier resolves either spelling into a role, which is what lets a
// parser read a Darwin-flavoured file and a printer emit an ELF-flavoured one.
func LookupModifier(s string) (Modifier, bool) {
	switch lower(s) {
	case ":pg_hi21:", "@page":
		return ModPage, true
	case ":lo12:", "@pageoff":
		return ModPageOff, true
	case ":got:", "@gotpage":
		return ModGotPage, true
	case ":got_lo12:", "@gotpageoff":
		return ModGotPageOff, true
	}
	return ModNone, false
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}