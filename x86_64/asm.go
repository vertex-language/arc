// x86_64/asm.go
package x86_64

import (
	"encoding/binary"
	"fmt"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/text"
)

// Assembler is one object under construction: a list of sections in creation
// order, a symbol table, and the feature set every encoding is gated on.
//
// Errors are collected rather than returned. A builder call returns nothing
// because the typed helper layer is twelve thousand methods and an error
// return on each is an error return checked at no call site — the pattern
// this tree would grow is `t.MovR64Imm64(...)` twelve times with twelve
// ignored errors, which is worse than not offering them. The first failure
// is kept with the section and offset it happened at, every later call is a
// no-op, and Serialize returns it. Err reports it early for a caller who
// wants to stop sooner.
//
// This is not error swallowing: the offset in the diagnostic is the one the
// failing instruction would have been written at, which is the same place a
// per-call error would have named.
type Assembler struct {
	cfg config

	sections []*Section
	byName   map[SectionName]*Section

	// syms is every symbol this object knows, in definition order. Order is
	// the object file's symbol table order, so it is creation order here
	// too — an ELF writer sorts locals ahead of globals and needs a stable
	// input to do it from.
	syms   []*Symbol
	symsBy map[string]*Symbol

	err error
}

func newAssembler(c config) *Assembler {
	return &Assembler{
		cfg:    c,
		byName: make(map[SectionName]*Section, 8),
		symsBy: make(map[string]*Symbol, 32),
	}
}

// Platform is the object format this assembler writes.
func (a *Assembler) Platform() Platform { return a.cfg.platform }

// Features is the feature set every encoding is gated on. It was settled at
// New and there is no setter: a set that changed halfway through an object
// would make every gating diagnostic unfalsifiable.
func (a *Assembler) Features() FeatureSet { return a.cfg.features }

// Err is the first error, or nil. Serialize returns the same value.
func (a *Assembler) Err() error { return a.err }

// fail records the first error and makes every later call a no-op.
func (a *Assembler) fail(err error) {
	if a.err == nil && err != nil {
		a.err = err
	}
}

// SetBaseAddress sets the load address a Flat image starts at — the 0x7C00
// of a boot sector.
//
// It is an error anywhere else. ELF, COFF and Mach-O all have a header field
// for a load address and none of them lets an object file state one; an
// object is relocatable and the address is the linker's. Accepting the call
// and ignoring it would be a silent lie about where the code will run.
func (a *Assembler) SetBaseAddress(addr uint64) {
	if a.cfg.platform != Flat {
		a.fail(platformErr(a.cfg.platform,
			"a base address is a property of a loaded image, and a relocatable "+
				"object does not have one; only Flat takes SetBaseAddress"))
		return
	}
	a.cfg.base = addr
}

// BaseAddress is the Flat load address, or zero.
func (a *Assembler) BaseAddress() uint64 { return a.cfg.base }

// ---- sections ----------------------------------------------------------

// SectionName is a section's name as the object format spells it.
//
// On ELF and COFF that is a plain name. On Mach-O it is a `segment,section`
// pair and goes through verbatim: __TEXT,__text is two names and one string,
// and inventing the segment from the section is how __DATA,__const ends up
// in __TEXT.
type SectionName string

// The four sections every object has a name for. Mach-O's spellings are
// different strings for the same things and are mapped in write_macho.go,
// which is where the format's vocabulary lives.
const (
	Text   SectionName = ".text"
	Data   SectionName = ".data"
	Rodata SectionName = ".rodata"
	BSS    SectionName = ".bss"
)

// Section is one section: a byte buffer, a fixup list, and the symbols
// defined inside it.
//
// That is the whole of it. There is no statement list, no instruction
// objects, no tree — Emit appends bytes and nothing survives the call, which
// is what makes a section a thing you can reason about the size of.
type Section struct {
	a     *Assembler
	name  SectionName
	index int

	// code marks a section Align pads with no-ops rather than zeros. It is
	// derived from the name, because that is the only thing a caller states
	// — and a zero-padded code section decodes as `add [rax], al` repeated,
	// which is a real instruction and a confusing disassembly.
	code bool

	// nobits marks a section with no contents in the file: .bss. Emitting
	// bytes into one is refused rather than written, because the format has
	// nowhere to put them.
	nobits bool

	align  int
	data   []byte
	fixups []fixup
	syms   []*Symbol
}

// Section returns the section by name, creating it on first use.
//
// Sections come out in creation order, all the way down to objectfile/. This
// is not a layout decision — address assignment is linker/'s — it is the
// absence of one: nothing here reorders, so the order you asked for is the
// order you get.
func (a *Assembler) Section(name SectionName) *Section {
	if s, ok := a.byName[name]; ok {
		return s
	}
	s := &Section{
		a:      a,
		name:   name,
		index:  len(a.sections),
		code:   isCodeSection(name),
		nobits: isNobitsSection(name),
		align:  1,
	}
	a.sections = append(a.sections, s)
	a.byName[name] = s
	return s
}

// SectionNamed is Section for a name the format spells and this package has
// no constant for: a Mach-O `segment,section` pair, an ELF section with a
// suffix, a debug section.
func (a *Assembler) SectionNamed(name string) *Section { return a.Section(SectionName(name)) }

// Sections is every section, in creation order.
func (a *Assembler) Sections() []*Section { return a.sections }

// isCodeSection reports whether a name is executable, by the conventions all
// three formats share. A caller with a section this does not recognize can
// say so with SetCode.
func isCodeSection(n SectionName) bool {
	switch n {
	case Text:
		return true
	}
	s := string(n)
	return hasPrefix(s, ".text") || hasPrefix(s, "__TEXT,__text") || hasPrefix(s, ".init") ||
		hasPrefix(s, ".fini") || hasPrefix(s, ".plt")
}

func isNobitsSection(n SectionName) bool {
	if n == BSS {
		return true
	}
	s := string(n)
	return hasPrefix(s, ".bss") || hasPrefix(s, "__DATA,__bss") || hasPrefix(s, ".tbss")
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// SetCode states whether this section holds instructions, for a name the
// conventions do not cover. It decides what Align pads with and nothing else.
func (s *Section) SetCode(code bool) { s.code = code }

// Name is the section's name as the format spells it.
func (s *Section) Name() SectionName { return s.name }

// Offset is the current end of the section, which is where the next byte
// goes. A caller computing a symbol's size reads it twice and subtracts.
func (s *Section) Offset() int { return len(s.data) }

// Size is Offset. A nobits section has a size and no data, and this is the
// number the format records.
func (s *Section) Size() int { return len(s.data) }

// Alignment is the strongest alignment asked for in this section.
func (s *Section) Alignment() int { return s.align }

// Bytes is the section's contents. The slice is the section's own and
// appending to it corrupts the object; copy before keeping it.
func (s *Section) Bytes() []byte { return s.data }

// Align pads to a multiple of n and raises the section's own alignment.
//
// A code section pads with the canonical multi-byte no-ops, which is what
// GNU as emits and therefore what the differential suite compares against.
// Anything else pads with zeros. A nobits section only advances, since there
// is nothing to write.
func (s *Section) Align(n int) {
	if s.a.err != nil {
		return
	}
	if n <= 0 || n&(n-1) != 0 {
		s.a.fail(&Error{
			Section: string(s.name), Offset: len(s.data), HasOff: true,
			Err: fmt.Errorf("%w: alignment must be a power of two, got %d", ErrForm, n),
		})
		return
	}
	if n > s.align {
		s.align = n
	}

	pad := (n - len(s.data)%n) % n
	if pad == 0 {
		return
	}
	if s.nobits {
		s.data = append(s.data, make([]byte, pad)...)
		return
	}
	if s.code {
		s.data = append(s.data, encode.Nops(pad)...)
		return
	}
	s.data = append(s.data, make([]byte, pad)...)
}

// ---- symbols -----------------------------------------------------------

// SymAttr is a symbol attribute: a binding or a type.
//
// One type carries both axes because Label takes them variadically and a
// caller writes `Label("_start", Global, Func)` rather than naming two
// parameters, one of which is almost always the zero value. The axes are
// separate ranges and setting two bindings is the second one winning, which
// is what a .globl after a .weak does in gas.
type SymAttr uint8

const (
	// Bindings. Local is the default and is worth writing when a name would
	// otherwise be exported by convention.
	Local SymAttr = iota
	Global
	Weak

	// Hidden is a binding modifier rather than a binding: a hidden symbol is
	// global to the link and invisible outside the image.
	Hidden

	// Types. None is the default.
	None
	Func
	Object
	TLS
)

func (t SymAttr) isBinding() bool { return t <= Hidden }

// Symbol is a name this object defines or references.
type Symbol struct {
	Name string

	// Section is where the symbol is defined, or nil for an undefined one.
	Section *Section

	// Offset is the position within Section.
	Offset int

	Binding SymAttr
	Type    SymAttr

	// Size is the symbol's extent in bytes. It is set explicitly by SetSize
	// or closed at Serialize, where the distance to the next symbol in the
	// section is finally known.
	Size    int64
	HasSize bool

	// Defined distinguishes a symbol at offset zero of a section from one
	// nothing defines. Section != nil answers it, and this is the field the
	// writers read so the question is asked one way everywhere.
	Defined bool
}

// Label defines a symbol at the current offset.
//
//	t.Label("_start", x86_64.Global, x86_64.Func)
//
// Redefining a name is an error. Two definitions of one symbol is a question
// the object format cannot record and the linker would answer arbitrarily.
func (s *Section) Label(name string, attrs ...SymAttr) *Symbol {
	if s.a.err != nil {
		return nil
	}
	if prev, ok := s.a.symsBy[name]; ok && prev.Defined {
		s.a.fail(&Error{
			Section: string(s.name), Offset: len(s.data), HasOff: true,
			Err: fmt.Errorf("%w: %s is already defined at %s+%#x",
				ErrForm, name, prev.Section.name, prev.Offset),
		})
		return prev
	}

	sym := s.a.symbol(name)
	sym.Section = s
	sym.Offset = len(s.data)
	sym.Defined = true
	applyAttrs(sym, attrs)

	s.syms = append(s.syms, sym)
	return sym
}

// Declare names a symbol this object references and does not define. It is
// the .extern of the builder API, and it exists so a reference to a name
// nothing defines is a deliberate act rather than a Serialize-time surprise.
func (a *Assembler) Declare(name string, attrs ...SymAttr) *Symbol {
	sym := a.symbol(name)
	if !sym.Defined && len(attrs) == 0 {
		sym.Binding = Global
	}
	applyAttrs(sym, attrs)
	return sym
}

// Symbol returns the named symbol, defined or not, creating a reference to
// it if this is the first mention.
func (a *Assembler) Symbol(name string) *Symbol { return a.symbol(name) }

// Symbols is every symbol, in the order it was first mentioned.
func (a *Assembler) Symbols() []*Symbol { return a.syms }

func (a *Assembler) symbol(name string) *Symbol {
	if s, ok := a.symsBy[name]; ok {
		return s
	}
	s := &Symbol{Name: name, Binding: Local, Type: None}
	a.syms = append(a.syms, s)
	a.symsBy[name] = s
	return s
}

func applyAttrs(s *Symbol, attrs []SymAttr) {
	for _, t := range attrs {
		if t.isBinding() {
			s.Binding = t
			continue
		}
		s.Type = t
	}
}

// SetSize states a symbol's extent explicitly, for a caller who knows it
// before the next symbol arrives.
func (a *Assembler) SetSize(name string, size int64) {
	s := a.symbol(name)
	s.Size, s.HasSize = size, true
}

// ---- instructions ------------------------------------------------------

// Emit assembles one instruction, resolving the shortest legal encoding.
//
// It never picks a different instruction. If no form of this mnemonic takes
// these operands that is an error, not a relaxation into one that does — and
// among the forms that do match, the shortest wins, with ties broken toward
// the earlier row of the table. If you care which encoding you get, the
// typed helper layer is what names one.
//
// Operands are in Intel order: destination first.
func (s *Section) Emit(mnemonic string, ops ...any) {
	s.EmitWith(encode.Opts{}, mnemonic, ops...)
}

// EmitWith is Emit with the EVEX modifiers set: zeroing, broadcast,
// embedded rounding, suppress-all-exceptions.
//
// The writemask is not here. A mask names a register, the form declares a
// slot for it in EVEX.aaa, and it is passed as an operand — `EmitWith(o,
// "vpaddd", ZMM0, K1, ZMM1, ZMM2)`. The modifiers in Opts are one bit each
// with no register behind them, which is why they are a separate parameter.
func (s *Section) EmitWith(o encode.Opts, mnemonic string, ops ...any) {
	if s.a.err != nil {
		return
	}
	off := len(s.data)

	args, err := encode.Args(ops...)
	if err != nil {
		s.a.fail(at(string(s.name), off, s.a.cfg.features, err))
		return
	}
	f, err := isa.Resolve(s.a.cfg.features, mnemonic, args...)
	if err != nil {
		s.a.fail(at(string(s.name), off, s.a.cfg.features, err))
		return
	}
	s.put(f, o, ops...)
}

// put encodes a form that has already been chosen.
//
// This is what the generated helper layer calls: a helper knows its form at
// compile time, so it skips Resolve entirely and there is nothing left to
// choose. It is also where the feature gate is checked for that path —
// Resolve does it for Emit, and a helper that bypassed Resolve would
// otherwise bypass the gate with it.
func (s *Section) put(f *isa.Form, o encode.Opts, ops ...any) {
	if s.a.err != nil {
		return
	}
	off := len(s.data)

	if s.nobits {
		s.a.fail(&Error{
			Section: string(s.name), Offset: off, HasOff: true,
			Err: fmt.Errorf("%w: %s holds no bytes in the file", ErrPlatform, s.name),
		})
		return
	}
	if !f.Enabled(s.a.cfg.features) {
		s.a.fail(at(string(s.name), off, s.a.cfg.features, &isa.GateError{
			Mnemonic: f.Op, Need: f.Need, Active: s.a.cfg.features,
		}))
		return
	}

	b, fx, err := encode.EncodeWith(f, o, ops...)
	if err != nil {
		s.a.fail(at(string(s.name), off, s.a.cfg.features, err))
		return
	}

	for _, x := range fx {
		s.fixups = append(s.fixups, fixup{
			off:     off + x.Offset,
			size:    x.Size,
			tail:    x.Tail,
			pcrel:   x.PCRel,
			use:     x.Use,
			kind:    x.Kind,
			target:  x.Target,
			addend:  x.Addend,
			instOff: off,
		})
		s.a.symbol(x.Target.SymName())
	}
	s.data = append(s.data, b...)
}

// fixup is a field this section left blank because its value is an address
// that is not yet a number. It is encode.Fixup with the section's base added
// and the source position, if there was one, attached.
//
// It is still not a relocation. The mapping to R_X86_64_*,
// IMAGE_REL_AMD64_* or X86_64_RELOC_* is the platform writer's, because the
// constants are — and so is the conversion from the logical addend written
// here to the raw one the format wants.
type fixup struct {
	off   int
	size  int
	tail  int
	pcrel bool

	use    encode.Use
	kind   RelocKind
	target Target
	addend int64

	// instOff is the start of the instruction the field belongs to, for a
	// diagnostic that wants to name the instruction rather than the field.
	instOff int

	// pos is the source position, for a fixup that came from text. Zero for
	// one built through the typed API, which has no line to name.
	pos text.Pos
}

// ---- data --------------------------------------------------------------

// WriteBytes appends raw bytes.
func (s *Section) WriteBytes(b []byte) {
	if !s.writable(len(b)) {
		return
	}
	s.data = append(s.data, b...)
}

// Byte, Word, Long and Quad append little-endian integers.
//
// The names are gas's and the widths are the architecture's, which is why
// Word is two bytes here and the machine is sixty-four bits wide: .word
// predates long mode and neither assembler renamed it.
func (s *Section) Byte(vs ...uint8) {
	if !s.writable(len(vs)) {
		return
	}
	s.data = append(s.data, vs...)
}

func (s *Section) Word(vs ...uint16) {
	if !s.writable(2 * len(vs)) {
		return
	}
	var b [2]byte
	for _, v := range vs {
		binary.LittleEndian.PutUint16(b[:], v)
		s.data = append(s.data, b[:]...)
	}
}

func (s *Section) Long(vs ...uint32) {
	if !s.writable(4 * len(vs)) {
		return
	}
	var b [4]byte
	for _, v := range vs {
		binary.LittleEndian.PutUint32(b[:], v)
		s.data = append(s.data, b[:]...)
	}
}

func (s *Section) Quad(vs ...uint64) {
	if !s.writable(8 * len(vs)) {
		return
	}
	var b [8]byte
	for _, v := range vs {
		binary.LittleEndian.PutUint64(b[:], v)
		s.data = append(s.data, b[:]...)
	}
}

// Ascii appends a string's bytes with no terminator.
func (s *Section) Ascii(str string) {
	if !s.writable(len(str)) {
		return
	}
	s.data = append(s.data, str...)
}

// Asciz appends a string's bytes and a terminating zero.
func (s *Section) Asciz(str string) {
	if !s.writable(len(str) + 1) {
		return
	}
	s.data = append(s.data, str...)
	s.data = append(s.data, 0)
}

// Zero appends n zero bytes. In a nobits section this is the only thing that
// can be appended, and it advances the size without writing anything.
func (s *Section) Zero(n int) {
	if n <= 0 {
		return
	}
	if s.a.err != nil {
		return
	}
	s.data = append(s.data, make([]byte, n)...)
}

// Fill appends count elements of the given width, each holding value. It is
// gas's .fill and NASM's `times n dq v`.
func (s *Section) Fill(count int, width int, value uint64) {
	if count <= 0 {
		return
	}
	switch width {
	case 1, 2, 4, 8:
	default:
		s.a.fail(&Error{
			Section: string(s.name), Offset: len(s.data), HasOff: true,
			Err: fmt.Errorf("%w: a fill element is 1, 2, 4 or 8 bytes, got %d", ErrForm, width),
		})
		return
	}
	if !s.writable(count * width) {
		return
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], value)
	for i := 0; i < count; i++ {
		s.data = append(s.data, b[:width]...)
	}
}

// LongRef and QuadRef append a symbol's address as data, recording an
// absolute fixup.
//
// This is the path a jump table or a pointer table needs, and it is the one
// place data carries a relocation. It is available here and not yet through
// text: `.quad . - msg` needs the same fixup and Assemble refuses it with a
// specific error rather than writing eight wrong bytes, because the residue
// analysis exists in text/ and the backpatch that consumes it does not.
func (s *Section) LongRef(t Target) { s.dataRef(t, 4) }

// QuadRef appends a symbol's full 64-bit address.
func (s *Section) QuadRef(t Target) { s.dataRef(t, 8) }

func (s *Section) dataRef(t Target, size int) {
	if !s.writable(size) {
		return
	}
	if t == nil {
		s.a.fail(&Error{
			Section: string(s.name), Offset: len(s.data), HasOff: true,
			Err: fmt.Errorf("%w: a data reference needs a target", ErrUndefined),
		})
		return
	}
	s.fixups = append(s.fixups, fixup{
		off:     len(s.data),
		size:    size,
		tail:    0,
		pcrel:   false,
		use:     encode.UseAbs,
		kind:    t.Reloc(),
		target:  t,
		addend:  targetAddend(t),
		instOff: len(s.data),
	})
	s.a.symbol(t.SymName())
	s.data = append(s.data, make([]byte, size)...)
}

func targetAddend(t Target) int64 {
	if r, ok := t.(SymRef); ok {
		return r.Addend
	}
	return 0
}

// writable reports whether n bytes may be appended here, recording the error
// if not.
func (s *Section) writable(n int) bool {
	if s.a.err != nil {
		return false
	}
	if s.nobits && n > 0 {
		s.a.fail(&Error{
			Section: string(s.name), Offset: len(s.data), HasOff: true,
			Err: fmt.Errorf("%w: %s holds no bytes in the file; use Zero to reserve space",
				ErrPlatform, s.name),
		})
		return false
	}
	return true
}