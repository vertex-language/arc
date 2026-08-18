package i386

import (
	"fmt"

	"github.com/vertex-language/arc/objectfile/flat"
)

// The Flat platform writer: concatenates every Section this package
// accumulated, in creation order, with no header and no symbol table —
// objectfile/flat's own shape.
//
// flat.SectionKind's nine values are declared in the same order as this
// package's SectionKind (asm.go), elf.SectionKind, and coff.SectionKind, so
// all four translate by a bare conversion, the same one-round-trip-test
// rule docs/builder.md states for every arch's write_*.go.
//
// Flat binary forbids relocations outright — flat.Section has no Relocs
// field to begin with, and its own doc says a section carrying one is
// meant to be an impossible state to construct. A Ref operand reaching
// this writer with its fixup still unresolved means exactly that: this
// section referenced something outside itself (another section, an
// external symbol) that a flat image has no loader and no linker to
// resolve at load time. That is refused here, at the one place a flat
// target can still say why, rather than silently emitting bytes for
// whatever the fixup's zero-filled placeholder happened to be.
func (a *Assembler) serializeFlat() ([]byte, error) {
	f := flat.NewFile()
	if a.hasBase {
		f.SetBaseAddress(uint64(a.baseAddr))
	}

	for _, s := range a.sections {
		if err := s.checkNoRelocs(); err != nil {
			return nil, err
		}
		f.AddSection(s.toFlat())
	}

	b, err := f.Serialize()
	if err != nil {
		return nil, &Error{Err: ErrReloc, msg: err.Error()}
	}
	return b, nil
}

// checkNoRelocs refuses a section that still carries a cross-section or
// external fixup. write.go's resolveLabelFixups has already turned every
// same-section FixupLabel into bytes and dropped it from the list by the
// time Serialize gets here, so anything left in s.fixups is exactly the
// shape flat cannot hold.
func (s *Section) checkNoRelocs() error {
	if len(s.fixups) == 0 {
		return nil
	}
	fx := s.fixups[0]
	return relocErr(s.Name, uint32(fx.offset),
		fmt.Sprintf("reference to %q cannot be resolved in a flat image", fx.name),
		"flat binary has no relocation record and no linker to resolve one at load time",
		"only references within one section, to a bare Label already defined there, can appear in a flat target")
}

// toFlat converts one Section. BSS is passed as VSize with no Code, the
// same convention write_elf.go and write_coff.go follow — sectionChunk
// materialises the zero bytes itself, so there is nothing for this package
// to zero-fill in advance.
//
// Symbols are not translated at all: flat.Section.Symbols exists only for
// call-site symmetry with the other three format packages and is never
// read by flat's own encoder, so building the slice here would be work
// with no effect on the output. A flat image simply has no symbol table
// for a Label's attributes to land in.
func (s *Section) toFlat() flat.Section {
	fs := flat.Section{
		Kind:   flat.SectionKind(s.Kind),
		Custom: s.customName(),
		Align:  uint32(sectionAlignHint(s.Kind)),
	}
	if s.Kind == BSS {
		fs.VSize = uint64(len(s.bytes))
	} else {
		fs.Code = s.bytes
	}
	return fs
}

// sectionAlignHint is a modest default tail-padding boundary per kind, since
// asm.go's Section carries no Align field of its own — alignment within a
// flat image is something the caller controls directly with Section.Align
// (asm.go's, not flat's) before emitting bytes, so this only affects the
// padding between one section's end and the next section's start.
func sectionAlignHint(k SectionKind) int {
	if k.Code() {
		return 16
	}
	return 1
}