// Package aarch64 assembles, encodes, decodes and writes AArch64 machine code.
//
// The import is the target declaration. Nothing in a file using this package
// names an architecture a second time: New takes what is left of the target
// after the arch is fixed, which here is a platform and some options.
//
// A platform is an object format, not an operating system. This package does
// not know what Linux is; the OS and vendor fields of an LLVM triple are
// discarded at the boundary before anything here sees them.
package aarch64

import (
	"fmt"

	"github.com/vertex-language/arc/aarch64/feature"
)

// Platform is the object format an assembler writes.
type Platform uint8

const (
	// ELF is a full ET_REL relocatable object targeting EM_AARCH64.
	ELF Platform = iota

	// COFF is a full Win64 object targeting IMAGE_FILE_MACHINE_ARM64.
	COFF

	// MachO is a full MH_OBJECT file targeting CPU_TYPE_ARM64.
	MachO

	// Flat is a raw concatenation of sections in creation order: no header,
	// no symbol table, and nowhere to put a relocation.
	Flat

	platformCount
)

// Valid reports whether p names a platform.
func (p Platform) Valid() bool { return p < platformCount }

func (p Platform) String() string {
	switch p {
	case ELF:
		return "elf"
	case COFF:
		return "coff"
	case MachO:
		return "macho"
	case Flat:
		return "flat"
	}
	return "invalid"
}

// Relocatable reports whether the format records relocations.
//
// Flat is the one that does not, and it is the reason this question has a
// method rather than being a comparison at each call site: everything the flat
// writer refuses, it refuses because the answer here is false.
func (p Platform) Relocatable() bool { return p != Flat }

// ParsePlatform resolves a platform name, for the half of `-t` that is not the
// arch.
func ParsePlatform(name string) (Platform, error) {
	switch name {
	case "elf":
		return ELF, nil
	case "coff":
		return COFF, nil
	case "macho":
		return MachO, nil
	case "flat", "bin", "binary":
		return Flat, nil
	}
	return 0, fmt.Errorf("aarch64: unknown platform %q", name)
}

// Platforms is every platform this package writes, which is what `arc targets`
// prints in the PLATFORMS column.
//
// It is the encoder's own data rather than a table in cmd/arc, so the column
// cannot drift from what `arc build` accepts.
func Platforms() []Platform { return []Platform{ELF, MachO, COFF, Flat} }

// ABIs is empty, and the emptiness is the statement.
//
// An ABI knob exists only where the psABI defines a choice and the object
// records it. AArch64 has neither: there is one procedure call standard, and
// ILP32 is not a variant of this target but a different one — a different ELF
// class and a different relocation namespace, R_AARCH64_P32_*. `--abi` on an
// aarch64 target is a usage error naming that, not a no-op.
func ABIs() []string { return nil }

// Dialects is empty for the same kind of reason.
//
// There is one A64 syntax. NASM has no A64 grammar to accept and inventing one
// would be inventing syntax, so `--dialect` here is a usage error rather than
// something silently ignored.
//
// What does vary is modifier spelling — :lo12: against @PAGEOFF — and that
// varies by platform rather than by dialect, because the bytes are identical
// and the two spellings name one thing. Treating it as a dialect would let a
// caller ask for @PAGEOFF in an ELF object, which is not a preference but a
// file no assembler on that platform will read back.
func Dialects() []string { return nil }

// Baseline is the default feature set: Armv8-A, floating point and Advanced
// SIMD and nothing above them.
func Baseline() FeatureSet { return feature.Baseline() }

// BaselineName is what `arc targets` prints in the BASELINE column.
func BaselineName() string { return Baseline().String() }

// Option configures an assembler at New.
//
// Options are read once, at construction. There is no SetFeatures and no
// SetPlatform: a feature set that changed halfway through an object would make
// a gating diagnostic unfalsifiable, since the flag it names would not have
// been the flag in effect when the instruction was refused.
type Option func(*config)

type config struct {
	features FeatureSet
}

// WithFeatures sets the active feature set.
//
// The parameter accepts a Level or a FeatureSet and nothing else. A parameter
// of type any would also accept a bare Feature, which is one extension rather
// than a set, and WithFeatures(LSE) meaning "LSE and no floating point" is not
// what anyone writing it intends.
func WithFeatures[T Level | FeatureSet](f T) Option {
	return func(c *config) {
		switch v := any(f).(type) {
		case Level:
			c.features = v.Set()
		case FeatureSet:
			c.features = v
		}
	}
}

// New starts an assembler for a platform.
//
// There is no ABI parameter and no dialect parameter, for the reasons ABIs and
// Dialects give. Big-endian is likewise not a knob: endianness is part of an
// arch name, and aarch64_be is not one of the nine.
func New(p Platform, opts ...Option) *Assembler {
	c := config{features: Baseline()}
	for _, o := range opts {
		o(&c)
	}
	return newAssembler(p, c)
}

// DefaultFeatures is Baseline, named so a caller passing it to Assemble reads
// as having chosen it rather than having left a parameter blank.
func DefaultFeatures() FeatureSet { return Baseline() }