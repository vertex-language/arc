# CLI

`cmd/arc` is a thin client over the library. The rules are the library's rules:
nothing invented that already has a name, aliases resolved in one table at the
boundary, and a vocabulary that cannot drift because registers and relocation
kinds don't drift.

---

## Shape

```
arc <command> [flags] [args]
```

Verb first, like `go` and `git` — not `nasm f.s`, not `as f.s`.

`arc` does five unrelated things: assemble, link, decode, inspect, query. A flat
flag namespace forces mode flags — `llvm-mc -assemble` vs `-disassemble` — and
mode flags are how `llvm-mc` ended up with `--filetype`, `--output-asm-variant`,
`-x86-asm-syntax`, and `-aarch64-neon-syntax` all describing overlapping things.
A verb has a namespace of its own. `arc dis` needs no `--mode=disassemble`.

The cost: `arc main.s -o main.o` from the README becomes `arc build -o main.o
main.s`. Worth it.

| | |
| --- | --- |
| **Build** | |
| `arc build` | assemble to a relocatable object |
| `arc link` | link objects into an image |
| `arc run` | assemble, link, execute |
| `arc fmt` | normalize or translate assembly text |
| **Inspect** | |
| `arc enc` | encode instructions given as arguments |
| `arc dis` | decode bytes, objects, or images |
| `arc explain` | break one encoding into its fields |
| `arc obj` | inspect an object file or image |
| **Query** | |
| `arc isa` | look up instruction forms |
| `arc regs` | look up registers and their views |
| `arc targets` | print the target matrix |
| `arc env` | show resolved configuration |
| `arc version` | print version |
| `arc completion` | emit a shell completion script |

Query commands read the same generated tables the encoder does. `arc isa` cannot
describe an instruction `arc build` won't encode.

---

## Target selection

Five knobs, one boundary. `-t` is the one in every example; the rest are
overrides documented here and nowhere else.

```
-t, --target    <arch>-<platform>   default: host
    --arch      <arch>              overrides the arch half
    --platform  <platform>          overrides the platform half
    --abi       <name>              default: psABI default for the target
    --dialect   gas | nasm          x86_64 and i386 only; default gas
    --features  <set>               default: psABI baseline
```

```
arc build -t aarch64-macho -o f.o f.s
arc build -t x86_64-elf --dialect nasm --features avx2,bmi2 -o k.o kernel.s
arc build -t riscv64-elf --abi lp64f --features rva23u64 -o k.o kernel.s
```

Default target is the host: `arch.FromGOARCH(runtime.GOARCH)` plus the native
object format for `runtime.GOOS`. One call each, no table.

### Architectures

Canonical spelling = psABI document name, ELF `e_machine`, and LLVM triple, in
that order of authority. Endianness lives in the name; there is no `--endian`.

| Canonical | Aliases accepted |
| --- | --- |
| `x86_64` | `amd64`, `x64`, `x86-64`, `em64t`, `intel64` |
| `i386` | `i686`, `ia32`, `386`, `x86_32` |
| `aarch64` | `arm64`, `armv8`, `armv8-a` |
| `arm` | `arm32`, `armv7`, `armv7a`, `thumbv7` |
| `riscv64` | `rv64`, `rv64gc` |
| `riscv32` | `rv32`, `rv32gc` |
| `powerpc64le` | `ppc64le`, `powerpc64el` |
| `s390x` | `s390`, `zarch`, `systemz` |
| `loongarch64` | `la64`, `loongarch` |

Full LLVM triples are accepted and reduced. They carry vendor, OS, and ABI
fields; the first two `arc` has no use for, and the third resolves to `--abi`.
Discarding vendor and OS at the boundary is cheaper than explaining that `arc`
has no libc.

**`x86` is rejected, not aliased.** It names the family, and half the world
means 32-bit by it while the other half means the family including 64-bit:

```
error: ambiguous arch "x86"
  note: "x86" names the family; use i386 (32-bit) or x86_64 (64-bit)
```

**`armhf` and `armel` are rejected, not aliased.** They name an arch *and* a
float ABI, and the ABI is recorded in the object header. Aliasing the pair to
the bare arch would silently produce an object with the wrong `e_flags`:

```
error: "armhf" names an arch and an ABI
  note: use -t arm-elf --abi hard
```

**Where the line sits.** 32- and 64-bit general-purpose silicon with a published
psABI and a GNU `as` to differential-test the round trip against. That excludes
8/16-bit MCU targets — they have psABIs and GNU support, but their world is flat
binaries and vendor flashing tools, with no linked-image story worth the
`linker/` tree. It excludes `mips64el`, `sparc64`, `m68k`, `ia64` on volume.
`wasm`, `nvptx`, `amdgcn`, `spirv` are out under "real silicon." State the line;
don't enumerate the rejections.

### Platforms

`elf`, `macho`, `coff`. The platform names an object format, not an OS. `arc`
does not know what Linux is.

### The matrix

Nine arches by three platforms is 27 pairs and about half are real. `-t` is a
matrix lookup, and a miss names the valid other half:

```
$ arc build -t riscv64-macho f.s
error: no such target: riscv64-macho
  note: riscv64 supports: elf
  note: macho supports: x86_64, aarch64
```

```
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

That table is the whole target model on one screen, and it is generated from the
same tables the encoder reads — so it cannot drift from what `arc build`
accepts. `arc targets <arch>` narrows it; `--json` gives the same matrix.

### ABI

An ABI value exists exactly where the psABI defines more than one calling
convention *and the object file records which one is in use*. That is the test,
and it is why `--abi` is a separate flag rather than a feature: features change
which instructions encode, ABI changes the file header, and the linker enforces
it.

```
--abi lp64d          RISC-V, LoongArch: data model + float convention
--abi hard           ARM: AAPCS-VFP
--abi help           list what this target has
```

On ELF the value lands in `e_flags` — `EF_RISCV_FLOAT_ABI_DOUBLE`,
`EF_ARM_ABI_FLOAT_HARD` — and for `arm` also in the `Tag_ABI_VFP_args` build
attribute. `arc obj header` prints it. `arc link` refuses to merge objects that
disagree, because every psABI that defines the field says the linker must:

```
error: incompatible float ABI
  math.o:   lp64d
  kernel.o: lp64s
  note: objects must agree; the psABI does not define a mixed image
```

Where the column is a dash, the platform determines the convention and there is
nothing to choose. `--abi` on those arches is a usage error, not a no-op.

### Dialects

A **dialect** is an alternate spelling of content `arc` already models: same
instructions, same directives, same bytes. If translating in either direction
would need a macro expander, a conditional-assembly evaluator, or a column
model, it is a *language*, and languages are out.

That leaves two values, both on x86:

| Canonical | Aliases | Reference |
| --- | --- | --- |
| `gas` | `att`, `at&t` | GNU `as` |
| `nasm` | `intel` | NASM |

`gas` and `nasm` are canonical because `--dialect intel` is a lie of omission —
FASM, MASM, NASM, TASM and YASM all claim Intel syntax, and GAS itself has
supported a fourth variant via `.intel_syntax` since 2.10. `--dialect nasm`
names what you actually get.

On every other arch the flag is a usage error, not a silently-ignored no-op:

```
$ arc build -t riscv64-elf --dialect nasm f.s
error: --dialect does not apply to riscv64
  note: riscv64 has one documented syntax; only x86_64 and i386 have a choice
```

Single-dialect arches get a dash in `arc targets`. Nothing is named `a64` or
`ual` or `riscv`, because a set with one member doesn't need names. That also
disposes of UAL cleanly: UAL is A32/T32 syntax and `arm`'s only one, A64 is a
different language and `aarch64`'s only one, and neither is ever typed. Both
still get a rejection, because both will be typed:

```
error: unknown dialect "ual" for aarch64
  note: UAL is the A32/T32 syntax; A64 assembly is a different language
  note: aarch64 has one dialect; --dialect does not apply
```

**MASM and HLASM are excluded as languages, not as quirks.** MASM has `PROC`/
`ENDP` with automatic prologue and epilogue, `INVOKE`, and `PROTO` — structured
constructs and an implicit calling-convention model, which "not a compiler
driver" and "not a preprocessor" both already reject. HLASM has conditional
assembly, macro generation, and a fixed-column source format where column 72 is
continuation and 73–80 are sequence numbers. LLVM carries a SystemZ HLASM
dialect; `arc` does not, and the reason is the rule above rather than volume.

**What a dialect covers.** Mnemonics, operand order, operand syntax, size
disambiguation, and the directive spellings for section, symbol, and data
definition — `section .text` / `global` / `db` against `.section` / `.globl` /
`.byte`. Never macros. Never bytes. Size is recoverable in both directions
because the encoder resolves the form before the printer runs; that is why the
round trip holds and why a text-level translator can't claim the same.

**ARM instruction-set state is not a dialect.** `arm` vs `thumb` is encoding
state, selected by `.arm`/`.thumb` in the source the way GNU `as` does it, with
`--features +thumb` gating availability. It changes bytes; a dialect never does.

### Features

One flag, one alias table, arch-specific spellings resolved at the boundary
exactly like arch names.

```
--features avx2,bmi2         set exactly
--features +avx2,-sse4.2     adjust the baseline
--features x86-64-v3         microarchitecture level, expanded
--features rva23u64          ratified RISC-V profile, expanded
--features rv64imafdc_zicsr  RISC-V ISA string, expanded
--features armv8.2-a+fp16    architecture revision + extension
--features z16               IBM Z generation level
--features power10           Power ISA level
--features help              list what this target has
```

Levels, profiles, and ISA strings are the same mechanism: a name that expands to
a set. `x86-64-v3` and `rva23u64` differ only in who ratified them.

Feature gating is enforced at encode time, so an unlisted extension is an error
with a line number, not a silent emission. The error names the flag that would
have allowed it, and prints the active set in canonical order:

```
kernel.s:14:5: error: vsetvli requires v, not in the active feature set
  active: rv64imafdc_zicsr_zifencei
  note: add --features +v
```

RISC-V ISA strings are canonically ordered. Input order is free; output order is
not. `arc env ARC_FEATURES` and every diagnostic print the canonical form — the
same one-spelling-out, many-in rule as hex input to `arc dis`.

Extensions that are not ratified are not encodable, whatever the toolchain
you came from spells them:

```
error: experimental extension "zvfbfmin" is not accepted
  note: arc encodes ratified extensions only
  note: experimental encodings change between toolchain versions
```

### Aliases

Resolved here and nowhere else, in the one lookup table. The arch rows are in
Architectures above; the rest:

| Accepted | Canonical |
| --- | --- |
| `att`, `at&t` | `--dialect gas` |
| `intel` | `--dialect nasm` |
| `x86_64-linux-gnu`, `x86_64-unknown-linux-musl` | `x86_64-elf` |
| `aarch64-apple-darwin`, `arm64-apple-macos` | `aarch64-macho` |
| `x86_64-pc-windows-msvc` | `x86_64-coff` |
| `riscv64-linux-gnu` | `riscv64-elf --abi lp64d` |

---

## Conventions

The grammar every command below inherits.

**Short flags are scarce on purpose.** `-o`, `-t`, `-w`, `-v`, `-q`, `-h`. Every
flag has a long form; most have only that. Burning `-f` on "format" the way NASM
does means the next format-adjacent flag has to be `-F`.

**Standard names where a standard exists.** `-o` is `-o`. If a flag would need a
name no existing assembler has, that's a signal the behavior doesn't belong in
`arc`.

**stdout is data, stderr is narration.** Piping `arc dis` never picks up a
progress line. `-v` and `-q` change narration volume only — they never add to,
remove from, or reorder stdout. `-q` silences everything on stderr except
diagnostics.

**`-` means stdin/stdout** anywhere a path is accepted.

**`arc` never prompts.** There is no interactive mode, no confirmation, no
`--yes`. The only TTY-dependent behavior in the tool is color.

**Output is deterministic.** Identical input and flags produce byte-identical
output. Embedded timestamps — COFF `TimeDateStamp`, archive member dates — are
zero unless `SOURCE_DATE_EPOCH` is set, in which case they are clamped to it.
There is no `--deterministic` flag because there is no other mode.

**`--json` is the stable contract.** Human output is prose and may be reworded in
any release. JSON is versioned: every top-level object carries `"schema"` with
an integer that increments only on a breaking change. Saying so up front is
cheaper than the alternative, which is discovering four years later that someone
regexes your column alignment.

**Diagnostics are `file:line:col: level: message`,** the format every editor
already parses. Notes attach below with `note:` and suggest the flag or spelling
that would fix it.

**Colors:** `--color auto|always|never`. `auto` means TTY. `NO_COLOR` and
`TERM=dumb` disable color; `--color always` overrides both.

| Exit | |
| --- | --- |
| 0 | success |
| 1 | diagnostics — the input was wrong |
| 2 | usage — the command line was wrong |
| 3 | I/O — the filesystem was wrong |
| 4 | check failed — `fmt --check` found a file that would change |

4 exists because a file that needs reformatting is not wrong input, and CI needs
to tell the two apart without parsing stderr.

**Help:** `arc`, `arc help`, `arc help build`, `arc build --help`, and `arc build
-h` all work. Help leads with examples, then flags. `--version` is an alias for
`arc version`; `-v` is verbose and always has been.

**Completions:** `arc completion bash|zsh|fish|pwsh` writes a script to stdout.

---

## Build

```
arc build [flags] <file.s>...

  -o, --output <path>              output path; `-` for stdout
      --list <path>                listing: source, offsets, bytes
      --debug-section <name>=<path>  attach bytes as an opaque payload
```

One input, one object. `-o` defaults to the input basename with the format's
extension (`.o`, `.obj`). Multiple inputs assemble independently, each to its own
basename; `-o` with multiple inputs is a usage error, because merging objects is
linking and `arc link` already does it.

```
arc build main.s                       # → main.o
arc build -o main.o main.s
arc build *.s                          # → one object each
cat main.s | arc build -o main.o -     # stdin
arc build --debug-section .debug_info=dw.bin -o main.o main.s
```

`--debug-section` writes the file's bytes into the named section verbatim. `arc`
never parses, validates, or rewrites them, and `arc obj hexdump --section` gets
the same bytes back. Repeatable. This is the whole of the "no `-g`" story.

`--link` is gone. `arc link` accepts `.s` directly.

---

## Link

```
arc link [flags] <file.{s,o}>...

  -o, --output <path>
      --entry <symbol>    default: _start (ELF), _main (Mach-O),
                          mainCRTStartup (PE)
      --image <format>    exe (default) | flat
      --base <addr>       load address; required for flat
      --debug-section <name>=<path>
```

```
arc link -o main main.s                  # assembles on the way in
arc link -o main a.o b.o
arc link -o boot.bin --image flat --base 0x7c00 boot.s
```

Inputs must agree on arch and ABI. Disagreement is a diagnostic naming both
files and both values; there is no `--force`, because the psABI does not define
a mixed image.

`--image flat` writes pre-resolved bytes and has no `linker/` counterpart in the
tree — the flag routes to `objectfile/flat`, and `--base` is mandatory there
because nothing is left to relocate. Debug payloads do not survive `--image
flat`; on `exe` they pass through untouched.

---

## Run

```
arc run hello.s
arc run hello.s -- --flag arg
```

Assemble, link to a temp path, exec, forward the exit code. Everything after
`--` goes to the program.

Refuses when the resolved target is not the host, with the target named. Cross
`run` would mean an emulator, and emulators have vendors.

---

## Fmt

The round-trip guarantee, exposed.

```
arc fmt [flags] <path>...

  -w, --write             rewrite in place
      --check             exit 4 if any file would change; print names
      --dialect <d>       print in this dialect
      --diff              unified diff to stdout
```

```
arc fmt -w main.s
arc fmt --dialect nasm att.s           # translate
arc fmt --check .                      # CI gate; directories walk recursively
```

Directory arguments are walked recursively for `.s`. There is no `./...` — that
is Go's package pattern, `arc` has no packages, and borrowing the spelling would
invent a second meaning for a token that already has one.

`fmt` is `parse` then `print`, which are inverses at the semantic level. Anything
`fmt` changes assembles to identical bytes, and the differential suite against
GNU `as` and NASM is the proof. This is the only formatter for assembly that can
make that claim without a caveat, and it comes for free from a test that already
had to exist.

`--dialect` on `fmt` is dialect translation, bounded by the reference
implementations: `gas` is GNU `as`, `nasm` is NASM. Round-tripping MASM or HLASM
through `arc` is not a supported operation, and neither is a dialect value.

---

## Enc

```
$ arc enc 'mov rax, 60'
48 c7 c0 3c 00 00 00

$ arc enc --dialect gas 'movq $60, %rax'
48 c7 c0 3c 00 00 00

$ arc enc -t aarch64-elf 'mov x0, #60'
80 07 80 d2
```

Multiple instructions, one per argument or one per line on stdin. `--json` emits
`{bytes, length, form, features}` per instruction.

`--all` shows every legal encoding of the same instruction, which the tables
already know and no other assembler will tell you:

```
$ arc enc --all 'add rax, 1'
48 83 c0 01           ADD r/m64, imm8      4 bytes
48 05 01 00 00 00     ADD RAX, imm32       6 bytes   ; eAX short form
```

---

## Dis

```
$ arc dis 48c7c03c000000
;; x86_64 · gas · host default target
mov rax, 60

$ arc dis -t aarch64-elf d2800780
;; aarch64
mov x0, #60

$ arc dis main.o                    # symbols, relocations, section boundaries
$ arc dis --section .text main.o
$ arc dis --sym _start main.o
```

Hex input is accepted bare, spaced, comma-separated, `0x`-prefixed, or as a C
array. Requiring one exact spelling is the single most-complained-about thing
about `llvm-mc` and costs nothing to fix.

Bare hex carries no arch, so it uses the resolved target — and unlike everywhere
else in the tool, a wrong target here produces plausible wrong output instead of
an error. So `dis` prints the target it used as a leading comment whenever the
input is bare bytes, and says `host default target` when nothing selected it.
Object and image inputs carry their own arch and print no such line; passing
`-t` that contradicts the file is a diagnostic.

Relocation sites in an object are annotated inline with the psABI constant name,
not a number.

---

## Explain

One instruction, every field named. Three renderings, chosen by the encoding
rather than a flag — bytewise rows for variable-length x86, a bit ruler for
fixed-width 32-bit, a 16-bit ruler for RVC and Thumb. There is no `--format`.

```
$ arc explain 48 c7 c0 3c 00 00 00
mov rax, 60                                    x86_64 · base · 7 bytes

  48         REX      W=1 R=0 X=0 B=0          64-bit operand size
  c7         opcode   MOV r/m64, imm32         /0
  c0         ModRM    mod=11 reg=000 rm=000    → rax
  3c000000   imm32    60                       0x3c

$ arc explain -t aarch64-elf d2800780
mov x0, #60                                    aarch64 · base · 4 bytes

  1 10 100101 00 0000000000111100 00000
  │ │  │      │  │                └── Rd    = 0    → x0
  │ │  │      │  └─────────────────── imm16 = 60
  │ │  │      └────────────────────── hw    = 0    → LSL #0
  │ │  └───────────────────────────── op    = MOVZ
  │ └──────────────────────────────── opc   = 10
  └────────────────────────────────── sf    = 1    → 64-bit
```

`--json` gives the same decomposition for all three, as a structure with byte
and bit offsets.

This is the answer to "why is my `mov` seven bytes," and it exists because the
tables are generated rather than hand-written — the field names are already in
them.

---

## Obj

Noun-verb, because there are several unrelated views of one file. Accepts
relocatable objects and linked images; each view prints what the file actually
is, by the format's own name.

```
arc obj <view> <file>

  header      format, arch, abi, entry, flags
  sections    name, size, offset, alignment, flags
  syms        name, binding, section, value, size
  relocs      offset, kind, symbol, addend
  hexdump     raw bytes; --section to narrow
```

```
$ arc obj syms main.o
$ arc obj relocs --section .text main.o
$ arc obj header main
FORMAT   PE32+
ARCH     x86_64
ENTRY    mainCRTStartup    0x140001000
```

`header` says `ELF64`, `Mach-O 64`, `COFF`, or `PE32+` — never a flattened
"object." The tree is scrupulous about `objectfile/` versus `linker/` and COFF
versus PE32+, and the CLI does not get to be looser than the tree.

`relocs` prints `Addend` as the format means it: written to `r_addend` for ELF,
patched into `Code` with zero on disk for COFF and Mach-O. The column header
says which. A single spelling would flatten that into a footnote, same reason
the format packages don't share a `Reloc` type.

---

## Query

Same depth for same-tier commands. All five take `--json`.

### isa

```
$ arc isa mov
MOV r/m64, r64        REX.W 89 /r      base
MOV r64, r/m64        REX.W 8b /r      base
MOV r64, imm64        REX.W b8+r io    base
...                                    (24 forms, --all to list)

$ arc isa vpaddd --features avx512f
$ arc isa -t riscv64-elf vsetvli
```

Forms are filtered by the resolved feature set unless `--all`. An instruction
that exists but is gated prints with the flag that would enable it.

### regs

Register names are not unique across arches — `r3` is a GPR on `arm` and
`powerpc64le`, `a0` is an ABI name on both `riscv64` and `loongarch64`. So the
arch must be resolvable, and an ambiguous bare name is a diagnostic:

```
$ arc regs r3
error: ambiguous register name "r3"
  note: defined by arm, powerpc64le; use --arch

$ arc regs --arch riscv64 a0
a0    64  gp   x10   arg0/ret0    caller-saved

$ arc regs --arch loongarch64 v0
$a0   64  gp   $r4   arg0/ret0    caller-saved
  note: "$v0" is a deprecated alias of $a0

$ arc regs eax
eax   32  gp   parent rax   overlaps ax al ah
```

Three relations, printed differently because they are different: `eax`/`rax` are
two widths of one register, `a0`/`x10` are two names for one register, and
`$v0` is a name the psABI has retired. The `reg` package already distinguishes
them; the CLI should not collapse them.

### targets

The matrix, printed above under Target selection. `arc targets <arch>` narrows
to one row and expands the feature list.

### env

```
$ arc env
ARC_TARGET=x86_64-elf
ARC_ABI=
ARC_DIALECT=gas
ARC_FEATURES=x86-64-v1

$ arc env ARC_TARGET
x86_64-elf
```

Precedence: flag, then environment, then host default. Three levels, no fourth.
An empty value means the axis has no choice on this target.

There is no config file. A config file makes the same command mean different
things in different directories, which is the property `arc` is built to not
have. If you need repeatable flags, you already have a Makefile.

### version

```
$ arc version
arc 0.4.1 (a3f9c21, go1.24.2)
```

Version, commit, and toolchain, on one line. `--json` adds the table generation
stamp, which is the number that actually matters when two builds disagree about
an encoding.

---

## `arc --help`

```
arc — assembler, linker, and encoder for real silicon

USAGE
  arc <command> [flags] [args]

EXAMPLES
  arc build -o main.o main.s              assemble
  arc link -o main main.s                 assemble and link
  arc run hello.s                         assemble, link, execute
  arc enc 'mov rax, 60'                   → 48 c7 c0 3c 00 00 00
  arc dis 48c7c03c000000                  → mov rax, 60
  arc explain 48c7c03c000000              field-by-field breakdown
  arc fmt --dialect nasm att.s            translate dialect

BUILD
  build       assemble to an object file
  link        link objects into an image
  run         assemble, link, and execute
  fmt         normalize or translate assembly

INSPECT
  enc         encode instructions given on the command line
  dis         decode bytes, objects, or images
  explain     break one encoding into its fields
  obj         inspect an object file or image

QUERY
  isa         look up instruction forms
  regs        look up registers
  targets     print the target matrix
  env         show resolved configuration
  version     print version
  completion  emit a shell completion script

TARGET
  -t, --target <arch>-<platform>   default: host (x86_64-elf)
      --abi <name>                 default: psABI default for the target
      --dialect <gas|nasm>         x86_64 and i386 only; default gas
      --features <set>             default: psABI baseline

GLOBAL
      --json          machine-readable output
      --color <when>  auto (default), always, never
  -v, --verbose
  -q, --quiet
  -h, --help

  arc help <command> for detail.  arc targets for the target matrix.
  https://github.com/you/arc
```