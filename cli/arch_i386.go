// cli/arch_i386.go
//
// The i386 adapter. This file and arch_x86_64.go are the only two in the
// package that import an arch package, and neither imports the other's.
package cli

import (
	"fmt"

	"github.com/vertex-language/arc/i386"
)

type i386Ops struct{}

func (i386Ops) build(path string, src []byte, platform string, d dialect) ([]byte, error) {
	p, err := i386Platform(platform)
	if err != nil {
		return nil, err
	}
	dl, err := i386Dialect(d)
	if err != nil {
		return nil, err
	}

	u, err := i386.ParseFile(path, src, dl)
	if err != nil {
		return nil, err
	}
	return i386.Assemble(u, p, i386.DefaultFeatures())
}

// format is parse and print.
//
// i386 has no Translate: text.go exports ParseFile, ParseInst, Print and
// PrintInst and nothing that attaches a form. Within one dialect that is
// exactly right. Across two it is the case the root README's round-trip
// guarantee does not yet cover here — an operand whose size neither the
// mnemonic nor the operands state prints without one, because nothing in
// this path resolved the form that knows it. Wiring this to a Translate
// when i386 grows one is a two-line change.
func (i386Ops) format(path string, src []byte, from, to dialect) ([]byte, error) {
	in, err := i386Dialect(from)
	if err != nil {
		return nil, err
	}
	out, err := i386Dialect(to)
	if err != nil {
		return nil, err
	}

	u, err := i386.ParseFile(path, src, in)
	if err != nil {
		return nil, err
	}
	return i386.Print(u, out)
}

func (i386Ops) encode(line string, d dialect) ([]byte, error) {
	dl, err := i386Dialect(d)
	if err != nil {
		return nil, err
	}

	inst, err := i386.ParseInst(line, dl)
	if err != nil {
		return nil, err
	}
	b, _, err := i386.Encode(i386.DefaultFeatures(), inst)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// decode prints the decoded struct rather than rendering it.
//
// Rendering needs a decode.Inst → text.Inst step so PrintInst has something
// to take, and that step is the arch package's: this package cannot write
// it without reaching into decode/, which it may not import.
func (i386Ops) decode(b []byte) (string, error) {
	inst, err := i386.Decode(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%+v", inst), nil
}

func (i386Ops) explain(b []byte) (string, error) {
	fields, err := i386.Explain(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%+v", fields), nil
}

func i386Platform(platform string) (i386.Platform, error) {
	switch platform {
	case "elf":
		return i386.ELF, nil
	case "coff":
		return i386.COFF, nil
	case "flat":
		return i386.Flat, nil
	}
	return 0, unsupportedPlatform(archI386, platform)
}

func i386Dialect(d dialect) (i386.Dialect, error) {
	switch d {
	case dialectGAS:
		return i386.GAS, nil
	case dialectNASM:
		return i386.NASM, nil
	}
	return 0, fmt.Errorf("i386: no dialect named; a unit has to be printed in one")
}