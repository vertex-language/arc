package operand

import "github.com/vertex-language/arc/i386/reg"

// Memory is any memory operand, at any access width. It exists so that isa/
// and encode/ can accept "a memory operand" where a form takes one without
// enumerating the eight width types — LEA's operand is an address, not an
// access, and has no width at all.
//
// The interface is closed in practice: every method below is promoted from
// the unexported mem struct, which only this package can embed.
type Memory interface {
	Operand

	Base() (reg.R32, bool)
	Index() (reg.R32, uint8, bool)
	Disp() int32
	Sym() (SymRef, bool)
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