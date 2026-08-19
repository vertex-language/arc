package operand

// PrfOp is PRFM's five-bit operand: what to prefetch, into which cache level,
// and whether the line is expected to be reused.
//
// The field is three fields. Bits 4:3 are the type — load, instruction, store —
// bits 2:1 are the cache level, and bit 0 is the retention policy. That is why
// the names are mechanical: PLDL1KEEP is prefetch-for-load, level one, keep,
// and every combination of the three has a name.
type PrfOp uint8

// The three components.
const (
	PrfLoad  = 0 // PLD
	PrfInstr = 1 // PLI
	PrfStore = 2 // PST

	PrfL1 = 0
	PrfL2 = 1
	PrfL3 = 2

	PrfKeep   = 0
	PrfStrm   = 1
)

// NewPrfOp composes an operand from its three parts.
func NewPrfOp(typ, level, policy uint8) PrfOp {
	return PrfOp(typ&3<<3 | level&3<<1 | policy&1)
}

func (p PrfOp) Type() uint8   { return uint8(p) >> 3 & 3 }
func (p PrfOp) Level() uint8  { return uint8(p) >> 1 & 3 }
func (p PrfOp) Policy() uint8 { return uint8(p) & 1 }

// The named operands.
const (
	PLDL1KEEP PrfOp = 0
	PLDL1STRM PrfOp = 1
	PLDL2KEEP PrfOp = 2
	PLDL2STRM PrfOp = 3
	PLDL3KEEP PrfOp = 4
	PLDL3STRM PrfOp = 5

	PLIL1KEEP PrfOp = 8
	PLIL1STRM PrfOp = 9
	PLIL2KEEP PrfOp = 10
	PLIL2STRM PrfOp = 11
	PLIL3KEEP PrfOp = 12
	PLIL3STRM PrfOp = 13

	PSTL1KEEP PrfOp = 16
	PSTL1STRM PrfOp = 17
	PSTL2KEEP PrfOp = 18
	PSTL2STRM PrfOp = 19
	PSTL3KEEP PrfOp = 20
	PSTL3STRM PrfOp = 21

	prfOpCount = 32
)

// Valid reports whether p is in the field. Every value from 0 to 31 encodes;
// Named is the narrower question.
func (p PrfOp) Valid() bool { return p < prfOpCount }

// Named reports whether the architecture spells this combination. Level 4 and
// the unallocated type are encodable and unnamed, and PRFM writes them as a
// bare number the same way DMB does.
func (p PrfOp) Named() bool { return p.Valid() && p.Level() != 3 && p.Type() != 3 }

var prfType = [4]string{"pld", "pli", "pst", ""}

func (p PrfOp) String() string {
	if !p.Named() {
		return "#" + itoa(int(p))
	}
	s := prfType[p.Type()] + "l" + itoa(int(p.Level())+1)
	if p.Policy() == PrfStrm {
		return s + "strm"
	}
	return s + "keep"
}

// LookupPrfOp resolves a prefetch operand name.
func LookupPrfOp(name string) (PrfOp, bool) {
	s := lower(name)
	for p := PrfOp(0); p < prfOpCount; p++ {
		if p.Named() && p.String() == s {
			return p, true
		}
	}
	return 0, false
}