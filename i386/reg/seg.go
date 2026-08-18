package reg

// Sreg is a segment register.
//
// The order below is the encoding of the ModRM reg field for segment operands
// and is not alphabetical and not the order these are conventionally listed
// in. ES is 0 and DS is 3.
type Sreg uint8

const (
	ES Sreg = iota
	CS
	SS
	DS
	FS
	GS
)

var sregNames = [6]string{"es", "cs", "ss", "ds", "fs", "gs"}

var sregRole = [6]string{"", "", "", "", "", "reserved for the system as the thread-specific data register"}

func (r Sreg) spec() spec {
	return spec{name: sregNames[r], class: ClassSeg, num: uint8(r), root: uint8(r), lo: 0, hi: 16}
}

func (r Sreg) Name() string        { return sregNames[r] }
func (r Sreg) String() string      { return sregNames[r] }
func (r Sreg) Num() uint8          { return uint8(r) }
func (r Sreg) Bits() int           { return 16 }
func (r Sreg) Class() Class        { return ClassSeg }
func (r Sreg) Save() Save          { return SaveNone }
func (r Sreg) Role() string        { return sregRole[r] }
func (r Sreg) Overlaps(o Reg) bool { return overlaps(r, o) }

// Intel386 psABI v1.1, Table 2.14: ES is 40 through GS at 45.
func (r Sreg) DWARF() (int, bool) { return 40 + int(r), true }