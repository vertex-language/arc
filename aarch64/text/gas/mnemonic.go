package gas

import (
	"strings"

	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/reg"
	"github.com/vertex-language/arc/aarch64/text"
)

// Names: mnemonics, registers, and the alias table .req maintains.

// aliases is a source file's register-name vocabulary.
//
// It is per-parse state rather than a package table because .req and .unreq
// change it mid-file: a file's set of valid register names is a moving target,
// and reg.Lookup is what this starts from rather than what it consults.
//
// The four built-ins are gas's, not this package's invention. ip0, ip1, lr and
// fp are automatically defined to alias X16, X17, X30 and X29, and .unreq can
// undefine them — which is why they live in this mutable table and not in
// reg.Lookup, whose comment correctly says the architectural names are the only
// ones it answers to.
type aliases struct {
	m map[string]reg.Reg
}

func newAliases() *aliases {
	a := &aliases{m: map[string]reg.Reg{}}
	a.m["ip0"] = reg.X16
	a.m["ip1"] = reg.X17
	a.m["lr"] = reg.X30
	a.m["fp"] = reg.X29
	return a
}

// define adds an alias, as `foo .req w0` does.
func (a *aliases) define(name string, r reg.Reg) { a.m[strings.ToLower(name)] = r }

// undefine removes one. gas errors when the name was not defined, and so does
// the caller of this; the bool is how it knows.
func (a *aliases) undefine(name string) bool {
	n := strings.ToLower(name)
	if _, ok := a.m[n]; !ok {
		return false
	}
	delete(a.m, n)
	return true
}

// lookup resolves a register name: the architecture's first, then the file's.
//
// The order matters and it is this way round because an alias may not shadow an
// architectural name. gas says the built-in register names other than the four
// above cannot be undefined, and a `x0 .req x1` that took effect would make
// every subsequent line mean something other than it reads.
func (a *aliases) lookup(name string) (reg.Reg, bool) {
	if r, ok := reg.Lookup(name); ok {
		return r, true
	}
	r, ok := a.m[strings.ToLower(name)]
	return r, ok
}

// splitMnemonic separates a mnemonic from a trailing condition.
//
// `b.eq` is the B.cond form with EQ in its condition field, and the table
// spells that form's mnemonic "b.cond" — so the dot is a separator here and
// part of the name in `.text`. Nothing else in the A64 mnemonic set uses a dot,
// which is what keeps this to one case rather than a general suffix grammar.
func splitMnemonic(s string) (mnem string, cond operand.Cond, hasCond bool) {
	i := strings.IndexByte(s, '.')
	if i <= 0 || i == len(s)-1 {
		return strings.ToLower(s), 0, false
	}
	base, suffix := strings.ToLower(s[:i]), strings.ToLower(s[i+1:])
	if base != "b" {
		return strings.ToLower(s), 0, false
	}
	c, ok := operand.LookupCond(suffix)
	if !ok {
		return strings.ToLower(s), 0, false
	}
	return "b.cond", c, true
}

// splitRegSuffix separates a register name from a vector arrangement.
//
// v0.4s is one identifier token, and this is where it stops being one. The
// lane form v2.s[1] arrives as `v2.s` with the bracket still ahead, so this
// returns the element width and the caller reads the index.
func splitRegSuffix(s string) (base, suffix string) {
	i := strings.IndexByte(s, '.')
	if i <= 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}

// arrangement resolves the spelling after the dot: 16b, 8h, 4s, 2d, and the
// short forms.
//
// The Go identifiers are V16B and V4S rather than 16B and 4S because an
// identifier cannot start with a digit — and B16 was already taken by the
// scalar register b16, which is why reg/ spells them the way it does.
func arrangement(s string) (reg.Arrangement, bool) {
	switch strings.ToLower(s) {
	case "8b":
		return reg.V8B, true
	case "16b":
		return reg.V16B, true
	case "4h":
		return reg.V4H, true
	case "8h":
		return reg.V8H, true
	case "2s":
		return reg.V2S, true
	case "4s":
		return reg.V4S, true
	case "1d":
		return reg.V1D, true
	case "2d":
		return reg.V2D, true
	}
	return reg.ArrNone, false
}

// element resolves a lane's element width: the `s` of v2.s[1].
func element(s string) (reg.Elem, bool) {
	switch strings.ToLower(s) {
	case "b":
		return reg.ElemB, true
	case "h":
		return reg.ElemH, true
	case "s":
		return reg.ElemS, true
	case "d":
		return reg.ElemD, true
	}
	return reg.ElemNone, false
}

// shiftOp resolves a shift mnemonic.
func shiftOp(s string) (operand.Shift, bool) {
	switch strings.ToLower(s) {
	case "lsl":
		return operand.LSL, true
	case "lsr":
		return operand.LSR, true
	case "asr":
		return operand.ASR, true
	case "ror":
		return operand.ROR, true
	}
	return 0, false
}

// extendOp resolves an extend mnemonic.
func extendOp(s string) (operand.Extend, bool) {
	switch strings.ToLower(s) {
	case "uxtb":
		return operand.UXTB, true
	case "uxth":
		return operand.UXTH, true
	case "uxtw":
		return operand.UXTW, true
	case "uxtx":
		return operand.UXTX, true
	case "sxtb":
		return operand.SXTB, true
	case "sxth":
		return operand.SXTH, true
	case "sxtw":
		return operand.SXTW, true
	case "sxtx":
		return operand.SXTX, true
	}
	return 0, false
}

// modifier resolves an address-role modifier, in either platform's spelling.
//
// The wide-move family — :abs_g0:, :abs_g1_nc: and the rest — is recognized and
// refused rather than unrecognized. It is real syntax naming a real relocation
// group that this tree declares and does not wire, and "unknown modifier" would
// send a reader looking for a spelling error that is not there.
func modifier(name string) (text.Modifier, bool, string) {
	n := strings.ToLower(name)
	if m, ok := text.LookupModifier(":" + n + ":"); ok {
		return m, true, ""
	}
	if strings.HasPrefix(n, "abs_g") || strings.HasPrefix(n, "prel_g") {
		return text.ModNone, false,
			"the MOVW_UABS and MOVW_PREL relocation families are declared and not wired; " +
				"materialize the address with adrp and add instead"
	}
	if strings.HasPrefix(n, "tprel_") || strings.HasPrefix(n, "tlsdesc_") ||
		strings.HasPrefix(n, "gottprel_") || strings.HasPrefix(n, "tlsgd_") {
		return text.ModNone, false,
			"the TLS relocation family is declared and not wired"
	}
	return text.ModNone, false, ""
}