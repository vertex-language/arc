package text

import (
	"strings"
	"testing"

	"github.com/vertex-language/arc/i386/reg"
)

var nowhere = Pos{File: "t.s", Line: 1, Col: 1}

func i(n int64) Expr        { return &Int{P: nowhere, Value: n} }
func s(name string) Expr    { return &SymExpr{P: nowhere, Name: name} }
func here() Expr            { return &Here{P: nowhere} }
func bin(o BinaryOp, x, y Expr) Expr {
	return &Binary{P: nowhere, Op: o, X: x, Y: y}
}

func evalOK(t *testing.T, e Expr) Value {
	t.Helper()
	v, err := Eval(e, nil)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	return v
}

// A word is two bytes here and four in arm/text. That constant is the reason
// this package exists.
func TestWordIsTwoBytes(t *testing.T) {
	if WordWidth != Width16 || WordWidth.Bytes() != 2 {
		t.Errorf("WordWidth = %v (%d bytes), want 16-bit", WordWidth, WordWidth.Bytes())
	}
	for _, c := range []struct {
		w     Width
		bytes int
		bits  int
	}{
		{Width8, 1, 8}, {Width16, 2, 16}, {Width32, 4, 32},
		{Width64, 8, 64}, {Width80, 10, 80}, {Width128, 16, 128},
	} {
		if c.w.Bytes() != c.bytes || c.w.Bits() != c.bits {
			t.Errorf("%v: %d bytes, %d bits", c.w, c.w.Bytes(), c.w.Bits())
		}
	}
}

// 80 bits is an operand width, not a data width: writing eighty bits of
// initialised data needs a float literal and arc has none.
func TestDataWidths(t *testing.T) {
	for _, w := range []Width{Width8, Width16, Width32, Width64, Width128} {
		if !DataWidth(w) {
			t.Errorf("%v rejected as a data width", w)
		}
	}
	for _, w := range []Width{WidthNone, Width80} {
		if DataWidth(w) {
			t.Errorf("%v accepted as a data width", w)
		}
	}
}

// Both readings of a literal are accepted; only a value outside both is an
// error. .byte 0xff and .byte -1 are the same byte.
func TestFits(t *testing.T) {
	for _, c := range []struct {
		w    Width
		v    int64
		want bool
	}{
		{Width8, 0, true}, {Width8, 255, true}, {Width8, -128, true},
		{Width8, 256, false}, {Width8, -129, false},
		{Width16, 65535, true}, {Width16, -32768, true}, {Width16, 65536, false},
		{Width32, 4294967295, true}, {Width32, 4294967296, false},
		{Width64, 1 << 62, true},
	} {
		if got := c.w.Fits(c.v); got != c.want {
			t.Errorf("%v.Fits(%d) = %v, want %v", c.w, c.v, got, c.want)
		}
	}
}

// The evaluator is GNU as's model: apart from + and -, both arguments must be
// absolute and the result is absolute.
func TestAbsoluteArithmetic(t *testing.T) {
	for _, c := range []struct {
		e    Expr
		want int64
	}{
		{bin(Add, i(1), i(2)), 3},
		{bin(Mul, bin(Add, i(1), i(2)), i(3)), 9},
		{bin(Div, i(7), i(2)), 3},
		{bin(Mod, i(7), i(2)), 1},
		{bin(Shl, i(1), i(4)), 16},
		{bin(Shr, i(-16), i(2)), -4},
		{bin(Or, i(1), i(2)), 3},
		{bin(Xor, i(3), i(1)), 2},
		{&Unary{P: nowhere, Op: Neg, X: i(5)}, -5},
		{&Unary{P: nowhere, Op: Not, X: i(0)}, -1},
	} {
		v := evalOK(t, c.e)
		if !v.IsAbs() || v.Const != c.want {
			t.Errorf("= %s, want %d", v, c.want)
		}
	}
}

// A comparison is -1 for true and a logical operator is 1. The asymmetry is
// GNU as's and changing it would change what a program assembles to.
func TestComparisonAndLogicalDiffer(t *testing.T) {
	if v := evalOK(t, bin(Eq, i(1), i(1))); v.Const != -1 {
		t.Errorf("1==1 = %d, want -1", v.Const)
	}
	if v := evalOK(t, bin(Lt, i(2), i(1))); v.Const != 0 {
		t.Errorf("2<1 = %d, want 0", v.Const)
	}
	if v := evalOK(t, bin(LAnd, i(1), i(2))); v.Const != 1 {
		t.Errorf("1&&2 = %d, want 1", v.Const)
	}
}

// The shapes a relocation exists for, and the one it does not.
func TestValueKinds(t *testing.T) {
	for _, c := range []struct {
		name string
		e    Expr
		want Kind
	}{
		{"12", i(12), Absolute},
		{"msg", s("msg"), Relocatable},
		{"msg+4", bin(Add, s("msg"), i(4)), Relocatable},
		{"4+msg", bin(Add, i(4), s("msg")), Relocatable},
		{"msg-.", bin(Sub, s("msg"), here()), PCRelative},
		{"msg-.+4", bin(Add, bin(Sub, s("msg"), here()), i(4)), PCRelative},
		{"end-start", bin(Sub, s("end"), s("start")), Difference},
	} {
		v := evalOK(t, c.e)
		if got := v.Kind(); got != c.want {
			t.Errorf("%s: kind %s, want %s", c.name, got, c.want)
		}
	}

	// A symbol cancels itself and needs no relocation at all.
	if v := evalOK(t, bin(Sub, s("msg"), s("msg"))); v.Kind() != Absolute || v.Const != 0 {
		t.Errorf("msg-msg = %s, want absolute 0", v)
	}
	if v := evalOK(t, bin(Sub, here(), here())); v.Kind() != Absolute {
		t.Errorf(". - . = %s, want absolute", v)
	}
}

// (end-start)/8 works because the subtraction cancels before the division
// sees it. This is the whole reason normalization happens at every + and -.
func TestDifferenceFoldsBeforeDivision(t *testing.T) {
	e := bin(Div, bin(Sub, s("end"), s("start")), i(8))
	if _, err := Eval(e, nil); err == nil {
		t.Fatal("a difference of two undefined symbols divided: want an error")
	}

	// With both resolved to constants it is arithmetic.
	lookup := func(n string) (int64, bool) {
		switch n {
		case "end":
			return 64, true
		case "start":
			return 0, true
		}
		return 0, false
	}
	v, err := Eval(e, lookup)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if !v.IsAbs() || v.Const != 8 {
		t.Errorf("(end-start)/8 = %s, want 8", v)
	}
}

// No relocation multiplies a symbol, adds two of them, or crosses sections.
func TestRejectedShapes(t *testing.T) {
	for _, c := range []struct {
		name string
		e    Expr
		want string
	}{
		{"msg+msg", bin(Add, s("msg"), s("msg")), "appears 2 times"},
		{"msg+other", bin(Add, s("msg"), s("other")), "neither absolute nor relocatable"},
		{"msg*2", bin(Mul, s("msg"), i(2)), "absolute arguments"},
		{"2/msg", bin(Div, i(2), s("msg")), "absolute arguments"},
		{"msg<<1", bin(Shl, s("msg"), i(1)), "absolute arguments"},
		{"-msg", &Unary{P: nowhere, Op: Neg, X: s("msg")}, "absolute value"},
		{"1/0", bin(Div, i(1), i(0)), "division by zero"},
		{"1<<64", bin(Shl, i(1), i(64)), "shift count"},
	} {
		_, err := Eval(c.e, nil)
		if err == nil {
			t.Errorf("%s: want an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q, want it to mention %q", c.name, err, c.want)
		}
	}
}

// A modifier rides on the symbol, survives evaluation, and is never a number
// here — the psABI constant is the arch root's.
func TestModifierSurvivesEvaluation(t *testing.T) {
	e := bin(Add, &SymExpr{P: nowhere, Name: "puts", Mod: ModPLT}, i(0))
	v := evalOK(t, e)
	name, mod, ok := v.Sym()
	if !ok || name != "puts" || mod != ModPLT {
		t.Fatalf("Sym() = %q, %v, %v", name, mod, ok)
	}

	// A modified symbol is not folded even when it resolves as a constant:
	// the modifier is a request for a relocation, and a constant has none.
	lookup := func(string) (int64, bool) { return 7, true }
	v, err := Eval(&SymExpr{P: nowhere, Name: "puts", Mod: ModPLT}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind() != Relocatable {
		t.Errorf("a modified symbol folded to %s", v.Kind())
	}
}

func TestParseModifier(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Modifier
	}{
		{"PLT", ModPLT}, {"plt", ModPLT},
		{"GOT", ModGOT}, {"GOTOFF", ModGOTOFF}, {"gotpc", ModGOTPC},
		{"TPOFF", ModTPOFF},
	} {
		got, ok := ParseModifier(c.in)
		if !ok || got != c.want {
			t.Errorf("ParseModifier(%q) = %v, %v", c.in, got, ok)
		}
	}
	// GOTPCREL is x86-64's. i386 has no PC-relative addressing mode, which is
	// why GOTOFF and GOTPC exist instead.
	for _, in := range []string{"GOTPCREL", "PAGE", "", "PLTOFF"} {
		if m, ok := ParseModifier(in); ok {
			t.Errorf("ParseModifier(%q) = %v, want failure", in, m)
		}
	}
	if !ModTPOFF.TLS() || ModPLT.TLS() {
		t.Error("TLS() classifies the wrong things")
	}
}

// Sections and symbols are derived by walking, so they cannot drift from the
// statements they are derived from.
func TestUnitViews(t *testing.T) {
	u := &Unit{File: "t.s"}
	u.Add(&SectionDecl{Kind: SectionText, Name: ".text", Short: true})
	u.Add(&SymbolDecl{Names: []string{"_start"}, Attrs: AttrGlobal, Type: TypeFunc})
	u.Add(&Label{Name: "_start"})
	u.Add(&Inst{Mnemonic: "ret"})
	u.Add(&SectionDecl{Kind: SectionROData, Name: ".rodata"})
	u.Add(&Label{Name: "msg", Attached: true})
	u.Add(&Data{Width: Width8, Items: []DataItem{{Str: "hi", IsStr: true}}})
	u.Add(&SectionDecl{Kind: SectionText, Name: ".text", Short: true})
	u.Add(&SymbolDecl{Names: []string{"puts"}, Attrs: AttrExtern})

	secs := u.Sections()
	if len(secs) != 2 || secs[0].Name != ".text" || secs[1].Name != ".rodata" {
		t.Fatalf("Sections() = %v", secs)
	}

	syms := u.Symbols()
	want := map[string]struct {
		defined bool
		attrs   Attr
	}{
		"_start": {true, AttrGlobal},
		"msg":    {true, 0},
		"puts":   {false, AttrExtern},
	}
	if len(syms) != len(want) {
		t.Fatalf("Symbols() = %v", syms)
	}
	for _, s := range syms {
		w, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected symbol %q", s.Name)
			continue
		}
		if s.Defined != w.defined || s.Attrs != w.attrs {
			t.Errorf("%s: defined %v attrs %b, want %v %b",
				s.Name, s.Defined, s.Attrs, w.defined, w.attrs)
		}
	}
}

// An .equ whose right-hand side is absolute is a constant; one that names an
// address is not, and resolving it needs section offsets this package has no
// access to.
func TestEquates(t *testing.T) {
	u := &Unit{File: "t.s"}
	u.Add(&Equ{Name: "SIZE", Value: i(64)})
	u.Add(&Equ{Name: "HALF", Value: bin(Div, s("SIZE"), i(2))})
	u.Add(&Equ{Name: "ALIAS", Value: s("msg")})

	eq := u.Equates()
	if eq["SIZE"] != 64 || eq["HALF"] != 32 {
		t.Errorf("Equates() = %v", eq)
	}
	if _, ok := eq["ALIAS"]; ok {
		t.Error("a symbol alias was reported as a constant")
	}
}

// Only the standard names classify. .text.hot is a section whose name begins
// with .text, not a text section with a suffix.
func TestStandardSection(t *testing.T) {
	for _, c := range []struct {
		name string
		kind SectionKind
		std  bool
	}{
		{".text", SectionText, true},
		{".rodata", SectionROData, true},
		{".bss", SectionBSS, true},
		{".tdata", SectionTLS, true},
		{".text.hot", SectionCustom, false},
		{"__TEXT,__text", SectionCustom, false},
		{".mysection", SectionCustom, false},
	} {
		k, std := StandardSection(c.name)
		if k != c.kind || std != c.std {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, k, std, c.kind, c.std)
		}
	}
	if !SectionText.Code() || SectionData.Code() {
		t.Error("Code() classifies the wrong kinds")
	}
}

// Two spellings of one boundary, one field.
func TestAlignBoundary(t *testing.T) {
	for _, c := range []struct {
		exp  int64
		want int64
	}{{0, 1}, {4, 16}, {12, 4096}} {
		got, err := AlignBoundary(nowhere, c.exp)
		if err != nil || got != c.want {
			t.Errorf("AlignBoundary(%d) = %d, %v", c.exp, got, err)
		}
	}
	if _, err := AlignBoundary(nowhere, 32); err == nil {
		t.Error("exponent 32 accepted")
	}
	if err := CheckAlign(nowhere, 12); err == nil {
		t.Error("alignment 12 accepted")
	}
	if err := CheckAlign(nowhere, 16); err != nil {
		t.Errorf("alignment 16 rejected: %v", err)
	}
}

// A register is the one operand fully determined by its spelling, which is
// why it is the only thing resolved at parse time.
func TestRegisterResolution(t *testing.T) {
	r, ok := LookupRegister("eax")
	if !ok || r != reg.Reg(reg.EAX) {
		t.Errorf("LookupRegister(eax) = %v, %v", r, ok)
	}
	// Sigils are a dialect's business and are stripped before the call.
	if _, ok := LookupRegister("%eax"); ok {
		t.Error("a sigil reached reg.Lookup")
	}
}

// Diagnostics are file:line:col: level: message, the format every editor
// already parses, with notes below.
func TestDiagnosticFormat(t *testing.T) {
	e := Errorf(Pos{File: "kernel.s", Line: 14, Col: 5},
		"vsetvli requires v, not in the active feature set").
		Note("add --features +v")
	want := "kernel.s:14:5: error: vsetvli requires v, not in the active feature set\n  note: add --features +v"
	if got := e.Error(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	var l ErrorList
	if l.Err() != nil {
		t.Error("an empty list is not a nil error")
	}
	l.Add(Errorf(Pos{File: "a.s", Line: 9}, "second"))
	l.Add(Errorf(Pos{File: "a.s", Line: 2}, "first"))
	l.Sort()
	if l[0].Msg != "first" {
		t.Errorf("Sort() left %q first", l[0].Msg)
	}
	if l.Err() == nil {
		t.Error("a non-empty list is a nil error")
	}
}