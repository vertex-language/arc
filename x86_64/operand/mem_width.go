// x86_64/operand/mem_width.go
//
// M8 through M512 are memory references with the width fixed in the type, so
// the generated helper layer can reject a width mismatch at compile time
// rather than at Emit. Each is a thin wrapper whose builders return its own
// type; the logic lives once, on Mem.
//
// The repetition is the point. A single type carrying a width field would put
// the check back at run time, which is the thing the typed layer exists to
// avoid.
package operand

import "github.com/vertex-language/arc/x86_64/reg"

type M8 struct{ Mem }
type M16 struct{ Mem }
type M32 struct{ Mem }
type M64 struct{ Mem }
type M128 struct{ Mem }
type M256 struct{ Mem }
type M512 struct{ Mem }

func (m M8) Segment(s reg.Sreg) M8               { return M8{m.Mem.Segment(s)} }
func (m M8) Disp(d int32) M8                     { return M8{m.Mem.Displace(d)} }
func (m M8) Index(r reg.Reg64, scale uint8) M8   { return M8{m.Mem.Indexed(r, scale)} }
func (m M8) Sym(t Target) M8                     { return M8{m.Mem.WithSym(t)} }
func (m M8) Addr32() M8                          { return M8{m.Mem.Use32()} }

func (m M16) Segment(s reg.Sreg) M16             { return M16{m.Mem.Segment(s)} }
func (m M16) Disp(d int32) M16                   { return M16{m.Mem.Displace(d)} }
func (m M16) Index(r reg.Reg64, scale uint8) M16 { return M16{m.Mem.Indexed(r, scale)} }
func (m M16) Sym(t Target) M16                   { return M16{m.Mem.WithSym(t)} }
func (m M16) Addr32() M16                        { return M16{m.Mem.Use32()} }

func (m M32) Segment(s reg.Sreg) M32             { return M32{m.Mem.Segment(s)} }
func (m M32) Disp(d int32) M32                   { return M32{m.Mem.Displace(d)} }
func (m M32) Index(r reg.Reg64, scale uint8) M32 { return M32{m.Mem.Indexed(r, scale)} }
func (m M32) Sym(t Target) M32                   { return M32{m.Mem.WithSym(t)} }
func (m M32) Addr32() M32                        { return M32{m.Mem.Use32()} }

func (m M64) Segment(s reg.Sreg) M64             { return M64{m.Mem.Segment(s)} }
func (m M64) Disp(d int32) M64                   { return M64{m.Mem.Displace(d)} }
func (m M64) Index(r reg.Reg64, scale uint8) M64 { return M64{m.Mem.Indexed(r, scale)} }
func (m M64) Sym(t Target) M64                   { return M64{m.Mem.WithSym(t)} }
func (m M64) Addr32() M64                        { return M64{m.Mem.Use32()} }

func (m M128) Segment(s reg.Sreg) M128             { return M128{m.Mem.Segment(s)} }
func (m M128) Disp(d int32) M128                   { return M128{m.Mem.Displace(d)} }
func (m M128) Index(r reg.Reg64, scale uint8) M128 { return M128{m.Mem.Indexed(r, scale)} }
func (m M128) Sym(t Target) M128                   { return M128{m.Mem.WithSym(t)} }
func (m M128) Addr32() M128                        { return M128{m.Mem.Use32()} }

func (m M256) Segment(s reg.Sreg) M256             { return M256{m.Mem.Segment(s)} }
func (m M256) Disp(d int32) M256                   { return M256{m.Mem.Displace(d)} }
func (m M256) Index(r reg.Reg64, scale uint8) M256 { return M256{m.Mem.Indexed(r, scale)} }
func (m M256) Sym(t Target) M256                   { return M256{m.Mem.WithSym(t)} }
func (m M256) Addr32() M256                        { return M256{m.Mem.Use32()} }

func (m M512) Segment(s reg.Sreg) M512             { return M512{m.Mem.Segment(s)} }
func (m M512) Disp(d int32) M512                   { return M512{m.Mem.Displace(d)} }
func (m M512) Index(r reg.Reg64, scale uint8) M512 { return M512{m.Mem.Indexed(r, scale)} }
func (m M512) Sym(t Target) M512                   { return M512{m.Mem.WithSym(t)} }
func (m M512) Addr32() M512                        { return M512{m.Mem.Use32()} }

// Narrowing a width-agnostic Mem — Abs, AbsSym and RIPRel produce one, since
// an absolute or %rip-relative address has no width of its own until an
// instruction reads through it.
func (m Mem) M8() M8     { return M8{m.WithWidth(W8)} }
func (m Mem) M16() M16   { return M16{m.WithWidth(W16)} }
func (m Mem) M32() M32   { return M32{m.WithWidth(W32)} }
func (m Mem) M64() M64   { return M64{m.WithWidth(W64)} }
func (m Mem) M128() M128 { return M128{m.WithWidth(W128)} }
func (m Mem) M256() M256 { return M256{m.WithWidth(W256)} }
func (m Mem) M512() M512 { return M512{m.WithWidth(W512)} }