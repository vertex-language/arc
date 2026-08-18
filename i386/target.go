// Package i386 is the Intel386 arch package: registers, ISA, encoder,
// decoder, text layer, and the assembler that ties them to an object file.
//
// This file is target selection — the arch half of the target is the
// import path itself, so New takes what is left: the platform, and the
// feature set by option. See docs/builder.md for the shape this and every
// other arch package share.
package i386

import (
	"github.com/vertex-language/arc/i386/feature"
)

// Platform is an output object format this package can write.
//
// i386 declares ELF, COFF and Flat. There is no MachO here — Mach-O on x86
// is x86_64's platform in the matrix arc targets prints, and a riscv64-style
// "undefined: i386.MachO" is exactly how that miss is meant to fail, at
// compile time rather than as a flag-parsing diagnostic.
type Platform uint8

const (
	ELF Platform = iota
	COFF
	Flat
)

var platformNames = [...]string{"elf", "coff", "flat"}

func (p Platform) String() string {
	if int(p) < len(platformNames) {
		return platformNames[p]
	}
	return "?"
}

// Platforms is every platform this package declares, for arc targets.
func Platforms() []Platform { return []Platform{ELF, COFF, Flat} }

// Dialect selects the text syntax arc fmt and arc build --dialect choose
// between. It has no bearing on New — a dialect is a parser and a printer,
// not a target — and is threaded through separately, in text.go.
type Dialect uint8

const (
	GAS Dialect = iota
	NASM
)

var dialectNames = [...]string{"gas", "nasm"}

func (d Dialect) String() string {
	if int(d) < len(dialectNames) {
		return dialectNames[d]
	}
	return "?"
}

// Dialects is every dialect this package declares.
func Dialects() []Dialect { return []Dialect{GAS, NASM} }

// Baseline is the feature set assembly starts from when --features selects
// nothing. It is feature.Baseline under this package's own name, because
// arc targets prints this value per package and the two must never read
// differently for one target.
const Baseline = feature.I686

// i386 has no ABI parameter. Where an arch has one — riscv64.LP64D,
// arm.HardFloat — New takes it as a second argument; i386 has nothing to
// pass, so New's signature simply has one fewer parameter, and that
// difference is the whole of "i386 supports: (dash)" in the ABI column of
// arc targets.

// New returns an Assembler targeting the given platform, at Baseline unless
// WithFeatures says otherwise.
//
// The target is fixed here and stays fixed: there is no SetFeatures, because
// a feature set that changed mid-object would make every gating diagnostic
// already emitted unfalsifiable.
func New(p Platform, opts ...Option) *Assembler {
	a := &Assembler{platform: p, features: feature.New(Baseline)}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Option configures an Assembler at New. There is deliberately only one
// right now — options are read once, at construction, for the same reason
// there is no SetFeatures.
type Option func(*Assembler)

// WithFeatures sets the feature set assembly is gated against, in place of
// Baseline.
func WithFeatures(s feature.Set) Option {
	return func(a *Assembler) { a.features = s }
}