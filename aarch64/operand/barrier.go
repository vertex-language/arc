package operand

// Barrier is the option field of DMB, DSB and ISB: which accesses the barrier
// orders, and in which direction.
//
// The encoding is two independent halves. Bits 3:2 are the domain — outer
// shareable, non-shareable, inner shareable, full system — and bits 1:0 are
// the access types, load and store. That structure is why the names are not an
// arbitrary list: ISH is inner shareable with both bits set, ISHLD is the same
// domain with loads only, and every legal name is one of the domains crossed
// with one of the three access combinations.
type Barrier uint8

const (
	// The domains, at bits 3:2.
	domainOSH Barrier = 2 << 2
	domainNSH Barrier = 3 << 2
	domainISH Barrier = 1 << 2 // 0b01 with the low bits, per the encoding table
	domainSY  Barrier = 3 << 2

	// The access types, at bits 1:0.
	accessLD Barrier = 1
	accessST Barrier = 2
	accessLDST Barrier = 3
)

// The named options, as the architecture's table spells them. The values are
// the field's, taken from that table rather than composed, because the domain
// encoding is not the tidy two bits the names suggest — OSHLD is 1 and OSH is
// 3, and deriving them would get one of the four wrong.
const (
	OSHLD Barrier = 1
	OSHST Barrier = 2
	OSH   Barrier = 3
	NSHLD Barrier = 5
	NSHST Barrier = 6
	NSH   Barrier = 7
	ISHLD Barrier = 9
	ISHST Barrier = 10
	ISH   Barrier = 11
	LD    Barrier = 13
	ST    Barrier = 14
	SY    Barrier = 15

	barrierCount = 16
)

// Valid reports whether b is in the field.
//
// Every value from 0 to 15 encodes. The ones without names — 0, 4, 8, 12 — are
// reserved and behave as SY on the hardware, so they are valid to decode and
// wrong to write; String prints them as a bare number and the parser refuses
// them, which is the split between what exists and what a source file may say.
func (b Barrier) Valid() bool { return b < barrierCount }

// Named reports whether the value is one the architecture spells.
func (b Barrier) Named() bool { return barrierName[b&15] != "" }

var barrierName = [barrierCount]string{
	OSHLD: "oshld", OSHST: "oshst", OSH: "osh",
	NSHLD: "nshld", NSHST: "nshst", NSH: "nsh",
	ISHLD: "ishld", ISHST: "ishst", ISH: "ish",
	LD: "ld", ST: "st", SY: "sy",
}

// String is the option's name, or its number where it has none — dmb #4 is how
// the architecture itself writes a reserved value, and a disassembler that
// printed "sy" for it would be claiming the word says something it does not.
func (b Barrier) String() string {
	if n := barrierName[b&15]; n != "" {
		return n
	}
	return "#" + itoa(int(b&15))
}

// LookupBarrier resolves a barrier option name. It accepts only the named
// options; a reserved value has to be written as the number it is.
func LookupBarrier(name string) (Barrier, bool) {
	s := lower(name)
	for i, n := range barrierName {
		if n != "" && s == n {
			return Barrier(i), true
		}
	}
	return 0, false
}