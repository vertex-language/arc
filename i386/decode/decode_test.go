package decode

import (
	"errors"
	"testing"

	"github.com/vertex-language/arc/i386/feature"
	"github.com/vertex-language/arc/i386/isa"
	"github.com/vertex-language/arc/i386/operand"
	"github.com/vertex-language/arc/i386/reg"
)

func dec(t *testing.T, b ...byte) Inst {
	t.Helper()
	i, err := Decode(b, feature.Default())
	if err != nil {
		t.Fatalf("% x: %v", b, err)
	}
	return i
}

// The encodings that appear in the documentation, read in the other
// direction. These are encode_test's TestRegisterOperands backwards.
func TestKnownEncodings(t *testing.T) {
	for _, c := range []struct {
		in   []byte
		sig  string
		ops  []operand.Operand
	}{
		{[]byte{0x89, 0xc8}, "MOV r/m32, r32", []operand.Operand{reg.EAX, reg.ECX}},
		{[]byte{0x31, 0xff}, "XOR r/m32, r32", []operand.Operand{reg.EDI, reg.EDI}},
		{[]byte{0xb8, 0x3c, 0, 0, 0}, "MOV r32, imm32", []operand.Operand{reg.EAX, operand.Imm(60)}},
		{[]byte{0xc7, 0xc0, 0x3c, 0, 0, 0}, "MOV r/m32, imm32", []operand.Operand{reg.EAX, operand.Imm(60)}},
		{[]byte{0x83, 0xc0, 0x01}, "ADD r/m32, imm8", []operand.Operand{reg.EAX, operand.Imm(1)}},
		{[]byte{0x05, 0x01, 0, 0, 0}, "ADD EAX, imm32", []operand.Operand{reg.EAX, operand.Imm(1)}},
		{[]byte{0x55}, "PUSH r32", []operand.Operand{reg.EBP}},
		{[]byte{0x5d}, "POP r32", []operand.Operand{reg.EBP}},
		{[]byte{0x41}, "INC r32", []operand.Operand{reg.ECX}},
		{[]byte{0xc3}, "RET", nil},
		{[]byte{0xc9}, "LEAVE", nil},
		{[]byte{0xf7, 0xd3}, "NOT r/m32", []operand.Operand{reg.EBX}},
		{[]byte{0x0f, 0xaf, 0xc1}, "IMUL r32, r/m32", []operand.Operand{reg.EAX, reg.ECX}},
		{[]byte{0xd1, 0xe0}, "SHL r/m32, 1", []operand.Operand{reg.EAX, operand.Imm(1)}},
		{[]byte{0x0f, 0xc8}, "BSWAP r32", []operand.Operand{reg.EAX}},
	} {
		i := dec(t, c.in...)
		if got := i.Form.Signature(); got != c.sig {
			t.Errorf("% x = %q, want %q", c.in, got, c.sig)
			continue
		}
		if i.Len() != len(c.in) {
			t.Errorf("%s: Len() = %d, want %d", c.sig, i.Len(), len(c.in))
		}
		if len(i.Ops) != len(c.ops) {
			t.Errorf("%s: %d operands, want %d", c.sig, len(i.Ops), len(c.ops))
			continue
		}
		for n, want := range c.ops {
			if i.Ops[n] != want {
				t.Errorf("%s: operand %d = %v, want %v", c.sig, n, i.Ops[n], want)
			}
		}
	}
}

// The four encodings whose fields do not mean what they say, decoded. This is
// encode_test's TestAddressingSpecialCases with the arrow reversed.
func TestAddressingSpecialCases(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		base reg.R32
		hasBase bool
		index reg.R32
		scale uint8
		hasIndex bool
		disp int32
	}{
		{"[eax]", []byte{0x8b, 0x00}, reg.EAX, true, 0, 0, false, 0},
		{"[ecx]+8", []byte{0x8b, 0x41, 0x08}, reg.ECX, true, 0, 0, false, 8},
		{"[ecx]+1000", []byte{0x8b, 0x81, 0xe8, 0x03, 0x00, 0x00}, reg.ECX, true, 0, 0, false, 1000},

		// mod=01 with an explicit zero: the mod=00 slot for rm=101 is disp32.
		{"[ebp]", []byte{0x8b, 0x45, 0x00}, reg.EBP, true, 0, 0, false, 0},

		// rm=100 is "SIB follows", never ESP.
		{"[esp]", []byte{0x8b, 0x04, 0x24}, reg.ESP, true, 0, 0, false, 0},
		{"[esp]+8", []byte{0x8b, 0x44, 0x24, 0x08}, reg.ESP, true, 0, 0, false, 8},

		{"[eax+ecx*4]", []byte{0x8b, 0x04, 0x88}, reg.EAX, true, reg.ECX, 4, true, 0},
		{"[ebp+ecx*4]", []byte{0x8b, 0x44, 0x8d, 0x00}, reg.EBP, true, reg.ECX, 4, true, 0},

		// SIB.base=101 with mod=00 is no base at all.
		{"[ecx*8+16]", []byte{0x8b, 0x04, 0xcd, 0x10, 0, 0, 0}, 0, false, reg.ECX, 8, true, 16},

		// Absolute. The same encoding is RIP-relative in 64-bit mode.
		{"[0x1234]", []byte{0x8b, 0x05, 0x34, 0x12, 0, 0}, 0, false, 0, 0, false, 0x1234},
	} {
		i := dec(t, c.in...)
		m, ok := i.Ops[1].(operand.Memory)
		if !ok {
			t.Errorf("%s: operand 1 is %T, want memory", c.name, i.Ops[1])
			continue
		}
		if b, has := m.Base(); has != c.hasBase || (has && b != c.base) {
			t.Errorf("%s: base %s,%v want %s,%v", c.name, b, has, c.base, c.hasBase)
		}
		if x, s, has := m.Index(); has != c.hasIndex || (has && (x != c.index || s != c.scale)) {
			t.Errorf("%s: index %s×%d,%v want %s×%d,%v", c.name, x, s, has, c.index, c.scale, c.hasIndex)
		}
		if d := m.Disp(); d != c.disp {
			t.Errorf("%s: disp %d, want %d", c.name, d, c.disp)
		}
		if i.Len() != len(c.in) {
			t.Errorf("%s: Len() = %d, want %d", c.name, i.Len(), len(c.in))
		}
	}
}

// An exact opcode match outranks an opcode-plus-register match, which is the
// whole of why 0x90 is a nop and not an xchg with itself.
func TestExactOpcodeBeatsPlusR(t *testing.T) {
	if got := dec(t, 0x90).Form.Signature(); got != "NOP" {
		t.Errorf("90 = %q, want NOP", got)
	}
	i := dec(t, 0x91)
	if got := i.Form.Signature(); got != "XCHG EAX, r32" {
		t.Fatalf("91 = %q, want XCHG EAX, r32", got)
	}
	if i.Ops[1] != operand.Operand(reg.ECX) {
		t.Errorf("91: operand 1 = %v, want ecx", i.Ops[1])
	}
}

// An alias emits its target, so a listing says what the silicon does. Nothing
// this package returns is an alias form.
func TestAliasesAreNeverDecoded(t *testing.T) {
	if got := dec(t, 0x74, 0x00).Form.Mnemonic; got != "je" {
		t.Errorf("74 = %q, want je (not jz)", got)
	}
	if got := dec(t, 0xd1, 0xe0).Form.Mnemonic; got != "shl" {
		t.Errorf("d1 /4 = %q, want shl (not sal)", got)
	}
	for _, e := range entries {
		if e.form.AliasOf != "" {
			t.Errorf("alias form %q is in the index", e.form.Signature())
		}
	}
}

// The segment override table is the inverse of encode's, and it is a switch
// on both sides because the prefix bytes are not in encoding-number order.
func TestSegmentOverride(t *testing.T) {
	for _, c := range []struct {
		b    byte
		want reg.Sreg
	}{
		{0x26, reg.ES}, {0x2e, reg.CS}, {0x36, reg.SS},
		{0x3e, reg.DS}, {0x64, reg.FS}, {0x65, reg.GS},
	} {
		i := dec(t, c.b, 0x8b, 0x00)
		if !i.Prefixes.HasSeg || i.Prefixes.Seg != c.want {
			t.Errorf("%#02x: segment %s, want %s", c.b, i.Prefixes.Seg, c.want)
		}
		m := i.Ops[1].(operand.Memory)
		if s, ok := m.Seg(); !ok || s != c.want {
			t.Errorf("%#02x: override not applied to the operand", c.b)
		}
	}
}

func TestPrefixes(t *testing.T) {
	i := dec(t, 0xf0, 0x0f, 0xb1, 0x0b) // lock cmpxchg [ebx], ecx
	if !i.Prefixes.Lock {
		t.Error("f0: lock not recorded")
	}
	if i.Form.Mnemonic != "cmpxchg" || i.Len() != 4 {
		t.Errorf("f0 0f b1 0b = %q, %d bytes", i.Form.Signature(), i.Len())
	}
}

// The address-size override selects a ModRM table operand/ does not model.
// Absent rather than half-supported, on both sides.
func TestAddressSizeOverrideRejected(t *testing.T) {
	_, err := Decode([]byte{0x67, 0x8b, 0x00}, feature.Default())
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("0x67 accepted: %v", err)
	}
}

// A disassembler walking to the end of a section needs "not all here" to be
// distinguishable from "not an instruction".
func TestTruncated(t *testing.T) {
	for _, b := range [][]byte{
		{},
		{0x89},
		{0x8b, 0x81, 0x00},
		{0xb8, 0x3c, 0x00},
		{0xe8, 0x01},
		{0x0f},
	} {
		_, err := Decode(b, feature.Default())
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("% x: err = %v, want truncated", b, err)
		}
	}
}

// 0x82 is a working duplicate of 0x80 that the table deliberately does not
// declare, so it does not decode either. The two facts are one fact.
func TestUnknownOpcode(t *testing.T) {
	for _, b := range [][]byte{
		{0x82, 0xc0, 0x01}, // the undeclared group-1 duplicate
		{0xf7, 0xc8},       // /1 of group 3, undocumented
		{0x8d, 0xc0},       // lea with mod=11
		{0x0f, 0x0b},       // ud2, not in the table
	} {
		_, err := Decode(b, feature.Default())
		if !errors.Is(err, ErrUnknown) {
			t.Errorf("% x: err = %v, want unknown opcode", b, err)
		}
	}
}

// Gating is the encoder's, applied in the other direction: bytes that would
// #UD on the selected target are an error naming the flag, not a decode.
func TestFeatureGating(t *testing.T) {
	cmove := []byte{0x0f, 0x44, 0xc1}

	i, err := Decode(cmove, feature.Default())
	if err != nil {
		t.Fatalf("cmove at baseline: %v", err)
	}
	if i.Form.Signature() != "CMOVE r32, r/m32" {
		t.Errorf("= %q", i.Form.Signature())
	}

	_, err = Decode(cmove, feature.New(feature.I586))
	if err == nil {
		t.Fatal("cmove decoded below i686")
	}
	if !errors.Is(err, ErrDecode) {
		t.Errorf("err = %v", err)
	}
}

// The mod field of a control-register move is ignored by the processor and
// treated as 11b. Refusing those bytes would be a decoder disagreeing with
// the silicon about what it just ran.
func TestControlRegisterModIsIgnored(t *testing.T) {
	for _, b := range [][]byte{{0x0f, 0x20, 0xc0}, {0x0f, 0x20, 0x00}} {
		i := dec(t, b...)
		if i.Form.Signature() != "MOV r32, CR" {
			t.Fatalf("% x = %q", b, i.Form.Signature())
		}
		if i.Ops[0] != operand.Operand(reg.EAX) || i.Ops[1] != operand.Operand(reg.CR0) {
			t.Errorf("% x: ops = %v, %v", b, i.Ops[0], i.Ops[1])
		}
	}
}

// A rel operand holds the displacement, because the name it had before
// assembly is not in the bytes and a decoder will not invent one.
func TestBranchDisplacement(t *testing.T) {
	i := dec(t, 0xe8, 0xfb, 0xff, 0xff, 0xff)
	if !i.HasRel || i.Rel != -5 {
		t.Fatalf("rel = %d, %v", i.Rel, i.HasRel)
	}
	if got, ok := i.Target(0x1000); !ok || got != 0x1000 {
		t.Errorf("Target = %#x, want %#x", got, 0x1000)
	}
	if i.Ops[0] != operand.Operand(operand.Imm(-5)) {
		t.Errorf("operand = %v, want -5", i.Ops[0])
	}

	if got := dec(t, 0x75, 0x02).Rel; got != 2 {
		t.Errorf("jne rel8 = %d, want 2", got)
	}
}

// The sign-extended byte is a different form from the four-byte immediate,
// and the difference is visible in the decoded value.
func TestImmediateSignedness(t *testing.T) {
	if got := dec(t, 0x83, 0xc0, 0xff).Ops[1]; got != operand.Operand(operand.Imm(-1)) {
		t.Errorf("imm8s = %v, want -1", got)
	}
	if got := dec(t, 0xcd, 0x80).Ops[0]; got != operand.Operand(operand.Imm(0x80)) {
		t.Errorf("imm8 = %v, want 128", got)
	}
	if got := dec(t, 0x05, 0xff, 0xff, 0xff, 0xff).Ops[1]; got != operand.Operand(operand.Imm(0xffffffff)) {
		t.Errorf("imm32 = %v, want 4294967295", got)
	}
}

// Every consumed byte belongs to exactly one field, in byte order. This is
// what lets a renderer lay out rows without re-deriving anything.
func TestExplainTilesTheInstruction(t *testing.T) {
	for _, b := range [][]byte{
		{0xc3},
		{0x89, 0xc8},
		{0xc7, 0xc0, 0x3c, 0, 0, 0},
		{0x65, 0x8b, 0x04, 0xcd, 0x10, 0, 0, 0},
		{0x8b, 0x44, 0x8d, 0x08},
		{0xe8, 0xfb, 0xff, 0xff, 0xff},
		{0xf0, 0x0f, 0xb1, 0x0b},
	} {
		x, err := Explain(b, feature.Default())
		if err != nil {
			t.Fatalf("% x: %v", b, err)
		}
		off := 0
		for _, f := range x.Fields {
			if f.Offset != off {
				t.Errorf("% x: field %s at %d, want %d", b, f.Name, f.Offset, off)
			}
			if f.Len != len(f.Bytes) {
				t.Errorf("% x: field %s Len %d, Bytes %d", b, f.Name, f.Len, len(f.Bytes))
			}
			off += f.Len
		}
		if off != x.Inst.Len() {
			t.Errorf("% x: fields cover %d bytes, instruction is %d", b, off, x.Inst.Len())
		}
	}
}

// ModRM and SIB decompose into three sub-fields each, and their bit positions
// are the SDM's.
func TestExplainBitFields(t *testing.T) {
	x, err := Explain([]byte{0x8b, 0x04, 0x88}, feature.Default())
	if err != nil {
		t.Fatal(err)
	}
	var modrm, sib *Field
	for i := range x.Fields {
		switch x.Fields[i].Kind {
		case FieldModRM:
			modrm = &x.Fields[i]
		case FieldSIB:
			sib = &x.Fields[i]
		}
	}
	if modrm == nil || sib == nil {
		t.Fatal("no ModRM or SIB field")
	}
	for _, c := range []struct {
		f    *Field
		want []BitField
	}{
		{modrm, []BitField{{Name: "mod", Hi: 7, Lo: 6}, {Name: "reg", Hi: 5, Lo: 3}, {Name: "rm", Hi: 2, Lo: 0}}},
		{sib, []BitField{{Name: "scale", Hi: 7, Lo: 6}, {Name: "index", Hi: 5, Lo: 3}, {Name: "base", Hi: 2, Lo: 0}}},
	} {
		if len(c.f.Bits) != 3 {
			t.Fatalf("%s: %d sub-fields, want 3", c.f.Name, len(c.f.Bits))
		}
		for i, w := range c.want {
			g := c.f.Bits[i]
			if g.Name != w.Name || g.Hi != w.Hi || g.Lo != w.Lo {
				t.Errorf("%s: sub-field %d = %s[%d:%d], want %s[%d:%d]",
					c.f.Name, i, g.Name, g.Hi, g.Lo, w.Name, w.Hi, w.Lo)
			}
		}
	}
	if got := modrm.Bits[0].Width() + modrm.Bits[1].Width() + modrm.Bits[2].Width(); got != 8 {
		t.Errorf("ModRM sub-fields cover %d bits, want 8", got)
	}
}

// The index is built from the form table and from nothing else, so every
// non-alias form is reachable and no form is registered in two maps.
func TestIndexCoversTheTable(t *testing.T) {
	seen := make(map[*isa.Form]int)
	for _, m := range []*[256][]*entry{&map1, &map0F, &map0F38, &map0F3A} {
		for _, bucket := range m {
			for _, e := range bucket {
				seen[e.form]++
			}
		}
	}
	for _, f := range isa.All() {
		n := seen[f]
		switch {
		case f.AliasOf != "":
			if n != 0 {
				t.Errorf("%s: alias registered %d times", f.Signature(), n)
			}
		case n == 0:
			t.Errorf("%s: not in any opcode map", f.Signature())
		}
	}
}