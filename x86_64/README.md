# x86_64

The AMD64 arch package: registers, ISA tables, encoder, decoder, text layer,
generated helpers, and the assembler that ties them to an object file.

## Layout

```
x86_64/
├── target.go            Platform, Dialect, Baseline, New, Option
├── error.go              Error, ErrFeature/ErrForm/ErrReloc/ErrUndefined/ErrPlatform
├── asm.go                 Assembler, Section, Align, Label, data emission, Emit
├── code.go                 Encode, Forms, Decode, Explain — forwards to isa/encode/decode
├── text.go                  ParseFile, ParseInst, Print, PrintInst — GAS/NASM dispatch
├── assemble.go               Assemble — text.Unit → object bytes, arc build's whole job
├── feature.go                 re-export of feature/: FeatureSet, Level, V1…V4, ParseFeatures
├── operand.go                  re-export of reg/ and operand/: registers, Imm, M8…M512, RIPRel, Label, Ref
│
├── helpers_base_gen.go          one method per form, generated from isa/
├── helpers_sse_gen.go
├── helpers_avx_gen.go
├── helpers_avx512_gen.go
├── helpers_amx_gen.go
│
├── reloc.go                      RelocKind validity table, RelocName/RelocPlatform/RelocSize
├── reloc_elf.go                   R_X86_64_*
├── reloc_coff.go                   IMAGE_REL_AMD64_*
├── reloc_macho.go                   X86_64_RELOC_*
├── write.go                          Serialize, WriteTo, fixup resolution, symbol-size closing
├── write_elf.go                       ELF platform writer
├── write_coff.go                       COFF platform writer
├── write_macho.go                       Mach-O platform writer
├── write_flat.go                         Flat platform writer — concatenation, no relocations
│
├── feature/               Level, Feature, Set — the extension vocabulary and gating
├── reg/                    R8, R16, R32, R64, Sreg, St, Mm, Xmm, Ymm, Zmm, K, Tmm, Cr, Dr
├── operand/                Imm, M8…M512, RIPRel, Label, SymRef — the operand set
├── isa/                    Form, Class, Slot — the instruction table
├── encode/                 form + operands → bytes + fixups
├── decode/                 bytes → form + operands, and the Explain field breakdown
└── text/
    ├── unit.go, node.go, expr.go, inst.go, operand.go, directive.go, pos.go   dialect-neutral tree
    ├── gas/                GNU as syntax: lex.go, parse.go, print.go, directive.go, expr.go, mnemonic.go
    └── nasm/                NASM syntax: lex.go, parse.go, print.go, directive.go, expr.go
```

## Subpackages

- **feature/** — the microarchitecture levels (`V1` through `V4`) and the
  orthogonal extensions they are built from (`MMX` through `AVX512VBMI2`,
  `AMXTILE`, `AMXINT8`, `AMXBF16`), each closed under its own requirements.
  `Baseline` is `V1` — x86-64-v1, which is SSE2 and nothing above it.
- **reg/** — every physical register this target has, at every width, with
  `Num`, `Bits`, `Class`, `Save`, `DWARF`, `Parent`, and `Overlaps`. This is
  where `R8`…`R15`, the REX-only byte registers `SPL`/`BPL`/`SIL`/`DIL`, the
  mask registers `K0`…`K7`, and the AMX tiles `TMM0`…`TMM7` live.
- **operand/** — the operand set: `Imm`, the memory types `M8` through `M512`
  with base/index/scale/displacement/segment builders, `RIPRel`, `Label`, and
  `SymRef`.
- **isa/** — the form table: every declared encoding of every mnemonic, its
  operand classes, its opcode bytes, and the level or feature that gates it.
  `Forms`, `Enabled`, and `Resolve` are the query surface, and the generator
  that writes `helpers_*_gen.go` reads nothing else.
- **encode/** — a pure function from a resolved `*isa.Form` and operand values
  to bytes and fixups: REX/VEX/EVEX construction, ModRM/SIB, displacement
  sizing, immediate width selection. Also holds `Nops`, the multi-byte no-op
  tables `Align` pads a code section with.
- **decode/** — the inverse machine: the three opcode maps built from
  `isa.All()`, prefix disambiguation, VEX/EVEX unwinding, ModRM/SIB
  disassembly, and `Explain`'s field-by-field breakdown.
- **text/** — the dialect-neutral tree (`Unit`, `Node`, `Expr`, `Inst`) both
  syntaxes parse to and print from, plus the expression evaluator and directive
  semantics that are the arch's rather than the syntax's.
- **text/gas/** — GNU as (AT&T syntax): operand order reversed at the edges,
  size as a mnemonic suffix, `@`-modifiers, `%rip`-relative spelling, gas's own
  bitwise-above-additive expression precedence.
- **text/nasm/** — NASM (Intel syntax): operand order stored as written, size as
  a `BYTE`/`WORD`/`DWORD`/`QWORD` keyword, `REL`/`WRT` modifiers, NASM's own
  C-like expression precedence.

## Platforms

```go
a := x86_64.New(x86_64.ELF)
a := x86_64.New(x86_64.COFF)
a := x86_64.New(x86_64.MachO)
a := x86_64.New(x86_64.Flat)
```

- **ELF** — full ET_REL relocatable objects via `objectfile/elf`,
  `R_X86_64_64`/`R_X86_64_PC32`/`R_X86_64_PLT32`/`R_X86_64_GOTPCREL` wired end
  to end. The wider `R_X86_64_*` set — `32`, `32S`, `REX_GOTPCRELX`, the TLSGD
  and TLSLD families — is declared in `reloc_elf.go` for completeness; anything
  past those four is refused at `Serialize` with a note naming the gap, not
  silently miscoded.
- **COFF** — full Win64 objects via `objectfile/coff`, targeting
  `IMAGE_FILE_MACHINE_AMD64`. Unlike `i386`, `write_coff.go` applies no name
  mangling: Win64 dropped the cdecl leading underscore, so a symbol goes into
  the table spelled the way you wrote it. Of the `IMAGE_REL_AMD64_*` kinds
  declared in `reloc_coff.go`, only `ADDR64`, `ADDR32`, `ADDR32NB`, `REL32` and
  `SECREL` have an `objectfile/coff` mapping today; the rest — including the
  `REL32_1` through `REL32_5` displaced variants — are refused at `Serialize`
  the same way an unmapped ELF relocation is.
- **Mach-O** — full `MH_OBJECT` files via `objectfile/macho`, targeting
  `CPU_TYPE_X86_64`. Section names are `segment,section` pairs and go through
  `SectionNamed` verbatim; `Text` maps to `__TEXT,__text` and the rest are the
  table in `objectfile/README.md`. `arc` does not add the leading underscore
  Mach-O tooling expects on C symbols — `_main` is a name you write, the same as
  it is in a `.s` file. `X86_64_RELOC_UNSIGNED`, `SIGNED`, `BRANCH`,
  `GOT` and `GOT_LOAD` are wired; `SUBTRACTOR` and the `SIGNED_1`/`_2`/`_4`
  displaced variants are declared and refused. Objects carry no
  `LC_BUILD_VERSION` — `arc` cannot invent a minimum macOS version you didn't
  state, and `a.MachO()` is where you set one.
- **Flat** — a raw concatenation of every section in creation order via
  `objectfile/flat`, with no header and no symbol table. `SetBaseAddress` sets
  the load address the image starts at. Flat binary has no relocation record and
  no linker to resolve one at load time, so a fixup that leaves its section — a
  reference to another section or to an external symbol — is refused at
  `Serialize`, naming the reference. A `%rip`-relative reference to a bare
  `Label` already defined in the same section resolves and survives; one to a
  `Ref` does not.

There is no ABI parameter — `x86_64.New` takes only a `Platform` and options.

## Features

```go
a := x86_64.New(x86_64.ELF, x86_64.WithFeatures(x86_64.V3))
a := x86_64.New(x86_64.ELF, x86_64.WithFeatures(x86_64.V1.Plus(x86_64.AVX512F)))

f, err := x86_64.ParseFeatures("x86-64-v2+avx512f")
```

The default is `Baseline()`, which is `V1`. Levels are shorthand for a closed
set of extensions, not a separate axis: `V3` is `V2` plus AVX, AVX2, BMI1, BMI2,
F16C, FMA, LZCNT and MOVBE, and it prints back as its canonical spelling.

Gating is at encode time, at the call, with the flag that would have allowed it:

```
.text+0x1c: vpaddd requires avx512f, not in the active feature set
  active: x86-64-v1
  note: x86_64.WithFeatures(x86_64.V1.Plus(x86_64.AVX512F))
```

Options are read at `New`. There is no `SetFeatures`; a feature set that changed
halfway through an object would make that diagnostic unfalsifiable.

## Dialects

```go
u, err := x86_64.ParseFile("main.s", src, x86_64.GAS)
u, err := x86_64.ParseFile("main.asm", src, x86_64.NASM)
out, err := x86_64.Print(u, x86_64.NASM)
```

Both parse to and print from the same `text.Unit`, so a file read in one dialect
can be formatted in the other.

## Assembling from source

```go
u, err := x86_64.ParseFile("main.s", src, x86_64.GAS)
if err != nil {
	// ...
}
b, err := x86_64.Assemble(u, x86_64.ELF, x86_64.DefaultFeatures())
```

`Assemble` is `arc build`'s whole job in one call: it walks `u` in source order,
places every statement into a fresh `Assembler`'s sections exactly as the
hand-built calls below would, and returns `Serialize`'s bytes — no separate
translation step for a caller to get wrong.

Instruction operands have no gap here: a symbolic call or `%rip`-relative
reference carries through to `Serialize` via the usual fixups, whether it came
from text or from the typed builder API. What `Assemble` does not yet do is fold
a `.quad` or `.fill` value that is not a plain constant — `.quad . - msg` and
similar section-relative or symbolic data need a fixup the same way an operand
does, and that backpatch path isn't wired up yet. Such an item is refused with a
specific error rather than silently miswritten.

## Registers and operands

```go
x86_64.RAX, x86_64.R11D, x86_64.SIL, x86_64.XMM0, x86_64.ZMM31, x86_64.K1, x86_64.CR0
x86_64.Mem64(x86_64.RBX).Disp(8)
x86_64.Mem64(x86_64.RBX).Index(x86_64.RCX, 4)
x86_64.RIPRel(x86_64.Ref("msg", x86_64.R_X86_64_PC32))
x86_64.Imm(60)
x86_64.Ref("puts", x86_64.R_X86_64_PLT32)
```

Re-exported from `reg/` and `operand/` so a caller never needs the second
import. `x86_64.EAX` is a view that zero-extends into `RAX` on write and answers
`Parent()` accordingly; `i386.EAX` is a full architectural register and answers
nothing. That difference is why the two packages are copies rather than one.

## Building an object by hand

`exit(60)` — hand-built with `Section`, `Label` and the generated helpers, no
text file involved:

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
	os.WriteFile("exit.o", b, 0o644)
}
```

One helper per form, named after the form the table spells: `MovR64Imm64` is
`MOV r64, imm64` and nothing else will quietly relax it to `MOV r/m64, imm32`.
A width mismatch is a compile error rather than a diagnostic:

```go
t.MovR64Imm64(x86_64.EAX, 60)
// cannot use x86_64.EAX (variable of type reg.R32) as reg.R64 value
```

The size of that layer is the size of the ISA — roughly twelve thousand methods
on `*Section` with AVX-512 and AMX enabled, split across `helpers_*_gen.go` by
feature. Godoc renders one alphabetized wall. That is the accepted price of "you
get the bytes you asked for."

`Emit` is the other layer, for code that builds operands at runtime and doesn't
know the form yet:

```go
t := a.Section(x86_64.Text)
t.Label("greet", x86_64.Global, x86_64.Func)
t.Emit("mov", x86_64.RAX, x86_64.Mem64(x86_64.RBX).Segment(x86_64.FS).Disp(8))
t.Emit("lea", x86_64.RSI, x86_64.RIPRel(x86_64.Ref("msg", x86_64.R_X86_64_PC32)))
t.Emit("call", x86_64.Ref("puts", x86_64.R_X86_64_PLT32))
t.Emit("ret")
```

`Emit` resolves the shortest legal encoding from `isa.Resolve` and
`encode.Encode` against `a.Features()`, breaking ties by table order. If you
care which encoding you get, that is what the typed layer is for.

No `Addend: -4` on the `call`. The displacement is four bytes ending the
instruction and the assembler knows that because it placed the field; you write
the logical addend and `write_elf.go` hands `objectfile/elf` the raw one.

Nothing survives either call.