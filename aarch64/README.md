# aarch64

The AArch64 arch package: registers, ISA tables, encoder, decoder, text layer, and the assembler that ties them to an object file.

## Layout

```
aarch64/
├── target.go            Platform, New, Option, Platforms/ABIs/Dialects/Baseline
├── error.go              Error, ErrFeature/ErrForm/ErrReloc/ErrUndefined/ErrPlatform
├── asm.go                 Assembler, Section, Align, Label, Emit, data emission
├── code.go                 Encode, Forms, Decode, Explain — forwards to isa/encode/decode
├── text.go                  ParseFile, ParseInst, Print, PrintInst, ResolveUnit, Format
├── assemble.go               Assemble — text.Unit → object bytes, arc build's whole job
├── feature.go                 re-export of feature/: FeatureSet, Level, Armv8A…Armv9_5A, ParseFeatures
├── operand.go                  re-export of reg/ and operand/: registers, Imm, Mem, Shift, Extend, Page, PageOff, Label, Ref, the Operand interface
│
├── helpers_base_gen.go          one method per form, generated from isa/ — not yet generated
├── helpers_simd_gen.go
├── helpers_sve_gen.go
├── helpers_sys_gen.go
│
├── reloc.go                      RelocKind registry, validity table, role → kind mapping
├── reloc_elf.go                   R_AARCH64_*
├── reloc_coff.go                   IMAGE_REL_ARM64_*
├── reloc_macho.go                   ARM64_RELOC_*
├── write.go                          Serialize, WriteTo, fixup folding, symbol-size closing
├── write_elf.go                       ELF platform writer — mapping symbols, .note.GNU-stack
├── write_coff.go                       COFF platform writer
├── write_macho.go                       Mach-O platform writer — the ADDEND pairing
├── write_flat.go                         Flat platform writer — concatenation, no relocations
│
├── feature/               Level, Feature, Set — the extension vocabulary and gating
│   ├── feature.go          the vocabulary, Requires, closure edges
│   ├── level.go             Armv8A…Armv8_9A, Armv9A…Armv9_5A as closed sets, Baseline
│   └── parse.go              ParseFeature, ParseFeatures, the +ext / +noext grammar
│
├── reg/                    X, W, Xsp, V/Q/D/S/H/B, Z, P, Sys
│   ├── reg.go                Class, File, the Reg interface, Overlaps
│   ├── gpr.go                 X0…X30, W0…W30, and the register-31 split: XZR/WZR vs SP/WSP
│   ├── vec.go                  V0…V31 and the Q/D/S/H/B scalar views, Arrangement, Lane
│   ├── sve.go                   Z0…Z31, P0…P15, the governing-predicate distinction
│   ├── sys.go                    the {op0,op1,CRn,CRm,op2} system-register space, NZCV, FPCR, FPSR
│   ├── name.go                    Name, String, Lookup, the .req alias table's read side
│   ├── dwarf.go                    the aadwarf64 numbering — *not* a permutation, unlike x86_64's
│   └── save.go                      Save — AAPCS64 preservation, including v8–v15's low-64-only rule
│
├── operand/                the operand set
│   ├── operand.go            Width, the Operand vocabulary
│   ├── imm.go                 Imm and the width predicates: Imm12, Imm16Shifted, Imm7Scaled, Imm9
│   ├── bitmask.go              the logical-immediate encoder — N:immr:imms, and Encodable
│   ├── mem.go                   the six addressing forms, Validate
│   ├── shift.go                  LSL/LSR/ASR/ROR on a register operand
│   ├── extend.go                  UXTB…SXTX with an optional shift amount
│   └── sym.go                      RelocKind, Target, Label, SymRef, Page, PageOff, GotPage, GotPageOff
│
├── isa/                    Form, Class, Slot — the instruction table
│   ├── isa.go                All, Forms, Mnemonics, Enabled — the registry
│   ├── form.go                 Form, finish's table-time checks, GoName, Word, Mask
│   ├── class.go                  Class — width, register file, memory-acceptance, in one value
│   ├── slot.go                    Slot, Role, Field — one operand, and the bits it lands in
│   ├── alias.go                    the architecture's own alias relation and preferred-disassembly rule
│   ├── arg.go                       Arg, Class.Match — what Resolve matches against
│   ├── resolve.go                    Resolve, UnknownError/FormError/GateError
│   ├── build.go                       the fluent modifiers table rows are built from
│   ├── table_base.go                   the base integer, branch, and load/store set
│   ├── table_simd.go                    Advanced SIMD and scalar FP, and crypto
│   ├── table_sve.go                      SVE and SVE2
│   └── table_sys.go                       MRS/MSR, the system instruction class, barriers, hints
│
├── encode/                 form + operands → one 32-bit word + fixups
│   ├── encode.go              Encode, EncodeForm, Opts
│   ├── operand.go               the lowering from caller values to internal vals
│   ├── field.go                  bit-field placement, and the split fields (ADRP's immlo/immhi)
│   ├── imm.go                     immediate width and scaling selection
│   ├── bitmask.go                  the logical-immediate search
│   ├── fixup.go                     Fixup, Role — what a platform writer needs
│   ├── nop.go                        Nop — the one padding word Align uses
│   └── error.go                       CountError/OperandError/RegisterError/RangeError/…
│
├── decode/                 word → form + operands, and Explain
│   ├── decode.go              Inst, Decode, DecodeAll
│   ├── table.go                 the four-level decode table built from isa.All()
│   ├── operand.go                rebuilding operand values from decoded fields
│   ├── alias.go                   applying the preferred-disassembly rule
│   ├── explain.go                  Explanation, Field — arc explain's output
│   └── error.go                     ErrTruncated, ErrUnaligned, UnknownError, ClassError
│
└── text/
    ├── unit.go, node.go, pos.go     Unit, Node, Pos — the tree
    ├── expr.go                       Expr, Eval, Reduce, Value — the symbolic-residue evaluator
    ├── inst.go                        Inst, Lower
    ├── operand.go                     Operand, MemRef, Arrangement, Lower to operand/ values
    ├── directive.go                   Directive, Kind, Values/Const/Alignment/ArchState
    ├── modifier.go                    Modifier — the platform-neutral :lo12: / @PAGEOFF
    └── gas/                A64 syntax: lex.go, parse.go, print.go, directive.go, expr.go, mnemonic.go
```

Only two files import more than one subpackage tree: `text.go`, the sole importer of `text/gas`, and the five `write_*.go` files, the sole importers of `objectfile/`. Nothing below the root imports the root.

## Subpackages

- **feature/** — the architecture ladder (`Armv8A` through `Armv8_9A`, `Armv9A` through `Armv9_5A`) and the orthogonal extensions above it (`CRC`, `LSE`, `RDMA`, `FP16`, `DotProd`, `SHA3`, `SM4`, `RCPC`, `SVE`, `SVE2`, `BF16`, `I8MM`, `MemTag`, `PAuth`, `BTI`, `SME`…), each closed under its own requirements. `Baseline` is `Armv8A`. A version is shorthand for a closed set, not a separate axis, and the ladder is not monotone in the way x86's levels are: `CRC` is optional at `Armv8A` and mandatory from `Armv8_1A`, so `Armv8A.Plus(CRC)` and `Armv8_1A` are different sets that overlap, and `Decompose` names each of them the way the world spells it.
- **reg/** — every physical register, with `Num`, `Bits`, `Class`, `Save` (AAPCS64 preservation, which for `V8`…`V15` is *partial* — the low 64 bits only, and `Save` says so rather than rounding it to "preserved"), `DWARF`, and `Overlaps`. `W0` is a view that zero-extends into `X0` on write and answers `Parent()` accordingly. Register number 31 is two registers: `XZR`/`WZR` and `SP`/`WSP` are separate values of separate types, because the encoding is identical and only the form knows which one a slot means — the aarch64 shape of x86_64's `AH`/`SPL` collision, and the reason a shared `Reg` across arches was never possible.
- **operand/** — `Imm` and the width-and-scale predicates each field imposes (`ADD`'s 12 bits with an optional `LSL #12`, `MOVZ`/`MOVK`'s 16 bits at four shifts, `LDP`'s scaled 7, `LDUR`'s signed 9); `bitmask.go`, the logical-immediate encoder, where "is this constant even expressible" is a first-class question with a real answer; `Mem` and the six addressing forms (`[Xn]`, `[Xn, #imm]` scaled unsigned and unscaled signed, `[Xn, Xm{, LSL #s}]`, `[Xn, Wm, SXTW #s]`, pre-index `[Xn, #imm]!`, post-index `[Xn], #imm`) with a `Validate` that catches what has no encoding; `Shift` and `Extend`, which decorate a register operand rather than naming a different instruction; and `Target`/`Label`/`SymRef` plus the four address roles, `Page`, `PageOff`, `GotPage`, `GotPageOff`.
- **isa/** — the form table: every declared encoding, its operand classes, its base word and mask, and the feature that gates it. Because the width is fixed there is no shortest-form search: `Resolve` matches operand classes to exactly one form, and a mnemonic that could plausibly mean two things is a table-time error rather than a tie broken at runtime. `alias.go` holds the architecture's own alias relation — `CMP` is `SUBS` with `XZR` as the destination, `MOV` is `ORR` or `ADD` depending on the operands, `LSL` is `UBFM`, `RET` is `RET X30` — one-to-one in both directions, which is what makes it an alias and not instruction selection.
- **encode/** — a pure function from a resolved `*isa.Form` and operand values to one word and its `Fixup`s. There are no prefixes and no displacement sizing; the work is bit-field placement, including the fields the architecture splits (`ADRP` puts its low two immediate bits at 30:29 and the rest at 23:5) and the immediates that are computed rather than copied (the logical-immediate `N:immr:imms` triple). `Nop` is one word, `d503201f`.
- **decode/** — the inverse machine, built from `isa.All()` and nothing else: the four-level decode table the ARM ARM's own structure describes, operand reconstruction, and the preferred-disassembly rule from `alias.go`, so `arc dis` prints what `objdump` prints. `Decode`'s operand values are the exact concrete types `encode.Encode` accepts, which is the round-trip guarantee.
- **text/** — the tree, plus `Eval`/`Reduce`, which turns `. - msg` into a symbolic `Value` (a constant, at most one symbol added, at most one subtracted) rather than failing outright.
- **text/gas/** — A64 syntax as GNU `as` and the LLVM integrated assembler accept it: destination first, immediates optionally `#`-prefixed, memory in `[brackets]`, vector arrangements and lanes as operand suffixes (`v0.4s`, `v2.s[1]`), `//` and `;` as comment and separator.

## Platforms

```go
a := aarch64.New(aarch64.ELF)
a := aarch64.New(aarch64.COFF)
a := aarch64.New(aarch64.MachO)
a := aarch64.New(aarch64.Flat)
```

A platform is an object format, not an operating system. This package does not know what Linux is; the OS and vendor fields of an LLVM triple are discarded at the boundary before anything here sees them.

- **ELF** — full ET_REL relocatable objects via `objectfile/elf`, targeting `EM_AARCH64`. `.note.GNU-stack` is emitted by default, for the same reason it is on x86_64. AAELF64 requires mapping symbols marking where code ends and data begins, so `$x` and `$d` are emitted automatically at every transition; they are not something a caller states and not something a caller can suppress, because an object without them disassembles wrong. `.note.gnu.property` carrying the BTI and PAC feature bits is emitted only if asked, via `a.ELF().SetFeature1(...)` — `arc` cannot claim a guarantee about code it did not write.
- **COFF** — full Win64 objects via `objectfile/coff`, targeting `IMAGE_FILE_MACHINE_ARM64`. No name mangling, the same as x86_64 and unlike i386.
- **Mach-O** — full `MH_OBJECT` files via `objectfile/macho`, targeting `CPU_TYPE_ARM64`. `objectfile/macho` prepends the leading underscore, so a name written with one arrives doubled. `LC_BUILD_VERSION` only via `a.MachO().SetMinOS(...)`. This is the platform with the most refused relocations — see below.
- **Flat** — a raw concatenation of every section in creation order via `objectfile/flat`, no header and no symbol table. `SetBaseAddress` sets the load address, and is a usage error on any other platform. This is the platform boot code and firmware want, and the one where a surviving fixup is fatal.

Section naming differs on Mach-O, where a name is a `segment,section` pair: `Section(Text)` maps to `__TEXT,__text`, `SectionNamed("__DATA,__const")` goes through verbatim, and a bare `__const` is refused because two real sections spell it.

There is no ABI parameter — `aarch64.New` takes only a `Platform` and options. ILP32 is not a target here: it is a different ELF class and a different relocation namespace (`R_AARCH64_P32_*`), which makes it a different target rather than a knob on this one. Big-endian is likewise not a knob; endianness is in an arch name, and `aarch64_be` is not one of the nine.

## Features

```go
a := aarch64.New(aarch64.ELF, aarch64.WithFeatures(aarch64.Armv8_2A))
a := aarch64.New(aarch64.ELF, aarch64.WithFeatures(aarch64.Armv8A.Plus(aarch64.LSE)))

f, err := aarch64.ParseFeatures("armv8.2-a+sve+nofp16")
```

The default is `Baseline()`, `Armv8A`. `ParseFeatures` accepts the `+ext` / `+noext` grammar the world writes, including the rule that removals follow additions, and a set is always closed under requirements in both directions: adding `SVE2` pulls in `SVE`, and dropping `FP16` drops everything that needs it.

Gating is at encode time, at the call, with the flag that would have allowed it:

```
.text+0x18: casal requires lse, not in the active feature set
  active: armv8-a
  note: aarch64.WithFeatures(aarch64.Armv8A.Plus(aarch64.LSE))
```

Options are read at `New`; there is no `SetFeatures`, because a feature set that changed halfway through would make that diagnostic unfalsifiable.

The one exception is the text path, and it is the source's exception rather than the API's. `.arch`, `.arch_extension` and `.cpu` change the accepted set mid-file, and `Assemble` honours them, because each is a statement at a line number: the diagnostic still names a flag and still names the line that set it. `.arch` clears extensions selected before it, which is what GNU `as` does and what code written against `as` depends on.

## Syntax

```go
u, err := aarch64.ParseFile("boot.s", src)
out, err := aarch64.Print(u)
```

There is one syntax and no `Dialect` parameter. NASM has no A64 grammar to accept, and inventing one would be inventing syntax. `--dialect` on an aarch64 target is a usage error naming that, not a no-op.

What does vary is the modifier spelling, and it varies by *platform* rather than by dialect. GNU `as` writes `adrp x0, :pg_hi21:foo` (the prefix optional) and `add x0, x0, #:lo12:foo`; the Darwin assembler writes `foo@PAGE` and `foo@PAGEOFF`, and rejects `:got:`/`:got_lo12:` outright. Both are the same four roles — page, page offset, GOT page, GOT page offset — so `text.Modifier` stores the role and the printer picks the spelling from the target:

```go
b, err := aarch64.Format("boot.s", src, aarch64.ELF)     // :lo12:
b, err := aarch64.Format("boot.s", src, aarch64.MachO)   // @PAGEOFF
```

This is the same reasoning that makes `--dialect` a spelling and never a byte: the bytes are identical, and the two spellings name one thing. Treating it as a dialect would let a caller ask for `@PAGEOFF` in an ELF object, which is not a preference — it is a file no assembler on that platform will read back.

## Aliases and pseudo-instructions

The architecture defines aliases, and this package implements them. `cmp x0, x1` resolves to the `SUBS` form with `XZR` as its destination; `mov x0, x1` resolves to `ORR`; `ret` resolves to `RET X30`; `lsl x0, x1, #4` resolves to `UBFM`. Each is one-to-one, each is in the ARM ARM's own table, and the decoder applies the preferred-disassembly rule so `arc dis` prints `cmp` where `objdump` prints `cmp`.

What this package does not implement is the assembler-invented layer, each refused with a message naming what it would have taken:

- **`ldr x0, =value`** and the literal pool it implies, along with `.ltorg` and `.pool`. Placing a constant into a pool and generating a PC-relative load to it means the assembler choosing where data lives, which is a layout engine, and choosing an instruction the caller did not name, which is instruction selection. Write the constant into a section and load from it.
- **`mov x0, #0x123456789`** where no single `MOVZ` covers the value. GNU `as` expands this to a `MOVZ`/`MOVK` chain. That is one mnemonic becoming three instructions, and `Emit` picks an encoding of the instruction you named or nothing.
- **`.macro`, `.rept`, `.if`.** Go is the macro language, as everywhere else in this tree.

`.inst` is accepted: it states a word rather than naming an instruction, which is the one case where emitting bytes nobody selected is exactly what was asked.

## Assembling from source

```go
u, err := aarch64.ParseFile("boot.s", src)
if err != nil {
	// ...
}
b, err := aarch64.Assemble(u, aarch64.ELF, aarch64.DefaultFeatures())
```

`Assemble` is `arc build`'s whole job in one call. It walks `u` in source order: a label places a symbol; a directive is dispatched on its `Kind` — section changes, binding and type declarations, data, space, and the arch-state group (`.arch`, `.arch_extension`, `.cpu`, `.req`, `.unreq`); an instruction is lowered, resolved against the active feature set, and placed.

What `Assemble` refuses, each with a specific error naming the gap:

- `.comm`, `.lcomm` and `.equ`, which need a value threaded across statements — an `Env` — and `Assemble` runs with none.
- `.org`, which needs an image-layout step this tree has no linker-free version of yet.
- A `.quad`/`.word`/`.fill` value that reduces to something other than a plain constant or a single added symbol. `QuadRef`/`WordRef` cover the symbolic case in the builder API, so the gap is in the text path only.
- The pseudo-instruction set above.

## Registers and operands

```go
aarch64.X0, aarch64.W11, aarch64.XZR, aarch64.SP, aarch64.X30
aarch64.V0, aarch64.Q3, aarch64.D7, aarch64.S12, aarch64.Z31, aarch64.P1
aarch64.V0.Arr(aarch64.B16)          // v0.16b
aarch64.V2.Lane(aarch64.S, 1)        // v2.s[1]
aarch64.Mem(aarch64.X1).Off(8)       // [x1, #8]
aarch64.Mem(aarch64.SP).Pre(-32)     // [sp, #-32]!
aarch64.Mem(aarch64.X1).Post(16)     // [x1], #16
aarch64.Mem(aarch64.X1).Index(aarch64.W2, aarch64.SXTW, 3)
aarch64.Imm(93)
aarch64.Page(aarch64.Label("msg"))
aarch64.Ref("puts", aarch64.R_AARCH64_CALL26)
```

Re-exported from `reg/` and `operand/` so a caller never needs the second import. These are type and constant aliases, not parallel definitions.

`XZR` and `SP` are distinct values with distinct types even though both encode as 31. A form that reads register 31 as the stack pointer will not accept `XZR`, and the mismatch is a compile error at the typed call and a `RegisterError` at `Emit`. Rounding them into one value would make `Overlaps` and `Parent` answer questions that have two different right answers.

## Building an object by hand

`exit(0)` — hand-built with `Section`, `Label` and the generated helpers, no text file involved:

```go
package main

import (
	"os"

	"github.com/vertex-language/arc/aarch64"
)

func main() {
	a := aarch64.New(aarch64.ELF)

	t := a.Section(aarch64.Text)
	t.Align(4)
	t.Label("_start", aarch64.Global, aarch64.Func)
	t.MovzImm64(aarch64.X8, 93)        // d2 80 0b a8   mov x8, #93
	t.MovzImm64(aarch64.X0, 0)         // d2 80 00 00   mov x0, #0
	t.Svc(0)                           // d4 00 00 01   svc #0

	b, err := a.Serialize()
	if err != nil {
		panic(err)
	}
	os.WriteFile("exit.o", b, 0o644)
}
```

One helper per form, named after the form the ARM ARM spells: `AddImm64` is `ADD (immediate)` 64-bit, `AddShifted64` is `ADD (shifted register)`, `AddExt64` is `ADD (extended register)`, and none of them will quietly become another. A width mismatch is a compile error:

```go
t.MovzImm64(aarch64.W8, 93)
// cannot use aarch64.W8 (variable of type reg.W) as reg.X value
```

The layer is smaller than x86_64's — a few thousand methods rather than twelve — because the ISA has fewer encodings per mnemonic, which is the same property that removes the shortest-form search. See Status: the generator has not landed and the calls above do not compile yet.

`Emit` is the layer that does work today:

```go
t := a.Section(aarch64.Text)
t.Label("greet", aarch64.Global, aarch64.Func)
t.Emit("adrp", aarch64.X1, aarch64.Page(aarch64.Label("msg")))
t.Emit("add", aarch64.X1, aarch64.X1, aarch64.PageOff(aarch64.Label("msg")))
t.Emit("ldr", aarch64.X2, aarch64.Mem(aarch64.X1).Off(aarch64.PageOff(aarch64.Label("n"))))
t.Emit("bl", aarch64.Ref("puts", aarch64.R_AARCH64_CALL26))
t.Emit("ret")
```

Nothing survives either call. A section is a byte buffer and a fixup list.

## Errors are collected, not returned

Identical to x86_64: builder calls return nothing, the first failure is kept with its section and offset, every later call is a no-op, and `Serialize` returns it. `Err` reports early. Categories are `ErrFeature`, `ErrForm`, `ErrReloc`, `ErrUndefined`, `ErrPlatform`, derived from the wrapped error rather than stored.

## Symbols

```go
t.Label("_start", aarch64.Global, aarch64.Func)
t.Label(".L2")
a.Declare("puts", aarch64.Global)
a.SetSize("_start", 32)
a.SetVariantPCS("neon_helper")
```

Bindings, types, size-closing at `Serialize`, and redefinition-is-an-error all work as they do on x86_64. `SetVariantPCS` sets `STO_AARCH64_VARIANT_PCS`, the ELF-only marker that a function does not follow the base call standard — it exists because the linker needs it to avoid inserting a stub that clobbers registers the base standard says are free, and there is no way to infer it from the bytes.

## Data

```go
r := a.Section(aarch64.Rodata)
r.Label("msg", aarch64.Local, aarch64.Object)
r.Ascii("hello, silicon\n")
r.Quad(0x1000)
r.Fill(64, 1, 0)

j := a.Section(aarch64.Data)
j.QuadRef(aarch64.Label("case0"))
```

`Byte`, `Half`, `Word` and `Quad` append little-endian integers of 1, 2, 4 and 8 bytes. The names are the architecture's rather than gas's inherited ones: `.word` is four bytes here and `.dword`/`.xword` is eight, which is the opposite of x86's convention and the reason this package does not reuse x86_64's spelling.

`Align` pads a code section with `NOP` (`d503201f`) and everything else with zeros. Only one no-op is needed, because every instruction is one word — there is no multi-byte no-op table and no question of where a decoder resumes. An alignment that is not a multiple of 4 on a code section is refused rather than rounded.

On ELF, a code-to-data transition inside one section emits a `$d` mapping symbol and the transition back emits `$x`. This is not optional decoration; it is how the format says which bytes are instructions.

## Relocations and fixups

A fixup is a field an encoding left blank because its value is an address that is not yet a number. It is not a relocation.

On this architecture, one address reference is usually *two* fixups. `adrp`+`add` materializes an address as a page and an offset within it, and each half needs its own record. The caller states the role — `Page`, `PageOff`, `GotPage`, `GotPageOff` — and the platform writer picks the kind, because the kind is per format and the role is the only portable part.

The offset half also depends on the instruction the fixup lands in, and this is where the fixup/relocation distinction pays for itself. Under ELF, `add x1, x1, :lo12:msg` wants `ADD_ABS_LO12_NC`, while `ldr x2, [x1, :lo12:n]` wants `LDST8`, `LDST16`, `LDST32`, `LDST64` or `LDST128` according to the access width, because the immediate is scaled by that width. The caller does not know the width — the *form* does, and the encoder records it on the `Fixup`. Under COFF the same fixup maps to two kinds rather than five, split by instruction class instead: `PAGEOFFSET_12A` for the add and `PAGEOFFSET_12L` for the load, with the shift derived from the instruction itself. One role, five ELF answers and two COFF answers, and no caller writes either.

At `Serialize`, a fixup is folded or recorded. It is folded when all four hold: the field is pc-relative, the symbol is defined in the same section, the caller named no relocation kind, and the symbol is local. A `b`/`bl` to a local label two lines away folds; everything else becomes a record.

### The addend

`Fixup.Tail` is always zero on this arch, and the code that computes it is a constant. Every instruction is one word, the field is inside it, and every pc-relative relocation is defined against the address of that word. There is no `A - (size + tail)` correction, which is the entire x86_64 addend table collapsing into nothing.

What replaces it is a three-way disagreement about *where the addend goes*:

| | where A lives | `bl puts+8` |
| --- | --- | --- |
| ELF | `r_addend`, in the RELA record | one record, addend 8 |
| COFF | in place, in the instruction's immediate field | one record, 8 encoded in the branch |
| Mach-O | a preceding `ARM64_RELOC_ADDEND` record | two records, instruction field zeroed |

Mach-O's answer is the one that costs something. An addend on `BRANCH26`, `PAGE21` or `PAGEOFF12` cannot be written into the instruction and cannot be written into the record; it needs a second record placed before the first, carrying the value in its symbol-index field, and it is limited to a signed 24-bit range. That is a paired-record emission `objectfile/macho` does not have a case for, which makes it the same shape of hole as x86_64's `SUBTRACTOR`.

### What is wired

Each format's full relocation set is declared in `reloc_*.go`. A kind that is declared but has no mapping in `objectfile/` is refused at `Serialize` with a message naming the gap.

| Platform | Wired end to end | Declared and refused |
| --- | --- | --- |
| ELF | `NONE`, `ABS64`, `ABS32`, `PREL64`, `PREL32`, `CALL26`, `JUMP26`, `CONDBR19`, `TSTBR14`, `LD_PREL_LO19`, `ADR_PREL_PG_HI21`, `ADD_ABS_LO12_NC`, `LDST8/16/32/64/128_ABS_LO12_NC`, `ADR_GOT_PAGE`, `LD64_GOT_LO12_NC` | `ABS16`, `PREL16`, `ADR_PREL_LO21`, `ADR_PREL_PG_HI21_NC`, the `MOVW_UABS_*`, `MOVW_SABS_*` and `MOVW_PREL_*` families, `LD64_GOTOFF_LO15`, `GOT_LD_PREL19`, the whole TLS and TLSDESC family, `COPY`, `GLOB_DAT`, `JUMP_SLOT`, `RELATIVE`, `IRELATIVE` |
| COFF | `ABSOLUTE`, `ADDR64`, `ADDR32`, `ADDR32NB`, `BRANCH26`, `PAGEBASE_REL21`, `REL21`, `PAGEOFFSET_12A`, `PAGEOFFSET_12L`, `SECREL` | `SECREL_LOW12A`, `SECREL_HIGH12A`, `SECREL_LOW12L`, `TOKEN`, `SECTION`, `BRANCH19`, `BRANCH14`, `REL32` |
| Mach-O | `UNSIGNED`, `BRANCH26`, `PAGE21`, `PAGEOFF12`, `GOT_LOAD_PAGE21`, `GOT_LOAD_PAGEOFF12` | `SUBTRACTOR`, `POINTER_TO_GOT`, `TLVP_LOAD_PAGE21`, `TLVP_LOAD_PAGEOFF12`, `ADDEND` |

Two holes worth knowing about before targeting Mach-O. `ARM64_RELOC_ADDEND` being unwired means a reference with a nonzero addend — `bl puts+8`, `adrp x0, msg+16` — assembles for ELF and COFF and is refused here, which is a failure on an ordinary instruction rather than an exotic one. `ARM64_RELOC_SUBTRACTOR` is the other half of the `.quad a - b` gap, the same paired-record shape.

The dynamic ELF kinds (`COPY`, `GLOB_DAT`, `JUMP_SLOT`, `RELATIVE`, `IRELATIVE`) are declared because `linker/elf` reads them, and refused here because an assembler has no business emitting one into a relocatable object.

`Weak` and `Hidden` degrade the same way they do on x86_64, for the same reasons in `objectfile/`.

## Flat images

```go
a := aarch64.New(aarch64.Flat)
a.SetBaseAddress(0x80000)
```

Flat is the platform with the most to refuse, and on this arch it refuses more than elsewhere, because the common way to name an address is `adrp`+`add` and *both halves* are references. A page-relative fixup to a symbol in the same section resolves and survives; anything crossing a section, naming an external symbol, or carrying an explicit relocation kind is refused at `Serialize` with a message saying which of the three applies.

`SectionBSS` is zeroes in the file here, unlike the other three formats: there is no loader to zero-fill a reservation.

## Encoding and decoding without an object

```go
b, fx, err := aarch64.Encode(aarch64.Baseline(), "mov", aarch64.X8, aarch64.Imm(93))
in, err := aarch64.Decode(b)
ex, err := aarch64.Explain(b)
```

`Decode` takes bytes in memory order and reads the word little-endian; a length that is not a multiple of 4 is `ErrUnaligned` rather than a truncated guess.

`Explain` is the field-by-field breakdown `arc explain` prints:

```
$ arc explain a80b80d2
mov x8, #93                                    aarch64 · armv8-a · 4 bytes

  word     d2800ba8   MOVZ (64-bit)            alias: mov
  sf       1          [31]                     64-bit operand size
  opc      10         [30:29]                  MOVZ
  hw       00         [22:21]                  shift 0
  imm16    0x005d     [20:5]                   93
  Rd       01000      [4:0]                    → x8
```

The alias line is part of the answer rather than a footnote: `mov x8, #93` and `movz x8, #93` are the same word, and a breakdown that named only one of them would leave a reader unable to match it against either the source or the disassembly.

## Status

The design is settled; nothing in this tree has landed.

| | |
| --- | --- |
| `objectfile/elf`, `coff`, `macho`, `flat` | implemented for this arch already |
| `linker/elf`, `macho`, `pe` | implemented |
| everything in this directory | unlanded |

`aarch64` lands after `x86_64` and `i386` and before `riscv64`. Until then `arc -t aarch64-*` reports the arch as unlanded, naming this file.