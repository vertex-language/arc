// x86_64/isa/resolve.go
package isa

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/feature"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// UnknownError is a mnemonic with no form at all, at any feature level.
type UnknownError struct{ Mnemonic string }

func (e *UnknownError) Error() string {
	return fmt.Sprintf("unknown instruction %q", e.Mnemonic)
}

// FormError is a mnemonic this target has, with no form accepting these
// operands. It names what was asked for and what exists, because "invalid
// operands" sends the reader to a manual and this does not.
type FormError struct {
	Mnemonic string
	Args     []Arg
	Have     []*Form // every declared form, for the note lines
}

func (e *FormError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "no form of %s takes (", e.Mnemonic)
	for i, a := range e.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(a.String())
	}
	b.WriteString(")")
	n := 0
	for _, f := range e.Have {
		if n == 4 {
			fmt.Fprintf(&b, "\n  ... and %d more", len(e.Have)-n)
			break
		}
		fmt.Fprintf(&b, "\n  have: %s", f)
		n++
	}
	return b.String()
}

// GateError is a form that exists and matches, held back by the feature set.
// Need is the feature that would have allowed it; the root builds the note
// line from Active.Plus(Need).GoExpr() so the message names something that
// compiles.
type GateError struct {
	Mnemonic string
	Need     feature.Feature
	Active   feature.Set
}

func (e *GateError) Error() string {
	return fmt.Sprintf("%s requires %s, not in the active feature set\n  active: %s",
		e.Mnemonic, e.Need, e.Active)
}

// Resolve picks the encoding for a mnemonic and a set of operands: the
// shortest legal one, breaking ties toward the earlier row of the table.
//
// It never picks a different instruction. If no form of this mnemonic takes
// these operands, that is a FormError, not a relaxation into one that does.
func Resolve(s feature.Set, mnemonic string, args ...Arg) (*Form, error) {
	all := byMnemonic[mnemonic]
	if len(all) == 0 {
		return nil, &UnknownError{Mnemonic: mnemonic}
	}

	var best *Form
	bestSize := 0
	gated := feature.Feature(0)
	gatedSeen := false

	for _, f := range all {
		if !f.matches(args) {
			continue
		}
		if !f.Enabled(s) {
			// Remember the cheapest gate, so the diagnostic names the
			// smallest flag that would have worked rather than the first.
			if !gatedSeen || len(f.Need.Requires()) < len(gated.Requires()) {
				gated, gatedSeen = f.Need, true
			}
			continue
		}
		n := f.size(args)
		if best == nil || n < bestSize {
			best, bestSize = f, n
		}
	}

	if best != nil {
		return best, nil
	}
	if gatedSeen {
		return nil, &GateError{Mnemonic: mnemonic, Need: gated, Active: s}
	}
	return nil, &FormError{Mnemonic: mnemonic, Args: args, Have: all}
}

func (f *Form) matches(args []Arg) bool {
	i := 0
	for _, s := range f.Slots {
		if s.Implicit {
			continue
		}
		if i >= len(args) {
			return false
		}
		if !s.Class.Match(args[i]) {
			return false
		}
		i++
	}
	return i == len(args)
}

// size is the encoded length in bytes, for ordering candidates. encode/ is
// authoritative for the actual bytes; this agrees with it on every field that
// can differ between two forms of the same mnemonic, which is what ordering
// needs and all it needs.
func (f *Form) size(args []Arg) int {
	n := 0

	switch f.Enc {
	case EncLegacy:
		if f.Attrs&Data16 != 0 {
			n++
		}
		if f.Pfx != PfxNone {
			n++
		}
		n += len(f.Map.Escape())
		if f.W == W1 || needsREX(args) {
			n++
		}
	case EncVEX:
		// Two bytes when the short form can carry it: map 0F, W0 or WIG, and
		// no X or B extension to spell.
		if f.Map == Map0F && f.W != W1 && !needsXB(args) {
			n += 2
		} else {
			n += 3
		}
	case EncEVEX:
		n += 4
	}

	n++ // opcode
	if f.Attrs&HasModRM != 0 {
		n++
		n += memBytes(args)
	}
	if f.Attrs&PlusReg != 0 && f.Attrs&HasModRM == 0 {
		// nothing: the register is in the opcode byte already counted
	}
	n += f.Imm.Bytes()
	if f.MemSlot() < 0 && f.Attrs&HasModRM == 0 {
		for _, s := range f.Slots {
			if s.Field == InMoffs {
				n += 8
				break
			}
		}
	}
	return n
}

// rexer is the question "does naming this register cost a REX byte". Reg8
// answers it for SPL/BPL/SIL/DIL, which have no encoding without one.
type rexer interface{ RexRequired() bool }

func needsREX(args []Arg) bool {
	for _, a := range args {
		switch a.Kind {
		case KindReg:
			if a.Reg.Num() >= 8 {
				return true
			}
			if r, ok := a.Reg.(rexer); ok && r.RexRequired() {
				return true
			}
		case KindMem:
			if a.Mem.HasBase && a.Mem.Base.Num() >= 8 {
				return true
			}
			if a.Mem.HasIndex && a.Mem.Index.Num() >= 8 {
				return true
			}
		}
	}
	return false
}

// needsXB is the same question for VEX's two-byte form, which folds away only
// when X and B are both unset.
func needsXB(args []Arg) bool {
	for _, a := range args {
		if a.Kind == KindMem {
			if a.Mem.HasBase && a.Mem.Base.Num() >= 8 {
				return true
			}
			if a.Mem.HasIndex && a.Mem.Index.Num() >= 8 {
				return true
			}
		}
	}
	// The r/m register, wherever it landed, extends through B.
	for _, a := range args {
		if a.Kind == KindReg && a.Reg.Num() >= 8 {
			return true
		}
	}
	return false
}

// memBytes is the SIB and displacement cost, which is a property of the
// operand rather than the form and so is the same for every candidate.
func memBytes(args []Arg) int {
	for _, a := range args {
		if a.Kind != KindMem {
			continue
		}
		m := a.Mem
		n := 0
		if m.NeedsSIB() {
			n++
		}
		switch {
		case m.RIP, m.Fixup():
			n += 4
		case m.NeedsZeroDisp8():
			n++
		case m.Disp == 0:
		case operand.Imm(m.Disp).FitsInt8():
			n++
		default:
			n += 4
		}
		return n
	}
	return 0
}

var _ = reg.RAX // the table files below name registers; keep the import honest