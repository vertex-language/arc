package reg

// The vector registers. i386 has eight of each width: psABI v1.1 section
// 2.2.1 states %xmm0-%xmm7, %ymm0-%ymm7, %zmm0-%zmm7 and %k0-%k7.
//
// Xmm, Ymm and Zmm share one architectural root per index, so XMM0 overlaps
// YMM0 and ZMM0. None of the three declares a Parent method: whether XMM0 has
// a wider register above it depends on the active feature set, and this
// package does not import feature. Overlaps answers the static question;
// Parent would have to answer a question with a runtime dependency.

// Xmm is a 128-bit vector register.
type Xmm uint8

const (
	XMM0 Xmm = iota
	XMM1
	XMM2
	XMM3
	XMM4
	XMM5
	XMM6
	XMM7
)

var xmmRole = [8]string{"__m128 return; first __m128 parameter", "second __m128 parameter", "third __m128 parameter", "", "", "", "", ""}

func (r Xmm) spec() spec {
	return spec{name: vecName("xmm", uint8(r)), class: ClassVec, num: uint8(r), root: uint8(r), lo: 0, hi: 128}
}

func (r Xmm) Name() string        { return vecName("xmm", uint8(r)) }
func (r Xmm) String() string      { return r.Name() }
func (r Xmm) Num() uint8          { return uint8(r) }
func (r Xmm) Bits() int           { return 128 }
func (r Xmm) Class() Class        { return ClassVec }
func (r Xmm) Save() Save          { return CallerSaved }
func (r Xmm) Role() string        { return xmmRole[r] }
func (r Xmm) Overlaps(o Reg) bool { return overlaps(r, o) }

// Intel386 psABI v1.1, Table 2.14: xmm0 is 21 through xmm7 at 28.
func (r Xmm) DWARF() (int, bool) { return 21 + int(r), true }

// Ymm is a 256-bit vector register.
type Ymm uint8

const (
	YMM0 Ymm = iota
	YMM1
	YMM2
	YMM3
	YMM4
	YMM5
	YMM6
	YMM7
)

var ymmRole = [8]string{"__m256 return; first __m256 parameter", "second __m256 parameter", "third __m256 parameter", "", "", "", "", ""}

func (r Ymm) spec() spec {
	return spec{name: vecName("ymm", uint8(r)), class: ClassVec, num: uint8(r), root: uint8(r), lo: 0, hi: 256}
}

func (r Ymm) Name() string        { return vecName("ymm", uint8(r)) }
func (r Ymm) String() string      { return r.Name() }
func (r Ymm) Num() uint8          { return uint8(r) }
func (r Ymm) Bits() int           { return 256 }
func (r Ymm) Class() Class        { return ClassVec }
func (r Ymm) Save() Save          { return CallerSaved }
func (r Ymm) Role() string        { return ymmRole[r] }
func (r Ymm) Overlaps(o Reg) bool { return overlaps(r, o) }

// psABI Table 2.14 assigns no DWARF number to the AVX registers.
func (r Ymm) DWARF() (int, bool) { return noDWARF, false }

// Zmm is a 512-bit vector register.
type Zmm uint8

const (
	ZMM0 Zmm = iota
	ZMM1
	ZMM2
	ZMM3
	ZMM4
	ZMM5
	ZMM6
	ZMM7
)

var zmmRole = [8]string{"__m512 return; first __m512 parameter", "second __m512 parameter", "third __m512 parameter", "", "", "", "", ""}

func (r Zmm) spec() spec {
	return spec{name: vecName("zmm", uint8(r)), class: ClassVec, num: uint8(r), root: uint8(r), lo: 0, hi: 512}
}

func (r Zmm) Name() string        { return vecName("zmm", uint8(r)) }
func (r Zmm) String() string      { return r.Name() }
func (r Zmm) Num() uint8          { return uint8(r) }
func (r Zmm) Bits() int           { return 512 }
func (r Zmm) Class() Class        { return ClassVec }
func (r Zmm) Save() Save          { return CallerSaved }
func (r Zmm) Role() string        { return zmmRole[r] }
func (r Zmm) Overlaps(o Reg) bool { return overlaps(r, o) }

func (r Zmm) DWARF() (int, bool) { return noDWARF, false }

// K is a 64-bit AVX-512 vector mask register. Mask registers are their own
// class and overlap nothing.
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

func (r K) spec() spec {
	return spec{name: vecName("k", uint8(r)), class: ClassMask, num: uint8(r), root: uint8(r), lo: 0, hi: 64}
}

func (r K) Name() string        { return vecName("k", uint8(r)) }
func (r K) String() string      { return r.Name() }
func (r K) Num() uint8          { return uint8(r) }
func (r K) Bits() int           { return 64 }
func (r K) Class() Class        { return ClassMask }
func (r K) Save() Save          { return CallerSaved }
func (r K) Role() string        { return "" }
func (r K) Overlaps(o Reg) bool { return overlaps(r, o) }

func (r K) DWARF() (int, bool) { return noDWARF, false }

func vecName(prefix string, n uint8) string {
	return prefix + string(rune('0'+n))
}