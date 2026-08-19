package decode

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vertex-language/arc/aarch64/isa"
)

// Explanation is the field-by-field breakdown arc explain prints: one line per
// field, naming it, its contents, and what it does to the instruction.
//
// The point is not "here are some bytes" but "here is why these are the bytes",
// which is why every line carries a note and why the alias line is part of the
// answer rather than a footnote. mov x8, #93 and movz x8, #93 are the same
// word, and a breakdown naming only one of them leaves a reader unable to match
// it against either the source or the disassembly.
type Explanation struct {
	Word uint32

	// Text is the instruction as a printer would write it, alias applied.
	Text string

	// Form is the underlying encoding.
	Form *isa.Form

	// Alias is the preferred alias, or nil.
	Alias *isa.Form

	// Fields are the breakdown lines, most significant bit first.
	Fields []Field
}

// Field is one line of a breakdown.
type Field struct {
	// Name is the architecture's name for the field where the table states one,
	// or the operand's role where it does not.
	Name string

	// Bits is the field's contents, formatted the way the field is read:
	// binary for a register or a small selector, hex for a wide immediate.
	Bits string

	// Pos is the bit range, [31] or [20:5] or a split field's two ranges.
	Pos string

	// Note is what the contents do: the register they name, the value they
	// hold, the operand size they select.
	Note string
}

// Explain decodes a word and breaks it down.
func Explain(b []byte) (Explanation, error) {
	in, err := Decode(b)
	if err != nil {
		return Explanation{}, err
	}
	return ExplainInst(in), nil
}

// ExplainInst breaks down an instruction already decoded.
func ExplainInst(in Inst) Explanation {
	e := Explanation{
		Word:  in.Word,
		Form:  in.Form,
		Alias: in.Alias,
		Text:  render(in),
	}

	e.Fields = append(e.Fields, Field{
		Name: "word",
		Bits: fmt.Sprintf("%08x", in.Word),
		Pos:  formTitle(in.Form),
		Note: aliasNote(in),
	})

	// The fixed bits, as one line. The table states a form's Word and Mask but
	// gives no name to any run within them, so this cannot break them out as
	// sf, opc and the rest the way the ARM ARM's diagram does. Naming them
	// needs the table to carry named fixed fields — see the note in the
	// package README; until then the honest line is the opcode as a whole.
	e.Fields = append(e.Fields, Field{
		Name: "opcode",
		Bits: fmt.Sprintf("%08x", in.Form.Word),
		Pos:  fmt.Sprintf("mask %08x", in.Form.Mask),
		Note: in.Form.Signature(),
	})

	for _, s := range in.Form.Slots {
		if s.Field.N == 0 {
			continue
		}
		v := s.Field.Get(in.Word)
		e.Fields = append(e.Fields, Field{
			Name: slotName(s),
			Bits: format(s, v),
			Pos:  s.Field.String(),
			Note: note(in, s, v),
		})
	}
	return e
}

func render(in Inst) string {
	var b strings.Builder
	b.WriteString(in.Mnem())
	for i, o := range in.Ops {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteString(", ")
		}
		if s, ok := o.(fmt.Stringer); ok {
			b.WriteString(s.String())
			continue
		}
		b.WriteString(fmt.Sprint(o))
	}
	return b.String()
}

func formTitle(f *isa.Form) string {
	return strings.ToUpper(f.Mnem) + " (" + widthLabel(f) + ")"
}

func widthLabel(f *isa.Form) string {
	switch regWidth(f) {
	case 32:
		return "32-bit"
	case 64:
		return "64-bit"
	}
	return f.GoName()
}

func aliasNote(in Inst) string {
	if in.Alias == nil {
		return ""
	}
	return "alias: " + in.Alias.Mnem
}

func slotName(s isa.Slot) string {
	if s.Class == isa.ClassImm || s.Class == isa.ClassLabel {
		return "imm" + strconv.Itoa(int(s.Field.Width()))
	}
	switch s.Role {
	case isa.RoleDest:
		return "Rd"
	case isa.RoleBase:
		return "Rn"
	case isa.RoleTarget, isa.RolePage:
		return "imm" + strconv.Itoa(int(s.Field.Width()))
	case isa.RoleOffset:
		return "off"
	case isa.RoleModifier:
		return "opt"
	}
	return "Rm"
}

// format writes a field's contents the way that field is read: a register
// number in binary, because its bits are what the encoding names; a wide
// immediate in hex, because its value is.
func format(s isa.Slot, v uint64) string {
	w := int(s.Field.Width())
	if s.Class.Reg() || w <= 4 {
		return pad(strconv.FormatUint(v, 2), w)
	}
	return fmt.Sprintf("%#0*x", (w+3)/4, v)
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat("0", w-len(s)) + s
}

// note is what the field's contents do.
func note(in Inst, s isa.Slot, v uint64) string {
	if s.Class.Reg() {
		if r, ok := regOf(s.Class, v); ok {
			return "→ " + r.String()
		}
		return "no such register"
	}
	if s.Class == isa.ClassImm || s.Class == isa.ClassLabel {
		val, _, err := immOf(in.Form, s, in.Word)
		if err != nil {
			return err.Error()
		}
		if st, ok := val.(fmt.Stringer); ok {
			return strings.TrimPrefix(st.String(), "#")
		}
	}
	switch s.Class {
	case isa.ClassShift:
		return shiftOf(in.Form, s, in.Word).String()
	case isa.ClassExtend:
		return extendOf(in.Form, s, in.Word).String()
	case isa.ClassCond:
		return "condition"
	}
	return ""
}

// String renders the breakdown the way arc explain prints it.
func (e Explanation) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-46s aarch64 · 4 bytes\n\n", e.Text)
	for _, f := range e.Fields {
		fmt.Fprintf(&b, "  %-8s %-10s %-24s %s\n", f.Name, f.Bits, f.Pos, f.Note)
	}
	return b.String()
}