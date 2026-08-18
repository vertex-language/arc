package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/encode"
	"github.com/vertex-language/arc/objectfile/coff"
)

// The COFF platform writer: translates every Section this package
// accumulated into objectfile/coff's own Section/Symbol/Reloc shapes and
// hands them to coff.File.Serialize.
//
// coff.SectionKind's nine values are declared in the same order as this
// package's SectionKind (asm.go) and elf.SectionKind, so all three
// translate by a bare conversion — the same one-round-trip-test-instead-of-
// a-switch rule docs/builder.md states for every arch's write_*.go.
//
// i386 cdecl name mangling: Win32's classic C calling convention prefixes
// every external symbol with a leading underscore — "main" becomes "_main"
// in both the symbol table and every relocation that references it.
// objectfile/coff never does this on its own — its package doc's "i386
// caveat" says so explicitly — so it happens here, uniformly, for every
// symbol this package defines or references on a COFF target. stdcall's
// trailing "@N" and fastcall's leading "@" are not applied: arc's text
// layer carries no calling-convention annotation to derive them from, so
// only the cdecl prefix is added. A symbol meant to be called stdcall from
// another translation unit needs its fully decorated name spelled out by
// hand until that information has somewhere to live.
func coffTarget() coff.Target { return coff.TargetWindowsX86 }

// coffRelocKind maps this package's IMAGE_REL_I386_* (reloc_coff.go) to
// objectfile/coff's RelocKind. Only the four objectfile/coff's own
// relocType actually wires up for machineI386 are listed — ABSOLUTE, DIR16,
// REL16 and SECTION have no i386 case there and are refused at the call
// site below rather than reaching coff.File.Serialize and failing with a
// less specific message.
var coffRelocKind = map[RelocKind]coff.RelocKind{
	IMAGE_REL_I386_DIR32:   coff.RelocAbs32,
	IMAGE_REL_I386_DIR32NB: coff.RelocAddr32NB,
	IMAGE_REL_I386_REL32:   coff.RelocPCRel32,
	IMAGE_REL_I386_SECREL:  coff.RelocTLSIE,
}

func (a *Assembler) serializeCOFF() ([]byte, error) {
	f := coff.NewFile(coffTarget())

	for _, s := range a.sections {
		cs, err := s.toCOFF()
		if err != nil {
			return nil, err
		}
		f.AddSection(cs)
	}

	b, err := f.Serialize()
	if err != nil {
		return nil, &Error{Err: ErrReloc, msg: err.Error()}
	}
	return b, nil
}

// mangle applies i386's cdecl leading underscore. It is unconditional:
// every name this package hands to objectfile/coff for an X86 target goes
// through it — a defined symbol's own name and every relocation that names
// it are mangled the same way, so they still agree after decoration.
func mangle(name string) string { return "_" + name }

// toCOFF is write_elf.go's toELF read against a different format package.
// BSS is passed as VSize with no Code for the same reason it is there: a
// section with no file bytes should not get any, not even zeros.
func (s *Section) toCOFF() (coff.Section, error) {
	cs := coff.Section{
		Kind:   coff.SectionKind(s.Kind),
		Custom: s.customName(),
	}

	if s.Kind == BSS {
		cs.VSize = uint64(len(s.bytes))
	} else {
		cs.Code = s.bytes
	}

	sizes := closeSymbolSizes(s)
	for name, sym := range s.symbols {
		cs.Symbols = append(cs.Symbols, coffSymbol(name, sym, s.Kind, sizes[name]))
	}

	for _, fx := range s.fixups {
		if fx.kind != encode.FixupReloc {
			// write.go's resolveLabelFixups has already turned every
			// same-section fixup into bytes and dropped it from this list.
			continue
		}
		ck, ok := coffRelocKind[fx.reloc]
		if !ok {
			return coff.Section{}, relocErr(s.Name, uint32(fx.offset),
				fmt.Sprintf("%s has no COFF relocation objectfile/coff implements for i386", RelocName(fx.reloc)))
		}
		cs.Relocs = append(cs.Relocs, coff.Reloc{
			Offset: uint32(fx.offset),
			Symbol: mangle(fx.name),
			Kind:   ck,
			// The same "ELF RELA convention" addend write_elf.go computes —
			// logical addend plus the field-position correction. COFF's own
			// applyImplicitAddends adds the REL32-specific +4 when it bakes
			// this into the section's bytes; that offset is COFF's to make,
			// not this file's to anticipate, per write.go's own comment on
			// why the ELF and COFF field definitions differ by exactly 4.
			Addend: int64(fx.addend) + int64(fx.adjust),
		})
	}

	return cs, nil
}

// coffSymbol translates one asm.go symbolInfo. COFF has no symbol
// visibility of its own — no STV_HIDDEN/STV_PROTECTED equivalent to
// refuse against — so unlike elfSymbol this never errors on
// Hidden/Protected/Internal; those attributes simply have nothing to
// attach to on this platform and are dropped rather than rejected, since
// there is no meaningful COFF output that "hidden" could correspond to.
func coffSymbol(name string, sym *symbolInfo, kind SectionKind, size uint32) coff.Symbol {
	binding := coff.BindingLocal
	switch {
	case sym.attrs&Global != 0:
		binding = coff.BindingGlobal
	case sym.attrs&Weak != 0:
		binding = coff.BindingWeak
	}

	skind := coff.SymData
	switch sym.typ {
	case symFunc:
		skind = coff.SymFunc
	case symObject:
		skind = coff.SymData
	case symTLS:
		skind = coff.SymData
	case symNone:
		if kind.Code() {
			skind = coff.SymFunc
		}
	}

	return coff.Symbol{
		Name: mangle(name), Offset: sym.offset, Size: size,
		Binding: binding, Kind: skind, DLLExport: sym.dllExport,
	}
}