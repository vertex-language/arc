# x86_64

The AMD64 arch package: registers, ISA tables, encoder, decoder, text layer, and the assembler that ties them to an object file.

## Layout

```
x86_64/
├── target.go            Platform, Dialect, New, Option, Platforms/ABIs/Dialects/Baseline
├── error.go              Error, ErrFeature/ErrForm/ErrReloc/ErrUndefined/ErrPlatform
├── asm.go                 Assembler, Section, Align, Label, Emit, data emission
├── code.go                 Encode, Forms, Decode, Explain — forwards to isa/encode/decode
├── text.go                  ParseFile, Print, ResolveUnit, Translate — GAS/NASM dispatch
├── assemble.go               Assemble — text.Unit → object bytes, arc build's whole job
├── feature.go                 re-export of feature/: FeatureSet, Level, V1…V4, ParseFeatures
├── operand.go                  re-export of reg/ and operand/: registers, Imm, M8…M512, RIPRel, Label, Ref, the Operand interface
│
├── helpers_base_gen.go          one method per form, generated from isa/ — not yet generated
├── helpers_sse_gen.go
├── helpers_avx_gen.go
├── helpers_avx512_gen.go
├── helpers_amx_gen.go
│
├── reloc.go                      RelocKind registry, validity table, modifier mapping
├── reloc_elf.go                   R_X86_64_*
├── reloc_coff.go                   IMAGE_REL_AMD64_*
├── reloc_macho.go                   X86_64_RELOC_*
├── write.go                          Serialize, WriteTo, fixup folding, symbol-size closing
├── write_elf.go                       ELF platform writer
├── write_coff.go                       COFF platform writer
├── write_macho.go                      Mach-O platform writer
├── write_flat.go                        Flat platform writer — concatenation, no relocations
│
├── feature/               Level, Feature, Set — the extension vocabulary and gating
│   ├── feature.go          the vocabulary, Requires, closure edges
│   ├── level.go            V1…V4 as closed sets, Baseline
│   └── parse.go            ParseFeature, ParseFeatures, aliases
│
├── reg/                    Reg8…Reg64, Sreg, St, Mm, Xmm, Ymm, Zmm, K, Tmm, Cr, Dr
│   ├── reg.go                Class, File, Loc, the Reg interface, Overlaps
│   ├── gpr.go                 the general-purpose registers, the AH/SPL collision
│   ├── vec.go                  Xmm/Ymm/Zmm, K, Tmm
│   ├── x87.go                   St, Mm
│   ├── sys.go                    Sreg, Cr, Dr
│   ├── name.go                    Name, String, Lookup
│   ├── dwarf.go                    the psABI DWARF numbering — not derived from Num
│   └── save.go                     Save — System V vs. Win64 preservation
│
├── operand/                the operand set: Imm, Mem, M8…M512, RIPRel, Label, SymRef
│   ├── operand.go            Width
│   ├── imm.go                  Imm, Uimm, the Fits*/Narrowest predicates
│   ├── mem.go                   Mem, the six addressing forms, Validate
│   ├── mem_width.go              M8…M512, the width-fixing wrappers
│   └── sym.go                     RelocKind, Target, Label, SymRef
│
├── isa/                    Form, Class, Slot — the instruction table
│   ├── isa.go                All, Forms, Mnemonics, Enabled — the registry
│   ├── form.go                 Form, finish's table-time checks, GoName, Opcodes
│   ├── class.go                  Class — width, register file, memory-acceptance, all at once
│   ├── slot.go                    Slot, Role, Field — one operand of one form
│   ├── encoding.go                 Enc, Map, Pfx, WBit, VLen, Tuple, Attr
│   ├── arg.go                       Arg, Class.Match — what Resolve matches against
│   ├── resolve.go                    Resolve, UnknownError/FormError/GateError
│   ├── build.go                       L/V/E and the fluent modifiers table rows are built from
│   ├── table_base.go                   the base instruction set
│   ├── table_sse.go                     MMX through SSE4.2, and crypto
│   ├── table_avx.go                      VEX-encoded AVX/AVX2/F16C/FMA/BMI
│   └── table_avx512.go                    EVEX-encoded AVX-512, and AMX
│
├── encode/                 form + operands → bytes + fixups
│   ├── encode.go              Encode, EncodeWith, Opts, RoundMode
│   ├── operand.go               the lowering from caller values to internal vals
│   ├── prefix.go                 legacy prefixes, REX
│   ├── modrm.go                    ModRM, SIB, displacement, moffs
│   ├── vex.go                       the VEX and EVEX prefixes
│   ├── imm.go                        the immediate field
│   ├── fixup.go                       Fixup, Use — what a platform writer needs
│   ├── nop.go                          Nop, Nops — the padding Align uses
│   └── error.go                         CountError/OperandError/RegisterError/…
│
├── decode/                 bytes → form + operands, and Explain
│   ├── decode.go              Inst, Decode, DecodeAll
│   ├── prefix.go                the legacy-prefix scan and REX/VEX/EVEX dispatch
│   ├── table.go                  the opcode-map lookup built from isa.All()
│   ├── modrm.go                   ModRM/SIB/displacement, EVEX's disp8*N
│   ├── vex.go                      unwinding VEX and EVEX
│   ├── operand.go                   rebuilding operand values from decoded fields
│   ├── explain.go                    Explanation, Field — arc explain's output
│   └── error.go                       ErrTruncated, UnknownError, ClassError
│
└── text/
    ├── unit.go, node.go, pos.go     Unit, Node, Pos — the dialect-neutral tree
    ├── expr.go                       Expr, Eval, Reduce, Value — the symbolic-residue evaluator
    ├── inst.go                        Inst, OperandSize, Sized, Lower
    ├── operand.go                     Operand, MemRef, Lower to operand/ values
    ├── directive.go                   Directive, Kind, Values/Const/Alignment
    ├── modifier.go                    Modifier — the dialect-neutral @PLT / wrt ..plt
    ├── gas/                GNU as syntax: lex.go, parse.go, print.go, directive.go, expr.go, mnemonic.go
    └── nasm/                NASM syntax: lex.go, parse.go, print.go, directive.go, expr.go
```

Only two files import more than one subpackage tree: `text.go`, the sole importer of both `text/gas` and `text/nasm`, and the four `write_*.go` files, the sole importers of `objectfile/`. Nothing below the root imports the root.

## Subpackages

- **feature/** — the microarchitecture levels (`V1` through `V4`) and the orthogonal extensions they are built from, each closed under its own requirements (`Set.Plus`/`Minus` compute the closure; nothing tabulates it). `Baseline` is `V1` — x86-64-v1, SSE2 and nothing above it. `Set.String` and `Set.GoExpr` both go through `Decompose`, which names the highest level a set fully contains plus the minimal extras needed to reach the rest, so a diagnostic's note line and a set's own printed form can never disagree about what it's called.
- **reg/** — every physical register at every width, with `Num` (the architectural encoding), `Bits`, `Class`, `Save` (calling-convention preservation), `DWARF` (the psABI number, a *different* permutation of the encoding order — RCX and RDX swap, RSP/RBP/RSI/RDI rotate), `Parent`, and `Overlaps`. This is where `R8`…`R15`, the REX-only byte registers `SPL`/`BPL`/`SIL`/`DIL`, the legacy high bytes `AH`/`CH`/`DH`/`BH` that collide with them at the encoding level, the mask registers `K0`…`K7`, and the AMX tiles `TMM0`…`TMM7` live.
- **operand/** — `Imm` and its narrowing predicates; `Mem`, the six addressing forms (`[base]`, `[base+disp]`, `[base+index*scale+disp]`, indexless, absolute, `%rip`-relative) with a `Validate` that catches what has no encoding (RSP as an index, a scale that isn't 1/2/4/8, `%rip` with a base); the width-fixing wrappers `M8` through `M512`; and `Target`/`Label`/`SymRef`, what a displacement can point at before it is a number.
- **isa/** — the form table: every declared encoding of every mnemonic, its operand classes (`Class`, which is a width, a register file, and whether memory fills the slot, all in one value — the whole reason `MOV` has four ways to move a register), its opcode bytes, and the feature that gates it. `Resolve` picks the shortest legal form and never relaxes into a different instruction; `Forms`, `Enabled`, and `All` are the read side the generator and `arc isa` use.
- **encode/** — a pure function from a resolved `*isa.Form` and operand values to bytes and `Fixup`s: REX/VEX/EVEX construction, ModRM/SIB, displacement sizing (including EVEX's compressed disp8*N), immediate width selection. A `Fixup` is not a relocation — it carries `Use` (abs/pc-relative/branch) and `Tail` (bytes of the instruction that follow the field), which is what lets a caller never write `Addend: -4` by hand. Also holds `Nops`, the multi-byte no-op tables `Align` pads a code section with.
- **decode/** — the inverse machine, built from `isa.All()` and nothing else: opcode-map lookup, prefix disambiguation (the mandatory-SIMD-vs-operand-size-override guess that `MOVSD` depends on), VEX/EVEX unwinding, ModRM/SIB disassembly, and `Explain`'s field-by-field breakdown. `Decode`'s operand values are the exact concrete types `encode.Encode` accepts, which is the whole round-trip guarantee.
- **text/** — the dialect-neutral tree (`Unit`, `Node`, `Inst`, `Directive`, `Expr`) both syntaxes parse to and print from, plus `Eval`/`Reduce`, the expression evaluator that turns `. - msg` into a symbolic `Value` (a constant, at most one symbol added, at most one subtracted — the whole vocabulary a relocation can express) rather than failing outright.
- **text/gas/** — GNU as (AT&T syntax): operand order reversed at the edges, size as a mnemonic suffix, `@`-modifiers, `%rip`-relative spelling, gas's bitwise-above-additive expression precedence.
- **text/nasm/** — NASM (Intel syntax): operand order stored as written, size as a `BYTE`/`WORD`/`DWORD`/`QWORD` keyword, `REL`/`WRT` modifiers, NASM's C-like expression precedence.

## Platforms

```go
a := x86_64.New(x86_64.ELF)
a := x86_64.New(x86_64.COFF)
a := x86_64.New(x86_64.MachO)
a := x86_64.New(x86_64.Flat)
```

A platform is an object format, not an operating system. This package does not know what Linux is; the OS and vendor fields of an LLVM triple are discarded at the boundary before anything here sees them. The format does fix the calling convention, which is why `reg.Save` takes one.

- **ELF** — full ET_REL relocatable objects via `objectfile/elf`. `.note.GNU-stack` is emitted by default, because an object without one tells the linker nothing and the linker's answer to nothing is an executable stack.
- **COFF** — full Win64 objects via `objectfile/coff`, targeting `IMAGE_FILE_MACHINE_AMD64`. Unlike `i386`, `write_coff.go` applies no name mangling: Win64 dropped the cdecl leading underscore, so a symbol goes into the table spelled the way you wrote it.
- **Mach-O** — full `MH_OBJECT` files via `objectfile/macho`, targeting `CPU_TYPE_X86_64`. `arc` does not add the leading underscore Mach-O tooling expects on C symbols; `objectfile/macho` prepends one, so a name written with it arrives doubled. Objects carry no `LC_BUILD_VERSION` unless you ask, via `a.MachO().SetMinOS(...)` — `arc` cannot invent a minimum macOS version you didn't state.
- **Flat** — a raw concatenation of every section in creation order via `objectfile/flat`, no header and no symbol table. `SetBaseAddress` sets the load address the image starts at, and is a usage error on any other platform: a relocatable object doesn't have one.

Section naming differs on Mach-O, where a name is a `segment,section` pair. Both spellings work — `Section(Text)` maps to `__TEXT,__text`, and `SectionNamed("__DATA,__objc_classlist")` goes through verbatim. A bare `__const` is refused, because `__TEXT,__const` and `__DATA,__const` are two real sections and picking one would be guessing.

There is no ABI parameter — `x86_64.New` takes only a `Platform` and options.

## Features

```go
a := x86_64.New(x86_64.ELF, x86_64.WithFeatures(x86_64.V3))
a := x86_64.New(x86_64.ELF, x86_64.WithFeatures(x86_64.V1.Plus(x86_64.AVX512F)))

f, err := x86_64.ParseFeatures("x86-64-v2+avx512f")
```

The default is `Baseline()`, `V1`. Levels are shorthand for a closed set of extensions, not a separate axis: `V3` is `V2` plus AVX, AVX2, BMI1, BMI2, F16C, FMA, LZCNT and MOVBE, and it prints back as its canonical spelling. A set is always closed under requirements — there is no way to build one holding `AVX512BW` without `AVX512F`, and dropping a feature with `Minus` drops everything that depends on it too.

`WithFeatures` takes a `Level` or a `FeatureSet` and nothing else. A parameter of type `any` would also accept a bare `Feature`, which is one extension rather than a set, and `WithFeatures(AVX512F)` meaning "AVX-512F and no SSE2" is not what anyone writing it intends.

`ParseFeatures` accepts the aliases the world actually writes — `sse4_1`, `cx16`, `abm` — and discards them: the canonical spelling is what comes back out and what `String`/`GoExpr` print. A leading level sets the base; without one the set starts empty, not at `Baseline`, so `"sse2"` means sse2 alone.

Gating is at encode time, at the call, with the flag that would have allowed it:

```
.text+0x1c: vpaddd requires avx512f, not in the active feature set
  active: x86-64-v1
  note: x86_64.WithFeatures(x86_64.V1.Plus(x86_64.AVX512F))
```

Options are read at `New`. There is no `SetFeatures`; a feature set that changed halfway through an object would make that diagnostic unfalsifiable.

## Dialects

```go
u, err := x86_64.ParseFile("main.s", src, x86_64.GAS)
u, err := x86_64.ParseFile("main.asm", src, x86_64.NASM)
out, err := x86_64.Print(u, x86_64.NASM)
```

Both parse to and print from the same `text.Unit`, so a file read in one dialect can be formatted in the other. `Print` with `DialectNone` prints the unit in the dialect it was parsed from, which is what `arc fmt` without `--dialect` does.

Translating between dialects needs one more step:

```go
b, err := x86_64.Translate("main.asm", src, x86_64.NASM, x86_64.GAS, x86_64.Baseline())
```

`Translate` is parse, resolve, print. The resolution is not optional: gas writes an operand size as a mnemonic suffix and NASM writes it as an operand keyword, so going from `mov qword [rbx], 1` to `movq $1, (%rbx)` means knowing a width neither source states outright everywhere. The only thing that knows it is the form the encoder resolved, which is why `text.Inst` has a `Form` field and a text-level translator can't make this guarantee in both directions.

`Format` is the same call without the resolution step, for reprinting within one dialect. `ResolveUnit` is the middle step alone, for a caller that wants the resolved tree afterward rather than printed source.

## Assembling from source

```go
u, err := x86_64.ParseFile("main.s", src, x86_64.GAS)
if err != nil {
	// ...
}
b, err := x86_64.Assemble(u, x86_64.ELF, x86_64.DefaultFeatures())
```

`Assemble` is `arc build`'s whole job in one call. It walks `u` in source order: a label places a symbol (gas's numeric labels, `1:`, are position references and excluded, the same as `Unit.Defined`); a directive is dispatched on its `Kind` — section changes, binding and type declarations, data, space; an instruction is lowered, resolved against the active feature set if it doesn't already carry a `Form`, and placed. Serialize's bytes come back with no separate translation step for a caller to get wrong.

Instruction operands have no gap: a symbolic call or `%rip`-relative reference carries through to `Serialize` via the usual fixups, whether it came from text or from the typed builder API. What `Assemble` refuses, each with a specific error naming the gap rather than writing wrong bytes:

- `.comm`, `.lcomm` and `.equ`, which need a value threaded across statements — an `Env` — and `Assemble` runs with none.
- `.org`, which needs an image-layout step this tree has no linker-free version of yet.
- A `.quad`/`.long`/`.fill` value that reduces to something other than a plain constant or a single added symbol — `.quad . - msg` is legal assembly and needs a fixup the same way an operand does, and the backpatch that would consume the residue `text.Reduce` already computes isn't wired up here. The builder API's `QuadRef`/`LongRef` cover the symbolic case directly, so the gap is in the text path only.

## Registers and operands

```go
x86_64.RAX, x86_64.R11D, x86_64.SIL, x86_64.XMM0, x86_64.ZMM31, x86_64.K1, x86_64.CR0
x86_64.Mem64(x86_64.RBX).Disp(8)
x86_64.Mem64(x86_64.RBX).Index(x86_64.RCX, 4)
x86_64.RIPRel(x86_64.Ref("msg", x86_64.R_X86_64_PC32))
x86_64.Imm(60)
x86_64.Ref("puts", x86_64.R_X86_64_PLT32)
```

Re-exported from `reg/` and `operand/` so a caller never needs the second import. These are type and constant aliases, not parallel definitions: `x86_64.RAX` and `reg.RAX` are the same value of the same type, so a helper taking a `reg.Reg64` accepts either spelling.

`Operand` is declared here rather than in either subpackage, because stating it in `operand/` would mean `operand/` importing `reg/`'s implementers back, and `operand/` already imports `reg/`. It is deliberately loose — any `Stringer` satisfies it — which turns the common mistakes (an int, a string, a `*Section`) into compile errors at the call site rather than an `OperandError` at encode time; it buys nothing toward exhaustiveness, and `encode/`'s type switch is still what actually decides.

`x86_64.EAX` is a view that zero-extends into `RAX` on write and answers `Parent()` accordingly; `i386.EAX` is a full architectural register and answers nothing. That difference is why `reg/` here and i386's equivalent are separate copies rather than one shared package.

## Building an object by hand

`exit(60)` — hand-built with `Section`, `Label` and the generated helpers, no text file involved:

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

One helper per form, named after the form the table spells: `MovR64Imm64` is `MOV r64, imm64` and nothing else will quietly relax it to `MOV r/m64, imm32`. A width mismatch is a compile error rather than a diagnostic:

```go
t.MovR64Imm64(x86_64.EAX, 60)
// cannot use x86_64.EAX (variable of type reg.Reg32) as reg.Reg64 value
```

The size of that layer is the size of the ISA — roughly twelve thousand methods on `*Section` with AVX-512 and AMX enabled, split across `helpers_*_gen.go` by feature. That is the accepted price of "you get the bytes you asked for" — see Status below, since the generator that writes these hasn't landed and the examples above don't compile yet.

`Emit` is the layer that does work today, for code that builds operands at runtime and doesn't need a named form:

```go
t := a.Section(x86_64.Text)
t.Label("greet", x86_64.Global, x86_64.Func)
t.Emit("mov", x86_64.RAX, x86_64.Mem64(x86_64.RBX).Segment(x86_64.FS).Disp(8))
t.Emit("lea", x86_64.RSI, x86_64.RIPRel(x86_64.Ref("msg", x86_64.R_X86_64_PC32)))
t.Emit("call", x86_64.Ref("puts", x86_64.R_X86_64_PLT32))
t.Emit("ret")
```

`Emit` resolves the shortest legal encoding from `isa.Resolve` and `encode.Encode` against `a.Features()`, breaking ties by table order. `EmitWith` takes the EVEX modifiers alongside the operands:

```go
t.EmitWith(x86_64.Opts{Broadcast: true}, "vpaddd",
	x86_64.ZMM0, x86_64.K1, x86_64.ZMM1, x86_64.Mem512(x86_64.RAX).Disp(64))
```

Zeroing, broadcast, rounding and SAE are one bit each with no register behind them, so they go in `Opts`. A writemask is not one of them: it names a register, the form declares a slot for it in `EVEX.aaa`, and it is passed as an operand — the `K1` above.

Nothing survives either call. A section is a byte buffer and a fixup list.

## Errors are collected, not returned

Builder calls return nothing. The first failure is kept with the section and offset it happened at, every later call is a no-op, and `Serialize` returns it. `Err` reports it early for a caller that wants to stop sooner.

```go
t.MovR64Imm64(x86_64.RAX, 60)
t.Emit("vpaddd", x86_64.ZMM0, x86_64.K1, x86_64.ZMM1, x86_64.ZMM2)  // gated at v1
t.Syscall()                                                          // no-op

_, err := a.Serialize()   // .text+0x7: vpaddd requires avx512f, …
```

This is not error swallowing: the offset in the diagnostic is the one the failing instruction would have been written at, the same place a per-call error would have named. Errors carry one of five categories, for `errors.Is`, derived from the wrapped error rather than stored — so a new error type in `isa/` or `encode/` classifies correctly the moment it is added to one switch and can't disagree with itself in the meantime:

| | |
| --- | --- |
| `ErrFeature` | the form exists and matches; the feature set held it back |
| `ErrForm` | no encoding for these operands, or an operand with no encoding at all |
| `ErrReloc` | a relocation kind with no mapping in this object format |
| `ErrUndefined` | a fixup pointing at a name nothing defines |
| `ErrPlatform` | something this format cannot express |

## Symbols

```go
t.Label("_start", x86_64.Global, x86_64.Func)
t.Label(".L2")                                  // local, no type
a.Declare("puts", x86_64.Global)                // referenced, not defined
a.SetSize("_start", 32)
```

`Label` takes a binding and a type in one variadic list, because `Label("_start", Global, Func)` reads better than two named parameters one of which is almost always the zero value. Bindings are `Local`, `Global`, `Weak` and `Hidden`; types are `None`, `Func`, `Object` and `TLS`. Setting two bindings is the second one winning, what a `.globl` after a `.weak` does in gas.

A symbol's size is closed at `Serialize`: the distance to the next symbol in its section, or to the end of the section for the last one — the same answer `.size f, .-f` computes, and only computable there, since at `Label` time the next symbol hasn't arrived. Only `Func` and `Object` symbols are closed; a bare label is a position, not an extent.

Redefining a name is an error. Two definitions of one symbol is a question the object format cannot record and the linker would answer arbitrarily.

## Data

```go
r := a.Section(x86_64.Rodata)
r.Label("msg", x86_64.Local, x86_64.Object)
r.Ascii("hello, silicon\n")
r.Quad(0x1000)
r.Fill(64, 1, 0)

j := a.Section(x86_64.Data)
j.QuadRef(x86_64.Label("case0"))   // a jump table entry, with its fixup
```

`Byte`, `Word`, `Long` and `Quad` append little-endian integers. The names are gas's and the widths are the architecture's, which is why `Word` is two bytes and the machine is sixty-four bits wide — `.word` predates long mode and neither assembler renamed it.

`LongRef` and `QuadRef` append a symbol's address, recording an absolute fixup: the one place data carries a relocation, and how a jump table or a pointer table is expressed.

A `.bss` section takes `Zero` and nothing else: it has a size and no bytes in the file, so appending content to one is refused rather than written somewhere the format has no room for.

`Align` pads a code section with the canonical multi-byte no-ops and everything else with zeros. The no-ops are the encodings Intel and AMD both document and the ones GNU `as` emits, which matters because a decoder walking a padded section has to find the same instruction boundaries either tool produces. `SetCode` states which a section is, for a name the conventions don't cover.

## Relocations and fixups

A fixup is a field an encoding left blank because its value is an address that is not yet a number. It is not a relocation: a relocation is a format's record with a format's kind and a format's addend convention, and `encode/` knows about no format.

At `Serialize`, a fixup is folded or recorded. It is folded when all four of these hold:

- the field is pc-relative — an absolute address isn't known until the image is laid out, which is `linker/`'s business;
- the symbol is defined in the same section — across sections the distance depends on where the linker puts them;
- the caller named no relocation kind — writing `Ref("f", R_X86_64_PLT32)` asks for a PLT entry, and resolving it to a direct call answers a different question;
- the symbol is local — a global or weak definition can be interposed at link time, so GNU `as` emits a relocation for a call to a global defined two lines above and so does this.

Everything else becomes a relocation record. If the caller named no kind, one is chosen from what the field is for: a branch, a pc-relative data reference, or an absolute address. That mapping is per platform, because "a branch" is `R_X86_64_PLT32` under ELF, `IMAGE_REL_AMD64_REL32` under COFF and `X86_64_RELOC_BRANCH` under Mach-O — three answers to one question, and the question is the only portable part.

### The addend

No `Addend: -4` anywhere. You write the logical addend — the offset from the symbol you meant — and the platform writer converts.

The conversion needs one number the caller doesn't have: how many bytes of the instruction follow the field. A call's displacement ends the instruction, so its tail is zero; the displacement of `mov dword [rip+x], 5` is followed by four bytes of immediate, so its tail is four. The encoder knows because it placed the field, and records it as `Fixup.Tail`.

The three formats then disagree about which address the linker calls P:

| | pc-relative addend | `call puts` | `mov [rip+x], dword 5` |
| --- | --- | --- | --- |
| ELF | `A - (size + tail)` | −4 | −8 |
| COFF | `A - (size + tail)` | −4 | −8 |
| Mach-O | `A - tail` | 0 | −4 |

ELF and COFF compute from the start of the field; Mach-O computes from the end, so the width is already accounted for. COFF reaching the same expression as ELF is not a coincidence — its `REL32` is defined against P+4 and `objectfile/coff` adds that 4 itself, so what it wants handed over is exactly ELF's convention.

### What is wired

Each format's full relocation set is declared in `reloc_*.go`. A kind that is declared but has no mapping in `objectfile/` is refused at `Serialize` with a message naming the gap — not "unknown relocation", which would send you looking for a spelling error that isn't there.

| Platform | Wired end to end | Declared and refused |
| --- | --- | --- |
| ELF | `NONE`, `64`, `PC32`, `PLT32`, `GOTPCREL` | `GOT32`, `COPY`, `GLOB_DAT`, `JUMP_SLOT`, `RELATIVE`, `32`, `32S`, `16`, `PC16`, `8`, `PC8`, the TLS family, `PC64`, `GOTOFF64`, `GOTPC32`, `GOT64`, `GOTPCREL64`, `GOTPC64`, `GOTPLT64`, `PLTOFF64`, `SIZE32`, `SIZE64`, the TLSDESC family, `IRELATIVE`, `RELATIVE64`, `GOTPCRELX`, `REX_GOTPCRELX` |
| COFF | `ADDR64`, `ADDR32`, `ADDR32NB`, `REL32`, `SECREL` | `ABSOLUTE`, the `REL32_1`…`_5` displaced variants, `SECTION`, `SECREL7`, `TOKEN`, `SREL32`, `PAIR`, `SSPAN32` |
| Mach-O | `UNSIGNED`, `BRANCH`, `GOT_LOAD`, `TLV` | `SIGNED`, `GOT`, `SUBTRACTOR`, `SIGNED_1`/`_2`/`_4` |

The Mach-O row has a hole worth knowing about before you target it. `X86_64_RELOC_SIGNED` is how a `%rip`-relative *data* reference is recorded, and `objectfile/macho` has no case that emits it — so `lea rsi, [rip+msg]` assembles for ELF and COFF and is refused for Mach-O. That is the right failure rather than a wrong encoding, but it is a failure on an ordinary instruction. `X86_64_RELOC_SUBTRACTOR` is the other half of the `.quad a - b` gap: it needs a paired record rather than one, the same gap the text path's missing backpatch sits alongside.

Two more format limits, both silent downgrades rather than refusals:

- **COFF `Weak`** links as a strong global. COFF spells weakness with `IMAGE_SYM_CLASS_WEAK_EXTERNAL` and an auxiliary record naming the fallback, and neither is reachable through `objectfile/coff`.
- **`Hidden`** links as a plain global on every format. ELF's `st_other` visibility byte and Mach-O's `N_PEXT` are both unexposed below. Demoting to `Local` instead would be worse — a local symbol is invisible to the linker entirely, and a hidden one is not.

## Flat images

```go
a := x86_64.New(x86_64.Flat)
a.SetBaseAddress(0x7C00)
```

Flat is the platform with the most to refuse, because there is no relocation record and no linker to read one. A fixup that survives folding can never be resolved, and writing zeros where an address belongs produces an image that boots and jumps to nowhere.

So a reference out of its section is refused at `Serialize`, and the message says which of three reasons applies, because they have different fixes: the symbol is undefined, it's in a different section (a distance nothing here can know until layout), or the reference names an absolute address or an explicit relocation kind, neither of which a header-less image has anywhere to put. A `%rip`-relative reference to a bare `Label` already defined in the same section resolves and survives.

`SectionBSS` is zeroes in the file here, unlike the other three formats. There is no loader to zero-fill a reservation — that is what a header is for, and this format has none.

## Encoding and decoding without an object

```go
b, fx, err := x86_64.Encode(x86_64.Baseline(), "mov", x86_64.RAX, x86_64.Imm(60))
in, err := x86_64.Decode(b)
ex, err := x86_64.Explain(b)
```

`Encode` assembles one instruction with no section and no symbol table around it. The fixups come back rather than being resolved, because there is nothing here to resolve them against.

`Decode` reads what the architecture decodes, not what this assembler emits: a redundant prefix, a `disp32` where a `disp8` would have fit, an empty REX where none was needed. Re-encoding those produces the shorter form, a difference `arc fmt` is allowed to make and `arc dis` is not. `Decode`'s operand values are the same concrete types `Encode` accepts, so handing `Ops` and `Form` back to `EncodeForm` returns the bytes it started with — the round trip the differential suite checks on every instruction it generates.

`Explain` is the field-by-field breakdown `arc explain` prints, one line naming a field, its contents, and what it does to the instruction — not "here are some bytes" but "here is why these are the bytes."

## Status

The design is settled; the tree is not finished.

| | |
| --- | --- |
| `feature/`, `reg/`, `operand/`, `isa/`, `encode/`, `decode/`, `text/` | implemented |
| `target.go`, `error.go`, `asm.go`, `code.go`, `reloc*.go`, `write*.go`, `text.go`, `assemble.go` | implemented |
| `helpers_*_gen.go` | unlanded — the generator lives in `internal/gen` |

Until the helpers land, `Emit`/`EmitWith` and text-driven `Assemble` are the instruction paths that work, and the typed calls in "Building an object by hand" above do not compile. `Assemble` itself is complete for everything but three gaps, each refused with a specific error rather than miswritten: `.comm`/`.lcomm`/`.equ` (need an `Env` `Assemble` doesn't carry), `.org` (needs an image-layout step that doesn't exist yet), and a `.quad`/`.long`/`.fill` value that isn't a plain constant or a single added symbol (`. - msg` and similar — the backpatch that would consume `text.Reduce`'s residue isn't wired up). The builder API's `QuadRef`/`LongRef` cover the symbolic-data case directly, so that last gap is in the text path only.