package operand

import "github.com/vertex-language/arc/i386/reg"

// Memory is any memory operand, at any access width. It exists so that isa/
// and encode/ can accept "a memory operand" where a form takes one without
// enumerating the eight width types — LEA's operand is an address, not an
// access, and has no width at all.
//
// The interface is closed in practice: every method below is promoted from
// the unexported mem struct, which only this package can embed.
//
// The accessor names here are IndexReg, Displacement and Symbol rather than
// Index, Disp and Sym, because M8..M512 already use those three names for
// the chained builders (Mem32(reg.EAX).Disp(8).Index(reg.ECX, 4)) and a
// method declared directly on a type always shadows a promoted method of the
// same name, whatever its signature. Base needs no such rename since it has
// no builder counterpart, and Seg was already distinct from the Segment
// builder before this interface existed.
type Memory interface {
	Operand

	Base() (reg.R32, bool)
	IndexReg() (reg.R32, uint8, bool)
	Displacement() int32
	Symbol() (SymRef, bool)
	Seg() (reg.Sreg, bool)
	DefaultSeg() reg.Sreg
	Err() error
}

var (
	_ Memory = M8{}
	_ Memory = M16{}
	_ Memory = M32{}
	_ Memory = M64{}
	_ Memory = M80{}
	_ Memory = M128{}
	_ Memory = M256{}
	_ Memory = M512{}
)