package encode

import "encoding/binary"

// NopWord is the canonical no-op, d503201f.
//
// There is one, and it is one word. x86_64 needs a table of multi-byte no-ops
// because a decoder walking padding has to find the same instruction boundaries
// GNU as produced; here every instruction is four bytes and there is no
// question of where a decoder resumes.
const NopWord uint32 = 0xd503201f

// Nop returns the no-op word.
func Nop() uint32 { return NopWord }

// NopBytes is the no-op in memory order.
func NopBytes() []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], NopWord)
	return b[:]
}

// Pad appends n bytes of code padding. n must be a multiple of four; an
// alignment that is not is refused by Align in asm.go rather than rounded here,
// because rounding would put a partial instruction in a code section.
func Pad(dst []byte, n int) []byte {
	for i := 0; i+4 <= n; i += 4 {
		dst = append(dst, NopBytes()...)
	}
	return dst
}