package reg

// The general purpose registers. Three widths over eight architectural
// registers, numbered identically in each width by the w bit of the opcode.
//
// Intel SDM Vol. 2, ModRM reg/rm tables:
//
//	num   8-bit   16-bit   32-bit
//	0     al      ax       eax
//	1     cl      cx       ecx
//	2     dl      dx       edx
//	3     bl      bx       ebx
//	4     ah      sp       esp
//	5     ch      bp       ebp
//	6     dh      si       esi
//	7     bh      di       edi
//
// Slots 4-7 of the byte column are AH, CH, DH and BH unconditionally. There
// is no REX prefix in i386, so SPL, BPL, SIL and DIL do not exist and the
// high-byte registers are never displaced. This is the concrete reason i386
// is a package and not a build tag on x86_64.

// R32 is a 32-bit general purpose register.
//
// R32 has no Parent method. In i386 there is no wider register, and an absent
// method is a compile error where a zero value would be a silent wrong answer.
type R32 uint8

const (
	EAX R32 = iota
	ECX
	EDX
	EBX
	ESP
	EBP
	ESI
	EDI
)

var r32Names = [8]string{"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi"}

// Intel386 psABI v1.1, Table 2.3.
var r32Save = [8]Save{
	CallerSaved, // eax
	CallerSaved, // ecx
	CallerSaved, // edx
	CalleeSaved, // ebx
	CalleeSaved, // esp
	CalleeSaved, // ebp
	CalleeSaved, // esi
	CalleeSaved, // edi
}

var r32Role = [8]string{
	"integer and pointer return; address of a returned struct",
	"scratch",
	"high half of a 64-bit return",
	"GOT pointer for calls through the PLT",
	"stack pointer",
	"frame pointer (optional)",
	"",
	"",
}

func (r R32) spec() spec {
	return spec{name: r32Names[r], class: ClassGP, num: uint8(r), root: uint8(r), lo: 0, hi: 32}
}

func (r R32) Name() string        { return r32Names[r] }
func (r R32) String() string      { return r32Names[r] }
func (r R32) Num() uint8          { return uint8(r) }
func (r R32) Bits() int           { return 32 }
func (r R32) Class() Class        { return ClassGP }
func (r R32) Save() Save          { return r32Save[r] }
func (r R32) Role() string        { return r32Role[r] }
func (r R32) Overlaps(o Reg) bool { return overlaps(r, o) }

// DWARF numbers for the 32-bit registers are their encoding numbers.
// Intel386 psABI v1.1, Table 2.14.
func (r R32) DWARF() (int, bool) { return int(r), true }

// R16 is a 16-bit general purpose register: the low half of an R32.
type R16 uint8

const (
	AX R16 = iota
	CX
	DX
	BX
	SP
	BP
	SI
	DI
)

var r16Names = [8]string{"ax", "cx", "dx", "bx", "sp", "bp", "si", "di"}

// Parent is the 32-bit register this is the low half of.
func (r R16) Parent() R32 { return R32(r) }

func (r R16) spec() spec {
	return spec{name: r16Names[r], class: ClassGP, num: uint8(r), root: uint8(r), lo: 0, hi: 16}
}

func (r R16) Name() string        { return r16Names[r] }
func (r R16) String() string      { return r16Names[r] }
func (r R16) Num() uint8          { return uint8(r) }
func (r R16) Bits() int           { return 16 }
func (r R16) Class() Class        { return ClassGP }
func (r R16) Overlaps(o Reg) bool { return overlaps(r, o) }

// Save is the parent's: preserving EBX preserves BX.
func (r R16) Save() Save { return r.Parent().Save() }

// Role is empty. The calling convention in psABI Table 2.3 is stated over
// whole registers; the return-value locations for narrow types are Table 2.4
// and are a property of a type, not of a register.
func (r R16) Role() string { return "" }

// DWARF assigns no number to the 16-bit registers.
func (r R16) DWARF() (int, bool) { return noDWARF, false }

// R8 is an 8-bit general purpose register.
//
// The eight byte registers are drawn from only four architectural registers:
// AL and AH are disjoint bytes of AX, and neither ESP, EBP, ESI nor EDI has a
// byte view in i386 at all.
type R8 uint8

const (
	AL R8 = iota
	CL
	DL
	BL
	AH
	CH
	DH
	BH
)

var r8Names = [8]string{"al", "cl", "dl", "bl", "ah", "ch", "dh", "bh"}

// Parent is the 16-bit register this is a byte of. AL and AH share AX.
func (r R8) Parent() R16 { return R16(r & 3) }

// High reports whether this is a high-byte register: bits 8-15 of its parent.
func (r R8) High() bool { return r >= AH }

func (r R8) spec() spec {
	var lo uint16
	if r.High() {
		lo = 8
	}
	return spec{name: r8Names[r], class: ClassGP, num: uint8(r), root: uint8(r & 3), lo: lo, hi: lo + 8}
}

func (r R8) Name() string         { return r8Names[r] }
func (r R8) String() string       { return r8Names[r] }
func (r R8) Num() uint8           { return uint8(r) }
func (r R8) Bits() int            { return 8 }
func (r R8) Class() Class         { return ClassGP }
func (r R8) Save() Save           { return r.Parent().Save() }
func (r R8) Role() string         { return "" }
func (r R8) Overlaps(o Reg) bool  { return overlaps(r, o) }
func (r R8) DWARF() (int, bool)   { return noDWARF, false }