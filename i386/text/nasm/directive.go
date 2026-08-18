package nasm

import (
	"strings"

	"github.com/vertex-language/arc/i386/text"
)

// The directive and pseudo-instruction spellings of this dialect.
//
// dataWidths are the Dx family. dy and dz — 32- and 64-byte YMM/ZMM-width
// data — are recognised only to be refused: text.Width has no constant for
// either.
var dataWidths = map[string]text.Width{
	"db": text.Width8, "dw": text.Width16, "dd": text.Width32,
	"dq": text.Width64, "dt": text.Width80, "do": text.Width128,
}

// resWidths are the RESx family. Count is a unit count, not a byte count —
// resw 5 is five words — which is exactly what text.Fill.Count already means
// ("Count copies of a Value that is Size bytes wide"), so no conversion is
// needed at the boundary.
var resWidths = map[string]text.Width{
	"resb": text.Width8, "resw": text.Width16, "resd": text.Width32,
	"resq": text.Width64, "rest": text.Width80, "reso": text.Width128,
}

var wideGap = map[string]bool{"dy": true, "dz": true, "resy": true, "resz": true}

var shortSections = map[string]text.SectionKind{
	".text": text.SectionText, ".data": text.SectionData,
	".bss": text.SectionBSS, ".rodata": text.SectionROData,
}

// symbolAttrs are the directives that attach a binding to a name. required is
// EXTERN's unconditional sibling; the reference-counting distinction the
// manual draws between them is not modeled, so both become AttrExtern.
var symbolAttrs = map[string]text.Attr{
	"global": text.AttrGlobal, "extern": text.AttrExtern,
	"required": text.AttrExtern, "static": text.AttrLocal,
}

var symbolTypes = map[string]text.SymbolType{
	"function": text.TypeFunc, "data": text.TypeObject,
}

// rejected names directives arc understands well enough to refuse by name.
var rejected = map[string]string{
	"common":    "a common symbol has no section arc can place it in; declare it in .bss with resb",
	"absolute":  "arc has no hypothetical, unaddressed section to point ABSOLUTE at",
	"incbin":    "arc does not open files the command line did not name",
	"cpu":       "the target is selected by -t and --features, not from inside a file",
	"default":   "REL/ABS and BND default state is not modeled; write it explicitly per instruction",
	"float":     "arc emits db, dw, dd and dq; there is no floating-point directive",
	"warning":   "warnings are a command-line concern (-w), not an in-file directive",
	"list":      "listing output is a command-line concern, not an in-file directive",
	"dollarhex": "the $ hexadecimal prefix is deprecated in NASM itself; write 0x instead",
	"struc":     "structure layouts are a macro-package feature; Go is arc's macro language",
	"endstruc":  "structure layouts are a macro-package feature; Go is arc's macro language",
	"istruc":    "structure layouts are a macro-package feature; Go is arc's macro language",
	"iend":      "structure layouts are a macro-package feature; Go is arc's macro language",
	"align":     "ALIGN is a macro-package directive; arc does not expand macro packages",
	"alignb":    "ALIGNB is a macro-package directive; arc does not expand macro packages",
	"sectalign": "SECTALIGN is a macro-package directive; arc does not expand macro packages",
}

var bitsWords = map[string]bool{"bits": true, "use16": true, "use32": true, "use64": true}

// directiveNames is the union of every word one() must not mistake for a
// mnemonic, built once at init so the sets above stay the single source of
// truth.
var directiveNames = map[string]bool{
	"section": true, "segment": true, "times": true,
}

func init() {
	for k := range dataWidths {
		directiveNames[k] = true
	}
	for k := range resWidths {
		directiveNames[k] = true
	}
	for k := range wideGap {
		directiveNames[k] = true
	}
	for k := range symbolAttrs {
		directiveNames[k] = true
	}
	for k := range rejected {
		directiveNames[k] = true
	}
	for k := range bitsWords {
		directiveNames[k] = true
	}
}

func (p *parser) directive(pos text.Pos, word string) bool {
	d := strings.ToLower(word)

	if why, no := rejected[d]; no {
		p.errorf(pos, "%s is not accepted", strings.ToUpper(d)).Note("%s", why)
		p.skipLine()
		return false
	}
	if bitsWords[d] {
		p.errorf(pos, "%s is not accepted", strings.ToUpper(d)).
			Note("arc's i386 target is fixed 32-bit protected mode")
		p.skipLine()
		return false
	}
	if wideGap[d] {
		p.errorf(pos, "%s is not accepted", strings.ToUpper(d)).
			Note("arc's Width type has no 32- or 64-byte data size yet")
		p.skipLine()
		return false
	}
	if w, ok := dataWidths[d]; ok {
		return p.data(pos, w)
	}
	if w, ok := resWidths[d]; ok {
		return p.resFill(pos, w)
	}

	switch d {
	case "section", "segment":
		return p.section(pos)
	case "times":
		return p.times(pos)
	}

	if attr, ok := symbolAttrs[d]; ok {
		return p.symAttr(pos, attr, d == "global")
	}

	p.errorf(pos, "unknown directive %s", word).
		Note("arc accepts the directives that name sections, symbols and data")
	return false
}

// data parses a comma-separated Dx list: strings and numeric expressions
// freely mixed, which is the one shape gas's printer has to split back apart
// when a unit written here is reprinted as GNU as.
func (p *parser) data(pos text.Pos, w text.Width) bool {
	n := &text.Data{Common: text.Common{P: pos}, Width: w}
	for {
		item := text.DataItem{Pos: p.tok.pos}
		if p.tok.kind == tString {
			item.Str, item.IsStr = p.tok.str, true
			p.advance()
		} else {
			item.X = p.expr()
		}
		n.Items = append(n.Items, item)
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}
	p.unit.Add(n)
	return true
}

func (p *parser) resFill(pos text.Pos, w text.Width) bool {
	p.unit.Add(&text.Fill{Common: text.Common{P: pos}, Size: w, Count: p.expr()})
	return true
}

// section parses SECTION/SEGMENT name. Flags and format-specific extensions
// pass through nowhere: this front end accepts the portable name only, same
// restraint gas's .section takes with @progbits.
func (p *parser) section(pos text.Pos) bool {
	if p.tok.kind != tIdent && p.tok.kind != tString {
		p.errorf(p.tok.pos, "expected a section name, got %s", p.tok)
		return false
	}
	name := p.tok.str
	p.advance()

	d := &text.SectionDecl{Common: text.Common{P: pos}, Name: name}
	d.Kind, _ = text.StandardSection(name)
	if k, ok := shortSections[strings.ToLower(name)]; ok {
		d.Kind, d.Short = k, true
	}
	p.unit.Add(d)
	return true
}

// times parses TIMES count directive. This tree has no statement expander
// (see node.go's own note on the subject), so TIMES over an instruction, or
// over a Dx list with more than one item, is refused rather than
// approximated; TIMES over a single Dx value or a RESx count folds directly
// into text.Fill.
func (p *parser) times(pos text.Pos) bool {
	count := p.expr()

	if p.tok.kind != tIdent {
		p.errorf(p.tok.pos, "expected a data directive after TIMES, got %s", p.tok)
		return false
	}
	word := strings.ToLower(p.tok.str)

	if w, ok := dataWidths[word]; ok {
		p.advance()
		if p.tok.kind == tString {
			p.errorf(pos, "TIMES over a string needs an expander arc does not have").
				Note("write the bytes out with db, or repeat a single numeric value")
			return false
		}
		val := p.expr()
		if p.tok.is(",") {
			p.errorf(pos, "TIMES over more than one value needs an expander arc does not have").
				Note("write one TIMES line per value")
			return false
		}
		p.unit.Add(&text.Fill{Common: text.Common{P: pos}, Count: count, Size: w, Value: val})
		return true
	}

	if w, ok := resWidths[word]; ok {
		p.advance()
		units := p.expr()
		total := &text.Binary{P: pos, Op: text.Mul, X: count, Y: units}
		p.unit.Add(&text.Fill{Common: text.Common{P: pos}, Count: total, Size: w})
		return true
	}

	p.errorf(pos, "TIMES over an instruction needs an expander arc does not have").
		Note("Go is arc's macro language; unroll the repeat there instead")
	p.skipLine()
	return false
}

// symAttr parses GLOBAL/EXTERN/REQUIRED/STATIC. Only GLOBAL accepts the
// per-symbol `:function`/`:data` suffix; a name that carries one gets its own
// SymbolDecl, since text.SymbolDecl.Type is one value for the whole
// declaration and GLOBAL a:function, b:data names two.
func (p *parser) symAttr(pos text.Pos, attr text.Attr, allowType bool) bool {
	var plain []string
	for {
		if p.tok.kind != tIdent {
			p.errorf(p.tok.pos, "expected a symbol name, got %s", p.tok)
			return false
		}
		name := p.tok.str
		p.advance()

		if allowType && p.tok.is(":") {
			p.advance()
			if p.tok.kind != tIdent {
				p.errorf(p.tok.pos, "expected function or data after ':'")
				return false
			}
			t, ok := symbolTypes[strings.ToLower(p.tok.str)]
			if !ok {
				p.errorf(p.tok.pos, "unknown symbol type %q", p.tok.str).
					Note("arc records function and data")
				return false
			}
			p.advance()
			p.unit.Add(&text.SymbolDecl{
				Common: text.Common{P: pos}, Names: []string{name}, Attrs: attr, Type: t,
			})
		} else {
			plain = append(plain, name)
		}
		if !p.tok.is(",") {
			break
		}
		p.advance()
	}
	if len(plain) > 0 {
		p.unit.Add(&text.SymbolDecl{Common: text.Common{P: pos}, Names: plain, Attrs: attr})
	}
	return true
}