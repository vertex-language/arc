package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/decode"
	"github.com/vertex-language/arc/i386/encode"
	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/text"
)

// Encode, Forms, Decode and Explain — the single-instruction surface arc
// enc, arc dis and arc explain call. This file is thin by design: it
// resolves a text.Inst to a form and operand values, then hands both to
// encode/ or decode/, which never see text themselves.
//
// The one real work this file does is toOperands: turning text/'s dialect-
// neutral operand shapes (text.Reg, text.Imm, text.Mem, text.Indirect) into
// this package's typed Operand values (reg.Reg, operand.Imm, operand.M8
// through M512, operand.Label, operand.SymRef). Two things make that
// non-trivial:
//
//   - A memory operand's access width is not always stated. text.Inst.Size
//     carries it when the source did (a GAS suffix, a NASM BYTE/WORD/DWORD
//     keyword); failing that, a sibling register operand's width is used;
//     failing that, the operand is genuinely ambiguous and toMem refuses it,
//     the same refusal STRICT/a size keyword exists to prevent in NASM
//     itself.
//   - A symbolic expression cannot be an operand.Imm at all — that type is a
//     bare int64, per operand.go's own doc, because on x86 an immediate's
//     width is the form's and not the value's. So the branch between Imm and
//     Label/SymRef is not a judgment call about context: text.Eval decides
//     it by whether the expression folds to a constant, full stop.

// The typed aliases arc enc, arc dis and arc explain print through.
type (
	EncodedInst  = encode.Inst
	DecodedInst  = decode.Inst
	Explanation  = decode.Explanation
	ExplainField = decode.Field
	ExplainBits  = decode.BitField
	Form         = isa.Form
)

// Encode resolves ti against the active feature set and returns the
// shortest legal encoding, breaking ties by isa/'s own table order — the
// same rule Emit follows, because arc enc and Emit share nothing else but
// must never disagree about which bytes a form produces.
func (a *Assembler) Encode(ti *text.Inst) (EncodedInst, *Form, error) {
	ops, err := a.toOperands(ti)
	if err != nil {
		return EncodedInst{}, nil, err
	}

	match, gated := isa.Resolve(ti.Mnemonic, a.features, ops)
	if len(match) == 0 {
		if len(gated) > 0 {
			return EncodedInst{}, nil, a.gatedErr(ti.Mnemonic, gated[0])
		}
		return EncodedInst{}, nil, formErr("", 0, ti.Mnemonic)
	}

	var best *Form
	var bestInst EncodedInst
	for _, f := range match {
		inst, err := encode.Encode(f, ops)
		if err != nil {
			continue
		}
		if best == nil || inst.Len() < bestInst.Len() {
			best, bestInst = f, inst
		}
	}
	if best == nil {
		return EncodedInst{}, nil, formErr("", 0, ti.Mnemonic)
	}
	return bestInst, best, nil
}

// Forms returns every candidate form ti's mnemonic and operands resolve to,
// matched and gated separately — arc enc --all's data, undecided about
// which one Encode would actually pick.
func (a *Assembler) Forms(ti *text.Inst) (match, gated []*Form) {
	ops, err := a.toOperands(ti)
	if err != nil {
		return nil, nil
	}
	return isa.Resolve(ti.Mnemonic, a.features, ops)
}

// Decode reads one instruction from the front of b, gated against the same
// feature set Encode is.
func (a *Assembler) Decode(b []byte) (DecodedInst, error) {
	return decode.Decode(b, a.features)
}

// Explain is Decode's field decomposition, for arc explain's three
// renderings.
func (a *Assembler) Explain(b []byte) (Explanation, error) {
	return decode.Explain(b, a.features)
}

func (a *Assembler) gatedErr(mnemonic string, f *Form) *Error {
	need := feature.New(f.Level)
	if f.HasFeat {
		need = need.Add(f.Feat)
	}
	return featureErr("", 0, mnemonic, a.features.Missing(need), a.features)
}

// toOperands converts every operand of ti in one pass, inferring a shared
// memory width up front so every text.Mem operand in the instruction agrees
// with it — two memory operands of different implied widths in one
// instruction is not a shape isa/ has a form for anyway.
func (a *Assembler) toOperands(ti *text.Inst) ([]Operand, error) {
	w := ti.Size
	if w == text.WidthNone {
		for _, o := range ti.Ops {
			if r, ok := o.(text.Reg); ok {
				w = text.Width(r.R.Bits() / 8)
				break
			}
		}
	}

	out := make([]Operand, len(ti.Ops))
	for i, o := range ti.Ops {
		v, err := a.toOperand(o, w)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (a *Assembler) toOperand(o text.Operand, w text.Width) (Operand, error) {
	switch v := o.(type) {
	case text.Reg:
		rv, ok := v.R.(Operand)
		if !ok {
			return nil, fmt.Errorf("i386: %s cannot appear as an operand", v.R.Name())
		}
		return rv, nil

	case text.Imm:
		return a.toValue(v.X)

	case text.Mem:
		return a.toMem(v, w)

	case text.Indirect:
		return a.toOperand(v.X, w)
	}
	return nil, fmt.Errorf("i386: %T is not an operand this arch encodes", o)
}

// toValue folds x to an operand.Imm, or — when it does not fold, which is
// exactly the shape a branch target or an external reference takes — builds
// a Label (same-section, no relocation) or a SymRef (a relocation, kind
// resolved from x's modifier). A Label has no addend field, so a nonzero
// constant on a plain name with no modifier is refused rather than dropped.
func (a *Assembler) toValue(x text.Expr) (Operand, error) {
	v, err := text.Eval(x, nil)
	if err != nil {
		return nil, err
	}
	if v.IsAbs() {
		return operand.Imm(v.Const), nil
	}

	switch v.Kind() {
	case text.Relocatable, text.PCRelative:
		name, mod, _ := v.Sym()
		if mod == text.ModNone {
			if v.Const != 0 {
				return nil, fmt.Errorf(
					"i386: %s+%d has no local-label form; a label carries no addend, only a relocation does",
					name, v.Const)
			}
			return operand.Label(name), nil
		}
		kind, err := a.relocKindFor(mod)
		if err != nil {
			return nil, err
		}
		return operand.Ref(name, kind).Plus(int32(v.Const)), nil

	default:
		return nil, fmt.Errorf("i386: %s is not a value this arch can encode", v)
	}
}

// relocKindFor maps a text.Modifier — the word after '@' or WRT — to this
// platform's relocation number. i386 has no PC-relative addressing mode,
// which is why GOTOFF and GOTPC exist at all rather than a GOTPCREL the way
// x86_64 has one.
var elfModKind = map[text.Modifier]RelocKind{
	text.ModPLT:      R_386_PLT32,
	text.ModGOT:       R_386_GOT32,
	text.ModGOTOFF:    R_386_GOTOFF,
	text.ModGOTPC:     R_386_GOTPC,
	text.ModTLSGD:     R_386_TLS_GD,
	text.ModTLSLDM:    R_386_TLS_LDM,
	text.ModDTPOFF:    R_386_TLS_LDO_32,
	text.ModGOTTPOFF:  R_386_TLS_IE,
	text.ModTPOFF:     R_386_TLS_LE,
}

func (a *Assembler) relocKindFor(mod text.Modifier) (RelocKind, error) {
	if a.platform != ELF {
		return 0, fmt.Errorf("i386: %s is an ELF relocation modifier; target is %s", mod, a.platform)
	}
	k, ok := elfModKind[mod]
	if !ok {
		return 0, fmt.Errorf("i386: %s has no i386 relocation mapping", mod)
	}
	return k, nil
}

// toMem builds the memory operand of width w. w must be known by the time
// this is called — toOperands has already tried the instruction's size hint
// and a sibling register's width, and if neither applied, the ambiguity is
// real and gets refused here rather than guessed at.
func (a *Assembler) toMem(m text.Mem, w text.Width) (Operand, error) {
	if w == text.WidthNone {
		return nil, fmt.Errorf(
			"i386: memory operand width is ambiguous; state it explicitly (e.g. dword)")
	}

	var disp int32
	var sym operand.SymRef
	hasSym := false

	if m.Disp != nil {
		v, err := text.Eval(m.Disp, nil)
		if err != nil {
			return nil, err
		}
		if v.IsAbs() {
			disp = int32(v.Const)
		} else {
			switch v.Kind() {
			case text.Relocatable, text.PCRelative:
				name, mod, _ := v.Sym()
				kind := R_386_32
				if mod != text.ModNone {
					k, err := a.relocKindFor(mod)
					if err != nil {
						return nil, err
					}
					kind = k
				}
				sym, hasSym = operand.Ref(name, kind).Plus(int32(v.Const)), true
			default:
				return nil, fmt.Errorf("i386: %s is not a displacement this arch can encode", v)
			}
		}
	}

	switch w {
	case text.Width8:
		mm := memBase8(m)
		if hasSym {
			mm = mm.Sym(sym)
		} else {
			mm = mm.Disp(disp)
		}
		if m.HasIndex {
			mm = mm.Index(m.Index, m.Scale)
		}
		if m.HasSeg {
			mm = mm.Segment(m.Seg)
		}
		return mm, nil

	case text.Width16:
		mm := memBase16(m)
		if hasSym {
			mm = mm.Sym(sym)
		} else {
			mm = mm.Disp(disp)
		}
		if m.HasIndex {
			mm = mm.Index(m.Index, m.Scale)
		}
		if m.HasSeg {
			mm = mm.Segment(m.Seg)
		}
		return mm, nil

	case text.Width32:
		mm := memBase32(m)
		if hasSym {
			mm = mm.Sym(sym)
		} else {
			mm = mm.Disp(disp)
		}
		if m.HasIndex {
			mm = mm.Index(m.Index, m.Scale)
		}
		if m.HasSeg {
			mm = mm.Segment(m.Seg)
		}
		return mm, nil

	case text.Width64:
		mm := memBase64(m)
		if hasSym {
			mm = mm.Sym(sym)
		} else {
			mm = mm.Disp(disp)
		}
		if m.HasIndex {
			mm = mm.Index(m.Index, m.Scale)
		}
		if m.HasSeg {
			mm = mm.Segment(m.Seg)
		}
		return mm, nil

	case text.Width128:
		mm := memBase128(m)
		if hasSym {
			mm = mm.Sym(sym)
		} else {
			mm = mm.Disp(disp)
		}
		if m.HasIndex {
			mm = mm.Index(m.Index, m.Scale)
		}
		if m.HasSeg {
			mm = mm.Segment(m.Seg)
		}
		return mm, nil
	}

	return nil, fmt.Errorf("i386: %v is not a memory access width this arch encodes", w)
}

func memBase8(m text.Mem) M8 {
	if m.HasBase {
		return Mem8(m.Base)
	}
	return Abs8()
}
func memBase16(m text.Mem) M16 {
	if m.HasBase {
		return Mem16(m.Base)
	}
	return Abs16()
}
func memBase32(m text.Mem) M32 {
	if m.HasBase {
		return Mem32(m.Base)
	}
	return Abs32()
}
func memBase64(m text.Mem) M64 {
	if m.HasBase {
		return Mem64(m.Base)
	}
	return Abs64()
}
func memBase128(m text.Mem) M128 {
	if m.HasBase {
		return Mem128(m.Base)
	}
	return Abs128()
}