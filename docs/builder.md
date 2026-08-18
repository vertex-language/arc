# Builder API

`arc` is a library with a CLI attached, not the other way round. Every rule in
[cli.md](cli.md) is a rule the library already enforces; `cmd/arc` only parses
flags, switches once on the resolved arch, and prints.

The test for anything proposed here: if it can't be reached from the CLI, or the
CLI can do it and this can't, one of the two is wrong.

---

## Shape

There is no builder package. There are nine, one per arch, and each is
complete:

```
x86_64/  i386/  aarch64/  arm/  riscv64/  riscv32/
powerpc64le/  s390x/  loongarch64/
```

Each holds its own registers, ISA tables, generated helpers, operand types,
relocation kinds, assembler, encoder, decoder, and text layer. The package name
is the canonical arch spelling — the psABI document name — so the top-level
directory listing and the first column of `arc targets` are the same nine
strings.

```go
import "github.com/vertex-language/arc/x86_64"
```

That import is the target declaration. Nothing else in your file names an arch,
because there is nothing else to name: `x86_64.RAX` has no interface broad
enough to accept it elsewhere, and `x86_64.Section` has no relation to
`aarch64.Section` beyond spelling.

**Nothing lives at the module root.** `github.com/vertex-language/arc` is a
module path, not an import — the thing that used to be package `arc` is what got
monomorphized, and the nine copies are the tree. Same rule
`linker/README.md` states about `linker/` itself, one level up.

Below the nine sits `objectfile/`, which each of them writes to; beside it sits
`linker/`, which consumes what they write. Neither imports the other, and no
arch package imports another arch package.

There is no `arch` package. Canonical spellings are the directory names; the
alias table maps a flag string to which of nine packages to call, which is
`cmd/arc`'s switch; `Host()` is `runtime.GOARCH` and `runtime.GOOS` into that
same switch. A library user who imported `x86_64` already made the selection.

---

## Repository layout

```
arc/
├── go.mod
├── README.md
├── docs/
│   ├── cli.md
│   └── builder.md
│
├── cmd/arc/
│   ├── main.go            verb dispatch
│   ├── target.go          -t --arch --platform --abi --dialect --features;
│   │                      the alias table, host default, the nine-case switch
│   ├── build.go  link.go  run.go  fmt.go
│   ├── enc.go  dis.go  explain.go  obj.go
│   └── isa.go  regs.go  targets.go  env.go  version.go  completion.go
│
├── x86_64/  i386/  aarch64/  arm/  riscv64/  riscv32/
├── powerpc64le/  s390x/  loongarch64/
│
├── objectfile/            relocatable objects — four independent format packages
│   ├── elf/  coff/  macho/  flat/
│
├── linker/                linked images — three independent linkers
│   ├── elf/  macho/  pe/
│
└── internal/
    ├── gen/               table generators — build-time only, imported by nothing
    └── difftest/          differential suite against GNU as and NASM
```

Directories are arches; files are tasks. Adding an arch is the recurring change
— five of nine are unlanded — and the verb list is closed, so the tree is cut on
the axis that moves and the verbs show up as file names.

`objectfile/` and `linker/` cut by format instead, because that is the axis
their contents vary along. Relocation patching varies by format first and arch
second, which is why `linker/elf/x86_64` is a subpackage and not a peer.

### Inside one arch package

```
x86_64/
├── target.go          Platforms ABIs Dialects Baseline
├── asm.go             New, Assembler, Section, Align, Label, Data, Emit
├── code.go            Encode Forms Decode Explain — thin, forwards
├── text.go            GAS/NASM consts, two-case dispatch — thin
├── feature.go         re-export: FeatureSet, ParseFeatures, V2/V3, level table
├── operand.go         re-export: RAX, Imm, M8…M512, Label, Ref
├── error.go           Error, ErrFeature ErrForm ErrReloc ErrUndefined ErrPlatform
│
├── helpers_base_gen.go
├── helpers_sse_gen.go
├── helpers_avx_gen.go
├── helpers_avx512_gen.go
├── helpers_amx_gen.go
│
├── reloc.go           Kind, platform validity table, field-width checks
├── reloc_elf.go       R_X86_64_*
├── reloc_coff.go      IMAGE_REL_AMD64_*
├── reloc_macho.go     X86_64_RELOC_*
│
├── write.go           Serialize, WriteTo, Assemble; fixup resolution, symbol
│                      closing, alignment nops, the platform switch
├── write_elf.go       elf.Machine literal; a.ELF()
├── write_coff.go      coff.Machine literal, DLLExport; a.COFF()
├── write_macho.go     macho.Machine literal, "seg,sect"; a.MachO()
├── write_flat.go      ErrReloc on any Ref; a.Flat()
│
├── feature/           Set, bit assignment, canonical order, parse
├── reg/               register file, widths, views, Lookup()
├── operand/           Operand, Imm, M8…M512, Label, Ref
├── isa/               forms, operand classes, feature gates, Forms()
├── encode/            REX/VEX/EVEX, ModRM/SIB, disp sizing, imm width, nops
├── decode/            opcode maps, prefix/VEX/EVEX walk, Decode, Explain
└── text/
    ├── unit.go  node.go  inst.go  directive.go  expr.go
    ├── gas/     lex.go parse.go print.go      ↔ GNU as
    └── nasm/    lex.go parse.go print.go      ↔ NASM
```

**The subpackage line is fixed by Go, not by taste.** A subpackage cannot
declare methods on a parent type, so `x86_64/isa` can never hold
`func (s *Section) MovR64Imm64(...)`. Anything that is a method on `Assembler`
or `Section` stays in the arch package permanently; anything that is a table or
a plain function can move. `feature/`, `reg/`, `operand/`, `isa/`, `encode/`,
and `decode/` are the second kind.

`feature/` is the sink: everything above may import it and it imports nothing,
which is what lets `isa/` express a gate as a typed `feature.Set` rather than an
opaque bitmask the parent has to interpret. That is why `arc isa vpaddd
--features avx512f` cannot drift from what `arc build` accepts — the filter and
the gate are the same type from the same package.

`decode/` earns its own package on merit rather than tidiness: an x86 decoder is
a separate machine from the encoder — three opcode maps, prefix disambiguation,
ModRM/SIB, VEX/EVEX unwinding — sharing only the form table, and it has no
mirror in the builder API at all. `encode/` is that argument's mirror image and
exists for the same reason: REX/VEX/EVEX construction, ModRM/SIB, displacement
sizing and immediate width selection are several thousand lines of pure
function over resolved forms, and nothing survives the call anyway. It takes a
form and operand values and returns bytes and fixups, so it never imports
`text/`.

`encode/` and `operand/` exist in `x86_64` and `i386` and nowhere else. On a
fixed-width arch, encoding is bitfield packing against the form table and stays
in `asm.go`; there is no separate machine to name. That asymmetry is the same
one that makes `aarch64/text` flat.

**The write path stays in the arch package.** `Serialize` is a method on
`*Assembler` and reads `Section`'s unexported byte buffer, fixup list, and
symbol offsets. Moving it into an `obj/` subpackage would mean exporting all
three, which turns "a section is a byte buffer and a fixup list" from an
implementation note into a public promise. It splits into files instead, one per
platform, and a package that has three platforms has three files.

Generated helpers split by feature into files, not packages: same `*Section`,
same flat namespace, one file per generator pass, and a diff on the AVX-512
table doesn't touch the base file.

Not every package has every entry. `aarch64/text/` is flat — `lex.go`,
`parse.go`, `print.go`, `unit.go` — with no children, because seven of nine
arches show a dash in the DIALECTS column and a set with one member doesn't need
names. `riscv64/` has no `write_coff.go`, no `write_macho.go`, and no matching
`reloc_*.go` for either. `arm/` adds `isa_state.go` for `.arm`/`.thumb` and
`abi.go` for `Tag_ABI_VFP_args`. Absent rather than empty.

### Duplication is the design

`i386` is not a build tag on `x86_64`. `riscv32` is not a flag on `riscv64`.
ModRM encoding appears twice, the ADRP helper appears in `aarch64` and nowhere
else, and `i386/text/gas` plus `i386/text/nasm` are full copies of x86_64's two
parsers.

This is the rule `linker/macho` already follows when `arm64`, `arm64e`, and
`arm64_32` each carry a verbatim `encodeADRP`, and the rule `linker/` follows
when `elf`, `macho`, and `pe` each carry their own bounds-checked `reader.go`.
Stated once for the tree: **a helper is copied into every package that needs it,
and a name is redeclared in every package that uses it.** Factoring one out
later is a change that has to be argued for, not a cleanup.

The `x86_64`/`i386` pair is the case that looks most like waste and is the
clearest justification:

```go
x86_64.EAX.Parent()     // RAX
i386.EAX.Parent()       // nothing — in 32-bit there is no wider register
```

`i386.EAX` is a full architectural register; `x86_64.EAX` is a view that
zero-extends into one on write. One type spelled `EAX` would answer `Parent()`
differently depending on which mode you were thinking about, which is the
definition of a name that has to split. Everything else the two share — opcode
bytes, ModRM layout, the GAS and NASM printers — is copied, and the copy costs a
diff neither can silently break the other with.

**The test is whether the copies can diverge in meaning.** They can here, which
is why they are copies. They cannot in the ELF header — `powerpc64le` and
`loongarch64` differ by `e_machine` and nothing else — which is why that is a
struct literal and not a ninth copy of a writer. Duplication is defended where
the answers differ, not where the constants do.

**The known costs, named rather than discovered:** four x86 lexers and four x86
printers across the pair, and a duplicated x86 decoder maintained twice. That is
the largest duplication in the tree and it is accepted.

### Import graph

```
cmd/arc ──┬──> x86_64 ──┬──> x86_64/{feature,reg,operand,isa,encode,decode}
          ├──> i386 ────┼──> x86_64/text{,/gas,/nasm}
          ├──> …        └──> objectfile/{elf,coff,macho,flat}
          └──> linker/{elf,macho,pe}
```

Seven rules, all checkable by a script and all worth a CI gate:

- No arch package imports another arch package. Nine independent leaves.
- No arch package imports `linker/`, and no `linker/` package imports an arch
  package. The linker consumes bytes, not builders.
- Nothing imports an arch package. The nine are leaves in both directions except
  toward `objectfile/`.
- `x86_64/text/gas` imports `x86_64/{text,isa,reg,operand}` and never `x86_64`
  itself. Generally: no arch subpackage imports its own arch root, which is what
  makes `feature.go` and `operand.go` re-export boundaries rather than
  conveniences.
- `cmd/arc` is the only thing that imports more than one arch package, behind
  one switch, one case per package.
- `objectfile/*` imports the standard library and nothing else.
- `grep -rn 'AMD64\|ARM64\|RISCV\|X86' objectfile/` returns only `EM_*`,
  `IMAGE_FILE_MACHINE_*`, `CPU_TYPE_*`, and `presets.go` — spec constants and
  preset data, never a type or a switch. The day it returns a `switch arch`, the
  format layer has grown teeth again.

`internal/gen` and `internal/difftest` do know all nine. `gen` runs at build
time, produces text, and is imported by nothing — a generated file has no idea
another arch exists, which is the property that matters. `difftest` is `_test`
only. If `gen` grew a shared `Form` type that the nine imported, the design
would be back where it started.

---

## Target selection

The arch half of the target is the import path, so `New` takes what is left:

| CLI | Library |
| --- | --- |
| `-t <arch>-<platform>` | the import, plus `New`'s platform argument |
| `--arch` | the import |
| `--platform` | `New`'s first argument |
| `--abi` | `New`'s second argument, where the arch has one |
| `--features` | `WithFeatures(…)` |
| `--dialect` | **not here** — see [Text](#text) |

```go
a := x86_64.New(x86_64.ELF)
a := x86_64.New(x86_64.MachO, x86_64.WithFeatures(x86_64.V3))
a := aarch64.New(aarch64.COFF)
a := riscv64.New(riscv64.ELF, riscv64.LP64D)
```

**Each package declares only the platforms it has.** `riscv64` declares
`riscv64.ELF` and `riscv64.Flat` and nothing else, so the matrix miss the CLI
reports as a diagnostic is not expressible in Go at all:

```go
riscv64.New(riscv64.MachO)
// undefined: riscv64.MachO
```

`New` therefore has nothing to fail on and returns one value. The matrix lives
in the nine `target.go` files as nine short lists; `arc targets` is `cmd/arc`
asking each package for `Platforms()`, `ABIs()`, `Dialects()`, and `Baseline()`
and printing the rows. It still cannot drift from what `arc build` accepts — it
is the encoder's own data — but the join happens in the one place already
allowed to import all nine.

The string-level error stays where strings are:

```
$ arc build -t riscv64-macho f.s
error: no such target: riscv64-macho
  note: riscv64 supports: elf
  note: macho supports: x86_64, aarch64
```

`cmd/arc` builds that sentence from the same `Platforms()` calls. It is a
diagnostic about a flag value, not a library error, because past the flag
boundary the invalid pair has no representation.

**ABI is a parameter where it exists and absent where it doesn't.**

```go
riscv64.New(riscv64.ELF, riscv64.LP64D)   // required
arm.New(arm.ELF, arm.HardFloat)           // required
x86_64.New(x86_64.ELF)                    // no such parameter
```

The dash column in `arc targets` and this signature are the same fact.
`ErrABI` is gone; `--abi lp64d` against `x86_64` is still a CLI usage error, and
the library version of it is that `x86_64.New` takes one argument.

The ABI value is not decorative: `abi.go` in the packages that have one turns it
into the `e_flags` bits the object header records —
`EF_RISCV_FLOAT_ABI_DOUBLE`, `EF_ARM_ABI_FLOAT_HARD` — which is what `arc obj
header` prints and what `arc link` compares when it refuses to merge objects
that disagree.

**Aliases resolve at one boundary and this isn't it.** `cmd/arc/target.go` holds
the whole alias table and each package's `ParseFeatures` holds its own. Past
them only canonical values exist, and there is no in-memory representation of
`amd64` to leak because there is no in-memory representation of an arch at all —
there is a package.

**Options are read at `New`.** The target is fixed before the first instruction
because the first instruction is encoded against it. There is no `SetFeatures`;
a feature set that changed halfway through an object would make the gating
diagnostics unfalsifiable.

---

## Sections and symbols

```go
t := a.Section(x86_64.Text)
r := a.Section(x86_64.ROData)
c := a.SectionNamed("__DATA,__objc_classlist")

t.Align(16)
```

Section kinds — `Text`, `Data`, `ROData`, `BSS`, `Unwind`, `InitArray`,
`FiniArray`, `TLS`, `Custom` — are declared by every arch package. They mean the
same thing in all nine and are still redeclared, because an import list that
names one package is worth more than a constant declared once. The per-format
name mapping is the table in `objectfile/README.md` and is not restated here.

`SectionNamed` passes its argument through verbatim, which makes it the one call
that is not portable across platforms — `.text.hot` is not `__TEXT,__text` and
`arc` will not pretend otherwise.

Sections are emitted in the order first created. Two calls with the same kind
return the same `*x86_64.Section`.

`Align` in a code section emits the arch's multi-byte nop sequence and in a data
section emits zeros. That table is per-arch and lives in `write.go`, because
padding `.text` with `0x00` produces a listing that disassembles into garbage.

### Labels and symbols are different things

```go
t.Label("loop")                                        // fixup target only
t.Label("_start", x86_64.Global, x86_64.Func)          // fixup target and symbol
t.Label("tbl", x86_64.Local, x86_64.Data)
t.Label("MyExport", x86_64.Global, x86_64.Func, x86_64.DLLExport)
```

A bare `Label` is a name for an offset — resolvable by branches in the same
section, present in no symbol table, the same thing GNU `as` does with `.L`
names minus the naming convention. Attach any attribute and it becomes a symbol,
with `Offset` at the current position and `Size` closed at the next symbol or
the end of the section, which is `.type`/`.size` pairing without the two
directives.

`DLLExport` is declared by `x86_64`, `i386`, `aarch64`, and `arm` — the four
with a COFF platform — and is an error on their ELF and Mach-O targets, because
`coff.Symbol.DLLExport` is COFF-only. The other five do not declare it at all.

### Data

```go
r.Byte(0x55, 0xAA)
r.Long(0xdeadbeef)
r.Quad(uint64(len(msg)))
r.Ascii("hello, silicon\n")
r.Asciz("hello, silicon")
r.Zero(64)
r.Bytes(precomputed)
```

`.byte`, `.long`, `.quad`, `.ascii`, `.asciz`, `.zero`. Every one already had a
name. All nine declare all seven — and `Word` is not among them, because it is
two bytes on x86 and four on ARM, AArch64, and RISC-V. The text layer accepts
`.word` and resolves it per arch; the Go API does not offer a name whose width
depends on which package you are in.

---

## Instructions

Two layers over one table, per package. They differ in when they check and in
who picks the encoding.

### Typed helpers — one form, exact bytes

```go
t.MovR64Imm64(x86_64.RAX, 60)          // 48 c7 c0 3c 00 00 00
t.XorRM64R64(x86_64.RDI, x86_64.RDI)
t.Syscall()
```

Generated from `isa/`. One helper per form, named after the form. Operand types
are the form's operand classes, so a width mismatch is a compile error and not a
diagnostic:

```go
t.MovR64Imm64(x86_64.EAX, 60)
// cannot use x86_64.EAX (variable of type reg.R32) as reg.R64 value
```

**The name is the form, spelled the way the table spells it.** Mnemonic, then
each operand class in order, `/` dropped:

| Form | Helper |
| --- | --- |
| `MOV r64, imm64` | `MovR64Imm64` |
| `MOV r/m64, r64` | `MovRM64R64` |
| `ADD r/m64, imm8` | `AddRM64Imm8` |
| `ADD RAX, imm32` | `AddRAXImm32` |

The obvious shorter scheme — `MovRI`, `MovRM` — is the `--dialect intel`
mistake. x86 has `MOV r64, imm64` and `MOV r/m64, imm32`, and both are "RI";
`ADD r/m64, imm8` and `ADD RAX, imm32` are both "add a constant." A name that
cannot distinguish two forms cannot be the name of a form. This is
`arc enc --all` made reachable from Go: the six-byte `ADD RAX, imm32` is
`t.AddRAXImm32`, and nothing will quietly relax it to the four-byte one.

`r/m` classes take an interface satisfied by the register and the memory type of
that width — `RM64` by `R64` and `M64` — so one helper covers both without
covering the wrong width. That interface lives in one package and its
implementations are a closed set of two, which is what keeps it a width rather
than an abstraction.

Vendor-documented aliases get helpers and are marked as aliases in the generated
docs. `aarch64.MovR64Imm16` exists because the ARM ARM says `MOV` is an alias of
`MOVZ`; it emits `MOVZ`, and `arc explain` names `MOVZ`. Aliases `arc` would
have to invent do not exist.

**The size of this layer is the size of the ISA, and that is the honest price.**
x86_64 with AVX-512 is roughly twelve thousand methods on `*Section`. Godoc
renders one alphabetized wall; autocomplete on `t.` is a wall too. No subpackage
can fix it, because these are methods on a parent type. The alternative —
generating helpers for base, SSE, and AVX only and routing AVX-512 through
`Emit` — is worse: it reintroduces a tier where some forms are reachable exactly
and some are not, which is the `MovRI` problem wearing a feature flag. Twelve
thousand methods, split across `helpers_*_gen.go` by feature, is the accepted
cost of "you get the bytes you asked for."

### `Emit` — table-driven selection

```go
t.Emit(x86_64.MOV, x86_64.RAX, x86_64.Imm(60))
t.Emit(x86_64.ADD, x86_64.RAX, x86_64.Imm(1))   // 48 83 c0 01 — shortest legal
t.Emit(x86_64.SYSCALL)
```

For code that builds operands at runtime and doesn't know the form yet. `Emit`
resolves the form from `isa/` — the same table the helpers were generated from —
choosing the shortest legal encoding and breaking ties by table order: fixed,
documented, part of the deterministic-output guarantee. If you care which
encoding you get, that is what the typed layer is for.

`x86_64.Operand` is the variadic's type, satisfied only by this package's
registers, `Imm`, `M8`…`M512`, `Label`, and `Ref`. Two properties keep it from
being an IR node. Its inhabitants are in one-to-one correspondence with operand
classes real x86-64 silicon accepts, so it cannot name an operation no encoding
exists for. And **nothing survives the call** — `Emit` resolves the form and
appends bytes before it returns, so there is no held representation to walk, no
pass, no lowering. A `*x86_64.Section` is a byte buffer and a fixup list.

No legal form, or no form legal under the active feature set, is an error at the
call and not at `Serialize`:

```go
t.Emit(x86_64.MOV, x86_64.RAX, x86_64.XMM0)
// .text+0x0: no form of MOV takes (r64, xmm)
//   note: arc isa mov
```

### ISA state

`arm` vs `thumb` is encoding state, not a dialect:

```go
t.SetISA(arm.Thumb)     // .thumb
t.SetISA(arm.ARM)       // .arm
```

`SetISA` is declared by `arm` and by nothing else, available only when
`arm.Thumb` is in the feature set. It changes bytes, which is why it exists at
all and why no printer takes it.

---

## Operands

```go
x86_64.Imm(60)
x86_64.RAX
x86_64.M64(x86_64.RAX).Disp(8)               // 8(%rax)
x86_64.M64(x86_64.RAX).Index(x86_64.RCX, 4)  // (%rax,%rcx,4)
x86_64.RIPRel(ref)                            // msg(%rip)
x86_64.Label("loop")
ref                                           // symbol reference — see Relocations
```

The memory type carries its access width — `M8` through `M512`,
`aarch64.Mem64(aarch64.X1).PreIndex(16)` — because the width is what makes an
operand ambiguous in text and there is no reason to reintroduce the ambiguity in
Go. Size disambiguation being recoverable is the same property that makes `fmt`
a round trip.

On `x86_64` and `i386` these types are declared in `operand/` and re-exported by
`operand.go`, so `encode/` can take them without importing its parent.
`x86_64.RAX` and `x86_64/operand.RAX` are the same value under two spellings and
callers never need the second import. The seven fixed-width arches declare them
in `operand.go` directly, because there is no `encode/` below to serve.

`Label` resolves within one section, at `Serialize`, as a direct fixup with no
relocation record. Anything crossing a section boundary or leaving the object is
a relocation and must say so; there is no case where `arc` guesses which one you
meant. An unresolved label names itself:

```
arc: .text: undefined label "retry"
```

---

## Relocations

Explicit, and named by the psABI constant:

```go
t.CallRel32(x86_64.Ref("puts", x86_64.R_X86_64_PLT32))
t.MovR64M64(x86_64.RAX,
    x86_64.RIPRel(x86_64.Ref("errno", x86_64.R_X86_64_GOTPCREL)))

t.AdrpR64Imm21(aarch64.X0,
    aarch64.Ref("msg", aarch64.R_AARCH64_ADR_PREL_PG_HI21))
t.AddR64R64Imm12(aarch64.X0, aarch64.X0,
    aarch64.Ref("msg", aarch64.R_AARCH64_ADD_ABS_LO12_NC))
```

**The names are the psABI's, underscores included.** Go style says
`RX8664PLT32`. The System V AMD64 psABI says `R_X86_64_PLT32`, and that is the
string in the specification, in `readelf` output, in `arc dis`, and in the
comment on the line you are debugging. `grep -r R_X86_64_PLT32` should find your
source. The constants are typed, so the spelling costs nothing but a lint
suppression.

The package qualifier is now redundant with the constant's own prefix —
`x86_64.R_X86_64_PLT32` says x86-64 twice — and that is accepted rather than
shortened, because trimming to `x86_64.R_PLT32` would break the grep the rule
exists to protect.

**The full psABI set is declared, not a portable subset.** `reloc_elf.go` holds
every ELF relocation the arch defines — including `R_X86_64_32S`,
`R_X86_64_REX_GOTPCRELX`, the TLSLD family, and on RISC-V the `PCREL_HI20` /
`PCREL_LO12_I` pair and `R_RISCV_RELAX`. Each is a number the psABI assigned,
and the arch hands `objectfile` that number rather than selecting from a
cross-arch enum. A portable enum would be the union of nine psABIs living in a
package that is supposed to know none of them, and it would be the one place a
format package named an arch.

All three formats' spellings live in the arch package that has those platforms,
and using one against the wrong platform is caught at the call:

```go
// a := x86_64.New(x86_64.ELF)
t.CallRel32(x86_64.Ref("puts", x86_64.IMAGE_REL_AMD64_REL32))
// .text+0x0: IMAGE_REL_AMD64_REL32 is a COFF relocation; target is x86_64-elf
//   note: ELF spelling is R_X86_64_PLT32
```

A relocation kind the field can't hold is likewise an error, not a truncation.

**Addends are logical.** `x86_64.Ref("buf", kind).Plus(16)` means `buf+16`. The
field-position correction — the `-4` you write by hand against `objectfile/elf`
because a rel32 displacement is relative to the end of the instruction — is
computed by the assembler, which knows where the field sits because it just
placed it. It hands `objectfile` the raw addend that layer expects; ELF writes
it to `r_addend`, COFF and Mach-O patch it into `Code`, and the difference stays
where `objectfile/README.md` documents it.

---

## Errors

One sticky error per section, `errors.Is`-able, positioned. Each package
declares its own `Error` and its own sentinels; they are not comparable across
packages and nothing in the tree compares them.

```go
t.MovR64Imm64(x86_64.RAX, 60)
if err := t.Err(); err != nil { … }

b, err := a.Serialize()     // first error from any section
```

```go
type Error struct {
    Section string        // ".text"
    Offset  uint32        // where it would have gone
    Op      string        // "vsetvli"
    Missing FeatureSet    // non-empty for ErrFeature
    Active  FeatureSet
    Err     error         // ErrFeature, ErrForm, ErrReloc,
}                         // ErrUndefined, ErrPlatform
```

`ErrTarget` and `ErrABI` are gone: an invalid target is now a compile error, and
an ABI that doesn't apply is a parameter that doesn't exist.

Codegen is a long run of calls that should not each be followed by three lines
of error handling, and every error here is a programming error rather than a
runtime condition — so the first is retained, subsequent emits are dropped, and
it surfaces at `Err()` or `Serialize()`. Nothing is silently encoded after a
failure.

```
.text+0x1c: vsetvli requires v, not in the active feature set
  active: rv64imafdc_zicsr_zifencei
  note: riscv64.WithFeatures(riscv64.V)
```

The CLI's `file:line:col` version is this sentence with the position the parser
already has. Feature sets print in canonical order here for the same reason
`arc env` does: one spelling out, many in.

---

## Output

```go
b, err := a.Serialize()
n, err := a.WriteTo(w)
```

`Serialize` is deterministic. Identical calls produce identical bytes; COFF
`TimeDateStamp` is zero unless `SOURCE_DATE_EPOCH` is set, in which case it is
clamped to it. There is no mode in which it isn't.

Internally this is `write.go` plus one file per platform. `write.go` resolves
fixups, closes symbol sizes, pads with the arch's nops, and switches on the
platform; each `write_*.go` builds that format's own `Section` values and
declares the typed handle.

**What the arch hands `objectfile` is scalars, not a switch.** The format
packages know no arch names, so the whole of the per-arch header story is a
struct literal:

```go
// x86_64/write_elf.go
var elfMachine = elf.Machine{EM: elf.EM_X86_64, Class: elf.Class64, Data: elf.LSB}

// s390x/write_elf.go
var elfMachine = elf.Machine{EM: elf.EM_S390, Class: elf.Class64, Data: elf.MSB}

// riscv64/write_elf.go
func elfMachine(abi ABI) elf.Machine {
    return elf.Machine{EM: elf.EM_RISCV, Class: elf.Class64,
        Data: elf.LSB, Flags: abi.eflags()}
}
```

`EM_X86_64` is the ELF specification's own name for an ELF header field, which
is the same rule that keeps `R_X86_64_PLT32` unshortened — a spec constant is
not `arc` naming an arch. `elf.Arch`, `elf.OS`, and `elf.Target` do not exist,
because `arc` does not know what Linux is and a format package has no business
holding a second spelling of an arch below the boundary where the alias table
resolved the first. Endianness is a field for the same reason it is in the arch
name: `s390x` is big-endian and that is one constant, not a fork of the writer.

Section kinds are pinned to the same values on both sides and translate by cast,
with one round-trip test per format standing in for the switch that would
otherwise be copied fifteen times.

**The builder targets freestanding.** The ELF objects it writes are
`OSABI_None`; Mach-O objects carry no `LC_BUILD_VERSION`. `arc` does not know
what Linux is and cannot invent a minimum macOS version you didn't state. Where
you need those fields, take the typed handle:

```go
f, err := a.ELF()               // *elf.File; ErrPlatform if the target isn't ELF
f.SetOSABI(elf.OSABI_FreeBSD)
b, err := f.Serialize()
```

`a.ELF`, `a.COFF`, `a.MachO`, `a.Flat` are the escape hatch to the format layer,
and each package declares only the ones its platform list contains — `riscv64`
has `ELF` and `Flat` and no `MachO` method to call. This is the reason the
assembler does not grow a `SetOSABI` of its own: one knob, one owner.

Images are `linker/`'s job. `--image flat` routes to `objectfile/flat`, which is
why a flat image is `a.Flat()` and not a link.

---

## Text

`parse` and `print` are inverses, and they live under the arch package in
`text/`.

```go
import (
    "github.com/vertex-language/arc/x86_64"
    "github.com/vertex-language/arc/x86_64/text"
    "github.com/vertex-language/arc/x86_64/text/gas"
    "github.com/vertex-language/arc/x86_64/text/nasm"
)

u, err := gas.ParseFile("main.s", src)      // *text.Unit
b, err := x86_64.Assemble(u, x86_64.ELF, feat)
out, err := nasm.Print(u)                    // arc fmt --dialect nasm
```

**A dialect is a directory.** `x86_64/text/gas` and `x86_64/text/nasm` are the
two, named canonically for the reason cli.md gives — `--dialect intel` is a lie
of omission, since FASM, MASM, NASM, TASM, and YASM all claim Intel syntax.
There is no `att/` directory; `att` and `intel` are aliases resolved in
`cmd/arc/target.go` and nowhere else.

`text.Unit` is what a `.s` file denotes: the same sections, symbols, and
instructions the typed helpers produce by hand. Both dialects parse *to* it and
print *from* it, which is why it lives in `text/` and not in either
subdirectory. `arc build` is `gas.ParseFile` then `Assemble`; `arc fmt` is
`gas.ParseFile` then `nasm.Print`; that the two paths meet in one type is what
makes the round trip a property of the code rather than a claim in a README.

`text/` also owns directive semantics, because they are not portable: `.word` is
two bytes here and four in `arm/text`. It owns the expression evaluator too —
`msg - . + 4`, `(end-start)/8`, `.long 1+2*3`. Both dialects have an expression
grammar and it is the same arithmetic over the same tree, so `expr.go` sits
beside `directive.go` and the two surface syntaxes parse into it.

**Seven arches have no subdirectories.** `aarch64/text` is flat — one grammar,
one lexer, one printer:

```go
u, err := text.ParseFile("main.s", src)
out, err := text.Print(u)
```

No dialect argument, so no `ErrDialect`; the CLI's `--dialect does not apply to
riscv64` is a usage error about a flag, and below it there is no parameter to
pass a bad value to. That asymmetry is the DIALECTS column of `arc targets`, and
the reason for it is cli.md's: a set with one member doesn't need names. It also
disposes of MASM and HLASM visibly — they are excluded as languages, and now
that is a directory that isn't there.

Each dialect directory has exactly one reference implementation, which is what
bounds the round-trip claim: `internal/difftest/gas` tests `text/gas` against
GNU `as`, `internal/difftest/nasm` tests `text/nasm` against NASM. The guarantee
is per-directory and provable per-directory.

Single instructions, for `arc enc` / `arc dis` / `arc explain`:

```go
inst, err := gas.ParseInst("movq $60, %rax")
b, form, err := x86_64.Encode(feat, inst)
all := x86_64.Forms(feat, inst)              // arc enc --all
inst, n, err := x86_64.Decode(bytes)         // arc dis
fields, err := x86_64.Explain(bytes)         // arc explain
out := nasm.PrintInst(inst)
```

`arc dis` is `Decode` then `PrintInst`, and that split is the enforcement of "a
dialect is a spelling, never a byte": `decode/` has no dialect notion at all,
and the only thing that takes one returns text. The same split holds upward:
`code.go` resolves a `text.Inst` to a form and operand values before calling
`encode/`, so the encoder never sees text either.

`Explain` returns the field decomposition as data — byte and bit offsets, field
names, decoded values. The three renderings in `arc explain` are three printers
over this structure, chosen by encoding width, which is why there is no
`--format`.

**`Assemble` is a function, not a method.** `text` cannot import its parent —
the parent imports it — and the serialize path is shared with the builder's
`Assembler`, so it lives in `write.go` and takes a `*text.Unit`. The alternative
is duplicating serialization into `text/`, which is the one place in this tree
where duplication would be the wrong call.

---

## Query

The tables, directly. `arc isa`, `arc regs`, and `arc targets` are printers over
these and hold no data of their own — that is the whole of "`arc isa` cannot
describe an instruction `arc build` won't encode."

```go
x86_64.Platforms()                  // ELF, MachO, COFF, Flat
x86_64.Dialects()                   // GAS, NASM
x86_64.Baseline()                   // x86-64-v1
isa.Forms("mov")                    // every form, with feature gates
reg.Lookup("eax")                   // → R32, parent RAX, overlaps AX AL AH
riscv64reg.Lookup("a0")             // → x10, gp, arg0/ret0, caller-saved
```

There is no `LookupAny`. `arc regs r3` is ambiguous because `arm/reg.Lookup` and
`powerpc64le/reg.Lookup` both succeed, and the place that knows both answers is
`cmd/arc`, which called both:

```
$ arc regs r3
error: ambiguous register name "r3"
  note: defined by arm, powerpc64le; use --arch
```

That switch — nine cases, one per package — is the only cross-arch code in the
tree, and it exists at the boundary where the arch arrives as a string.

---

## Registers

Physical, with sub-register views first class:

```go
x86_64.RAX.Bits()                 // 64
x86_64.EAX.Parent()               // RAX
x86_64.EAX.Overlaps(x86_64.AL)    // true
x86_64.EAX.Overlaps(x86_64.BL)    // false
```

Three relations, three answers, matching `arc regs`: `EAX` and `RAX` are two
widths of one register, `a0` and `x10` are two names for one register, and `$v0`
is a name the psABI retired. Each package's `reg/` carries which of the three it
is; the API does not flatten them into "alias" any more than the CLI does.

Register types are per width and per class — `R64`, `R32`, `Xmm`, `aarch64.X`,
`aarch64.V` — which is what lets a generated helper signature reject a
wrong-width operand at compile time without a validator. The types live in
`reg/` and are re-exported from the arch package, so `x86_64.RAX` and
`x86_64/reg.RAX` are the same value under two spellings and callers never need
the second import.

---

## Examples

### exit(60) — x86_64, ELF

```go
a := x86_64.New(x86_64.ELF)

t := a.Section(x86_64.Text)
t.Align(16)
t.Label("_start", x86_64.Global, x86_64.Func)
t.MovR64Imm64(x86_64.RAX, 60)
t.XorRM64R64(x86_64.RDI, x86_64.RDI)
t.Syscall()

b, err := a.Serialize()
```

### Calling an external symbol — x86_64, ELF

```go
t.Label("greet", x86_64.Global, x86_64.Func)
t.CallRel32(x86_64.Ref("puts", x86_64.R_X86_64_PLT32))
t.Ret()
```

No `Addend: -4`. The `call` is four bytes of displacement ending the
instruction, and the assembler knows that because it placed the field.

### Loading a string — aarch64, Mach-O

```go
a := aarch64.New(aarch64.MachO)

r := a.Section(aarch64.ROData)
r.Label("msg", aarch64.Local, aarch64.Data)
r.Ascii("hello, silicon\n")

t := a.Section(aarch64.Text)
t.Label("_main", aarch64.Global, aarch64.Func)
t.AdrpR64Imm21(aarch64.X0,
    aarch64.Ref("msg", aarch64.R_AARCH64_ADR_PREL_PG_HI21))
t.AddR64R64Imm12(aarch64.X0, aarch64.X0,
    aarch64.Ref("msg", aarch64.R_AARCH64_ADD_ABS_LO12_NC))
t.Bl(aarch64.Ref("_puts", aarch64.R_AARCH64_CALL26))
t.Ret()
```

### A gated instruction — riscv64

```go
a := riscv64.New(riscv64.ELF, riscv64.LP64D)
t := a.Section(riscv64.Text)
t.Emit(riscv64.VSETVLI, riscv64.T0, riscv64.A0, riscv64.E8M1)

// .text+0x0: vsetvli requires v, not in the active feature set
//   active: rv64imafdc_zicsr_zifencei
//   note: riscv64.WithFeatures(riscv64.V)
```

### Big-endian — s390x, ELF

```go
a := s390x.New(s390x.ELF)

t := a.Section(s390x.Text)
t.Label("_start", s390x.Global, s390x.Func)
t.Emit(s390x.LGHI, s390x.R2, s390x.Imm(0))
t.Emit(s390x.BR, s390x.R14)

b, err := a.Serialize()
```

Nothing here says "big-endian." `s390x/write_elf.go` carries `Data: elf.MSB` and
`objectfile/elf` writes every header, symbol, and RELA record through it. The
instruction bytes were already big-endian because the encoder emitted them that
way, and `arc` has no `--endian` because endianness is in the arch name.

### Boot sector — flat

```go
a := i386.New(i386.Flat)
a.SetBaseAddress(0x7C00)

t := a.Section(i386.Text)
t.Align(1)
// … 510 bytes …
t.Byte(0x55, 0xAA)

b, _ := a.Serialize()
```

`i386.Ref` on a flat target is `ErrReloc`: flat binary forbids relocations as a
matter of format, which is why `flat.Section` has no `Relocs` field to begin
with. The assembler inherits the impossibility rather than re-checking it.

---

## What the arch packages are not

The exclusions are the library's, not the CLI's; the CLI just can't reach past
them.

- **Not a register allocator.** Every operand is a physical register. There is
  no virtual register type to pass.
- **Not an instruction selector.** `Emit` picks an *encoding* of the instruction
  you named. It never picks a different instruction, never folds, never
  strength-reduces, never reorders.
- **Not a macro expander.** There is no `.if`, no `.rept`, no `.macro`. Go is
  the macro language and it is better at it.
- **Not a layout engine.** Sections come out in creation order with the
  alignment you asked for. Address assignment is `linker/`'s.
- **Not an object-file writer.** Headers, string tables, symbol table encoding,
  relocation records, file offsets, and COMDAT emission are `objectfile/`'s. The
  arch contributes instruction bytes, resolved fixups, logical addends, and the
  psABI scalars the header records — and contributes them as data.
- **No cross-arch anything.** Not a shared `Reg`, not a shared `Section`, not a
  shared `Operand`, not a shared `Unit`, not a shared lexer, not a shared `Imm`.
  The nine do not import each other and nothing imports more than one of them
  except the switch in `cmd/arc`. A shared type would mean one name whose
  meaning depends on which arch you are currently thinking about, and a shared
  type that carried operands would be an IR — the thing this tree is defined by
  not having.