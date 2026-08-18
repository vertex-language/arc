package reg

// Control and debug registers. Both are addressed by the ModRM reg field with
// mod fixed at 11, so their numbering is the ordinary 0-7.
//
// Test registers TR0-TR7 are not declared. They existed on the 80386 and
// 80486 and were removed in the Pentium; i386's baseline is i686, so there is
// no silicon in this target's range that implements them. Absent rather than
// empty.

// Cr is a control register.
//
// CR1 and CR5-CR7 are reserved: they are encodable and generate #UD on access.
// They are declared because the encoding exists.
type Cr uint8

const (
	CR0 Cr = iota
	CR1
	CR2
	CR3
	CR4
	CR5
	CR6
	CR7
)

func (r Cr) spec() spec {
	return spec{name: vecName("cr", uint8(r)), class: ClassControl, num: uint8(r), root: uint8(r), lo: 0, hi: 32}
}

func (r Cr) Name() string        { return vecName("cr", uint8(r)) }
func (r Cr) String() string      { return r.Name() }
func (r Cr) Num() uint8          { return uint8(r) }
func (r Cr) Bits() int           { return 32 }
func (r Cr) Class() Class        { return ClassControl }
func (r Cr) Save() Save          { return SaveNone }
func (r Cr) Role() string        { return "" }
func (r Cr) Overlaps(o Reg) bool { return overlaps(r, o) }

func (r Cr) DWARF() (int, bool) { return noDWARF, false }

// Dr is a debug register.
//
// DR4 and DR5 alias DR6 and DR7 when CR4.DE is clear and fault when it is set.
// That is a runtime condition, not an encoding one, so both are declared.
type Dr uint8

const (
	DR0 Dr = iota
	DR1
	DR2
	DR3
	DR4
	DR5
	DR6
	DR7
)

func (r Dr) spec() spec {
	return spec{name: vecName("dr", uint8(r)), class: ClassDebug, num: uint8(r), root: uint8(r), lo: 0, hi: 32}
}

func (r Dr) Name() string        { return vecName("dr", uint8(r)) }
func (r Dr) String() string      { return r.Name() }
func (r Dr) Num() uint8          { return uint8(r) }
func (r Dr) Bits() int           { return 32 }
func (r Dr) Class() Class        { return ClassDebug }
func (r Dr) Save() Save          { return SaveNone }
func (r Dr) Role() string        { return "" }
func (r Dr) Overlaps(o Reg) bool { return overlaps(r, o) }

func (r Dr) DWARF() (int, bool) { return noDWARF, false }