package gas

import (
	"strings"

	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/text"
)

// Mnemonic spelling: the suffix, the AT&T-only names, and the operand order.
//
// The operand size in this dialect is a letter on the end of the mnemonic,
// and stripping it is the one place a parser can quietly turn `call` into
// `cal` with an l suffix. The rule below never does, because it asks the ISA
// table before it cuts: a word that names a form is a mnemonic, whatever it
// ends in.

// suffixWidth maps the AT&T size letters to widths. q is here because the
// letter exists in the family and a q-suffixed mnemonic on i386 should say
// "64-bit operands need x86_64" rather than "unknown mnemonic".
var suffixWidth = map[byte]text.Width{
	'b': text.Width8,
	'w': text.Width16,
	'l': text.Width32,
	'q': text.Width64,
}

// attNames are the mnemonics this dialect spells differently from the SDM.
// These are not aliases in the ISA sense — the encoding is the same and the
// table declares one name — they are two syntaxes' names for one instruction,
// which is exactly what a dialect is.
var attNames = map[string]string{
	"cbtw": "cbw",
	"cwtl": "cwde",
	"cltd": "cdq",
	"cwtd": "cwd",
}

// attPrinted is the reverse, for the printer.
var attPrinted = map[string]string{}

func init() {
	for att, canon := range attNames {
		attPrinted[canon] = att
	}
}

// noReverse names the mnemonics AT&T does not write backwards.
//
// The reversal is otherwise total — even the three-operand IMUL is written
// backwards — but ENTER takes its two immediates in the same order in both
// syntaxes, because they are a frame size and a nesting level rather than a
// source and a destination. One exception, named, is cheaper than a rule with
// a footnote.
var noReverse = map[string]bool{"enter": true}

// splitMnemonic resolves a written word to a canonical mnemonic and a size.
//
// The full word is tried first, so `call`, `mul` and `sal` keep their final
// letter. Only when it names no form is a suffix considered, and only when
// what remains does name one.
func splitMnemonic(word string) (mnemonic string, size text.Width, ok bool) {
	w := strings.ToLower(word)

	if canon, is := attNames[w]; is {
		return canon, text.WidthNone, true
	}
	if len(isa.Forms(w)) > 0 {
		return w, text.WidthNone, true
	}

	// The two-suffix forms. movzbl is MOVZX with a byte source and a
	// doubleword destination; the destination width is the register operand's
	// and needs no field, so only the source width is kept.
	if len(w) == 6 && (strings.HasPrefix(w, "movz") || strings.HasPrefix(w, "movs")) {
		src, dst := w[4], w[5]
		sw, okSrc := suffixWidth[src]
		_, okDst := suffixWidth[dst]
		if okSrc && okDst {
			name := "movzx"
			if w[3] == 's' {
				name = "movsx"
			}
			if len(isa.Forms(name)) > 0 {
				return name, sw, true
			}
		}
	}

	if len(w) > 1 {
		if sz, is := suffixWidth[w[len(w)-1]]; is {
			base := w[:len(w)-1]
			if canon, isAtt := attNames[base]; isAtt {
				return canon, sz, true
			}
			if len(isa.Forms(base)) > 0 {
				return base, sz, true
			}
		}
	}

	return w, text.WidthNone, false
}

// unknownMnemonic is the diagnostic for a word that names no form, with the
// note that tells the two likely causes apart.
func unknownMnemonic(p text.Pos, word string) *text.Error {
	e := text.Errorf(p, "unknown instruction %q", word)
	w := strings.ToLower(word)
	if len(w) > 1 {
		if _, is := suffixWidth[w[len(w)-1]]; is {
			if w[len(w)-1] == 'q' {
				return e.Note("the q suffix is 64-bit; i386 is 32-bit, use -t x86_64-elf").
					Note("arc isa %s lists the forms this target has", w[:len(w)-1])
			}
			e.Note("no instruction %q, with or without the %c suffix", w[:len(w)-1], w[len(w)-1])
		}
	}
	return e.Note("arc isa lists every mnemonic this target encodes")
}

// isBranch reports whether any form of a mnemonic takes a branch displacement.
//
// This is what tells a bare name in an operand slot from a bare name in a
// memory slot: `jmp foo` is a relative branch to foo and `mov foo, %eax`
// loads through it. The answer comes from the form table rather than a list
// of branch mnemonics, so a Jcc added to the table is a branch here without
// this file changing.
func isBranch(mnemonic string) bool {
	for _, f := range isa.Forms(mnemonic) {
		for _, o := range f.Ops {
			if o.Slot == isa.SlotRel {
				return true
			}
		}
	}
	return false
}

// reverse turns written order into stored order, or back. AT&T writes source
// first and text.Inst stores destination first, matching isa.Form.Ops.
func reverse(mnemonic string, ops []text.Operand) []text.Operand {
	if noReverse[mnemonic] || len(ops) < 2 {
		return ops
	}
	out := make([]text.Operand, len(ops))
	for i, o := range ops {
		out[len(ops)-1-i] = o
	}
	return out
}

// printedMnemonic is the name this dialect writes for a canonical mnemonic.
func printedMnemonic(mnemonic string, size text.Width) string {
	if att, ok := attPrinted[mnemonic]; ok && size == text.WidthNone {
		return att
	}

	// MOVZX and MOVSX carry both widths in the name, and the destination one
	// is not in the Inst — it is the register operand's. The printer supplies
	// l, because a 16-bit destination needs the 0x66 prefix and arc's i386
	// does not declare that form.
	if size != text.WidthNone && (mnemonic == "movzx" || mnemonic == "movsx") {
		return mnemonic[:4] + string(suffixLetter(size)) + "l"
	}

	if size == text.WidthNone {
		return mnemonic
	}
	return mnemonic + string(suffixLetter(size))
}

func suffixLetter(w text.Width) byte {
	switch w {
	case text.Width8:
		return 'b'
	case text.Width16:
		return 'w'
	case text.Width32:
		return 'l'
	case text.Width64:
		return 'q'
	}
	return 0
}