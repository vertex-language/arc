package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/text"
)

// Assemble builds a complete object from a parsed unit: every statement in u
// is walked in source order and placed into a fresh Assembler's sections,
// then Serialize produces the bytes for p. This is arc build's whole job in
// one call — ParseFile, then Assemble, then write the result — with no
// separate translation step for a caller to get wrong.
//
// What this does not yet do: fold a data or fill value that is not a plain
// constant. ".long . - msg" and similar section-relative or symbolic data
// need a fixup the way an instruction operand gets one, and that backpatch
// path is not wired up here yet — such an item is refused with a specific
// error rather than silently miswritten. Instruction operands have no such
// limit: Encode's own fixups carry a symbolic call or memory reference
// through to Serialize exactly as they do via the typed builder API.
func Assemble(u *text.Unit, p Platform, features feature.Set) ([]byte, error) {
	a := New(p, WithFeatures(features))

	equates := u.Equates()
	lookup := func(name string) (int64, bool) {
		c, ok := equates[name]
		return c, ok
	}

	symAttrs := make(map[string]symInfo)
	for _, sym := range u.Symbols() {
		if sym.Defined {
			symAttrs[sym.Name] = symInfo{attrs: sym.Attrs, typ: sym.Type}
		}
	}

	var cur *Section
	for _, n := range u.Nodes {
		switch d := n.(type) {
		case *text.SectionDecl:
			cur = sectionFor(a, d)

		case *text.Label:
			if cur == nil {
				return nil, fmt.Errorf("i386: %s: label %q outside any section", d.P, d.Name)
			}
			cur.Label(d.Name, labelAttrs(symAttrs[d.Name])...)

		case *text.SymbolDecl:
			// Attributes are collected from u.Symbols() up front and applied
			// at the matching *text.Label above. A SymbolDecl with no local
			// definition — an .extern/EXTERN nothing ever defines — has no
			// offset to attach to and is not yet representable here.

		case *text.Equ:
			// Folded once into equates above; nothing to place.

		case *text.Data:
			if cur == nil {
				return nil, fmt.Errorf("i386: %s: data outside any section", d.P)
			}
			if err := emitData(cur, d, lookup); err != nil {
				return nil, err
			}

		case *text.Fill:
			if cur == nil {
				return nil, fmt.Errorf("i386: %s: fill outside any section", d.P)
			}
			if err := emitFill(cur, d, lookup); err != nil {
				return nil, err
			}

		case *text.Align:
			if cur == nil {
				return nil, fmt.Errorf("i386: %s: align outside any section", d.P)
			}
			if err := emitAlign(cur, d, lookup); err != nil {
				return nil, err
			}

		case *text.Inst:
			if cur == nil {
				return nil, fmt.Errorf("i386: %s: instruction outside any section", d.P)
			}
			inst, _, err := a.Encode(d)
			if err != nil {
				return nil, fmt.Errorf("i386: %s: %w", d.P, err)
			}
			cur.place(inst)

		default:
			return nil, fmt.Errorf("i386: %T is not a statement this builder places", n)
		}
	}

	return a.Serialize()
}

// sectionFor resolves a SectionDecl to the Section it names. Kind and
// SectionKind share one order (asm.go's own note on the cast), so the
// standard eight translate directly; a custom name goes through
// SectionNamed, which is what keeps "__DATA,__objc_classlist" spelled
// exactly as written.
func sectionFor(a *Assembler, d *text.SectionDecl) *Section {
	if d.Kind == text.SectionCustom {
		return a.SectionNamed(d.Name)
	}
	return a.Section(SectionKind(d.Kind))
}

type symInfo struct {
	attrs text.Attr
	typ   text.SymbolType
}

func labelAttrs(s symInfo) []LabelAttr {
	var out []LabelAttr
	if s.attrs&text.AttrGlobal != 0 {
		out = append(out, Global)
	}
	if s.attrs&text.AttrLocal != 0 {
		out = append(out, Local)
	}
	if s.attrs&text.AttrWeak != 0 {
		out = append(out, Weak)
	}
	if s.attrs&text.AttrHidden != 0 {
		out = append(out, Hidden)
	}
	if s.attrs&text.AttrProtected != 0 {
		out = append(out, Protected)
	}
	if s.attrs&text.AttrInternal != 0 {
		out = append(out, Internal)
	}
	if s.attrs&text.AttrExtern != 0 {
		out = append(out, Extern)
	}
	switch s.typ {
	case text.TypeFunc:
		out = append(out, Func)
	case text.TypeObject:
		out = append(out, Object)
	case text.TypeTLS:
		out = append(out, ThreadLocal)
	}
	return out
}

func emitData(s *Section, d *text.Data, lookup text.Lookup) error {
	for _, it := range d.Items {
		if it.IsStr {
			if it.Terminated {
				s.Asciz(it.Str)
			} else {
				s.Ascii(it.Str)
			}
			continue
		}
		v, err := text.Eval(it.X, lookup)
		if err != nil {
			return err
		}
		if !v.IsAbs() {
			return fmt.Errorf(
				"i386: %s: a data value that is not a constant (%s) needs a fixup Assemble does not place yet",
				it.Pos, v)
		}
		switch d.Width {
		case text.Width8:
			s.Byte(uint8(v.Const))
		case text.Width16:
			s.Byte(uint8(v.Const), uint8(v.Const>>8))
		case text.Width32:
			s.Long(uint32(v.Const))
		case text.Width64:
			s.Quad(uint64(v.Const))
		case text.Width128:
			b := make([]byte, 16)
			u := uint64(v.Const)
			for i := 0; i < 8; i++ {
				b[i] = byte(u >> (8 * uint(i)))
			}
			if v.Const < 0 {
				for i := 8; i < 16; i++ {
					b[i] = 0xff
				}
			}
			s.Bytes(b)
		default:
			return fmt.Errorf("i386: %s: %s is not a data width this builder emits", it.Pos, d.Width)
		}
	}
	return nil
}

func emitFill(s *Section, d *text.Fill, lookup text.Lookup) error {
	cv, err := text.Eval(d.Count, lookup)
	if err != nil {
		return err
	}
	if !cv.IsAbs() {
		return fmt.Errorf("i386: a fill count must be a constant, got %s", cv)
	}
	count := cv.Const
	if count < 0 {
		return fmt.Errorf("i386: a fill count cannot be negative")
	}

	if d.Value == nil {
		s.Zero(int(count) * d.Size.Bytes())
		return nil
	}

	vv, err := text.Eval(d.Value, lookup)
	if err != nil {
		return err
	}
	if !vv.IsAbs() {
		return fmt.Errorf("i386: a fill value must be a constant, got %s", vv)
	}

	unit := make([]byte, d.Size.Bytes())
	for i := range unit {
		unit[i] = byte(vv.Const >> (8 * uint(i)))
	}
	buf := make([]byte, 0, int(count)*len(unit))
	for i := int64(0); i < count; i++ {
		buf = append(buf, unit...)
	}
	s.Bytes(buf)
	return nil
}

func emitAlign(s *Section, d *text.Align, lookup text.Lookup) error {
	if d.Value != nil {
		return fmt.Errorf("i386: a custom alignment fill byte is not yet supported by this builder")
	}
	bv, err := text.Eval(d.Bytes, lookup)
	if err != nil {
		return err
	}
	if !bv.IsAbs() {
		return fmt.Errorf("i386: an alignment boundary must be a constant, got %s", bv)
	}
	s.Align(int(bv.Const))
	return nil
}

// Encode is the package-level convenience the typed helpers don't need:
// resolve ti under features and return its shortest legal encoding. No
// platform is meaningful to an encoding — only Serialize needs one — so this
// builds a throwaway ELF-targeted Assembler, exactly as Decode and Explain
// below need no real platform either.
func Encode(features feature.Set, ti *text.Inst) ([]byte, *Form, error) {
	a := New(ELF, WithFeatures(features))
	inst, f, err := a.Encode(ti)
	if err != nil {
		return nil, nil, err
	}
	return inst.Bytes, f, nil
}

// Decode and Explain need no Assembler configuration beyond the default
// Baseline feature set — decode/ is a pure function of bytes and a feature
// set — but arc dis and arc explain read more naturally against a named
// package-level call than against a bare Assembler a caller has to build
// just to throw away.
func Decode(b []byte) (DecodedInst, error) {
	return New(ELF).Decode(b)
}

func Explain(b []byte) (Explanation, error) {
	return New(ELF).Explain(b)
}