// x86_64/write_macho.go
//
// The Mach-O platform writer.
package x86_64

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/objectfile/macho"
)

var machoTarget = macho.TargetDarwinAMD64

// MachOOptions are the Mach-O settings that have no equivalent anywhere
// else, set through a.MachO() before Serialize.
//
// There is exactly one, and it is here rather than an Option on New because
// it is a property of the output format and not of the encoding: a feature
// set changes which bytes are legal, and a minimum OS version does not.
type MachOOptions struct {
	a *Assembler
}

// MachO returns the Mach-O settings for this object.
func (a *Assembler) MachO() *MachOOptions { return &MachOOptions{a: a} }

// SetMinOS records an LC_BUILD_VERSION load command.
//
// Without it the object carries none, which is deliberate: `arc` cannot
// invent a minimum macOS version you did not state, and a wrong one is
// worse than an absent one — ld64 warns about the absence and refuses a
// version the SDK does not support.
func (o *MachOOptions) SetMinOS(platform MachOPlatform, major, minor uint8) {
	if o.a.cfg.platform != MachO {
		o.a.fail(platformErr(o.a.cfg.platform,
			"LC_BUILD_VERSION is a Mach-O load command"))
		return
	}
	o.a.cfg.machoBuild = true
	o.a.cfg.machoPlatform = platform
	o.a.cfg.machoMajor = major
	o.a.cfg.machoMinor = minor
}

// MachOPlatform is the Darwin platform an LC_BUILD_VERSION names. It is a
// separate axis from Platform: an object is Mach-O, and a Mach-O object
// targets one of these.
type MachOPlatform = macho.Platform

const (
	MacOS    = macho.MacOS
	IOS      = macho.IOS
	TVOS     = macho.TVOS
	WatchOS  = macho.WatchOS
	VisionOS = macho.VisionOS
)

func (a *Assembler) serializeMachO() ([]byte, error) {
	if err := a.checkNonEmpty(); err != nil {
		return nil, err
	}

	f := macho.NewFile(machoTarget)
	if a.cfg.machoBuild {
		f.SetMinOS(a.cfg.machoPlatform, a.cfg.machoMajor, a.cfg.machoMinor)
	}

	for _, s := range a.sections {
		sec, err := a.machoSection(s)
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

func (a *Assembler) machoSection(s *Section) (macho.Section, error) {
	code, fixups, err := a.prepare(s)
	if err != nil {
		return macho.Section{}, err
	}

	kind, custom, err := machoSectionKind(s.name)
	if err != nil {
		return macho.Section{}, err
	}

	out := macho.Section{
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
		out.Symbols = append(out.Symbols, macho.Symbol{
			Name:    sym.Name,
			Offset:  uint32(sym.Offset),
			Size:    uint32(sym.Size),
			Binding: machoBinding(sym.Binding),
			Kind:    machoSymKind(sym.Type),
		})
	}

	for _, fx := range fixups {
		kind, ok := machoRelocKind(fx.kind)
		if !ok {
			return macho.Section{}, &Error{
				Section: string(s.name), Offset: fx.off, HasOff: true,
				Err: fmt.Errorf("%w: %s is marked mapped and has no objectfile/macho kind",
					ErrReloc, RelocName(fx.kind)),
			}
		}
		out.Relocs = append(out.Relocs, macho.Reloc{
			Offset: uint32(fx.off),
			Symbol: fx.symbol,
			Kind:   kind,
			Addend: machoAddend(fixup{
				size: fx.size, tail: fx.tail, pcrel: fx.pcrel, addend: fx.addend,
			}),
		})
	}

	return out, nil
}

// machoSectionKind maps a section name to objectfile/macho's kind.
//
// Mach-O names a section with a `segment,section` pair, and both halves
// matter: `__TEXT,__const` and `__DATA,__const` are different sections with
// the same second name. A name written that way goes through verbatim as
// SectionCustom. A name written the ELF way — `.text`, `.rodata` — maps to
// the kind, and objectfile/macho supplies the pair.
//
// What is refused is a name that is neither: a bare `__const` names a
// section in a segment nobody stated, and picking one would be guessing
// between two real answers.
func machoSectionKind(n SectionName) (macho.SectionKind, string, error) {
	switch n {
	case Text:
		return macho.SectionText, "", nil
	case Data:
		return macho.SectionData, "", nil
	case Rodata:
		return macho.SectionROData, "", nil
	case BSS:
		return macho.SectionBSS, "", nil
	}

	s := string(n)
	switch s {
	case ".eh_frame":
		return macho.SectionCustom, "__TEXT,__eh_frame", nil
	case ".init_array":
		return macho.SectionInitArray, "", nil
	case ".fini_array":
		return macho.SectionFiniArray, "", nil
	case ".tdata", ".tbss":
		return macho.SectionTLS, "", nil
	case "__TEXT,__text":
		return macho.SectionText, "", nil
	case "__DATA,__data":
		return macho.SectionData, "", nil
	case "__TEXT,__const":
		return macho.SectionROData, "", nil
	case "__DATA,__bss":
		return macho.SectionBSS, "", nil
	case "__TEXT,__unwind_info":
		return macho.SectionUnwind, "", nil
	}

	if strings.Contains(s, ",") {
		return macho.SectionCustom, s, nil
	}
	return 0, "", &Error{
		Section: s,
		Err: fmt.Errorf("%w: a Mach-O section is a segment,section pair, and %q names only one half",
			ErrPlatform, s),
	}
}

func machoBinding(b SymAttr) macho.Binding {
	switch b {
	case Global, Hidden:
		// N_PEXT — private extern, Mach-O's hidden — is a separate bit that
		// objectfile/macho does not expose, so Hidden links as a plain
		// global here the same way it does under ELF.
		return macho.BindingGlobal
	case Weak:
		return macho.BindingWeak
	}
	return macho.BindingLocal
}

// machoSymKind is what nlist_64 does not record.
//
// Mach-O has no function-versus-data distinction beyond N_SECT itself:
// which section a symbol is in *is* the type. objectfile/macho accepts a
// SymbolKind for parity across its four packages and never reads it, so
// this maps for shape and nothing depends on the answer.
func machoSymKind(t SymAttr) macho.SymbolKind {
	if t == Func {
		return macho.SymFunc
	}
	return macho.SymData
}

// machoRelocKind maps this package's kind to objectfile/macho's.
//
// Two are conspicuously absent. X86_64_RELOC_SIGNED is how a %rip-relative
// *data* reference is recorded — `lea rsi, [rip+msg]`, which is in this
// package's own README — and objectfile/macho has no case that emits it.
// X86_64_RELOC_GOT is the same story for a non-load GOT reference. Both are
// marked unmapped in reloc_macho.go, so they are refused at checkFixup with
// a message naming the gap rather than reaching here and falling off the
// end of this switch.
func machoRelocKind(k RelocKind) (macho.RelocKind, bool) {
	switch k {
	case X86_64_RELOC_UNSIGNED:
		return macho.RelocAbs64, true
	case X86_64_RELOC_BRANCH:
		return macho.RelocPCRel32, true
	case X86_64_RELOC_GOT_LOAD:
		return macho.RelocGOTLoad, true
	case X86_64_RELOC_TLV:
		return macho.RelocTLSGD, true
	}
	return 0, false
}