// x86_64/text/modifier.go
package text

// Modifier is the relocation modifier a source wrote on a symbol: gas
// spells it `puts@PLT` and NASM spells it `puts wrt ..plt`, and both name
// the same thing.
//
// It is not a RelocKind. The R_X86_64_*, IMAGE_REL_AMD64_* and
// X86_64_RELOC_* constants are declared at the root, and no dialect package
// may import the root — so the tree carries the modifier and the root maps
// it per platform, which is where the mapping belongs anyway.
type Modifier uint8

const (
	ModNone Modifier = iota
	ModPLT
	ModGOT
	ModGOTPCREL
	ModGOTOFF
	ModTPOFF
	ModDTPOFF
	ModTLSGD
	ModTLSLD
	ModSize
)

var modNames = [...]string{
	ModNone: "", ModPLT: "plt", ModGOT: "got", ModGOTPCREL: "gotpcrel",
	ModGOTOFF: "gotoff", ModTPOFF: "tpoff", ModDTPOFF: "dtpoff",
	ModTLSGD: "tlsgd", ModTLSLD: "tlsld", ModSize: "size",
}

func (m Modifier) String() string {
	if int(m) >= len(modNames) {
		return "?"
	}
	return modNames[m]
}