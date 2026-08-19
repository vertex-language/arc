// x86_64/assemble.go
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/text"
)

// Assemble assembles a parsed text unit into object file bytes for the given
// platform and active feature set.
func Assemble(u *Unit, p Platform, f FeatureSet) ([]byte, error) {
	if u == nil {
		return nil, fmt.Errorf("%w: nil unit", ErrForm)
	}

	if errs := u.Validate(); len(errs) > 0 {
		return nil, wrapText(errs[0])
	}

	a := New(p, WithFeatures(f))
	curSec := a.Section(Text)

	for _, item := range u.Items {
		switch it := item.(type) {
		case *text.SectionDirective:
			curSec = a.SectionNamed(it.Name)
			if it.Align > 1 {
				curSec.Align(int(it.Align))
			}

		case *text.LabelDef:
			var attrs []SymAttr
			if it.Global {
				attrs = append(attrs, Global)
			}
			if it.Weak {
				attrs = append(attrs, Weak)
			}
			if it.Hidden {
				attrs = append(attrs, Hidden)
			}
			switch it.Type {
			case text.TypeFunc:
				attrs = append(attrs, Func)
			case text.TypeObject:
				attrs = append(attrs, Object)
			case text.TypeTLS:
				attrs = append(attrs, TLS)
			}
			curSec.Label(it.Name, attrs...)

		case *text.SymbolAttrDirective:
			sym := a.Symbol(it.Name)
			switch it.Attr {
			case text.AttrGlobal:
				sym.Binding = Global
			case text.AttrWeak:
				sym.Binding = Weak
			case text.AttrHidden:
				sym.Binding = Hidden
			case text.AttrFunc:
				sym.Type = Func
			case text.AttrObject:
				sym.Type = Object
			case text.AttrTLS:
				sym.Type = TLS
			}

		case *text.AlignDirective:
			curSec.Align(int(it.Bytes))

		case *text.DataDirective:
			switch it.Width {
			case 1:
				for _, v := range it.Values {
					curSec.Byte(uint8(v))
				}
			case 2:
				for _, v := range it.Values {
					curSec.Word(uint16(v))
				}
			case 4:
				for _, v := range it.Values {
					curSec.Long(uint32(v))
				}
			case 8:
				for _, v := range it.Values {
					curSec.Quad(v)
				}
			}

		case *text.DataRefDirective:
			switch it.Width {
			case 4:
				curSec.LongRef(Ref(it.Symbol, RelocNone).At(it.Addend))
			case 8:
				curSec.QuadRef(Ref(it.Symbol, RelocNone).At(it.Addend))
			default:
				return nil, atPos(it.Pos, f, fmt.Errorf("%w: unsupported data reference width %d", ErrForm, it.Width))
			}

		case *text.StringDirective:
			if it.NUL {
				curSec.Asciz(it.Value)
			} else {
				curSec.Ascii(it.Value)
			}

		case *text.SpaceDirective:
			curSec.Zero(int(it.Bytes))

		case *text.FillDirective:
			curSec.Fill(int(it.Count), int(it.Size), it.Value)

		case *TextInst:
			ops, err := it.Lower(nil)
			if err != nil {
				return nil, atPos(it.Position, f, err)
			}

			if it.Form != nil {
				curSec.put(it.Form, encode.Opts{}, ops...)
			} else {
				args, err := encode.Args(ops...)
				if err != nil {
					return nil, atPos(it.Position, f, err)
				}
				form, err := Resolve(f, it.Mnemonic, args...)
				if err != nil {
					return nil, atPos(it.Position, f, err)
				}
				curSec.put(form, encode.Opts{}, ops...)
			}
			if a.Err() != nil {
				return nil, a.Err()
			}
		}
	}

	if err := a.Err(); err != nil {
		return nil, err
	}
	return a.Serialize()
}

// EncodeInst lowers and encodes a single text instruction without placing it
// in a section.
func EncodeInst(f FeatureSet, inst *TextInst) ([]byte, []Fixup, error) {
	if inst == nil {
		return nil, nil, fmt.Errorf("%w: nil instruction", ErrForm)
	}

	ops, err := inst.Lower(nil)
	if err != nil {
		return nil, nil, atPos(inst.Position, f, err)
	}

	args, err := encode.Args(ops...)
	if err != nil {
		return nil, nil, atPos(inst.Position, f, err)
	}

	form, err := Resolve(f, inst.Mnemonic, args...)
	if err != nil {
		return nil, nil, atPos(inst.Position, f, err)
	}

	b, fx, err := encode.Encode(form, ops...)
	if err != nil {
		return nil, nil, atPos(inst.Position, f, err)
	}
	return b, fx, nil
}