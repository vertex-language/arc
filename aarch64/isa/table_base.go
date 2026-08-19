package isa

// The base integer, branch and load/store instruction set.
//
// Base words and masks are the ARM ARM's, one row per declared encoding per
// width. The 32- and 64-bit variants are separate rows rather than one row with
// a varying sf bit, because they are separate forms to a caller: AddImm32 and
// AddImm64 take different register types and a width mismatch should be a
// compile error at the typed call rather than a runtime check.
//
// This table is the part of the tree that must be differential-tested rather
// than reviewed. Every row here is checked against GNU as by internal/difftest.
func init() {
	register(
		// ---- Arithmetic, immediate ----
		L("add", 0x11000000, 0x7f800000).
			Dst(ClassWsp, Rd).Src(ClassWsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Name("AddImm32"),
		L("add", 0x91000000, 0x7f800000).
			Dst(ClassXsp, Rd).Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Name("AddImm64"),
		L("adds", 0x31000000, 0x7f800000).
			Dst(ClassW, Rd).Src(ClassWsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Flags().Name("AddsImm32"),
		L("adds", 0xb1000000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Flags().Name("AddsImm64"),
		L("sub", 0x51000000, 0x7f800000).
			Dst(ClassWsp, Rd).Src(ClassWsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Name("SubImm32"),
		L("sub", 0xd1000000, 0x7f800000).
			Dst(ClassXsp, Rd).Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Name("SubImm64"),
		L("subs", 0x71000000, 0x7f800000).
			Dst(ClassW, Rd).Src(ClassWsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Flags().Name("SubsImm32"),
		L("subs", 0xf1000000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Flags().Name("SubsImm64"),

		// ---- Arithmetic, shifted register ----
		L("add", 0x0b000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Name("AddShifted32"),
		L("add", 0x8b000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Name("AddShifted64"),
		L("adds", 0x2b000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("AddsShifted32"),
		L("adds", 0xab000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("AddsShifted64"),
		L("sub", 0x4b000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Name("SubShifted32"),
		L("sub", 0xcb000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Name("SubShifted64"),
		L("subs", 0x6b000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("SubsShifted32"),
		L("subs", 0xeb000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("SubsShifted64"),

		// ---- Arithmetic, extended register ----
		L("add", 0x0b200000, 0x7fe00000).
			Dst(ClassWsp, Rd).Src(ClassWsp, Rn).Src(ClassW, Rm).Opt(ClassExtend, Opt, 2).
			Name("AddExt32"),
		L("add", 0x8b200000, 0x7fe00000).
			Dst(ClassXsp, Rd).Src(ClassXsp, Rn).Src(ClassX, Rm).Opt(ClassExtend, Opt, 3).
			Name("AddExt64"),
		L("subs", 0xeb200000, 0x7fe00000).
			Dst(ClassX, Rd).Src(ClassXsp, Rn).Src(ClassX, Rm).Opt(ClassExtend, Opt, 3).
			Flags().Name("SubsExt64"),

		// ---- Logical, shifted register ----
		L("and", 0x0a000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Name("AndShifted32"),
		L("and", 0x8a000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Name("AndShifted64"),
		L("orr", 0x2a000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Name("OrrShifted32"),
		L("orr", 0xaa000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Name("OrrShifted64"),
		L("eor", 0x4a000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Name("EorShifted32"),
		L("eor", 0xca000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Name("EorShifted64"),
		L("ands", 0x6a000000, 0x7f200000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("AndsShifted32"),
		L("ands", 0xea000000, 0x7f200000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().Name("AndsShifted64"),

		// ---- Logical, bitmask immediate ----
		// The immediate is the N:immr:imms triple operand/bitmask.go computes.
		// Whether a constant is expressible at all is that encoder's question,
		// asked before this form is ever reached.
		L("and", 0x12000000, 0x7f800000).
			Dst(ClassWsp, Rd).Src(ClassW, Rn).Imm(Imms).
			Name("AndImm32"),
		L("and", 0x92000000, 0x7f800000).
			Dst(ClassXsp, Rd).Src(ClassX, Rn).Imm(Imms).
			Name("AndImm64"),
		L("orr", 0x32000000, 0x7f800000).
			Dst(ClassWsp, Rd).Src(ClassW, Rn).Imm(Imms).
			Name("OrrImm32"),
		L("orr", 0xb2000000, 0x7f800000).
			Dst(ClassXsp, Rd).Src(ClassX, Rn).Imm(Imms).
			Name("OrrImm64"),
		L("eor", 0x52000000, 0x7f800000).
			Dst(ClassWsp, Rd).Src(ClassW, Rn).Imm(Imms).
			Name("EorImm32"),
		L("eor", 0xd2000000, 0x7f800000).
			Dst(ClassXsp, Rd).Src(ClassX, Rn).Imm(Imms).
			Name("EorImm64"),

		// ---- Move wide ----
		L("movz", 0x52800000, 0x7f800000).
			Dst(ClassW, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovzImm32"),
		L("movz", 0xd2800000, 0x7f800000).
			Dst(ClassX, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovzImm64"),
		L("movn", 0x12800000, 0x7f800000).
			Dst(ClassW, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovnImm32"),
		L("movn", 0x92800000, 0x7f800000).
			Dst(ClassX, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovnImm64"),
		L("movk", 0x72800000, 0x7f800000).
			SrcDst(ClassW, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovkImm32"),
		L("movk", 0xf2800000, 0x7f800000).
			SrcDst(ClassX, Rd).Imm(Imm16).Opt(ClassShift, Hw, 0).
			Name("MovkImm64"),

		// ---- PC-relative address ----
		L("adr", 0x10000000, 0x9f000000).
			Dst(ClassX, Rd).Addr(RoleTarget, ImmPCRel).
			Name("Adr"),
		L("adrp", 0x90000000, 0x9f000000).
			Dst(ClassX, Rd).Addr(RolePage, ImmPCRel).
			Name("Adrp"),

		// ---- Bitfield ----
		L("ubfm", 0x53000000, 0x7f800000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Imm(Immr).Imm(Imms).Name("Ubfm32"),
		L("ubfm", 0xd3400000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).Imm(Imms).Name("Ubfm64"),
		L("sbfm", 0x13000000, 0x7f800000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Imm(Immr).Imm(Imms).Name("Sbfm32"),
		L("sbfm", 0x93400000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).Imm(Imms).Name("Sbfm64"),
		L("bfm", 0x33000000, 0x7f800000).
			SrcDst(ClassW, Rd).Src(ClassW, Rn).Imm(Immr).Imm(Imms).Name("Bfm32"),
		L("bfm", 0xb3400000, 0x7f800000).
			SrcDst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).Imm(Imms).Name("Bfm64"),
		L("extr", 0x13800000, 0x7fa00000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Imm(Imms).Name("Extr32"),
		L("extr", 0x93c00000, 0x7fa00000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Imm(Imms).Name("Extr64"),

		// ---- Variable shift, multiply, divide ----
		L("lslv", 0x1ac02000, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Lslv32"),
		L("lslv", 0x9ac02000, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Lslv64"),
		L("lsrv", 0x1ac02400, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Lsrv32"),
		L("lsrv", 0x9ac02400, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Lsrv64"),
		L("asrv", 0x1ac02800, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Asrv32"),
		L("asrv", 0x9ac02800, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Asrv64"),
		L("rorv", 0x1ac02c00, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Rorv32"),
		L("rorv", 0x9ac02c00, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Rorv64"),
		L("udiv", 0x1ac00800, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Udiv32"),
		L("udiv", 0x9ac00800, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Udiv64"),
		L("sdiv", 0x1ac00c00, 0x7fe0fc00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Name("Sdiv32"),
		L("sdiv", 0x9ac00c00, 0x7fe0fc00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Name("Sdiv64"),
		L("madd", 0x1b000000, 0x7fe08000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Src(ClassW, Ra).Name("Madd32"),
		L("madd", 0x9b000000, 0x7fe08000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Src(ClassX, Ra).Name("Madd64"),
		L("msub", 0x1b008000, 0x7fe08000).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Src(ClassW, Ra).Name("Msub32"),
		L("msub", 0x9b008000, 0x7fe08000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Src(ClassX, Ra).Name("Msub64"),

		// ---- Conditional select and compare ----
		L("csel", 0x1a800000, 0x7fe00c00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Cnd(CondHi).Name("Csel32"),
		L("csel", 0x9a800000, 0x7fe00c00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Cnd(CondHi).Name("Csel64"),
		L("csinc", 0x1a800400, 0x7fe00c00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Cnd(CondHi).Name("Csinc32"),
		L("csinc", 0x9a800400, 0x7fe00c00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Cnd(CondHi).Name("Csinc64"),
		L("csinv", 0x5a800000, 0x7fe00c00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Cnd(CondHi).Name("Csinv32"),
		L("csinv", 0xda800000, 0x7fe00c00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Cnd(CondHi).Name("Csinv64"),
		L("csneg", 0x5a800400, 0x7fe00c00).
			Dst(ClassW, Rd).Src(ClassW, Rn).Src(ClassW, Rm).Cnd(CondHi).Name("Csneg32"),
		L("csneg", 0xda800400, 0x7fe00c00).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).Cnd(CondHi).Name("Csneg64"),

		// ---- Data processing, one source ----
		L("rbit", 0x5ac00000, 0x7ffffc00).Dst(ClassW, Rd).Src(ClassW, Rn).Name("Rbit32"),
		L("rbit", 0xdac00000, 0x7ffffc00).Dst(ClassX, Rd).Src(ClassX, Rn).Name("Rbit64"),
		L("rev16", 0x5ac00400, 0x7ffffc00).Dst(ClassW, Rd).Src(ClassW, Rn).Name("Rev16_32"),
		L("rev16", 0xdac00400, 0x7ffffc00).Dst(ClassX, Rd).Src(ClassX, Rn).Name("Rev16_64"),
		L("rev", 0x5ac00800, 0x7ffffc00).Dst(ClassW, Rd).Src(ClassW, Rn).Name("Rev32"),
		L("rev", 0xdac00c00, 0x7ffffc00).Dst(ClassX, Rd).Src(ClassX, Rn).Name("Rev64"),
		L("clz", 0x5ac01000, 0x7ffffc00).Dst(ClassW, Rd).Src(ClassW, Rn).Name("Clz32"),
		L("clz", 0xdac01000, 0x7ffffc00).Dst(ClassX, Rd).Src(ClassX, Rn).Name("Clz64"),
		L("cls", 0x5ac01400, 0x7ffffc00).Dst(ClassW, Rd).Src(ClassW, Rn).Name("Cls32"),
		L("cls", 0xdac01400, 0x7ffffc00).Dst(ClassX, Rd).Src(ClassX, Rn).Name("Cls64"),

		// ---- Branches ----
		L("b", 0x14000000, 0xfc000000).Target(Imm26).Name("B"),
		L("bl", 0x94000000, 0xfc000000).Target(Imm26).Name("Bl"),
		L("b.cond", 0x54000000, 0xff000010).Cnd(Cond).Target(Imm19).Name("BCond"),
		L("cbz", 0x34000000, 0x7f000000).Src(ClassW, Rt).Target(Imm19).Name("Cbz32"),
		L("cbz", 0xb4000000, 0x7f000000).Src(ClassX, Rt).Target(Imm19).Name("Cbz64"),
		L("cbnz", 0x35000000, 0x7f000000).Src(ClassW, Rt).Target(Imm19).Name("Cbnz32"),
		L("cbnz", 0xb5000000, 0x7f000000).Src(ClassX, Rt).Target(Imm19).Name("Cbnz64"),
		L("tbz", 0x36000000, 0x7f000000).
			Src(ClassX, Rt).Imm(BitPos).Target(Imm14).Name("Tbz"),
		L("tbnz", 0x37000000, 0x7f000000).
			Src(ClassX, Rt).Imm(BitPos).Target(Imm14).Name("Tbnz"),
		L("br", 0xd61f0000, 0xfffffc1f).Src(ClassX, Rn).Name("Br"),
		L("blr", 0xd63f0000, 0xfffffc1f).Src(ClassX, Rn).Name("Blr"),
		// RET's operand is optional and defaults to X30. That default is the
		// architecture's, stated in the encoding, not an alias.
		L("ret", 0xd65f0000, 0xfffffc1f).OptReg(ClassX, Rn, 30).Name("Ret"),

		// ---- Load and store, unsigned scaled offset ----
		L("strb", 0x39000000, 0xffc00000).
			Src(ClassW, Rt).Mem(8, Rn, Imm12).Attr(AttrScaled).Name("StrbImm"),
		L("ldrb", 0x39400000, 0xffc00000).
			Dst(ClassW, Rt).Mem(8, Rn, Imm12).Attr(AttrScaled).Name("LdrbImm"),
		L("strh", 0x79000000, 0xffc00000).
			Src(ClassW, Rt).Mem(16, Rn, Imm12).Attr(AttrScaled).Name("StrhImm"),
		L("ldrh", 0x79400000, 0xffc00000).
			Dst(ClassW, Rt).Mem(16, Rn, Imm12).Attr(AttrScaled).Name("LdrhImm"),
		L("str", 0xb9000000, 0xffc00000).
			Src(ClassW, Rt).Mem(32, Rn, Imm12).Attr(AttrScaled).Name("StrImm32"),
		L("ldr", 0xb9400000, 0xffc00000).
			Dst(ClassW, Rt).Mem(32, Rn, Imm12).Attr(AttrScaled).Name("LdrImm32"),
		L("str", 0xf9000000, 0xffc00000).
			Src(ClassX, Rt).Mem(64, Rn, Imm12).Attr(AttrScaled).Name("StrImm64"),
		L("ldr", 0xf9400000, 0xffc00000).
			Dst(ClassX, Rt).Mem(64, Rn, Imm12).Attr(AttrScaled).Name("LdrImm64"),
		L("ldrsw", 0xb9800000, 0xffc00000).
			Dst(ClassX, Rt).Mem(32, Rn, Imm12).Attr(AttrScaled).Name("LdrswImm"),

		// ---- Load and store, unscaled ----
		L("stur", 0xb8000000, 0xffe00c00).
			Src(ClassW, Rt).Mem(32, Rn, Imm9).Name("SturImm32"),
		L("ldur", 0xb8400000, 0xffe00c00).
			Dst(ClassW, Rt).Mem(32, Rn, Imm9).Name("LdurImm32"),
		L("stur", 0xf8000000, 0xffe00c00).
			Src(ClassX, Rt).Mem(64, Rn, Imm9).Name("SturImm64"),
		L("ldur", 0xf8400000, 0xffe00c00).
			Dst(ClassX, Rt).Mem(64, Rn, Imm9).Name("LdurImm64"),

		// ---- Load and store pair ----
		L("stp", 0x29000000, 0xffc00000).
			Src(ClassW, Rt).Src(ClassW, Rt2).Mem(32, Rn, Imm7).
			Attr(AttrScaled).Name("Stp32"),
		L("ldp", 0x29400000, 0xffc00000).
			Dst(ClassW, Rt).Dst(ClassW, Rt2).Mem(32, Rn, Imm7).
			Attr(AttrScaled).Name("Ldp32"),
		L("stp", 0xa9000000, 0xffc00000).
			Src(ClassX, Rt).Src(ClassX, Rt2).Mem(64, Rn, Imm7).
			Attr(AttrScaled).Name("Stp64"),
		L("ldp", 0xa9400000, 0xffc00000).
			Dst(ClassX, Rt).Dst(ClassX, Rt2).Mem(64, Rn, Imm7).
			Attr(AttrScaled).Name("Ldp64"),
		L("stp", 0xa9800000, 0xffc00000).
			Src(ClassX, Rt).Src(ClassX, Rt2).Mem(64, Rn, Imm7).
			Attr(AttrScaled | AttrPreIndex).Name("StpPre64"),
		L("ldp", 0xa8c00000, 0xffc00000).
			Dst(ClassX, Rt).Dst(ClassX, Rt2).Mem(64, Rn, Imm7).
			Attr(AttrScaled | AttrPostIndex).Name("LdpPost64"),

		// ---- Load literal ----
		L("ldr", 0x18000000, 0xff000000).
			Dst(ClassW, Rt).Target(Imm19).Name("LdrLit32"),
		L("ldr", 0x58000000, 0xff000000).
			Dst(ClassX, Rt).Target(Imm19).Name("LdrLit64"),

		// ---- Exceptions, hints, barriers ----
		L("svc", 0xd4000001, 0xffe0001f).Imm(Imm16).Name("Svc"),
		L("hvc", 0xd4000002, 0xffe0001f).Imm(Imm16).Name("Hvc"),
		L("smc", 0xd4000003, 0xffe0001f).Imm(Imm16).Name("Smc"),
		L("brk", 0xd4200000, 0xffe0001f).Imm(Imm16).Name("Brk"),
		L("hlt", 0xd4400000, 0xffe0001f).Imm(Imm16).Name("Hlt"),

		L("nop", 0xd503201f, 0xffffffff).Name("Nop"),
		L("yield", 0xd503203f, 0xffffffff).Name("Yield"),
		L("wfe", 0xd503205f, 0xffffffff).Name("Wfe"),
		L("wfi", 0xd503207f, 0xffffffff).Name("Wfi"),
		L("sev", 0xd503209f, 0xffffffff).Name("Sev"),
		L("sevl", 0xd50320bf, 0xffffffff).Name("Sevl"),

		L("dsb", 0xd503309f, 0xfffff0ff).Opt(ClassBarrier, CRm, 15).Name("Dsb"),
		L("dmb", 0xd50330bf, 0xfffff0ff).Opt(ClassBarrier, CRm, 15).Name("Dmb"),
		L("isb", 0xd50330df, 0xfffff0ff).Opt(ClassBarrier, CRm, 15).Name("Isb"),

		// ---- System register move ----
		L("mrs", 0xd5300000, 0xfff00000).
			Dst(ClassX, Rt).SysReg(SysReg).Name("Mrs"),
		L("msr", 0xd5100000, 0xfff00000).
			SysReg(SysReg).Src(ClassX, Rt).Name("MsrReg"),
	)

	// ---- The architecture's aliases ----
	//
	// Each is one-to-one with a word of the instruction it aliases, and each
	// pins the field that makes it narrower. The preferred-disassembly
	// predicates are the ARM ARM's, and they are what make arc dis agree with
	// objdump.
	register(
		L("cmp", 0xeb00001f, 0x7f20001f).
			Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().AliasOf("subs").Pins(Rd, 31).Name("CmpShifted64"),
		L("cmp", 0xf100001f, 0x7f80001f).
			Src(ClassXsp, Rn).Imm(Imm12).Opt(ClassShift, Sh, 0).
			Flags().AliasOf("subs").Pins(Rd, 31).Name("CmpImm64"),
		L("cmn", 0xab00001f, 0x7f20001f).
			Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().AliasOf("adds").Pins(Rd, 31).Name("CmnShifted64"),
		L("tst", 0xea00001f, 0x7f20001f).
			Src(ClassX, Rn).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			Flags().AliasOf("ands").Pins(Rd, 31).Name("TstShifted64"),
		L("neg", 0xcb0003e0, 0x7f2003e0).
			Dst(ClassX, Rd).Src(ClassX, Rm).Opt(ClassShift, Shift, 0).
			AliasOf("sub").Pins(Rn, 31).Name("NegShifted64"),

		// MOV (register) is ORR with the zero register as its first source.
		// It is preferred only when no shift is applied; ORR with a shift is
		// not a move and the ARM ARM says so.
		L("mov", 0xaa0003e0, 0x7fe0ffe0).
			Dst(ClassX, Rd).Src(ClassX, Rm).
			AliasOf("orr").Pins(Rn, 31).Name("MovReg64"),
		L("mov", 0x2a0003e0, 0x7fe0ffe0).
			Dst(ClassW, Rd).Src(ClassW, Rm).
			AliasOf("orr").Pins(Rn, 31).Name("MovReg32"),

		// MOV (to/from SP) is ADD with a zero immediate, and is preferred only
		// when one of the two registers really is SP. Otherwise the word is a
		// plain add of zero and prints as one.
		L("mov", 0x91000000, 0xff8003ff).
			Dst(ClassXsp, Rd).Src(ClassXsp, Rn).
			AliasOf("add").Pins(Imm12, 0).
			PreferredWhen(func(w uint32) bool {
				return Rd.Get(w) == 31 || Rn.Get(w) == 31
			}).Name("MovSp64"),

		// MOV (wide immediate) is MOVZ, preferred unless the immediate is zero
		// and shifted — movz x0, #0, lsl #16 is not a move of anything.
		L("mov", 0xd2800000, 0x7f800000).
			Dst(ClassX, Rd).Imm(Imm16).
			AliasOf("movz").
			PreferredWhen(func(w uint32) bool {
				return !(Imm16.Get(w) == 0 && Hw.Get(w) != 0)
			}).Name("MovWide64"),

		// The shift aliases are UBFM and SBFM with computed immediates. The
		// computation is encode/'s; the alias relation is the architecture's.
		L("lsl", 0xd3400000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).
			AliasOf("ubfm").Name("LslImm64"),
		L("lsr", 0xd3400000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).
			AliasOf("ubfm").Name("LsrImm64"),
		L("asr", 0x93400000, 0x7f800000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Imm(Immr).
			AliasOf("sbfm").Name("AsrImm64"),

		L("cset", 0x9a9f07e0, 0x7fe0ffe0).
			Dst(ClassX, Rd).Cnd(CondHi).
			AliasOf("csinc").Pins(Rn, 31).Pins(Rm, 31).Name("Cset64"),
		L("mul", 0x9b007c00, 0x7fe08000).
			Dst(ClassX, Rd).Src(ClassX, Rn).Src(ClassX, Rm).
			AliasOf("madd").Pins(Ra, 31).Name("Mul64"),

		// .inst states a word rather than naming an instruction. It is the one
		// case where emitting bytes nobody selected is what was asked for.
		L(".inst", 0, 0).Imm(F(0, 32)).Name("Inst"),
	)

	checkTable()
}