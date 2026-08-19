// cli/enc.go
package cli

import "fmt"

func runEnc(args []string) error {
	fs := newFlagSet("enc")
	targetSpec := targetFlag(fs)
	dialectSpec := dialectFlag(fs, "input syntax: gas | nasm")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	lines := fs.Args()
	if len(lines) == 0 {
		return usagef("enc: no instruction given")
	}

	tgt, d, err := resolve(*targetSpec, *dialectSpec)
	if err != nil {
		return err
	}
	ops := opsFor(tgt.arch)

	// There is no file here to take a dialect from, so an unnamed one is
	// gas rather than an error.
	d = d.or(dialectGAS)

	for _, line := range lines {
		b, err := ops.encode(line, d)
		if err != nil {
			return fmt.Errorf("%q: %w", line, err)
		}
		fmt.Println(hexBytes(b))
	}
	return nil
}