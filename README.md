# arc

A toolchain for native code generation: exact encoding control, real object
files, real linked images, and an API built out of terms that don't move.

```
arc build -o main.o main.s
arc link  -o main   main.s
arc run   hello.s
```

`arc` assembles text to machine code, writes ELF, COFF, Mach-O, and flat
binaries, and links them into images. The CLI is a thin client over the library:
nothing is invented that already has a name, aliases resolve in one table at the
boundary, and the vocabulary cannot drift because registers and relocation kinds
don't drift.

---

## Install

```sh
GOPROXY=direct go install github.com/vertex-language/arc/cmd/arc@latest
```

As a library:

```sh
go get github.com/vertex-language/arc
```

The module root is a module path, not an import. You import an arch:

```go
import "github.com/vertex-language/arc/x86_64"
```

---

## Quick start

```asm
# hello.s
        .section .text
        .globl _start
_start:
        mov     $1, %rax            # write
        mov     $1, %rdi            # stdout
        lea     msg(%rip), %rsi
        mov     $15, %rdx
        syscall

        mov     $60, %rax           # exit
        xor     %rdi, %rdi
        syscall

        .section .rodata
msg:    .ascii "hello, silicon\n"
```

```console
$ arc run hello.s
hello, silicon

$ arc build -t aarch64-macho -o hello.o hello.s     # cross-assemble
$ arc fmt --dialect nasm hello.s                    # translate, byte-identical
$ arc dis 48c7c03c000000
;; x86_64 · gas · host default target
mov rax, 60

$ arc explain 48c7c03c000000
mov rax, 60                                    x86_64 · base · 7 bytes

  48         REX      W=1 R=0 X=0 B=0          64-bit operand size
  c7         opcode   MOV r/m64, imm32         /0
  c0         ModRM    mod=11 reg=000 rm=000    → rax
  3c000000   imm32    60                       0x3c
```

The same program from Go:

```go
package main

import (
    "os"

    "github.com/vertex-language/arc/x86_64"
)

func main() {
    a := x86_64.New(x86_64.ELF)

    t := a.Section(x86_64.Text)
    t.Align(16)
    t.Label("_start", x86_64.Global, x86_64.Func)
    t.MovR64Imm64(x86_64.RAX, 60)      // 48 c7 c0 3c 00 00 00
    t.XorRM64R64(x86_64.RDI, x86_64.RDI)
    t.Syscall()

    b, err := a.Serialize()
    if err != nil {
        panic(err)
    }
    os.WriteFile("exit.o", b, 0644)
}
```

The import is the target declaration — the arch half of `-t` is the import path,
and nothing else in the file names an arch. `New` takes what is left, and each
package declares only the platforms it has, so `riscv64.MachO` is not a runtime
error but a compile error: it is undefined.

---

## Why

Compiler backends get rewritten every time the IR changes, because they are
built against the IR's shape.

`arc` is described purely in terms that don't move: physical registers,
immediates, memory operands, relocation kinds, sections, and symbols. An API
built only from these inherits their stability. The consequence, and the point:
**`arc` does not know your IR exists.**

---

## What arc is not

Load-bearing exclusions, not gaps to fill later.

- **Not an IR.** No intermediate representation, no optimizer, no instruction
  selection. `Emit` picks an *encoding* of the instruction you named; it never
  picks a different instruction, never folds, never reorders. Nothing survives
  the call — a section is a byte buffer and a fixup list.
- **Not a register allocator.** Every operand is a physical register. There is no
  virtual register type to pass.
- **Not a macro expander.** No `.if`, no `.rept`, no `.macro`. Go is the macro
  language and it is better at it.
- **Not a layout engine.** Sections come out in creation order with the alignment
  you asked for. Address assignment is `linker/`'s.
- **Not a place to invent syntax.** Every grammar accepted already exists and is
  documented elsewhere. If translating would need a macro expander or a
  conditional-assembly evaluator, it is a *language*, and languages are out —
  which is why MASM and HLASM are excluded and `gas`/`nasm` are not.
- **Not a compiler driver.** No `-g`, no libc, no C runtime, no linker script
  DSL. Debug bytes attach as opaque payloads and pass through untouched.
- **Not multi-target beyond real silicon.** 32- and 64-bit general-purpose
  silicon with a published psABI and a GNU `as` to differential-test against.
  Bytecode, VMs, `wasm`, and GPUs are out; 8/16-bit MCUs are out for lack of a
  linked-image story.
- **No cross-arch anything.** Not a shared `Reg`, not a shared `Section`, not a
  shared `Operand`, not a shared lexer. A shared type that carried operands
  would be an IR — the thing this tree is defined by not having.

---

## The target model

Five knobs, one matrix lookup at the boundary.

```
-t, --target    <arch>-<platform>   default: host
    --abi       <name>              default: psABI default for the target
    --dialect   gas | nasm          x86_64 and i386 only
    --features  <set>               default: psABI baseline
```

```console
$ arc targets
ARCH          PLATFORMS        ABIS                  DIALECTS   BASELINE
x86_64        elf macho coff   —                     gas nasm   x86-64-v1
i386          elf coff         —                     gas nasm   i686
aarch64       elf macho coff   —                     —          armv8-a
arm           elf coff         hard softfp soft      —          armv7-a+vfpv3
riscv64       elf              lp64d lp64f lp64s     —          rv64gc
riscv32       elf              ilp32d ilp32f ilp32s  —          rv32gc
powerpc64le   elf              —                     —          power8
s390x         elf              —                     —          z13
loongarch64   elf              lp64d lp64f lp64s     —          la64v1.0
```

That table is `cmd/arc` asking each of the nine packages for `Platforms()`,
`ABIs()`, `Dialects()`, and `Baseline()` — the encoder's own data, so it cannot
drift from what `arc build` accepts.

- **Arch names are canonical.** psABI document, ELF `e_machine`, LLVM triple, in
  that order — and they are the nine directory names. `amd64`, `arm64`, `rv64gc`
  and full LLVM triples are accepted and discarded at the boundary. Endianness
  is in the name; there is no `--endian`.
- **Ambiguity is rejected, not guessed.** `x86` names a family and half the
  world means 32-bit by it. `armhf` names an arch *and* a float ABI that lands
  in the object header. Both are errors that name the two spellings you meant.
- **Platforms are object formats**, not operating systems. `arc` does not know
  what Linux is; OS and vendor fields from LLVM triples are discarded.
- **ABI exists only where the psABI defines a choice and the object records it**
  — `--abi lp64d`, `--abi hard`. Elsewhere it is a usage error, not a no-op, and
  `arc link` refuses to merge objects that disagree. In the library it is a
  parameter where it exists and absent where it doesn't.
- **Features are gated at encode time.** An unlisted extension is a diagnostic
  with a line number naming the flag that would have allowed it. Unratified
  extensions are not encodable at all.
- **A dialect is a spelling, never a byte.** Mnemonics, operand order, size
  disambiguation, and the section/symbol/data directive spellings. Never macros.
  ARM `.arm`/`.thumb` is encoding state, not a dialect, because it changes bytes.

Full detail: [docs/cli.md](docs/cli.md).

---

## The round-trip guarantee

`parse` and `print` are inverses at the semantic level. Both dialects parse *to*
and print *from* one `text.Unit`, so the round trip is a property of the code
rather than a claim in a README. Anything `arc fmt` changes assembles to
identical bytes, verified continuously against GNU `as` and NASM by a
differential suite that had to exist anyway.

```sh
arc fmt -w main.s              # normalize
arc fmt --dialect nasm att.s   # translate
arc fmt --check .              # CI gate, exit 4
```

This is the only formatter for assembly that can make that claim without a
caveat, and the reason it can is that the encoder resolves the form before the
printer runs — a text-level translator can't recover operand size in both
directions.

---

## Repository layout

```
arc/
├── cmd/arc/         the CLI — verb dispatch, flag parsing, the alias table,
│                    and the one nine-case switch on arch
│
├── x86_64/  i386/  aarch64/  arm/  riscv64/  riscv32/
├── powerpc64le/  s390x/  loongarch64/
│                    nine complete, independent arch packages: registers, ISA
│                    tables, generated helpers, relocation kinds, encoder,
│                    decoder, and text/ — the dialects, one directory each
│
├── objectfile/      elf, coff, macho, flat — relocatable objects
├── linker/          elf, macho, pe — linked images
│
└── internal/
    ├── gen/         table generators — build-time only, imported by nothing
    └── difftest/    differential suite against GNU as and NASM
```

**Nothing lives at the module root.** There is no `arc` package, no `arch`
package, and no builder package — there are nine, and each is complete. Their
directory names are the first column of `arc targets`. Directories are arches;
files are tasks, because adding an arch is the recurring change and the verb
list is closed.

**Duplication is the design.** ModRM encoding appears twice; `i386/text/gas` is
a full copy of x86_64's parser. A helper is copied into every package that needs
it, and a name is redeclared in every package that uses it. `i386.EAX` is an
architectural register and `x86_64.EAX` is a view that zero-extends into one —
one type spelled `EAX` would answer `Parent()` differently depending on which
mode you were thinking about.

Five import rules, all checkable by a script and all worth a CI gate: no arch
package imports another arch package; no arch package imports `linker/` and no
`linker/` package imports an arch package; nothing imports an arch package;
`x86_64/text/gas` never imports `x86_64` itself; and `cmd/arc` is the only thing
that imports more than one arch package, behind one switch.

Two READMEs below this one carry their own rules and are worth reading before
using either tree:

- [`objectfile/README.md`](objectfile/README.md) — four independent format
  packages with **no shared types**, on purpose. `elf.Section` and
  `coff.Section` are not interchangeable and there is no `Builder` interface,
  because `Reloc.Addend` means `r_addend` in ELF and an in-place patch of `Code`
  in COFF and Mach-O, and one type would flatten that into a doc comment.
- [`linker/README.md`](linker/README.md) — three independent linkers, selected
  by format, each with its own `Target` spelled the way that format's native
  tooling spells it. No package lives at `linker/` itself.

Each arch package and `linker/` are the only places in the tree that import more
than one `objectfile` sub-package, and all of them do it behind a switch on the
platform, never behind an interface.

---

## Documentation

| | |
| --- | --- |
| [docs/cli.md](docs/cli.md) | every command, flag, exit code, and the target model |
| [docs/builder.md](docs/builder.md) | the library API the CLI is a client of |
| [objectfile/README.md](objectfile/README.md) | object formats, section and relocation mapping |
| [linker/README.md](linker/README.md) | image formats, routing to the three backends |

---

## Status

The design is settled; the tree is not finished. What is real today:

| | |
| --- | --- |
| `objectfile/elf`, `coff`, `macho`, `flat` | implemented — x86_64, aarch64, riscv64, i386 |
| `linker/elf`, `macho`, `pe` | implemented |
| `x86_64`, `i386`, `aarch64`, `riscv64` | in progress |
| `arm`, `riscv32`, `powerpc64le`, `s390x`, `loongarch64` | unlanded |
| `cmd/arc` | in progress |

The nine-arch matrix above is the target model; five of the nine packages are
unlanded, and each lands in `objectfile` before it lands in `arc targets`.

---

## License

MIT