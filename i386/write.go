package i386

import (
	"fmt"
	"io"
	"sort"

	"github.com/vertex-language/arc/i386/encode"
)

// Serialize, WriteTo, and the one pass every platform shares before its own
// writer runs: resolving same-section fixups into bytes, and closing symbol
// sizes at the next symbol or the end of the section.
//
// What is platform-specific — turning a cross-section fixup into a
// relocation record, naming and flagging sections, writing the header —
// lives in write_elf.go, write_coff.go and write_flat.go, one file per
// format, the same split docs/builder.md describes for every other arch
// package's write.go.

// Serialize assembles every section into a complete object file for
// a.Platform(). Deterministic: identical calls on an Assembler nothing has
// mutated since produce identical bytes, because nothing here reads a clock
// or an environment variable of its own — objectfile/elf's, objectfile/coff's
// and objectfile/flat's own determinism is inherited, not re-implemented.
func (a *Assembler) Serialize() ([]byte, error) {
	if a.err != nil {
		return nil, a.err
	}
	if err := a.resolveLabelFixups(); err != nil {
		return nil, err
	}

	switch a.platform {
	case ELF:
		return a.serializeELF()
	case COFF:
		return a.serializeCOFF()
	case Flat:
		return a.serializeFlat()
	}
	return nil, fmt.Errorf("i386: unknown platform %s", a.platform)
}

// WriteTo is the io.WriterTo form of Serialize.
func (a *Assembler) WriteTo(w io.Writer) (int64, error) {
	b, err := a.Serialize()
	if err != nil {
		return 0, err
	}
	n, werr := w.Write(b)
	return int64(n), werr
}

// resolveLabelFixups patches every same-section fixup (encode.FixupLabel —
// a bare Label operand, per operand/'s own doc "resolves at Serialize as a
// direct fixup with no relocation record") directly into its section's
// bytes, and drops it from the section's fixup list. What remains after
// this pass is exactly the fixups that need to become relocation records —
// or, on a Flat target, the fixups that have nowhere to go at all and get
// refused by write_flat.go's own check — which is the same set regardless
// of which platform writes them next.
func (a *Assembler) resolveLabelFixups() error {
	for _, s := range a.sections {
		var remaining []fixupEntry
		for _, fx := range s.fixups {
			if fx.kind != encode.FixupLabel {
				remaining = append(remaining, fx)
				continue
			}
			target, ok := s.marks[fx.name]
			if !ok {
				return undefinedErr(s.Name, fx.name)
			}
			// See i386/encode.Fixup's own doc: the field is resolved
			// against the address one past its end, and Adjust already
			// carries that correction — so the value written is simply
			// target - fieldOffset + Adjust, no instruction-length lookup
			// needed here.
			rel := int64(target) - int64(fx.offset) + int64(fx.adjust)
			if err := patchLE(s.bytes, fx.offset, fx.size, rel); err != nil {
				return relocErr(s.Name, uint32(fx.offset), err.Error())
			}
		}
		s.fixups = remaining
	}
	return nil
}

func patchLE(b []byte, offset, size int, v int64) error {
	if offset < 0 || size < 0 || offset+size > len(b) {
		return fmt.Errorf("fixup at %#x runs past the section", offset)
	}
	lo, hi := signedRange(size)
	if v < lo || v > hi {
		return fmt.Errorf("branch displacement %d does not fit a %d-byte field", v, size)
	}
	for i := 0; i < size; i++ {
		b[offset+i] = byte(v >> (8 * uint(i)))
	}
	return nil
}

func signedRange(size int) (int64, int64) {
	if size <= 0 || size >= 8 {
		return 0, 0
	}
	bits := uint(size) * 8
	return -(int64(1) << (bits - 1)), int64(1)<<(bits-1) - 1
}

// closeSymbolSizes computes each symbol's size as the distance to the next
// symbol in offset order, or to the end of the section for the last one —
// .type/.size pairing without the two directives, exactly as asm.go's Label
// doc promises. It is platform-agnostic, so every write_*.go shares it
// rather than each closing sizes its own way. write_flat.go does not call
// this — a flat image has no symbol table for a size to describe.
func closeSymbolSizes(s *Section) map[string]uint32 {
	type entry struct {
		name string
		off  uint32
	}
	entries := make([]entry, 0, len(s.symbols))
	for name, sym := range s.symbols {
		entries = append(entries, entry{name, sym.offset})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].off != entries[j].off {
			return entries[i].off < entries[j].off
		}
		return entries[i].name < entries[j].name // deterministic among ties
	})

	sizes := make(map[string]uint32, len(entries))
	end := uint32(len(s.bytes))
	for i, e := range entries {
		next := end
		if i+1 < len(entries) {
			next = entries[i+1].off
		}
		sizes[e.name] = next - e.off
	}
	return sizes
}