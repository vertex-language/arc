// x86_64/isa/table_avx.go
//
// VEX-encoded forms. The three-operand shape is the point of the encoding:
// the destination is ModRM.reg, the first source is VEX.vvvv, the second is
// ModRM.rm — which is why `vaddps` does not clobber either input and `addps`
// does. Two rows, two encodings, one mnemonic apart.
package isa

import "github.com/vertex-language/arc/x86_64/feature"

func avxForms() []*Form {
	return []*Form{
		// ---- AVX: the SSE set, non-destructive ------------------------
		V("vmovups", L128, PfxNone, Map0F, 0x10,
			S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).wig().need(feature.AVX),
		V("vmovups", L256, PfxNone, Map0F, 0x10,
			S(Ymm, Write, InReg), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vmovaps", L128, PfxNone, Map0F, 0x28,
			S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).wig().need(feature.AVX),
		V("vmovaps", L256, PfxNone, Map0F, 0x28,
			S(Ymm, Write, InReg), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vmovaps", L256, PfxNone, Map0F, 0x29,
			S(YmmM256, Write, InRM), S(Ymm, Read, InReg)).wig().need(feature.AVX),
		V("vmovdqa", L256, Pfx66, Map0F, 0x6f,
			S(Ymm, Write, InReg), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vmovdqu", L256, PfxF3, Map0F, 0x6f,
			S(Ymm, Write, InReg), S(YmmM256, Read, InRM)).wig().need(feature.AVX),

		V("vaddps", L128, PfxNone, Map0F, 0x58,
			S(Xmm, Write, InReg), S(Xmm, Read, InVVVV), S(XmmM128, Read, InRM)).wig().need(feature.AVX),
		V("vaddps", L256, PfxNone, Map0F, 0x58,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vaddpd", L256, Pfx66, Map0F, 0x58,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vaddss", L128, PfxF3, Map0F, 0x58,
			S(Xmm, Write, InReg), S(Xmm, Read, InVVVV), S(XmmM32, Read, InRM)).wig().need(feature.AVX),
		V("vmulps", L256, PfxNone, Map0F, 0x59,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX),
		V("vxorps", L256, PfxNone, Map0F, 0x57,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX),

		// VZEROUPPER is VEX.128 with no operands at all — the length field
		// still has to be right, which is why LNone is not an option here.
		V("vzeroupper", L128, PfxNone, Map0F, 0x77).wig().need(feature.AVX),
		V("vzeroall", L256, PfxNone, Map0F, 0x77).wig().need(feature.AVX),

		V("vbroadcastss", L256, Pfx66, Map0F38, 0x18,
			S(Ymm, Write, InReg), S(XmmM32, Read, InRM)).need(feature.AVX),

		// The is4 forms: a fourth operand in the high nibble of the
		// immediate byte. Nothing else on this target encodes an operand
		// there, and it is the reason Field has an InIS4 at all.
		V("vblendvps", L256, Pfx66, Map0F3A, 0x4a,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV),
			S(YmmM256, Read, InRM), S(Ymm, Read, InIS4)).need(feature.AVX),

		// ---- AVX2 -----------------------------------------------------
		V("vpaddd", L128, Pfx66, Map0F, 0xfe,
			S(Xmm, Write, InReg), S(Xmm, Read, InVVVV), S(XmmM128, Read, InRM)).wig().need(feature.AVX),
		V("vpaddd", L256, Pfx66, Map0F, 0xfe,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX2),
		V("vpxor", L256, Pfx66, Map0F, 0xef,
			S(Ymm, Write, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).wig().need(feature.AVX2),
		V("vpermq", L256, Pfx66, Map0F3A, 0x00,
			S(Ymm, Write, InReg), S(YmmM256, Read, InRM), S(Imm8, Read, InImm)).w1().need(feature.AVX2),
		V("vpbroadcastd", L256, Pfx66, Map0F38, 0x58,
			S(Ymm, Write, InReg), S(XmmM32, Read, InRM)).need(feature.AVX2),

		// ---- F16C and FMA ---------------------------------------------
		V("vcvtph2ps", L256, Pfx66, Map0F38, 0x13,
			S(Ymm, Write, InReg), S(XmmM128, Read, InRM)).need(feature.F16C),
		V("vfmadd132ps", L256, Pfx66, Map0F38, 0x98,
			S(Ymm, ReadWrite, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).need(feature.FMA),
		V("vfmadd213ps", L256, Pfx66, Map0F38, 0xa8,
			S(Ymm, ReadWrite, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).need(feature.FMA),
		V("vfmadd231ps", L256, Pfx66, Map0F38, 0xb8,
			S(Ymm, ReadWrite, InReg), S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).need(feature.FMA),

		// ---- BMI1 and BMI2: VEX around general-purpose registers -------
		// LZ, not L128: the length field must be zero and a decoder that
		// read L128 here would accept an encoding the silicon refuses.
		V("andn", LZ, PfxNone, Map0F38, 0xf2,
			S(R32, Write, InReg), S(R32, Read, InVVVV), S(RM32, Read, InRM)).need(feature.BMI1),
		V("andn", LZ, PfxNone, Map0F38, 0xf2,
			S(R64, Write, InReg), S(R64, Read, InVVVV), S(RM64, Read, InRM)).w1().need(feature.BMI1),
		V("blsr", LZ, PfxNone, Map0F38, 0xf3,
			S(R64, Write, InVVVV), S(RM64, Read, InRM)).ext(1).w1().need(feature.BMI1),
		V("blsi", LZ, PfxNone, Map0F38, 0xf3,
			S(R64, Write, InVVVV), S(RM64, Read, InRM)).ext(3).w1().need(feature.BMI1),
		V("bzhi", LZ, PfxNone, Map0F38, 0xf5,
			S(R64, Write, InReg), S(RM64, Read, InRM), S(R64, Read, InVVVV)).w1().need(feature.BMI2),
		V("pdep", LZ, PfxF2, Map0F38, 0xf5,
			S(R64, Write, InReg), S(R64, Read, InVVVV), S(RM64, Read, InRM)).w1().need(feature.BMI2),
		V("pext", LZ, PfxF3, Map0F38, 0xf5,
			S(R64, Write, InReg), S(R64, Read, InVVVV), S(RM64, Read, InRM)).w1().need(feature.BMI2),
		V("mulx", LZ, PfxF2, Map0F38, 0xf6,
			S(R64, Write, InReg), S(R64, Write, InVVVV), S(RM64, Read, InRM),
			Imp(RAX, Read)).w1().need(feature.BMI2),
		V("rorx", LZ, PfxF2, Map0F3A, 0xf0,
			S(R64, Write, InReg), S(RM64, Read, InRM), S(Imm8, Read, InImm)).w1().need(feature.BMI2),
		V("shlx", LZ, Pfx66, Map0F38, 0xf7,
			S(R64, Write, InReg), S(RM64, Read, InRM), S(R64, Read, InVVVV)).w1().need(feature.BMI2),
		V("sarx", LZ, PfxF3, Map0F38, 0xf7,
			S(R64, Write, InReg), S(RM64, Read, InRM), S(R64, Read, InVVVV)).w1().need(feature.BMI2),
		V("shrx", LZ, PfxF2, Map0F38, 0xf7,
			S(R64, Write, InReg), S(RM64, Read, InRM), S(R64, Read, InVVVV)).w1().need(feature.BMI2),
	}
}