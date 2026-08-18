package decode

import (
	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

// Inst is one decoded instruction: the form the bytes name, the operand
// values they carry, and the prefixes in front of them.
//
// Ops is what the encoding holds, not what the assembler was given. For every
// slot but one the two are the same value. The exception is a branch
// displacement: the source said `jne retry` and the bytes say `-11`, and the
// name is not recoverable from the encoding — so a rel slot holds the
// displacement as an operand.Imm and Rel holds it again as a number. arc dis
// on an object recovers the name from the symbol table or the relocation at
// that offset, which is work done above this package because it needs a file.
// A decoder given seven bytes has no name to give back and will not invent
// one.
//
// A consequence worth stating: Ops does not necessarily satisfy Form.Matches.
// Matches asks the encode-direction question — may these operands be this
// form — and a displacement is not a Label.
//
// There is no String method. Printing an instruction means choosing a dialect,
// and this package does not know that dialects exist.
type Inst struct {
	// Form is the form the bytes decoded to. It is never an alias form: an
	// alias emits its target, so a listing says what the silicon does.
	Form *isa.Form

	// Ops are the operand values, in the form's source order.
	Ops []operand.Operand

	// Prefixes are the legacy prefixes that preceded the opcode. A segment
	// override is also applied to the memory operand, where there is one.
	Prefixes Prefixes

	// Bytes is the instruction, aliasing the input slice.
	Bytes []byte

	// Rel is the branch displacement, when the form has one.
	Rel    int32
	HasRel bool
}

// Len is the encoded length in bytes.
func (i Inst) Len() int { return len(i.Bytes) }

// Mnemonic is the form's, lowercase and canonical.
func (i Inst) Mnemonic() string {
	if i.Form == nil {
		return ""
	}
	return i.Form.Mnemonic
}

// Target resolves a branch displacement against the address this instruction
// starts at. The processor computes the target from the address of the next
// instruction, which is why the length is added.
func (i Inst) Target(pc uint32) (uint32, bool) {
	if !i.HasRel {
		return 0, false
	}
	return pc + uint32(i.Len()) + uint32(i.Rel), true
}

// Prefixes are the legacy prefixes, one field per group.
//
// Group 1 is lock and the two repeat prefixes, group 2 the segment overrides,
// group 3 the operand-size override, group 4 the address-size override —
// which arc rejects rather than models, so it has no field here.
type Prefixes struct {
	Lock   bool
	Rep    bool // 0xf3
	RepNE  bool // 0xf2
	OpSize bool // 0x66

	Seg    reg.Sreg
	HasSeg bool
}

// Any reports whether any prefix was present.
func (p Prefixes) Any() bool {
	return p.Lock || p.Rep || p.RepNE || p.OpSize || p.HasSeg
}