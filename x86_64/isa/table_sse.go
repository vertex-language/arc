// x86_64/isa/table_sse.go
//
// MMX through SSE4.2, and the crypto forms that ride on SSE2. Every form here
// is legacy-encoded: the same opcode under VEX is a different row, in
// table_avx.go, because it is a different encoding and Resolve chooses
// between them by size.
package isa

import "github.com/vertex-language/arc/x86_64/feature"

func sseForms() []*Form {
	return []*Form{
		// ---- MMX ----------------------------------------------------
		L("emms", 0x77).m0F().need(feature.MMX),
		L("movq", 0x6f, S(Mm, Write, InReg), S(MmM64, Read, InRM)).m0F().need(feature.MMX),
		L("movq", 0x7f, S(MmM64, Write, InRM), S(Mm, Read, InReg)).m0F().need(feature.MMX),
		L("paddd", 0xfe, S(Mm, ReadWrite, InReg), S(MmM64, Read, InRM)).m0F().need(feature.MMX),

		// ---- SSE / SSE2 moves ----------------------------------------
		L("movups", 0x10, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("movups", 0x11, S(XmmM128, Write, InRM), S(Xmm, Read, InReg)).m0F().need(feature.SSE),
		L("movaps", 0x28, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("movaps", 0x29, S(XmmM128, Write, InRM), S(Xmm, Read, InReg)).m0F().need(feature.SSE),
		L("movapd", 0x28, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("movapd", 0x29, S(XmmM128, Write, InRM), S(Xmm, Read, InReg)).m0F().p66().need(feature.SSE2),
		L("movdqa", 0x6f, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("movdqa", 0x7f, S(XmmM128, Write, InRM), S(Xmm, Read, InReg)).m0F().p66().need(feature.SSE2),
		L("movdqu", 0x6f, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().pF3().need(feature.SSE2),
		L("movdqu", 0x7f, S(XmmM128, Write, InRM), S(Xmm, Read, InReg)).m0F().pF3().need(feature.SSE2),
		L("movss", 0x10, S(Xmm, Write, InReg), S(XmmM32, Read, InRM)).m0F().pF3().need(feature.SSE),
		L("movss", 0x11, S(XmmM32, Write, InRM), S(Xmm, Read, InReg)).m0F().pF3().need(feature.SSE),
		L("movsd", 0x10, S(Xmm, Write, InReg), S(XmmM64, Read, InRM)).m0F().pF2().need(feature.SSE2),
		L("movsd", 0x11, S(XmmM64, Write, InRM), S(Xmm, Read, InReg)).m0F().pF2().need(feature.SSE2),

		// MOVD and MOVQ across register files. Same opcode, W picks which.
		L("movd", 0x6e, S(Xmm, Write, InReg), S(RM32, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("movq", 0x6e, S(Xmm, Write, InReg), S(RM64, Read, InRM)).m0F().p66().w1().need(feature.SSE2),
		L("movd", 0x7e, S(RM32, Write, InRM), S(Xmm, Read, InReg)).m0F().p66().need(feature.SSE2),
		L("movq", 0x7e, S(RM64, Write, InRM), S(Xmm, Read, InReg)).m0F().p66().w1().need(feature.SSE2),

		// ---- SSE / SSE2 arithmetic -----------------------------------
		L("addps", 0x58, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("addss", 0x58, S(Xmm, ReadWrite, InReg), S(XmmM32, Read, InRM)).m0F().pF3().need(feature.SSE),
		L("addpd", 0x58, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("addsd", 0x58, S(Xmm, ReadWrite, InReg), S(XmmM64, Read, InRM)).m0F().pF2().need(feature.SSE2),
		L("mulps", 0x59, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("subps", 0x5c, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("divps", 0x5e, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("sqrtps", 0x51, S(Xmm, Write, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),

		L("xorps", 0x57, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("andps", 0x54, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().need(feature.SSE),
		L("pxor", 0xef, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("pand", 0xdb, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("paddb", 0xfc, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("paddw", 0xfd, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("paddd", 0xfe, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("paddq", 0xd4, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("pcmpeqb", 0x74, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m0F().p66().need(feature.SSE2),
		L("pshufd", 0x70, S(Xmm, Write, InReg), S(XmmM128, Read, InRM),
			S(Imm8, Read, InImm)).m0F().p66().need(feature.SSE2),
		L("pmovmskb", 0xd7, S(R32, Write, InReg), S(Xmm, Read, InRM)).m0F().p66().need(feature.SSE2),

		// ---- conversions ---------------------------------------------
		L("cvtsi2ss", 0x2a, S(Xmm, Write, InReg), S(RM32, Read, InRM)).m0F().pF3().need(feature.SSE),
		L("cvtsi2ss", 0x2a, S(Xmm, Write, InReg), S(RM64, Read, InRM)).m0F().pF3().w1().need(feature.SSE),
		L("cvtsi2sd", 0x2a, S(Xmm, Write, InReg), S(RM32, Read, InRM)).m0F().pF2().need(feature.SSE2),
		L("cvtsi2sd", 0x2a, S(Xmm, Write, InReg), S(RM64, Read, InRM)).m0F().pF2().w1().need(feature.SSE2),
		L("cvttss2si", 0x2c, S(R32, Write, InReg), S(XmmM32, Read, InRM)).m0F().pF3().need(feature.SSE),
		L("cvttsd2si", 0x2c, S(R64, Write, InReg), S(XmmM64, Read, InRM)).m0F().pF2().w1().need(feature.SSE2),
		L("ucomiss", 0x2e, S(Xmm, Read, InReg), S(XmmM32, Read, InRM)).m0F().need(feature.SSE),
		L("ucomisd", 0x2e, S(Xmm, Read, InReg), S(XmmM64, Read, InRM)).m0F().p66().need(feature.SSE2),

		// ---- SSSE3 / SSE4 --------------------------------------------
		L("pshufb", 0x00, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m38().p66().need(feature.SSSE3),
		L("pmulld", 0x40, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m38().p66().need(feature.SSE41),
		L("ptest", 0x17, S(Xmm, Read, InReg), S(XmmM128, Read, InRM)).m38().p66().need(feature.SSE41),
		// The blend forms read XMM0 without naming it in the encoding. It is
		// a fixed operand, written in source and carried in no field.
		L("blendvps", 0x14, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM),
			S(XMM0, Read, InNone)).m38().p66().need(feature.SSE41),
		L("pcmpistri", 0x63, S(Xmm, Read, InReg), S(XmmM128, Read, InRM),
			S(Imm8, Read, InImm)).m3A().p66().need(feature.SSE42),
		L("crc32", 0xf0, S(R32, ReadWrite, InReg), S(RM8, Read, InRM)).m38().pF2().need(feature.SSE42),
		L("crc32", 0xf1, S(R32, ReadWrite, InReg), S(RM32, Read, InRM)).m38().pF2().need(feature.SSE42),
		L("crc32", 0xf1, S(R64, ReadWrite, InReg), S(RM64, Read, InRM)).m38().pF2().w1().need(feature.SSE42),

		// ---- crypto ---------------------------------------------------
		L("aesenc", 0xdc, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m38().p66().need(feature.AES),
		L("aesenclast", 0xdd, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM)).m38().p66().need(feature.AES),
		L("aeskeygenassist", 0xdf, S(Xmm, Write, InReg), S(XmmM128, Read, InRM),
			S(Imm8, Read, InImm)).m3A().p66().need(feature.AES),
		L("pclmulqdq", 0x44, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM),
			S(Imm8, Read, InImm)).m3A().p66().need(feature.PCLMULQDQ),
		L("sha256rnds2", 0xcb, S(Xmm, ReadWrite, InReg), S(XmmM128, Read, InRM),
			S(XMM0, Read, InNone)).m38().need(feature.SHA),
	}
}