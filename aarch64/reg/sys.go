package reg

// Sys is a system register, named by the five-field coordinate
// {op0, op1, CRn, CRm, op2} and stored packed into sixteen bits in that order.
//
// The packing is the conventional one — op0 at bit 14, op1 at 11, CRn at 7,
// CRm at 3, op2 at 0 — which makes the numeric order the same as the order the
// generic S<op0>_<op1>_<Cn>_<Cm>_<op2> syntax writes the fields in.
type Sys uint16

// NewSys builds a system register from its five fields. Out-of-range fields are
// truncated to their encodings; validity is a question for whoever supplied
// them.
func NewSys(op0, op1, crn, crm, op2 uint8) Sys {
	return Sys(uint16(op0&0x3)<<14 |
		uint16(op1&0x7)<<11 |
		uint16(crn&0xf)<<7 |
		uint16(crm&0xf)<<3 |
		uint16(op2&0x7))
}

func (s Sys) Op0() uint8 { return uint8(s>>14) & 0x3 }
func (s Sys) Op1() uint8 { return uint8(s>>11) & 0x7 }
func (s Sys) CRn() uint8 { return uint8(s>>7) & 0xf }
func (s Sys) CRm() uint8 { return uint8(s>>3) & 0xf }
func (s Sys) Op2() uint8 { return uint8(s) & 0x7 }

func (s Sys) Num() uint16  { return uint16(s) }
func (s Sys) Bits() uint16 { return 64 }
func (s Sys) Class() Class { return ClassSys }

// Movable reports whether this register is reachable by MRS and MSR (register).
//
// Those instructions carry a single o0 bit and compute op0 as 2+o0, so they
// reach op0 of 2 and 3 only. The rest of the encoding space — op0 of 0 and 1 —
// is the cache and TLB maintenance, address translation and prediction
// restriction instructions, which are SYS and SYSL and a different form
// entirely. A caller that hands one of those to an MRS form has not made a
// spelling mistake, and the diagnostic should not suggest it has.
func (s Sys) Movable() bool { return s.Op0() >= 2 }

// The named system registers.
//
// This is a starting set, not the architecture's full table: the complete list
// runs to thousands of entries across every extension and is generated from
// Arm's own machine-readable register description. Anything absent is still
// reachable through NewSys and through the generic S3_0_c0_c0_0 spelling that
// Lookup accepts, which is exactly what that syntax is for.
const (
	// Special-purpose and process state.
	NZCV      = Sys(3<<14 | 3<<11 | 4<<7 | 2<<3 | 0)
	DAIF      = Sys(3<<14 | 3<<11 | 4<<7 | 2<<3 | 1)
	CurrentEL = Sys(3<<14 | 0<<11 | 4<<7 | 2<<3 | 2)
	SPSel     = Sys(3<<14 | 0<<11 | 4<<7 | 2<<3 | 0)

	// Floating point control and status.
	FPCR = Sys(3<<14 | 3<<11 | 4<<7 | 4<<3 | 0)
	FPSR = Sys(3<<14 | 3<<11 | 4<<7 | 4<<3 | 1)

	// Thread ID.
	TPIDR_EL0   = Sys(3<<14 | 3<<11 | 13<<7 | 0<<3 | 2)
	TPIDRRO_EL0 = Sys(3<<14 | 3<<11 | 13<<7 | 0<<3 | 3)
	TPIDR_EL1   = Sys(3<<14 | 0<<11 | 13<<7 | 0<<3 | 4)
	TPIDR_EL2   = Sys(3<<14 | 4<<11 | 13<<7 | 0<<3 | 2)
	TPIDR_EL3   = Sys(3<<14 | 6<<11 | 13<<7 | 0<<3 | 2)

	// Identification.
	MIDR_EL1   = Sys(3<<14 | 0<<11 | 0<<7 | 0<<3 | 0)
	MPIDR_EL1  = Sys(3<<14 | 0<<11 | 0<<7 | 0<<3 | 5)
	CTR_EL0    = Sys(3<<14 | 3<<11 | 0<<7 | 0<<3 | 1)
	DCZID_EL0  = Sys(3<<14 | 3<<11 | 0<<7 | 0<<3 | 7)
	CNTVCT_EL0 = Sys(3<<14 | 3<<11 | 14<<7 | 0<<3 | 2)

	// EL1 system control.
	SCTLR_EL1 = Sys(3<<14 | 0<<11 | 1<<7 | 0<<3 | 0)
	TTBR0_EL1 = Sys(3<<14 | 0<<11 | 2<<7 | 0<<3 | 0)
	TTBR1_EL1 = Sys(3<<14 | 0<<11 | 2<<7 | 0<<3 | 1)
	TCR_EL1   = Sys(3<<14 | 0<<11 | 2<<7 | 0<<3 | 2)
	ESR_EL1   = Sys(3<<14 | 0<<11 | 5<<7 | 2<<3 | 0)
	FAR_EL1   = Sys(3<<14 | 0<<11 | 6<<7 | 0<<3 | 0)
	VBAR_EL1  = Sys(3<<14 | 0<<11 | 12<<7 | 0<<3 | 0)
	ELR_EL1   = Sys(3<<14 | 0<<11 | 4<<7 | 0<<3 | 1)
	SPSR_EL1  = Sys(3<<14 | 0<<11 | 4<<7 | 0<<3 | 0)
)