// cli/arch_x86_64.go
//
// The x86_64 adapter.
package cli

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64"
)

type x86_64Ops struct{}

func (x86_64Ops) build(path string, src []byte, platform string, d dialect) ([]byte, error) {
	p, err := x86Platform(platform)
	if err != nil {
		return nil, err
	}
	dl, err := x86Dialect(d)
	if err != nil {
		return nil, err
	}

	u, err := x86_64.ParseFile(path, src, dl)
	if err != nil {
		return nil, err
	}
	return x86_64.Assemble(u, p, x86_64.DefaultFeatures())
}

// format reprints within a dialect and translates across two.
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

func (x86_64Ops) encode(line string, d dialect) ([]byte, error) {
	dl, err := x86Dialect(d)
	if err != nil {
		return nil, err
	}

	inst, err := x86_64.ParseInst(line, dl)
	if err != nil {
		return nil, err
	}

	b, _, err := x86_64.EncodeInst(x86_64.DefaultFeatures(), inst)
	if err != nil {
		return nil, err
	}
	return b, nil
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