// x86_64/reg/vec.go
package reg

// Xmm is a 128-bit vector register. XMM16–XMM31 exist only under EVEX: no
// REX prefix can reach them, so a legacy-encoded or VEX-encoded form naming
// one has no encoding at all.
type Xmm uint8

// Ymm is a 256-bit vector register, the low 256 bits of the matching Zmm.
type Ymm uint8

// Zmm is a 512-bit vector register.
type Zmm uint8

const (
	XMM0, YMM0, ZMM0    = Xmm(0), Ymm(0), Zmm(0)
	XMM1, YMM1, ZMM1    = Xmm(1), Ymm(1), Zmm(1)
	XMM2, YMM2, ZMM2    = Xmm(2), Ymm(2), Zmm(2)
	XMM3, YMM3, ZMM3    = Xmm(3), Ymm(3), Zmm(3)
	XMM4, YMM4, ZMM4    = Xmm(4), Ymm(4), Zmm(4)
	XMM5, YMM5, ZMM5    = Xmm(5), Ymm(5), Zmm(5)
	XMM6, YMM6, ZMM6    = Xmm(6), Ymm(6), Zmm(6)
	XMM7, YMM7, ZMM7    = Xmm(7), Ymm(7), Zmm(7)
	XMM8, YMM8, ZMM8    = Xmm(8), Ymm(8), Zmm(8)
	XMM9, YMM9, ZMM9    = Xmm(9), Ymm(9), Zmm(9)
	XMM10, YMM10, ZMM10 = Xmm(10), Ymm(10), Zmm(10)
	XMM11, YMM11, ZMM11 = Xmm(11), Ymm(11), Zmm(11)
	XMM12, YMM12, ZMM12 = Xmm(12), Ymm(12), Zmm(12)
	XMM13, YMM13, ZMM13 = Xmm(13), Ymm(13), Zmm(13)
	XMM14, YMM14, ZMM14 = Xmm(14), Ymm(14), Zmm(14)
	XMM15, YMM15, ZMM15 = Xmm(15), Ymm(15), Zmm(15)
	XMM16, YMM16, ZMM16 = Xmm(16), Ymm(16), Zmm(16)
	XMM17, YMM17, ZMM17 = Xmm(17), Ymm(17), Zmm(17)
	XMM18, YMM18, ZMM18 = Xmm(18), Ymm(18), Zmm(18)
	XMM19, YMM19, ZMM19 = Xmm(19), Ymm(19), Zmm(19)
	XMM20, YMM20, ZMM20 = Xmm(20), Ymm(20), Zmm(20)
	XMM21, YMM21, ZMM21 = Xmm(21), Ymm(21), Zmm(21)
	XMM22, YMM22, ZMM22 = Xmm(22), Ymm(22), Zmm(22)
	XMM23, YMM23, ZMM23 = Xmm(23), Ymm(23), Zmm(23)
	XMM24, YMM24, ZMM24 = Xmm(24), Ymm(24), Zmm(24)
	XMM25, YMM25, ZMM25 = Xmm(25), Ymm(25), Zmm(25)
	XMM26, YMM26, ZMM26 = Xmm(26), Ymm(26), Zmm(26)
	XMM27, YMM27, ZMM27 = Xmm(27), Ymm(27), Zmm(27)
	XMM28, YMM28, ZMM28 = Xmm(28), Ymm(28), Zmm(28)
	XMM29, YMM29, ZMM29 = Xmm(29), Ymm(29), Zmm(29)
	XMM30, YMM30, ZMM30 = Xmm(30), Ymm(30), Zmm(30)
	XMM31, YMM31, ZMM31 = Xmm(31), Ymm(31), Zmm(31)
)

func (r Xmm) Num() uint8 { return uint8(r) }
func (r Ymm) Num() uint8 { return uint8(r) }
func (r Zmm) Num() uint8 { return uint8(r) }

func (Xmm) Bits() int { return 128 }
func (Ymm) Bits() int { return 256 }
func (Zmm) Bits() int { return 512 }

func (Xmm) Class() Class { return ClassXmm }
func (Ymm) Class() Class { return ClassYmm }
func (Zmm) Class() Class { return ClassZmm }

func (r Xmm) Loc() Loc { return Loc{FileVec, uint8(r), 0, 128} }
func (r Ymm) Loc() Loc { return Loc{FileVec, uint8(r), 0, 256} }
func (r Zmm) Loc() Loc { return Loc{FileVec, uint8(r), 0, 512} }

func (r Xmm) Parent() Zmm { return Zmm(r) }
func (r Ymm) Parent() Zmm { return Zmm(r) }

// EVEXOnly reports whether the register is unreachable without EVEX, i.e.
// whether its number is 16 or above.
func (r Xmm) EVEXOnly() bool { return r >= 16 }
func (r Ymm) EVEXOnly() bool { return r >= 16 }

// K is a 64-bit opmask register. K0 is legal as a source but means "no mask"
// when used as a writemask; isa/ gates that, not this package.
type K uint8

const (
	K0 K = iota
	K1
	K2
	K3
	K4
	K5
	K6
	K7
)

func (r K) Num() uint8  { return uint8(r) }
func (K) Bits() int     { return 64 }
func (K) Class() Class  { return ClassK }
func (r K) Loc() Loc    { return Loc{FileMask, uint8(r), 0, 64} }

// Tmm is an AMX tile register. Its shape is set at run time by LDTILECFG;
// Bits reports the architectural maximum of 1 KiB.
type Tmm uint8

const (
	TMM0 Tmm = iota
	TMM1
	TMM2
	TMM3
	TMM4
	TMM5
	TMM6
	TMM7
)

func (r Tmm) Num() uint8 { return uint8(r) }
func (Tmm) Bits() int    { return 8192 }
func (Tmm) Class() Class { return ClassTmm }
func (r Tmm) Loc() Loc   { return Loc{FileTile, uint8(r), 0, 8192} }