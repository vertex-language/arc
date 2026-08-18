// x86_64/text/gas/mnemonic.go
package gas

import (
	"strings"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/text"
)

// suffixes are gas's one-character operand-size modifiers.
var suffixes = map[byte]operand.Width{
	'b': operand.W8,
	'w': operand.W16,
	'l': operand.W32,
	'q': operand.W64,
	// 's' is short and 'x'/'y'/'z' appear on some vector forms; they are
	// not sizes in the sense this field means and are not accepted here.
}

// attNames are the mnemonics AT&T spells differently from Intel. gas accepts
// both spellings for the conversion instructions and only its own for the
// rest, so this maps one way: into the canonical name isa/ knows.
//
// The conversions are the interesting ones. cltq is "convert long to quad",
// which is Intel's CDQE, and the two names describe the same bytes from
// opposite ends — Intel names the destination, AT&T names both.
var attNames = map[string]string{
	"cbtw": "cbw",
	"cwtl": "cwde",
	"cltq": "cdqe",
	"cwtd": "cwd",
	"cltd": "cdq",
	"cqto": "cqo",

	// Far transfers, which AT&T spells with a leading l.
	"lcall": "callf",
	"ljmp":  "jmpf",
	"lret":  "retf",

	// The string instructions carry their size in the mnemonic and have no
	// unsuffixed spelling, so they are named outright rather than reached
	// through suffix stripping.
	"cltq_": "cdqe",
}

// ccAliases are the condition-code spellings that name an existing
// encoding. gas accepts every one of them and they are all the same bytes,
// so they resolve here and the canonical name is what reaches the tree.
//
// isa/ declares sixteen conditions and these are the other sixteen names
// for them. A table row per alias would be a row nothing distinguishes.
var ccAliases = map[string]string{
	"nae": "b", "c": "b",
	"nb": "ae", "nc": "ae",
	"z":  "e",
	"nz": "ne",
	"na": "be",
	"nbe": "a",
	"pe": "p", "po": "np",
	"nge": "l", "nl": "ge",
	"ng": "le", "nle": "g",
}

var ccPrefixes = []string{"j", "set", "cmov"}

// resolveMnemonic folds a written mnemonic into the canonical name and the
// size its suffix stated.
//
// The algorithm is table-driven and has to be: `call` ends in 'l' and is not
// `cal` with a long suffix, `movsb` is a string move and not `movs` with a
// byte suffix. So the full name is tried against isa/ first, and a suffix is
// only stripped when the full name is not a mnemonic and the base is.
func resolveMnemonic(s string) (name string, size operand.Width, err error) {
	s = strings.ToLower(s)

	if n, ok := attNames[s]; ok {
		return n, operand.WidthNone, nil
	}
	if isa.Forms(s) != nil {
		return s, operand.WidthNone, nil
	}

	// The two-suffix extends: movsbl, movzwq and the rest. The base is
	// movs or movz, then a from-size and a to-size.
	if n, w, ok := extendMnemonic(s); ok {
		return n, w, nil
	}

	if len(s) > 1 {
		if w, ok := suffixes[s[len(s)-1]]; ok {
			base := s[:len(s)-1]
			if n, ok := attNames[base]; ok {
				return n, w, nil
			}
			if isa.Forms(base) != nil {
				return base, w, nil
			}
			if n, ok := resolveCC(base); ok {
				return n, w, nil
			}
		}
	}

	if n, ok := resolveCC(s); ok {
		return n, operand.WidthNone, nil
	}

	return "", operand.WidthNone, text.Errorf(text.Pos{}, "unknown instruction %q", s)
}

// extendMnemonic handles movsbl, movzbq and friends: base, from-size,
// to-size. Intel writes one mnemonic and gets the sizes from the operands;
// AT&T writes both sizes and gets one mnemonic.
func extendMnemonic(s string) (string, operand.Width, bool) {
	var base string
	switch {
	case strings.HasPrefix(s, "movs") && len(s) == 6:
		base = "movsx"
	case strings.HasPrefix(s, "movz") && len(s) == 6:
		base = "movzx"
	default:
		return "", operand.WidthNone, false
	}
	from, ok1 := suffixes[s[4]]
	to, ok2 := suffixes[s[5]]
	if !ok1 || !ok2 || from >= to {
		return "", operand.WidthNone, false
	}
	// The destination size is the instruction's operand size; the source
	// size is the operand's own and reaches the memory operand through
	// text.Inst.Sized.
	return base, to, true
}

func resolveCC(s string) (string, bool) {
	for _, p := range ccPrefixes {
		if !strings.HasPrefix(s, p) {
			continue
		}
		cc := s[len(p):]
		if alias, ok := ccAliases[cc]; ok {
			if name := p + alias; isa.Forms(name) != nil {
				return name, true
			}
		}
	}
	return "", false
}

// suffixFor is the character gas would write for a width.
func suffixFor(w operand.Width) string {
	switch w {
	case operand.W8:
		return "b"
	case operand.W16:
		return "w"
	case operand.W32:
		return "l"
	case operand.W64:
		return "q"
	}
	return ""
}

// printMnemonic is the inverse: the canonical name plus the suffix gas needs.
//
// A suffix is written only when the operands do not settle the size on their
// own — `mov %rax, %rbx` needs none and `movq $1, (%rbx)` does. Writing one
// where gas would not is a spelling change arc fmt is not allowed to make,
// which is why this asks the resolved form rather than guessing.
func printMnemonic(i *text.Inst) string {
	name := i.Mnemonic
	for att, canonical := range attNames {
		if canonical == name && !strings.HasSuffix(att, "_") {
			name = att
			break
		}
	}

	if !needsSuffix(i) {
		return name
	}
	w := i.Size
	if w == operand.WidthNone && i.Form != nil {
		w = formWidth(i.Form)
	}
	return name + suffixFor(w)
}

// needsSuffix reports whether gas requires the size on the mnemonic: when
// no operand is a register, because then nothing else states it.
func needsSuffix(i *text.Inst) bool {
	if len(i.Operands) == 0 {
		return false
	}
	sawMem := false
	for _, o := range i.Operands {
		switch o.Kind {
		case text.KindReg:
			return false // a register settles the size
		case text.KindMem:
			sawMem = true
		}
	}
	return sawMem
}

// formWidth is the operand size a resolved form operates at, taken from the
// first slot that fixes one.
//
// This is what a text-level translator cannot do, and the reason arc fmt
// resolves before it prints: going from NASM's `mov qword [rbx], 1` to gas's
// `movq $1, (%rbx)` means knowing the width, and the only thing that knows
// it is the form.
func formWidth(f *isa.Form) operand.Width {
	for _, s := range f.Slots {
		if s.Implicit {
			continue
		}
		if b := s.Class.Bits(); b > 0 && b <= 64 {
			return operand.Width(b)
		}
	}
	return operand.WidthNone
}