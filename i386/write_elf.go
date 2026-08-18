package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/encode"
	"github.com/vertex-language/arc/objectfile/elf"
)

// The ELF platform writer: translates every Section this package
// accumulated into objectfile/elf's own Section/Symbol/Reloc shapes and
// hands them to elf.File.Serialize, which owns the actual byte layout.
//
// elf.SectionKind's nine values are declared in the same order as this
// package's SectionKind (asm.go), so the two translate by a bare
// conversion — one round-trip fact rather than a switch copied a second
// time, the same rule docs/builder.md states for every arch's write_elf.go.

// elfTarget is the (Arch, OS) pair objectfile/elf writes for. i386 has one
// e_machine regardless of OS; Freestanding is the OSABI-agnostic choice
// until this package exposes its own SetOSABI the way the typed a.ELF()
// escape hatch is meant to (see docs/builder.md's own note that the
// builder targets freestanding by default).
func elfTarget() elf.Target { return elf.TargetFreestandingX86 }

// elfRelocKind maps this package's R_386_* (reloc_elf.go) to
// objectfile/elf's RelocKind. Only three have a mapping: elf.File's own
// relocType wires up exactly R_386_32, R_386_PC32 and R_386_GOT32 for
// e_machine EM_386 today. A PLT call, a GOTOFF/GOTPC reference, or any
// R_386_TLS_* is a real relocation this package knows the number for — it
// simply has nowhere to write it yet, and toELF below says so rather than
// letting the byte reach objectfile/elf and fail there with a less specific
// "unsupported relocation ... for e_machine 3".
var elfRelocKind = map[RelocKind]elf.RelocKind{
	R_386_32:    elf.RelocAbs32,
	R_386_PC32:  elf.RelocPCRel32,
	R_386_GOT32: elf.RelocGOTLoad,
}

func (a *Assembler) serializeELF() ([]byte, error) {
	f := elf.NewFile(elfTarget())

	for _, s := range a.sections {
		es, err := s.toELF()
		if err != nil {
			return nil, err
		}
		f.AddSection(es)
	}

	b, err := f.Serialize()
	if err != nil {
		return nil, &Error{Section: "", Err: ErrReloc, msg: err.Error()}
	}
	return b, nil
}

// toELF converts one Section. BSS is passed as VSize with no Code, since a
// NOBITS section's bytes are definitionally all zero and materialising them
// into the file would be exactly the "listing that disassembles into
// garbage" problem Align's own doc explains — the difference is that BSS
// should not go in the file at all.
func (s *Section) toELF() (elf.Section, error) {
	es := elf.Section{
		Kind:   elf.SectionKind(s.Kind),
		Custom: s.customName(),
	}

	if s.Kind == BSS {
		es.VSize = uint64(len(s.bytes))
	} else {
		es.Code = s.bytes
	}

	sizes := closeSymbolSizes(s)
	for name, sym := range s.symbols {
		esym, err := elfSymbol(name, sym, s.Kind, sizes[name])
		if err != nil {
			return elf.Section{}, err
		}
		es.Symbols = append(es.Symbols, esym)
	}

	for _, fx := range s.fixups {
		if fx.kind != encode.FixupReloc {
			// write.go's resolveLabelFixups has already turned every
			// FixupLabel entry into bytes and dropped it from this list;
			// reaching this branch with anything else is this package's
			// own bug, not a user-facing one, so it is skipped rather than
			// silently mis-emitted.
			continue
		}
		ek, ok := elfRelocKind[fx.reloc]
		if !ok {
			return elf.Section{}, relocErr(s.Name, uint32(fx.offset),
				fmt.Sprintf("%s has no ELF relocation objectfile/elf implements yet", RelocName(fx.reloc)))
		}
		es.Relocs = append(es.Relocs, elf.Reloc{
			Offset: uint32(fx.offset),
			Symbol: fx.name,
			Kind:   ek,
			// The logical addend plus the field-position correction —
			// Adjust is zero for a non-PC-relative fixup, so this is
			// exactly fx.addend there, and the addend a PLT/PC-relative
			// call needs when it is not.
			Addend: int64(fx.addend) + int64(fx.adjust),
		})
	}

	return es, nil
}

// customName is empty for every section created through Section(kind):
// elf's own writer names the eight standard kinds itself and only reads
// Custom for SectionCustom.
func (s *Section) customName() string {
	if s.Kind == Custom {
		return s.Name
	}
	return ""
}

// elfSymbol translates one asm.go symbolInfo. Symbol visibility
// (Hidden/Protected/Internal) has no field in objectfile/elf's Symbol at
// all — STV_HIDDEN changes whether the linker routes a call through a PLT
// stub, so silently dropping it could turn a correct build into a
// link-time-only surprise, and this returns an error instead.
func elfSymbol(name string, sym *symbolInfo, kind SectionKind, size uint32) (elf.Symbol, error) {
	if sym.attrs&(Hidden|Protected|Internal) != 0 {
		return elf.Symbol{}, &Error{
			Err: ErrPlatform,
			msg: fmt.Sprintf(
				"symbol %q: visibility (hidden/protected/internal) is not yet supported by objectfile/elf", name),
		}
	}

	binding := elf.BindingLocal
	switch {
	case sym.attrs&Global != 0:
		binding = elf.BindingGlobal
	case sym.attrs&Weak != 0:
		binding = elf.BindingWeak
	}

	skind := elf.SymData
	switch sym.typ {
	case symFunc:
		skind = elf.SymFunc
	case symObject:
		skind = elf.SymData
	case symTLS:
		// objectfile/elf has no STT_TLS. This is the nearest available
		// type rather than a correct one — worth revisiting once a TLS
		// sequence is actually exercised against a real linker.
		skind = elf.SymData
	case symNone:
		if kind.Code() {
			skind = elf.SymFunc
		}
	}

	return elf.Symbol{
		Name: name, Offset: sym.offset, Size: size,
		Binding: binding, Kind: skind,
	}, nil
}