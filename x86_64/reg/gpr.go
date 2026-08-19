// x86_64/reg/gpr.go
package reg

// Reg64 is a 64-bit general-purpose register.
type Reg64 uint8

// Reg32 is a 32-bit general-purpose register, the low 32 bits of the matching Reg64.
// Writing to it zero-extends into the 64-bit parent.
type Reg32 uint8

// Reg16 is a 16-bit general-purpose register, the low 16 bits of the matching Reg64.
type Reg16 uint8

// Reg8 is an 8-bit general-purpose register. The legacy high-byte registers
// (AH, CH, DH, BH) share encoding numbers 4-7 with the REX-only low-byte
// registers (SPL, BPL, SIL, DIL), so their Num() alone does not identify them.
type Reg8 uint8

const (
	RAX, EAX, AX, AL = Reg64(0), Reg32(0), Reg16(0), Reg8(0)
	RCX, ECX, CX, CL = Reg64(1), Reg32(1), Reg16(1), Reg8(1)
	RDX, EDX, DX, DL = Reg64(2), Reg32(2), Reg16(2), Reg8(2)
	RBX, EBX, BX, BL = Reg64(3), Reg32(3), Reg16(3), Reg8(3)
	RSP, ESP, SP     = Reg64(4), Reg32(4), Reg16(4)
	RBP, EBP, BP     = Reg64(5), Reg32(5), Reg16(5)
	RSI, ESI, SI     = Reg64(6), Reg32(6), Reg16(6)
	RDI, EDI, DI     = Reg64(7), Reg32(7), Reg16(7)

	R8, R8D, R8W, R8B     = Reg64(8), Reg32(8), Reg16(8), Reg8(8)
	R9, R9D, R9W, R9B     = Reg64(9), Reg32(9), Reg16(9), Reg8(9)
	R10, R10D, R10W, R10B = Reg64(10), Reg32(10), Reg16(10), Reg8(10)
	R11, R11D, R11W, R11B = Reg64(11), Reg32(11), Reg16(11), Reg8(11)
	R12, R12D, R12W, R12B = Reg64(12), Reg32(12), Reg16(12), Reg8(12)
	R13, R13D, R13W, R13B = Reg64(13), Reg32(13), Reg16(13), Reg8(13)
	R14, R14D, R14W, R14B = Reg64(14), Reg32(14), Reg16(14), Reg8(14)
	R15, R15D, R15W, R15B = Reg64(15), Reg32(15), Reg16(15), Reg8(15)

	// The REX-only low byte registers. They encode as 4-7 but require a REX prefix.
	SPL = Reg8(4)
	BPL = Reg8(5)
	SIL = Reg8(6)
	DIL = Reg8(7)

	// The legacy high byte registers. They encode as 4-7 but forbid a REX prefix.
	AH = Reg8(16)
	CH = Reg8(17)
	DH = Reg8(18)
	BH = Reg8(19)
)

func (r Reg64) Num() uint8 { return uint8(r) }
func (r Reg32) Num() uint8 { return uint8(r) }
func (r Reg16) Num() uint8 { return uint8(r) }

// Num for a Reg8 returns the architectural encoding number.
func (r Reg8) Num() uint8 {
	if r >= 16 {
		return uint8(r) - 12 // AH (16) -> 4, CH (17) -> 5, etc.
	}
	return uint8(r)
}

func (Reg64) Bits() int { return 64 }
func (Reg32) Bits() int { return 32 }
func (Reg16) Bits() int { return 16 }
func (Reg8) Bits() int  { return 8 }

func (Reg64) Class() Class { return ClassGP64 }
func (Reg32) Class() Class { return ClassGP32 }
func (Reg16) Class() Class { return ClassGP16 }
func (Reg8) Class() Class  { return ClassGP8 }

func (r Reg64) Loc() Loc { return Loc{FileGPR, uint8(r), 0, 64} }
func (r Reg32) Loc() Loc { return Loc{FileGPR, uint8(r), 0, 32} }
func (r Reg16) Loc() Loc { return Loc{FileGPR, uint8(r), 0, 16} }

func (r Reg8) Loc() Loc {
	if r >= 16 {
		return Loc{FileGPR, uint8(r) - 16, 8, 16} // AH, CH, DH, BH
	}
	return Loc{FileGPR, uint8(r), 0, 8} // AL, CL, etc.
}

// Parent returns the 64-bit register that contains this register.
func (r Reg32) Parent() Reg64 { return Reg64(r) }
func (r Reg16) Parent() Reg64 { return Reg64(r) }
func (r Reg8) Parent() Reg64 {
	return Reg64(r.Loc().Index)
}

// RexRequired reports whether the register requires a REX prefix to be addressable.
func (r Reg8) RexRequired() bool {
	return r == SPL || r == BPL || r == SIL || r == DIL || r >= 8 && r <= 15
}

// RexForbidden reports whether the register cannot be addressed if a REX prefix is present.
func (r Reg8) RexForbidden() bool {
	return r >= 16 // AH, CH, DH, BH
}