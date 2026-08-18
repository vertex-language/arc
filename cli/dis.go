// cli/dis.go
package cli

import (
	"encoding/hex"
	"flag"
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386"
)

func runDis(args []string) error {
	fs := flag.NewFlagSet("dis", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("dis: expected one hex argument")
	}

	b, err := decodeHex(fs.Arg(0))
	if err != nil {
		return err
	}

	inst, err := i386.Decode(b)
	if err != nil {
		return err
	}

	// TODO: render inst through the dialect printer once decode.Inst's shape
	// is known here — that needs a DecodedInst -> text.Inst step this
	// package can't write without seeing decode/. Until then, print the
	// decoded struct directly, the same way arc explain does.
	fmt.Printf("%+v\n", inst)
	return nil
}

func decodeHex(s string) ([]byte, error) {
	s = strings.NewReplacer(" ", "", ",", "", "0x", "").Replace(s)
	return hex.DecodeString(s)
}