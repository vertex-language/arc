// cli/target.go
//
// The target and dialect vocabulary the flags speak, and the alias table
// that resolves it. A canonical arch name and a canonical platform name are
// all that cross into an adapter; every spelling the world uses stops here.
package cli

import (
	"fmt"
	"path/filepath"
	"strings"
)

// target is the resolved -t/--target.
type target struct {
	arch     arch
	platform string
	name     string
}

func (t target) String() string { return t.name }

// defaultPlatform is elf for both wired arches, and is the psABI's own
// default rather than a preference: an object format is what the platform
// half names, and every arch here writes ELF.
const defaultPlatform = "elf"

// parseTarget accepts "<arch>-<platform>", a bare "<arch>", or a bare
// "<platform>". The empty string is the host arch and its default format.
func parseTarget(s string) (target, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return newTarget(hostArch(), defaultPlatform)
	}

	key := strings.ToLower(s)

	// A whole-string arch first, so x86-64 is one name and not a name with
	// a platform hanging off it.
	if a, ok := archAliases[key]; ok {
		return newTarget(a, defaultPlatform)
	}
	if both, ok := ambiguousArch[key]; ok {
		return target{}, fmt.Errorf("%q names a family, not an arch; say %s or %s",
			s, both[0], both[1])
	}

	// A bare platform, against the host arch.
	if isPlatformName(key) {
		return newTarget(hostArch(), key)
	}

	i := strings.LastIndexByte(key, '-')
	if i < 0 {
		return target{}, fmt.Errorf("no such target %q (arc has %s wired up)", s, wiredNames())
	}

	a, err := parseArch(key[:i])
	if err != nil {
		return target{}, err
	}
	return newTarget(a, key[i+1:])
}

func newTarget(a arch, platform string) (target, error) {
	if !platformSupported(a, platform) {
		return target{}, unsupportedPlatform(a, platform)
	}
	return target{arch: a, platform: platform, name: arches[a].name + "-" + platform}, nil
}

// isPlatformName reports whether a name is an object format any wired arch
// writes. Whether *this* arch writes it is newTarget's question.
func isPlatformName(s string) bool {
	for _, a := range wired {
		if platformSupported(a, s) {
			return true
		}
	}
	return false
}

// objectExt is the extension a default output name gets. It is the
// format's convention and not the arch's, which is why it lives here.
func objectExt(platform string) string {
	switch platform {
	case "coff":
		return ".obj"
	case "flat":
		return ".bin"
	}
	return ".o"
}

// ---- dialects ----------------------------------------------------------

// dialect is a spelling, never a byte. It is the CLI's own enumeration
// because i386.Dialect and x86_64.Dialect are two types with no relation —
// each arch's tree owns its own syntax — and this is the value that gets
// converted at the boundary into whichever one is being asked.
type dialect uint8

const (
	dialectNone dialect = iota
	dialectGAS
	dialectNASM
)

func (d dialect) String() string {
	switch d {
	case dialectGAS:
		return "gas"
	case dialectNASM:
		return "nasm"
	}
	return "none"
}

// parseDialect resolves a dialect name. The empty string is dialectNone —
// "nobody said" — rather than a default, because build and fmt fill that in
// from different places and a default here would hide which.
func parseDialect(s string) (dialect, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return dialectNone, nil
	case "gas", "att", "at&t":
		return dialectGAS, nil
	case "nasm", "intel":
		return dialectNASM, nil
	}
	return dialectNone, usagef("unknown dialect %q (arc has gas, nasm)", s)
}

// dialectOfExt is the dialect a filename implies. .asm is NASM's convention
// and .s is gas's; anything else reads as gas, which is what an assembler
// with no other information has always assumed.
func dialectOfExt(path string) dialect {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".asm", ".nasm":
		return dialectNASM
	}
	return dialectGAS
}

// or fills in a dialect nobody named.
func (d dialect) or(fallback dialect) dialect {
	if d == dialectNone {
		return fallback
	}
	return d
}