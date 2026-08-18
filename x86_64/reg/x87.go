// x86_64/reg/x87.go
package reg

// St is an x87 stack register. The number is a stack position at the moment
// the instruction executes, not a fixed physical register.
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

// Mm is an MMX register: the low 64 bits of the matching x87 register's
// mantissa. Writing MM(i) sets ST(i)'s exponent to all ones, which is why
// they share a file entry here.
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

func (r St) Num() uint8 { return uint8(r) }
func (r Mm) Num() uint8 { return uint8(r) }

func (St) Bits() int { return 80 }
func (Mm) Bits() int { return 64 }

func (St) Class() Class { return ClassSt }
func (Mm) Class() Class { return ClassMm }

func (r St) Loc() Loc { return Loc{FileX87, uint8(r), 0, 80} }
func (r Mm) Loc() Loc { return Loc{FileX87, uint8(r), 0, 64} }

func (r Mm) Parent() St { return St(r) }