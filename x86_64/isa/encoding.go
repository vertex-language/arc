// x86_64/isa/encoding.go
package isa

// Enc is the encoding scheme: which prefix carries the register extension
// bits. It is not a feature — an AVX-512 form is EVEX because EVEX is where
// its opmask lives, not because AVX512F is enabled.
type Enc uint8

const (
	EncLegacy Enc = iota // no prefix, or REX
	EncVEX
	EncEVEX
)

func (e Enc) String() string {
	switch e {
	case EncVEX:
		return "VEX"
	case EncEVEX:
		return "EVEX"
	}
	return "legacy"
}

// Map is the opcode map the primary opcode byte lives in. Legacy spells it
// with escape bytes; VEX and EVEX spell it as an mmmmm field, and the same
// value means the same map in both.
type Map uint8

const (
	Map1     Map = iota // one-byte opcodes
	Map0F               // 0F xx
	Map0F38             // 0F 38 xx
	Map0F3A             // 0F 3A xx
)

func (m Map) String() string {
	switch m {
	case Map0F:
		return "0F"
	case Map0F38:
		return "0F38"
	case Map0F3A:
		return "0F3A"
	}
	return ""
}

// Escape is the legacy escape bytes this map costs. Zero-length for Map1.
func (m Map) Escape() []byte {
	switch m {
	case Map0F:
		return []byte{0x0f}
	case Map0F38:
		return []byte{0x0f, 0x38}
	case Map0F3A:
		return []byte{0x0f, 0x3a}
	}
	return nil
}

// Pfx is the mandatory prefix, which VEX and EVEX compress into pp. It is
// deliberately not the same field as Attr's Data16: both emit 0x66 in a
// legacy encoding, but one selects an instruction and the other selects an
// operand size, and a decoder that conflates them mis-reads every SSE form.
type Pfx uint8

const (
	PfxNone Pfx = iota // NP
	Pfx66
	PfxF3
	PfxF2
)

func (p Pfx) String() string {
	switch p {
	case Pfx66:
		return "66"
	case PfxF3:
		return "F3"
	case PfxF2:
		return "F2"
	}
	return "NP"
}

// Byte is the prefix byte, or zero for NP.
func (p Pfx) Byte() byte {
	switch p {
	case Pfx66:
		return 0x66
	case PfxF3:
		return 0xf3
	case PfxF2:
		return 0xf2
	}
	return 0
}

// WBit is REX.W, VEX.W or EVEX.W depending on Enc. WIG means the bit is
// ignored and the encoder may emit either; it is not the same as W0, and a
// table that wrote W0 for WIG would refuse a legal re-encoding.
type WBit uint8

const (
	W0 WBit = iota
	W1
	WIG
)

func (w WBit) String() string {
	switch w {
	case W1:
		return "W1"
	case WIG:
		return "WIG"
	}
	return "W0"
}

// VLen is the vector length field: VEX.L, or EVEX.L'L.
type VLen uint8

const (
	LNone VLen = iota // not a vector encoding
	L128
	L256
	L512
	LZ  // VEX.LZ — the length field must be zero, as in the BMI forms
	LIG // ignored
)

func (l VLen) String() string {
	switch l {
	case L128:
		return "128"
	case L256:
		return "256"
	case L512:
		return "512"
	case LZ:
		return "LZ"
	case LIG:
		return "LIG"
	}
	return ""
}

// ImmW is the width of the immediate field. Derived from the slots, never
// written in the table: two places to state it is one place to get it wrong.
type ImmW uint8

const (
	ImmNone ImmW = iota
	ImmB         // ib — one byte
	ImmWord      // iw — two
	ImmD         // id — four
	ImmQ         // io — eight
)

func (i ImmW) Bytes() int {
	switch i {
	case ImmB:
		return 1
	case ImmWord:
		return 2
	case ImmD:
		return 4
	case ImmQ:
		return 8
	}
	return 0
}

func (i ImmW) String() string {
	switch i {
	case ImmB:
		return "ib"
	case ImmWord:
		return "iw"
	case ImmD:
		return "id"
	case ImmQ:
		return "io"
	}
	return ""
}

// Tuple is the EVEX disp8*N tuple type. N depends on the tuple, the vector
// length and EVEX.b, so the table states the tuple and encode/ computes N;
// a table of N values would be a table with a vector-length axis folded into
// it, which is how a disp8 ends up eight times too small.
type Tuple uint8

const (
	TupleNone Tuple = iota
	TupleFull
	TupleHalf
	TupleFullMem
	Tuple1Scalar
	Tuple1Fixed
	Tuple2
	Tuple4
	Tuple8
	TupleHalfMem
	TupleQuarterMem
	TupleEighthMem
	TupleMem128
	TupleMOVDDUP
)

var tupleNames = [...]string{
	TupleNone: "", TupleFull: "Full", TupleHalf: "Half",
	TupleFullMem: "Full Mem", Tuple1Scalar: "Tuple1 Scalar",
	Tuple1Fixed: "Tuple1 Fixed", Tuple2: "Tuple2", Tuple4: "Tuple4",
	Tuple8: "Tuple8", TupleHalfMem: "Half Mem", TupleQuarterMem: "Quarter Mem",
	TupleEighthMem: "Eighth Mem", TupleMem128: "Mem128", TupleMOVDDUP: "MOVDDUP",
}

func (t Tuple) String() string {
	if int(t) >= len(tupleNames) {
		return "?"
	}
	return tupleNames[t]
}

// Attr is the set of yes-or-no facts about a form that are not operands.
type Attr uint16

const (
	HasModRM  Attr = 1 << iota // derived, not written
	PlusReg                    // +rb/+rw/+rd/+ro
	Data16                     // legacy operand-size override, the 0x66 that is not a mandatory prefix
	Lockable                   // LOCK is legal on this form
	Default64                  // 64-bit operand size without REX.W: push, pop, call, jmp, ret
	Masked                     // takes {k1}
	Zeroing                    // takes {z}; implies Masked
	Broadcast                  // takes {1toN} — Elem says how wide an element
	SAE                        // takes {sae}
	RoundCtl                   // takes {er}; implies SAE
	Branch                     // control transfer — the fixup is PC-relative
	Terminal                   // control does not fall through: ret, jmp, ud2, hlt
)