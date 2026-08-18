// cli/build.go
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vertex-language/arc/i386"
)

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	t := fs.String("t", "", "target, e.g. i386-elf")
	fs.StringVar(t, "target", "", "target, e.g. i386-elf")
	dialectFlag := fs.String("dialect", "gas", "gas | nasm")
	out := fs.String("o", "", "output path")
	fs.StringVar(out, "output", "", "output path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	inputs := fs.Args()
	if len(inputs) == 0 {
		return fmt.Errorf("build: no input files")
	}
	if len(inputs) > 1 && *out != "" {
		return fmt.Errorf("build: -o is a usage error with multiple inputs")
	}

	tgt, err := parseTarget(*t)
	if err != nil {
		return err
	}
	dialect, err := parseDialect(*dialectFlag)
	if err != nil {
		return err
	}

	for _, in := range inputs {
		if err := buildOne(in, *out, tgt, dialect); err != nil {
			return fmt.Errorf("%s: %w", in, err)
		}
	}
	return nil
}

func buildOne(path, out string, tgt target, dialect i386.Dialect) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	unit, err := i386.ParseFile(path, string(src), dialect)
	if err != nil {
		return err
	}

	b, err := i386.Assemble(unit, tgt.platform, i386.Baseline())
	if err != nil {
		return err
	}

	if out == "" {
		out = defaultOutput(path, tgt.platform)
	}
	return os.WriteFile(out, b, 0o644)
}

func defaultOutput(path string, p i386.Platform) string {
	ext := ".o"
	if p == i386.COFF {
		ext = ".obj"
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return base + ext
}