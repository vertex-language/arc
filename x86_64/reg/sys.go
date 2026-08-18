// x86_64/reg/sys.go
package reg

// Sreg is a segment register. In 64-bit mode only FS and GS carry a nonzero
// base; the others are ignored for address computation. A segment override
// on a memory operand is operand/'s business, not this package's.
type Sreg uint8

const (
	ES Sreg = iota
	CS
	SS
	DS
	FS
	GS
)

func (r Sreg) Num() uint8 { return uint8(r) }
func (Sreg) Bits() int    { return 16 }
func (Sreg) Class() Class { return ClassSreg }
func (r Sreg) Loc() Loc   { return Loc{FileSeg, uint8(r), 0, 16} }

// Cr is a control register. CR8 through CR15 are reachable only via REX.R.
type Cr uint8

// Dr is a debug register.
type Dr uint8

const (
	CR0 Cr = iota
	CR1
	CR2
	CR3
	CR4
	CR5
	CR6
	CR7
	CR8
	CR9
	CR10
	CR11
	CR12
	CR13
	CR14
	CR15
)

const (
	DR0 Dr = iota
	DR1
	DR2
	DR3
	DR4
	DR5
	DR6
	DR7
	DR8
	DR9
	DR10
	DR11
	DR12
	DR13
	DR14
	DR15
)

func (r Cr) Num() uint8 { return uint8(r) }
func (r Dr) Num() uint8 { return uint8(r) }

func (Cr) Bits() int { return 64 }
func (Dr) Bits() int { return 64 }

func (Cr) Class() Class { return ClassCr }
func (Dr) Class() Class { return ClassDr }

func (r Cr) Loc() Loc { return Loc{FileCtrl, uint8(r), 0, 64} }
func (r Dr) Loc() Loc { return Loc{FileDebug, uint8(r), 0, 64} }

func (r Cr) Extended() bool { return r >= CR8 }
func (r Dr) Extended() bool { return r >= DR8 }