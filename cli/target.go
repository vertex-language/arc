// cli/target.go
package cli

import (
	"fmt"

	"github.com/vertex-language/arc/i386"
)

// target is the resolved -t/--target. Arch is implied — i386 is the
// only package wired up, so there's nothing to switch on yet.
type target struct {
	platform i386.Platform
	name     string
}

var platforms = map[string]i386.Platform{
	"elf":  i386.ELF,
	"coff": i386.COFF,
	"flat": i386.Flat,
}

// parseTarget accepts "i386-<platform>" or bare "<platform>".
// Defaults to i386-elf.
func parseTarget(s string) (target, error) {
	if s == "" {
		return target{platform: i386.ELF, name: "i386-elf"}, nil
	}

	arch, plat := splitTarget(s)
	if arch != "" && arch != "i386" {
		return target{}, fmt.Errorf("no such arch %q (only i386 is wired up)", arch)
	}

	p, ok := platforms[plat]
	if !ok {
		return target{}, fmt.Errorf(
			"no such target: i386-%s\n  note: i386 supports: elf, coff, flat", plat)
	}
	return target{platform: p, name: "i386-" + plat}, nil
}

func splitTarget(s string) (arch, platform string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

func parseDialect(s string) (i386.Dialect, error) {
	switch s {
	case "", "gas", "att", "at&t":
		return i386.GAS, nil
	case "nasm", "intel":
		return i386.NASM, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q (i386 has gas, nasm)", s)
	}
}