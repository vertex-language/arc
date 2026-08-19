package reg

// The DWARF register numbering of aadwarf64.
//
// This is not a permutation of the architectural encoding the way x86_64's is,
// but it is not the identity either. The general-purpose registers map through
// unchanged, and everything else sits at a per-file base with a reserved hole
// between the thread-ID registers and VG:
//
//	0-30    X0-X30
//	31      SP
//	32      PC
//	33      ELR_mode
//	34      RA_SIGN_STATE
//	35-39   TPIDRRO_EL0, TPIDR_EL0, TPIDR_EL1, TPIDR_EL2, TPIDR_EL3
//	40-45   reserved
//	46      VG
//	47      FFR
//	48-63   P0-P15
//	64-95   V0-V31
//	96-127  Z0-Z31
//
// Two consequences shape the signature below.
//
// First, XZR has no number. Slot 31 is the stack pointer, and the zero register
// is not a location — nothing can be spilled to it or restored from it, so
// there is nothing for an unwinder to say. DWARF therefore reports whether an
// answer exists rather than inventing one.
//
// Second, V and Z have separate ranges even though a V register is the low 128
// bits of the corresponding Z. That is deliberate: call frame instructions do
// not carry a register's size, so the size has to come from the number, and one
// number cannot mean both. A consumer is expected to handle the aliasing
// itself. This function must therefore not route through Parent or Overlaps.
const (
	DWARFPC          = 32
	DWARFELRMode     = 33
	DWARFRASignState = 34
	DWARFVG          = 46
	DWARFFFR         = 47
)

const (
	dwarfPBase = 48
	dwarfVBase = 64
	dwarfZBase = 96
)

// DWARF reports the aadwarf64 register number for a register, and whether one
// exists.
func DWARF(r Reg) (int, bool) {
	switch v := r.(type) {
	case X:
		if v == XZR {
			return 0, false
		}
		return int(v), true
	case W:
		if v == WZR {
			return 0, false
		}
		return int(v), true
	case Xsp:
		return int(v), true // 31 is SP, which is the number DWARF assigns
	case Wsp:
		return int(v), true
	case V:
		return dwarfVBase + int(v), true
	case Q:
		return dwarfVBase + int(v), true
	case D:
		return dwarfVBase + int(v), true
	case S:
		return dwarfVBase + int(v), true
	case H:
		return dwarfVBase + int(v), true
	case B:
		return dwarfVBase + int(v), true
	case Vec:
		return dwarfVBase + int(v.R), true
	case VLane:
		return dwarfVBase + int(v.R), true
	case Z:
		return dwarfZBase + int(v), true
	case P:
		return dwarfPBase + int(v), true
	case Ffr:
		return DWARFFFR, true
	case Sys:
		switch v {
		case TPIDRRO_EL0:
			return 35, true
		case TPIDR_EL0:
			return 36, true
		case TPIDR_EL1:
			return 37, true
		case TPIDR_EL2:
			return 38, true
		case TPIDR_EL3:
			return 39, true
		}
		return 0, false
	}
	return 0, false
}