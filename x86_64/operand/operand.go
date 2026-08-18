// x86_64/operand/operand.go
//
// Package operand is the operand set: immediates, memory references, and the
// symbolic targets a fixup can point at.
//
// There is no Operand interface here. Registers live in reg/, this package
// imports reg/, and reg/ implementing an interface declared here would close
// the cycle. The root package imports both and declares the interface; encode/
// imports both and switches on concrete types.
package operand

// Width is an operand width in bits. Zero means unspecified — a state `lea`
// genuinely occupies, since it computes an address and never loads through it.
type Width int

const (
	WidthNone Width = 0
	W8        Width = 8
	W16       Width = 16
	W32       Width = 32
	W64       Width = 64
	W128      Width = 128
	W256      Width = 256
	W512      Width = 512
)

func (w Width) Bytes() int { return int(w) / 8 }

func (w Width) String() string {
	switch w {
	case WidthNone:
		return "unsized"
	case W8:
		return "byte"
	case W16:
		return "word"
	case W32:
		return "dword"
	case W64:
		return "qword"
	case W128:
		return "xmmword"
	case W256:
		return "ymmword"
	case W512:
		return "zmmword"
	}
	return "?"
}