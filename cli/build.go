// cli/build.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runBuild(args []string) error {
	fs := newFlagSet("build")
	targetSpec := targetFlag(fs)
	dialectSpec := dialectFlag(fs, "input syntax: gas | nasm (default: from the file extension)")
	out := fs.String("o", "", "output path")
	fs.StringVar(out, "output", "", "same as -o")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	inputs := fs.Args()
	if len(inputs) == 0 {
		return usagef("build: no input files")
	}
	if len(inputs) > 1 && *out != "" {
		return usagef("build: -o names one output and there are %d inputs", len(inputs))
	}

	tgt, d, err := resolve(*targetSpec, *dialectSpec)
	if err != nil {
		return err
	}
	ops := opsFor(tgt.arch)

	for _, in := range inputs {
		if err := buildOne(ops, in, *out, tgt, d); err != nil {
			return fmt.Errorf("%s: %w", in, err)
		}
	}
	return nil
}

func buildOne(ops archOps, path, out string, tgt target, d dialect) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	b, err := ops.build(path, src, tgt.platform, d.or(dialectOfExt(path)))
	if err != nil {
		return err
	}

	if out == "" {
		out = defaultOutput(path, tgt.platform)
	}
	return os.WriteFile(out, b, 0o644)
}

// defaultOutput is the input's base name with the format's extension, in
// the working directory. It follows the input's name and not its directory,
// which is what every assembler does and what a Makefile expects.
func defaultOutput(path, platform string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return base + objectExt(platform)
}