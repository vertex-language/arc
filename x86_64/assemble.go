// x86_64/assemble.go
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/text"
)

// Assemble assembles a parsed text unit into object file bytes for the given
// platform and active feature set.
//
// It walks u.Nodes in source order. A *text.Label places a symbol at the
// current offset; a *text.Directive carries everything else — section
// changes, binding and type declarations, data, and space — dispatched on
// its Kind rather than on a family of directive types, because that is what
// the tree now hands back from either dialect's parser.
func Assemble(u *Unit, p Platform, f FeatureSet) ([]byte, error) {
	if u == nil {
		return nil, fmt.Errorf("%w: nil unit", ErrForm)
	}

	if errs := u.Validate(); len(errs) > 0 {
		return nil, wrapText(errs[0])
	}

	a := New(p, WithFeatures(f))
	curSec := a.Section(Text)

	for _, node := range u.Nodes {
		switch it := node.(type) {
		case *text.Label:
			// Numeric labels (gas's `1:`, referenced as `1b`/`1f`) are
			// position references and not symbols — Unit.Defined excludes
			// them for the same reason. Resolving one is the same
			// backpatch path `.quad . - msg` is waiting on: see the
			// x86_64 README's "What Assemble does not yet do".
			if it.Numeric {
				continue
			}
			curSec.Label(it.Name)

		case *text.Directive:
			sec, err := assembleDirective(a, curSec, it)
			if err != nil {
				return nil, err
			}
			curSec = sec

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

		case *text.Comment, *text.Blank:
			// Carry no bytes.
		}
	}

	if err := a.Err(); err != nil {
		return nil, err
	}
	return a.Serialize()
}

// assembleDirective handles one *text.Directive, returning the section that
// is current afterward — the only directive that changes it is Section
// itself.
func assembleDirective(a *Assembler, cur *Section, d *text.Directive) (*Section, error) {
	switch d.Kind {
	case text.Section:
		name := d.SectionName()
		if name == "" {
			return cur, atPos(d.Position, a.cfg.features,
				fmt.Errorf("%w: .section names no section", ErrForm))
		}
		return a.SectionNamed(name), nil

	case text.Global, text.Weak, text.Hidden, text.Local:
		attr := symAttrForKind(d.Kind)
		for _, name := range d.Symbols() {
			a.Declare(name, attr)
		}
		return cur, nil

	case text.Extern:
		for _, name := range d.Symbols() {
			a.Declare(name, Global)
		}
		return cur, nil

	case text.Type:
		return cur, assembleType(a, d)

	case text.Size:
		assembleSize(a, d)
		return cur, nil

	case text.Comm, text.LComm, text.Equ:
		// A common symbol and .equ both need a value threaded across
		// statements — an Env — and Assemble runs with none: see the
		// backpatch gap noted in the README.
		return cur, atPos(d.Position, a.cfg.features,
			fmt.Errorf("%w: .%s is not wired into Assemble yet", ErrForm, d.Kind))

	case text.Align, text.P2Align:
		n, err := d.Alignment(nil)
		if err != nil {
			return cur, wrapText(err)
		}
		cur.Align(int(n))
		return cur, nil

	case text.Byte, text.Word, text.Long, text.Quad:
		return cur, assembleData(cur, d)

	case text.Ascii:
		cur.Ascii(d.Str)
		return cur, nil

	case text.Asciz:
		cur.Asciz(d.Str)
		return cur, nil

	case text.Fill:
		count, err := d.Const(nil, 0)
		if err != nil {
			return cur, wrapText(err)
		}
		width := int64(1)
		if len(d.Args) > 1 {
			if width, err = d.Const(nil, 1); err != nil {
				return cur, wrapText(err)
			}
		}
		value := int64(0)
		if len(d.Args) > 2 {
			if value, err = d.Const(nil, 2); err != nil {
				return cur, wrapText(err)
			}
		}
		cur.Fill(int(count), int(width), uint64(value))
		return cur, nil

	case text.Zero:
		n, err := d.Const(nil, 0)
		if err != nil {
			return cur, wrapText(err)
		}
		cur.Zero(int(n))
		return cur, nil

	case text.Org:
		// No linker-free image-layout step resolves an origin yet.
		return cur, atPos(d.Position, a.cfg.features,
			fmt.Errorf("%w: .org is not wired into Assemble yet", ErrForm))
	}
	return cur, nil
}

// symAttrForKind maps a binding directive's Kind to this package's SymAttr.
func symAttrForKind(k text.Kind) SymAttr {
	switch k {
	case text.Global:
		return Global
	case text.Weak:
		return Weak
	case text.Hidden:
		return Hidden
	}
	return Local
}

// assembleType applies a .type directive: `.type name, @function` and its
// NASM and gas-@%# spelling variants, resolved by text.ParseSymbolType.
func assembleType(a *Assembler, d *text.Directive) error {
	syms := d.Symbols()
	if len(syms) == 0 || len(d.Args) < 2 {
		return atPos(d.Position, a.cfg.features,
			fmt.Errorf("%w: .type needs a symbol and a type", ErrForm))
	}

	spell, ok := d.Args[1].(*text.Sym)
	if !ok {
		return atPos(d.Position, a.cfg.features,
			fmt.Errorf("%w: .type's second argument names a type", ErrForm))
	}
	st, err := text.ParseSymbolType(spell.Name)
	if err != nil {
		return atPos(d.Position, a.cfg.features, fmt.Errorf("%w: %v", ErrForm, err))
	}

	name := syms[0]
	switch st {
	case text.TypeFunc:
		a.Declare(name, Func)
	case text.TypeObject:
		a.Declare(name, Object)
	case text.TypeTLS:
		a.Declare(name, TLS)
	}
	return nil
}

// assembleSize applies a .size directive when its value is a constant.
// `.size f, .-f` reduces to a symbolic Value this package cannot fold
// standalone; that case is left to Serialize's automatic closing, which
// computes the same distance once every symbol in the section has landed.
func assembleSize(a *Assembler, d *text.Directive) {
	syms := d.Symbols()
	if len(syms) == 0 || len(d.Args) < 2 {
		return
	}
	if v, err := text.Eval(d.Args[1], nil); err == nil {
		a.SetSize(syms[0], v)
	}
}

// assembleData emits a Byte/Word/Long/Quad directive's values: a constant
// writes directly, and a single added symbol becomes a data reference fixup
// via LongRef/QuadRef — the same path a jump table entry takes.
func assembleData(cur *Section, d *text.Directive) error {
	width := d.DataWidth()

	vals, err := d.Values(nil)
	if err != nil {
		return wrapText(err)
	}

	for _, v := range vals {
		if v.IsConst() {
			switch width {
			case 1:
				cur.Byte(uint8(v.Const))
			case 2:
				cur.Word(uint16(v.Const))
			case 4:
				cur.Long(uint32(v.Const))
			case 8:
				cur.Quad(uint64(v.Const))
			}
			continue
		}

		if v.Sub != "" || v.Add == "" {
			return fmt.Errorf("%w: %s is not an address a relocation can name", ErrForm, v)
		}

		switch width {
		case 4:
			cur.LongRef(Ref(v.Add, RelocNone).At(v.Const))
		case 8:
			cur.QuadRef(Ref(v.Add, RelocNone).At(v.Const))
		default:
			return fmt.Errorf("%w: unsupported data reference width %d", ErrForm, width)
		}
	}
	return nil
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