// cli/enc.go
package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/vertex-language/arc/i386"
)

func runEnc(args []string) error {
	fs := flag.NewFlagSet("enc", flag.ExitOnError)
	dialectFlag := fs.String("dialect", "gas", "gas | nasm")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dialect, err := parseDialect(*dialectFlag)
	if err != nil {
		return err
	}

	for _, line := range fs.Args() {
		inst, err := i386.ParseInst(line, dialect)
		if err != nil {
			return fmt.Errorf("%q: %w", line, err)
		}
		b, _, err := i386.Encode(i386.DefaultFeatures(), inst)
		if err != nil {
			return fmt.Errorf("%q: %w", line, err)
		}
		fmt.Println(hexBytes(b))
	}
	return nil
}

func hexBytes(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(parts, " ")
}