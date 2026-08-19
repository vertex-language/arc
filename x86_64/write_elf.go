// x86_64/write_elf.go
//
// The ELF platform writer: this package's sections and symbols translated
// into objectfile/elf's, and the one place the logical addend becomes
// r_addend.
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/objectfile/elf"
)

// elfTarget is the target handed to objectfile/elf.
//
// The OS field is OSFreestanding rather than OSLinux, and that is not a
// claim that the output will not run on Linux. `arc` does not know what
// Linux is: a platform here is an object format, the OS and vendor fields
// of a triple are discarded at the boundary, and OSFreestanding is
// objectfile/elf's spelling of "no OS was stated." It selects nothing —
// e_machine and layout come from Arch — and EI_OSABI stays at the System V
// default, which is what both Linux and a bare-metal loader expect.
var elfTarget = elf.Target{Arch: elf.ArchAMD64, OS: elf.OSFreestanding}

func (a *Assembler) serializeELF() ([]byte, error) {
	if err := a.checkNonEmpty(); err != nil {
		return nil, err
	}

	f := elf.NewFile(elfTarget)

	// .note.GNU-stack is emitted by default and left that way. An object
	// without one tells a modern linker nothing about whether it needs an
	// executable stack, and the linker's answer to nothing is to mark the
	// whole image executable-stack — so omitting the section is not a
	// neutral act, it is a decision made by silence.
	f.EnableGNUStack(true)

	for _, s := range a.sections {
		sec, err := a.elfSection(s)
		if err != nil {
			return nil, err
		}
		f.AddSection(sec)
	}

	b, err := f.Serialize()
	if err != nil {
		// objectfile/elf's diagnostics know the format and not the source.
		// Wrapping keeps the category and adds nothing false: there is no
		// single section or offset to name for a whole-file failure.
		return nil, &Error{Err: fmt.Errorf("%w: %s", ErrPlatform, err)}
	}
	return b, nil
}

func (a *Assembler) elfSection(s *Section) (elf.Section, error) {
	code, fixups, err := a.prepare(s)
	if err != nil {
		return elf.Section{}, err
	}

	kind, custom := elfSectionKind(s.name)
	out := elf.Section{
		Kind:   kind,
		Custom: custom,
		Align:  uint32(s.align),
	}

	// A nobits section has a size and no bytes. objectfile/elf reads the
	// extent from VSize and writes no file content, which is why the bytes
	// are dropped here rather than handed over and ignored there.
	if s.nobits {
		out.VSize = uint64(len(code))
	} else {
		out.Code = code
	}

	for _, sym := range s.syms {
		if !sym.Defined {
			continue
		}
		out.Symbols = append(out.Symbols, elf.Symbol{
			Name:    sym.Name,
			Offset:  uint32(sym.Offset),
			Size:    uint32(sym.Size),
			Binding: elfBinding(sym.Binding),
			Kind:    elfSymKind(sym.Type),
		})
	}

	for _, f := range fixups {
		kind, ok := elfRelocKind(f.kind)
		if !ok {
			// Unreachable through checkFixup, which refuses an unmapped
			// kind before this runs. Reaching it means reloc_elf.go marked
			// a kind mapped that this switch has no case for, which is a
			// table error rather than a caller error — and saying so is
			// more useful than a missing relocation.
			return elf.Section{}, &Error{
				Section: string(s.name), Offset: f.off, HasOff: true,
				Err: fmt.Errorf("%w: %s is marked mapped and has no objectfile/elf kind",
					ErrReloc, RelocName(f.kind)),
			}
		}
		out.Relocs = append(out.Relocs, elf.Reloc{
			Offset: uint32(f.off),
			Symbol: f.symbol,
			Kind:   kind,
			Addend: elfAddend(fixup{
				size: f.size, tail: f.tail, pcrel: f.pcrel, addend: f.addend,
			}),
		})
	}

	return out, nil
}

// elfSectionKind maps a section name to objectfile/elf's kind, which is what
// picks the section's flags and its canonical name.
//
// A name this does not recognise goes through as SectionCustom with the
// name written verbatim. That is the honest answer: `arc` does not have a
// table of every section name in use, and inventing flags for `.text.hot`
// by prefix-matching would be guessing at what the caller meant.
func elfSectionKind(n SectionName) (elf.SectionKind, string) {
	switch n {
	case Text:
		return elf.SectionText, ""
	case Data:
		return elf.SectionData, ""
	case Rodata:
		return elf.SectionROData, ""
	case BSS:
		return elf.SectionBSS, ""
	}
	switch string(n) {
	case ".eh_frame":
		return elf.SectionUnwind, ""
	case ".init_array":
		return elf.SectionInitArray, ""
	case ".fini_array":
		return elf.SectionFiniArray, ""
	case ".tdata", ".tbss":
		return elf.SectionTLS, ""
	}
	return elf.SectionCustom, string(n)
}

func elfBinding(b SymAttr) elf.Binding {
	switch b {
	case Global, Hidden:
		// Hidden is a global binding plus a visibility byte. objectfile/elf
		// has no st_other parameter, so the binding survives and the
		// visibility does not — which is a gap in the layer below, not a
		// reason to demote the symbol to local here. A local symbol is
		// invisible to the linker entirely; a hidden one is not.
		return elf.BindingGlobal
	case Weak:
		return elf.BindingWeak
	}
	return elf.BindingLocal
}

func elfSymKind(t SymAttr) elf.SymbolKind {
	switch t {
	case Func:
		return elf.SymFunc
	case Object, TLS:
		return elf.SymData
	}
	// objectfile/elf has SymFunc, SymData and SymSection and no notype.
	// SymData is STT_OBJECT, which is wrong for a bare label — but
	// SymSection is worse, since it would claim the symbol names a whole
	// section. This is the closest of three imperfect answers and the
	// reason a caller who cares writes the type.
	return elf.SymData
}

// elfRelocKind maps this package's kind to objectfile/elf's much smaller
// vocabulary.
//
// The narrowness is the point of the mapped/unmapped split in reloc_elf.go:
// this switch is what "mapped" means, and a kind absent from both is
// refused at checkFixup with a message naming the gap.
func elfRelocKind(k RelocKind) (elf.RelocKind, bool) {
	switch k {
	case R_X86_64_64:
		return elf.RelocAbs64, true
	case R_X86_64_32:
		return elf.RelocAbs32, true
	case R_X86_64_PC32:
		return elf.RelocPCRel32, true
	case R_X86_64_PLT32:
		return elf.RelocPLT32, true
	case R_X86_64_GOTPCREL:
		return elf.RelocGOTLoad, true
	case R_X86_64_TLSGD:
		return elf.RelocTLSGD, true
	case R_X86_64_GOTTPOFF:
		return elf.RelocTLSIE, true
	case R_X86_64_TPOFF32:
		return elf.RelocTLSLE, true
	}
	return 0, false
}