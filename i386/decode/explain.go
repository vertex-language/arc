package decode

import "github.com/vertex-language/arc/i386/feature"

// Explain is the field decomposition, as data.
//
// arc explain renders three ways — bytewise rows for variable-length x86, a
// bit ruler for fixed-width 32-bit, a 16-bit ruler for RVC and Thumb — and
// chooses by the encoding rather than a flag. All three are printers over a
// structure like this one, which is why there is no --format: the renderings
// differ, the data does not.
//
// The field names are the SDM's, and they are here at all because the form
// table is generated rather than hand-written. This is the answer to "why is
// my mov six bytes."
func Explain(b []byte, s feature.Set) (Explanation, error) {
	i, fields, err := walk(b, s)
	if err != nil {
		return Explanation{}, err
	}
	return Explanation{Inst: i, Fields: fields}, nil
}

// Explanation is one instruction broken into its fields.
//
// The fields tile the instruction exactly: they are in byte order, the first
// starts at 0, each begins where the last ended, and the last ends at
// Inst.Len(). Every consumed byte is accounted for by exactly one field,
// which is the property that makes a rendering possible without the renderer
// re-deriving anything.
type Explanation struct {
	Inst   Inst
	Fields []Field
}

// FieldKind is what a field is, for a renderer that wants to style prefixes
// differently from immediates.
type FieldKind uint8

const (
	FieldPrefix FieldKind = iota
	FieldOpcode
	FieldModRM
	FieldSIB
	FieldDisp
	FieldImm
	FieldRel
)

var fieldKindNames = [...]string{
	"prefix", "opcode", "ModRM", "SIB", "disp", "imm", "rel",
}

func (k FieldKind) String() string {
	if int(k) < len(fieldKindNames) {
		return fieldKindNames[k]
	}
	return "?"
}

// Field is one field of one instruction.
//
// Value is the decoded content and Note is what it means — the two columns
// after the field name in arc explain. Bits is the field's own decomposition
// where it has one: ModRM and SIB are three sub-fields each, and their bit
// positions are why "byte and bit offsets" is the shape of this structure and
// not just byte offsets.
type Field struct {
	Kind   FieldKind
	Name   string
	Offset int
	Len    int
	Bytes  []byte
	Value  string
	Note   string
	Bits   []BitField
}

// BitField is a sub-field of one byte. Hi and Lo are bit positions, most
// significant bit 7, both inclusive — the SDM's own numbering.
type BitField struct {
	Name   string
	Hi, Lo int
	Value  uint32
	Note   string
}

// Width is the number of bits the sub-field occupies.
func (b BitField) Width() int { return b.Hi - b.Lo + 1 }