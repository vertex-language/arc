package reg

// Preservation under the AAPCS64.
//
// Two things stop this from being a single lookup keyed on the register.
//
// The first is width. Only the low 64 bits of V8-V15 are callee-saved; a caller
// that needs more must preserve it itself. Save says so rather than rounding to
// "preserved", because rounding either way produces wrong code — up, and a
// caller skips a spill it needed; down, and a callee spills what nobody asked
// it to. Note that this makes the answer depend on the *view*: D8 is preserved
// in full, because a D is 64 bits wide and the preserved part is exactly the
// low 64. Q8 is not.
//
// The second is the procedure call variant. The preservation of the SVE
// registers depends on the interface of the function they appear in, not on the
// registers themselves: a subroutine that takes or returns arguments in Z or P
// registers preserves all of Z8-Z23 and P4-P15, and one that does not preserves
// only the low 64 bits of Z8-Z15 and none of the predicates at all. No property
// of Z8 decides which, so Save takes the variant as a parameter. A function
// that guessed would be wrong for half its callers and silently so.
type Variant uint8

const (
	// Base is the base procedure call standard: the subroutine passes and
	// returns nothing in scalable registers.
	Base Variant = iota

	// SVEArgs is the variant for a subroutine that takes at least one argument
	// in a scalable vector or predicate register, or returns a result in one.
	SVEArgs
)

// Saved is a preservation rule.
type Saved uint8

const (
	// Caller means the register's value is not preserved across a call. The
	// caller saves it if it needs it.
	Caller Saved = iota

	// Callee means the whole of the value named by this register is preserved.
	Callee

	// CalleeLow64 means the low 64 bits are preserved and the rest is not. It
	// is the honest answer for a 128-bit or scalable view of a register whose
	// preserved part is narrower than the view.
	CalleeLow64
)

func (s Saved) String() string {
	switch s {
	case Callee:
		return "callee-saved"
	case CalleeLow64:
		return "callee-saved (low 64 bits)"
	}
	return "caller-saved"
}

// Save reports how a register is preserved across a call under the given
// procedure call variant.
//
// The zero register is reported as Caller. It holds nothing, so the question
// does not arise; Caller is the answer that causes no one to emit a spill.
func Save(r Reg, v Variant) Saved {
	switch x := r.(type) {
	case X:
		return gprSave(uint16(x), x == XZR)
	case W:
		return gprSave(uint16(x), x == WZR)
	case Xsp:
		return gprSave(uint16(x), false) // SP is callee-saved
	case Wsp:
		return gprSave(uint16(x), false)

	case V, Q:
		return vecSave(r.Num(), 128)
	case D:
		return vecSave(r.Num(), 64)
	case S:
		return vecSave(r.Num(), 32)
	case H:
		return vecSave(r.Num(), 16)
	case B:
		return vecSave(r.Num(), 8)
	case Vec:
		return vecSave(uint16(x.R), x.A.Bits())
	case VLane:
		return vecSave(uint16(x.R), x.E.Bits())

	case Z:
		if v == SVEArgs {
			if x >= 8 && x <= 23 {
				return Callee
			}
			return Caller
		}
		if x >= 8 && x <= 15 {
			return CalleeLow64
		}
		return Caller

	case P:
		if v == SVEArgs && x >= 4 {
			return Callee
		}
		return Caller

	case Ffr:
		return Caller
	case Sys:
		// FPSR is not preserved, FPMR is caller-saved, and FPCR is a global
		// whose bits are governed by rules an assembler cannot express as a
		// preservation rule. None of them is callee-saved.
		return Caller
	}
	return Caller
}

// gprSave applies the general-purpose rule: r19-r29 and SP are callee-saved,
// and all 64 bits of r19-r29 are, even under ILP32. r18 is caller-saved here,
// which is the base standard's answer; a platform that claims it as the
// platform register imposes something stricter than callee-saved, and that is a
// platform ABI's statement to make rather than this table's.
func gprSave(num uint16, zero bool) Saved {
	if zero {
		return Caller
	}
	if num >= 19 && num <= 31 {
		return Callee
	}
	return Caller
}

// vecSave applies the SIMD rule: V8-V15 have their low 64 bits preserved. A
// view no wider than 64 bits is therefore preserved entirely.
func vecSave(num uint16, bits uint16) Saved {
	if num < 8 || num > 15 {
		return Caller
	}
	if bits > 0 && bits <= 64 {
		return Callee
	}
	return CalleeLow64
}