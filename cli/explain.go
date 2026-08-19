// cli/explain.go
package cli

import (
	"fmt"
	"strings"
)

func runExplain(args []string) error {
	fs := newFlagSet("explain")
	targetSpec := targetFlag(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return usagef("explain: no bytes given")
	}

	tgt, _, err := resolve(*targetSpec, "")
	if err != nil {
		return err
	}

	b, err := decodeHex(strings.Join(fs.Args(), " "))
	if err != nil {
		return err
	}

	out, err := opsFor(tgt.arch).explain(b)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}