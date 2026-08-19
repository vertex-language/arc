// cli/flags.go
//
// The flags every verb shares, registered in one place so -t and --target
// cannot drift apart or mean different things in two commands.
package cli

import (
	"flag"
	"os"
)

func newFlagSet(verb string) *flag.FlagSet {
	fs := flag.NewFlagSet("arc "+verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// parseFlags parses and converts the flag package's own exit behaviour into
// this package's. flag has already written the message, and for -h the
// defaults too, so the error carries a code and no text.
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return silentError{code: 2}
	}
	return nil
}

// targetFlag registers -t and --target over one variable.
func targetFlag(fs *flag.FlagSet) *string {
	p := fs.String("t", "", "target: <arch>-<platform>, e.g. x86_64-elf (default: host)")
	fs.StringVar(p, "target", "", "same as -t")
	return p
}

func dialectFlag(fs *flag.FlagSet, usage string) *string {
	return fs.String("dialect", "", usage)
}

// resolve turns the two flags every verb has into the two values every verb
// needs. Both errors are usage errors: they are mistakes in the command
// line and not in any input.
func resolve(targetSpec, dialectSpec string) (target, dialect, error) {
	tgt, err := parseTarget(targetSpec)
	if err != nil {
		return target{}, dialectNone, &usageError{err: err}
	}
	d, err := parseDialect(dialectSpec)
	if err != nil {
		return target{}, dialectNone, err
	}
	return tgt, d, nil
}