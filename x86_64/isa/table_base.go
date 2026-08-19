// x86_64/isa/table_base.go
//
// The base 64-bit instruction set: everything long mode gives you before any
// extension. Gated on feature.Base, which every Set contains.
package isa

import "github.com/vertex-language/arc/x86_64/feature"

// The eight ALU operations share one shape: eight opcodes at a fixed stride,
// plus the 80/81/83 immediate group at a fixed /digit. Writing the shape once
// and the differences in a table is the same reasoning as computing feature
// closure instead of tabulating it — 160 hand-written rows can disagree with
// each other, and this cannot.
var aluOps = []struct {
	op   string
	base byte // opcode of "op r/m8, r8"; the next seven follow
	ext  int8 // /digit in the 80/81/83 group
	cmp  bool // CMP and TEST read their destination rather than writing it
}{
	{"add", 0x00, 0, false},
	{"or", 0x08, 1, false},
	{"adc", 0x10, 2, false},
	{"sbb", 0x18, 3, false},
	{"and", 0x20, 4, false},
	{"sub", 0x28, 5, false},
	{"xor", 0x30, 6, false},
	{"cmp", 0x38, 7, true},
}

func aluForms() []*Form {
	var out []*Form
	for _, a := range aluOps {
		dst := ReadWrite
		if a.cmp {
			dst = Read
		}
		lockable := func(f *Form) *Form {
			if a.cmp {
				return f
			}
			return f.lock()
		}

		out = append(out,
			// op r/m, r
			lockable(L(a.op, a.base+0, S(RM8, dst, InRM), S(R8, Read, InReg))),
			lockable(L(a.op, a.base+1, S(RM16, dst, InRM), S(R16, Read, InReg)).d16()),
			lockable(L(a.op, a.base+1, S(RM32, dst, InRM), S(R32, Read, InReg))),
			lockable(L(a.op, a.base+1, S(RM64, dst, InRM), S(R64, Read, InReg)).w1()),

			// op r, r/m
			L(a.op, a.base+2, S(R8, dst, InReg), S(RM8, Read, InRM)),
			L(a.op, a.base+3, S(R16, dst, InReg), S(RM16, Read, InRM)).d16(),
			L(a.op, a.base+3, S(R32, dst, InReg), S(RM32, Read, InRM)),
			L(a.op, a.base+3, S(R64, dst, InReg), S(RM64, Read, InRM)).w1(),

			// op eAX, imm — one byte shorter than the group form, which is
			// the whole reason it exists and the reason Resolve orders by size.
			L(a.op, a.base+4, S(AL, dst, InNone), S(Imm8, Read, InImm)),
			L(a.op, a.base+5, S(AX, dst, InNone), S(Imm16, Read, InImm)).d16(),
			L(a.op, a.base+5, S(EAX, dst, InNone), S(Imm32, Read, InImm)),
			L(a.op, a.base+5, S(RAX, dst, InNone), S(Imm32, Read, InImm)).w1(),

			// the immediate group
			lockable(L(a.op, 0x80, S(RM8, dst, InRM), S(Imm8, Read, InImm)).ext(a.ext)),
			lockable(L(a.op, 0x81, S(RM16, dst, InRM), S(Imm16, Read, InImm)).ext(a.ext).d16()),
			lockable(L(a.op, 0x81, S(RM32, dst, InRM), S(Imm32, Read, InImm)).ext(a.ext)),
			lockable(L(a.op, 0x81, S(RM64, dst, InRM), S(Imm32, Read, InImm)).ext(a.ext).w1()),

			// sign-extended imm8: four bytes shorter, and the form Resolve
			// picks for `add rax, 1`.
			lockable(L(a.op, 0x83, S(RM16, dst, InRM), S(Imm8, Read, InImm)).ext(a.ext).d16()),
			lockable(L(a.op, 0x83, S(RM32, dst, InRM), S(Imm8, Read, InImm)).ext(a.ext)),
			lockable(L(a.op, 0x83, S(RM64, dst, InRM), S(Imm8, Read, InImm)).ext(a.ext).w1()),
		)
	}
	return out
}

// Condition codes, in encoding order. The tttn value is the offset from
// 0x70 (Jcc rel8), 0x0F 0x80 (Jcc rel32), 0x0F 0x90 (SETcc) and 0x0F 0x40
// (CMOVcc), which is why one table drives four instruction families.
//
// Only canonical spellings are here. jz, jnae, setc and the rest are the same
// four encodings under other names, and an alias that reached this table
// would be a second row claiming the same bytes. text/gas and text/nasm fold
// them at the boundary.
var conditions = []string{
	"o", "no", "b", "ae", "e", "ne", "be", "a",
	"s", "ns", "p", "np", "l", "ge", "le", "g",
}

func ccForms() []*Form {
	var out []*Form
	for i, cc := range conditions {
		n := byte(i)
		out = append(out,
			L("j"+cc, 0x70+n, S(Rel8, Read, InImm)),
			L("j"+cc, 0x80+n, S(Rel32, Read, InImm)).m0F(),

			L("set"+cc, 0x90+n, S(RM8, Write, InRM)).m0F().ext(0),

			L("cmov"+cc, 0x40+n, S(R16, Write, InReg), S(RM16, Read, InRM)).m0F().d16(),
			L("cmov"+cc, 0x40+n, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F(),
			L("cmov"+cc, 0x40+n, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().w1(),
		)
	}
	return out
}

// The shift and rotate group: one opcode pair per count source, one /digit
// per operation.
var shiftOps = []struct {
	op  string
	ext int8
}{
	{"rol", 0}, {"ror", 1}, {"rcl", 2}, {"rcr", 3},
	{"shl", 4}, {"shr", 5}, {"sar", 7},
}

func shiftForms() []*Form {
	var out []*Form
	for _, s := range shiftOps {
		for _, w := range []struct {
			cls  Class
			d16  bool
			w1   bool
			byte bool
		}{
			{RM8, false, false, true},
			{RM16, true, false, false},
			{RM32, false, false, false},
			{RM64, false, true, false},
		} {
			d1, dCL, dImm := byte(0xd1), byte(0xd3), byte(0xc1)
			if w.byte {
				d1, dCL, dImm = 0xd0, 0xd2, 0xc0
			}
			mk := func(f *Form) *Form {
				if w.d16 {
					f = f.d16()
				}
				if w.w1 {
					f = f.w1()
				}
				return f
			}
			out = append(out,
				mk(L(s.op, d1, S(w.cls, ReadWrite, InRM), S(One, Read, InNone)).ext(s.ext)),
				mk(L(s.op, dCL, S(w.cls, ReadWrite, InRM), S(CL, Read, InNone)).ext(s.ext)),
				mk(L(s.op, dImm, S(w.cls, ReadWrite, InRM), S(Imm8, Read, InImm)).ext(s.ext)),
			)
		}
	}
	return out
}

// The F6/F7 unary group. MUL, IMUL, DIV and IDIV name one operand and touch
// the AX/DX pair implicitly; declaring that here is what lets a caller ask
// what an instruction clobbers without consulting a manual.
func unaryForms() []*Form {
	var out []*Form
	widths := []struct {
		rm  Class
		op  byte
		d16 bool
		w1  bool
		acc Class
		hi  Class
	}{
		{RM8, 0xf6, false, false, AL, AX},
		{RM16, 0xf7, true, false, AX, DX},
		{RM32, 0xf7, false, false, EAX, EAX},
		{RM64, 0xf7, false, true, RAX, RAX},
	}
	for _, w := range widths {
		mk := func(f *Form) *Form {
			if w.d16 {
				f = f.d16()
			}
			if w.w1 {
				f = f.w1()
			}
			return f
		}
		out = append(out,
			mk(L("test", w.op, S(w.rm, Read, InRM), S(immFor(w.rm), Read, InImm)).ext(0)),
			mk(L("not", w.op, S(w.rm, ReadWrite, InRM)).ext(2).lock()),
			mk(L("neg", w.op, S(w.rm, ReadWrite, InRM)).ext(3).lock()),
			mk(L("mul", w.op, S(w.rm, Read, InRM), Imp(w.acc, ReadWrite), Imp(w.hi, Write)).ext(4)),
			mk(L("imul", w.op, S(w.rm, Read, InRM), Imp(w.acc, ReadWrite), Imp(w.hi, Write)).ext(5)),
			mk(L("div", w.op, S(w.rm, Read, InRM), Imp(w.acc, ReadWrite), Imp(w.hi, ReadWrite)).ext(6)),
			mk(L("idiv", w.op, S(w.rm, Read, InRM), Imp(w.acc, ReadWrite), Imp(w.hi, ReadWrite)).ext(7)),
		)
	}
	return out
}

// immFor is the immediate class that pairs with an r/m width. 64-bit r/m
// takes imm32 sign-extended; there is no imm64 outside MOV.
func immFor(rm Class) Class {
	switch rm {
	case RM8:
		return Imm8
	case RM16:
		return Imm16
	}
	return Imm32
}

func baseForms() []*Form {
	out := []*Form{
		// ---- MOV ----------------------------------------------------
		L("mov", 0x88, S(RM8, Write, InRM), S(R8, Read, InReg)),
		L("mov", 0x89, S(RM16, Write, InRM), S(R16, Read, InReg)).d16(),
		L("mov", 0x89, S(RM32, Write, InRM), S(R32, Read, InReg)),
		L("mov", 0x89, S(RM64, Write, InRM), S(R64, Read, InReg)).w1(),

		L("mov", 0x8a, S(R8, Write, InReg), S(RM8, Read, InRM)),
		L("mov", 0x8b, S(R16, Write, InReg), S(RM16, Read, InRM)).d16(),
		L("mov", 0x8b, S(R32, Write, InReg), S(RM32, Read, InRM)),
		L("mov", 0x8b, S(R64, Write, InReg), S(RM64, Read, InRM)).w1(),

		L("mov", 0x8c, S(RM16, Write, InRM), S(Sreg, Read, InReg)),
		L("mov", 0x8e, S(Sreg, Write, InReg), S(RM16, Read, InRM)),

		L("mov", 0xa0, S(AL, Write, InNone), S(Moffs8, Read, InMoffs)),
		L("mov", 0xa1, S(EAX, Write, InNone), S(Moffs32, Read, InMoffs)),
		L("mov", 0xa1, S(RAX, Write, InNone), S(Moffs64, Read, InMoffs)).w1(),
		L("mov", 0xa2, S(Moffs8, Write, InMoffs), S(AL, Read, InNone)),
		L("mov", 0xa3, S(Moffs32, Write, InMoffs), S(EAX, Read, InNone)),
		L("mov", 0xa3, S(Moffs64, Write, InMoffs), S(RAX, Read, InNone)).w1(),

		L("mov", 0xb0, S(R8, Write, InOpcode), S(Imm8, Read, InImm)),
		L("mov", 0xb8, S(R16, Write, InOpcode), S(Imm16, Read, InImm)).d16(),
		L("mov", 0xb8, S(R32, Write, InOpcode), S(Imm32, Read, InImm)),
		// The only imm64 this target has. Ten bytes with REX; C7 /0 id is
		// seven and Resolve picks it whenever the value fits. MovR64Imm64 is
		// how you ask for this one anyway.
		L("mov", 0xb8, S(R64, Write, InOpcode), S(Imm64, Read, InImm)).w1(),

		L("mov", 0xc6, S(RM8, Write, InRM), S(Imm8, Read, InImm)).ext(0),
		L("mov", 0xc7, S(RM16, Write, InRM), S(Imm16, Read, InImm)).ext(0).d16(),
		L("mov", 0xc7, S(RM32, Write, InRM), S(Imm32, Read, InImm)).ext(0),
		L("mov", 0xc7, S(RM64, Write, InRM), S(Imm32, Read, InImm)).ext(0).w1(),

		// MOV to and from the privileged files. No REX.W: these are always
		// 64-bit in long mode, and CR8–CR15 come from REX.R instead.
		L("mov", 0x20, S(R64, Write, InRM), S(Cr, Read, InReg)).m0F(),
		L("mov", 0x22, S(Cr, Write, InReg), S(R64, Read, InRM)).m0F(),
		L("mov", 0x21, S(R64, Write, InRM), S(Dr, Read, InReg)).m0F(),
		L("mov", 0x23, S(Dr, Write, InReg), S(R64, Read, InRM)).m0F(),

		// ---- moves that change width -------------------------------
		L("movzx", 0xb6, S(R16, Write, InReg), S(RM8, Read, InRM)).m0F().d16(),
		L("movzx", 0xb6, S(R32, Write, InReg), S(RM8, Read, InRM)).m0F(),
		L("movzx", 0xb6, S(R64, Write, InReg), S(RM8, Read, InRM)).m0F().w1(),
		L("movzx", 0xb7, S(R32, Write, InReg), S(RM16, Read, InRM)).m0F(),
		L("movzx", 0xb7, S(R64, Write, InReg), S(RM16, Read, InRM)).m0F().w1(),

		L("movsx", 0xbe, S(R16, Write, InReg), S(RM8, Read, InRM)).m0F().d16(),
		L("movsx", 0xbe, S(R32, Write, InReg), S(RM8, Read, InRM)).m0F(),
		L("movsx", 0xbe, S(R64, Write, InReg), S(RM8, Read, InRM)).m0F().w1(),
		L("movsx", 0xbf, S(R32, Write, InReg), S(RM16, Read, InRM)).m0F(),
		L("movsx", 0xbf, S(R64, Write, InReg), S(RM16, Read, InRM)).m0F().w1(),
		// MOVSXD without REX.W is a 32-bit move that sign-extends nothing.
		// It is declared because it decodes; it is not what `movsxd` means.
		L("movsxd", 0x63, S(R64, Write, InReg), S(RM32, Read, InRM)).w1(),

		// ---- LEA ----------------------------------------------------
		// The source is memory that is never read. Width-agnostic on
		// purpose: `lea rsi, [rip+msg]` computes an address, so operand/'s
		// unsized Mem is exactly right here and nowhere else.
		L("lea", 0x8d, S(R16, Write, InReg), S(MAny, Read, InRM)).d16(),
		L("lea", 0x8d, S(R32, Write, InReg), S(MAny, Read, InRM)),
		L("lea", 0x8d, S(R64, Write, InReg), S(MAny, Read, InRM)).w1(),

		// ---- stack --------------------------------------------------
		// Default64: 64-bit operand size without REX.W, which is why
		// `push rax` is one byte.
		L("push", 0x50, S(R64, Read, InOpcode)).def64(),
		L("push", 0xff, S(RM64, Read, InRM)).ext(6).def64(),
		L("push", 0x6a, S(Imm8, Read, InImm)).def64(),
		L("push", 0x68, S(Imm32, Read, InImm)).def64(),
		L("pop", 0x58, S(R64, Write, InOpcode)).def64(),
		L("pop", 0x8f, S(RM64, Write, InRM)).ext(0).def64(),

		L("leave", 0xc9).def64(),
		L("enter", 0xc8, S(Imm16, Read, InImm), S(Imm8, Read, InImm)).def64(),

		// ---- control transfer ---------------------------------------
		L("call", 0xe8, S(Rel32, Read, InImm)).def64(),
		L("call", 0xff, S(RM64, Read, InRM)).ext(2).def64(),
		L("jmp", 0xeb, S(Rel8, Read, InImm)).def64().term(),
		L("jmp", 0xe9, S(Rel32, Read, InImm)).def64().term(),
		L("jmp", 0xff, S(RM64, Read, InRM)).ext(4).def64().term(),
		L("ret", 0xc3).def64().term(),
		L("ret", 0xc2, S(Imm16, Read, InImm)).def64().term(),

		L("syscall", 0x05).m0F(),
		L("sysret", 0x07).m0F().term(),
		L("int", 0xcd, S(Imm8, Read, InImm)),
		L("int3", 0xcc),
		L("ud2", 0x0b).m0F().term(),
		L("hlt", 0xf4).term(),

		// ---- test, exchange, compare-exchange ------------------------
		L("test", 0x84, S(RM8, Read, InRM), S(R8, Read, InReg)),
		L("test", 0x85, S(RM16, Read, InRM), S(R16, Read, InReg)).d16(),
		L("test", 0x85, S(RM32, Read, InRM), S(R32, Read, InReg)),
		L("test", 0x85, S(RM64, Read, InRM), S(R64, Read, InReg)).w1(),
		L("test", 0xa8, S(AL, Read, InNone), S(Imm8, Read, InImm)),
		L("test", 0xa9, S(EAX, Read, InNone), S(Imm32, Read, InImm)),
		L("test", 0xa9, S(RAX, Read, InNone), S(Imm32, Read, InImm)).w1(),

		L("xchg", 0x86, S(RM8, ReadWrite, InRM), S(R8, ReadWrite, InReg)).lock(),
		L("xchg", 0x87, S(RM32, ReadWrite, InRM), S(R32, ReadWrite, InReg)).lock(),
		L("xchg", 0x87, S(RM64, ReadWrite, InRM), S(R64, ReadWrite, InReg)).w1().lock(),

		L("cmpxchg", 0xb0, S(RM8, ReadWrite, InRM), S(R8, Read, InReg), Imp(AL, ReadWrite)).m0F().lock(),
		L("cmpxchg", 0xb1, S(RM32, ReadWrite, InRM), S(R32, Read, InReg), Imp(EAX, ReadWrite)).m0F().lock(),
		L("cmpxchg", 0xb1, S(RM64, ReadWrite, InRM), S(R64, Read, InReg), Imp(RAX, ReadWrite)).m0F().w1().lock(),
		L("cmpxchg16b", 0xc7, S(M128, ReadWrite, InRM),
			Imp(RAX, ReadWrite), Imp(DX, ReadWrite)).m0F().ext(1).w1().lock().
			need(feature.CMPXCHG16B),

		L("xadd", 0xc0, S(RM8, ReadWrite, InRM), S(R8, ReadWrite, InReg)).m0F().lock(),
		L("xadd", 0xc1, S(RM32, ReadWrite, InRM), S(R32, ReadWrite, InReg)).m0F().lock(),
		L("xadd", 0xc1, S(RM64, ReadWrite, InRM), S(R64, ReadWrite, InReg)).m0F().w1().lock(),

		// ---- increment, bit scan, byte swap --------------------------
		L("inc", 0xfe, S(RM8, ReadWrite, InRM)).ext(0).lock(),
		L("inc", 0xff, S(RM32, ReadWrite, InRM)).ext(0).lock(),
		L("inc", 0xff, S(RM64, ReadWrite, InRM)).ext(0).w1().lock(),
		L("dec", 0xfe, S(RM8, ReadWrite, InRM)).ext(1).lock(),
		L("dec", 0xff, S(RM32, ReadWrite, InRM)).ext(1).lock(),
		L("dec", 0xff, S(RM64, ReadWrite, InRM)).ext(1).w1().lock(),

		L("bsf", 0xbc, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F(),
		L("bsf", 0xbc, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().w1(),
		L("bsr", 0xbd, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F(),
		L("bsr", 0xbd, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().w1(),
		L("bswap", 0xc8, S(R32, ReadWrite, InOpcode)).m0F(),
		L("bswap", 0xc8, S(R64, ReadWrite, InOpcode)).m0F().w1(),

		// IMUL's two- and three-operand forms are not in the F6/F7 group,
		// because they name a destination the one-operand form does not.
		L("imul", 0xaf, S(R32, ReadWrite, InReg), S(RM32, Read, InRM)).m0F(),
		L("imul", 0xaf, S(R64, ReadWrite, InReg), S(RM64, Read, InRM)).m0F().w1(),
		L("imul", 0x6b, S(R32, Write, InReg), S(RM32, Read, InRM), S(Imm8, Read, InImm)),
		L("imul", 0x6b, S(R64, Write, InReg), S(RM64, Read, InRM), S(Imm8, Read, InImm)).w1(),
		L("imul", 0x69, S(R32, Write, InReg), S(RM32, Read, InRM), S(Imm32, Read, InImm)),
		L("imul", 0x69, S(R64, Write, InReg), S(RM64, Read, InRM), S(Imm32, Read, InImm)).w1(),

		// ---- sign extension of the accumulator ----------------------
		// One opcode, three mnemonics, distinguished only by operand size.
		// gas spells them cwtl/cltq/cltd; that is text/gas's table, not this one.
		L("cwde", 0x98),
		L("cdqe", 0x98).w1(),
		L("cdq", 0x99),
		L("cqo", 0x99).w1(),

		// ---- no-ops -------------------------------------------------
		// 0x90 is XCHG EAX, EAX with the operands folded away. The
		// multi-byte 0F 1F forms are what Align pads with; the tables that
		// choose between them live in encode/, because padding is a length
		// problem and this is a naming one.
		L("nop", 0x90),
		L("nop", 0x1f, S(RM16, Read, InRM)).m0F().ext(0).d16(),
		L("nop", 0x1f, S(RM32, Read, InRM)).m0F().ext(0),

		// ---- fences, serialization, counters -------------------------
		L("cpuid", 0xa2).m0F(),
		L("rdtsc", 0x31).m0F(),
		L("lfence", 0xae).m0F().ext(5),
		L("mfence", 0xae).m0F().ext(6),
		L("sfence", 0xae).m0F().ext(7),
		L("pause", 0x90).pF3(),

		// ---- scalar extensions above the baseline --------------------
		L("popcnt", 0xb8, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F().pF3().need(feature.POPCNT),
		L("popcnt", 0xb8, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().pF3().w1().need(feature.POPCNT),
		L("lzcnt", 0xbd, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F().pF3().need(feature.LZCNT),
		L("lzcnt", 0xbd, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().pF3().w1().need(feature.LZCNT),
		L("tzcnt", 0xbc, S(R32, Write, InReg), S(RM32, Read, InRM)).m0F().pF3().need(feature.BMI1),
		L("tzcnt", 0xbc, S(R64, Write, InReg), S(RM64, Read, InRM)).m0F().pF3().w1().need(feature.BMI1),

		L("movbe", 0xf0, S(R32, Write, InReg), S(M32, Read, InRM)).m38().need(feature.MOVBE),
		L("movbe", 0xf0, S(R64, Write, InReg), S(M64, Read, InRM)).m38().w1().need(feature.MOVBE),
		L("movbe", 0xf1, S(M32, Write, InRM), S(R32, Read, InReg)).m38().need(feature.MOVBE),
		L("movbe", 0xf1, S(M64, Write, InRM), S(R64, Read, InReg)).m38().w1().need(feature.MOVBE),

		L("lahf", 0x9f, Imp(AX, Write)).need(feature.LAHFSAHF),
		L("sahf", 0x9e, Imp(AX, Read)).need(feature.LAHFSAHF),

		L("adcx", 0xf6, S(R32, ReadWrite, InReg), S(RM32, Read, InRM)).m38().p66().need(feature.ADX),
		L("adcx", 0xf6, S(R64, ReadWrite, InReg), S(RM64, Read, InRM)).m38().p66().w1().need(feature.ADX),
		L("adox", 0xf6, S(R32, ReadWrite, InReg), S(RM32, Read, InRM)).m38().pF3().need(feature.ADX),
		L("adox", 0xf6, S(R64, ReadWrite, InReg), S(RM64, Read, InRM)).m38().pF3().w1().need(feature.ADX),

		L("rdrand", 0xc7, S(R32, Write, InRM)).m0F().ext(6).need(feature.RDRAND),
		L("rdrand", 0xc7, S(R64, Write, InRM)).m0F().ext(6).w1().need(feature.RDRAND),
		L("rdseed", 0xc7, S(R32, Write, InRM)).m0F().ext(7).need(feature.RDSEED),
		L("rdseed", 0xc7, S(R64, Write, InRM)).m0F().ext(7).w1().need(feature.RDSEED),

		L("rdfsbase", 0xae, S(R64, Write, InRM)).m0F().pF3().ext(0).w1().need(feature.FSGSBASE),
		L("rdgsbase", 0xae, S(R64, Write, InRM)).m0F().pF3().ext(1).w1().need(feature.FSGSBASE),
		L("wrfsbase", 0xae, S(R64, Read, InRM)).m0F().pF3().ext(2).w1().need(feature.FSGSBASE),
		L("wrgsbase", 0xae, S(R64, Read, InRM)).m0F().pF3().ext(3).w1().need(feature.FSGSBASE),
	}

	out = append(out, aluForms()...)
	out = append(out, ccForms()...)
	out = append(out, shiftForms()...)
	out = append(out, unaryForms()...)
	return out
}