package i386

import (
	"fmt"

	"github.com/vertex-language/arc/i386/text"
	"github.com/vertex-language/arc/i386/text/gas"
	"github.com/vertex-language/arc/i386/text/nasm"
)

// The GAS/NASM dispatch. Two cases, because a dialect is a directory —
// text/gas and text/nasm — and this is the one place that has to know both
// exist. Everything above this file works in text.Unit and never asks which
// dialect a Unit came from; everything below it (gas.ParseFile, nasm.Print,
// ...) has no notion that the other one exists either.

func dialectErr(d Dialect) error {
	return fmt.Errorf("i386: unknown dialect %s", d)
}

// ParseFile parses one .s file in dialect d. arc build is ParseFile then
// Assemble.
func ParseFile(name string, src []byte, d Dialect) (*text.Unit, error) {
	switch d {
	case GAS:
		return gas.ParseFile(name, src)
	case NASM:
		return nasm.ParseFile(name, src)
	}
	return nil, dialectErr(d)
}

// ParseInst parses a single instruction in dialect d, for arc enc and arc
// explain's command-line argument.
func ParseInst(s string, d Dialect) (*text.Inst, error) {
	switch d {
	case GAS:
		return gas.ParseInst(s)
	case NASM:
		return nasm.ParseInst(s)
	}
	return nil, dialectErr(d)
}

// Print renders a unit in dialect d. arc fmt is ParseFile in one dialect
// then Print in whichever --dialect names; that the two paths meet in
// text.Unit is what makes the round trip a property of the code and not a
// claim in a README.
func Print(u *text.Unit, d Dialect) ([]byte, error) {
	switch d {
	case GAS:
		return gas.Print(u)
	case NASM:
		return nasm.Print(u)
	}
	return nil, dialectErr(d)
}

// PrintInst renders one instruction in dialect d, for arc dis.
func PrintInst(in *text.Inst, d Dialect) (string, error) {
	switch d {
	case GAS:
		return gas.PrintInst(in), nil
	case NASM:
		return nasm.PrintInst(in), nil
	}
	return "", dialectErr(d)
}