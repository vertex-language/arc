// cli/fmt.go
package cli

import (
	"bytes"
	"fmt"
	"os"
)

func runFmt(args []string) error {
	fs := newFlagSet("fmt")
	targetSpec := targetFlag(fs)
	dialectSpec := dialectFlag(fs, "print in this syntax: gas | nasm (default: the input's)")
	write := fs.Bool("w", false, "rewrite the file in place")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	paths := fs.Args()
	if len(paths) == 0 {
		return usagef("fmt: no input files")
	}

	tgt, out, err := resolve(*targetSpec, *dialectSpec)
	if err != nil {
		return err
	}
	ops := opsFor(tgt.arch)

	for _, path := range paths {
		if err := fmtOne(ops, path, *write, out); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// fmtOne reads the input's dialect from its extension and prints in
// whichever one --dialect named, defaulting to the same one.
func fmtOne(ops archOps, path string, write bool, out dialect) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	in := dialectOfExt(path)
	text, err := ops.format(path, src, in, out.or(in))
	if err != nil {
		return err
	}

	if !write {
		_, err = os.Stdout.Write(text)
		return err
	}

	// A file that is already formatted is not rewritten. Touching its mtime
	// would rebuild everything downstream of it for no change in bytes.
	if bytes.Equal(text, src) {
		return nil
	}
	return os.WriteFile(path, text, 0o644)
}