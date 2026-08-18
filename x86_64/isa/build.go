// x86_64/isa/build.go
//
// The constructors the tables are written in. They exist so a table row reads
// like the manual line it came from and not like a struct literal — a table
// nobody can proofread against the SDM is a table that drifts from it.
package isa

import "github.com/vertex-language/arc/x86_64/feature"

// L is a legacy form: mnemonic, opcode, slots.
func L(op string, opcode byte, slots ...Slot) *Form {
	return &Form{Op: op, Opcode: opcode, Slots: slots, Ext: NoExt, Enc: EncLegacy}
}

// V is a VEX form. The length and map come from the manual's prefix spelling:
// VEX.128.66.0F.W0 is V(op, L128, Pfx66, Map0F, opcode, ...).
func V(op string, l VLen, p Pfx, m Map, opcode byte, slots ...Slot) *Form {
	return &Form{Op: op, Opcode: opcode, Slots: slots, Ext: NoExt,
		Enc: EncVEX, Len: l, Pfx: p, Map: m}
}

// E is an EVEX form.
func E(op string, l VLen, p Pfx, m Map, opcode byte, slots ...Slot) *Form {
	return &Form{Op: op, Opcode: opcode, Slots: slots, Ext: NoExt,
		Enc: EncEVEX, Len: l, Pfx: p, Map: m}
}

func (f *Form) ext(d int8) *Form   { f.Ext = d; return f }
func (f *Form) m0F() *Form         { f.Map = Map0F; return f }
func (f *Form) m38() *Form         { f.Map = Map0F38; return f }
func (f *Form) m3A() *Form         { f.Map = Map0F3A; return f }
func (f *Form) p66() *Form         { f.Pfx = Pfx66; return f }
func (f *Form) pF3() *Form         { f.Pfx = PfxF3; return f }
func (f *Form) pF2() *Form         { f.Pfx = PfxF2; return f }
func (f *Form) w1() *Form          { f.W = W1; return f }
func (f *Form) wig() *Form         { f.W = WIG; return f }
func (f *Form) d16() *Form         { f.Attrs |= Data16; return f }
func (f *Form) lock() *Form        { f.Attrs |= Lockable; return f }
func (f *Form) def64() *Form       { f.Attrs |= Default64; return f }
func (f *Form) term() *Form        { f.Attrs |= Terminal; return f }
func (f *Form) need(x feature.Feature) *Form { f.Need = x; return f }

func (f *Form) mask() *Form  { f.Attrs |= Masked | Zeroing; return f }
func (f *Form) merge() *Form { f.Attrs |= Masked; return f }
func (f *Form) sae() *Form   { f.Attrs |= SAE; return f }
func (f *Form) er() *Form    { f.Attrs |= RoundCtl; return f }

func (f *Form) tup(t Tuple) *Form { f.Tuple = t; return f }
func (f *Form) bcst(elem int8) *Form {
	f.Attrs |= Broadcast
	f.Elem = elem
	return f
}