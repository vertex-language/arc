// cli/cli.go
package cli

import (
	"fmt"
	"os"
)

// Run dispatches a verb and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	verb, rest := args[0], args[1:]

	var err error
	switch verb {
	case "build":
		err = runBuild(rest)
	case "fmt":
		err = runFmt(rest)
	case "enc":
		err = runEnc(rest)
	case "dis":
		err = runDis(rest)
	case "explain":
		err = runExplain(rest)
	case "version", "--version":
		printVersion()
		return 0
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "arc: unknown command %q\n", verb)
		printUsage(os.Stderr)
		return 2
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "arc: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `arc — assembler (i386 only, early)

USAGE
  arc <command> [flags] [args]

COMMANDS
  build     assemble to a relocatable object
  fmt       normalize or translate assembly text
  enc       encode an instruction given on the command line
  dis       decode bytes to an instruction
  explain   break one encoding into its fields
  version   print version
  help      show this message

Only i386 is wired up. -t/--target accepts i386-elf, i386-coff,
i386-flat (default i386-elf). --dialect accepts gas (default) or nasm.
Everything else in the full design (docs/cli.md) — link, run, obj,
isa, regs, targets, env, --abi, --features — isn't reachable yet.
`)
}