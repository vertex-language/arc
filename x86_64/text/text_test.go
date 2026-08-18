// x86_64/text/text_test.go
package text

import (
	"errors"
	"testing"

	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/reg"
)

// env is a test Env with a few absolute symbols and a location.
type env struct {
	syms map[string]int64
	dot  int64
	sect int64
	has  bool
}

func (e *env) Lookup(n string) (int64, bool) { v, ok := e.syms[n]; return v, ok }
func (e *env) Dot() (int64, bool)            { return e.dot, e.has }
func (e *env) SectionStart() (int64, bool)   { return e.sect, e.has }

func num(v int64) Expr        { return &Num{Value: v} }
func sym(n string) Expr       { return &Sym{Name: n} }
func bin(o Op, x, y Expr) Expr { return &Binary{Op: o, X: x, Y: y} }

// The truth value both assemblers agree on is all-ones, not one. A caller
// masking with a comparison depends on it.
func TestComparisonIsAllOnes(t *testing.T) {
	v, err := Eval(bin(OpEq, num(3), num(3)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != -1 {
		t.Errorf("3 == 3 is %d, want -1", v)
	}
}

// gas divides signed and NASM divides unsigned. One operator would make the
// tree unable to print back into the dialect it came from.
func TestSignedAndUnsignedDivisionAreDifferentOperators(t *testing.T) {
	s, err := Eval(bin(OpDiv, num(-8), num(2)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s != -4 {
		t.Errorf("signed -8/2 = %d, want -4", s)
	}
	u, err := Eval(bin(OpUDiv, num(-8), num(2)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if u != int64(uint64(0xfffffffffffffff8)/2) {
		t.Errorf("unsigned -8/2 = %d", u)
	}
}

func TestDivideByZero(t *testing.T) {
	_, err := Eval(bin(OpDiv, num(1), num(0)), nil)
	if !errors.Is(err, ErrDivideByZero) {
		t.Errorf("got %v, want ErrDivideByZero", err)
	}
}

// A nil Env means constants only, and a symbol is then not an error in the
// tree but an answer about the expression.
func TestNilEnvIsConstantsOnly(t *testing.T) {
	if _, err := Eval(sym("msg"), nil); err == nil {
		t.Error("a symbol has no constant value")
	}
	if v, err := Eval(bin(OpAdd, num(2), num(3)), nil); err != nil || v != 5 {
		t.Errorf("2+3 = %d, %v", v, err)
	}
}

// Reduce is the path a fixup needs: a symbol plus a constant is exactly
// what a relocation records.
func TestReduceSymbolPlusConstant(t *testing.T) {
	v, err := Reduce(bin(OpAdd, sym("msg"), num(4)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Add != "msg" || v.Const != 4 || v.Sub != "" {
		t.Errorf("got %+v, want msg+4", v)
	}
	if v.IsConst() {
		t.Error("a symbolic value is not constant")
	}
}

// `. - msg` is one symbol subtracted from another, which is a relocation
// and folds to a constant when both land in the same section.
func TestReduceDotMinusSymbol(t *testing.T) {
	v, err := Reduce(bin(OpSub, &Dot{}, sym("msg")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Dot || v.Sub != "msg" {
		t.Errorf("got %+v, want . - msg", v)
	}
}

// Two symbol addresses added, or a symbol multiplied, have no relocation.
// Refusing here is the difference between a diagnostic and wrong data.
func TestReduceRefusesWhatHasNoRelocation(t *testing.T) {
	for _, e := range []Expr{
		bin(OpAdd, sym("a"), sym("b")),
		bin(OpMul, sym("a"), num(4)),
		bin(OpAnd, sym("a"), num(7)),
	} {
		if _, err := Reduce(e, nil); err == nil {
			t.Errorf("%s must be refused", exprString(e))
		}
	}
}

// A symbol the Env knows is a constant, and reduces to one.
func TestReduceFoldsKnownSymbols(t *testing.T) {
	e := &env{syms: map[string]int64{"COUNT": 8}}
	v, err := Reduce(bin(OpMul, sym("COUNT"), num(4)), e)
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsConst() || v.Const != 32 {
		t.Errorf("got %+v, want the constant 32", v)
	}
}

// gas states the size on the mnemonic and NASM on the operand. Both reach
// the same M64, and Sized is where that happens.
func TestSizeFlowsToUnsizedMemory(t *testing.T) {
	// movq $1, (%rbx) — the size is on the mnemonic.
	i := &Inst{
		Mnemonic: "mov",
		Size:     operand.W64,
		Operands: []*Operand{
			MemOp(Pos{}, MemRef{Base: reg.RBX, HasBase: true}, operand.WidthNone),
			ImmOp(Pos{}, num(1)),
		},
	}
	ops := i.Sized()
	if ops[0].Size != operand.W64 {
		t.Errorf("size did not reach the operand: %v", ops[0].Size)
	}
	// The original is untouched: Sized copies rather than mutates, because
	// a printer may want the source's own spelling afterward.
	if i.Operands[0].Size != operand.WidthNone {
		t.Error("Sized must not mutate the tree")
	}

	lowered, err := i.Lower(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lowered[0].(operand.M64); !ok {
		t.Errorf("lowered to %T, want operand.M64", lowered[0])
	}
}

// A register operand states the size by being what it is.
func TestOperandSizeFromRegister(t *testing.T) {
	i := &Inst{
		Mnemonic: "mov",
		Operands: []*Operand{
			RegOp(Pos{}, reg.EAX),
			MemOp(Pos{}, MemRef{Base: reg.RBX, HasBase: true}, operand.WidthNone),
		},
	}
	if got := i.OperandSize(); got != operand.W32 {
		t.Errorf("OperandSize = %v, want dword", got)
	}
}

// The operand's own rules, checked where a line number exists.
func TestOperandValidate(t *testing.T) {
	bad := MemOp(Pos{Line: 3}, MemRef{
		Base: reg.RAX, HasBase: true,
		Index: reg.RSP, HasIndex: true, Scale: 1,
	}, operand.W64)
	err := bad.Validate()
	if err == nil {
		t.Fatal("rsp cannot be an index")
	}
	var e *Error
	if !errors.As(err, &e) || e.Pos.Line != 3 {
		t.Errorf("diagnostic lost its position: %v", err)
	}

	rip := MemOp(Pos{}, MemRef{RIP: true, Base: reg.RBX, HasBase: true}, operand.W64)
	if rip.Validate() == nil {
		t.Error("rip-relative takes no base")
	}
}

// A branch target and an immediate are the same NASM syntax and different
// relocations. Which one a bare symbol is is a fact about the instruction.
func TestIsBranch(t *testing.T) {
	for _, m := range []string{"call", "jmp", "je"} {
		if !IsBranch(m) {
			t.Errorf("%s takes a branch displacement", m)
		}
	}
	for _, m := range []string{"mov", "add", "lea"} {
		if IsBranch(m) {
			t.Errorf("%s does not", m)
		}
	}
}

// .align is a byte count and .p2align is an exponent, and normalizing here
// means nothing downstream has to remember which it read.
func TestAlignmentNormalizes(t *testing.T) {
	a := &Directive{Kind: Align, Args: []Expr{num(16)}}
	if v, err := a.Alignment(nil); err != nil || v != 16 {
		t.Errorf(".align 16 = %d, %v", v, err)
	}
	p := &Directive{Kind: P2Align, Args: []Expr{num(4)}}
	if v, err := p.Alignment(nil); err != nil || v != 16 {
		t.Errorf(".p2align 4 = %d, %v", v, err)
	}
	bad := &Directive{Kind: Align, Args: []Expr{num(15)}}
	if _, err := bad.Alignment(nil); err == nil {
		t.Error("15 is not a power of two")
	}
}

// A count cannot depend on a symbol: the size of a statement is not
// something a linker can decide.
func TestCountsMustBeConstant(t *testing.T) {
	d := &Directive{Kind: Fill, Args: []Expr{sym("n"), num(1), num(0)}}
	if _, err := d.Const(nil, 0); err == nil {
		t.Error(".fill needs a constant count")
	}
}

// A label defined twice is the unit's error, not any statement's.
func TestValidateCatchesRedefinition(t *testing.T) {
	u := &Unit{Name: "t.s", Nodes: []Node{
		&Label{Position: Pos{Line: 1}, Name: "main"},
		&Label{Position: Pos{Line: 9}, Name: "main"},
	}}
	errs := u.Validate()
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
}

// Numeric labels may be redefined, so they are not symbols and do not
// collide.
func TestNumericLabelsMayRepeat(t *testing.T) {
	u := &Unit{Nodes: []Node{
		&Label{Name: "1", Numeric: true},
		&Label{Name: "1", Numeric: true},
	}}
	if errs := u.Validate(); len(errs) != 0 {
		t.Errorf("got %v", errs)
	}
	if len(u.Defined()) != 0 {
		t.Error("a numeric label is not a defined symbol")
	}
}

func TestSectionsInCreationOrder(t *testing.T) {
	u := &Unit{Nodes: []Node{
		&Directive{Kind: Section, Args: []Expr{sym(".text")}},
		&Directive{Kind: Section, Args: []Expr{sym(".rodata")}},
		&Directive{Kind: Section, Args: []Expr{sym(".text")}},
	}}
	got := u.Sections()
	if len(got) != 2 || got[0] != ".text" || got[1] != ".rodata" {
		t.Errorf("Sections() = %v", got)
	}
}

func TestParseSymbolType(t *testing.T) {
	for _, s := range []string{"@function", "%function", "function", "func"} {
		if v, err := ParseSymbolType(s); err != nil || v != TypeFunc {
			t.Errorf("%q → %v, %v", s, v, err)
		}
	}
	if _, err := ParseSymbolType("@widget"); err == nil {
		t.Error("unknown types must be refused")
	}
}