// x86_64/reloc.go
//
// The relocation vocabulary: the kinds each object format defines, which of
// them this tree can actually write, and the mapping from a source's
// relocation modifier to a kind.
//
// The constants live in reloc_elf.go, reloc_coff.go and reloc_macho.go, one
// file per format, and register themselves here. This file holds only what
// is common: the type, the registry, and the four questions everything else
// asks about a kind.
package x86_64

import (
	"fmt"
	"strings"

	"github.com/vertex-language/arc/x86_64/encode"
	"github.com/vertex-language/arc/x86_64/operand"
	"github.com/vertex-language/arc/x86_64/text"
)

// RelocKind is a relocation kind.
//
// The type is declared in operand/, because a SymRef has to carry one and
// nothing below this package may import it. The constants are declared here,
// because they are the format's and the formats are this package's business.
type RelocKind = operand.RelocKind

// RelocNone is the zero kind: "the encoder picks". A Ref built without an
// explicit kind gets this, and the platform writer chooses from the field's
// Use — a call site gets a branch relocation, a %rip-relative load gets a
// pc-relative one, and naming a kind overrides that.
const RelocNone = operand.RelocNone

// The three formats number their relocations from zero and they collide:
// R_X86_64_64 is 1, IMAGE_REL_AMD64_ADDR64 is 1, and X86_64_RELOC_SIGNED is
// 1. One RelocKind space holding all three needs them apart, so the format
// is a tag in the high byte and the format's own number is the low one.
//
// This is why RelocRaw exists: the writers need the number the format
// actually records, and it is not the constant's value.
const (
	relocELF   = 0x0100
	relocCOFF  = 0x0200
	relocMachO = 0x0300

	relocFormatMask = 0xff00
	relocValueMask  = 0x00ff
)

// relocInfo is everything this package knows about one kind.
type relocInfo struct {
	name     string
	platform Platform

	// raw is the number the object format records, which is the constant
	// with its format tag stripped.
	raw uint16

	// size is the field width in bytes. A relocation whose size disagrees
	// with the field the encoder placed is a relocation that patches the
	// wrong bytes, and checkFixup is where the two are compared.
	size int

	// pcrel is whether the format computes this relocation relative to the
	// place it patches.
	pcrel bool

	// mapped is whether objectfile/ can write this kind today.
	//
	// A kind that is declared and unmapped is a gap, not a mistake: the
	// reloc_*.go files declare each format's full set for completeness so
	// that a caller naming one gets a diagnostic saying the mapping is
	// missing, rather than "unknown relocation" — which would send them
	// looking for a spelling error that is not there.
	mapped bool
}

var relocs = map[RelocKind]relocInfo{}
var relocsByName = map[string]RelocKind{}

// registerReloc is called from each format's init.
func registerReloc(k RelocKind, info relocInfo) {
	if prev, dup := relocs[k]; dup {
		panic(fmt.Sprintf("x86_64: relocation %#x is both %s and %s",
			uint16(k), prev.name, info.name))
	}
	if prev, dup := relocsByName[info.name]; dup {
		panic(fmt.Sprintf("x86_64: two relocations named %s (%#x and %#x)",
			info.name, uint16(prev), uint16(k)))
	}
	relocs[k] = info
	relocsByName[info.name] = k
}

// RelocName is the kind's spelling in its format's own vocabulary:
// R_X86_64_PLT32, IMAGE_REL_AMD64_REL32, X86_64_RELOC_BRANCH.
func RelocName(k RelocKind) string {
	if k == RelocNone {
		return "none"
	}
	if info, ok := relocs[k]; ok {
		return info.name
	}
	return fmt.Sprintf("reloc(%#x)", uint16(k))
}

// RelocPlatform is the object format this kind belongs to.
func RelocPlatform(k RelocKind) (Platform, bool) {
	info, ok := relocs[k]
	return info.platform, ok
}

// RelocSize is the width in bytes of the field this kind patches, or zero
// for an unknown kind.
func RelocSize(k RelocKind) int { return relocs[k].size }

// RelocPCRel reports whether the format computes this kind relative to the
// place it patches.
func RelocPCRel(k RelocKind) bool { return relocs[k].pcrel }

// RelocRaw is the number the object format records. It is not the constant's
// value: the constant carries a format tag so the three vocabularies can
// share one type without colliding.
func RelocRaw(k RelocKind) uint16 { return uint16(k) & relocValueMask }

// RelocMapped reports whether this tree can write the kind today. A declared
// but unmapped kind is refused at Serialize with a note naming the gap.
func RelocMapped(k RelocKind) bool { return relocs[k].mapped }

// ValidReloc reports whether the kind belongs to the platform. This is the
// validity table: a relocation is valid where its format is and nowhere
// else, so `Ref("puts", R_X86_64_PLT32)` in a COFF object is caught here
// rather than written as a number COFF reads as something different.
func ValidReloc(k RelocKind, p Platform) bool {
	if k == RelocNone {
		return true
	}
	info, ok := relocs[k]
	return ok && info.platform == p
}

// Relocs is every kind declared for a platform, in numeric order of the
// format's own numbering. `arc targets --relocs` prints this.
func Relocs(p Platform) []RelocKind {
	var out []RelocKind
	for k, info := range relocs {
		if info.platform == p {
			out = append(out, k)
		}
	}
	sortRelocs(out)
	return out
}

// ParseReloc resolves a kind by name, in any case.
func ParseReloc(s string) (RelocKind, error) {
	if k, ok := relocsByName[strings.ToUpper(strings.TrimSpace(s))]; ok {
		return k, nil
	}
	return RelocNone, fmt.Errorf("%w: no relocation named %q", ErrReloc, s)
}

func sortRelocs(ks []RelocKind) {
	for i := 1; i < len(ks); i++ {
		for j := i; j > 0 && ks[j] < ks[j-1]; j-- {
			ks[j], ks[j-1] = ks[j-1], ks[j]
		}
	}
}

// ---- modifiers ---------------------------------------------------------

// modKey is a source's relocation modifier under one platform.
type modKey struct {
	platform Platform
	mod      text.Modifier
}

var modifierRelocs = map[modKey]RelocKind{}

// registerModifier maps a dialect-neutral modifier to a kind, from each
// format's init.
//
// gas writes `puts@PLT` and NASM writes `puts wrt ..plt`; both parse to
// text.ModPLT, which is neither format's vocabulary and deliberately so.
// This is where it becomes one — at the root, which is the only place that
// knows both the modifier and the constants, and per platform, because the
// same modifier is a different number in each of the three.
func registerModifier(p Platform, m text.Modifier, k RelocKind) {
	modifierRelocs[modKey{p, m}] = k
}

// RelocForModifier is the kind a source's modifier asks for on this
// platform. ModNone answers RelocNone, which leaves the choice to the field.
func RelocForModifier(m text.Modifier, p Platform) (RelocKind, error) {
	if m == text.ModNone {
		return RelocNone, nil
	}
	if k, ok := modifierRelocs[modKey{p, m}]; ok {
		return k, nil
	}
	return RelocNone, fmt.Errorf("%w: %s has no meaning on %s", ErrReloc, m, p)
}

// ---- checking ----------------------------------------------------------

// checkFixup is the validation every platform writer runs before mapping a
// fixup to a relocation record.
//
// It asks three questions in the order that gives the most useful answer:
// does the kind belong to this format, can this tree write it, and does its
// field width match the field the encoder actually placed. The third is the
// one that would otherwise fail silently — a four-byte relocation over an
// eight-byte field patches half of it and leaves the rest as whatever the
// encoder wrote, which is zero, which is a working program until the symbol
// lands above 4GB.
func checkFixup(section string, f fixup, p Platform) error {
	if f.kind == RelocNone {
		return nil
	}
	if !ValidReloc(f.kind, p) {
		owner, known := RelocPlatform(f.kind)
		if !known {
			return &Error{
				Section: section, Offset: f.off, HasOff: true,
				Err: fmt.Errorf("%w: %s is not a relocation kind", ErrReloc, RelocName(f.kind)),
			}
		}
		return &Error{
			Section: section, Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s is %s's, and this object is %s",
				ErrReloc, RelocName(f.kind), owner, p),
		}
	}
	if !RelocMapped(f.kind) {
		return relocErr(section, f.off, f.kind, p)
	}
	if n := RelocSize(f.kind); n != 0 && n != f.size {
		return &Error{
			Section: section, Offset: f.off, HasOff: true,
			Err: fmt.Errorf("%w: %s patches %d bytes and the field is %d",
				ErrReloc, RelocName(f.kind), n, f.size),
		}
	}
	return nil
}

// defaultReloc is the kind for a fixup whose caller named none, chosen from
// what the field is for.
//
// The mapping is per platform and lives in each reloc_*.go, because "a
// branch" is R_X86_64_PLT32 under ELF, IMAGE_REL_AMD64_REL32 under COFF and
// X86_64_RELOC_BRANCH under Mach-O — three answers to one question, and the
// question is the only part that is portable.
var defaultRelocs = map[struct {
	platform Platform
	use      encode.Use
}]RelocKind{}

func registerDefault(p Platform, u encode.Use, k RelocKind) {
	defaultRelocs[struct {
		platform Platform
		use      encode.Use
	}{p, u}] = k
}

func defaultReloc(u encode.Use, size int, p Platform) (RelocKind, bool) {
	k, ok := defaultRelocs[struct {
		platform Platform
		use      encode.Use
	}{p, u}]
	if !ok {
		return RelocNone, false
	}
	// An absolute field is the one case where the width picks the kind:
	// eight bytes is a full address and four is the truncated form, and
	// they are different relocations in every format.
	if u == encode.UseAbs && size != RelocSize(k) {
		if alt, ok := absRelocs[struct {
			platform Platform
			size     int
		}{p, size}]; ok {
			return alt, true
		}
		return RelocNone, false
	}
	return k, true
}

var absRelocs = map[struct {
	platform Platform
	size     int
}]RelocKind{}

func registerAbs(p Platform, size int, k RelocKind) {
	absRelocs[struct {
		platform Platform
		size     int
	}{p, size}] = k
}