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
//
// Imm is a struct rather than a bare int64 so it can embed reg.Seal: the seal
// lives in reg, and joining reg.Value from outside that package requires
// promoting reg.Seal's isValue, which only an embedding struct can do. A
// hand-written isValue method here would compile — Go does not error on an
// unused method — but it would be a different method from reg.Value's, since
// unexported method identity includes the declaring package. That mismatch is
// invisible inside this package, where nothing checks Imm against Operand, and
// only surfaces the moment something like isa.Class.Matches tries to use an
// Imm as one.
type Imm struct {
	reg.Seal
	n int64
}

// NewImm builds an immediate. The constructor is spelled apart from the type,
// as Ref is from SymRef: Go cannot have both a type and a function named Imm,
// and every existing call site already reads as construction.
func NewImm(n int64) Imm { return Imm{n: n} }

// Int64 is the immediate's value, sign-extended to 64 bits. The encoder and
// isa.Class.Matches narrow or range-check from this; nothing in this package
// interprets the value itself.
func (i Imm) Int64() int64 { return i.n }

// Bits is 0: an immediate takes its width from the form it is encoded into.
func (Imm) Bits() int { return 0 }

func (i Imm) String() string { return fmt.Sprintf("%d", i.n) }

// Label is a name for an offset within one section.
//
// A Label resolves at Serialize as a direct fixup with no relocation record.
// Anything crossing a section boundary or leaving the object is a relocation
// and must say so as a SymRef; arc never guesses which one was meant.
//
// Like Imm, Label is a struct embedding reg.Seal rather than a bare string,
// for the same sealing reason.
type Label struct {
	reg.Seal
	name string
}

// NewLabel names an offset within the section it is emitted into.
func NewLabel(name string) Label { return Label{name: name} }

func (Label) Bits() int { return 0 }

func (l Label) String() string { return l.name }

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

func (r SymRef) Name() string    { return r.name }
func (r SymRef) Kind() RelocKind { return r.kind }
func (r SymRef) Addend() int32   { return r.addend }
func (r SymRef) Bits() int       { return 0 }

func (r SymRef) String() string {
	if r.addend == 0 {
		return r.name
	}
	return fmt.Sprintf("%s%+d", r.name, r.addend)
}