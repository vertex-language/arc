package isa

import "fmt"

// Field is where an operand's bits land in the 32-bit word.
//
// Some fields are split. ADRP puts the low two bits of its immediate at 30:29
// and the remaining nineteen at 23:5, so a field is a list of parts read from
// the value's least significant bit upward. Two parts is the most the
// architecture uses, which keeps Field comparable and copyable.
type Field struct {
	Parts [2]FieldPart
	N     uint8 // number of parts in use
}

// FieldPart is one contiguous run of bits in the word.
type FieldPart struct {
	Lo    uint8 // bit position of the part's least significant bit
	Width uint8
}

// F builds a single-part field.
func F(lo, width uint8) Field {
	return Field{Parts: [2]FieldPart{{lo, width}}, N: 1}
}

// FSplit builds a two-part field. The first part holds the value's low bits.
func FSplit(lo0, w0, lo1, w1 uint8) Field {
	return Field{Parts: [2]FieldPart{{lo0, w0}, {lo1, w1}}, N: 2}
}

// Width is the total number of bits the field holds.
func (f Field) Width() uint8 {
	var n uint8
	for i := uint8(0); i < f.N; i++ {
		n += f.Parts[i].Width
	}
	return n
}

// Mask is the bits of the word this field occupies.
func (f Field) Mask() uint32 {
	var m uint32
	for i := uint8(0); i < f.N; i++ {
		p := f.Parts[i]
		m |= ((1 << p.Width) - 1) << p.Lo
	}
	return m
}

// Put places a value into a word. Bits above the field's width are dropped;
// range checking belongs to encode/, which has the operand and can name it.
func (f Field) Put(word uint32, v uint64) uint32 {
	for i := uint8(0); i < f.N; i++ {
		p := f.Parts[i]
		chunk := uint32(v&((1<<p.Width)-1)) << p.Lo
		word = (word &^ (((1 << p.Width) - 1) << p.Lo)) | chunk
		v >>= p.Width
	}
	return word
}

// Get reads a value out of a word, reassembling a split field.
func (f Field) Get(word uint32) uint64 {
	var v uint64
	var shift uint8
	for i := uint8(0); i < f.N; i++ {
		p := f.Parts[i]
		chunk := uint64((word >> p.Lo) & ((1 << p.Width) - 1))
		v |= chunk << shift
		shift += p.Width
	}
	return v
}

func (f Field) String() string {
	if f.N == 1 {
		p := f.Parts[0]
		if p.Width == 1 {
			return fmt.Sprintf("[%d]", p.Lo)
		}
		return fmt.Sprintf("[%d:%d]", p.Lo+p.Width-1, p.Lo)
	}
	a, b := f.Parts[1], f.Parts[0]
	return fmt.Sprintf("[%d:%d]:[%d:%d]", a.Lo+a.Width-1, a.Lo, b.Lo+b.Width-1, b.Lo)
}

// The fields the architecture names. Every table row refers to these rather
// than to bit positions, so a row reads the way the ARM ARM's encoding diagram
// does.
//
// These are values, not identities: Imm6 and Imms are both F(10, 6) and
// therefore compare equal. Nothing may key a rule on a field for that reason —
// see ImmKind below.
var (
	Rd  = F(0, 5)
	Rn  = F(5, 5)
	Rm  = F(16, 5)
	Ra  = F(10, 5)
	Rt  = F(0, 5)
	Rt2 = F(10, 5)
	Rs  = F(16, 5)

	Imm6  = F(10, 6)
	Imm7  = F(15, 7)
	Imm9  = F(12, 9)
	Imm12 = F(10, 12)
	Imm14 = F(5, 14)
	Imm16 = F(5, 16)
	Imm19 = F(5, 19)
	Imm26 = F(0, 26)

	Immr = F(16, 6)
	Imms = F(10, 6)
	NBit = F(22, 1)

	// The logical immediate is one operand, not three: N:immr:imms is
	// contiguous at 22:10, and the value encode/ computes fills all thirteen
	// bits at once. A row that declared Imms alone would silently drop N and
	// immr, which is why AND, ORR and EOR (immediate) name this field and not
	// that one.
	ImmLogical13 = F(10, 13)

	Hw    = F(21, 2)
	Sh    = F(22, 1)
	Shift = F(22, 2)
	Opt   = F(13, 3)
	Amt   = F(10, 3)

	Cond   = F(0, 4)
	CondHi = F(12, 4)
	Nzcv   = F(0, 4)

	Q    = F(30, 1)
	Size = F(22, 2)
	CRm  = F(8, 4)
	Op2  = F(5, 3)

	// ADR and ADRP: the value's low two bits sit above its high nineteen.
	ImmPCRel = FSplit(29, 2, 5, 19)

	// TBZ and TBNZ: the bit number is split across the word.
	BitPos = FSplit(19, 5, 31, 1)

	// MRS and MSR: o0:op1:CRn:CRm:op2, contiguous but read as one system
	// register number in the order reg.Sys packs it, minus op0's high bit.
	SysReg = F(5, 15)

	// The whole word, for .inst.
	Word32 = F(0, 32)
)

// ImmKind is the arithmetic an immediate goes through on its way into its
// field: which range it must fall in, what it is scaled or rotated by, and
// which sibling field it computes along the way.
//
// It is stated on the slot rather than derived because neither of the two
// things it could be derived from carries it.
//
// The class cannot: ClassImm is one class over every immediate the
// architecture has, and splitting it per rule would put the rule in the
// operand vocabulary, where a caller writing Imm(4096) would have to know
// which kind of immediate the form it has not yet resolved wants.
//
// The field cannot either, and this is the sharper reason. Field is a
// comparable value type, so two fields of the same shape are the same value:
// Imm6 and Imms are both F(10, 6), and a table keyed on the field could not
// tell a plain six-bit constant from the imms half of a bitfield operand.
//
// So the rule is table data, one column of the row that already states the
// mnemonic and the word. encode/imm.go is the switch over it, and it is the
// only switch over it.
type ImmKind uint8

const (
	// ImmNone is a slot that is not an immediate, or a row that has not
	// stated a rule. encode/ treats it as ImmPlain, so an un-annotated row
	// still encodes rather than failing; a row whose immediate needs more
	// than a range check says so.
	ImmNone ImmKind = iota

	// ImmPlain is the value, range-checked against the field's width and
	// written into it. Signed and unsigned are both accepted, because the
	// field's width is the only constraint and a caller may legitimately
	// write either spelling of the same bit pattern.
	ImmPlain

	// ImmAddSub12 is ADD and SUB's twelve unsigned bits with an optional
	// LSL #12. It computes the Sh field, so a form using it needs a shift
	// slot for the computed bit to land in.
	ImmAddSub12

	// ImmMoveWide is MOVZ, MOVN and MOVK's sixteen bits at one of four
	// shifts. It computes the Hw field from where the halfword sits. A value
	// spanning two halfwords has no encoding and is refused rather than
	// expanded into a chain, which would be one mnemonic becoming several.
	ImmMoveWide

	// ImmLogical is the N:immr:imms triple operand/bitmask.go computes:
	// thirteen bits naming a rotated run of ones replicated to fill the
	// register. Whether a constant is expressible at all is that encoder's
	// question, and a constant that is not is a different error from one that
	// is out of range.
	ImmLogical

	// ImmScaled is a load or store offset in units of the access width: the
	// unsigned twelve of the scaled forms, or the signed seven of a pair.
	// Which of the two follows from the field's width, and the access width
	// follows from the form's memory class.
	ImmScaled

	// ImmUnscaled is LDUR and STUR's signed nine bits, in bytes. That it is
	// not scaled is the whole difference between LDUR and LDR, which is why
	// they are separate mnemonics rather than one with two ranges.
	ImmUnscaled

	// ImmBitPos is TBZ and TBNZ's bit number, 0 to 63, into a split field.
	ImmBitPos

	// ImmBranch is a pc-relative displacement in bytes, divided by four. A
	// displacement that is not word-aligned has no encoding.
	ImmBranch

	// ImmPage is ADR and ADRP's twenty-one bits. ADRP counts 4KiB pages;
	// ADR counts bytes, and states ImmPlain on a signed field instead.
	ImmPage

	// ImmRaw32 is .inst: a whole word, stated rather than encoded.
	ImmRaw32

	immKindCount
)

// Valid reports whether k names a rule.
func (k ImmKind) Valid() bool { return k < immKindCount }

// Computes reports whether this rule fills a sibling field the caller may not
// have written — the Sh bit of an ADD, the Hw field of a MOVZ. A form using
// such a rule must declare a shift slot for the value to land in, which is
// what finish checks.
func (k ImmKind) Computes() bool {
	return k == ImmAddSub12 || k == ImmMoveWide
}

func (k ImmKind) String() string {
	switch k {
	case ImmPlain:
		return "plain"
	case ImmAddSub12:
		return "addsub12"
	case ImmMoveWide:
		return "movewide"
	case ImmLogical:
		return "logical"
	case ImmScaled:
		return "scaled"
	case ImmUnscaled:
		return "unscaled"
	case ImmBitPos:
		return "bitpos"
	case ImmBranch:
		return "branch"
	case ImmPage:
		return "page"
	case ImmRaw32:
		return "raw32"
	}
	return "none"
}

// Role is what an operand means, as opposed to where it goes. The platform
// writer reads this to pick a relocation kind; the encoder reads Field.
type Role uint8

const (
	RoleNone Role = iota

	RoleDest   // written
	RoleSrc    // read
	RoleSrcDst // read and written

	RoleBase   // the base register of an address
	RoleIndex  // the index register of an address
	RoleOffset // the displacement of an address

	RoleTarget   // a branch destination
	RolePage     // the page of an address: ADRP
	RolePageOff  // the offset within a page
	RolePred     // a governing predicate
	RoleModifier // a shift, extend, condition or option
)

// Slot is one operand of one form: what it accepts, what it means, where its
// bits go, and — for an immediate — what arithmetic it goes through.
//
// A slot is not an operand. A memory operand fills two of these, a base and an
// offset, which is why encode/ walks slots and arguments with separate indices
// rather than zipping them.
type Slot struct {
	Class Class
	Role  Role
	Field Field

	// Imm is the immediate rule, for a slot whose class is ClassImm. It is
	// ImmNone on every other slot and on a plain constant.
	Imm ImmKind

	// Optional marks a slot the assembly syntax may omit, such as RET's
	// register, which defaults to X30, or a shift that defaults to LSL #0.
	Optional bool

	// Default is the field value used when an Optional slot is omitted.
	Default uint64
}

// IsImm reports whether this slot takes an immediate.
func (s Slot) IsImm() bool { return s.Class == ClassImm }

// Signature is the slot as Form.Signature spells it: the class, the immediate
// rule where one distinguishes the slot, and optionality.
//
// The rule is part of the signature because two forms can be identical in
// class and differ only in it. LSL and LSR both alias UBFM with an Xn, Xn,
// #imm shape and differ only in the immr/imms pair encode/ computes; without
// the rule here they collide in checkTable, and the collision is real rather
// than a naming accident.
func (s Slot) Signature() string {
	out := s.Class.String()
	if s.Imm != ImmNone && s.Imm != ImmPlain {
		out += ":" + s.Imm.String()
	}
	if s.Optional {
		out += "?"
	}
	return out
}