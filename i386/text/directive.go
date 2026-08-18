package text

// Directive semantics. Not directive spellings — those are per-dialect and
// live in gas/ and nasm/, because `.long` and `dd` are two names for one
// thing and neither is canonical.
//
// What is here is the part that is the arch's rather than the syntax's, and
// the reason this file exists at all is one constant: a word is two bytes on
// i386 and four on ARM, AArch64 and RISC-V. `.word` therefore cannot mean
// anything portable, the Go API declines to offer a name whose width depends
// on which package you are in, and the text layer resolves it per arch — in
// this file, once.

// Width is a data or operand width in bytes. The value is the byte count, so
// Width8 is 1.
//
// Naming widths by their bit count rather than by byte, word, long, quad or
// by byte, word, dword, qword is deliberate: those are two dialects' names
// for one set of sizes, and picking either would put a spelling in the
// package that is supposed to have none.
type Width uint8

const (
	WidthNone Width = 0
	Width8    Width = 1
	Width16   Width = 2
	Width32   Width = 4
	Width64   Width = 8
	Width80   Width = 10
	Width128  Width = 16
)

// Bytes is the width in bytes.
func (w Width) Bytes() int { return int(w) }

// Bits is the width in bits.
func (w Width) Bits() int { return int(w) * 8 }

func (w Width) String() string {
	switch w {
	case WidthNone:
		return "unsized"
	case Width80:
		return "80-bit"
	}
	if w == 0 {
		return "?"
	}
	return itoa(int(w)*8) + "-bit"
}

// WordWidth is what a word is on i386: two bytes.
//
// This is the constant arm/text spells four. Every dialect directive that
// says "word" resolves through here rather than through a literal, so the
// fact appears once in the package and moving arches cannot silently keep it.
const WordWidth = Width16

// DataWidth reports whether w may be the width of a data directive.
//
// 80 is an operand width and not a data width: it exists because fldt and
// fstpt take an 80-bit memory operand, and the only way to write eighty bits
// of initialised data is a float literal, which arc does not accept. The
// builder API declares seven data calls and none of them is a float; a text
// layer that took one would be a thing .s files could say and Go could not.
func DataWidth(w Width) bool {
	switch w {
	case Width8, Width16, Width32, Width64, Width128:
		return true
	}
	return false
}

// Fits reports whether v is representable in w bytes, read either as signed
// or as unsigned. Both readings are accepted because .byte 0xff and .byte -1
// are both ordinary and denote the same byte; only a value outside both
// ranges is an error.
func (w Width) Fits(v int64) bool {
	if w == WidthNone || w >= Width64 {
		return true
	}
	bits := uint(w) * 8
	min := int64(-1) << (bits - 1)
	max := int64(1)<<bits - 1
	return v >= min && v <= max
}

// TruncationError is the diagnostic for a value that does not fit. It names
// both readings, because a caller who wrote 0xffff into a byte usually meant
// one of them.
func TruncationError(p Pos, w Width, v int64) *Error {
	bits := uint(w) * 8
	return Errorf(p, "value %d does not fit in %s", v, w).
		Note("the range is %d to %d signed, 0 to %d unsigned",
			int64(-1)<<(bits-1), int64(1)<<(bits-1)-1, int64(1)<<bits-1)
}

// StandardSection resolves a section name to its portable kind.
//
// The names are the ones both dialects write and every object format has a
// mapping for. Anything else is SectionCustom, and a custom section's name
// and flags pass through verbatim: arc does not classify a section it was not
// told about, and it does not invent flags for one.
//
// The .text.hot and .data.rel.ro families are deliberately not matched by
// prefix. `.text.hot` is a section whose name begins with .text, not a text
// section with a suffix, and a linker that groups them does so by name — a
// rule that belongs to the linker and not to a parser.
func StandardSection(name string) (SectionKind, bool) {
	switch name {
	case ".text":
		return SectionText, true
	case ".data":
		return SectionData, true
	case ".rodata":
		return SectionROData, true
	case ".bss":
		return SectionBSS, true
	case ".eh_frame":
		return SectionUnwind, true
	case ".init_array", ".ctors":
		return SectionInitArray, true
	case ".fini_array", ".dtors":
		return SectionFiniArray, true
	case ".tdata", ".tbss":
		return SectionTLS, true
	}
	return SectionCustom, false
}

// AlignBoundary converts a power-of-two exponent to a byte count, for the
// .p2align spelling. The two spellings are one boundary, so the tree stores
// one and the printer writes back whichever the source used.
func AlignBoundary(p Pos, exp int64) (int64, *Error) {
	if exp < 0 || exp > 31 {
		return 0, Errorf(p, "alignment exponent %d is out of range", exp).
			Note("the boundary is 2**n bytes; n must be 0 through 31")
	}
	return int64(1) << uint(exp), nil
}

// CheckAlign reports whether n is a usable byte boundary.
func CheckAlign(p Pos, n int64) *Error {
	if n <= 0 || n&(n-1) != 0 {
		return Errorf(p, "alignment %d is not a power of two", n)
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}