package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/encode"
	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
)

// Assembler, Section, Align, Label, Data emission, and Emit — the hand-built
// half of the builder API. code.go forwards the typed-form/decode surface;
// write.go turns what a Section accumulated here into bytes.

// Assembler is one object under construction.
type Assembler struct {
	platform Platform
	features feature.Set

	sections []*Section
	byKind   map[SectionKind]*Section
	byName   map[string]*Section

	baseAddr uint32
	hasBase  bool

	err *Error // first error from any section; sticky
}

// Platform is the target this Assembler was built for.
func (a *Assembler) Platform() Platform { return a.platform }

// Features is the active feature set every Emit and typed helper is gated
// against.
func (a *Assembler) Features() feature.Set { return a.features }

// SetBaseAddress fixes the address a flat image starts loading at — the
// boot-sector 0x7C00 in docs/builder.md's own example. It has no effect
// outside Flat; write_flat.go is what reads it.
func (a *Assembler) SetBaseAddress(addr uint32) { a.baseAddr, a.hasBase = addr, true }

// Err is the first error from any section, or nil.
func (a *Assembler) Err() error {
	if a.err == nil {
		return nil
	}
	return a.err
}

// Sections is every section, in creation order — the order they are written
// in, which write.go relies on rather than re-deriving.
func (a *Assembler) Sections() []*Section { return a.sections }

// SectionKind is the portable meaning of a section. The nine kinds and their
// order are pinned to text.SectionKind's own, so the two translate by a bare
// cast; one round-trip test stands in for the switch that would otherwise be
// copied.
type SectionKind uint8

const (
	Text SectionKind = iota
	Data
	ROData
	BSS
	Unwind
	InitArray
	FiniArray
	TLS
	Custom
)

var sectionKindNames = [...]string{
	"text", "data", "rodata", "bss", "unwind", "init_array", "fini_array", "tls", "custom",
}

func (k SectionKind) String() string {
	if int(k) < len(sectionKindNames) {
		return sectionKindNames[k]
	}
	return "?"
}

// Code reports whether the kind holds instructions. Align pads a code
// section with the nop table and a data section with zeros — the one place
// this distinction changes bytes.
func (k SectionKind) Code() bool { return k == Text }

var defaultSectionName = map[SectionKind]string{
	Text: ".text", Data: ".data", ROData: ".rodata", BSS: ".bss",
	Unwind: ".eh_frame", InitArray: ".init_array", FiniArray: ".fini_array", TLS: ".tdata",
}

// Section returns the section of kind k, creating it with a portable
// default name on first call. Two calls with the same kind return the same
// *Section.
func (a *Assembler) Section(k SectionKind) *Section {
	if s, ok := a.byKind[k]; ok {
		return s
	}
	name := defaultSectionName[k]
	if name == "" {
		name = "." + k.String()
	}
	s := a.newSection(k, name)
	if a.byKind == nil {
		a.byKind = map[SectionKind]*Section{}
	}
	a.byKind[k] = s
	return s
}

// SectionNamed passes name through verbatim — the one call that is not
// portable across platforms, since "__DATA,__objc_classlist" is not
// ".text.hot" and arc will not pretend otherwise. Two calls with the same
// name return the same *Section.
func (a *Assembler) SectionNamed(name string) *Section {
	if s, ok := a.byName[name]; ok {
		return s
	}
	kind := Custom
	for k, n := range defaultSectionName {
		if n == name {
			kind = k
			break
		}
	}
	return a.newSection(kind, name)
}

func (a *Assembler) newSection(k SectionKind, name string) *Section {
	s := &Section{
		a: a, Kind: k, Name: name,
		marks:   map[string]uint32{},
		symbols: map[string]*symbolInfo{},
	}
	a.sections = append(a.sections, s)
	if a.byName == nil {
		a.byName = map[string]*Section{}
	}
	a.byName[name] = s
	return s
}

// Section is one section of the object under construction: a byte buffer and
// a fixup list, and nothing that survives past Serialize.
type Section struct {
	a    *Assembler
	Kind SectionKind
	Name string

	bytes  []byte
	fixups []fixupEntry

	// marks are every label in this section, bare or attributed — the fixup
	// target namespace. symbols is the subset that got at least one
	// attribute, promoted out of "a name for an offset" into something the
	// object's symbol table records.
	marks   map[string]uint32
	symbols map[string]*symbolInfo
}

type symbolInfo struct {
	offset    uint32
	attrs     LabelAttr
	typ       SymbolType
	dllExport bool
}

// fixupEntry is a field a placed instruction left for write.go to resolve.
// It carries exactly what encode.Fixup does, plus the absolute offset within
// the section's byte buffer.
type fixupEntry struct {
	offset int
	size   int
	pcRel  bool
	adjust int32
	kind   encode.FixupKind
	name   string
	reloc  RelocKind
	addend int32
}

func (s *Section) fail(format string, args ...any) {
	if s.a.err != nil {
		return
	}
	s.a.err = &Error{
		Section: s.Name, Offset: uint32(len(s.bytes)), Err: ErrForm,
		msg: fmt.Sprintf(format, args...),
	}
}

func (s *Section) failErr(e *Error) {
	if s.a.err != nil || e == nil {
		return
	}
	if e.Section == "" {
		e.Section = s.Name
	}
	s.a.err = e
}

// Align pads the section to an n-byte boundary. A code section is padded
// with the arch's multi-byte nop sequence, gated by the same feature set
// everything else in this Assembler is; a data section is padded with
// zeros. Padding .text with 0x00 produces a listing that disassembles into
// garbage, which is the whole reason this distinction exists.
func (s *Section) Align(n int) *Section {
	if s.a.err != nil {
		return s
	}
	if n <= 0 || n&(n-1) != 0 {
		s.fail("Align(%d): alignment is not a power of two", n)
		return s
	}
	pad := (n - len(s.bytes)%n) % n
	if pad == 0 {
		return s
	}
	if s.Kind.Code() {
		s.bytes = append(s.bytes, encode.Nops(pad, s.a.features)...)
	} else {
		s.bytes = append(s.bytes, make([]byte, pad)...)
	}
	return s
}

// LabelAttr is one attribute passed to Label: a binding, a symbol type, or
// DLLExport. Label rejects more than one binding-free type in a single call
// rather than silently keeping the last.
type LabelAttr uint16

const (
	Global LabelAttr = 1 << iota
	Local
	Weak
	Hidden
	Protected
	Internal
	Extern

	typeFunc
	typeObject
	typeTLS

	// DLLExport is declared here because i386 has a COFF platform; it is an
	// error on this package's ELF target, checked at the Label call rather
	// than by the type system, the same way docs/builder.md describes it.
	DLLExport
)

// Func, Object and ThreadLocal are the symbol-type attributes. ThreadLocal
// is spelled apart from the TLS SectionKind above so the two families of
// constant never collide under one name.
const (
	Func        = typeFunc
	Object      = typeObject
	ThreadLocal = typeTLS
)

const typeMask = typeFunc | typeObject | typeTLS

// SymbolType is what the object file records a symbol as. It has no
// exported constructor outside the Func/Object/ThreadLocal attributes above;
// write.go reads it off symbolInfo directly.
type SymbolType uint8

const (
	symNone SymbolType = iota
	symFunc
	symObject
	symTLS
)

func symbolTypeOf(a LabelAttr) SymbolType {
	switch a {
	case Func:
		return symFunc
	case Object:
		return symObject
	case ThreadLocal:
		return symTLS
	}
	return symNone
}

// Label names the current offset. With no attributes it is a fixup target
// only, resolvable by a branch in the same section and present in no symbol
// table. Attach any attribute and it becomes a symbol as well, with Offset
// at the current position; Size is closed by write.go at the next symbol or
// the end of the section, which is .type/.size pairing without the two
// directives.
func (s *Section) Label(name string, attrs ...LabelAttr) *Section {
	if s.a.err != nil {
		return s
	}
	if _, dup := s.marks[name]; dup {
		s.fail("Label(%q): already defined in this section", name)
		return s
	}
	off := uint32(len(s.bytes))
	s.marks[name] = off

	if len(attrs) == 0 {
		return s
	}

	var bind LabelAttr
	var typ SymbolType
	seenType, dllExport := false, false

	for _, a := range attrs {
		switch {
		case a == DLLExport:
			if s.a.platform != COFF {
				s.fail("DLLExport is a COFF attribute; target is %s", s.a.platform)
				return s
			}
			dllExport = true
		case a&typeMask != 0:
			if seenType {
				s.fail("Label(%q): more than one symbol type given", name)
				return s
			}
			seenType = true
			typ = symbolTypeOf(a)
		default:
			bind |= a
		}
	}

	s.symbols[name] = &symbolInfo{offset: off, attrs: bind, typ: typ, dllExport: dllExport}
	return s
}

// Byte, Long, Quad, Ascii, Asciz, Zero and Bytes: the seven data calls. There
// is no Word — it is two bytes here and four on ARM, AArch64 and RISC-V, and
// the Go API does not offer a name whose width depends on which package you
// are in.

func (s *Section) Byte(v ...uint8) *Section {
	if s.a.err != nil {
		return s
	}
	s.bytes = append(s.bytes, v...)
	return s
}

func (s *Section) Long(v ...uint32) *Section {
	if s.a.err != nil {
		return s
	}
	for _, x := range v {
		s.bytes = append(s.bytes, byte(x), byte(x>>8), byte(x>>16), byte(x>>24))
	}
	return s
}

func (s *Section) Quad(v ...uint64) *Section {
	if s.a.err != nil {
		return s
	}
	for _, x := range v {
		for i := 0; i < 8; i++ {
			s.bytes = append(s.bytes, byte(x>>(8*uint(i))))
		}
	}
	return s
}

func (s *Section) Ascii(str string) *Section {
	if s.a.err != nil {
		return s
	}
	s.bytes = append(s.bytes, str...)
	return s
}

func (s *Section) Asciz(str string) *Section {
	if s.a.err != nil {
		return s
	}
	s.bytes = append(s.bytes, str...)
	s.bytes = append(s.bytes, 0)
	return s
}

func (s *Section) Zero(n int) *Section {
	if s.a.err != nil {
		return s
	}
	if n < 0 {
		s.fail("Zero(%d): negative count", n)
		return s
	}
	s.bytes = append(s.bytes, make([]byte, n)...)
	return s
}

func (s *Section) Bytes(b []byte) *Section {
	if s.a.err != nil {
		return s
	}
	s.bytes = append(s.bytes, b...)
	return s
}

// Emit resolves the form from isa/ — the same table the typed helpers are
// generated from — choosing the shortest legal encoding and breaking ties by
// table order. Nothing survives the call: bytes and fixups are appended
// before it returns.
func (s *Section) Emit(mnemonic string, ops ...Operand) *Section {
	if s.a.err != nil {
		return s
	}

	match, gated := isa.Resolve(mnemonic, s.a.features, ops)
	if len(match) == 0 {
		if len(gated) > 0 {
			f := gated[0]
			need := feature.New(f.Level)
			if f.HasFeat {
				need = need.Add(f.Feat)
			}
			s.failErr(featureErr(s.Name, uint32(len(s.bytes)), mnemonic,
				s.a.features.Missing(need), s.a.features))
			return s
		}
		s.failErr(formErr(s.Name, uint32(len(s.bytes)), mnemonic))
		return s
	}

	var best *isa.Form
	var bestInst encode.Inst
	for _, f := range match {
		inst, err := encode.Encode(f, ops)
		if err != nil {
			continue
		}
		if best == nil || inst.Len() < bestInst.Len() {
			best, bestInst = f, inst
		}
	}
	if best == nil {
		s.failErr(formErr(s.Name, uint32(len(s.bytes)), mnemonic))
		return s
	}

	return s.place(bestInst)
}

// place appends an already-encoded instruction and translates its fixups to
// section-absolute offsets. Typed helpers, once generated, call this too —
// it is the one seam every encoding this package produces passes through.
func (s *Section) place(inst encode.Inst) *Section {
	base := len(s.bytes)
	s.bytes = append(s.bytes, inst.Bytes...)
	for _, fx := range inst.Fixups {
		s.fixups = append(s.fixups, fixupEntry{
			offset: base + fx.Offset, size: fx.Size, pcRel: fx.PCRel, adjust: fx.Adjust,
			kind: fx.Kind, name: fx.Name, reloc: fx.Reloc, addend: fx.Addend,
		})
	}
	return s
}