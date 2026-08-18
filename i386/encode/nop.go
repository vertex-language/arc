package encode

import "github.com/vertex-language/arc/i386/feature"

// Multi-byte no-operation sequences, for padding a code section to an
// alignment. Padding .text with 0x00 produces a listing that disassembles
// into garbage, which is why this table exists at all.
//
// There are two tables and the base level chooses between them. The 0F 1F
// multi-byte NOP is available only on processors reporting family 6 or F —
// Pentium Pro and later, which is exactly the i686 level — and raises #UD on
// anything earlier. Below i686 the filler is the lea-based sequence GNU as
// uses, which is architecturally a real instruction with no effect.
//
// This is the reason Align takes a feature set rather than being a constant
// table: at --features i486 the P6 sequence is not a slower nop, it is an
// illegal instruction.

// p6Nops is Intel SDM Vol. 2, Table 4-12, indexed by length.
var p6Nops = [10][]byte{
	1: {0x90},
	2: {0x66, 0x90},
	3: {0x0f, 0x1f, 0x00},
	4: {0x0f, 0x1f, 0x40, 0x00},
	5: {0x0f, 0x1f, 0x44, 0x00, 0x00},
	6: {0x66, 0x0f, 0x1f, 0x44, 0x00, 0x00},
	7: {0x0f, 0x1f, 0x80, 0x00, 0x00, 0x00, 0x00},
	8: {0x0f, 0x1f, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
	9: {0x66, 0x0f, 0x1f, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
}

// genericNops are the pre-P6 fillers: mov %esi,%esi and lea forms that touch
// nothing. Lengths 5 and 8 are a one-byte nop followed by the 4- and 7-byte
// sequences, as GNU as builds them.
var genericNops = [9][]byte{
	1: {0x90},
	2: {0x89, 0xf6},
	3: {0x8d, 0x76, 0x00},
	4: {0x8d, 0x74, 0x26, 0x00},
	5: {0x90, 0x8d, 0x74, 0x26, 0x00},
	6: {0x8d, 0xb6, 0x00, 0x00, 0x00, 0x00},
	7: {0x8d, 0xb4, 0x26, 0x00, 0x00, 0x00, 0x00},
	8: {0x90, 0x8d, 0xb4, 0x26, 0x00, 0x00, 0x00, 0x00},
}

// Nops returns exactly n bytes of padding for a code section.
//
// Runs longer than the table are built from repeated maximum-length
// sequences plus one remainder, longest first, so the output is deterministic
// for a given n and level.
func Nops(n int, s feature.Set) []byte {
	if n <= 0 {
		return nil
	}
	table := genericNops[:]
	if s.AtLeast(feature.I686) {
		table = p6Nops[:]
	}
	max := len(table) - 1

	out := make([]byte, 0, n)
	for n >= max {
		out = append(out, table[max]...)
		n -= max
	}
	if n > 0 {
		out = append(out, table[n]...)
	}
	return out
}