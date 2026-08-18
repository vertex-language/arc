package operand

import "github.com/vertex-language/arc/i386/reg"

// The memory operand types, one per access width.
//
// The width is a type rather than a field because it is what makes a
// generated helper reject a wrong-width operand at compile time. M32 and M8
// are different types for the same reason R32 and R8 are: a name that cannot
// distinguish two widths cannot be the name of an operand class.
//
// The width is the width of the access, not of the address. Every effective
// address here is 32 bits.
//
// Constructors are Mem8..Mem512 because the type takes the M8..M512 spelling.

type (
	M8   struct{ mem }
	M16  struct{ mem }
	M32  struct{ mem }
	M64  struct{ mem }
	M80  struct{ mem }
	M128 struct{ mem }
	M256 struct{ mem }
	M512 struct{ mem }
)

// M80 exists for the x87 double extended precision type: fldt and fstpt take
// an 80-bit memory operand, and the psABI's __float80 is that type.

func Mem8(base reg.R32) M8     { return M8{based(8, base)} }
func Mem16(base reg.R32) M16   { return M16{based(16, base)} }
func Mem32(base reg.R32) M32   { return M32{based(32, base)} }
func Mem64(base reg.R32) M64   { return M64{based(64, base)} }
func Mem80(base reg.R32) M80   { return M80{based(80, base)} }
func Mem128(base reg.R32) M128 { return M128{based(128, base)} }
func Mem256(base reg.R32) M256 { return M256{based(256, base)} }
func Mem512(base reg.R32) M512 { return M512{based(512, base)} }

// Abs* is an address with no base and no index: mod=00, rm=101, disp32.
//
// On i386 this is plain absolute addressing. The same encoding in 64-bit mode
// is RIP-relative, which is why x86_64 reaches absolute addressing through a
// SIB byte and i386 does not.
func Abs8() M8     { return M8{absolute(8)} }
func Abs16() M16   { return M16{absolute(16)} }
func Abs32() M32   { return M32{absolute(32)} }
func Abs64() M64   { return M64{absolute(64)} }
func Abs80() M80   { return M80{absolute(80)} }
func Abs128() M128 { return M128{absolute(128)} }
func Abs256() M256 { return M256{absolute(256)} }
func Abs512() M512 { return M512{absolute(512)} }

// Builders. Each returns its own type so a chain stays typed.

func (m M8) Disp(d int32) M8                     { return M8{m.disp_(d)} }
func (m M8) Index(r reg.R32, scale uint8) M8     { return M8{m.index_(r, scale)} }
func (m M8) Sym(r SymRef) M8                     { return M8{m.sym_(r)} }
func (m M8) Segment(s reg.Sreg) M8               { return M8{m.seg_(s)} }
func (M8) RM8()                                  {}

func (m M16) Disp(d int32) M16                   { return M16{m.disp_(d)} }
func (m M16) Index(r reg.R32, scale uint8) M16   { return M16{m.index_(r, scale)} }
func (m M16) Sym(r SymRef) M16                   { return M16{m.sym_(r)} }
func (m M16) Segment(s reg.Sreg) M16             { return M16{m.seg_(s)} }
func (M16) RM16()                                {}

func (m M32) Disp(d int32) M32                   { return M32{m.disp_(d)} }
func (m M32) Index(r reg.R32, scale uint8) M32   { return M32{m.index_(r, scale)} }
func (m M32) Sym(r SymRef) M32                   { return M32{m.sym_(r)} }
func (m M32) Segment(s reg.Sreg) M32             { return M32{m.seg_(s)} }
func (M32) RM32()                                {}

func (m M64) Disp(d int32) M64                   { return M64{m.disp_(d)} }
func (m M64) Index(r reg.R32, scale uint8) M64   { return M64{m.index_(r, scale)} }
func (m M64) Sym(r SymRef) M64                   { return M64{m.sym_(r)} }
func (m M64) Segment(s reg.Sreg) M64             { return M64{m.seg_(s)} }
func (M64) RM64()                                {}

func (m M80) Disp(d int32) M80                   { return M80{m.disp_(d)} }
func (m M80) Index(r reg.R32, scale uint8) M80   { return M80{m.index_(r, scale)} }
func (m M80) Sym(r SymRef) M80                   { return M80{m.sym_(r)} }
func (m M80) Segment(s reg.Sreg) M80             { return M80{m.seg_(s)} }

func (m M128) Disp(d int32) M128                 { return M128{m.disp_(d)} }
func (m M128) Index(r reg.R32, scale uint8) M128 { return M128{m.index_(r, scale)} }
func (m M128) Sym(r SymRef) M128                 { return M128{m.sym_(r)} }
func (m M128) Segment(s reg.Sreg) M128           { return M128{m.seg_(s)} }
func (M128) RM128()                              {}

func (m M256) Disp(d int32) M256                 { return M256{m.disp_(d)} }
func (m M256) Index(r reg.R32, scale uint8) M256 { return M256{m.index_(r, scale)} }
func (m M256) Sym(r SymRef) M256                 { return M256{m.sym_(r)} }
func (m M256) Segment(s reg.Sreg) M256           { return M256{m.seg_(s)} }
func (M256) RM256()                              {}

func (m M512) Disp(d int32) M512                 { return M512{m.disp_(d)} }
func (m M512) Index(r reg.R32, scale uint8) M512 { return M512{m.index_(r, scale)} }
func (m M512) Sym(r SymRef) M512                 { return M512{m.sym_(r)} }
func (m M512) Segment(s reg.Sreg) M512           { return M512{m.seg_(s)} }
func (M512) RM512()                              {}

// The r/m classes. Each is satisfied by exactly two types — the register of
// that width and the memory operand of that width — which is what keeps it a
// width rather than an abstraction.
//
// The marker methods are exported because reg and operand are different
// packages and an unexported one could not span both. The seal holds anyway:
// each interface embeds Operand, which only reg types and reg.Seal embedders
// satisfy.

type (
	RM8   interface{ Operand; RM8() }
	RM16  interface{ Operand; RM16() }
	RM32  interface{ Operand; RM32() }
	RM64  interface{ Operand; RM64() }
	RM128 interface{ Operand; RM128() }
	RM256 interface{ Operand; RM256() }
	RM512 interface{ Operand; RM512() }
)