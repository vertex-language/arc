// x86_64/operand/mem.go
package operand

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/vertex-language/arc/x86_64/reg"
)

// Mem is a memory reference: an optional segment override, an addressing form,
// and a width. The addressing forms this target has are
//
//	[base]
//	[base + disp]
//	[base + index*scale + disp]
//	[index*scale + disp]        no base
//	[disp]                      absolute, disp32 only
//	[rip + disp]                RIP-relative, no base and no index
//
// Width is the width of the access, not of the address. Address size is 64 in
// long mode and 32 only when Addr32 is set, which costs a 0x67 prefix.
type Mem struct {
	Width Width

	Seg    reg.Sreg
	HasSeg bool

	Base    reg.Reg64
	HasBase bool

	Index    reg.Reg64
	Scale    uint8 // 1, 2, 4 or 8; zero when there is no index
	HasIndex bool

	// Disp is the constant displacement. When Sym is non-nil it is the
	// constant part alongside the symbolic one.
	Disp int32

	// Sym is a symbolic displacement resolved by a fixup, or nil.
	Sym Target

	// RIP marks a %rip-relative reference. It excludes Base and Index:
	// the encoding is mod=00 rm=101, which has no room for either.
	RIP bool

	// Addr32 selects 32-bit address size, emitting a 0x67 prefix and
	// truncating the computed address to 32 bits. Base and Index are then
	// read as their 32-bit views.
	Addr32 bool
}

// Constructors, one per width. The width names the access, so Mem64(RBX) is
// the operand in `mov rax, [rbx]` and Mem8(RBX) is the one in `mov al, [rbx]`.
func Mem8(base reg.Reg64) M8     { return M8{newMem(W8, base)} }
func Mem16(base reg.Reg64) M16   { return M16{newMem(W16, base)} }
func Mem32(base reg.Reg64) M32   { return M32{newMem(W32, base)} }
func Mem64(base reg.Reg64) M64   { return M64{newMem(W64, base)} }
func Mem128(base reg.Reg64) M128 { return M128{newMem(W128, base)} }
func Mem256(base reg.Reg64) M256 { return M256{newMem(W256, base)} }
func Mem512(base reg.Reg64) M512 { return M512{newMem(W512, base)} }

func newMem(w Width, base reg.Reg64) Mem {
	return Mem{Width: w, Base: base, HasBase: true}
}

// Abs is an absolute memory reference with no base or index. The displacement
// is a disp32 sign-extended to 64 bits — this target has no 64-bit
// displacement outside the MOV moffs forms, so an address above 2GB has to go
// through a register.
func Abs(disp int32) Mem { return Mem{Disp: disp} }

// AbsSym is an absolute reference to a symbol, which becomes a fixup.
func AbsSym(t Target) Mem { return Mem{Sym: t} }

// RIPRel is a %rip-relative reference to a label or symbol. This is how
// position-independent code reaches static data on this target: the
// displacement is resolved against the end of the instruction, and the
// encoder knows where that is because it placed the field.
func RIPRel(t Target) Mem { return Mem{RIP: true, Sym: t} }

// RIPRelDisp is a %rip-relative reference to a constant offset. Rare outside
// hand-written code, and almost never what you want — the offset is from the
// next instruction, which moves when anything before it changes size.
func RIPRelDisp(disp int32) Mem { return Mem{RIP: true, Disp: disp} }

// Builders. Each returns a copy; a Mem is a value and nothing mutates in place.

func (m Mem) WithWidth(w Width) Mem { m.Width = w; return m }

func (m Mem) Segment(s reg.Sreg) Mem {
	m.Seg, m.HasSeg = s, true
	return m
}

func (m Mem) Disp8(d int8) Mem   { m.Disp = int32(d); return m }
func (m Mem) Displace(d int32) Mem { m.Disp = d; return m }

// Index sets the index register and scale. Scale must be 1, 2, 4 or 8.
func (m Mem) Indexed(r reg.Reg64, scale uint8) Mem {
	m.Index, m.Scale, m.HasIndex = r, scale, true
	return m
}

// Sym32 attaches a symbolic displacement.
func (m Mem) WithSym(t Target) Mem { m.Sym = t; return m }

// Use32 selects 32-bit address size.
func (m Mem) Use32() Mem { m.Addr32 = true; return m }

var (
	ErrScale     = errors.New("scale must be 1, 2, 4 or 8")
	ErrIndexRSP  = errors.New("rsp cannot be an index register")
	ErrRIPWithBase = errors.New("rip-relative addressing takes no base or index")
	ErrRIPWithDisp = errors.New("rip-relative addressing takes a symbol or a displacement, not both")
)

// Validate reports whether the addressing form has an encoding. It is the
// operand's own rules only — whether a *form* accepts memory in that slot is
// isa/'s question, and whether the fixup can be recorded is the platform
// writer's.
func (m Mem) Validate() error {
	if m.RIP {
		if m.HasBase || m.HasIndex {
			return ErrRIPWithBase
		}
		if m.Sym != nil && m.Disp != 0 {
			return ErrRIPWithDisp
		}
		return nil
	}

	if m.HasIndex {
		switch m.Scale {
		case 1, 2, 4, 8:
		default:
			return fmt.Errorf("%w (got %d)", ErrScale, m.Scale)
		}
		// Index field value 4 means "no index"; there is no escape for it,
		// so RSP is the one register that cannot be scaled. R12 encodes as
		// 4 with REX.X set and is fine.
		if m.Index == reg.RSP {
			return ErrIndexRSP
		}
	}
	return nil
}

// NeedsSIB reports whether the form requires a SIB byte. An index always does;
// so does RSP or R12 as base, because rm=100 is the escape to SIB and both
// encode as 4.
func (m Mem) NeedsSIB() bool {
	if m.RIP {
		return false
	}
	if m.HasIndex || !m.HasBase {
		return true
	}
	return m.Base.Num()&7 == 4 // RSP, R12
}

// NeedsZeroDisp8 reports whether a zero displacement must still be encoded, as
// a disp8 of zero. RBP and R13 encode as 5, and mod=00 with rm=5 means
// disp32-with-no-base, so [rbp] has no zero-displacement encoding.
func (m Mem) NeedsZeroDisp8() bool {
	if m.RIP || !m.HasBase || m.Sym != nil || m.Disp != 0 {
		return false
	}
	return m.Base.Num()&7 == 5 // RBP, R13
}

// Fixup reports whether this reference needs a relocation or backpatch.
func (m Mem) Fixup() bool { return m.Sym != nil }

func (m Mem) String() string {
	var s string
	if m.HasSeg {
		s += m.Seg.Name() + ":"
	}
	s += "["
	first := true
	add := func(part string) {
		if !first {
			s += "+"
		}
		s += part
		first = false
	}
	if m.RIP {
		add("rip")
	}
	if m.HasBase {
		add(m.baseName())
	}
	if m.HasIndex {
		add(m.indexName() + "*" + strconv.Itoa(int(m.Scale)))
	}
	if m.Sym != nil {
		add(fmt.Sprint(m.Sym))
	}
	if m.Disp != 0 || first {
		if m.Disp < 0 && !first {
			s += itoa(int64(m.Disp))
			first = false
		} else {
			add(itoa(int64(m.Disp)))
		}
	}
	return s + "]"
}

func (m Mem) baseName() string {
	if m.Addr32 {
		return reg.Reg32(m.Base).Name()
	}
	return m.Base.Name()
}

func (m Mem) indexName() string {
	if m.Addr32 {
		return reg.Reg32(m.Index).Name()
	}
	return m.Index.Name()
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }