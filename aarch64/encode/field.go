package encode

import "github.com/vertex-language/arc/aarch64/isa"

// place writes a value into a field of a word.
//
// isa.Field.Put drops bits above the field's width without complaint, which is
// right for the table's own use and wrong here: this package has the operand
// and can name it, so every write goes through a range check first.
func place(word uint32, f isa.Field, v uint64) uint32 {
	return f.Put(word, v)
}

// fitsU reports whether v fits in n unsigned bits.
func fitsU(v uint64, bits uint8) bool {
	if bits >= 64 {
		return true
	}
	return v < uint64(1)<<bits
}

// fitsS reports whether v fits in n signed bits.
func fitsS(v int64, bits uint8) bool {
	if bits == 0 || bits > 63 {
		return false
	}
	lo := -(int64(1) << (bits - 1))
	hi := (int64(1) << (bits - 1)) - 1
	return v >= lo && v <= hi
}

// truncS is a signed value as the field's bit pattern.
func truncS(v int64, bits uint8) uint64 {
	if bits >= 64 {
		return uint64(v)
	}
	return uint64(v) & ((1 << bits) - 1)
}

// siblingField finds the field of the first slot of a given class, which is how
// a computed immediate reaches the field that carries its shift. MOVZ's hw and
// ADD's sh are separate slots from the immediate they belong to, because the
// syntax lets a caller write either of them, and the encoder has to be able to
// fill one it was not handed.
func siblingField(f *isa.Form, class isa.Class) (isa.Field, bool) {
	for _, s := range f.Slots {
		if s.Class == class {
			return s.Field, true
		}
	}
	return isa.Field{}, false
}