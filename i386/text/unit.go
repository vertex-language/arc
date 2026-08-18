package text

import "github.com/vertex-language/arc/i386/reg"

// Unit is what a .s file denotes: a flat sequence of statements in source
// order.
//
// Flat rather than grouped by section, because a .s file may leave a section
// and come back to it — .text, .data, .text is three statements and two
// sections, and grouping them at parse would reorder the file. Assemble does
// the grouping, where it is a consequence of emitting bytes rather than a
// property of the text. Sections and Symbols below are derived by walking,
// so they cannot drift from the nodes.
//
// The unit is the meeting point of the two dialects and holds nothing either
// of them cannot say. There is no field here for a construct one dialect has
// and the other cannot express, because such a field would be a round trip
// that silently loses information.
type Unit struct {
	// File is the name diagnostics carry.
	File string

	// Nodes are the statements, in source order.
	Nodes []Node

	// Tail is trivia after the last statement: a file that ends in a comment
	// block ends in a comment block after arc fmt too.
	Tail Trivia
}

// Add appends a statement.
func (u *Unit) Add(n Node) { u.Nodes = append(u.Nodes, n) }

// SectionRef is one section the unit names, at its first appearance.
type SectionRef struct {
	Kind  SectionKind
	Name  string
	Decl  *SectionDecl
	Index int // position in Nodes
}

// Sections returns the sections in first-appearance order, deduplicated by
// name. This is the order Assemble creates them in, and it is the order the
// builder API documents: sections are emitted in the order first created.
func (u *Unit) Sections() []SectionRef {
	var out []SectionRef
	seen := make(map[string]bool)
	for i, n := range u.Nodes {
		d, ok := n.(*SectionDecl)
		if !ok || seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		out = append(out, SectionRef{Kind: d.Kind, Name: d.Name, Decl: d, Index: i})
	}
	return out
}

// SymbolRef is what the unit says about one name.
//
// Defined and Attrs are separate questions and a name may answer only one of
// them: `.globl puts` with no `puts:` is an undefined global, which is how a
// call to a shared library is written, and `msg:` with no `.globl` is a
// defined local.
type SymbolRef struct {
	Name    string
	Defined bool
	Attrs   Attr
	Type    SymbolType
	Pos     Pos
}

// Symbols returns every name the unit defines or declares, in first-appearance
// order.
func (u *Unit) Symbols() []SymbolRef {
	var out []SymbolRef
	at := make(map[string]int)

	touch := func(name string, p Pos) *SymbolRef {
		if i, ok := at[name]; ok {
			return &out[i]
		}
		at[name] = len(out)
		out = append(out, SymbolRef{Name: name, Pos: p})
		return &out[len(out)-1]
	}

	for _, n := range u.Nodes {
		switch d := n.(type) {
		case *Label:
			touch(d.Name, d.P).Defined = true
		case *SymbolDecl:
			for _, name := range d.Names {
				s := touch(name, d.P)
				s.Attrs |= d.Attrs
				if d.Type != TypeNone {
					s.Type = d.Type
				}
			}
		case *Equ:
			touch(d.Name, d.P).Defined = true
		}
	}
	return out
}

// Equates returns the constant definitions, as a lookup for Eval.
//
// An .equ whose right-hand side is not absolute is not a constant and is not
// returned; it is a symbol alias, and resolving it is the assembler's because
// it needs section offsets this package does not have.
func (u *Unit) Equates() map[string]int64 {
	out := make(map[string]int64)
	for _, n := range u.Nodes {
		e, ok := n.(*Equ)
		if !ok {
			continue
		}
		v, err := Eval(e.Value, func(name string) (int64, bool) {
			c, ok := out[name]
			return c, ok
		})
		if err == nil && v.Kind() == Absolute {
			out[e.Name] = v.Const
		}
	}
	return out
}

// SectionKind is the portable meaning of a section.
//
// The nine kinds are the ones every arch package declares, and the values are
// pinned to the arch package's own constants: the root translates by cast,
// with one round-trip test standing in for the switch that would otherwise be
// copied. This enum exists here rather than being imported because text/
// cannot import its parent — the parent imports it.
//
// Custom is not a fallback for a section arc failed to classify. It means the
// source named a section arc has no portable meaning for, and the name and
// flags pass through verbatim, which is the same promise SectionNamed makes
// in the builder API.
type SectionKind uint8

const (
	SectionText SectionKind = iota
	SectionData
	SectionROData
	SectionBSS
	SectionUnwind
	SectionInitArray
	SectionFiniArray
	SectionTLS
	SectionCustom
)

var sectionKindNames = [...]string{
	"text", "data", "rodata", "bss", "unwind", "init_array", "fini_array", "tls", "custom",
}

func (k SectionKind) String() string {
	if int(k) < len(sectionKindNames) {
		return sectionKindNames[k]
	}
	return "?"
}

// Code reports whether the kind holds instructions. Align pads a code section
// with the arch's nop sequence and a data section with zeros, which is the
// one place this distinction changes bytes.
func (k SectionKind) Code() bool { return k == SectionText }

// Attr is a symbol attribute set.
//
// Binding and visibility are separate bits rather than one enum because the
// source states them separately — `.globl foo` and `.hidden foo` are two
// directives — and collapsing them would make printing back a guess.
type Attr uint16

const (
	AttrGlobal Attr = 1 << iota
	AttrLocal
	AttrWeak
	AttrHidden
	AttrProtected
	AttrInternal

	// AttrExtern is the one attribute the two dialects disagree about the
	// need for. NASM requires `extern puts` before a reference; GNU as treats
	// an undefined name as external and accepts `.extern` as documentation.
	// The unit records it either way, so a GAS file that omits it still
	// prints as NASM that has it — synthesised by the printer from the
	// undefined names, not invented here.
	AttrExtern
)

// SymbolType is the .type of a symbol: what the object file records it as.
type SymbolType uint8

const (
	TypeNone SymbolType = iota
	TypeFunc
	TypeObject
	TypeTLS
)

var symbolTypeNames = [...]string{"", "function", "object", "tls_object"}

func (t SymbolType) String() string {
	if int(t) < len(symbolTypeNames) {
		return symbolTypeNames[t]
	}
	return "?"
}

// Prefix is a legacy instruction prefix written as a separate mnemonic.
//
// These are prefixes and not instructions: both dialects write `lock cmpxchg`
// and `rep movsb` as two words, and the encoder emits one byte in front of
// the instruction it modifies. The segment override is not here — it is
// written on the operand in both dialects, so it lives on Mem.
type Prefix uint8

const (
	PrefixNone Prefix = iota
	PrefixLock
	PrefixRep
	PrefixRepNE
)

var prefixNames = [...]string{"", "lock", "rep", "repne"}

func (p Prefix) String() string {
	if int(p) < len(prefixNames) {
		return prefixNames[p]
	}
	return "?"
}

// LookupRegister resolves a bare register name, for a dialect's lexer once it
// has stripped the sigil. This is reg.Lookup under a name that says where the
// call comes from; the % of AT&T syntax and NASM's bare spelling both arrive
// here as the same string.
func LookupRegister(name string) (reg.Reg, bool) { return reg.Lookup(name) }