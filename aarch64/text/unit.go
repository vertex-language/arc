package text

// Unit is one parsed source file: the statements, in source order, and nothing
// derived from them.
//
// Both parsing and printing go through this type, which is what makes the round
// trip a property of the code rather than a claim: Print(Parse(src)) differs
// from src only where the printer normalizes, and what it normalizes is a
// spelling rather than a meaning. Nothing here is a summary that could fall out
// of step with the statements — Defined and Referenced walk the list each time
// they are asked.
type Unit struct {
	// File is the name the parser was given, for positions.
	File string

	// Nodes are the statements in source order. Source order is the only order
	// that means anything here: a section directive changes where subsequent
	// statements land, so a pass that reordered nodes would change the object.
	Nodes Nodes
}

// Defined returns every symbol this unit defines, in order of first definition.
//
// Numeric labels are excluded. They are position references rather than
// symbols — 1: may appear a dozen times in one file and each is a different
// place — so a symbol table has nothing to record and a duplicate-definition
// check would fire on correct source.
func (u *Unit) Defined() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range u.Nodes {
		l, ok := n.(*Label)
		if !ok || l.Numeric || seen[l.Name] {
			continue
		}
		seen[l.Name] = true
		out = append(out, l.Name)
	}
	return out
}

// Referenced returns every symbol named by an operand or a directive argument
// and not defined here, which is what a caller declares as external.
//
// A name this unit also defines is not reported, even where the reference
// precedes the definition: which of the two a symbol is depends on the file as
// a whole, not on the order a reader meets them in.
func (u *Unit) Referenced() []string {
	defined := map[string]bool{}
	for _, n := range u.Defined() {
		defined[n] = true
	}

	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name == "" || defined[name] || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	for _, n := range u.Nodes {
		switch v := n.(type) {
		case *Inst:
			for _, o := range v.Ops {
				for _, s := range o.Symbols() {
					add(s)
				}
			}
		case *Directive:
			for _, e := range v.Args {
				for _, s := range Symbols(e) {
					add(s)
				}
			}
		}
	}
	return out
}

// Sections returns the section names this unit switches to, in first-use order.
func (u *Unit) Sections() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range u.Nodes.Directives(DirSection) {
		if d.Name == "" || seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		out = append(out, d.Name)
	}
	return out
}

// Resolved reports whether every instruction carries a Form.
//
// This is the question that decides whether a unit can be printed in another
// dialect or handed to the encoder without further work. On this architecture
// it matters less than on x86_64 — there is one dialect, so there is no
// translation that needs a width neither spelling states — but Assemble still
// wants the answer, because an unresolved instruction has to go back through
// isa.Resolve and that needs a feature set the printer does not have.
func (u *Unit) Resolved() bool {
	for _, in := range u.Nodes.Insts() {
		if in.Form == nil && in.Mnem != ".inst" {
			return false
		}
	}
	return true
}