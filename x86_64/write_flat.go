// x86_64/write_flat.go
//
// The Flat platform writer: every section concatenated in creation order,
// no header, no symbol table, no relocations.
//
// This is the writer with the most to refuse. The other three hand an
// unresolved reference to a linker; there is no linker here and no record
// to leave one, so a reference that cannot be folded at Serialize is a
// reference that can never be resolved — and writing zeros where an address
// belongs produces an image that boots and jumps to nowhere.
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/objectfile/flat"
)

func (a *Assembler) serializeFlat() ([]byte, error) {
	if err := a.checkNonEmpty(); err != nil {
		return nil, err
	}

	f := flat.NewFile()
	f.SetBaseAddress(a.cfg.base)

	for _, s := range a.sections {
		sec, err := a.flatSection(s)
		if err != nil {
			return nil, err
		}
		f.AddSection(sec)
	}

	b, err := f.Serialize()
	if err != nil {
		return nil, &Error{Err: fmt.Errorf("%w: %s", ErrPlatform, err)}
	}
	return b, nil
}

func (a *Assembler) flatSection(s *Section) (flat.Section, error) {
	code, fixups, err := a.prepare(s)
	if err != nil {
		return flat.Section{}, err
	}

	// Anything prepare could not fold has nowhere to go. flat.Section has
	// no Relocs field at all — the format forbids relocations, so a flat
	// section carrying one is not a value that can be constructed — and
	// this is where that becomes a diagnostic instead of a type error.
	//
	// The message names which of the three reasons applies, because they
	// have different fixes: define the symbol, move it into this section,
	// or stop naming a relocation kind.
	if len(fixups) > 0 {
		return flat.Section{}, a.flatRefusal(s, fixups[0])
	}

	out := flat.Section{
		Kind:   flatSectionKind(s.name),
		Custom: string(s.name),
		Align:  uint32(s.align),
	}

	// A flat BSS is zeroes in the file. There is no loader to zero-fill a
	// reservation — that is what a header is for and this format has none —
	// so the bytes are written out, which is why objectfile/flat reads the
	// size from VSize and emits it rather than recording it.
	if s.nobits {
		out.VSize = uint64(len(code))
	} else {
		out.Code = code
	}
	return out, nil
}

// flatRefusal explains why a fixup could not be folded.
func (a *Assembler) flatRefusal(s *Section, f resolvedFixup) error {
	sym := a.symsBy[f.symbol]

	switch {
	case sym == nil || !sym.Defined:
		return &Error{
			Section: string(s.name), Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s is defined nowhere in this image, and a flat "+
				"binary has no relocation record to leave for a linker",
				ErrUndefined, f.symbol),
		}

	case sym.Section != s:
		return &Error{
			Section: string(s.name), Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s is in %s and this reference is in %s; the distance "+
				"between two sections is not known until they are placed, and a flat "+
				"binary records nothing that would let anything place them",
				ErrPlatform, f.symbol, sym.Section.name, s.name),
		}

	case !f.pcrel:
		return &Error{
			Section: string(s.name), Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s is referenced by absolute address, which depends on "+
				"where the image is loaded; SetBaseAddress states that for the caller's "+
				"benefit and nothing here resolves against it",
				ErrPlatform, f.symbol),
		}

	default:
		// Foldable but for the named kind. prepare only folds when the
		// caller named none, because naming one asks for something a
		// direct displacement does not provide.
		return &Error{
			Section: string(s.name), Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: the reference to %s names %s, and a flat binary has no "+
				"relocation record to put it in; drop the kind and the displacement "+
				"resolves here",
				ErrPlatform, f.symbol, RelocName(f.kind)),
		}
	}
}

// flatSectionKind is the one thing flat's Kind decides: whether the section
// is BSS-like and emits zeroes, or emits Code as-is.
//
// It picks no name and no flags, because a flat image has neither. Custom
// is filled in anyway — objectfile/flat documents it as informational only,
// and a caller reading back the section list is better served by the name
// than by an empty string.
func flatSectionKind(n SectionName) flat.SectionKind {
	if isNobitsSection(n) {
		return flat.SectionBSS
	}
	switch n {
	case Text:
		return flat.SectionText
	case Data:
		return flat.SectionData
	case Rodata:
		return flat.SectionROData
	}
	return flat.SectionCustom
}