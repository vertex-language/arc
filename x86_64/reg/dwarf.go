// x86_64/reg/dwarf.go
package reg

// DWARF register numbers, psABI §3.6.2.
//
// These are NOT the encoding numbers. Indexed by encoding number, the
// 64-bit GPRs map 0,2,1,3,7,6,4,5 — RCX/RDX are swapped and RSP/RBP/RSI/RDI
// are permuted. Only RAX and RBX agree. Anything that computes one from the
// other passes every test written against RAX and is wrong everywhere else.
var gprDWARF = [16]int{
	0,  // RAX
	2,  // RCX
	1,  // RDX
	3,  // RBX
	7,  // RSP
	6,  // RBP
	4,  // RSI
	5,  // RDI
	8, 9, 10, 11, 12, 13, 14, 15, // R8–R15
}

// ReturnAddress is the DWARF column for the return address. It is not a
// physical register; the value lives at 0(%rsp).
const ReturnAddress = 16

func (r Reg64) DWARF() int { return gprDWARF[r] }
func (r Reg32) DWARF() int { return gprDWARF[r] }
func (r Reg16) DWARF() int { return gprDWARF[r] }

// DWARF is the number of the containing 64-bit register. DWARF assigns no
// number to a sub-register; a debugger names the part with DW_OP_bit_piece.
// AH answers RAX's number, not RSP's.
func (r Reg8) DWARF() int { return gprDWARF[r.Loc().Index] }

// Vector registers: XMM0–15 are 17–32, then a 34-number gap before XMM16–31
// resumes at 67.
func vecDWARF(i uint8) int {
	if i < 16 {
		return 17 + int(i)
	}
	return 67 + int(i-16)
}

func (r Xmm) DWARF() int { return vecDWARF(uint8(r)) }
func (r Ymm) DWARF() int { return vecDWARF(uint8(r)) }
func (r Zmm) DWARF() int { return vecDWARF(uint8(r)) }

func (r St) DWARF() int   { return 33 + int(r) }
func (r Mm) DWARF() int   { return 41 + int(r) }
func (r Sreg) DWARF() int { return 50 + int(r) } // ES=50 … GS=55
func (r K) DWARF() int    { return 118 + int(r) }
func (r Tmm) DWARF() int  { return 146 + int(r) }

// The psABI defines no mapping for privileged registers.
func (Cr) DWARF() int { return NoDWARF }
func (Dr) DWARF() int { return NoDWARF }

// Numbers with no register in this package: rFLAGS 49, fs.base 58,
// gs.base 59, tr 62, ldtr 63, mxcsr 64, fcw 65, fsw 66, tilecfg 154.
// APX R16–R31 occupy 130–145 and are not encodable at the declared
// feature ceiling, so they are absent here rather than declared and refused.
const (
	DWARFrFLAGS  = 49
	DWARFFSBase  = 58
	DWARFGSBase  = 59
	DWARFMXCSR   = 64
	DWARFFCW     = 65
	DWARFFSW     = 66
	DWARFTileCfg = 154
)