package reg

// all is every register this package declares, in declaration order. The
// order is the order arc regs prints them in and is part of the deterministic
// output guarantee.
var all []Reg

var byName map[string]Reg

func init() {
	for i := 0; i < 8; i++ {
		all = append(all, R32(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, R16(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, R8(i))
	}
	for i := 0; i < 6; i++ {
		all = append(all, Sreg(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, St(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Mm(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Xmm(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Ymm(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Zmm(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, K(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Cr(i))
	}
	for i := 0; i < 8; i++ {
		all = append(all, Dr(i))
	}

	byName = make(map[string]Reg, len(all))
	for _, r := range all {
		if prev, dup := byName[r.Name()]; dup {
			panic("i386/reg: duplicate register name " + r.Name() + " (" + prev.Class().String() + ")")
		}
		byName[r.Name()] = r
	}
}

// Lookup resolves a canonical register name. Names are bare: eax, not %eax.
// Dialect spellings — the AT&T sigil, st(0) for st0, db0 for dr0 — resolve in
// i386/text, which is the only place that knows a dialect exists.
func Lookup(name string) (Reg, bool) {
	r, ok := byName[name]
	return r, ok
}

// All returns every declared register in a fixed order.
func All() []Reg {
	out := make([]Reg, len(all))
	copy(out, all)
	return out
}