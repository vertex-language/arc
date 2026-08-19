// cli/arch_x86_64.go
//
// The x86_64 adapter.
package cli

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64"
)

type x86_64Ops struct{}

// build parses and then refuses.
//
// x86_64/assemble.go is unlanded, so there is no text.Unit → object bytes
// call to make: the typed builder API and Emit are the only paths to bytes
// in that package today, and neither takes a parsed file. Parsing anyway is
// deliberate — a syntax error in the source is worth reporting even when
// nothing downstream can emit, and it means this path starts working the
// moment Assemble lands rather than needing to be rewritten.
func (x86_64Ops) build(path string, src []byte, platform string, d dialect) ([]byte, error) {
	if _, err := x86Platform(platform); err != nil {
		return nil, err
	}
	dl, err := x86Dialect(d)
	if err != nil {
		return nil, err
	}
	if _, err := x86_64.ParseFile(path, src, dl); err != nil {
		return nil, err
	}

	// When assemble.go lands, the body is:
	//
	//	p, err := x86Platform(platform)
	//	u, err := x86_64.ParseFile(path, src, dl)
	//	return x86_64.Assemble(u, p, x86_64.DefaultFeatures())
	return nil, unlanded(archX86_64, "build",
		"the source parses, but x86_64/assemble.go is unlanded and nothing else "+
			"in that package takes a text.Unit to an object")
}

// format reprints within a dialect and translates across two.
//
// The split is not an optimization. Format is parse and print, which is
// exact when the target syntax spells sizes the same way the source did.
// Translate puts a resolution step in the middle, and it is required going
// the other way: `mov qword [rbx], 1` becomes `movq $1, (%rbx)` only if
// something knows the width, and the only thing that knows it is the form
// the encoder resolved.
func (x86_64Ops) format(path string, src []byte, from, to dialect) ([]byte, error) {
	in, err := x86Dialect(from)
	if err != nil {
		return nil, err
	}
	out, err := x86Dialect(to)
	if err != nil {
		return nil, err
	}

	if from == to {
		return x86_64.Format(path, src, in, out)
	}
	return x86_64.Translate(path, src, in, out, x86_64.DefaultFeatures())
}

// encode parses and then refuses.
//
// x86_64/code.go's Encode takes a mnemonic and operand values — the shape a
// caller building operands in Go has — and a parsed *TextInst is neither.
// Lowering one is text.Inst.Lower plus encode.Args, and encode/ is not
// importable from here. The missing call is one function next to Encode:
// EncodeInst(FeatureSet, *TextInst) ([]byte, []Fixup, error), which is also
// what ResolveUnit already does per instruction internally.
func (x86_64Ops) encode(line string, d dialect) ([]byte, error) {
	dl, err := x86Dialect(d)
	if err != nil {
		return nil, err
	}
	if _, err := x86_64.ParseInst(line, dl); err != nil {
		return nil, err
	}
	return nil, unlanded(archX86_64, "enc",
		"the instruction parses, but x86_64.Encode takes a mnemonic and operand "+
			"values rather than a parsed instruction, and lowering one needs encode/")
}

func (x86_64Ops) decode(b []byte) (string, error) {
	inst, err := x86_64.Decode(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%+v", inst), nil
}

func (x86_64Ops) explain(b []byte) (string, error) {
	ex, err := x86_64.Explain(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%+v", ex), nil
}

// x86Platform goes through the package's own ParsePlatform rather than a
// switch here, so the CLI cannot accept a spelling the encoder does not.
func x86Platform(platform string) (x86_64.Platform, error) {
	p, err := x86_64.ParsePlatform(platform)
	if err != nil {
		return 0, unsupportedPlatform(archX86_64, platform)
	}
	return p, nil
}

func x86Dialect(d dialect) (x86_64.Dialect, error) {
	switch d {
	case dialectGAS:
		return x86_64.GAS, nil
	case dialectNASM:
		return x86_64.NASM, nil
	}
	return 0, fmt.Errorf("x86_64: no dialect named; a unit has to be printed in one")
}