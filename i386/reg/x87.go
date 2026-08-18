package reg

// St is an x87 floating point register, 80 bits wide.
//
// The name is the psABI's: st0 through st7. GNU as spells these st(0) through
// st(7) and accepts %st for %st(0); those are dialect spellings and belong in
// i386/text/gas, not here.
//
// Num is the stack-relative index, not a physical register. See the note on
// overlaps in reg.go for why this package does not relate St to Mm.
type St uint8

const (
	ST0 St = iota
	ST1
	ST2
	ST3
	ST4
	ST5
	ST6
	ST7
)

var stNames = [8]string{"st0", "st1", "st2", "st3", "st4", "st5", "st6", "st7"}

func (r St) spec() spec {
	return spec{name: stNames[r], class: ClassX87, num: uint8(r), root: uint8(r), lo: 0, hi: 80}
}

func (r St) Name() string        { return stNames[r] }
func (r St) String() string      { return stNames[r] }
func (r St) Num() uint8          { return uint8(r) }
func (r St) Bits() int           { return 80 }
func (r St) Class() Class        { return ClassX87 }
func (r St) Save() Save          { return CallerSaved }
func (r St) Overlaps(o Reg) bool { return overlaps(r, o) }

// Intel386 psABI v1.1, Table 2.14: st0 is 11 through st7 at 18.
func (r St) DWARF() (int, bool) { return 11 + int(r), true }

func (r St) Role() string {
	if r == ST0 {
		return "floating point return"
	}
	return ""
}

// Mm is an MMX register, 64 bits wide.
type Mm uint8

const (
	MM0 Mm = iota
	MM1
	MM2
	MM3
	MM4
	MM5
	MM6
	MM7
)

var mmNames = [8]string{"mm0", "mm1", "mm2", "mm3", "mm4", "mm5", "mm6", "mm7"}

var mmRole = [8]string{"__m64 return; first __m64 parameter", "second __m64 parameter", "third __m64 parameter", "", "", "", "", ""}

func (r Mm) spec() spec {
	return spec{name: mmNames[r], class: ClassMMX, num: uint8(r), root: uint8(r), lo: 0, hi: 64}
}

func (r Mm) Name() string        { return mmNames[r] }
func (r Mm) String() string      { return mmNames[r] }
func (r Mm) Num() uint8          { return uint8(r) }
func (r Mm) Bits() int           { return 64 }
func (r Mm) Class() Class        { return ClassMMX }
func (r Mm) Save() Save          { return CallerSaved }
func (r Mm) Role() string        { return mmRole[r] }
func (r Mm) Overlaps(o Reg) bool { return overlaps(r, o) }

// Intel386 psABI v1.1, Table 2.14: mm0 is 29 through mm7 at 36.
func (r Mm) DWARF() (int, bool) { return 29 + int(r), true }