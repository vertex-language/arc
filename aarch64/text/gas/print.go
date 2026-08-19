package gas

import (
	"strconv"
	"strings"

	"github.com/vertex-language/arc/aarch64/operand"
	"github.com/vertex-language/arc/aarch64/text"
)

// Printing.
//
// The printer is the parser's inverse, and the round trip is a property of the
// code rather than a claim: anything Print changes assembles to identical
// bytes, because what it changes is whitespace and what it preserves is every
// token that carries meaning — the spelling of a directive, the presence of a
// grouping, the comment on a line.
//
// Modifier spelling is the one thing that varies, and it varies by platform
// rather than by dialect. Both spellings name the same four roles, so the
// target decides which comes out.

// Platform selects the modifier spelling.
type Platform uint8

const (
	// ELF and COFF write :lo12:, which is GNU as's spelling.
	ELF Platform = iota
	COFF
	// MachO writes @PAGEOFF, which is the Darwin assembler's.
	MachO
)

func (pl Platform) modifier(m text.Modifier) string {
	if pl == MachO {
		return m.MachO()
	}
	return m.GAS()
}

// Options controls formatting that carries no meaning.
type Options struct {
	Platform Platform

	// Indent is what precedes a non-label statement. Labels sit at column
	// zero, which is the convention every A64 source file follows and the one
	// thing about layout worth being opinionated on.
	Indent string

	// MnemonicWidth is the column operands start at. Zero means one space.
	MnemonicWidth int
}

// DefaultOptions is four spaces and an eight-column mnemonic field, which is
// what GNU as's own output and every hand-written file in the wild use.
var DefaultOptions = Options{Indent: "        ", MnemonicWidth: 8}

type builder struct {
	b    strings.Builder
	opts Options
}

func (b *builder) str(s string)   { b.b.WriteString(s) }
func (b *builder) num(v int64)    { b.b.WriteString(strconv.FormatInt(v, 10)) }
func (b *builder) byteOut(c byte) { b.b.WriteByte(c) }

// Print writes a unit as A64 assembly.
func Print(u *text.Unit, opts Options) (string, error) {
	if opts.Indent == "" {
		opts.Indent = DefaultOptions.Indent
	}
	b := &builder{opts: opts}

	for _, n := range u.Nodes {
		printNode(b, n)
	}
	return b.b.String(), nil
}

func printNode(b *builder, n text.Node) {
	switch x := n.(type) {
	case *text.Comment:
		if x.Blank {
			b.str("\n")
			return
		}
		b.str(x.Text)
		b.str("\n")

	case *text.Label:
		b.str(x.Name)
		b.str(":")
		printComment(b, x.Comment)
		b.str("\n")

	case *text.Inst:
		b.str(b.opts.Indent)
		printInst(b, x)
		printComment(b, x.Comment)
		b.str("\n")

	case *text.Directive:
		b.str(b.opts.Indent)
		printDirective(b, x)
		printComment(b, x.Comment)
		b.str("\n")
	}
}

func printComment(b *builder, c string) {
	if c == "" {
		return
	}
	b.str("    ")
	b.str(c)
}

// printInst writes an instruction.
//
// The mnemonic is what the caller wrote. An alias stays the alias: `cmp` does
// not become `subs` on the way out, because Emit encodes the instruction you
// named and a printer that renamed it would have made a choice on the caller's
// behalf that the encoder deliberately does not make.
func printInst(b *builder, in *text.Inst) {
	mnem := in.Mnem
	ops := in.Ops

	// A condition carried in the mnemonic goes back into it: the table's
	// b.cond becomes b.eq, which is what the source said and what any A64
	// assembler will read back.
	if mnem == "b.cond" && len(ops) > 0 && ops[0].Kind == text.OpCond {
		mnem = "b." + ops[0].Cond.String()
		ops = ops[1:]
	}

	b.str(mnem)
	if len(ops) == 0 {
		return
	}
	pad := b.opts.MnemonicWidth - len(mnem)
	if pad < 1 {
		pad = 1
	}
	b.str(strings.Repeat(" ", pad))

	for i, o := range ops {
		if i > 0 {
			b.str(", ")
		}
		printOperand(b, o)
	}
}

func printOperand(b *builder, o *text.Operand) {
	switch o.Kind {
	case text.OpReg:
		printReg(b, o)

	case text.OpSys:
		b.str(o.Sys.String())

	case text.OpImm:
		b.str("#")
		printExpr(b, o.Expr)

	case text.OpTarget:
		if o.Mod.Valid() {
			m := b.opts.Platform.modifier(o.Mod)
			// The Darwin spelling is a suffix and the GNU one a prefix, which
			// is the whole reason the tree stores a role rather than a string.
			if b.opts.Platform == MachO {
				printExpr(b, o.Expr)
				b.str(m)
				return
			}
			b.str(m)
		}
		printExpr(b, o.Expr)

	case text.OpShift:
		b.str(o.Shift.String())
		printAmount(b, o.Amount)

	case text.OpExtend:
		b.str(o.Extend.String())
		printAmount(b, o.Amount)

	case text.OpCond:
		b.str(o.Cond.String())

	case text.OpBarrier:
		b.str(o.Barrier.String())

	case text.OpPrfOp:
		b.str(o.Prf.String())

	case text.OpMem:
		printMem(b, o)
	}
}

func printReg(b *builder, o *text.Operand) {
	b.str(o.Reg.String())
	switch {
	case o.HasLane:
		b.str("." + o.Elem.String() + "[" + strconv.Itoa(o.Lane) + "]")
	case o.Arr != 0:
		b.str("." + o.Arr.String())
	}
}

func printAmount(b *builder, e text.Expr) {
	if e == nil {
		return
	}
	b.str(" #")
	printExpr(b, e)
}

func printMem(b *builder, o *text.Operand) {
	b.str("[")
	b.str(o.Reg.String())

	switch o.Mem.Form {
	case operand.AddrBase:
		b.str("]")

	case operand.AddrOffset:
		if o.Mem.Disp != nil {
			b.str(", #")
			if o.Mem.Mod.Valid() {
				printModified(b, o.Mem.Mod, o.Mem.Disp)
			} else {
				printExpr(b, o.Mem.Disp)
			}
		}
		b.str("]")

	case operand.AddrPreIndex:
		b.str(", #")
		printExpr(b, o.Mem.Disp)
		b.str("]!")

	case operand.AddrPostIndex:
		b.str("], #")
		printExpr(b, o.Mem.Disp)

	case operand.AddrRegOffset:
		b.str(", ")
		b.str(o.Mem.Index.String())
		// `[x1, x2]` and `[x1, x2, lsl #0]` are the same address, and the
		// architecture's preferred disassembly is the shorter one.
		if o.Mem.Ext != operand.ExtLSL || o.Mem.Amount != nil {
			b.str(", ")
			if o.Mem.Ext == operand.ExtLSL {
				b.str("lsl")
			} else {
				b.str(o.Mem.Ext.String())
			}
			printAmount(b, o.Mem.Amount)
		}
		b.str("]")
	}
}

func printModified(b *builder, m text.Modifier, e text.Expr) {
	if b.opts.Platform == MachO {
		printExpr(b, e)
		b.str(m.MachO())
		return
	}
	b.str(m.GAS())
	printExpr(b, e)
}

func printDirective(b *builder, d *text.Directive) {
	// The spelling is reproduced rather than normalized. .global does not
	// become .globl: the two assemble identically, and rewriting one into the
	// other is a change to the file that nobody asked for.
	b.str(d.Spelling)

	var parts []string
	if d.Name != "" {
		parts = append(parts, d.Name)
	}

	switch d.Kind {
	case text.DirReq:
		// `name .req reg` puts the name first, before the directive.
		b.b.Reset()
		b.str(b.opts.Indent)
		b.str(d.Name + " .req " + strings.Join(d.Flags, " "))
		return

	case text.DirType:
		if len(d.Flags) > 0 {
			parts = append(parts, "@"+d.Flags[0])
		}

	case text.DirSection:
		if impliedSection(d.Spelling) != "" {
			parts = nil
		}
		parts = append(parts, d.Flags...)

	case text.DirCFI, text.DirOpaque:
		parts = append(parts, d.Flags...)
	}

	if len(parts) > 0 {
		b.str(" " + strings.Join(parts, ", "))
	}

	for i, e := range d.Args {
		if i == 0 && len(parts) == 0 {
			b.str(" ")
		} else {
			b.str(", ")
		}
		printExpr(b, e)
	}
	for i, s := range d.Strings {
		if i == 0 && len(parts) == 0 && len(d.Args) == 0 {
			b.str(" ")
		} else {
			b.str(", ")
		}
		b.str(quote(s))
	}
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}