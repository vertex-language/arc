// cli/cli.go
//
// Verb dispatch and exit codes. Nothing here knows what an arch is.
package cli

import (
	"errors"
	"fmt"
	"os"
)

// Run dispatches a verb and returns a process exit code.
//
//	0  the command did what it said
//	1  a diagnostic — a file that will not parse, an instruction with no
//	   encoding, a relocation the format cannot record
//	2  usage — an unknown verb, a missing argument, a flag that does not
//	   apply here
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

	if err == nil {
		return 0
	}

	// A flag error has already been printed by the flag package, including
	// the defaults for -h. Printing it again would say it twice.
	var silent silentError
	if errors.As(err, &silent) {
		return silent.code
	}

	fmt.Fprintf(os.Stderr, "arc: %v\n", err)

	var u *usageError
	if errors.As(err, &u) {
		return 2
	}
	return 1
}

// usageError is a mistake in the command line rather than in the input. It
// is the only thing that separates exit 2 from exit 1, so the distinction
// lives in one type instead of in every return statement.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func usagef(format string, a ...any) error {
	return &usageError{err: fmt.Errorf(format, a...)}
}

// silentError has already been reported to stderr by whoever produced it.
type silentError struct{ code int }

func (silentError) Error() string { return "already reported" }

func printUsage(w *os.File) {
	fmt.Fprint(w, `arc — assembler

USAGE
  arc <command> [flags] [args]

COMMANDS
  build     assemble to a relocatable object
  fmt       normalize or translate assembly text
  enc       encode instructions given on the command line
  dis       decode bytes to an instruction
  explain   break one encoding into its fields
  version   print version
  help      show this message

TARGETS
  -t, --target <arch>-<platform>    default: host, falling back to x86_64-elf

    x86_64    elf coff macho flat
    i386      elf coff flat

  Either half may stand alone: -t x86_64 takes elf, -t coff takes the host
  arch. Aliases (amd64, x64, i686) resolve here and do not survive the
  boundary. -t x86 is an error, not a guess: it names a family and half the
  world means 32-bit by it.

  --dialect gas | nasm    (aliases: att, at&t, intel)

    build reads --dialect as the input syntax; without it the extension
    decides, and .asm means nasm. fmt reads it as the output syntax and
    takes the input's from the extension either way.

  Flags come before file arguments.

NOT REACHABLE YET
  link, run, obj, isa, regs, targets, env, --abi, --features, and the seven
  arch packages past x86_64 and i386. arc build and arc enc are wired for
  i386 only; the x86_64 side of both names the call it is waiting on. Full
  design: docs/cli.md.
`)
}