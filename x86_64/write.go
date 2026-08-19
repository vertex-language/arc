// x86_64/write.go
//
// The platform-independent half of serialisation: closing symbol sizes,
// folding the fixups that can be folded, and turning the rest into the
// (kind, symbol, addend) triples a platform writer records.
//
// What is not here is any format's vocabulary. The four write_*.go files
// own that, because R_X86_64_PLT32, IMAGE_REL_AMD64_REL32 and
// X86_64_RELOC_BRANCH are three answers to one question and the question is
// the only portable part.
package x86_64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"github.com/vertex-language/arc/x86_64/encode"
)

// Serialize builds the object file.
//
// It is safe to call more than once: every call works from a copy of each
// section's bytes, so the folding below never mutates the Assembler. An
// Assembler that recorded an error earlier returns it here — the builder
// API collects rather than returns, and this is where the collection is
// handed back.
func (a *Assembler) Serialize() ([]byte, error) {
	if a.err != nil {
		return nil, a.err
	}
	a.closeSymbolSizes()

	switch a.cfg.platform {
	case ELF:
		return a.serializeELF()
	case COFF:
		return a.serializeCOFF()
	case MachO:
		return a.serializeMachO()
	case Flat:
		return a.serializeFlat()
	}
	return nil, platformErr(a.cfg.platform, "no writer")
}

// WriteTo is the io.WriterTo form of Serialize.
func (a *Assembler) WriteTo(w io.Writer) (int64, error) {
	b, err := a.Serialize()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	return int64(n), err
}

// ---- symbol sizes ------------------------------------------------------

// closeSymbolSizes fills in the extent of every function and object symbol
// that did not state one.
//
// A symbol's size is the distance to the next symbol in its section, or to
// the end of the section for the last one. That is the same answer gas
// computes when a `.size f, .-f` directive names the location counter, and
// it is only computable here: at Label time the next symbol has not
// arrived, and the caller who knows the answer already has SetSize.
//
// Only Func and Object symbols are closed. A bare label is a position and
// not an extent — giving `.L2` a size because `.L3` happens to follow it
// would put a span in the symbol table that nothing in the source claimed.
func (a *Assembler) closeSymbolSizes() {
	for _, s := range a.sections {
		defined := make([]*Symbol, 0, len(s.syms))
		for _, sym := range s.syms {
			if sym.Defined {
				defined = append(defined, sym)
			}
		}
		// Stable by offset: two symbols at one offset keep definition
		// order, so the earlier one is the one that gets the extent.
		sort.SliceStable(defined, func(i, j int) bool {
			return defined[i].Offset < defined[j].Offset
		})

		for i, sym := range defined {
			if sym.HasSize || (sym.Type != Func && sym.Type != Object) {
				continue
			}
			end := len(s.data)
			for j := i + 1; j < len(defined); j++ {
				if defined[j].Offset > sym.Offset {
					end = defined[j].Offset
					break
				}
			}
			sym.Size = int64(end - sym.Offset)
			sym.HasSize = true
		}
	}
}

// ---- fixups ------------------------------------------------------------

// resolvedFixup is a fixup that survived folding: everything a platform
// writer needs to emit one relocation record, in that format's own terms
// except for the addend, which each writer converts last.
type resolvedFixup struct {
	off    int
	size   int
	tail   int
	pcrel  bool
	kind   RelocKind
	symbol string

	// addend is still the logical one — the offset from the symbol the
	// caller meant, never adjusted for the width or position of the field.
	// elfAddend, coffAddend and machoAddend are where it stops being that,
	// and they disagree, which is the whole reason it is carried this far
	// unconverted.
	addend int64
}

// prepare returns a section's bytes with every foldable reference resolved,
// and the fixups that remain.
//
// The copy is not an optimisation to skip: folding writes displacements
// into the buffer, and an Assembler whose Serialize mutated its own
// sections would produce different bytes the second time it was called.
func (a *Assembler) prepare(s *Section) ([]byte, []resolvedFixup, error) {
	code := make([]byte, len(s.data))
	copy(code, s.data)

	out := make([]resolvedFixup, 0, len(s.fixups))
	for _, f := range s.fixups {
		name := f.target.SymName()
		sym := a.symsBy[name]

		if a.foldable(s, f, sym) {
			if err := foldPCRel(code, string(s.name), f, sym); err != nil {
				return nil, nil, err
			}
			continue
		}

		kind := f.kind
		if kind == RelocNone {
			k, ok := defaultReloc(f.use, f.size, a.cfg.platform)
			if !ok {
				return nil, nil, &Error{
					Section: string(s.name), Offset: f.off, HasOff: true,
					Err: fmt.Errorf("%w: no %s relocation records a %d-byte %s field",
						ErrReloc, a.cfg.platform, f.size, f.use),
				}
			}
			kind = k
		}
		if err := checkFixup(string(s.name), f, a.cfg.platform); err != nil {
			return nil, nil, err
		}
		// checkFixup validated the caller's kind; the chosen one needs the
		// same treatment, and passing it back through is cheaper than a
		// second code path that could disagree.
		if err := checkFixup(string(s.name), fixup{off: f.off, size: f.size, kind: kind}, a.cfg.platform); err != nil {
			return nil, nil, err
		}

		out = append(out, resolvedFixup{
			off:    f.off,
			size:   f.size,
			tail:   f.tail,
			pcrel:  f.pcrel,
			kind:   kind,
			symbol: name,
			addend: f.addend,
		})
	}
	return code, out, nil
}

// foldable reports whether a reference can be resolved here rather than by
// the linker.
//
// Four conditions, and each one of them is load-bearing:
//
//   - The field is pc-relative. An absolute address is not known until the
//     image is laid out, which is linker/'s business and not this package's.
//   - The symbol is defined in this same section. Across sections the
//     distance depends on where the linker puts them.
//   - The caller named no relocation kind. Writing `Ref("f", R_X86_64_PLT32)`
//     asks for a PLT entry, and quietly resolving it to a direct call is
//     answering a different question.
//   - The symbol is local. A global or weak definition can be interposed at
//     link time — the whole point of interposition is that the definition
//     in this object may not be the one that wins — so GNU as emits a
//     relocation for a call to a global even when it is defined two lines
//     above, and so does this.
func (a *Assembler) foldable(s *Section, f fixup, sym *Symbol) bool {
	return f.pcrel &&
		f.kind == RelocNone &&
		sym != nil &&
		sym.Defined &&
		sym.Section == s &&
		sym.Binding == Local
}

// foldPCRel writes the displacement to a symbol in the same section.
//
// The displacement is from the end of the instruction, and the end of the
// instruction is the end of the field plus whatever follows it — which is
// Tail, computed by the encoder because the encoder placed the field.
func foldPCRel(code []byte, section string, f fixup, sym *Symbol) error {
	disp := int64(sym.Offset) - int64(f.off+f.size+f.tail) + f.addend

	if !fitsSigned(disp, f.size) {
		return &Error{
			Section: section, Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s is %d bytes away and the displacement field is %d bytes",
				ErrForm, sym.Name, disp, f.size),
		}
	}
	if f.off+f.size > len(code) {
		return &Error{
			Section: section, Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: fixup field runs past the end of the section", ErrForm),
		}
	}

	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(disp))
	copy(code[f.off:f.off+f.size], b[:f.size])
	return nil
}

func fitsSigned(v int64, size int) bool {
	switch size {
	case 1:
		return v >= -128 && v <= 127
	case 2:
		return v >= -32768 && v <= 32767
	case 4:
		return v >= -2147483648 && v <= 2147483647
	case 8:
		return true
	}
	return false
}

// ---- shared writer helpers ---------------------------------------------

// undefinedRefs is every symbol a fixup points at that this object neither
// defines nor declares.
//
// For ELF, COFF and Mach-O this is not an error — an undefined symbol is
// what a relocation against an external name is for, and the format has a
// slot for it. Flat is the exception, because there is no relocation record
// and no linker to read one, and that check lives in write_flat.go.
func (a *Assembler) undefinedRefs() []string {
	var out []string
	for _, sym := range a.syms {
		if !sym.Defined {
			out = append(out, sym.Name)
		}
	}
	return out
}

// checkNonEmpty rejects an object with nothing in it. Every format can
// express an empty object and none of them is useful, and the caller who
// built one has almost certainly not noticed that a builder call failed
// earlier — which, since the failure is already recorded, it has.
func (a *Assembler) checkNonEmpty() error {
	if len(a.sections) == 0 {
		return &Error{Err: fmt.Errorf("%w: the object has no sections", ErrForm)}
	}
	return nil
}

// buffer is the shape every writer returns through, so Serialize's contract
// is one shape rather than four.
func serialized(b []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return b, nil
}

var _ = bytes.MinRead // the writers below share this file's imports
var _ = encode.UseAbs