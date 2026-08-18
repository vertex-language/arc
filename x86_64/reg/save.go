// x86_64/reg/save.go
package reg

// Under System V, RBX, RSP, RBP and R12–R15 belong to the caller and every
// vector register is scratch. Win64 additionally preserves RSI, RDI and the
// low 128 bits of vector registers 6–15 — but only the low 128: the upper
// half of YMM6–YMM15 is the caller's problem, so the same physical register
// answers Preserved at 128 bits and PreservedLow at 256.
//
// Flat has no OS and no linker; System V rules apply for lack of anything
// else to follow.

var sysvPreservedGPR = [16]bool{
	3: true, // RBX
	4: true, // RSP
	5: true, // RBP
	12: true, 13: true, 14: true, 15: true,
}

var win64PreservedGPR = [16]bool{
	3: true, // RBX
	4: true, // RSP
	5: true, // RBP
	6: true, // RSI
	7: true, // RDI
	12: true, 13: true, 14: true, 15: true,
}

func gprSave(index uint8, p Platform) Preservation {
	var ok bool
	if p.win64() {
		ok = win64PreservedGPR[index]
	} else {
		ok = sysvPreservedGPR[index]
	}
	if ok {
		return Preserved
	}
	return Volatile
}

func vecSave(index uint8, hi uint16, p Platform) Preservation {
	if !p.win64() || index < 6 || index > 15 {
		return Volatile
	}
	if hi <= 128 {
		return Preserved
	}
	return PreservedLow
}

func (r Reg64) Save(p Platform) Preservation { return gprSave(uint8(r), p) }
func (r Reg32) Save(p Platform) Preservation { return gprSave(uint8(r), p) }
func (r Reg16) Save(p Platform) Preservation { return gprSave(uint8(r), p) }
func (r Reg8) Save(p Platform) Preservation  { return gprSave(r.Loc().Index, p) }

func (r Xmm) Save(p Platform) Preservation { return vecSave(uint8(r), 128, p) }
func (r Ymm) Save(p Platform) Preservation { return vecSave(uint8(r), 256, p) }
func (r Zmm) Save(p Platform) Preservation { return vecSave(uint8(r), 512, p) }

// Every x87 and MMX register is caller-saved under System V. Win64 states no
// explicit convention for them and forbids their use in kernel mode; treating
// them as volatile is the only claim that holds on both.
func (St) Save(Platform) Preservation { return Volatile }
func (Mm) Save(Platform) Preservation { return Volatile }

func (K) Save(Platform) Preservation   { return Volatile }
func (Tmm) Save(Platform) Preservation { return Volatile }

// Not part of any calling convention.
func (Sreg) Save(Platform) Preservation { return Volatile }
func (Cr) Save(Platform) Preservation   { return Volatile }
func (Dr) Save(Platform) Preservation   { return Volatile }

// The x87 control word and the MXCSR control bits are callee-saved; the x87
// status word and the MXCSR status bits are not. Neither is a register in
// this package — they are state an Assembler would have to track, and it
// does not. Recorded here so the omission is deliberate.