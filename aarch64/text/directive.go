package text

// Directive is a statement that is not an instruction.
//
// The Kind is the arch's classification of the directive, not the spelling's:
// .globl and .global are one kind with one meaning, and .xword and .dword are
// both an eight-byte datum. That classification is here rather than in the
// syntax package because it is what Assemble dispatches on, and a second
// classification living next to the parser would be a second place for the
// meaning of .section to be decided.
type Directive struct {
	Kind DirKind

	// Spelling is the directive as written, including the leading dot. It is
	// kept so a printer reproduces the file rather than normalizing .global
	// into .globl — a change that assembles identically and that nobody asked
	// for.
	Spelling string

	// Name is the principal string argument: a section name, a symbol name, an
	// architecture name.
	Name string

	// Args are the expression arguments: the values of a data directive, the
	// size of a .space, the alignment of a .align.
	Args []Expr

	// Flags is the raw remainder for directives whose tail is not expressions:
	// a section's flag string and type, a .type's @function.
	Flags []string

	P       Pos
	Comment string
}

func (d *Directive) Pos() Pos { return d.P }
func (*Directive) node()      {}

// DirKind is what a directive does.
//
// The set is closed and every member is something Assemble has a case for or
// deliberately refuses. A directive with no kind is a parse-level unknown, and
// unknown is an error rather than a no-op: silently ignoring a directive
// produces an object that is missing something the source asked for, which is
// the failure mode hardest to notice.
type DirKind uint8

const (
	DirNone DirKind = iota

	// Placement.
	DirSection // .section, .text, .data, .bss, .rodata
	DirAlign   // .align, .balign, .p2align
	DirOrg     // .org — refused; needs an image-layout step

	// Symbols.
	DirBinding    // .globl, .global, .weak, .local, .hidden
	DirType       // .type
	DirSize       // .size
	DirVariantPCS // .variant_pcs

	// Data.
	DirData  // .byte, .hword, .word, .xword, .quad, .ascii, .asciz, .string
	DirSpace // .space, .skip, .zero, .fill

	// Values threaded across statements. All refused: each needs an Env, and
	// Assemble runs with none.
	DirEqu  // .equ, .set, .equiv
	DirComm // .comm, .lcomm

	// Architecture state, the group that is this architecture's own.
	DirArch      // .arch
	DirArchExt   // .arch_extension
	DirCPU       // .cpu
	DirReq       // .req
	DirUnreq     // .unreq

	// Passed through as opaque payloads.
	DirCFI    // .cfi_*
	DirOpaque // .file, .loc, .ident, and the debug directives

	dirKindCount
)

// Refused reports whether Assemble declines this kind, which is what lets a
// caller check a unit before assembling rather than discovering it partway.
//
// Each refusal names what it would have taken, because "unsupported" sends a
// reader looking for a flag that does not exist. .equ and .comm need a value
// threaded across statements; .org needs an image-layout step this tree has no
// linker-free version of.
func (k DirKind) Refused() bool {
	return k == DirEqu || k == DirComm || k == DirOrg
}

// ArchState reports whether the directive changes the accepted instruction set
// or the register vocabulary mid-file.
//
// This is the one place the source overrides what the API fixes at New. It is
// the source's exception rather than the API's: each of these is a statement at
// a line number, so a gating diagnostic still names a flag and still names the
// line that set it, which is the property WithFeatures-only exists to protect.
func (k DirKind) ArchState() bool {
	switch k {
	case DirArch, DirArchExt, DirCPU, DirReq, DirUnreq:
		return true
	}
	return false
}

var dirKindName = [dirKindCount]string{
	DirSection: "section", DirAlign: "align", DirOrg: "org",
	DirBinding: "binding", DirType: "type", DirSize: "size",
	DirVariantPCS: "variant_pcs",
	DirData: "data", DirSpace: "space",
	DirEqu: "equ", DirComm: "comm",
	DirArch: "arch", DirArchExt: "arch_extension", DirCPU: "cpu",
	DirReq: "req", DirUnreq: "unreq",
	DirCFI: "cfi", DirOpaque: "opaque",
}

func (k DirKind) String() string {
	if k >= dirKindCount || dirKindName[k] == "" {
		return "unknown"
	}
	return dirKindName[k]
}

// Width is the datum size of a data directive, in bytes, or 0.
//
// The names are the architecture's rather than gas's inherited ones: .word is
// four bytes here and .xword or .dword is eight, which is the opposite of
// x86's convention and the reason this table is not shared with that tree.
type DataWidth uint8

const (
	DataNone   DataWidth = 0
	DataByte   DataWidth = 1
	DataHalf   DataWidth = 2
	DataWord   DataWidth = 4
	DataDouble DataWidth = 8

	// DataString is a string datum: the bytes as written, with a terminator or
	// without one. It has no fixed width, and Terminated reports which it is.
	DataString DataWidth = 255
)

// Values reports the datum width and count of a data directive.
func (d *Directive) Values() (DataWidth, int) {
	if d.Kind != DirData {
		return DataNone, 0
	}
	return dataWidth(d.Spelling), len(d.Args)
}

func dataWidth(spelling string) DataWidth {
	switch lower(spelling) {
	case ".byte":
		return DataByte
	case ".hword", ".short", ".2byte":
		return DataHalf
	case ".word", ".long", ".int", ".4byte":
		return DataWord
	case ".xword", ".dword", ".quad", ".8byte":
		return DataDouble
	case ".ascii", ".asciz", ".string":
		return DataString
	}
	return DataNone
}

// Terminated reports whether a string directive appends a NUL.
func (d *Directive) Terminated() bool {
	switch lower(d.Spelling) {
	case ".asciz", ".string":
		return true
	}
	return false
}

// Alignment is the byte alignment a .align-family directive asks for, and
// whether the directive states it as a power of two.
//
// The two spellings genuinely disagree: .align on this target takes a byte
// count, .p2align takes an exponent, and .balign takes a byte count on every
// target. Collapsing them at parse time would lose which was written; resolving
// them here keeps one answer and one place it is computed.
func (d *Directive) Alignment(env Env) (int64, error) {
	if d.Kind != DirAlign || len(d.Args) == 0 {
		return 0, nil
	}
	n, err := Eval(d.Args[0], env)
	if err != nil {
		return 0, err
	}
	if lower(d.Spelling) == ".p2align" {
		return int64(1) << uint(n), nil
	}
	return n, nil
}

// Const is the single constant argument of a directive that takes one: a
// .size, a .space, a .fill's count.
func (d *Directive) Const(env Env) (int64, error) {
	if len(d.Args) == 0 {
		return 0, nil
	}
	return Eval(d.Args[0], env)
}