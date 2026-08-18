package isa

import "github.com/vertex-language/arc/i386/feature"

// The base integer instruction set: everything gated by a base CPU level and
// nothing gated by an extension. This is what an i686 baseline assembles
// without any --features flag.
//
// Extension tranches are additional rows in additional files — table_mmx.go,
// table_sse.go — added the same way and gated by feature.Feature. Nothing
// about a row changes when they land.
//
// Opcode 0x82 is deliberately absent. In 32-bit mode it is a working duplicate
// of 0x80 with the same length and the same semantics, which would give Emit
// two candidates of equal cost and make the tie-break decide something the SDM
// lists as reserved. GNU as does not emit it either.

// alu is the eight-operator group that occupies opcodes 0x00 through 0x3f in a
// regular pattern, plus the /digit slots of 0x80, 0x81 and 0x83. The order is
// the encoding's own: the nth operator's base opcode is 8n and its extension
// digit is n.
var alu = []struct {
	name string
	n    byte
}{
	{"add", 0}, {"or", 1}, {"adc", 2}, {"sbb", 3},
	{"and", 4}, {"sub", 5}, {"xor", 6}, {"cmp", 7},
}

// shift is the group at 0xc0/0xc1, 0xd0/0xd1 and 0xd2/0xd3, by /digit.
// Digit 6 is an undocumented second SAL and is not declared.
var shift = []struct {
	name string
	n    int8
}{
	{"rol", 0}, {"ror", 1}, {"rcl", 2}, {"rcr", 3},
	{"shl", 4}, {"shr", 5}, {"sar", 7},
}

// cond is the condition code suffix table shared by Jcc, SETcc and CMOVcc.
// The tttn value is the low nibble of the opcode. Where the SDM documents two
// spellings for one encoding, both are declared and the second is an alias.
var cond = []struct {
	name  string
	tttn  byte
	alias string
}{
	{"o", 0x0, ""}, {"no", 0x1, ""},
	{"b", 0x2, ""}, {"c", 0x2, "b"}, {"nae", 0x2, "b"},
	{"ae", 0x3, ""}, {"nb", 0x3, "ae"}, {"nc", 0x3, "ae"},
	{"e", 0x4, ""}, {"z", 0x4, "e"},
	{"ne", 0x5, ""}, {"nz", 0x5, "ne"},
	{"be", 0x6, ""}, {"na", 0x6, "be"},
	{"a", 0x7, ""}, {"nbe", 0x7, "a"},
	{"s", 0x8, ""}, {"ns", 0x9, ""},
	{"p", 0xa, ""}, {"pe", 0xa, "p"},
	{"np", 0xb, ""}, {"po", 0xb, "np"},
	{"l", 0xc, ""}, {"nge", 0xc, "l"},
	{"ge", 0xd, ""}, {"nl", 0xd, "ge"},
	{"le", 0xe, ""}, {"ng", 0xe, "le"},
	{"g", 0xf, ""}, {"nle", 0xf, "g"},
}

// forms is every form this package knows, in table order. Emit breaks ties by
// this order, so it is part of the deterministic-output guarantee: rows are
// appended, never reordered.
var forms []*Form

func add(f *Form) { forms = append(forms, f) }

// r builds a Form with the common defaults.
func r(mn string, opcode []byte, ext int8, level feature.Level, ops ...Op) *Form {
	return &Form{Mnemonic: mn, Opcode: opcode, Ext: ext, Level: level, Ops: ops}
}

func op(c Class, s Slot) Op { return Op{Class: c, Slot: s} }

func init() {
	buildALU()
	buildMov()
	buildStack()
	buildArith()
	buildShift()
	buildBranch()
	buildCond()
	buildMisc()
	buildSystem()
}

func buildALU() {
	for _, a := range alu {
		b := a.n * 8
		// The r/m and reg direction pair, byte and doubleword.
		add(r(a.name, []byte{b + 0}, NoExt, feature.I386, op(RM8, SlotRM), op(R8, SlotReg)))
		add(r(a.name, []byte{b + 1}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
		add(r(a.name, []byte{b + 2}, NoExt, feature.I386, op(R8, SlotReg), op(RM8, SlotRM)))
		add(r(a.name, []byte{b + 3}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM)))

		// The accumulator short forms. These are one byte shorter than the
		// group-1 encoding and are a different form, not an optimisation.
		add(r(a.name, []byte{b + 4}, NoExt, feature.I386, op(AL, SlotFixed), op(Imm8, SlotImm)))
		add(r(a.name, []byte{b + 5}, NoExt, feature.I386, op(EAX, SlotFixed), op(Imm32, SlotImm)))

		// Group 1. The imm8 form is four bytes shorter than the imm32 form
		// and sign-extends; both are legal for a small constant, which is
		// exactly the choice Emit makes and the typed helpers do not.
		add(r(a.name, []byte{0x80}, int8(a.n), feature.I386, op(RM8, SlotRM), op(Imm8, SlotImm)))
		add(r(a.name, []byte{0x83}, int8(a.n), feature.I386, op(RM32, SlotRM), op(Imm8S, SlotImm)))
		add(r(a.name, []byte{0x81}, int8(a.n), feature.I386, op(RM32, SlotRM), op(Imm32, SlotImm)))
	}
}

func buildMov() {
	add(r("mov", []byte{0x88}, NoExt, feature.I386, op(RM8, SlotRM), op(R8, SlotReg)))
	add(r("mov", []byte{0x89}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("mov", []byte{0x8a}, NoExt, feature.I386, op(R8, SlotReg), op(RM8, SlotRM)))
	add(r("mov", []byte{0x8b}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM)))

	// The +rd immediate forms are one byte shorter than the C7 group form,
	// which is why MOV r32, imm32 and MOV r/m32, imm32 are separate names.
	add(r("mov", []byte{0xb0}, NoExt, feature.I386, op(R8, SlotOpcode), op(Imm8, SlotImm)))
	add(r("mov", []byte{0xb8}, NoExt, feature.I386, op(R32, SlotOpcode), op(Imm32, SlotImm)))
	add(r("mov", []byte{0xc6}, 0, feature.I386, op(RM8, SlotRM), op(Imm8, SlotImm)))
	add(r("mov", []byte{0xc7}, 0, feature.I386, op(RM32, SlotRM), op(Imm32, SlotImm)))

	add(r("mov", []byte{0x8c}, NoExt, feature.I386, op(RM32, SlotRM), op(Sreg, SlotReg)))
	add(r("mov", []byte{0x8e}, NoExt, feature.I386, op(Sreg, SlotReg), op(RM32, SlotRM)))

	add(r("lea", []byte{0x8d}, NoExt, feature.I386, op(R32, SlotReg), op(M, SlotRM)))

	add(r("movzx", []byte{0x0f, 0xb6}, NoExt, feature.I386, op(R32, SlotReg), op(RM8, SlotRM)))
	add(r("movzx", []byte{0x0f, 0xb7}, NoExt, feature.I386, op(R32, SlotReg), op(RM16, SlotRM)))
	add(r("movsx", []byte{0x0f, 0xbe}, NoExt, feature.I386, op(R32, SlotReg), op(RM8, SlotRM)))
	add(r("movsx", []byte{0x0f, 0xbf}, NoExt, feature.I386, op(R32, SlotReg), op(RM16, SlotRM)))

	add(r("xchg", []byte{0x90}, NoExt, feature.I386, op(EAX, SlotFixed), op(R32, SlotOpcode)))
	add(r("xchg", []byte{0x86}, NoExt, feature.I386, op(RM8, SlotRM), op(R8, SlotReg)))
	add(r("xchg", []byte{0x87}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))

	add(r("bswap", []byte{0x0f, 0xc8}, NoExt, feature.I486, op(R32, SlotOpcode)))

	add(r("cmpxchg", []byte{0x0f, 0xb0}, NoExt, feature.I486, op(RM8, SlotRM), op(R8, SlotReg)))
	add(r("cmpxchg", []byte{0x0f, 0xb1}, NoExt, feature.I486, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("xadd", []byte{0x0f, 0xc0}, NoExt, feature.I486, op(RM8, SlotRM), op(R8, SlotReg)))
	add(r("xadd", []byte{0x0f, 0xc1}, NoExt, feature.I486, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("cmpxchg8b", []byte{0x0f, 0xc7}, 1, feature.I586, op(RM64, SlotRM)))
}

func buildStack() {
	add(r("push", []byte{0x50}, NoExt, feature.I386, op(R32, SlotOpcode)))
	add(r("push", []byte{0xff}, 6, feature.I386, op(RM32, SlotRM)))
	add(r("push", []byte{0x6a}, NoExt, feature.I386, op(Imm8S, SlotImm)))
	add(r("push", []byte{0x68}, NoExt, feature.I386, op(Imm32, SlotImm)))
	add(r("pop", []byte{0x58}, NoExt, feature.I386, op(R32, SlotOpcode)))
	add(r("pop", []byte{0x8f}, 0, feature.I386, op(RM32, SlotRM)))

	add(r("pusha", []byte{0x60}, NoExt, feature.I386))
	add(r("popa", []byte{0x61}, NoExt, feature.I386))
	add(r("pushf", []byte{0x9c}, NoExt, feature.I386))
	add(r("popf", []byte{0x9d}, NoExt, feature.I386))

	add(r("enter", []byte{0xc8}, NoExt, feature.I386, op(Imm16, SlotImm), op(Imm8, SlotImm)))
	add(r("leave", []byte{0xc9}, NoExt, feature.I386))
}

func buildArith() {
	// Group 3.
	add(r("test", []byte{0xf6}, 0, feature.I386, op(RM8, SlotRM), op(Imm8, SlotImm)))
	add(r("test", []byte{0xf7}, 0, feature.I386, op(RM32, SlotRM), op(Imm32, SlotImm)))
	add(r("not", []byte{0xf6}, 2, feature.I386, op(RM8, SlotRM)))
	add(r("not", []byte{0xf7}, 2, feature.I386, op(RM32, SlotRM)))
	add(r("neg", []byte{0xf6}, 3, feature.I386, op(RM8, SlotRM)))
	add(r("neg", []byte{0xf7}, 3, feature.I386, op(RM32, SlotRM)))
	add(r("mul", []byte{0xf6}, 4, feature.I386, op(RM8, SlotRM)))
	add(r("mul", []byte{0xf7}, 4, feature.I386, op(RM32, SlotRM)))
	add(r("imul", []byte{0xf6}, 5, feature.I386, op(RM8, SlotRM)))
	add(r("imul", []byte{0xf7}, 5, feature.I386, op(RM32, SlotRM)))
	add(r("div", []byte{0xf6}, 6, feature.I386, op(RM8, SlotRM)))
	add(r("div", []byte{0xf7}, 6, feature.I386, op(RM32, SlotRM)))
	add(r("idiv", []byte{0xf6}, 7, feature.I386, op(RM8, SlotRM)))
	add(r("idiv", []byte{0xf7}, 7, feature.I386, op(RM32, SlotRM)))

	// The two- and three-operand IMULs are different instructions sharing a
	// mnemonic, not addressing modes of one.
	add(r("imul", []byte{0x0f, 0xaf}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM)))
	add(r("imul", []byte{0x6b}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM), op(Imm8S, SlotImm)))
	add(r("imul", []byte{0x69}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM), op(Imm32, SlotImm)))

	// Group 4 and 5.
	add(r("inc", []byte{0x40}, NoExt, feature.I386, op(R32, SlotOpcode)))
	add(r("inc", []byte{0xfe}, 0, feature.I386, op(RM8, SlotRM)))
	add(r("inc", []byte{0xff}, 0, feature.I386, op(RM32, SlotRM)))
	add(r("dec", []byte{0x48}, NoExt, feature.I386, op(R32, SlotOpcode)))
	add(r("dec", []byte{0xfe}, 1, feature.I386, op(RM8, SlotRM)))
	add(r("dec", []byte{0xff}, 1, feature.I386, op(RM32, SlotRM)))

	add(r("test", []byte{0x84}, NoExt, feature.I386, op(RM8, SlotRM), op(R8, SlotReg)))
	add(r("test", []byte{0x85}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("test", []byte{0xa8}, NoExt, feature.I386, op(AL, SlotFixed), op(Imm8, SlotImm)))
	add(r("test", []byte{0xa9}, NoExt, feature.I386, op(EAX, SlotFixed), op(Imm32, SlotImm)))

	add(r("cwde", []byte{0x98}, NoExt, feature.I386))
	add(r("cdq", []byte{0x99}, NoExt, feature.I386))
}

func buildShift() {
	for _, s := range shift {
		add(r(s.name, []byte{0xd0}, s.n, feature.I386, op(RM8, SlotRM), op(One, SlotFixed)))
		add(r(s.name, []byte{0xd1}, s.n, feature.I386, op(RM32, SlotRM), op(One, SlotFixed)))
		add(r(s.name, []byte{0xd2}, s.n, feature.I386, op(RM8, SlotRM), op(CL, SlotFixed)))
		add(r(s.name, []byte{0xd3}, s.n, feature.I386, op(RM32, SlotRM), op(CL, SlotFixed)))
		add(r(s.name, []byte{0xc0}, s.n, feature.I386, op(RM8, SlotRM), op(Imm8, SlotImm)))
		add(r(s.name, []byte{0xc1}, s.n, feature.I386, op(RM32, SlotRM), op(Imm8, SlotImm)))
	}
	// SAL is the ARM-ARM-style case: the SDM documents it as a second name
	// for the SHL encoding, so it exists and emits SHL.
	for _, opc := range [][3]interface{}{
		{[]byte{0xd0}, RM8, One}, {[]byte{0xd1}, RM32, One},
		{[]byte{0xd2}, RM8, CL}, {[]byte{0xd3}, RM32, CL},
	} {
		f := r("sal", opc[0].([]byte), 4, feature.I386,
			op(opc[1].(Class), SlotRM), op(opc[2].(Class), SlotFixed))
		f.AliasOf = "shl"
		add(f)
	}
	for _, opc := range [][2]interface{}{
		{[]byte{0xc0}, RM8}, {[]byte{0xc1}, RM32},
	} {
		f := r("sal", opc[0].([]byte), 4, feature.I386,
			op(opc[1].(Class), SlotRM), op(Imm8, SlotImm))
		f.AliasOf = "shl"
		add(f)
	}

	add(r("shld", []byte{0x0f, 0xa4}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg), op(Imm8, SlotImm)))
	add(r("shld", []byte{0x0f, 0xa5}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg), op(CL, SlotFixed)))
	add(r("shrd", []byte{0x0f, 0xac}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg), op(Imm8, SlotImm)))
	add(r("shrd", []byte{0x0f, 0xad}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg), op(CL, SlotFixed)))
}

func buildBranch() {
	add(r("jmp", []byte{0xeb}, NoExt, feature.I386, op(Rel8, SlotRel)))
	add(r("jmp", []byte{0xe9}, NoExt, feature.I386, op(Rel32, SlotRel)))
	add(r("jmp", []byte{0xff}, 4, feature.I386, op(RM32, SlotRM)))

	add(r("call", []byte{0xe8}, NoExt, feature.I386, op(Rel32, SlotRel)))
	add(r("call", []byte{0xff}, 2, feature.I386, op(RM32, SlotRM)))

	add(r("ret", []byte{0xc3}, NoExt, feature.I386))
	add(r("ret", []byte{0xc2}, NoExt, feature.I386, op(Imm16, SlotImm)))

	add(r("loop", []byte{0xe2}, NoExt, feature.I386, op(Rel8, SlotRel)))
	add(r("loope", []byte{0xe1}, NoExt, feature.I386, op(Rel8, SlotRel)))
	add(r("loopne", []byte{0xe0}, NoExt, feature.I386, op(Rel8, SlotRel)))
	add(r("jecxz", []byte{0xe3}, NoExt, feature.I386, op(Rel8, SlotRel)))
}

func buildCond() {
	for _, c := range cond {
		// Jcc rel8 and rel32 are different forms; the short one is three
		// bytes shorter and Emit picks it when the target is in range.
		j8 := r("j"+c.name, []byte{0x70 + c.tttn}, NoExt, feature.I386, op(Rel8, SlotRel))
		j32 := r("j"+c.name, []byte{0x0f, 0x80 + c.tttn}, NoExt, feature.I386, op(Rel32, SlotRel))
		set := r("set"+c.name, []byte{0x0f, 0x90 + c.tttn}, NoExt, feature.I386, op(RM8, SlotRM))

		// CMOVcc is the instruction that defines the i686 level.
		cmov := r("cmov"+c.name, []byte{0x0f, 0x40 + c.tttn}, NoExt, feature.I686,
			op(R32, SlotReg), op(RM32, SlotRM))

		if c.alias != "" {
			j8.AliasOf, j32.AliasOf = "j"+c.alias, "j"+c.alias
			set.AliasOf = "set" + c.alias
			cmov.AliasOf = "cmov" + c.alias
		}
		add(j8)
		add(j32)
		add(set)
		add(cmov)
	}
}

func buildMisc() {
	add(r("nop", []byte{0x90}, NoExt, feature.I386))
	add(r("int3", []byte{0xcc}, NoExt, feature.I386))
	add(r("int", []byte{0xcd}, NoExt, feature.I386, op(Imm8, SlotImm)))
	add(r("into", []byte{0xce}, NoExt, feature.I386))
	add(r("iret", []byte{0xcf}, NoExt, feature.I386))
	add(r("hlt", []byte{0xf4}, NoExt, feature.I386))
	add(r("cld", []byte{0xfc}, NoExt, feature.I386))
	add(r("std", []byte{0xfd}, NoExt, feature.I386))
	add(r("clc", []byte{0xf8}, NoExt, feature.I386))
	add(r("stc", []byte{0xf9}, NoExt, feature.I386))
	add(r("cmc", []byte{0xf5}, NoExt, feature.I386))
	add(r("lahf", []byte{0x9f}, NoExt, feature.I386))
	add(r("sahf", []byte{0x9e}, NoExt, feature.I386))

	add(r("bt", []byte{0x0f, 0xa3}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("bts", []byte{0x0f, 0xab}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("btr", []byte{0x0f, 0xb3}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("btc", []byte{0x0f, 0xbb}, NoExt, feature.I386, op(RM32, SlotRM), op(R32, SlotReg)))
	add(r("bsf", []byte{0x0f, 0xbc}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM)))
	add(r("bsr", []byte{0x0f, 0xbd}, NoExt, feature.I386, op(R32, SlotReg), op(RM32, SlotRM)))

	add(r("in", []byte{0xe4}, NoExt, feature.I386, op(AL, SlotFixed), op(Imm8, SlotImm)))
	add(r("in", []byte{0xec}, NoExt, feature.I386, op(AL, SlotFixed), op(DX, SlotFixed)))
	add(r("out", []byte{0xe6}, NoExt, feature.I386, op(Imm8, SlotImm), op(AL, SlotFixed)))
	add(r("out", []byte{0xee}, NoExt, feature.I386, op(DX, SlotFixed), op(AL, SlotFixed)))
}

func buildSystem() {
	// Control and debug register moves. These are the reason reg declares Cr
	// and Dr: the encoding exists at the i386 level and takes a ModRM byte
	// with mod fixed at 11.
	add(r("mov", []byte{0x0f, 0x20}, NoExt, feature.I386, op(R32, SlotRM), op(Cr, SlotReg)))
	add(r("mov", []byte{0x0f, 0x22}, NoExt, feature.I386, op(Cr, SlotReg), op(R32, SlotRM)))
	add(r("mov", []byte{0x0f, 0x21}, NoExt, feature.I386, op(R32, SlotRM), op(Dr, SlotReg)))
	add(r("mov", []byte{0x0f, 0x23}, NoExt, feature.I386, op(Dr, SlotReg), op(R32, SlotRM)))

	add(r("invd", []byte{0x0f, 0x08}, NoExt, feature.I486))
	add(r("wbinvd", []byte{0x0f, 0x09}, NoExt, feature.I486))
	add(r("invlpg", []byte{0x0f, 0x01}, 7, feature.I486, op(M, SlotRM)))

	add(r("cpuid", []byte{0x0f, 0xa2}, NoExt, feature.I586))
	add(r("rdtsc", []byte{0x0f, 0x31}, NoExt, feature.I586))
	add(r("rdmsr", []byte{0x0f, 0x32}, NoExt, feature.I586))
	add(r("wrmsr", []byte{0x0f, 0x30}, NoExt, feature.I586))
	add(r("rdpmc", []byte{0x0f, 0x33}, NoExt, feature.I686))
}