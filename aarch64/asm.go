package aarch64

import (
	"encoding/binary"

	"github.com/vertex-language/arc/aarch64/encode"
	"github.com/vertex-language/arc/aarch64/isa"
	"github.com/vertex-language/arc/aarch64/operand"
)

// The builder API.
//
// A section is a byte buffer and a fixup list. Nothing survives an Emit call:
// the operands are lowered, the word is placed, and what is left is bytes and
// the fields that could not be filled yet. There is no instruction list, no
// basic block, and nothing to walk afterward — which is what makes this not an
// IR.

// Assembler accumulates sections and symbols for one object.
type Assembler struct {
	platform Platform
	features FeatureSet

	sections []*Section
	byName   map[string]*Section

	symbols []*Symbol
	symByName map[string]*Symbol

	// baseAddr is the load address of a flat image. It is a usage error on
	// every other platform: a relocatable object does not have one.
	baseAddr    uint64
	hasBaseAddr bool

	// variantPCS names the functions marked STO_AARCH64_VARIANT_PCS.
	variantPCS map[string]bool

	// err is the first failure. Every later builder call is a no-op and
	// Serialize returns it.
	err error
}

func newAssembler(p Platform, c config) *Assembler {
	return &Assembler{
		platform:   p,
		features:   c.features,
		byName:     map[string]*Section{},
		symByName:  map[string]*Symbol{},
		variantPCS: map[string]bool{},
	}
}

// Platform is the object format this assembler writes.
func (a *Assembler) Platform() Platform { return a.platform }

// Features is the active feature set. It cannot change: options are read at
// New, and a set that changed halfway through would make a gating diagnostic
// unfalsifiable.
func (a *Assembler) Features() FeatureSet { return a.features }

// Err reports the first failure, for a caller that wants to stop before
// Serialize.
func (a *Assembler) Err() error { return a.err }

// fail records the first error and makes every later call a no-op.
//
// This is not error swallowing. The offset in the diagnostic is the one the
// failing instruction would have been written at — the same place a per-call
// error would have named — and every subsequent call being a no-op is what
// keeps a section from filling with bytes written after a mistake.
func (a *Assembler) fail(err error) {
	if a.err == nil && err != nil {
		a.err = err
	}
}

// SectionKind is what a section holds, for the conventions a name does not
// state.
type SectionKind uint8

const (
	// KindData is bytes with no special meaning: the default.
	KindData SectionKind = iota

	// KindCode is instructions. Align pads it with NOP rather than zeros, and
	// on ELF the mapping symbols $x and $d are emitted through it.
	KindCode

	// KindBSS has a size and no bytes in the file. Appending content to one is
	// refused rather than written somewhere the format has no room for.
	KindBSS
)

// The conventional section names.
const (
	Text   = ".text"
	Data   = ".data"
	Rodata = ".rodata"
	BSS    = ".bss"
)

// Section is a byte buffer, a fixup list, and the symbols defined in it.
type Section struct {
	a *Assembler

	// Name is the section name. On Mach-O it is a segment,section pair.
	Name string

	// Kind is what the section holds.
	Kind SectionKind

	// Code is the accumulated bytes.
	Code []byte

	// Fixups are the fields left blank, in the order they were placed.
	Fixups []Fixup

	// Align is the section's own alignment, the largest any Align call asked
	// for.
	Alignment int

	// marks records code/data transitions for the ELF mapping symbols. It is
	// filled here because only the builder knows where they are; the writer
	// turns them into $x and $d.
	marks []mark
}

type mark struct {
	offset int
	code   bool
}

// Symbol is a name with a place.
type Symbol struct {
	Name    string
	Section *Section
	Offset  int
	Binding Binding
	Type    SymType

	// Size is the symbol's extent, set explicitly or closed at Serialize.
	Size    int
	HasSize bool

	// Defined distinguishes a definition from a declaration. A declared symbol
	// is referenced and not defined here, which is what an external is.
	Defined bool
}

// Binding is a symbol's linkage.
type Binding uint8

const (
	Local Binding = iota
	Global
	Weak
	Hidden
)

func (b Binding) String() string {
	switch b {
	case Global:
		return "global"
	case Weak:
		return "weak"
	case Hidden:
		return "hidden"
	}
	return "local"
}

// SymType is what a symbol names.
type SymType uint8

const (
	None SymType = iota
	Func
	Object
	TLS
)

func (t SymType) String() string {
	switch t {
	case Func:
		return "func"
	case Object:
		return "object"
	case TLS:
		return "tls"
	}
	return "none"
}

// Section returns the section with this name, creating it if needed.
//
// Sections come out in creation order with the alignment asked for. Address
// assignment is linker/'s; there is no layout engine here.
func (a *Assembler) Section(name string) *Section {
	if s, ok := a.byName[name]; ok {
		return s
	}
	s := &Section{a: a, Name: name, Kind: kindOf(name), Alignment: 1}
	a.sections = append(a.sections, s)
	a.byName[name] = s
	return s
}

// SectionNamed is Section with an explicit name that the conventions do not
// cover, such as a Mach-O segment,section pair written out in full.
func (a *Assembler) SectionNamed(name string) *Section { return a.Section(name) }

// Sections returns every section in creation order.
func (a *Assembler) Sections() []*Section { return a.sections }

// kindOf applies the naming conventions, which are conventions and not rules:
// SetCode states the answer for a name they do not cover.
func kindOf(name string) SectionKind {
	switch name {
	case Text, "__TEXT,__text":
		return KindCode
	case BSS, "__DATA,__bss":
		return KindBSS
	}
	return KindData
}

// SetCode states whether a section holds instructions, for a name the
// conventions do not cover.
func (s *Section) SetCode(code bool) {
	if code {
		s.Kind = KindCode
		return
	}
	s.Kind = KindData
}

// Len is the number of bytes written so far, which is the offset the next
// thing lands at.
func (s *Section) Len() int { return len(s.Code) }

// Align pads to a boundary.
//
// A code section is padded with NOP and everything else with zeros. Only one
// no-op is needed here, because every instruction is one word and there is no
// question of where a decoder resumes.
//
// An alignment that is not a multiple of four on a code section is refused
// rather than rounded: rounding would put a partial instruction in a section
// something is going to disassemble.
func (s *Section) Align(n int) {
	if s.a.err != nil {
		return
	}
	if n <= 0 || n&(n-1) != 0 {
		s.a.fail(wrap(s.Name, s.Len(),
			&PlatformError{s.a.platform, "alignment is not a power of two", ""}))
		return
	}
	if s.Kind == KindCode && n%4 != 0 {
		s.a.fail(wrap(s.Name, s.Len(), &PlatformError{s.a.platform,
			"code alignment must be a multiple of 4",
			"every instruction is one word, and padding to a finer boundary would " +
				"put a partial instruction in the section"}))
		return
	}
	if n > s.Alignment {
		s.Alignment = n
	}

	pad := (n - s.Len()%n) % n
	if pad == 0 {
		return
	}
	if s.Kind == KindCode {
		s.Code = encode.Pad(s.Code, pad)
		return
	}
	s.Code = append(s.Code, make([]byte, pad)...)
}

// Label defines a symbol at the current offset.
//
// The binding and type arrive in one variadic list because Label("_start",
// Global, Func) reads better than two named parameters one of which is almost
// always the zero value. Setting two bindings is the second one winning, which
// is what a .globl after a .weak does in gas.
func (s *Section) Label(name string, attrs ...any) {
	if s.a.err != nil {
		return
	}
	sym := &Symbol{
		Name: name, Section: s, Offset: s.Len(), Defined: true,
	}
	for _, at := range attrs {
		switch v := at.(type) {
		case Binding:
			sym.Binding = v
		case SymType:
			sym.Type = v
		default:
			s.a.fail(wrap(s.Name, s.Len(), &PlatformError{s.a.platform,
				"Label takes a Binding and a SymType", ""}))
			return
		}
	}
	s.a.define(sym)
}

// define records a symbol, refusing a redefinition.
//
// Two definitions of one name is a question the object format cannot record
// and the linker would answer arbitrarily, so it is an error here rather than
// a choice made silently below.
func (a *Assembler) define(sym *Symbol) {
	if prev, ok := a.symByName[sym.Name]; ok && prev.Defined {
		a.fail(&Error{Section: sym.Section.Name, Offset: sym.Offset,
			Err: &PlatformError{a.platform,
				"redefinition of " + sym.Name,
				"first defined in " + prev.Section.Name}})
		return
	}
	if prev, ok := a.symByName[sym.Name]; ok {
		// Upgrading a declaration to a definition keeps the binding the
		// declaration stated, which is what .globl before a label does.
		if sym.Binding == Local {
			sym.Binding = prev.Binding
		}
		*prev = *sym
		return
	}
	a.symByName[sym.Name] = sym
	a.symbols = append(a.symbols, sym)
}

// Declare records a symbol referenced here and defined elsewhere.
func (a *Assembler) Declare(name string, b Binding) {
	if a.err != nil {
		return
	}
	if s, ok := a.symByName[name]; ok {
		s.Binding = b
		return
	}
	s := &Symbol{Name: name, Binding: b}
	a.symByName[name] = s
	a.symbols = append(a.symbols, s)
}

// SetSize states a symbol's extent explicitly, overriding the closing
// Serialize would do.
func (a *Assembler) SetSize(name string, size int) {
	if a.err != nil {
		return
	}
	s, ok := a.symByName[name]
	if !ok {
		a.fail(&UndefinedError{Symbol: name})
		return
	}
	s.Size, s.HasSize = size, true
}

// SetVariantPCS marks a function that does not follow the base procedure call
// standard.
//
// It is ELF-only and it cannot be inferred from the bytes. The linker needs it
// to avoid inserting a stub that clobbers registers the base standard says are
// free, and nothing about the instructions says whether a function's interface
// uses scalable registers.
func (a *Assembler) SetVariantPCS(name string) {
	if a.err != nil {
		return
	}
	a.variantPCS[name] = true
}

// SetBaseAddress sets the load address of a flat image.
//
// It is a usage error on every other platform, because a relocatable object
// does not have one — the linker assigns addresses, and stating one here would
// be stating something the format has nowhere to record.
func (a *Assembler) SetBaseAddress(addr uint64) {
	if a.err != nil {
		return
	}
	if a.platform != Flat {
		a.fail(&PlatformError{a.platform,
			"SetBaseAddress is meaningful only for a flat image",
			"a relocatable object's addresses are the linker's to assign"})
		return
	}
	a.baseAddr, a.hasBaseAddr = addr, true
}

// Emit assembles one instruction into the section.
//
// It resolves the form from isa.Resolve against the active feature set and
// encodes it. Nothing survives the call: the operands are lowered, the word is
// appended, and the fixups the encoder could not fill are recorded with the
// offset they were placed at.
//
// Builder calls return nothing. The first failure is kept with its section and
// offset, every later call is a no-op, and Serialize returns it.
func (s *Section) Emit(mnem string, ops ...any) {
	if s.a.err != nil {
		return
	}
	s.markCode()

	off := s.Len()
	word, fx, err := encode.EncodeWith(Opts{Offset: off}, s.a.features, mnem, ops...)
	if err != nil {
		s.a.fail(wrap(s.Name, off, err))
		return
	}
	s.word(word)
	s.Fixups = append(s.Fixups, fx...)
}

// EmitForm is Emit against a form the caller already resolved, which is what
// the generated helpers and the text path use.
func (s *Section) EmitForm(f *isa.Form, ops ...any) {
	if s.a.err != nil {
		return
	}
	s.markCode()

	off := s.Len()
	word, fx, err := encode.EncodeForm(f, ops, Opts{Offset: off})
	if err != nil {
		s.a.fail(wrap(s.Name, off, err))
		return
	}
	s.word(word)
	s.Fixups = append(s.Fixups, fx...)
}

// Word appends a raw instruction word: what .inst states.
//
// This is the one case where emitting bytes nobody selected is exactly what was
// asked for. It states a word rather than naming an instruction, so no form is
// resolved and no feature is gated.
func (s *Section) Word(w uint32) {
	if s.a.err != nil {
		return
	}
	s.markCode()
	s.word(w)
}

func (s *Section) word(w uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], w)
	s.Code = append(s.Code, b[:]...)
}

// markCode and markData record a transition, for the ELF mapping symbols.
//
// AAELF64 requires $x where code begins and $d where data does, and an object
// without them disassembles wrong — so they are not something a caller states
// and not something a caller can suppress. Recording the transitions is the
// builder's job because only it knows where they are.
func (s *Section) markCode() { s.mark(true) }
func (s *Section) markData() { s.mark(false) }

func (s *Section) mark(code bool) {
	if s.Kind != KindCode {
		return
	}
	if n := len(s.marks); n > 0 && s.marks[n-1].code == code {
		return
	}
	s.marks = append(s.marks, mark{offset: s.Len(), code: code})
}

// Data.

// Byte, Half, Word and Quad append little-endian integers of 1, 2, 4 and 8
// bytes.
//
// The names are the architecture's rather than gas's inherited ones: .word is
// four bytes here and .xword or .dword is eight, which is the opposite of
// x86's convention and the reason this package does not reuse x86_64's
// spelling.
func (s *Section) Byte(v ...uint8) {
	if !s.writable() {
		return
	}
	s.markData()
	s.Code = append(s.Code, v...)
}

func (s *Section) Half(v ...uint16) {
	if !s.writable() {
		return
	}
	s.markData()
	for _, x := range v {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], x)
		s.Code = append(s.Code, b[:]...)
	}
}

func (s *Section) Word32(v ...uint32) {
	if !s.writable() {
		return
	}
	s.markData()
	for _, x := range v {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], x)
		s.Code = append(s.Code, b[:]...)
	}
}

func (s *Section) Quad(v ...uint64) {
	if !s.writable() {
		return
	}
	s.markData()
	for _, x := range v {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], x)
		s.Code = append(s.Code, b[:]...)
	}
}

// Ascii appends a string's bytes with no terminator; Asciz adds one.
func (s *Section) Ascii(str string) {
	if !s.writable() {
		return
	}
	s.markData()
	s.Code = append(s.Code, str...)
}

func (s *Section) Asciz(str string) {
	s.Ascii(str)
	s.Byte(0)
}

// Fill appends count items of size bytes, each holding value.
func (s *Section) Fill(count, size int, value uint64) {
	if !s.writable() {
		return
	}
	s.markData()
	for i := 0; i < count; i++ {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], value)
		if size > 8 {
			s.Code = append(s.Code, b[:]...)
			s.Code = append(s.Code, make([]byte, size-8)...)
			continue
		}
		s.Code = append(s.Code, b[:size]...)
	}
}

// Zero appends n zero bytes. It is the one data call a BSS section accepts,
// because a BSS section has a size and no bytes in the file.
func (s *Section) Zero(n int) {
	if s.a.err != nil {
		return
	}
	s.Code = append(s.Code, make([]byte, n)...)
}

// WordRef and QuadRef append a symbol's address, recording an absolute fixup.
//
// This is the one place data carries a relocation, and how a jump table or a
// pointer table is expressed. The builder API covers the symbolic-data case
// directly, which is why the gap Assemble has is in the text path only.
func (s *Section) WordRef(t Target) { s.dataRef(t, 4) }
func (s *Section) QuadRef(t Target) { s.dataRef(t, 8) }

func (s *Section) dataRef(t Target, size int) {
	if !s.writable() {
		return
	}
	s.markData()

	off := s.Len()
	s.Code = append(s.Code, make([]byte, size)...)

	fx := Fixup{
		Offset: off,
		Target: t,
		Role:   operand.RoleDirect,
		Bits:   uint8(size * 8),
	}
	if ref, ok := t.(SymRef); ok {
		fx.Kind, fx.Addend = ref.Kind, ref.Addend
	}
	s.Fixups = append(s.Fixups, fx)
}

// writable reports whether content may be appended, failing if not.
func (s *Section) writable() bool {
	if s.a.err != nil {
		return false
	}
	if s.Kind == KindBSS {
		s.a.fail(wrap(s.Name, s.Len(), &PlatformError{s.a.platform,
			"a bss section takes Zero and nothing else",
			"it has a size and no bytes in the file, so there is nowhere to put content"}))
		return false
	}
	return true
}