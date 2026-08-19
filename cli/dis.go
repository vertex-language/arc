// cli/dis.go
package cli

import (
	"fmt"
	"strings"
)

func runDis(args []string) error {
	fs := newFlagSet("dis")
	targetSpec := targetFlag(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return usagef("dis: no bytes given")
	}

	tgt, _, err := resolve(*targetSpec, "")
	if err != nil {
		return err
	}

	// The arguments are joined rather than taken one at a time, so both
	// `arc dis 48c7c03c000000` and `arc dis 48 c7 c0 3c 00 00 00` name the
	// same seven bytes — the second is what a hex dump pastes as.
	b, err := decodeHex(strings.Join(fs.Args(), " "))
	if err != nil {
		return err
	}

	// TODO: render through the dialect printer once there is a
	// decode.Inst → text.Inst step to hand PrintInst. That step is the arch
	// package's; this one cannot import decode/ to write it.
	out, err := opsFor(tgt.arch).decode(b)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}