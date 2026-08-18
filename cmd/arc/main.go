// cmd/arc/main.go
package main

import (
	"os"

	"github.com/vertex-language/arc/cli"
)

func main() { os.Exit(cli.Run(os.Args[1:])) }