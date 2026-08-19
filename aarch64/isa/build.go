package isa

import (
	"strconv"
	"strings"

	"github.com/vertex-language/arc/aarch64/feature"
)

// The fluent builder table rows are written with.
//
// A row states the mnemonic, the base word, the mask, and the operands, and
// reads close to the encoding diagram it came from:
//
//	L("add", 0x91000000, 0xff800000).
//		Dst(ClassXsp, Rd).Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh).
//		Name("AddImm64")
func L(mnem string, word, mask uint32) *Form {
	return &Form{Mnem: mnem, Word: word, Mask: mask}
}

func (f *Form) slot(c Class, r Role, fld Field) *Form {
	f.Slots = append(f.Slots, Slot{Class: c, Role: r, Field: fld})
	return f
}

// Dst adds a written operand.
func (f *Form) Dst(c Class, fld Field) *Form { return f.slot(c, RoleDest, fld) }

// Src adds a read operand.
func (f *Form) Src(c Class, fld Field) *Form { return f.slot(c, RoleSrc, fld) }

// SrcDst adds an operand both read and written.
func (f *Form) SrcDst(c Class, fld Field) *Form { return f.slot(c, RoleSrcDst, fld) }

// Imm adds an immediate operand.
func (f *Form) Imm(fld Field) *Form { return f.slot(ClassImm, RoleSrc, fld) }

// Mem adds an address operand of the given access width and addressing form.
func (f *Form) Mem(bits uint16, base, off Field) *Form {
	c := memClass(bits)
	f.Slots = append(f.Slots, Slot{Class: c, Role: RoleBase, Field: base})
	if off.N != 0 {
		f.Slots = append(f.Slots, Slot{Class: ClassImm, Role: RoleOffset, Field: off})
	}
	return f
}

// Target adds a branch or address destination.
func (f *Form) Target(fld Field) *Form {
	f.Attrs |= AttrBranch
	return f.slot(ClassLabel, RoleTarget, fld)
}

// Addr adds a PC-relative address destination that is not a branch: ADR, ADRP.
func (f *Form) Addr(role Role, fld Field) *Form {
	return f.slot(ClassLabel, role, fld)
}

// Cond adds a condition operand.
func (f *Form) Cnd(fld Field) *Form { return f.slot(ClassCond, RoleModifier, fld) }

// Sys adds a system register operand.
func (f *Form) SysReg(fld Field) *Form { return f.slot(ClassSys, RoleSrc, fld) }

// Opt adds an optional operand with a default field value.
func (f *Form) Opt(c Class, fld Field, def uint64) *Form {
	f.Slots = append(f.Slots, Slot{
		Class: c, Role: RoleModifier, Field: fld, Optional: true, Default: def,
	})
	return f
}

// OptReg adds an optional register operand with a default encoding, which is
// what RET's omitted X30 is.
func (f *Form) OptReg(c Class, fld Field, def uint64) *Form {
	f.Slots = append(f.Slots, Slot{
		Class: c, Role: RoleSrc, Field: fld, Optional: true, Default: def,
	})
	return f
}

// Gate sets the extension this form requires.
func (f *Form) Gate2(x feature.Feature) *Form { f.Gate = x; return f }

// Attr sets form attributes.
func (f *Form) Attr(a Attr) *Form { f.Attrs |= a; return f }

// Flags marks a form that writes NZCV.
func (f *Form) Flags() *Form { f.Attrs |= AttrSetsFlags; return f }

// Name overrides the derived Go helper name.
func (f *Form) Name(n string) *Form { f.name = n; return f }

// AliasOf marks this form as an alias of another mnemonic.
func (f *Form) AliasOf(of string) *Form {
	f.Attrs |= AttrAlias
	if f.alias == nil {
		f.alias = &aliasOf{}
	}
	f.alias.of = of
	return f
}

// Pins records a field this alias fixes, which is what makes it narrower than
// what it aliases.
func (f *Form) Pins(fld Field, v uint64) *Form {
	if f.alias == nil {
		f.alias = &aliasOf{}
	}
	f.alias.fixed = append(f.alias.fixed, fixedField{fld, v})
	return f
}

// PreferredWhen sets the ARM ARM's preferred-disassembly predicate.
func (f *Form) PreferredWhen(p func(word uint32) bool) *Form {
	if f.alias == nil {
		f.alias = &aliasOf{}
	}
	f.alias.preferred = p
	return f
}

// goFormName derives a helper identifier from the mnemonic and operand shape.
// It is only a fallback: most rows state a name, because the derived one is
// rarely the one the ARM ARM's own form title suggests.
func goFormName(f *Form) string {
	var b strings.Builder
	b.WriteString(strings.ToUpper(f.Mnem[:1]))
	b.WriteString(strings.ToLower(f.Mnem[1:]))
	for _, s := range f.Slots {
		switch s.Class {
		case ClassImm:
			b.WriteString("Imm")
		case ClassLabel:
			b.WriteString("Label")
		case ClassCond:
			b.WriteString("Cond")
		}
	}
	for _, s := range f.Slots {
		if s.Class.Reg() {
			b.WriteString(strconv.Itoa(int(s.Class.Bits())))
			break
		}
	}
	return b.String()
}