package reg

// Value is anything that can appear as an instruction operand: a register, an
// immediate, a memory operand, a label, or a symbol reference.
//
// The interface is sealed. Registers implement isValue here; the operand types
// in i386/operand join by embedding Seal, whose method belongs to this
// package. Nothing outside those two packages can be a Value, which is what
// keeps Emit's variadic from accepting an arbitrary type.
//
// The seal lives here rather than in operand because reg is the lower package.
// operand imports reg, so a seal declared above could never be satisfied by a
// register — and a register is the operand this tree is built out of.
type Value interface {
	// Bits is the operand's width, or 0 where it has none. A Label and a bare
	// symbol reference have no width.
	Bits() int

	isValue()
}

// Seal is embedded by the operand types in i386/operand to join Value. It is
// zero-sized and carries no behaviour.
type Seal struct{}

func (Seal) isValue() {}

func (R32) isValue()  {}
func (R16) isValue()  {}
func (R8) isValue()   {}
func (Sreg) isValue() {}
func (St) isValue()   {}
func (Mm) isValue()   {}
func (Xmm) isValue()  {}
func (Ymm) isValue()  {}
func (Zmm) isValue()  {}
func (K) isValue()    {}
func (Cr) isValue()   {}
func (Dr) isValue()   {}

// The RM markers declare which registers may appear in a ModRM r/m field, and
// at which width. They are exported because i386/operand's memory types must
// carry the same marker, and an unexported method could not cross the package
// line. They do not weaken the seal: an r/m interface embeds Value, so a type
// outside these two packages cannot satisfy it whatever markers it declares.
func (R8) RM8()      {}
func (R16) RM16()    {}
func (R32) RM32()    {}
func (Mm) RM64()     {}
func (Xmm) RM128()   {}
func (Ymm) RM256()   {}
func (Zmm) RM512()   {}
func (K) RMMask()    {}