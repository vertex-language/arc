// x86_64/encode/imm.go
package encode

import (
	"encoding/binary"

	"github.com/vertex-language/arc/x86_64/isa"
)

// immediate emits the immediate field. Its width is the form's, not the
// value's: isa.Resolve already picked the form whose field the value fits,
// and a caller who named MovR64Imm64 gets eight bytes whether or not the
// value needed them.
func (e *enc) immediate() error {
	if e.f.Imm == isa.ImmNone {
		return nil
	}

	// The is4 byte carries a register in its high nibble. When there is
	// also a real immediate the form declares only one field, and this
	// target has no form that wants both.
	if e.is4v != nil && e.immv == nil {
		e.emit(e.is4v.num() << 4)
		return nil
	}

	if e.immv == nil {
		return ErrNoImmediate
	}

	size := e.f.Imm.Bytes()

	switch e.immv.kind {
	case kTarget:
		// A branch displacement, relative to the end of the instruction.
		// close() computes the tail, so the addend written here is the
		// logical one and stays that way until the platform writer.
		e.fixup(Fixup{
			Size:   size,
			PCRel:  true,
			Use:    UseBranch,
			Kind:   e.immv.sym.Reloc(),
			Target: e.immv.sym,
			Addend: addendOf(e.immv.sym),
		})
		e.emit(make([]byte, size)...)
		return nil

	case kSymImm:
		e.fixup(Fixup{
			Size:   size,
			Use:    UseAbs,
			Kind:   e.immv.sym.Reloc(),
			Target: e.immv.sym,
			Addend: addendOf(e.immv.sym),
		})
		e.emit(make([]byte, size)...)
		return nil

	case kImm:
		v := int64(e.immv.imm)
		if !fits(v, size) {
			return &ImmediateError{Form: e.f, Value: v, Size: size}
		}
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(v))
		e.emit(b[:size]...)
		return nil
	}
	return ErrNoImmediate
}

// fits reports whether a value survives a field of this width. Every
// immediate on this target is sign-extended except imm64, which has nothing
// to extend into — so the test is signed range, and a caller who wrote an
// unsigned constant above the signed maximum wrote the same bits.
func fits(v int64, size int) bool {
	switch size {
	case 1:
		return v >= -128 && v <= 255
	case 2:
		return v >= -32768 && v <= 65535
	case 4:
		return v >= -2147483648 && v <= 4294967295
	case 8:
		return true
	}
	return false
}