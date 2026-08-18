// x86_64/decode/explain.go
package decode

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/reg"
)

// fieldKind is what a run of bytes in the encoding is.
type fieldKind uint8

const (
	fieldPrefix fieldKind = iota
	fieldREX
	fieldVEX
	fieldEVEX
	fieldEscape
	fieldOpcode
	fieldModRM
	fieldSIB
	fieldDisp
	fieldImm
	fieldRel
	fieldIS4
)

var fieldNames = [...]string{
	fieldPrefix: "prefix", fieldREX: "REX", fieldVEX: "VEX", fieldEVEX: "EVEX",
	fieldEscape: "escape", fieldOpcode: "opcode", fieldModRM: "ModRM",
	fieldSIB: "SIB", fieldDisp: "disp", fieldImm: "imm", fieldRel: "rel",
	fieldIS4: "imm8[7:4]",
}

// Field is one run of bytes of an encoding, with what it is and what it
// says. This is what `arc explain` prints, one line per field.
type Field struct {
	// Offset and Bytes locate the field in the instruction.
	Offset int
	Bytes  []byte

	// Name is the field's name: REX, opcode, ModRM, imm32.
	Name string

	// Detail is the field's contents in its own terms — the bit
	// assignments of a prefix, the mod/reg/rm split of a ModRM byte, the
	// form the opcode selected.
	Detail string

	// Meaning is what the field does to the instruction, in a reader's
	// terms rather than the manual's.
	Meaning string
}

// Explanation is a decoded instruction broken into its fields.
type Explanation struct {
	Inst   *Inst
	Fields []Field

	// Feature is the extension that gates the form, for the header line.
	// It is isa's, and prints as the name a --features flag would take.
	Text string
}

// Explain decodes one instruction and breaks its encoding into fields.
//
// The output is the thing an assembler is for: not "here are some bytes"
// but "here is why these are the bytes." Every line names a field, its
// contents, and what it does — so a reader checking an encoding against the
// manual has the manual's own vocabulary in front of them.
func Explain(b []byte) (*Explanation, error) {
	d := &dec{b: b}
	if err := d.run(); err != nil {
		return nil, err
	}
	in, err := d.inst()
	if err != nil {
		return nil, err
	}

	ex := &Explanation{Inst: in, Text: d.form.String()}
	for _, s := range d.spans {
		if s.off+s.len > len(b) {
			continue
		}
		ex.Fields = append(ex.Fields, d.field(s))
	}
	return ex, nil
}

func (d *dec) field(s span) Field {
	f := Field{
		Offset: s.off,
		Bytes:  d.b[s.off : s.off+s.len],
		Name:   fieldNames[s.kind],
	}

	switch s.kind {
	case fieldPrefix:
		f.Detail = Prefix(d.b[s.off]).String()
		f.Meaning = prefixMeaning(d.b[s.off])

	case fieldREX:
		c := d.b[s.off]
		f.Detail = fmt.Sprintf("W=%d R=%d X=%d B=%d",
			c>>3&1, c>>2&1, c>>1&1, c&1)
		if c>>3&1 == 1 {
			f.Meaning = "64-bit operand size"
		} else {
			f.Meaning = "register extension"
		}

	case fieldVEX:
		f.Detail = fmt.Sprintf("%s.%s.%s.W%d",
			d.vlenName(), d.vpp, d.vmap, d.vw)
		f.Meaning = fmt.Sprintf("vvvv=%s", d.vvvvName())

	case fieldEVEX:
		f.Detail = fmt.Sprintf("%s.%s.%s.W%d aaa=%d z=%d b=%d",
			d.vlenName(), d.vpp, d.vmap, d.vw, d.aaa, boolBit(d.zero), boolBit(d.bcst))
		f.Meaning = d.evexMeaning()

	case fieldEscape:
		f.Detail = d.legacyMap.String()
		f.Meaning = "opcode map"

	case fieldOpcode:
		f.Detail = d.form.String()
		if d.form.Ext != isa.NoExt {
			f.Meaning = fmt.Sprintf("/%d", d.form.Ext)
		} else if d.form.Attrs&isa.PlusReg != 0 {
			f.Meaning = "+r"
		}

	case fieldModRM:
		c := d.b[s.off]
		f.Detail = fmt.Sprintf("mod=%02b reg=%03b rm=%03b", c>>6, c>>3&7, c&7)
		f.Meaning = d.modrmMeaning()

	case fieldSIB:
		c := d.b[s.off]
		f.Detail = fmt.Sprintf("scale=%d index=%03b base=%03b",
			1<<(c>>6), c>>3&7, c&7)
		f.Meaning = d.sibMeaning()

	case fieldDisp:
		f.Name = fmt.Sprintf("disp%d", s.len*8)
		f.Detail = fmt.Sprint(d.disp)
		f.Meaning = d.dispMeaning(s.len)

	case fieldImm:
		f.Name = fmt.Sprintf("imm%d", s.len*8)
		f.Detail = fmt.Sprint(d.imm)
		f.Meaning = fmt.Sprintf("%#x", uint64(d.imm))

	case fieldRel:
		f.Name = fmt.Sprintf("rel%d", s.len*8)
		f.Detail = fmt.Sprint(d.imm)
		f.Meaning = "from the end of the instruction"

	case fieldIS4:
		f.Detail = fmt.Sprintf("%d", d.is4)
		f.Meaning = "fourth operand"
	}
	return f
}

func prefixMeaning(c byte) string {
	switch c {
	case 0xf0:
		return "atomic read-modify-write"
	case 0x66:
		return "16-bit operand size"
	case 0x67:
		return "32-bit address size"
	case 0xf2, 0xf3:
		return "repeat, or a mandatory prefix"
	}
	return "segment override"
}

func (d *dec) modrmMeaning() string {
	if d.mod == 3 {
		if r, err := regOf(rmClass(d.form), d.rmNum()); err == nil {
			return "→ " + r.Name()
		}
		return "register"
	}
	if d.rip {
		return "→ [rip+disp32]"
	}
	if d.rm == 4 {
		return "→ SIB"
	}
	return "→ [" + reg.Reg64(d.bb<<3|d.rm).Name() + "]"
}

func (d *dec) sibMeaning() string {
	var parts []string
	if !(d.mod == 0 && d.base == 5) {
		parts = append(parts, reg.Reg64(d.bb<<3|d.base).Name())
	}
	if d.index != 4 || d.x == 1 {
		parts = append(parts,
			fmt.Sprintf("%s*%d", reg.Reg64(d.x<<3|d.index).Name(), d.scale))
	}
	return "→ [" + strings.Join(parts, "+") + "]"
}

func (d *dec) dispMeaning(n int) string {
	if n == 1 && d.dispScale() > 1 {
		// The compressed form is worth spelling out, because the byte in
		// the encoding is not the number it stands for.
		return fmt.Sprintf("%d × %d (disp8*N)", d.disp/int32(d.dispScale()), d.dispScale())
	}
	if d.rip {
		return "from the end of the instruction"
	}
	return fmt.Sprintf("%#x", uint32(d.disp))
}

func (d *dec) evexMeaning() string {
	var parts []string
	if d.aaa != 0 {
		parts = append(parts, "masked by k"+fmt.Sprint(d.aaa))
	}
	if d.zero {
		parts = append(parts, "zeroing")
	}
	if d.bcst {
		if d.mod == 3 {
			parts = append(parts, "rounding or SAE")
		} else {
			parts = append(parts, "broadcast")
		}
	}
	if len(parts) == 0 {
		return "vector encoding"
	}
	return strings.Join(parts, ", ")
}

func (d *dec) vlenName() string {
	switch d.ll {
	case 1:
		return "256"
	case 2:
		return "512"
	}
	return "128"
}

func (d *dec) vvvvName() string {
	if d.vvvvNum() == 0 && d.vvvv == 0 {
		return "unused"
	}
	return fmt.Sprintf("%d", d.vvvvNum())
}

func rmClass(f *isa.Form) isa.Class {
	for _, s := range f.Slots {
		if s.Field == isa.InRM {
			return s.Class
		}
	}
	return isa.ClassNone
}

func boolBit(b bool) int {
	if b {
		return 1
	}
	return 0
}