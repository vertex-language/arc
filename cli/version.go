// cli/version.go
package cli

import "fmt"

const version = "0.0.2"

func printVersion() {
	fmt.Printf("arc %s (%s)\n", version, wiredNames())
}