package aarch64

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/aarch64/feature"
	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/text"
)

// Assemble is arc build's whole job in one call.
//
// It walks the unit in source order, placing every statement into a fresh
// Assembler's sections exactly as the hand-built calls would, and returns
// Serialize's bytes — no separate translation step for a caller to get wrong.
//
// Source order is the only order that means anything: a section directive
// changes where subsequent statements land, so a pass that reordered
// statements would change the object.
func Assemble(u *Unit, p Platform, set FeatureSet) ([]byte, error) {
	a, err := AssembleTo(u, p, set)
	if err != nil {
		return nil, err
	}
	return a.Serialize()
}

// AssembleTo is Assemble stopping before Serialize, for a caller that wants to
// add to the object afterward or inspect it.
func AssembleTo(u *Unit, p Platform, set FeatureSet) (*Assembler, error) {
	a := New(p, WithFeatures(set))
	w := &walker{a: a, active: set, base: set}

	// A file that names no section starts in .text, which is what gas does and
	// what code written against it depends on.
	w.sec = a.Section(Text)

	for _, n := range u.Nodes {
		if err := w.node(n); err != nil {
			return nil, err
		}
		if a.Err() != nil {
			return nil, a.Err()
		}
	}
	return a, nil
}

// walker carries the state a walk needs that the tree does not: which section
// is current, and what the feature set has been changed to.
type walker struct {
	a   *Assembler
	sec *Section

	// active is the feature set instructions resolve against. It starts at the
	// caller's and moves with .arch, .arch_extension and .cpu.
	//
	// This is the one place the source overrides what the API fixed at New,
	// and it is the source's exception rather than the API's: each of those is
	// a statement at a line number, so a gating diagnostic still names a flag
	// and still names the line that set it. base is what .arch resets from.
	active feature.Set
	base   feature.Set

	// numeric tracks gas's numeric labels. They are position references rather
	// than symbols — the same digit is legitimately defined many times — so
	// they never reach the symbol table and are resolved here.
	numeric map[string][]numericLabel
}

type numericLabel struct {
	sec    *Section
	offset int
}

func (w *walker) node(n text.Node) error {
	switch x := n.(type) {
	case *text.Comment:
		return nil
	case *text.Label:
		return w.label(x)
	case *text.Inst:
		return w.inst(x)
	case *text.Directive:
		return w.directive(x)
	}
	return posErr(n.Pos(), "unknown statement")
}

func (w *walker) label(l *text.Label) error {
	if l.Numeric {
		if w.numeric == nil {
			w.numeric = map[string][]numericLabel{}
		}
		w.numeric[l.Name] = append(w.numeric[l.Name],
			numericLabel{sec: w.sec, offset: w.sec.Len()})
		return nil
	}
	w.sec.Label(l.Name)
	return nil
}

func (w *walker) inst(in *text.Inst) error {
	// .inst states a word rather than naming an instruction: the one case
	// where emitting bytes nobody selected is what was asked for.
	if in.Mnem == ".inst" {
		return w.rawWord(in)
	}

	if err := in.Resolve(w.active, nil); err != nil {
		return posErr(in.P, "%v", err)
	}
	ops, err := in.Lower(nil)
	if err != nil {
		return posErr(in.P, "%v", err)
	}
	w.sec.EmitForm(in.Form, ops...)
	return nil
}

func (w *walker) rawWord(in *text.Inst) error {
	if len(in.Ops) != 1 {
		return posErr(in.P, ".inst takes one word")
	}
	v, err := text.Eval(in.Ops[0].Expr, nil)
	if err != nil {
		return posErr(in.P, ".inst needs a constant: %v", err)
	}
	w.sec.Word(uint32(v))
	return nil
}

func (w *walker) directive(d *text.Directive) error {
	// The refusals come first, each naming what it would have taken. An
	// unsupported directive silently ignored produces an object missing
	// something the source asked for, which is the failure hardest to notice.
	if d.Kind.Refused() {
		return w.refuse(d)
	}

	switch d.Kind {
	case text.DirSection:
		w.sec = w.a.Section(d.Name)
		w.applySectionFlags(d)
		return nil

	case text.DirAlign:
		n, err := d.Alignment(nil)
		if err != nil {
			return posErr(d.P, "%v", err)
		}
		if n > 0 {
			w.sec.Align(int(n))
		}
		return nil

	case text.DirBinding:
		return w.binding(d)

	case text.DirType:
		return w.symType(d)

	case text.DirSize:
		n, err := d.Const(nil)
		if err != nil {
			return posErr(d.P, ".size needs a constant: %v", err)
		}
		w.a.SetSize(d.Name, int(n))
		return nil

	case text.DirVariantPCS:
		w.a.SetVariantPCS(d.Name)
		return nil

	case text.DirData:
		return w.data(d)

	case text.DirSpace:
		return w.space(d)

	case text.DirArch, text.DirArchExt, text.DirCPU:
		return w.archState(d)

	case text.DirReq, text.DirUnreq:
		// The alias table is the parser's: a register name resolves at parse
		// time or not at all, so by the time a unit exists these have already
		// done their work and there is nothing to place.
		return nil

	case text.DirCFI, text.DirOpaque:
		// Debug bytes attach as opaque payloads and pass through untouched.
		// This tree is not a compiler driver and does not interpret them.
		return nil
	}
	return posErr(d.P, "unhandled directive %s", d.Spelling)
}

// refuse produces the specific error a refused directive gets, naming the gap
// rather than saying "unsupported".
func (w *walker) refuse(d *text.Directive) error {
	switch d.Kind {
	case text.DirEqu:
		return posErr(d.P, "%s needs a value threaded across statements", d.Spelling).
			note("Assemble runs with no Env; define the constant in Go and pass it as an operand")

	case text.DirComm:
		return posErr(d.P, "%s needs a value threaded across statements", d.Spelling).
			note("Assemble runs with no Env; reserve the space with a bss section and Zero")

	case text.DirOrg:
		return posErr(d.P, ".org needs an image-layout step").
			note("this tree has no linker-free version of one yet; " +
				"for a flat image, place the section and use SetBaseAddress")

	case text.DirPool:
		return posErr(d.P, "%s implies a literal pool", d.Spelling).
			note("placing a constant into a pool means the assembler choosing where data " +
				"lives, which is a layout engine; write the constant into a section and " +
				"load from it")
	}
	return posErr(d.P, "%s is refused", d.Spelling)
}

func (w *walker) applySectionFlags(d *text.Directive) {
	for _, f := range d.Flags {
		// An "x" in the flag string is what says a section holds instructions,
		// for a name the conventions do not cover.
		if strings.Contains(f, "x") && !strings.HasPrefix(f, "@") {
			w.sec.SetCode(true)
		}
		if strings.Contains(f, "nobits") || f == "@nobits" {
			w.sec.Kind = KindBSS
		}
	}
}

func (w *walker) binding(d *text.Directive) error {
	b := Global
	if len(d.Flags) > 0 {
		switch d.Flags[0] {
		case "weak":
			b = Weak
		case "local":
			b = Local
		case "hidden", "protected", "internal":
			b = Hidden
		}
	}
	// A binding may precede or follow the label it applies to, so this is a
	// declaration that a later definition upgrades rather than an error when
	// the name is not yet known.
	w.a.Declare(d.Name, b)
	return nil
}

func (w *walker) symType(d *text.Directive) error {
	if len(d.Flags) == 0 {
		return posErr(d.P, ".type needs a type")
	}
	t := None
	switch strings.ToLower(d.Flags[0]) {
	case "function", "func", "sti_func":
		t = Func
	case "object":
		t = Object
	case "tls_object":
		t = TLS
	case "notype":
		t = None
	default:
		return posErr(d.P, "unknown symbol type %q", d.Flags[0])
	}
	return w.a.setType(d.Name, t)
}

func (w *walker) data(d *text.Directive) error {
	width, _ := d.Values()

	if width == text.DataString {
		for _, s := range d.Strings {
			if d.Terminated() {
				w.sec.Asciz(s)
				continue
			}
			w.sec.Ascii(s)
		}
		return nil
	}

	for i, e := range d.Args {
		v, err := text.Reduce(e, nil)
		if err != nil {
			return posErr(d.P, "argument %d: %v", i+1, err)
		}

		switch {
		case v.Constant():
			w.emitConst(width, v.Const)

		case v.Simple():
			// A constant plus one symbol is what a data fixup consumes
			// directly, and the builder API's WordRef and QuadRef are exactly
			// this call.
			if width != text.DataWord && width != text.DataDouble {
				return posErr(d.P, "argument %d: a symbolic value needs a 4- or 8-byte datum",
					i+1)
			}
			t := Target(operand.Label(v.Plus))
			if v.Const != 0 {
				t = Ref(v.Plus).Plus(v.Const)
			}
			if width == text.DataDouble {
				w.sec.QuadRef(t)
			} else {
				w.sec.WordRef(t)
			}

		default:
			// This is the gap the README names, and it is in the text path
			// only. `.xword . - msg` is legal assembly whose residue Reduce
			// already computes; what is missing is the backpatch that would
			// consume it, and on Mach-O the paired SUBTRACTOR record it would
			// need. Refused with the residue named rather than miswritten.
			return posErr(d.P,
				"argument %d reduces to %s, which needs a fixup this path does not wire",
				i+1, v).
				note("the builder API's WordRef and QuadRef cover a single added symbol; " +
					"a difference of two needs a paired relocation record")
		}
	}
	return nil
}

func (w *walker) emitConst(width text.DataWidth, v int64) {
	switch width {
	case text.DataByte:
		w.sec.Byte(uint8(v))
	case text.DataHalf:
		w.sec.Half(uint16(v))
	case text.DataWord:
		w.sec.Word32(uint32(v))
	case text.DataDouble:
		w.sec.Quad(uint64(v))
	}
}

func (w *walker) space(d *text.Directive) error {
	n, err := d.Const(nil)
	if err != nil {
		return posErr(d.P, "%s needs a constant: %v", d.Spelling, err)
	}
	if strings.EqualFold(d.Spelling, ".fill") {
		size, value := int64(1), int64(0)
		if len(d.Args) > 1 {
			if size, err = text.Eval(d.Args[1], nil); err != nil {
				return posErr(d.P, ".fill size: %v", err)
			}
		}
		if len(d.Args) > 2 {
			if value, err = text.Eval(d.Args[2], nil); err != nil {
				return posErr(d.P, ".fill value: %v", err)
			}
		}
		w.sec.Fill(int(n), int(size), uint64(value))
		return nil
	}
	w.sec.Zero(int(n))
	return nil
}

// archState applies .arch, .arch_extension and .cpu.
//
// .arch clears extensions selected before it, which is what GNU as does and
// what code written against as depends on. .arch_extension is incremental in
// both directions and applies to whatever is active.
func (w *walker) archState(d *text.Directive) error {
	switch d.Kind {
	case text.DirArch, text.DirCPU:
		set, err := feature.ParseFeatures(d.Name)
		if err != nil {
			return posErr(d.P, "%v", err)
		}
		w.active = set
		w.base = set
		return nil

	case text.DirArchExt:
		spec := d.Name
		if !strings.HasPrefix(spec, "+") {
			spec = "+" + spec
		}
		// ParseFeatures applies modifiers left to right from a base, so
		// re-parsing the active set's own spelling with the addition appended
		// is how an incremental change stays closed under requirements in
		// both directions.
		set, err := feature.ParseFeatures(w.active.String() + spec)
		if err != nil {
			return posErr(d.P, "%v", err)
		}
		w.active = set
		return nil
	}
	return nil
}

// posErr builds a positioned error.
type assembleError struct {
	pos  text.Pos
	msg  string
	hint string
}

func (e *assembleError) Error() string {
	s := e.pos.String() + ": " + e.msg
	if e.hint != "" {
		s += "\n  note: " + e.hint
	}
	return s
}

func (e *assembleError) note(h string) *assembleError { e.hint = h; return e }

func posErr(p text.Pos, format string, args ...any) *assembleError {
	return &assembleError{pos: p, msg: fmt.Sprintf(format, args...)}
}