// x86_64/operand/sym.go
package operand

// RelocKind is a relocation kind. The type lives here because a SymRef has to
// carry one and nothing may import the root package; the R_X86_64_*,
// IMAGE_REL_AMD64_* and X86_64_RELOC_* constants are declared at the root as
// values of this type, along with the table saying which are valid where.
type RelocKind uint16

// RelocNone is the zero kind: "the encoder picks". A Ref built without an
// explicit kind gets this, and encode/ chooses from the form — a call site
// gets PLT32, a RIP-relative load gets PC32. Naming a kind overrides that.
const RelocNone RelocKind = 0

// Target is something a displacement can point at: a label defined in this
// unit, or a symbol resolved later. Both are addresses that are not yet
// numbers, which is why they share a slot in Mem.
type Target interface {
	// SymName is the symbol or label name.
	SymName() string
	// Reloc is the relocation kind the caller asked for, or RelocNone.
	Reloc() RelocKind
	isTarget()
}

// Label is a reference to a label defined somewhere in this assembly unit.
// Whether it resolves without a relocation depends on where it lands: a label
// in the same section is folded at Serialize, one in another section needs a
// fixup, and on Flat the latter is refused.
type Label string

func (l Label) SymName() string   { return string(l) }
func (l Label) Reloc() RelocKind  { return RelocNone }
func (Label) isTarget()           {}
func (l Label) String() string    { return string(l) }

// SymRef is a reference to a symbol, with the relocation kind that should
// record it. The symbol need not be defined in this unit.
type SymRef struct {
	Name string
	Kind RelocKind

	// Addend is the logical addend: the offset from the symbol the caller
	// means. It is not adjusted for the width or position of the field.
	// A call's displacement ends the instruction and the assembler knows
	// that because it placed the field, so you never write -4 here;
	// write_elf.go converts to the raw addend the format wants.
	Addend int64
}

// Ref builds a symbol reference. Pass RelocNone to let the encoder choose the
// kind from the form.
func Ref(name string, kind RelocKind) SymRef {
	return SymRef{Name: name, Kind: kind}
}

// At returns a copy of the reference offset by n bytes.
func (s SymRef) At(n int64) SymRef { s.Addend += n; return s }

func (s SymRef) SymName() string  { return s.Name }
func (s SymRef) Reloc() RelocKind { return s.Kind }
func (SymRef) isTarget()          {}

func (s SymRef) String() string {
	if s.Addend == 0 {
		return s.Name
	}
	if s.Addend > 0 {
		return s.Name + "+" + itoa(s.Addend)
	}
	return s.Name + itoa(s.Addend)
}