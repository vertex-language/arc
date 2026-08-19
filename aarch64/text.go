package aarch64

import (
	"fmt"

	"github.com/vertex-language/arc/aarch64/text"
	"github.com/vertex-language/arc/aarch64/text/gas"
)

// The text layer's entry points.
//
// This file is the sole importer of text/gas, which is what keeps the syntax
// package reachable from exactly one place. There is one syntax, so the
// dispatch is trivial today; it is still a dispatch rather than a re-export so
// that the shape matches x86_64's, where the same file chooses between two.

// Unit is one parsed source file.
type Unit = text.Unit

// Node is one statement.
type Node = text.Node

// Env is what an expression needs that the tree does not carry: the values of
// symbols an earlier .equ defined, and where the location counter is.
type Env = text.Env

// ParseFile reads A64 assembly.
//
// There is no dialect parameter. NASM has no A64 grammar to accept and
// inventing one would be inventing syntax, so a --dialect flag on this target
// is a usage error naming that rather than a silently ignored option.
func ParseFile(file string, src []byte) (*Unit, error) {
	return gas.Parse(file, string(src))
}

// ParseInst reads one instruction, for a caller assembling a line at a time.
func ParseInst(src string) (*text.Inst, error) {
	u, err := gas.Parse("<inst>", src)
	if err != nil {
		return nil, err
	}
	for _, n := range u.Nodes {
		if in, ok := n.(*text.Inst); ok {
			return in, nil
		}
	}
	return nil, fmt.Errorf("aarch64: %q is not an instruction", src)
}

// Print writes a unit back out for a platform.
//
// The platform is a parameter and a dialect is not, because what varies is the
// modifier spelling and it varies by platform: GNU as writes :lo12: and the
// Darwin assembler writes @PAGEOFF. Both name the same four roles and produce
// identical bytes, so this is a spelling and never a byte — which is also why
// it cannot be a preference. Asking for @PAGEOFF in an ELF object would produce
// a file no assembler on that platform will read back.
func Print(u *Unit, p Platform) ([]byte, error) {
	opts := gas.DefaultOptions
	opts.Platform = printPlatform(p)
	s, err := gas.Print(u, opts)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// PrintInst writes one instruction, for a diagnostic that wants to quote what
// it is about to refuse.
func PrintInst(in *text.Inst, p Platform) (string, error) {
	u := &Unit{Nodes: text.Nodes{in}}
	b, err := Print(u, p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func printPlatform(p Platform) gas.Platform {
	switch p {
	case MachO:
		return gas.MachO
	case COFF:
		return gas.COFF
	}
	return gas.ELF
}

// Format is parse and print in one call: what arc fmt does.
//
// It does not resolve. On x86_64 the resolution is not optional — a width
// neither dialect states outright has to come from the form the encoder picked
// — but there is one syntax here and printing never needs a fact the source
// does not already carry, so formatting is a pure text-level operation and
// says so.
func Format(file string, src []byte, p Platform) ([]byte, error) {
	u, err := ParseFile(file, src)
	if err != nil {
		return nil, err
	}
	return Print(u, p)
}

// ResolveUnit fills every instruction's Form against a feature set.
//
// It is separate from Assemble for the caller that wants the resolved tree
// rather than bytes: arc fmt --check does, and so does anything asking what an
// instruction encodes to without wanting an object around it.
//
// The Env is threaded because a slot's acceptance can depend on an immediate's
// value and an immediate can be an expression. Passing nil is legal and means
// no symbol has a value — which is the right environment when the question is
// whether the source resolves on its own terms.
func ResolveUnit(u *Unit, set FeatureSet, env Env) error {
	for _, in := range u.Nodes.Insts() {
		if err := in.Resolve(set, env); err != nil {
			return &Error{Section: in.Pos().String(), Err: err}
		}
	}
	return nil
}