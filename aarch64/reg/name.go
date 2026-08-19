package reg

import (
	"strconv"
	"strings"
)

// Name is the architectural spelling of a register, lower case, as GNU as and
// the LLVM integrated assembler write it.
func Name(r Reg) string { return r.String() }

func (r X) String() string {
	if r == XZR {
		return "xzr"
	}
	return "x" + strconv.Itoa(int(r))
}

func (r W) String() string {
	if r == WZR {
		return "wzr"
	}
	return "w" + strconv.Itoa(int(r))
}

func (r Xsp) String() string {
	if r == SP {
		return "sp"
	}
	return "x" + strconv.Itoa(int(r))
}

func (r Wsp) String() string {
	if r == WSP {
		return "wsp"
	}
	return "w" + strconv.Itoa(int(r))
}

func (r V) String() string { return "v" + strconv.Itoa(int(r)) }
func (r Q) String() string { return "q" + strconv.Itoa(int(r)) }
func (r D) String() string { return "d" + strconv.Itoa(int(r)) }
func (r S) String() string { return "s" + strconv.Itoa(int(r)) }
func (r H) String() string { return "h" + strconv.Itoa(int(r)) }
func (r B) String() string { return "b" + strconv.Itoa(int(r)) }
func (r Z) String() string { return "z" + strconv.Itoa(int(r)) }
func (r P) String() string { return "p" + strconv.Itoa(int(r)) }

func (r Ffr) String() string { return "ffr" }

func (e Elem) String() string {
	switch e {
	case ElemB:
		return "b"
	case ElemH:
		return "h"
	case ElemS:
		return "s"
	case ElemD:
		return "d"
	}
	return ""
}

func (a Arrangement) String() string {
	if a == ArrNone {
		return ""
	}
	return strconv.Itoa(int(a.Lanes())) + a.Elem().String()
}

func (v Vec) String() string { return v.R.String() + "." + v.A.String() }

func (l VLane) String() string {
	return l.R.String() + "." + l.E.String() + "[" + strconv.Itoa(int(l.Index)) + "]"
}

func (s Sys) String() string {
	if n, ok := sysName[s]; ok {
		return n
	}
	return "s" + strconv.Itoa(int(s.Op0())) +
		"_" + strconv.Itoa(int(s.Op1())) +
		"_c" + strconv.Itoa(int(s.CRn())) +
		"_c" + strconv.Itoa(int(s.CRm())) +
		"_" + strconv.Itoa(int(s.Op2()))
}

var sysName = map[Sys]string{
	NZCV: "nzcv", DAIF: "daif", CurrentEL: "currentel", SPSel: "spsel",
	FPCR: "fpcr", FPSR: "fpsr",
	TPIDR_EL0: "tpidr_el0", TPIDRRO_EL0: "tpidrro_el0", TPIDR_EL1: "tpidr_el1",
	TPIDR_EL2: "tpidr_el2", TPIDR_EL3: "tpidr_el3",
	MIDR_EL1: "midr_el1", MPIDR_EL1: "mpidr_el1", CTR_EL0: "ctr_el0",
	DCZID_EL0: "dczid_el0", CNTVCT_EL0: "cntvct_el0",
	SCTLR_EL1: "sctlr_el1", TTBR0_EL1: "ttbr0_el1", TTBR1_EL1: "ttbr1_el1",
	TCR_EL1: "tcr_el1", ESR_EL1: "esr_el1", FAR_EL1: "far_el1",
	VBAR_EL1: "vbar_el1", ELR_EL1: "elr_el1", SPSR_EL1: "spsr_el1",
}

var sysByName = func() map[string]Sys {
	m := make(map[string]Sys, len(sysName))
	for s, n := range sysName {
		m[n] = s
	}
	return m
}()

// Lookup resolves an architectural register name. It is case-insensitive,
// because assembly source is.
//
// This is the fixed table only. GNU as lets .req add a name and .unreq remove
// one, including the built-in ones, so a source file's set of valid register
// names is a moving target that only the text layer can track. Lookup is what
// that layer starts from, not what it consults.
//
// A number 31 spelled x31 or w31 is rejected: the architecture writes it as xzr
// or sp, and which of the two is meant is a question this function has no way
// to answer.
func Lookup(name string) (Reg, bool) {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return nil, false
	}

	switch s {
	case "sp":
		return SP, true
	case "wsp":
		return WSP, true
	case "xzr":
		return XZR, true
	case "wzr":
		return WZR, true
	case "ffr":
		return FFR, true
	}

	if r, ok := sysByName[s]; ok {
		return r, true
	}
	if r, ok := parseGenericSys(s); ok {
		return r, true
	}

	prefix, n, ok := splitIndex(s)
	if !ok {
		return nil, false
	}
	switch prefix {
	case "x":
		if n <= 30 {
			return X(n), true
		}
	case "w":
		if n <= 30 {
			return W(n), true
		}
	case "v":
		if n <= 31 {
			return V(n), true
		}
	case "q":
		if n <= 31 {
			return Q(n), true
		}
	case "d":
		if n <= 31 {
			return D(n), true
		}
	case "s":
		if n <= 31 {
			return S(n), true
		}
	case "h":
		if n <= 31 {
			return H(n), true
		}
	case "b":
		if n <= 31 {
			return B(n), true
		}
	case "z":
		if n <= 31 {
			return Z(n), true
		}
	case "p":
		if n <= 15 {
			return P(n), true
		}
	}
	return nil, false
}

// splitIndex divides a name into its leading letters and trailing digits.
// A leading zero on the number is rejected, so x01 is not a spelling of x1.
func splitIndex(s string) (prefix string, n int, ok bool) {
	i := 0
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		i++
	}
	if i == 0 || i == len(s) {
		return "", 0, false
	}
	digits := s[i:]
	if len(digits) > 1 && digits[0] == '0' {
		return "", 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", 0, false
	}
	return s[:i], n, true
}

// parseGenericSys reads the S<op0>_<op1>_<Cn>_<Cm>_<op2> spelling the
// architecture provides for registers an assembler has no name for.
func parseGenericSys(s string) (Sys, bool) {
	if len(s) == 0 || s[0] != 's' {
		return 0, false
	}
	parts := strings.Split(s[1:], "_")
	if len(parts) != 5 {
		return 0, false
	}
	f := make([]uint8, 5)
	for i, p := range parts {
		if i == 2 || i == 3 {
			if len(p) < 2 || p[0] != 'c' {
				return 0, false
			}
			p = p[1:]
		}
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return 0, false
		}
		limit := 7
		switch i {
		case 0:
			limit = 3
		case 2, 3:
			limit = 15
		}
		if v > limit {
			return 0, false
		}
		f[i] = uint8(v)
	}
	return NewSys(f[0], f[1], f[2], f[3], f[4]), true
}