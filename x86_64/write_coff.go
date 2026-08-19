// x86_64/write_coff.go
//
// The COFF platform writer.
//
// Unlike i386, this mangles nothing. Win64 dropped the cdecl decoration —
// no leading underscore, no trailing @N for stdcall, no leading @ for
// fastcall — because Win64 has one calling convention and nothing left for
// the decoration to disambiguate. A symbol goes into the table spelled the
// way it was written.
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/objectfile/coff"
)

var coffTarget = coff.TargetWindowsAMD64

func (a *Assembler) serializeCOFF() ([]byte, error) {
	if err := a.checkNonEmpty(); err != nil {
		return nil, err
	}

	f := coff.NewFile(coffTarget)

	// The subsystem is left at objectfile/coff's default. It is a field of
	// an image's optional header and an object file has none — the value is
	// recorded and never written, so setting it here would be stating a
	// preference about a link this package is not performing.

	for _, s := range a.sections {
		sec, err := a.coffSection(s)
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

func (a *Assembler) coffSection(s *Section) (coff.Section, error) {
	code, fixups, err := a.prepare(s)
	if err != nil {
		return coff.Section{}, err
	}

	kind, custom := coffSectionKind(s.name)
	out := coff.Section{
		Kind:   kind,
		Custom: custom,
		Align:  uint32(s.align),
	}
	if s.nobits {
		out.VSize = uint64(len(code))
	} else {
		out.Code = code
	}

	for _, sym := range s.syms {
		if !sym.Defined {
			continue
		}
		out.Symbols = append(out.Symbols, coff.Symbol{
			Name:    sym.Name,
			Offset:  uint32(sym.Offset),
			Size:    uint32(sym.Size),
			Binding: coffBinding(sym.Binding),
			Kind:    coffSymKind(sym.Type),
		})
	}

	for _, fx := range fixups {
		kind, ok := coffRelocKind(fx.kind)
		if !ok {
			return coff.Section{}, &Error{
				Section: string(s.name), Offset: fx.off, HasOff: true,
				Err: fmt.Errorf("%w: %s is marked mapped and has no objectfile/coff kind",
					ErrReloc, RelocName(fx.kind)),
			}
		}
		out.Relocs = append(out.Relocs, coff.Reloc{
			Offset: uint32(fx.off),
			Symbol: fx.symbol,
			Kind:   kind,
			Addend: coffAddend(fixup{
				size: fx.size, tail: fx.tail, pcrel: fx.pcrel, addend: fx.addend,
			}),
		})
	}

	return out, nil
}

// coffSectionKind maps a section name to objectfile/coff's kind.
//
// The names differ from ELF's in two places that matter. Read-only data is
// `.rdata` rather than `.rodata`, and the CRT initialiser arrays are
// `.CRT$XCU` and `.CRT$XTZ` — dollar-suffixed section names that the
// Windows linker sorts alphabetically and concatenates, which is how
// Windows expresses what ELF expresses with an array of pointers. Both are
// objectfile/coff's business; this only has to name the kind.
func coffSectionKind(n SectionName) (coff.SectionKind, string) {
	switch n {
	case Text:
		return coff.SectionText, ""
	case Data:
		return coff.SectionData, ""
	case Rodata:
		return coff.SectionROData, ""
	case BSS:
		return coff.SectionBSS, ""
	}
	switch string(n) {
	case ".rdata":
		return coff.SectionROData, ""
	case ".pdata":
		return coff.SectionUnwind, ""
	case ".init_array", ".CRT$XCU":
		return coff.SectionInitArray, ""
	case ".fini_array", ".CRT$XTZ":
		return coff.SectionFiniArray, ""
	case ".tls", ".tdata", ".tbss":
		return coff.SectionTLS, ""
	}
	return coff.SectionCustom, string(n)
}

func coffBinding(b SymAttr) coff.Binding {
	switch b {
	case Global, Hidden:
		return coff.BindingGlobal
	case Weak:
		// objectfile/coff maps BindingWeak to IMAGE_SYM_CLASS_EXTERNAL,
		// which is a plain global: COFF spells weakness with a separate
		// IMAGE_SYM_CLASS_WEAK_EXTERNAL record and an auxiliary entry
		// naming the fallback, and neither is reachable from here. A weak
		// symbol therefore links as a strong one, which is a difference
		// that shows up only when two objects define the same name.
		return coff.BindingWeak
	}
	return coff.BindingLocal
}

func coffSymKind(t SymAttr) coff.SymbolKind {
	switch t {
	case Func:
		return coff.SymFunc
	}
	// Everything else is SymData. COFF's type field distinguishes only
	// function from not-function — the 0x20 in IMAGE_SYM_TYPE — so Object,
	// TLS and None collapse here rather than losing information they were
	// carrying.
	return coff.SymData
}

// coffRelocKind maps this package's kind to objectfile/coff's.
//
// ADDR32NB is reachable two ways in the layer below — RelocIAT and
// RelocAddr32NB both emit IMAGE_REL_AMD64_ADDR32NB — and this picks
// RelocAddr32NB. The distinction is about what the address points at rather
// than how it is computed, and nothing above here states it.
func coffRelocKind(k RelocKind) (coff.RelocKind, bool) {
	switch k {
	case IMAGE_REL_AMD64_ADDR64:
		return coff.RelocAbs64, true
	case IMAGE_REL_AMD64_ADDR32:
		return coff.RelocAbs32, true
	case IMAGE_REL_AMD64_ADDR32NB:
		return coff.RelocAddr32NB, true
	case IMAGE_REL_AMD64_REL32:
		return coff.RelocPCRel32, true
	case IMAGE_REL_AMD64_SECREL:
		return coff.RelocTLSIE, true
	}
	return 0, false
}