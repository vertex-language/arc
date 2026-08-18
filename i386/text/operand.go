package text

import "github.com/vertex-language/arc/i386/reg"

// Inst is an instruction statement: a mnemonic, an optional legacy prefix, an
// optional size forced by the syntax (a GAS suffix or a NASM keyword), and its
// operands in the order isa.Form declares them — NASM's order, since gas
// reverses at its own edges and hands this package an already-corrected
// slice.
type Inst struct {
	Common
	Mnemonic string
	Size     Width
	Prefix   Prefix
	Ops      []Operand
}

func (*Inst) node() {}

// Operand is one operand of an instruction. The set is closed: a register, an
// immediate (which doubles as a branch displacement — neither syntax marks
// the difference; the mnemonic does), an indirect wrapper (GAS's *%eax and
// *foo; NASM has no sigil and prints the inner operand bare), and a memory
// reference.
type Operand interface {
	Pos() Pos
	operand()
}

// Reg is a bare register operand.
type Reg struct {
	P Pos
	R reg.Reg
}

func (o Reg) Pos() Pos { return o.P }
func (Reg) operand()   {}

// Imm is an immediate value, or — in a branch mnemonic's operand slot — the
// branch displacement itself. GAS spells the former with a '$' and the latter
// with nothing; NASM spells neither with a sigil. Both arrive here as the
// same shape, and it is the mnemonic, not the operand, that says which.
type Imm struct {
	P Pos
	X Expr
}

func (o Imm) Pos() Pos { return o.P }
func (Imm) operand()   {}

// Indirect is GAS's '*' indirect-operand marker: *%eax, *foo. NASM has no
// sigil for this and its printer renders the inner operand bare, which is
// what makes an indirect call parsed from GAS print correctly as NASM.
type Indirect struct {
	P Pos
	X Operand
}

func (o Indirect) Pos() Pos { return o.P }
func (Indirect) operand()   {}

// Mem is a memory operand: an optional segment override, an optional base
// and scaled index, and an optional displacement expression. At least one of
// Base, Index, or Disp must be present — an empty () is rejected at parse.
type Mem struct {
	P Pos

	HasSeg bool
	Seg    reg.Sreg

	HasBase bool
	Base    reg.R32

	HasIndex bool
	Index    reg.R32
	Scale    uint8

	Disp Expr
}

func (o Mem) Pos() Pos { return o.P }
func (Mem) operand()   {}