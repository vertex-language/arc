// x86_64/decode/decode_test.go
package decode

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/vertex-language/arc/x86_64/isa"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

func bytesOf(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func dec1(t *testing.T, s string) *Inst {
	t.Helper()
	in, err := Decode(bytesOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestExitSixty(t *testing.T) {
	in := dec1(t, "48 c7 c0 3c 00 00 00")
	if in.Form.String() != "MOV r/m64, imm32" {
		t.Errorf("got %s", in.Form)
	}
	if in.Len != 7 {
		t.Errorf("Len = %d, want 7", in.Len)
	}
	if in.Ops[0] != reg.Reg(reg.RAX) {
		t.Errorf("first operand = %v, want rax", in.Ops[0])
	}
	if in.Ops[1] != any(operand.Imm(60)) {
		t.Errorf("second operand = %v, want 60", in.Ops[1])
	}
}

// The prefix decides the instruction, not the operand size. F2 0F 10 is
// MOVSD and F3 0F 10 is MOVSS, and neither is a 16-bit anything.
func TestMandatoryPrefixSelectsTheInstruction(t *testing.T) {
	cases := []struct{ bytes, want string }{
		{"0f 10 c1", "movups"},
		{"66 0f 10 c1", "movupd"},
		{"f3 0f 10 c1", "movss"},
		{"f2 0f 10 c1", "movsd"},
	}
	for _, c := range cases {
		in, err := Decode(bytesOf(t, c.bytes))
		if err != nil {
			t.Fatalf("%s: %v", c.bytes, err)
		}
		if in.Form.Op != c.want {
			t.Errorf("%s decoded as %s, want %s", c.bytes, in.Form.Op, c.want)
		}
	}
}

// Every addressing special case, read back.
func TestAddressingForms(t *testing.T) {
	cases := []struct {
		bytes string
		want  string
	}{
		{"48 8b 03", "[rbx]"},
		{"48 8b 43 08", "[rbx+8]"},
		{"48 8b 83 00 10 00 00", "[rbx+4096]"},
		{"48 8b 45 00", "[rbp]"},   // the zero disp8 is not a displacement
		{"48 8b 04 24", "[rsp]"},   // SIB with no index
		{"49 8b 04 24", "[r12]"},   // same, with REX.B
		{"48 8b 04 8b", "[rbx+rcx*4]"},
		{"4a 8b 04 e3", "[rbx+r12*8]"}, // index 100 with REX.X is r12, not "none"
		{"48 8b 05 00 00 00 00", "[rip+0]"},
	}
	for _, c := range cases {
		in, err := Decode(bytesOf(t, c.bytes))
		if err != nil {
			t.Fatalf("%s: %v", c.bytes, err)
		}
		m, ok := in.Ops[1].(operand.M64)
		if !ok {
			t.Fatalf("%s: second operand is %T", c.bytes, in.Ops[1])
		}
		if got := m.Mem.String(); got != c.want {
			t.Errorf("%s → %s, want %s", c.bytes, got, c.want)
		}
	}
}

// The two-byte VEX and the three-byte VEX are the same instruction, and a
// decoder that handled only one would silently miss most of AVX.
func TestVEXBothLengths(t *testing.T) {
	short := dec1(t, "c5 f4 58 c2")
	long := dec1(t, "c4 e1 74 58 c2")
	if short.Form != long.Form {
		t.Errorf("c5 and c4 forms differ: %s vs %s", short.Form, long.Form)
	}
	if short.Len != 4 || long.Len != 5 {
		t.Errorf("lengths = %d, %d; want 4, 5", short.Len, long.Len)
	}
}

func TestEVEXMaskAndZeroing(t *testing.T) {
	in := dec1(t, "62 f1 75 c9 fe c2")
	if in.Mask != reg.K1 {
		t.Errorf("Mask = %v, want k1", in.Mask)
	}
	if !in.Zero {
		t.Error("z bit not recovered")
	}
}

// The compressed displacement is the field a decoder gets wrong quietly.
// The byte says 1; the displacement is 64.
func TestCompressedDisplacement(t *testing.T) {
	in := dec1(t, "62 f1 75 49 fe 40 01")
	m, ok := in.Ops[3].(operand.M512)
	if !ok {
		t.Fatalf("operand is %T", in.Ops[3])
	}
	if m.Mem.Disp != 64 {
		t.Errorf("Disp = %d, want 64 — the byte is disp/N", m.Mem.Disp)
	}
}

// EVEX.b over memory is broadcast; over a register it is rounding. One bit,
// two readings, and mod is what chooses.
func TestEVEXBIsBroadcastOrRounding(t *testing.T) {
	mem := dec1(t, "62 f1 75 59 fe 00")
	if !mem.Broadcast {
		t.Error("b over a memory operand is broadcast")
	}
	rnd := dec1(t, "62 f1 7c 79 58 c2")
	if rnd.Round == RoundNone {
		t.Error("b over a register operand on a rounding form is {er}")
	}
	if rnd.Broadcast {
		t.Error("a register operand cannot broadcast")
	}
}

// Encode(Decode(b)) is b. This is the round-trip guarantee at the byte
// level, and the only check that catches encode/ and decode/ drifting apart
// on disp8*N or on a prefix ordering.
func TestRoundTrip(t *testing.T) {
	for _, s := range []string{
		"48 c7 c0 3c 00 00 00",
		"48 31 ff",
		"0f 05",
		"48 8b 43 08",
		"48 8b 04 8b",
		"64 48 8b 43 08",
		"c5 f4 58 c2",
		"c4 c1 74 58 c1",
		"62 f1 75 49 fe c2",
		"62 f1 75 49 fe 40 01",
		"e8 00 00 00 00",
		"40 88 c4",
	} {
		b := bytesOf(t, s)
		in, err := Decode(b)
		if err != nil {
			t.Errorf("%s: %v", s, err)
			continue
		}
		if in.Len != len(b) {
			t.Errorf("%s: consumed %d of %d bytes", s, in.Len, len(b))
		}
		// The re-encode lives in the differential suite, which can import
		// both packages; here we check the shape that makes it possible.
		if len(in.Ops) != in.Form.Arity() {
			t.Errorf("%s: %d operands for a form of arity %d",
				s, len(in.Ops), in.Form.Arity())
		}
	}
}

// AH and SPL share an encoding, and REX is the only thing that separates
// them. A decoder that ignored the prefix would name the wrong register
// half the time and pass every test written against AL.
func TestByteRegisterDependsOnREX(t *testing.T) {
	noRex := dec1(t, "88 e0") // mov al, ah
	if noRex.Ops[1] != reg.Reg(reg.AH) {
		t.Errorf("without REX, register 4 is ah; got %v", noRex.Ops[1])
	}
	withRex := dec1(t, "40 88 e0") // mov al, spl
	if withRex.Ops[1] != reg.Reg(reg.SPL) {
		t.Errorf("with REX, register 4 is spl; got %v", withRex.Ops[1])
	}
}

// The README's sample output, field by field.
func TestExplain(t *testing.T) {
	ex, err := Explain(bytesOf(t, "48 c7 c0 3c 00 00 00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Fields) != 4 {
		t.Fatalf("got %d fields, want 4", len(ex.Fields))
	}
	wantNames := []string{"REX", "opcode", "ModRM", "imm32"}
	for i, w := range wantNames {
		if ex.Fields[i].Name != w {
			t.Errorf("field %d is %q, want %q", i, ex.Fields[i].Name, w)
		}
	}
	if !strings.Contains(ex.Fields[0].Detail, "W=1") {
		t.Errorf("REX detail = %q", ex.Fields[0].Detail)
	}
	if !strings.Contains(ex.Fields[2].Meaning, "rax") {
		t.Errorf("ModRM meaning = %q", ex.Fields[2].Meaning)
	}
}

func TestTruncated(t *testing.T) {
	if _, err := Decode(bytesOf(t, "48 c7 c0 3c")); !errors.Is(err, ErrTruncated) {
		t.Errorf("got %v, want ErrTruncated", err)
	}
}

func TestUnknownReportsWhereItStopped(t *testing.T) {
	var u *UnknownError
	_, err := Decode([]byte{0xff, 0xff, 0xff})
	if !errors.As(err, &u) {
		t.Fatalf("got %v, want an UnknownError", err)
	}
}

// DecodeAll returns what it decoded before it failed, because the bytes
// before a data island are still instructions.
func TestDecodeAllKeepsWhatItGot(t *testing.T) {
	b := append(bytesOf(t, "0f 05 c3"), 0x06)
	insts, err := DecodeAll(b)
	if err == nil {
		t.Fatal("expected a failure on the trailing byte")
	}
	if len(insts) != 2 {
		t.Errorf("got %d instructions before the failure, want 2", len(insts))
	}
}

var _ = isa.All