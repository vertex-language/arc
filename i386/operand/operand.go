// Package operand is the i386 operand set: what an instruction can be given.
//
// The inhabitants are in one-to-one correspondence with the operand classes
// real i386 silicon accepts. There is no operand here that names something no
// encoding exists for, and there is no operand that survives an Emit — a
// section is a byte buffer and a fixup list, so nothing in this package is a
// node in anything.
//
// This package exists on i386 and x86_64 and nowhere else in the tree. On a
// fixed-width arch the operand types live in the arch root, because there is
// no encode/ below them to serve.
package operand

import (
	"fmt"

	"github.com/vertex-language/arc/i386/reg"
)

// Operand is anything an instruction can take. It is reg.Value under another
// name: the seal lives in reg because reg is the lower package, and this alias
// is the spelling the rest of the tree uses.
type Operand = reg.Value

// Imm is an immediate. Values are stored signed and widened or narrowed by the
// encoder to the width the resolved form calls for; this type carries no width
// of its own, because on x86 the immediate width is a property of the form.
type Imm int64

func (Imm) isValue() {}

// Bits is 0: an immediate takes its width from the form it is encoded into.
func (Imm) Bits() int { return 0 }

func (i Imm) String() string { return fmt.Sprintf("%d", int64(i)) }

// Label is a name for an offset within one section.
//
// A Label resolves at Serialize as a direct fixup with no relocation record.
// Anything crossing a section boundary or leaving the object is a relocation
// and must say so as a SymRef; arc never guesses which one was meant.
type Label string

func (Label) isValue() {}
func (Label) Bits() int { return 0 }

func (l Label) String() string { return string(l) }

// RelocKind identifies a relocation. The value space belongs to the i386
// package root, which declares the psABI and COFF constants and holds the
// platform validity table; this package carries the value and interprets none
// of it. That is what lets a SymRef exist below the package that knows what
// R_386_PLT32 means.
type RelocKind uint32

// SymRef is a reference to a symbol, with the relocation kind that resolves it.
//
// The constructor is spelled Ref, matching the call sites in the builder
// documentation; the type takes the other name because Go cannot have both.
type SymRef struct {
	reg.Seal
	name   string
	kind   RelocKind
	addend int32
}

// Ref names a symbol and the relocation kind that resolves it.
func Ref(name string, kind RelocKind) SymRef {
	return SymRef{name: name, kind: kind}
}

// Plus sets a logical addend: Ref("buf", k).Plus(16) means buf+16.
//
// The addend is logical. The field-position correction — the -4 written by
// hand against objectfile/elf because a rel32 displacement is relative to the
// end of the instruction — is the assembler's, which knows where the field
// sits because it just placed it.
func (r SymRef) Plus(n int32) SymRef { r.addend += n; return r }

func (r SymRef) Name() string     { return r.name }
func (r SymRef) Kind() RelocKind  { return r.kind }
func (r SymRef) Addend() int32    { return r.addend }
func (r SymRef) Bits() int        { return 0 }

func (r SymRef) String() string {
	if r.addend == 0 {
		return r.name
	}
	return fmt.Sprintf("%s%+d", r.name, r.addend)
}