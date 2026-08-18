// x86_64/decode/decode.go
//
// Package decode is the inverse of encode/: bytes to a form and operand
// values, and the field-by-field breakdown Explain prints.
//
// Its opcode maps are built from isa.All() at init and from nothing else. A
// second table here would be a second place the ISA is stated, and the two
// would disagree the first time a form was added to one of them — which is
// the failure mode this package is arranged to make impossible rather than
// unlikely.
//
// Decoding is not the reverse of Resolve. Resolve picks among forms that
// could encode an instruction; this identifies the one that did. Where two
// forms produce the same bytes — and a few do, because the architecture has
// redundant encodings — the earlier row of the table wins, so Decode is
// deterministic and Encode(Decode(b)) is b for everything a differential
// suite can reach.
package decode

import (
	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// Inst is one decoded instruction.
//
// Ops holds operand values of the same concrete types encode/ accepts:
// reg.Reg, operand.M8 through operand.M512, operand.Imm. They are in Intel
// order, one per explicit slot of Form. A caller can hand Ops straight back
// to encode.Encode with Form and get the bytes it started with — which is
// what `arc fmt`'s round-trip guarantee rests on, and what the differential
// suite checks on every instruction it generates.
type Inst struct {
	Form *isa.Form
	Ops  []any

	// Len is the instruction's length in bytes.
	Len int

	// Prefixes are the legacy prefixes that were present and are not part
	// of the form: LOCK, REP/REPNE where the form is not a string
	// instruction, and a segment override. A mandatory SIMD prefix is the
	// form's and does not appear here.
	Prefixes []Prefix

	// Rel is the branch displacement, for a form with isa.Branch. It is
	// relative to the end of the instruction; a caller that knows the
	// instruction's address turns it into a target.
	Rel int64

	// Mask, Zero, Broadcast and Round are the EVEX modifiers, recovered
	// from the prefix so a printer can spell {k1}{z} and {1to16}.
	Mask      reg.K
	Zero      bool
	Broadcast bool
	Round     RoundMode
	SAE       bool
}

// RoundMode mirrors encode.RoundMode. It is redeclared rather than imported
// because decode/ importing encode/ would put the two halves of the same
// question in a dependency order, and there is no reason this direction
// should be the one that survives.
type RoundMode uint8

const (
	RoundNone RoundMode = iota
	RoundNearest
	RoundDown
	RoundUp
	RoundZero
)

// Prefix is a legacy prefix byte that survived into the decoded instruction.
type Prefix uint8

const (
	PrefixLock  Prefix = 0xf0
	PrefixRepNE Prefix = 0xf2
	PrefixRep   Prefix = 0xf3
	PrefixES    Prefix = 0x26
	PrefixCS    Prefix = 0x2e
	PrefixSS    Prefix = 0x36
	PrefixDS    Prefix = 0x3e
	PrefixFS    Prefix = 0x64
	PrefixGS    Prefix = 0x65
)

func (p Prefix) String() string {
	switch p {
	case PrefixLock:
		return "lock"
	case PrefixRepNE:
		return "repne"
	case PrefixRep:
		return "rep"
	case PrefixES:
		return "es"
	case PrefixCS:
		return "cs"
	case PrefixSS:
		return "ss"
	case PrefixDS:
		return "ds"
	case PrefixFS:
		return "fs"
	case PrefixGS:
		return "gs"
	}
	return "?"
}

// Decode reads one instruction from the front of b.
//
// It decodes what the architecture decodes, not what this assembler emits:
// a redundant prefix, a disp32 where a disp8 would have fit, an empty REX
// where none was needed. Round-tripping those through Encode produces the
// shorter form, which is a difference `arc fmt` is allowed to make and
// `arc dis` is not.
func Decode(b []byte) (*Inst, error) {
	d := &dec{b: b}
	if err := d.run(); err != nil {
		return nil, err
	}
	return d.inst()
}

// DecodeAll decodes b until it is exhausted or an instruction fails to
// decode, returning what it got and the error. A caller disassembling a
// section wants both: the bytes before the failure are still instructions.
func DecodeAll(b []byte) ([]*Inst, error) {
	var out []*Inst
	for len(b) > 0 {
		in, err := Decode(b)
		if err != nil {
			return out, err
		}
		out = append(out, in)
		b = b[in.Len:]
	}
	return out, nil
}

// dec is one decode in progress.
type dec struct {
	b   []byte
	pos int

	// The prefix state, in the order the bytes are scanned.
	lock   bool
	rep    Prefix // 0xf2, 0xf3 or zero
	seg    Prefix
	data16 bool
	addr32 bool

	rex   byte
	hasRex bool

	enc  isa.Enc
	vmap isa.Map
	vpp  isa.Pfx
	vlen isa.VLen
	vw   byte

	// The extension bits, uninverted. r, x and b come from REX, VEX or
	// EVEX; rp and vp exist only under EVEX.
	r, x, bb, rp, vp byte
	vvvv             byte

	aaa  byte
	zero bool
	bcst bool // EVEX.b, before it is read as broadcast or as rounding
	ll   byte

	opcode byte
	form   *isa.Form

	// The decoded ModRM and what followed it.
	hasModRM bool
	mod      byte
	regf     byte
	rm       byte
	hasSIB   bool
	scale    uint8
	index    byte
	base     byte
	disp     int32
	hasDisp  bool
	rip      bool

	imm    int64
	hasImm bool
	is4    byte

	// The byte offsets of each field, for Explain.
	spans []span
}

type span struct {
	off  int
	len  int
	kind fieldKind
}

func (d *dec) run() error {
	if err := d.prefixes(); err != nil {
		return err
	}
	if err := d.opcodeByte(); err != nil {
		return err
	}
	if err := d.match(); err != nil {
		return err
	}
	if d.form.Attrs&isa.HasModRM != 0 {
		if err := d.modrm(); err != nil {
			return err
		}
	}
	return d.immediate()
}

func (d *dec) byteAt() (byte, error) {
	if d.pos >= len(d.b) {
		return 0, ErrTruncated
	}
	c := d.b[d.pos]
	d.pos++
	return c, nil
}

func (d *dec) peek() (byte, bool) {
	if d.pos >= len(d.b) {
		return 0, false
	}
	return d.b[d.pos], true
}

func (d *dec) mark(off, n int, k fieldKind) {
	d.spans = append(d.spans, span{off: off, len: n, kind: k})
}

// inst assembles the decoded fields into operand values.
func (d *dec) inst() (*Inst, error) {
	in := &Inst{
		Form: d.form,
		Len:  d.pos,
		Mask: reg.K(d.aaa),
		Zero: d.zero,
	}

	if d.lock {
		in.Prefixes = append(in.Prefixes, PrefixLock)
	}
	if d.seg != 0 {
		in.Prefixes = append(in.Prefixes, d.seg)
	}
	if d.rep != 0 && d.form.Pfx == isa.PfxNone && d.enc == isa.EncLegacy {
		in.Prefixes = append(in.Prefixes, d.rep)
	}

	if d.enc == isa.EncEVEX && d.bcst {
		if d.hasModRM && d.mod == 3 {
			// EVEX.b over a register operand is rounding control, and L'L
			// is the mode rather than the length. The same bit, read two
			// ways, decided by whether r/m is memory.
			if d.form.Attrs&isa.RoundCtl != 0 {
				in.Round = RoundMode(d.ll + 1)
			} else {
				in.SAE = true
			}
		} else {
			in.Broadcast = true
		}
	}

	ops, rel, err := d.operands()
	if err != nil {
		return nil, err
	}
	in.Ops = ops
	in.Rel = rel
	return in, nil
}

var (
	_ = operand.Imm(0)
	_ = reg.RAX
)