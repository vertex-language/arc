// cli/explain.go
package cli

import (
	"flag"
	"fmt"

	"github.com/vertex-language/arc/i386"
)

func runExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("explain: expected one hex argument")
	}

	b, err := decodeHex(fs.Arg(0))
	if err != nil {
		return err
	}

	fields, err := i386.Explain(b)
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", fields)
	return nil
}