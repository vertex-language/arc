// x86_64/isa/table_avx512.go
//
// EVEX-encoded forms, and the AMX tile forms — which are VEX-encoded despite
// belonging to a later extension, because a tile register is not a vector
// register and needs none of what EVEX adds.
//
// Every EVEX form that takes memory states a tuple: N is a function of the
// tuple, the vector length and EVEX.b, and encode/ computes it. A form with
// no tuple and a memory slot does not survive init.
package isa

import "github.com/vertex-language/arc/x86_64/feature"

func avx512Forms() []*Form {
	return []*Form{
		// VPADDD at three lengths. The 128- and 256-bit rows need AVX512VL
		// on top of AVX512F; the 512-bit row does not, and that asymmetry is
		// exactly what the feature gate is for.
		E("vpaddd", L128, Pfx66, Map0F, 0xfe,
			S(Xmm, Write, InReg), S(K, Read, InMask),
			S(Xmm, Read, InVVVV), S(XmmM128, Read, InRM)).
			tup(TupleFull).bcst(4).mask().need(feature.AVX512VL),
		E("vpaddd", L256, Pfx66, Map0F, 0xfe,
			S(Ymm, Write, InReg), S(K, Read, InMask),
			S(Ymm, Read, InVVVV), S(YmmM256, Read, InRM)).
			tup(TupleFull).bcst(4).mask().need(feature.AVX512VL),
		E("vpaddd", L512, Pfx66, Map0F, 0xfe,
			S(Zmm, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			tup(TupleFull).bcst(4).mask().need(feature.AVX512F),

		// VADDPS takes embedded rounding at 512 bits, which is EVEX.b again
		// under a different reading — the same bit means broadcast on a
		// memory operand and rounding control on a register one.
		E("vaddps", L512, PfxNone, Map0F, 0x58,
			S(Zmm, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			tup(TupleFull).bcst(4).mask().er().need(feature.AVX512F),
		E("vaddpd", L512, Pfx66, Map0F, 0x58,
			S(Zmm, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			w1().tup(TupleFull).bcst(8).mask().er().need(feature.AVX512F),

		E("vmovdqa32", L512, Pfx66, Map0F, 0x6f,
			S(Zmm, Write, InReg), S(K, Read, InMask), S(ZmmM512, Read, InRM)).
			tup(TupleFullMem).mask().need(feature.AVX512F),
		E("vmovdqa32", L512, Pfx66, Map0F, 0x7f,
			S(ZmmM512, Write, InRM), S(K, Read, InMask), S(Zmm, Read, InReg)).
			tup(TupleFullMem).merge().need(feature.AVX512F),
		E("vmovdqa64", L512, Pfx66, Map0F, 0x6f,
			S(Zmm, Write, InReg), S(K, Read, InMask), S(ZmmM512, Read, InRM)).
			w1().tup(TupleFullMem).mask().need(feature.AVX512F),

		// A scalar form: Tuple1 Scalar, no broadcast, and the memory operand
		// is 32 bits wide inside a 512-bit instruction.
		E("vaddss", L128, PfxF3, Map0F, 0x58,
			S(Xmm, Write, InReg), S(K, Read, InMask),
			S(Xmm, Read, InVVVV), S(XmmM32, Read, InRM)).
			tup(Tuple1Scalar).mask().er().need(feature.AVX512F),

		E("vpcmpeqd", L512, Pfx66, Map0F, 0x76,
			S(K, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			tup(TupleFull).bcst(4).merge().need(feature.AVX512F),

		// Mask-register instructions: VEX, not EVEX, and they never mask.
		V("kmovw", L128, PfxNone, Map0F, 0x90, S(K, Write, InReg), S(KM64, Read, InRM)).
			need(feature.AVX512F),
		V("kmovq", L128, PfxNone, Map0F, 0x90, S(K, Write, InReg), S(KM64, Read, InRM)).
			w1().need(feature.AVX512BW),
		V("kortestw", L128, PfxNone, Map0F, 0x98, S(K, Read, InReg), S(K, Read, InRM)).
			need(feature.AVX512F),

		E("vpaddb", L512, Pfx66, Map0F, 0xfc,
			S(Zmm, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			tup(TupleFullMem).mask().need(feature.AVX512BW),
		E("vpandd", L512, Pfx66, Map0F, 0xdb,
			S(Zmm, Write, InReg), S(K, Read, InMask),
			S(Zmm, Read, InVVVV), S(ZmmM512, Read, InRM)).
			tup(TupleFull).bcst(4).mask().need(feature.AVX512DQ),

		// ---- AMX ------------------------------------------------------
		// Tile shape is set at run time by LDTILECFG, so the operand carries
		// no width the table can check. Bits() answers 8192, the
		// architectural maximum, and nothing here reads it.
		V("ldtilecfg", L128, PfxNone, Map0F38, 0x49, S(MAny, Read, InRM)).
			ext(0).need(feature.AMXTILE),
		V("tilerelease", L128, PfxNone, Map0F38, 0x49).need(feature.AMXTILE),
		V("tilezero", L128, PfxF2, Map0F38, 0x49, S(Tmm, Write, InReg)).
			need(feature.AMXTILE),
		V("tileloadd", L128, PfxF2, Map0F38, 0x4b,
			S(Tmm, Write, InReg), S(MAny, Read, InRM)).need(feature.AMXTILE),
		V("tilestored", L128, PfxF3, Map0F38, 0x4b,
			S(MAny, Write, InRM), S(Tmm, Read, InReg)).need(feature.AMXTILE),
		V("tdpbssd", L128, PfxF2, Map0F38, 0x5e,
			S(Tmm, ReadWrite, InReg), S(Tmm, Read, InVVVV), S(Tmm, Read, InRM)).
			need(feature.AMXINT8),
		V("tdpbf16ps", L128, PfxF3, Map0F38, 0x5c,
			S(Tmm, ReadWrite, InReg), S(Tmm, Read, InVVVV), S(Tmm, Read, InRM)).
			need(feature.AMXBF16),
	}
}