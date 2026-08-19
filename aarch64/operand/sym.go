package operand

import "strconv"

// RelocKind names a relocation. It is an arc identifier rather than a format's
// number: R_AARCH64_ABS64 is 257, IMAGE_REL_ARM64_ADDR64 is 1, and
// ARM64_RELOC_UNSIGNED is 0, so a value that meant the format's number would
// collide across the three formats a single object tree has to spell.
//
// The constants and the kind-to-format table live in the arch package's
// reloc.go and reloc_*.go, which is also where a kind gets a name. This package
// declares the type and nothing else, because a Target has to be able to carry
// one and neither Label nor SymRef should have to know which formats exist.
type RelocKind uint16

// RelocNone is the zero value: the caller named no kind and left the choice of
// one to the platform writer, which picks from what the field is for.
const RelocNone RelocKind = 0

// Named reports whether the caller stated a kind.
func (k RelocKind) Named() bool { return k != RelocNone }

// Target is what an address operand points at before it is a number.
type Target interface {
	isTarget()
	String() string
}

// Label is a name defined somewhere in this object.
//
// A Label carries no relocation kind, which is what makes it foldable: a
// pc-relative reference to a local label in the same section resolves at
// Serialize and leaves no record behind. Ref is the spelling for everything
// else.
type Label string

func (Label) isTarget()       {}
func (l Label) String() string { return string(l) }

// SymRef is a reference to a symbol, with an addend and optionally the
// relocation kind the caller insists on.
//
// Stating a kind is a request, not a hint. Writing Ref("puts", R_AARCH64_CALL26)
// asks for a branch relocation, and folding it into a direct branch — even to a
// symbol two lines above — would answer a different question than the one
// asked.
type SymRef struct {
	Name   string
	Addend int64
	Kind   RelocKind
}

func (SymRef) isTarget() {}

func (s SymRef) String() string {
	out := s.Name
	if s.Addend > 0 {
		out += "+" + strconv.FormatInt(s.Addend, 10)
	} else if s.Addend < 0 {
		out += strconv.FormatInt(s.Addend, 10)
	}
	return out
}

// Sym builds a symbol reference. The arch package re-exports it as Ref, which
// is the spelling the README and the builder examples use.
func Sym(name string, kind ...RelocKind) SymRef {
	s := SymRef{Name: name}
	if len(kind) > 0 {
		s.Kind = kind[0]
	}
	return s
}

// Plus is a reference with an addend: Sym("puts").Plus(8) is bl puts+8.
func (s SymRef) Plus(addend int64) SymRef { s.Addend += addend; return s }

// AddrRole is which part of an address a reference names.
//
// Materializing an address on this architecture usually takes two instructions
// and therefore two references — adrp for the page, add or a load for the
// offset within it — and each needs its own record. The role is the portable
// part: GNU as spells the pair :pg_hi21: and :lo12:, the Darwin assembler
// spells it @PAGE and @PAGEOFF, and the kind is per format. The caller states
// the role and never the kind.
type AddrRole uint8

const (
	// RoleDirect is the address itself: a branch target, or a literal load.
	RoleDirect AddrRole = iota

	RolePage       // :pg_hi21: / @PAGE
	RolePageOff    // :lo12:    / @PAGEOFF
	RoleGotPage    // :got:     / @GOTPAGE
	RoleGotPageOff // :got_lo12:/ @GOTPAGEOFF
)

func (r AddrRole) String() string {
	switch r {
	case RolePage:
		return "page"
	case RolePageOff:
		return "pageoff"
	case RoleGotPage:
		return "gotpage"
	case RoleGotPageOff:
		return "gotpageoff"
	}
	return "direct"
}

// GOT reports whether the role goes through the global offset table, which is
// what makes it a reference to a slot holding the address rather than to the
// address.
func (r AddrRole) GOT() bool { return r == RoleGotPage || r == RoleGotPageOff }

// AddrRef is a target together with the role naming which half of its address
// this operand wants.
type AddrRef struct {
	T    Target
	Role AddrRole
}

// Page, PageOff, GotPage and GotPageOff are the four roles.
func Page(t Target) AddrRef       { return AddrRef{T: t, Role: RolePage} }
func PageOff(t Target) AddrRef    { return AddrRef{T: t, Role: RolePageOff} }
func GotPage(t Target) AddrRef    { return AddrRef{T: t, Role: RoleGotPage} }
func GotPageOff(t Target) AddrRef { return AddrRef{T: t, Role: RoleGotPageOff} }

// Direct wraps a bare target, so a caller lowering operands has one type to
// handle rather than two.
func Direct(t Target) AddrRef { return AddrRef{T: t, Role: RoleDirect} }

// Kind is the relocation kind the caller named on the underlying symbol, or
// RelocNone.
func (a AddrRef) Kind() RelocKind {
	if s, ok := a.T.(SymRef); ok {
		return s.Kind
	}
	return RelocNone
}

// Addend is the offset from the symbol the reference means.
func (a AddrRef) Addend() int64 {
	if s, ok := a.T.(SymRef); ok {
		return s.Addend
	}
	return 0
}

func (a AddrRef) String() string {
	if a.T == nil {
		return "<nil>"
	}
	switch a.Role {
	case RolePage:
		return ":pg_hi21:" + a.T.String()
	case RolePageOff:
		return ":lo12:" + a.T.String()
	case RoleGotPage:
		return ":got:" + a.T.String()
	case RoleGotPageOff:
		return ":got_lo12:" + a.T.String()
	}
	return a.T.String()
}