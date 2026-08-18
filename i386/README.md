# i386

The Intel386 arch package: registers, ISA tables, encoder, decoder, text layer, and the assembler that ties them to an object file.

## Layout

```
i386/
├── target.go            Platform, Dialect, Baseline, New, Option
├── error.go              Error, ErrFeature/ErrForm/ErrReloc/ErrUndefined/ErrPlatform
├── asm.go                 Assembler, Section, Align, Label, data emission, Emit
├── code.go                 Encode, Forms, Decode, Explain — forwards to isa/encode/decode
├── text.go                  ParseFile, ParseInst, Print, PrintInst — GAS/NASM dispatch
├── assemble.go                Assemble — text.Unit → object bytes, arc build's whole job
├── feature.go                  re-export of feature/: FeatureSet, Level, extensions, ParseFeatures
├── operand.go                   re-export of reg/ and operand/: registers, Imm, M8…M512, Label, Ref
├── reloc.go                      RelocKind validity table, RelocName/RelocPlatform/RelocSize
├── reloc_elf.go                   R_386_*
├── reloc_coff.go                   IMAGE_REL_I386_*
├── write.go                        Serialize, WriteTo, fixup resolution, symbol-size closing
├── write_elf.go                     ELF platform writer
├── write_coff.go                     COFF platform writer
├── write_flat.go                      Flat platform writer — concatenation, no relocations
│
├── feature/               Level, Feature, Set — the extension vocabulary and gating
├── reg/                    R8, R16, R32, Sreg, St, Mm, Xmm, Ymm, Zmm, K, Cr, Dr
├── operand/                Imm, M8…M512, Label, SymRef — the operand set
├── isa/                    Form, Class, Slot — the instruction table
├── encode/                 form + operands → bytes + fixups
├── decode/                 bytes → form + operands, and the Explain field breakdown
└── text/
    ├── unit.go, node.go, expr.go, inst.go, operand.go, directive.go, pos.go   dialect-neutral tree
    ├── gas/                GNU as syntax: lex.go, parse.go, print.go, directive.go, expr.go, mnemonic.go
    └── nasm/                NASM syntax: lex.go, parse.go, print.go, directive.go, expr.go
```

## Subpackages

- **feature/** — the base-CPU ladder (`I386`, `I486`, `I586`, `I686`) and the orthogonal extensions above it (`MMX` through `AVX512DQ`), each closed under its own requirements. `Baseline` is `I686`.
- **reg/** — every physical register this target has, at every width, with `Num`, `Bits`, `Class`, `Save`, `DWARF`, and `Overlaps`.
- **operand/** — the operand set: `Imm`, the memory types `M8` through `M512` with base/index/scale/displacement/segment builders, `Label`, and `SymRef`.
- **isa/** — the form table: every declared encoding of every mnemonic, its operand classes, its opcode bytes, and the level or feature that gates it. `Forms`, `Enabled`, and `Resolve` are the query surface.
- **encode/** — a pure function from a resolved `*isa.Form` and operand values to bytes and fixups. Also holds `Nops`, the multi-byte no-op tables `Align` pads a code section with.
- **decode/** — the inverse machine: opcode maps built from `isa.All()`, prefix and ModRM/SIB disassembly, and `Explain`'s field-by-field breakdown.
- **text/** — the dialect-neutral tree (`Unit`, `Node`, `Expr`, `Inst`) both syntaxes parse to and print from, plus the expression evaluator and directive semantics that are the arch's rather than the syntax's.
- **text/gas/** — GNU as (AT&T syntax): operand order reversed at the edges, size as a mnemonic suffix, `@`-modifiers, gas's own bitwise-above-additive expression precedence.
- **text/nasm/** — NASM (Intel syntax): operand order stored as written, size as a `BYTE`/`WORD`/`DWORD` keyword, `WRT` modifiers, NASM's own C-like expression precedence.

## Platforms

```go
a := i386.New(i386.ELF)
a := i386.New(i386.COFF)
a := i386.New(i386.Flat)
```

- **ELF** — full ET_REL relocatable objects via `objectfile/elf`, `R_386_32`/`R_386_PC32`/`R_386_GOT32` wired end to end. The wider `R_386_*` set is declared in `reloc_elf.go` for completeness; anything past those three is refused at `Serialize` with a note naming the gap, not silently miscoded.
- **COFF** — full Win32 objects via `objectfile/coff`, targeting `IMAGE_FILE_MACHINE_I386`. `write_coff.go` applies the cdecl leading-underscore mangling Win32 expects on every defined and referenced symbol, since `objectfile/coff` itself never mangles names. Of the `IMAGE_REL_I386_*` kinds declared in `reloc_coff.go`, only `DIR32`, `DIR32NB`, `REL32` and `SECREL` have an `objectfile/coff` mapping today; the rest are refused at `Serialize` the same way an unmapped ELF relocation is.
- **Flat** — a raw concatenation of every section in creation order via `objectfile/flat`, with no header and no symbol table. `SetBaseAddress` sets the load address a boot sector or similar image starts at — the `0x7C00` in the example below. Flat binary has no relocation record and no linker to resolve one at load time, so a fixup that leaves its section — a reference to another section or to an external symbol — is refused at `Serialize`, naming the reference; only a reference to a bare `Label` already defined in the same section survives.

There is no Mach-O platform and no ABI parameter — `i386.New` takes only a `Platform` and options.

## Dialects

```go
u, err := i386.ParseFile("boot.s", src, i386.GAS)
u, err := i386.ParseFile("boot.asm", src, i386.NASM)
out, err := i386.Print(u, i386.NASM)
```

Both parse to and print from the same `text.Unit`, so a file read in one dialect can be formatted in the other.

## Assembling from source

```go
u, err := i386.ParseFile("boot.s", src, i386.GAS)
if err != nil {
	// ...
}
b, err := i386.Assemble(u, i386.ELF, i386.DefaultFeatures())
```

`Assemble` is `arc build`'s whole job in one call: it walks `u` in source order, places every statement into a fresh `Assembler`'s sections exactly as the hand-built calls below would, and returns `Serialize`'s bytes — no separate translation step for a caller to get wrong.

Instruction operands have no gap here: a symbolic call or memory reference carries through to `Serialize` via the usual fixups, whether it came from text or from the typed builder API. What `Assemble` does not yet do is fold a `.long` or `.fill` value that is not a plain constant — `.long . - msg` and similar section-relative or symbolic data need a fixup the same way an operand does, and that backpatch path isn't wired up yet. Such an item is refused with a specific error rather than silently miswritten.

## Registers and operands

```go
i386.EAX, i386.AL, i386.XMM0, i386.CR0
i386.Mem32(i386.EBX).Disp(8)
i386.Imm(60)
i386.Ref("puts", i386.R_386_PLT32)
```

Re-exported from `reg/` and `operand/` so a caller never needs the second import.

## Building an object by hand

`exit(60)` — hand-built with `Section`, `Label` and `Emit`, no text file involved, using the same primitives `Assemble` drives internally:

```go
package main

import (
	"os"

	"github.com/vertex-language/arc/i386"
)

func main() {
	a := i386.New(i386.ELF)

	t := a.Section(i386.Text)
	t.Align(16)
	t.Label("_start", i386.Global, i386.Func)
	t.Emit("mov", i386.EAX, i386.Imm(60))
	t.Emit("xor", i386.EDI, i386.EDI)
	t.Emit("int", i386.Imm(0x80))

	b, err := a.Serialize()
	if err != nil {
		panic(err)
	}
	os.WriteFile("a.o", b, 0o644)
}
```

Calling an external symbol through the PLT, with a segment override on the memory operand:

```go
t := a.Section(i386.Text)
t.Label("greet", i386.Global, i386.Func)
t.Emit("mov", i386.EAX, i386.Mem32(i386.EBX).Segment(i386.GS).Disp(8))
t.Emit("call", i386.Ref("puts", i386.R_386_PLT32))
t.Emit("ret")
```

`Emit` resolves the shortest legal encoding from `isa.Resolve` and `encode.Encode` against `a.Features()`; nothing survives the call.