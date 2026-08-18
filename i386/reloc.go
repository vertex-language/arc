package i386

import "fmt"

// Kind, the platform validity table, and the field-width checks a relocation
// must pass before encode/'s bytes are allowed to carry it.
//
// RelocKind itself is declared in operand/ and re-exported by operand.go —
// carried, not interpreted, by the package below this one — which is what
// lets a SymRef exist under a package that has never heard of ELF or COFF.
// This file is where the numbers this package assigned start meaning
// something: which platform a kind belongs to, how wide a field it needs,
// and whether it is resolved against the end of the instruction or written
// as an absolute value.

type relocInfo struct {
	name  string
	plat  Platform
	size  int
	pcRel bool
}

// relocTable is built by reloc_elf.go's and reloc_coff.go's init functions,
// through addReloc below — one registration site, the same pattern
// table_base.go's add() follows for isa/'s form table, so there is no second
// place a relocation's platform or width could go stale against its
// declaration.
var relocTable = map[RelocKind]relocInfo{}

// addReloc registers one relocation kind and returns it, so a const block
// can read as a declaration and a registration in the same line.
func addReloc(k RelocKind, name string, plat Platform, size int, pcRel bool) RelocKind {
	if _, dup := relocTable[k]; dup {
		panic(fmt.Sprintf("i386: relocation kind %d registered twice (%s)", uint32(k), name))
	}
	relocTable[k] = relocInfo{name: name, plat: plat, size: size, pcRel: pcRel}
	return k
}

// RelocName is the psABI or PE/COFF spelling of k, for a diagnostic. It is a
// function rather than a String method because RelocKind is an alias for a
// type operand/ declares — this package cannot attach methods to it.
func RelocName(k RelocKind) string {
	if info, ok := relocTable[k]; ok {
		return info.name
	}
	return "?"
}

// RelocPlatform is the object format k belongs to.
func RelocPlatform(k RelocKind) (Platform, bool) {
	info, ok := relocTable[k]
	return info.plat, ok
}

// RelocSize is the field width, in bytes, k is defined to write.
func RelocSize(k RelocKind) (int, bool) {
	info, ok := relocTable[k]
	return info.size, ok
}

// RelocPCRelative reports whether k is resolved against the address of the
// next instruction — write.go's Adjust — rather than written as an
// absolute value.
func RelocPCRelative(k RelocKind) (bool, bool) {
	info, ok := relocTable[k]
	return info.pcRel, ok
}

// validateReloc checks that k belongs to p and fits the field encode/
// already placed it in. This is the check that turns
//
//	t.CallRel32(x86_64.Ref("puts", x86_64.IMAGE_REL_AMD64_REL32))
//
// against an ELF target into "IMAGE_REL_AMD64_REL32 is a COFF relocation;
// target is i386-elf" rather than a generic type mismatch — the diagnostic
// docs/builder.md shows for exactly this call.
func validateReloc(p Platform, k RelocKind, fieldSize int) *Error {
	info, ok := relocTable[k]
	if !ok {
		return &Error{Err: ErrReloc, msg: fmt.Sprintf("%d is not a relocation kind i386 declares", uint32(k))}
	}
	if info.plat != p {
		return relocErr("", 0, fmt.Sprintf("%s is a %s relocation; target is %s", info.name, info.plat, p))
	}
	if fieldSize != 0 && info.size != fieldSize {
		return relocErr("", 0, fmt.Sprintf("%s does not fit a %d-byte field", info.name, fieldSize))
	}
	return nil
}