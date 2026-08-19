package decode

import (
	"math/bits"
	"sort"
	"sync"

	"github.com/vertex-language/arc/aarch64/isa"
)

// The decode table.
//
// The ARM ARM describes A64 decoding as a tree keyed on op0 at 28:25, then on
// per-group fields, four levels deep. This builds the first level of that tree
// and scans within a bucket, because the second level onward is stated in the
// manual's prose per group rather than in any field this table carries — and a
// hand-written second level would be exactly the independent table this package
// exists not to have.
//
// The scan is not linear in practice: sixteen buckets over a table where most
// groups occupy one or two of them, and each bucket ordered by mask
// specificity, so the first match is the answer. If the table grows to the full
// ISA and a bucket gets long, the fix is a second index keyed on a field the
// rows already state, not a parallel structure.

const op0Lo, op0Width = 25, 4

type table struct {
	// buckets is indexed by op0. A form whose mask does not fix op0 is
	// reachable through every bucket, which is why a row can appear more than
	// once and why entries are compared by pointer, not by position.
	buckets [1 << op0Width][]*isa.Form

	// aliases maps an underlying mnemonic to the alias forms declared over it.
	aliases map[string][]*isa.Form
}

var (
	tbl     *table
	tblOnce sync.Once
)

func get() *table {
	tblOnce.Do(func() { tbl = build() })
	return tbl
}

func build() *table {
	t := &table{aliases: map[string][]*isa.Form{}}

	op0Mask := uint32(((1 << op0Width) - 1) << op0Lo)

	for _, f := range isa.All() {
		if f.Attrs&isa.AttrAlias != 0 {
			if of, ok := f.Alias(); ok {
				t.aliases[of] = append(t.aliases[of], f)
			}
			continue
		}
		// .inst has no encoding of its own — its mask is zero, so it would
		// match every word. It is a directive that states a word, not a form
		// that decodes one.
		if f.Mask == 0 {
			continue
		}

		if f.Mask&op0Mask == op0Mask {
			k := (f.Word & op0Mask) >> op0Lo
			t.buckets[k] = append(t.buckets[k], f)
			continue
		}
		for k := 0; k < 1<<op0Width; k++ {
			if f.Mask&op0Mask != 0 {
				// Partially fixed: only the buckets consistent with the bits
				// the mask does fix.
				if uint32(k)<<op0Lo&f.Mask != f.Word&op0Mask&f.Mask {
					continue
				}
			}
			t.buckets[k] = append(t.buckets[k], f)
		}
	}

	// Most specific first. A word matching two forms takes the one that fixes
	// more bits, which is the ARM ARM's own rule for a narrower encoding
	// carved out of a wider one; Check reports the pairs where that rule is
	// doing real work, since those are the ones worth a human deciding about.
	for k := range t.buckets {
		b := t.buckets[k]
		sort.SliceStable(b, func(i, j int) bool {
			return bits.OnesCount32(b[i].Mask) > bits.OnesCount32(b[j].Mask)
		})
	}
	return t
}

// find is the form lookup: the first match in the word's bucket.
func (t *table) find(word uint32) (*isa.Form, bool) {
	for _, f := range t.buckets[(word>>op0Lo)&((1<<op0Width)-1)] {
		if word&f.Mask == f.Word {
			return f, true
		}
	}
	return nil, false
}

// Check reports every pair of non-alias forms where one's encodings are a
// subset of another's, which is where find's specificity rule decides the
// answer rather than merely confirming it.
//
// This is not run at init. A subset pair is usually legitimate — MOV (register)
// is carved out of ORR, and the architecture intends exactly that — so panicking
// would be wrong. It is a test's assertion and `arc isa --check`'s output: the
// list should be short and every entry should be a pair somebody chose.
func Check() []*AmbiguousError {
	t := get()
	var out []*AmbiguousError
	seen := map[[2]*isa.Form]bool{}

	for _, b := range t.buckets {
		for i, a := range b {
			for _, c := range b[i+1:] {
				if a == c || seen[[2]*isa.Form{a, c}] {
					continue
				}
				// c's fixed bits are a superset of a's, and they agree
				// wherever a fixes anything: every word matching c matches a.
				if c.Mask&a.Mask == a.Mask && c.Word&a.Mask == a.Word {
					seen[[2]*isa.Form{a, c}] = true
					out = append(out, &AmbiguousError{Word: c.Word, A: c, B: a})
				}
			}
		}
	}
	return out
}

// ResolveWord is the table's answer, exported for the generator that checks
// this index against isa.ResolveWord's linear scan.
func ResolveWord(word uint32) (*isa.Form, bool) { return get().find(word) }