// x86_64/encode/nop.go
//
// The multi-byte no-op tables Align pads a code section with.
//
// These are the encodings Intel and AMD both document as the recommended
// padding, and the ones GNU as emits, which matters because the differential
// suite compares padded sections byte for byte. They are not the only legal
// no-ops and they are not chosen for beauty: a decoder walking a padded
// section has to get the same instruction boundaries either tool produces.
package encode

// nopTable is one canonical encoding per length, 1 through 9 bytes.
//
//	1  90                             xchg eax, eax
//	2  66 90                          66 xchg ax, ax
//	3  0f 1f 00                       nop dword [rax]
//	4  0f 1f 40 00                    nop dword [rax+0]
//	5  0f 1f 44 00 00                 nop dword [rax+rax*1+0]
//	6  66 0f 1f 44 00 00              nop word  [rax+rax*1+0]
//	7  0f 1f 80 00 00 00 00           nop dword [rax+0]
//	8  0f 1f 84 00 00 00 00 00        nop dword [rax+rax*1+0]
//	9  66 0f 1f 84 00 00 00 00 00     nop word  [rax+rax*1+0]
var nopTable = [10][]byte{
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

// MaxNop is the longest single no-op this package will emit. Longer
// encodings exist — a 0x66 prefix can be repeated up to the fifteen-byte
// instruction limit — but more than one redundant prefix is slow to decode
// on parts that are still shipping, so padding above nine bytes is a
// sequence rather than one instruction.
const MaxNop = 9

// Nop is the canonical no-op of exactly n bytes, or nil if n is outside
// 1..MaxNop.
func Nop(n int) []byte {
	if n < 1 || n > MaxNop {
		return nil
	}
	out := make([]byte, len(nopTable[n]))
	copy(out, nopTable[n])
	return out
}

// Nops is n bytes of padding: as many nine-byte no-ops as fit, then one
// shorter one for the remainder. Align calls this and appends the result;
// nothing else in the tree pads a code section.
func Nops(n int) []byte {
	if n <= 0 {
		return nil
	}
	out := make([]byte, 0, n)
	for n > 0 {
		k := n
		if k > MaxNop {
			k = MaxNop
		}
		out = append(out, nopTable[k]...)
		n -= k
	}
	return out
}