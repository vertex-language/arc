// x86_64/reg/name.go
package reg

import "strconv"

var (
	name64 = [16]string{"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi",
		"r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15"}
	name32 = [16]string{"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi",
		"r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d"}
	name16 = [16]string{"ax", "cx", "dx", "bx", "sp", "bp", "si", "di",
		"r8w", "r9w", "r10w", "r11w", "r12w", "r13w", "r14w", "r15w"}
	name8 = [20]string{"al", "cl", "dl", "bl", "spl", "bpl", "sil", "dil",
		"r8b", "r9b", "r10b", "r11b", "r12b", "r13b", "r14b", "r15b",
		"ah", "ch", "dh", "bh"}
	nameSreg = [6]string{"es", "cs", "ss", "ds", "fs", "gs"}
)

func (r Reg64) Name() string { return name64[r] }
func (r Reg32) Name() string { return name32[r] }
func (r Reg16) Name() string { return name16[r] }
func (r Reg8) Name() string  { return name8[r] }
func (r Sreg) Name() string  { return nameSreg[r] }

func (r Xmm) Name() string { return "xmm" + strconv.Itoa(int(r)) }
func (r Ymm) Name() string { return "ymm" + strconv.Itoa(int(r)) }
func (r Zmm) Name() string { return "zmm" + strconv.Itoa(int(r)) }
func (r St) Name() string  { return "st" + strconv.Itoa(int(r)) }
func (r Mm) Name() string  { return "mm" + strconv.Itoa(int(r)) }
func (r K) Name() string   { return "k" + strconv.Itoa(int(r)) }
func (r Tmm) Name() string { return "tmm" + strconv.Itoa(int(r)) }
func (r Cr) Name() string  { return "cr" + strconv.Itoa(int(r)) }
func (r Dr) Name() string  { return "dr" + strconv.Itoa(int(r)) }

// String is Name. These are bare names: the sigil is a dialect's, so
// text/gas prints "%rax" and text/nasm prints "rax" from the same value.
func (r Reg64) String() string { return r.Name() }
func (r Reg32) String() string { return r.Name() }
func (r Reg16) String() string { return r.Name() }
func (r Reg8) String() string  { return r.Name() }
func (r Sreg) String() string  { return r.Name() }
func (r Xmm) String() string   { return r.Name() }
func (r Ymm) String() string   { return r.Name() }
func (r Zmm) String() string   { return r.Name() }
func (r St) String() string    { return r.Name() }
func (r Mm) String() string    { return r.Name() }
func (r K) String() string     { return r.Name() }
func (r Tmm) String() string   { return r.Name() }
func (r Cr) String() string    { return r.Name() }
func (r Dr) String() string    { return r.Name() }

var byName map[string]Reg

func init() {
	byName = make(map[string]Reg, 256)
	for i := range name64 {
		byName[name64[i]] = Reg64(i)
		byName[name32[i]] = Reg32(i)
		byName[name16[i]] = Reg16(i)
	}
	for i := range name8 {
		byName[name8[i]] = Reg8(i)
	}
	for i := range nameSreg {
		byName[nameSreg[i]] = Sreg(i)
	}
	for i := 0; i < 32; i++ {
		byName[Xmm(i).Name()] = Xmm(i)
		byName[Ymm(i).Name()] = Ymm(i)
		byName[Zmm(i).Name()] = Zmm(i)
	}
	for i := 0; i < 8; i++ {
		byName[St(i).Name()] = St(i)
		byName[Mm(i).Name()] = Mm(i)
		byName[K(i).Name()] = K(i)
		byName[Tmm(i).Name()] = Tmm(i)
		byName["st(" + strconv.Itoa(i) + ")"] = St(i) // gas and nasm both accept
	}
	for i := 0; i < 16; i++ {
		byName[Cr(i).Name()] = Cr(i)
		byName[Dr(i).Name()] = Dr(i)
	}
}

// Lookup resolves a bare register name. The caller has already stripped any
// dialect sigil. Names are lowercase; case folding is the lexer's job,
// because gas and nasm disagree about it.
func Lookup(s string) (Reg, bool) {
	r, ok := byName[s]
	return r, ok
}