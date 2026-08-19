// cli/arch.go
//
// The arch vocabulary and the one switch on it. Only arch_i386.go and
// arch_x86_64.go import an arch package, and opsFor is the only thing that
// chooses between them.
//
// archOps is this package's own interface, declared here and known to
// nothing below it — an arch package has never heard of it and cannot
// implement it by accident. It carries paths, bytes, and canonical names,
// and never an operand, a register, a section or a form: a shared type that
// carried those would be an IR, which is the thing this tree is defined by
// not having. Every method is a thin adapter over one package's exported
// calls, so adding an arch is one file and one case.
package cli

import (
	"fmt"
	"runtime"
	"strings"
)

type arch uint8

const (
	archNone arch = iota
	archI386
	archX86_64
)

// archInfo is what the CLI knows about an arch without asking it.
//
// This is the one place the tree's target model is written twice, and it is
// a compromise rather than a design: the platform list belongs to the arch
// package and should come from its Platforms(). It is here because `arc
// targets` — the verb that would ask — is unlanded, and a list that exists
// in one place beats one spelled inline at four call sites.
type archInfo struct {
	name      string
	platforms []string
}

var arches = map[arch]archInfo{
	archI386:   {name: "i386", platforms: []string{"elf", "coff", "flat"}},
	archX86_64: {name: "x86_64", platforms: []string{"elf", "coff", "macho", "flat"}},
}

// wired is every arch this build can reach, in the order `arc targets`
// would print them.
var wired = []arch{archX86_64, archI386}

// archAliases resolve at the boundary and do not survive it. The canonical
// spelling is the directory name, and it is the only one anything below
// this package ever sees.
var archAliases = map[string]arch{
	"i386": archI386, "i686": archI386, "386": archI386, "ia32": archI386,

	"x86_64": archX86_64, "x86-64": archX86_64, "amd64": archX86_64,
	"x64": archX86_64, "em64t": archX86_64,
}

// ambiguousArch names a family rather than an arch. It is rejected with the
// two spellings that were meant, because guessing which one is how a
// 32-bit object ends up in a 64-bit link.
var ambiguousArch = map[string][]string{
	"x86": {"i386", "x86_64"},
}

func (a arch) String() string {
	if info, ok := arches[a]; ok {
		return info.name
	}
	return "arch(?)"
}

// hostArch is the default target's arch. An unwired host is not an error —
// it is a default, and a default that refused to exist would make every
// command on an aarch64 laptop require -t.
func hostArch() arch {
	switch runtime.GOARCH {
	case "386":
		return archI386
	default:
		return archX86_64
	}
}

func parseArch(s string) (arch, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if a, ok := archAliases[key]; ok {
		return a, nil
	}
	if both, ok := ambiguousArch[key]; ok {
		return archNone, fmt.Errorf("%q names a family, not an arch; say %s or %s",
			s, both[0], both[1])
	}
	return archNone, fmt.Errorf("no such arch %q (arc has %s wired up)", s, wiredNames())
}

func platformSupported(a arch, platform string) bool {
	for _, p := range arches[a].platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// unsupportedPlatform names the arch's own list, because the commonest way
// to reach it is asking a 32-bit target for Mach-O, and the fix is a
// different platform rather than a different spelling.
func unsupportedPlatform(a arch, platform string) error {
	return fmt.Errorf("no such target: %s-%s\n  note: %s supports %s",
		a, platform, a, strings.Join(arches[a].platforms, ", "))
}

func wiredNames() string {
	names := make([]string, 0, len(wired))
	for _, a := range wired {
		names = append(names, arches[a].name)
	}
	return strings.Join(names, ", ")
}

// unlanded is the diagnostic for a call an arch package does not export
// yet. It names the file the work lands in, because the gap is in the tree
// and not in the command line — the same reason a declared-but-unmapped
// relocation says which mapping is missing instead of "unknown relocation".
func unlanded(a arch, verb, why string) error {
	return fmt.Errorf("arc %s is not wired for %s yet: %s", verb, a, why)
}

// ---- the interface -----------------------------------------------------

type archOps interface {
	// build assembles one source file to an object file's bytes.
	build(path string, src []byte, platform string, d dialect) ([]byte, error)

	// format parses in one dialect and prints in another. from == to is a
	// reprint; from != to is a translation and needs the encoder to have
	// resolved a width neither syntax states.
	format(path string, src []byte, from, to dialect) ([]byte, error)

	// encode assembles one instruction with no section and no symbol table
	// around it.
	encode(line string, d dialect) ([]byte, error)

	// decode reads one instruction from the front of b.
	decode(b []byte) (string, error)

	// explain breaks one encoding into its fields.
	explain(b []byte) (string, error)
}

// opsFor is the switch. Nine cases when the tree is finished.
func opsFor(a arch) archOps {
	switch a {
	case archI386:
		return i386Ops{}
	case archX86_64:
		return x86_64Ops{}
	}
	panic(fmt.Sprintf("cli: no ops for %s", a))
}