// x86_64/target.go
//
// Package x86_64 is the AMD64 target: registers, ISA tables, encoder,
// decoder, text layer, generated helpers, and the assembler that ties them
// to an object file.
//
// The import is the target declaration. Nothing in this package names an
// arch, because the import path already did — which is why riscv64.MachO is
// a compile error rather than a runtime one, and why there is no Arch type
// here to get wrong.
package x86_64

import (
	"fmt"

	"github.com/vertex-language/arc/x86_64/feature"
	"github.com/vertex-language/arc/x86_64/reg"
	"github.com/vertex-language/arc/x86_64/text"
)

// Platform is the object format, which is also the calling convention: ELF
// and Mach-O are System V, COFF is Win64.
//
// It is not an operating system. This package does not know what Linux is,
// and the OS and vendor fields of an LLVM triple are discarded at the
// boundary before anything here sees them.
//
// The values mirror reg.Platform, which declares its own copy rather than
// importing this package — nothing imports an arch package, including the
// arch package's own subpackages. regPlatform is where the two are pinned
// together, and the compile-time assertions below are what notice if one of
// them is reordered.
type Platform uint8

const (
	ELF Platform = iota
	COFF
	MachO
	Flat
)

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
	return "platform(?)"
}

// Valid reports whether p names a platform this target has.
func (p Platform) Valid() bool { return p <= Flat }

// regPlatform is the same platform in reg/'s vocabulary, for Save().
func (p Platform) regPlatform() reg.Platform {
	switch p {
	case COFF:
		return reg.COFF
	case MachO:
		return reg.MachO
	case Flat:
		return reg.Flat
	}
	return reg.ELF
}

// The two enumerations are written twice and must agree. A reordering of
// either is a build failure here rather than a calling convention that is
// wrong on one platform and right on the others.
const (
	_ = uint8(ELF - Platform(reg.ELF))
	_ = uint8(COFF - Platform(reg.COFF))
	_ = uint8(MachO - Platform(reg.MachO))
	_ = uint8(Flat - Platform(reg.Flat))
)

// ParsePlatform resolves a platform name. The spellings are the ones
// `arc targets` prints and nothing else: an object format has one name.
func ParsePlatform(s string) (Platform, error) {
	switch s {
	case "elf":
		return ELF, nil
	case "coff":
		return COFF, nil
	case "macho":
		return MachO, nil
	case "flat":
		return Flat, nil
	}
	return 0, fmt.Errorf("%w: no platform named %q", ErrPlatform, s)
}

// Dialect is a spelling, never a byte. It is text/'s type rather than a
// copy, because a dialect is a property of the source syntax and the tree is
// where syntax lives.
type Dialect = text.Dialect

const (
	GAS  = text.GAS
	NASM = text.NASM
)

// ParseDialect resolves a dialect name.
func ParseDialect(s string) (Dialect, error) {
	switch s {
	case "gas", "att", "at&t":
		return GAS, nil
	case "nasm", "intel":
		return NASM, nil
	}
	return text.DialectNone, fmt.Errorf("no dialect named %q", s)
}

// ---- the four questions cmd/arc asks every arch package ----------------
//
// `arc targets` is one row per package built from these. They are the
// encoder's own data, so the table cannot drift from what `arc build`
// accepts — which it would the moment the CLI held a copy.

// Platforms is every object format this target writes.
func Platforms() []Platform { return []Platform{ELF, COFF, MachO, Flat} }

// ABIs is empty. The psABI defines no ABI choice for this target and the
// object records none, so `--abi` is a usage error here rather than a
// no-op, and there is no ABI parameter to New.
func ABIs() []string { return nil }

// Dialects is every syntax this target parses and prints.
func Dialects() []Dialect { return []Dialect{GAS, NASM} }

// Baseline is the default feature set: V1, which is SSE2 and nothing above
// it. It is the psABI's floor, not a guess about what silicon is common.
func Baseline() FeatureSet { return feature.Baseline() }

// DefaultFeatures is Baseline, under the name Assemble's callers use.
func DefaultFeatures() FeatureSet { return Baseline() }

// ---- construction ------------------------------------------------------

// config is what an Assembler is built from. Every field is settled at New
// and none of them changes afterward.
type config struct {
	platform Platform
	features FeatureSet

	// base is the load address a Flat image starts at. It is zero and
	// meaningless anywhere else, and SetBaseAddress refuses to set it on a
	// platform that has a header to put it in.
	base uint64

	// macho options
	machoBuild    bool
	machoPlatform MachOPlatform
	machoMajor    uint8
	machoMinor    uint8
}

// Option is a setting read at New.
//
// There is no option that can be applied later. A feature set that changed
// halfway through an object would make a gating diagnostic unfalsifiable —
// the message names the flag that would have allowed the instruction, and
// that claim is only checkable if the set was the same for every byte.
type Option func(*config)

// WithFeatures sets the feature set.
//
// The parameter takes a Level or a Set and nothing else, because those are
// the two things a caller has: V3 is shorthand for a closed set and
// V1.Plus(AVX512F) is a set built from one. A parameter of type any would
// accept a Feature, which is a single extension and not a set, and would
// silently enable one extension where the caller meant a baseline plus one.
//
//	WithFeatures(V3)
//	WithFeatures(V1.Plus(AVX512F))
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

// New builds an assembler for one platform.
//
// It takes only a Platform and options — there is no ABI parameter, because
// this target has no ABI choice for one to name. A platform outside the
// declared set is a programming error and panics: the alternative is an
// error return on a constructor that has nothing else to fail at, and a
// caller who ignored it would get an assembler writing a format nobody
// asked for.
func New(p Platform, opts ...Option) *Assembler {
	if !p.Valid() {
		panic(fmt.Sprintf("x86_64: no platform %d", uint8(p)))
	}
	c := config{platform: p, features: Baseline()}
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return newAssembler(c)
}