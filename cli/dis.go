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
	dialectFlag := fs.String("dialect", "gas", "gas | nasm")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dialect, err := parseDialect(*dialectFlag)
	if err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("dis: expected one hex argument")
	}

	b, err := decodeHex(fs.Arg(0))
	if err != nil {
		return err
	}

	inst, _, err := i386.Decode(b)
	if err != nil {
		return err
	}

	out, err := i386.PrintInst(inst, dialect)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func decodeHex(s string) ([]byte, error) {
	s = strings.NewReplacer(" ", "", ",", "", "0x", "").Replace(s)
	return hex.DecodeString(s)
}