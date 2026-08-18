package text

import "fmt"

// Modifier is a relocation modifier written on a symbol: `puts@PLT` in GAS,
// `puts wrt ..plt` in NASM.
//
// It is a spelling and never a number. The psABI constants — R_386_PLT32,
// R_386_GOTOFF — are declared in the arch root, which holds the platform
// validity table and knows that a COFF target has no GOT at all. text/ cannot
// import the root, and should not want to: the same source line must be able
// to produce an ELF relocation on i386-elf and a diagnostic on i386-coff, and
// only a package that knows the platform can decide that.
//
// So the parser records which modifier was written and the assembler resolves
// modifier plus platform plus the field it landed in to a relocation kind.
// This is the same one-boundary rule the alias table follows: past the
// resolver only canonical values exist, and there is no in-memory
// representation of "@PLT" below it.
//
// The names are the psABI's own vocabulary and both dialects use them; only
// the sigil differs. That is why this is an enum and not a string: the set is
// closed by the specification, and the root can switch on it exhaustively.
type Modifier uint8

const (
	ModNone Modifier = iota

	// The four that matter for ordinary code. i386 has no PC-relative
	// addressing mode, which is why GOTOFF and GOTPC exist here and have no
	// x86-64 counterpart: PIC materialises the GOT pointer in a register and
	// addresses everything as an offset from it.
	ModPLT    // R_386_PLT32
	ModGOT    // R_386_GOT32
	ModGOTOFF // R_386_GOTOFF
	ModGOTPC  // R_386_GOTPC

	// Thread-local storage. Declared because the psABI declares them and a
	// parser that rejected the spelling would be rejecting a program GNU as
	// assembles; whether the target supports them is the root's answer.
	ModTLSGD
	ModTLSLDM
	ModDTPOFF
	ModGOTTPOFF
	ModTPOFF
	ModINDNTPOFF
	ModNTPOFF
	ModGOTNTPOFF

	numModifiers
)

// names are the bare spellings, uppercase, without the sigil. GAS writes
// name@PLT and NASM writes name wrt ..plt; the word between is the same in
// both, which is what makes this table shared rather than per-dialect.
var modifierNames = [numModifiers]string{
	ModNone:      "",
	ModPLT:       "PLT",
	ModGOT:       "GOT",
	ModGOTOFF:    "GOTOFF",
	ModGOTPC:     "GOTPC",
	ModTLSGD:     "TLSGD",
	ModTLSLDM:    "TLSLDM",
	ModDTPOFF:    "DTPOFF",
	ModGOTTPOFF:  "GOTTPOFF",
	ModTPOFF:     "TPOFF",
	ModINDNTPOFF: "INDNTPOFF",
	ModNTPOFF:    "NTPOFF",
	ModGOTNTPOFF: "GOTNTPOFF",
}

func (m Modifier) String() string {
	if m < numModifiers {
		return modifierNames[m]
	}
	return "?"
}

// TLS reports whether the modifier is a thread-local one. The assembler needs
// the answer to decide whether the symbol's type must be tls_object, and
// asking it here keeps that list in the package that declares the set.
func (m Modifier) TLS() bool {
	switch m {
	case ModTLSGD, ModTLSLDM, ModDTPOFF, ModGOTTPOFF, ModTPOFF, ModINDNTPOFF,
		ModNTPOFF, ModGOTNTPOFF:
		return true
	}
	return false
}

// ParseModifier resolves a bare modifier word, case-insensitively. Each
// dialect strips its own sigil first: GAS the '@', NASM the `wrt ..`.
func ParseModifier(name string) (Modifier, bool) {
	for m := Modifier(1); m < numModifiers; m++ {
		if equalFold(name, modifierNames[m]) {
			return m, true
		}
	}
	return ModNone, false
}

// UnknownModifier is the diagnostic for a word that is not a modifier, with
// the list. It lives here because this package declares the set.
func UnknownModifier(p Pos, name string) *Error {
	return Errorf(p, "unknown relocation modifier %q", name).
		Note("i386 accepts PLT, GOT, GOTOFF, GOTPC and the TLS modifiers").
		Note("i386 has no PC-relative addressing mode; there is no GOTPCREL")
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'a' <= x && x <= 'z' {
			x -= 32
		}
		if 'a' <= y && y <= 'z' {
			y -= 32
		}
		if x != y {
			return false
		}
	}
	return true
}

var _ = fmt.Sprintf