// cli/version.go
package cli

import "fmt"

const version = "0.0.1-i386"

func printVersion() {
	fmt.Printf("arc %s (i386 only)\n", version)
}