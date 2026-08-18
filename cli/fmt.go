// cli/fmt.go
package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/vertex-language/arc/i386"
)

func runFmt(args []string) error {
	fs := flag.NewFlagSet("fmt", flag.ExitOnError)
	write := fs.Bool("w", false, "rewrite in place")
	dialectFlag := fs.String("dialect", "", "print in this dialect (default: same as input)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths := fs.Args()
	if len(paths) == 0 {
		return fmt.Errorf("fmt: no input files")
	}

	for _, path := range paths {
		if err := fmtOne(path, *write, *dialectFlag); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func fmtOne(path string, write bool, dialectFlag string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	in := i386.GAS
	if strings.HasSuffix(path, ".asm") {
		in = i386.NASM
	}

	unit, err := i386.ParseFile(path, string(src), in)
	if err != nil {
		return err
	}

	out := in
	if dialectFlag != "" {
		if out, err = parseDialect(dialectFlag); err != nil {
			return err
		}
	}

	text, err := i386.Print(unit, out)
	if err != nil {
		return err
	}

	if write {
		return os.WriteFile(path, []byte(text), 0o644)
	}
	_, err = fmt.Print(text)
	return err
}