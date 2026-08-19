package operand

// Cond is a condition code: the four-bit field B.cond, CSEL, CSINC and the
// rest select on.
//
// The values are the encoding's own. The low three bits name the test and the
// low bit inverts it, which is what makes Invert a single xor and what makes
// AL and NV — 0b1110 and 0b1111 — the pair where the inversion has nowhere to
// go: both always execute.
type Cond uint8

const (
	EQ Cond = 0  // equal — Z set
	NE Cond = 1  // not equal
	CS Cond = 2  // carry set — unsigned higher or same
	CC Cond = 3  // carry clear — unsigned lower
	MI Cond = 4  // minus — N set
	PL Cond = 5  // plus — N clear
	VS Cond = 6  // overflow set
	VC Cond = 7  // overflow clear
	HI Cond = 8  // unsigned higher
	LS Cond = 9  // unsigned lower or same
	GE Cond = 10 // signed greater or equal
	LT Cond = 11 // signed less than
	GT Cond = 12 // signed greater than
	LE Cond = 13 // signed less or equal
	AL Cond = 14 // always
	NV Cond = 15 // always — see below

	condCount = 16
)

// HS and LO are the unsigned spellings of CS and CC. They are the same
// encoding, and the architecture's preferred disassembly is CS and CC, so
// String never prints these; Lookup accepts them because source writes them.
const (
	HS = CS
	LO = CC
)

// Valid reports whether c names a condition.
//
// NV is included. It is a real encoding that behaves exactly as AL, and a
// disassembler that refused it would fail on a word the hardware executes. The
// architecture's own advice is not to write it, which is a matter for whoever
// is printing rather than for whether the value exists.
func (c Cond) Valid() bool { return c < condCount }

// Invert is the condition that holds exactly when this one does not.
//
// AL and NV are their own opposites in the encoding — both always execute — so
// this reports false for them rather than returning a value that would make
// b.al and its "inverse" the same branch. A caller inverting a branch has to
// know that an unconditional one cannot be inverted.
func (c Cond) Invert() (Cond, bool) {
	if c >= AL {
		return c, false
	}
	return c ^ 1, true
}

// Signed reports whether the condition tests a signed comparison, which is the
// GE/LT/GT/LE group. It is what a printer needs to explain a word and what a
// diagnostic needs to say "you compared signed and branched unsigned".
func (c Cond) Signed() bool {
	switch c {
	case GE, LT, GT, LE:
		return true
	}
	return false
}

var condName = [condCount]string{
	"eq", "ne", "cs", "cc", "mi", "pl", "vs", "vc",
	"hi", "ls", "ge", "lt", "gt", "le", "al", "nv",
}

// String is the architecture's preferred spelling, which is cs and cc rather
// than hs and lo.
func (c Cond) String() string {
	if !c.Valid() {
		return "?"
	}
	return condName[c]
}

// LookupCond resolves a condition name, including the two aliases.
func LookupCond(name string) (Cond, bool) {
	s := lower(name)
	for i, n := range condName {
		if s == n {
			return Cond(i), true
		}
	}
	switch s {
	case "hs":
		return HS, true
	case "lo":
		return LO, true
	}
	return 0, false
}