package object

import (
	"math"
	"strings"
	"testing"
	"time"
)

// describe returns an object's type and printed form, or "ERROR: <message>"
// for an error, for comparing results in tables.
func describe(o Object) string {
	if err, ok := o.(*Error); ok {
		return "ERROR: " + err.Message
	}
	return string(o.Type()) + " " + o.Inspect()
}

// checkResult compares a result with want. A want starting with "ERROR:"
// matches any error whose message contains the rest of want.
func checkResult(t *testing.T, name string, got Object, want string) {
	t.Helper()
	d := describe(got)
	if rest, isErr := strings.CutPrefix(want, "ERROR:"); isErr {
		if !strings.HasPrefix(d, "ERROR: ") || !strings.Contains(d, strings.TrimSpace(rest)) {
			t.Errorf("%s: expected an error containing %q, got %s", name, strings.TrimSpace(rest), d)
		}
		return
	}
	if d != want {
		t.Errorf("%s: expected %s, got %s", name, want, d)
	}
}

type infixCase struct {
	name  string
	left  Object
	op    string
	right Object
	want  string
}

func runInfixCases(t *testing.T, cases []infixCase) {
	t.Helper()
	for _, c := range cases {
		checkResult(t, c.name+" "+c.op, EvalInfix(c.left, c.op, c.right), c.want)
	}
}

func TestSignedIntegerInfix(t *testing.T) {
	i := func(v int16) Object { return &Int{Value: v} }
	l := func(v int64) Object { return &LInt{Value: v} }
	runInfixCases(t, []infixCase{
		{"add", i(3), "+", i(4), "INT 7"},
		{"sub", i(3), "-", i(4), "INT -1"},
		{"mul", i(3), "*", i(4), "INT 12"},
		{"div", i(9), "/", i(2), "INT 4"},
		{"mod", i(9), "MOD", i(4), "INT 1"},
		{"lt", i(3), "<", i(4), "BOOLEAN true"},
		{"gt", i(3), ">", i(4), "BOOLEAN false"},
		{"le", i(3), "<=", i(4), "BOOLEAN true"},
		{"ge", i(3), "GE", i(4), "BOOLEAN false"},
		{"eq", i(3), "=", i(3), "BOOLEAN true"},
		{"ne", i(3), "<>", i(3), "BOOLEAN false"},
		{"unknown", i(3), "**", i(3), "ERROR: unknown operator for signed integers: **"},
		{"div zero", i(3), "/", i(0), "ERROR: division by zero"},
		{"mod zero", i(3), "MOD", i(0), "ERROR: division by zero in MOD"},
		// The result must fit the result type.
		{"sint overflow", &SInt{Value: 100}, "+", &SInt{Value: 100}, "ERROR: SINT overflow: 200"},
		{"sint underflow", &SInt{Value: -100}, "-", &SInt{Value: 100}, "ERROR: SINT underflow: -200"},
		// 64-bit overflow is caught before it wraps.
		{"add overflow", l(math.MaxInt64), "+", l(1), "ERROR: signed integer overflow"},
		{"add negative overflow", l(math.MinInt64), "+", l(-1), "ERROR: signed integer overflow"},
		{"sub underflow", l(math.MinInt64), "-", l(1), "ERROR: signed integer underflow"},
		{"sub negative underflow", l(math.MaxInt64), "-", l(-1), "ERROR: signed integer underflow"},
		{"mul overflow", l(math.MaxInt64), "*", l(2), "ERROR: signed integer overflow"},
		{"mul negative overflow", l(math.MaxInt64), "*", l(-2), "ERROR: signed integer overflow"},
		{"div overflow", l(math.MinInt64), "/", l(-1), "ERROR: signed integer overflow"},
	})
}

func TestUnsignedIntegerInfix(t *testing.T) {
	u := func(v uint64) Object { return &ULInt{Value: v} }
	runInfixCases(t, []infixCase{
		{"add", &UInt{Value: 3}, "+", &UInt{Value: 4}, "UINT 7"},
		{"sub", u(9), "-", u(4), "ULINT 5"},
		{"mul", u(3), "*", u(4), "ULINT 12"},
		{"div", u(9), "/", u(2), "ULINT 4"},
		{"mod", u(9), "MOD", u(4), "ULINT 1"},
		{"lt", u(3), "LT", u(4), "BOOLEAN true"},
		{"gt", u(3), ">", u(4), "BOOLEAN false"},
		{"eq", u(3), "EQ", u(3), "BOOLEAN true"},
		{"ne", u(3), "NE", u(3), "BOOLEAN false"},
		{"le", u(3), "<=", u(3), "BOOLEAN true"},
		{"ge", u(3), ">=", u(4), "BOOLEAN false"},
		{"unknown", u(3), "**", u(3), "ERROR: unknown operator for unsigned integers: **"},
		{"underflow", u(3), "-", u(4), "ERROR: unsigned integer underflow"},
		{"overflow", u(math.MaxUint64), "+", u(1), "ERROR: unsigned integer overflow"},
		{"mul overflow", u(math.MaxUint64), "*", u(2), "ERROR: unsigned integer overflow"},
		{"div zero", u(3), "/", u(0), "ERROR: division by zero"},
		{"mod zero", u(3), "MOD", u(0), "ERROR: division by zero in MOD"},
		{"usint overflow", &USInt{Value: 200}, "+", &USInt{Value: 100}, "ERROR: USINT overflow: 300"},
	})
}

func TestMixedIntegerInfix(t *testing.T) {
	runInfixCases(t, []infixCase{
		// The wider type wins; equal widths of mixed sign widen to the next signed type.
		{"sint+int", &SInt{Value: 1}, "+", &Int{Value: 2}, "INT 3"},
		{"sint+usint", &SInt{Value: -1}, "+", &USInt{Value: 2}, "INT 1"},
		{"int+uint", &Int{Value: -1}, "+", &UInt{Value: 2}, "DINT 1"},
		{"dint+udint", &DInt{Value: -1}, "+", &UDInt{Value: 2}, "LINT 1"},
		{"lint+ulint", &LInt{Value: -1}, "+", &ULInt{Value: 2}, "LINT 1"},
		// A signed operand with a wider unsigned one gives the unsigned type.
		{"int+udint", &Int{Value: 5}, "+", &UDInt{Value: 3}, "UDINT 8"},
		{"int-udint negative", &Int{Value: 1}, "-", &UDInt{Value: 3}, "ERROR: UDINT underflow: -2"},
		{"int+dint", &Int{Value: 5}, "+", &DInt{Value: 3}, "DINT 8"},
	})
}

func TestGetResultIntegerType(t *testing.T) {
	tests := []struct{ a, b, want ObjectType }{
		{INT_OBJ, INT_OBJ, INT_OBJ},
		{REAL_OBJ, INT_OBJ, INT_OBJ}, // A non-integer type defers to the other.
		{INT_OBJ, REAL_OBJ, INT_OBJ},
		{UDINT_OBJ, SINT_OBJ, UDINT_OBJ},
		{ULINT_OBJ, LINT_OBJ, LINT_OBJ},
	}
	for _, tt := range tests {
		if got := getResultIntegerType(tt.a, tt.b); got != tt.want {
			t.Errorf("getResultIntegerType(%s, %s) = %s, want %s", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckAndCreateIntegerObject(t *testing.T) {
	tests := []struct {
		name       string
		typ        ObjectType
		val        int64
		uval       uint64
		isUnsigned bool
		want       string
	}{
		{"sint", SINT_OBJ, -5, 0, false, "SINT -5"},
		{"int", INT_OBJ, 300, 0, false, "INT 300"},
		{"dint", DINT_OBJ, 70000, 0, false, "DINT 70000"},
		{"lint", LINT_OBJ, -1, 0, false, "LINT -1"},
		{"usint", USINT_OBJ, 0, 200, true, "USINT 200"},
		{"uint", UINT_OBJ, 0, 60000, true, "UINT 60000"},
		{"udint", UDINT_OBJ, 0, 70000, true, "UDINT 70000"},
		{"ulint", ULINT_OBJ, 0, math.MaxUint64, true, "ULINT 18446744073709551615"},
		// A signed value stored in an unsigned type.
		{"signed to udint", UDINT_OBJ, 8, 0, false, "UDINT 8"},
		{"signed to udint negative", UDINT_OBJ, -1, 0, false, "ERROR: UDINT underflow: -1"},
		{"usint overflow", USINT_OBJ, 0, 256, true, "ERROR: USINT overflow: 256"},
		// An unsigned value stored in a signed type.
		{"unsigned to int", INT_OBJ, 0, 7, true, "INT 7"},
		{"unsigned to sint overflow", SINT_OBJ, 0, 200, true, "ERROR: SINT overflow: 200"},
		{"int underflow", INT_OBJ, -40000, 0, false, "ERROR: INT underflow: -40000"},
		{"unknown type", "FOO", 1, 0, false, "ERROR: internal error: unknown integer type FOO"},
	}
	for _, tt := range tests {
		checkResult(t, tt.name, checkAndCreateIntegerObject(tt.typ, tt.val, tt.uval, tt.isUnsigned), tt.want)
	}
}

func TestFloatInfix(t *testing.T) {
	r := func(v float64) Object { return &Real{Value: v} }
	lr := func(v float64) Object { return &LReal{Value: v} }
	runInfixCases(t, []infixCase{
		{"add", r(1.5), "+", r(2), "REAL 3.500000"},
		{"sub", r(1.5), "-", r(2), "REAL -0.500000"},
		{"mul", r(1.5), "*", r(2), "REAL 3.000000"},
		{"div", r(3), "/", r(2), "REAL 1.500000"},
		{"lt", r(1), "<", r(2), "BOOLEAN true"},
		{"gt", r(1), "GT", r(2), "BOOLEAN false"},
		{"eq", r(1), "=", r(1), "BOOLEAN true"},
		{"ne", r(1), "!=", r(1), "BOOLEAN false"},
		{"le", r(1), "LE", r(1), "BOOLEAN true"},
		{"ge", r(1), ">=", r(2), "BOOLEAN false"},
		{"div zero", r(1), "/", r(0), "ERROR: division by zero"},
		{"unknown", r(1), "MOD", r(1), "ERROR: unknown operator for REAL/LREAL: MOD"},
		// LREAL wins over REAL and integers; REAL wins over integers.
		{"lreal", lr(1), "+", r(2), "LREAL 3.000000"},
		{"lreal int", &Int{Value: 2}, "*", lr(1.5), "LREAL 3.000000"},
		{"real int", &Int{Value: 2}, "*", r(1.5), "REAL 3.000000"},
	})
}

func TestBooleanInfix(t *testing.T) {
	T, F := &Boolean{Value: true}, &Boolean{Value: false}
	runInfixCases(t, []infixCase{
		{"and", T, "AND", F, "BOOLEAN false"},
		{"amp", T, "&", T, "BOOLEAN true"},
		{"or", T, "OR", F, "BOOLEAN true"},
		{"xor", T, "XOR", T, "BOOLEAN false"},
		{"nand", T, "NAND", T, "BOOLEAN false"},
		{"nor", F, "NOR", F, "BOOLEAN true"},
		{"eq", T, "=", T, "BOOLEAN true"},
		{"ne", T, "<>", F, "BOOLEAN true"},
		{"lt", F, "<", T, "BOOLEAN true"},
		{"gt", T, ">", F, "BOOLEAN true"},
		{"le", F, "<=", F, "BOOLEAN true"},
		{"le false", T, "LE", F, "BOOLEAN false"},
		{"ge", T, ">=", F, "BOOLEAN true"},
		{"ge false", F, "GE", T, "BOOLEAN false"},
		{"unknown", T, "+", T, "ERROR: unknown operator for BOOLEAN: +"},
	})
}

func TestStringInfix(t *testing.T) {
	s := func(v string) Object { return &String{Value: v} }
	w := func(v string) Object { return &WString{Value: v} }
	runInfixCases(t, []infixCase{
		{"concat", s("ab"), "+", s("cd"), "STRING abcd"},
		{"eq", s("a"), "=", s("a"), "BOOLEAN true"},
		{"ne", s("a"), "<>", s("b"), "BOOLEAN true"},
		{"lt", s("a"), "<", s("b"), "BOOLEAN true"},
		{"gt", s("a"), ">", s("b"), "BOOLEAN false"},
		{"le", s("a"), "<=", s("a"), "BOOLEAN true"},
		{"ge", s("a"), ">=", s("b"), "BOOLEAN false"},
		{"unknown", s("a"), "-", s("b"), "ERROR: unsupported operator '-' for types STRING and STRING"},
		{"wconcat", w("ab"), "+", w("cd"), "WSTRING abcd"},
		{"weq", w("a"), "EQ", w("a"), "BOOLEAN true"},
		{"wne", w("a"), "NE", w("b"), "BOOLEAN true"},
		{"wlt", w("a"), "LT", w("b"), "BOOLEAN true"},
		{"wgt", w("a"), "GT", w("b"), "BOOLEAN false"},
		{"wle", w("b"), "LE", w("a"), "BOOLEAN false"},
		{"wge", w("b"), "GE", w("a"), "BOOLEAN true"},
		{"wunknown", w("a"), "*", w("b"), "ERROR: unsupported operator '*' for types WSTRING and WSTRING"},
	})
}

func TestBitStringInfix(t *testing.T) {
	b := func(v uint64) Object { return &BitString{Value: v, Width: 8} }
	lw := func(v uint64) Object { return &BitString{Value: v, Width: 64} }
	runInfixCases(t, []infixCase{
		{"and", b(0xF0), "AND", b(0x3C), "BITSTRING BYTE#16#30"},
		{"amp", b(0xF0), "&", b(0x3C), "BITSTRING BYTE#16#30"},
		{"or", b(0xF0), "OR", b(0x0F), "BITSTRING BYTE#16#FF"},
		{"xor", b(0xFF), "XOR", b(0x0F), "BITSTRING BYTE#16#F0"},
		// NAND and NOR keep the result within the width.
		{"nand", b(0xF0), "NAND", b(0xFF), "BITSTRING BYTE#16#F"},
		{"nor", b(0xF0), "NOR", b(0x00), "BITSTRING BYTE#16#F"},
		{"nand 64", lw(0), "NAND", lw(0), "BITSTRING LWORD#16#FFFFFFFFFFFFFFFF"},
		{"nor 64", lw(math.MaxUint64), "NOR", lw(0), "BITSTRING LWORD#16#0"},
		{"eq", b(1), "=", b(1), "BOOLEAN true"},
		{"ne", b(1), "<>", b(1), "BOOLEAN false"},
		{"le", b(1), "<=", b(2), "BOOLEAN true"},
		{"ge", b(1), ">=", b(2), "BOOLEAN false"},
		{"unknown", b(1), "+", b(1), "ERROR: unknown operator for bitstrings: +"},
		{"widths", b(1), "AND", &BitString{Value: 1, Width: 16}, "ERROR: bitstring operands must have same width, got 8 and 16"},
	})
}

func TestTimeAndDateInfix(t *testing.T) {
	tm := func(d time.Duration) Object { return &Time{Value: d} }
	day := func(d int) Object { return &Date{Value: time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC)} }
	tod := func(h int) Object { return &TimeOfDay{Value: time.Date(0, 1, 1, h, 30, 0, 0, time.UTC)} }
	dt := func(h int) Object { return &DateAndTime{Value: time.Date(2026, 1, 2, h, 0, 0, 0, time.UTC)} }
	runInfixCases(t, []infixCase{
		{"time add", tm(time.Second), "+", tm(2 * time.Second), "TIME T#3s"},
		{"time sub", tm(3 * time.Second), "-", tm(time.Second), "TIME T#2s"},
		{"time lt", tm(time.Second), "<", tm(2 * time.Second), "BOOLEAN true"},
		{"time mul time", tm(time.Second), "*", tm(time.Second), "ERROR: unsupported operator '*' for types TIME and TIME"},
		{"time unknown", tm(time.Second), "AND", tm(time.Second), "ERROR: unknown operator 'AND' for generic comparison"},
		{"time mul num", tm(time.Second), "*", &Int{Value: 3}, "TIME T#3s"},
		{"time div num", tm(3 * time.Second), "/", &Real{Value: 2}, "TIME T#1.5s"},
		{"time div zero", tm(time.Second), "/", &Int{Value: 0}, "ERROR: division by zero"},
		{"num mul time", &Int{Value: 2}, "*", tm(time.Second), "TIME T#2s"},
		{"num div time", &Int{Value: 2}, "/", tm(time.Second), "ERROR: unsupported operator '/' for types INT and TIME"},
		{"time add num", tm(time.Second), "+", &Int{Value: 2}, "ERROR: unsupported operator '+' for types TIME and INT"},

		{"date sub", day(3), "-", day(1), "TIME T#48h0m0s"},
		{"date lt", day(1), "<", day(3), "BOOLEAN true"},
		{"date add int", day(1), "+", &Int{Value: 1}, "ERROR: unsupported operator '+' for types DATE and INT"},

		{"tod add", tod(1), "+", tm(time.Hour), "TIME_OF_DAY TOD#02:30:00"},
		{"tod sub time", tod(2), "-", tm(time.Hour), "TIME_OF_DAY TOD#01:30:00"},
		{"tod sub tod", tod(3), "-", tod(1), "TIME T#2h0m0s"},
		{"tod lt", tod(1), "<", tod(3), "BOOLEAN true"},
		{"tod mul", tod(1), "*", tm(time.Hour), "ERROR: unsupported operator '*' for types TIME_OF_DAY and TIME"},
		{"time add tod", tm(time.Hour), "+", tod(1), "ERROR: unsupported operator '+' for types TIME and TIME_OF_DAY"},

		{"dt add", dt(1), "+", tm(time.Hour), "DATE_AND_TIME DT#2026-01-02-02:00:00"},
		{"dt sub time", dt(2), "-", tm(time.Hour), "DATE_AND_TIME DT#2026-01-02-01:00:00"},
		{"dt sub dt", dt(3), "-", dt(1), "TIME T#2h0m0s"},
		{"dt ge", dt(3), ">=", dt(1), "BOOLEAN true"},
		{"dt mul", dt(1), "*", tm(time.Hour), "ERROR: unsupported operator '*' for types DATE_AND_TIME and TIME"},
		{"time add dt", tm(time.Hour), "+", dt(1), "ERROR: unsupported operator '+' for types TIME and DATE_AND_TIME"},
	})
}

func TestEnumNullAndMismatchInfix(t *testing.T) {
	red := &EnumeratedValue{TypeName: "Color", Value: "Red"}
	runInfixCases(t, []infixCase{
		// Enumerated values compare by type and value, ignoring case.
		{"enum eq", red, "=", &EnumeratedValue{TypeName: "COLOR", Value: "red"}, "BOOLEAN true"},
		{"enum ne", red, "<>", &EnumeratedValue{TypeName: "Color", Value: "Blue"}, "BOOLEAN true"},
		{"enum other type", red, "=", &EnumeratedValue{TypeName: "Shade", Value: "Red"}, "BOOLEAN false"},
		{"enum order", red, "<", red, "ERROR: operator '<' is not defined for enumerated values"},
		{"null eq null", &Null{}, "=", &Null{}, "BOOLEAN true"},
		{"null eq int", &Null{}, "=", &Int{Value: 1}, "BOOLEAN false"},
		{"null ne", &Null{}, "<>", &Int{Value: 1}, "ERROR: type mismatch for comparison: NULL <> INT"},
		{"bitwise mismatch", &BitString{Value: 1, Width: 8}, "AND", &Boolean{Value: true}, "ERROR: type mismatch: BITSTRING AND BOOLEAN"},
		{"comparison mismatch", &Int{Value: 1}, "<", &String{Value: "a"}, "ERROR: type mismatch for comparison: INT < STRING"},
		{"unsupported", &String{Value: "a"}, "-", &Int{Value: 1}, "ERROR: unsupported operator '-' for types STRING and INT"},
	})
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		obj  Object
		want bool
	}{
		{&Boolean{Value: true}, true},
		{&Boolean{Value: false}, false},
		{&Null{}, false},
		{&Int{Value: 1}, false}, // Only a BOOL is truthy.
	}
	for _, tt := range tests {
		if got := IsTruthy(tt.obj); got != tt.want {
			t.Errorf("IsTruthy(%s) = %v, want %v", describe(tt.obj), got, tt.want)
		}
	}
}

func TestOperatorClassification(t *testing.T) {
	if !IsComparisonOperator("GE") || IsComparisonOperator("+") {
		t.Error("IsComparisonOperator misclassifies GE or +")
	}
	if !IsBitwiseOperator("NOR") || IsBitwiseOperator("<") {
		t.Error("IsBitwiseOperator misclassifies NOR or <")
	}
	for _, name := range []string{"t", "DATE", "tod", "Dt"} {
		if !IsTimeDateKeyword(name) {
			t.Errorf("IsTimeDateKeyword(%q) = false", name)
		}
	}
	if IsTimeDateKeyword("INT") {
		t.Error("IsTimeDateKeyword(INT) = true")
	}
}

func TestGetFloat64ValueAllTypes(t *testing.T) {
	objs := []Object{&SInt{Value: 1}, &Int{Value: 1}, &DInt{Value: 1}, &LInt{Value: 1},
		&USInt{Value: 1}, &UInt{Value: 1}, &UDInt{Value: 1}, &ULInt{Value: 1}, &Real{Value: 1}, &LReal{Value: 1}}
	for _, o := range objs {
		if v, ok := GetFloat64Value(o); !ok || v != 1 {
			t.Errorf("GetFloat64Value(%s) = %v, %v", describe(o), v, ok)
		}
	}
	if _, ok := GetFloat64Value(&String{Value: "1"}); ok {
		t.Error("GetFloat64Value(STRING) succeeded")
	}
}

func TestConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    Object
		from, to string
		want     string
	}{
		{"mismatch", &String{Value: "1"}, "INT", "LINT", "ERROR: type mismatch for INT_TO_LINT: input is STRING"},
		// Integers are range-checked against the target type.
		{"int to sint", &Int{Value: 5}, "INT", "SINT", "SINT 5"},
		{"int to sint range", &Int{Value: 300}, "INT", "SINT", "ERROR: value 300 is out of range for type SINT (-128 to 127)"},
		{"int to usint negative", &Int{Value: -1}, "INT", "USINT", "ERROR: value -1 is out of range for type USINT (0 to 255)"},
		{"any_int", &DInt{Value: 7}, "ANY_INT", "UINT", "UINT 7"},
		// Reals round to the nearest integer.
		{"real to int", &Real{Value: 2.5}, "REAL", "INT", "INT 3"},
		{"lreal to dint", &LReal{Value: -2.4}, "LREAL", "DINT", "DINT -2"},
		{"string to int", &String{Value: " 42 "}, "STRING", "INT", "LINT 42"},
		{"string to int bad", &String{Value: "x"}, "STRING", "INT", "ERROR: could not parse string to integer: x"},
		{"bool to int", &Boolean{Value: true}, "BOOL", "INT", "ERROR: conversion from BOOLEAN to INT is not supported"},
		{"int to real", &Int{Value: 2}, "INT", "REAL", "REAL 2.000000"},
		{"any_real", &LInt{Value: 2}, "ANY_REAL", "LREAL", "REAL 2.000000"},
		{"string to real", &String{Value: "1.5"}, "STRING", "REAL", "REAL 1.500000"},
		{"string to real bad", &String{Value: "x"}, "STRING", "REAL", "ERROR: could not parse string to real: x"},
		{"bool to real", &Boolean{Value: true}, "BOOL", "REAL", "ERROR: conversion from BOOLEAN to REAL is not supported"},
		{"true to string", &Boolean{Value: true}, "BOOL", "STRING", "STRING TRUE"},
		{"false to string", &Boolean{Value: false}, "BOOL", "STRING", "STRING FALSE"},
		{"time to string", &Time{Value: time.Second}, "TIME", "STRING", "STRING 1s"},
		{"real to string", &Real{Value: 1.5}, "REAL", "STRING", "STRING 1.500000"},
		{"int to string", &Int{Value: 7}, "INT", "STRING", "STRING 7"},
		{"bool to bool", &Boolean{Value: true}, "BOOL", "BOOL", "BOOLEAN true"},
		{"int to bool", &Int{Value: 2}, "INT", "BOOL", "BOOLEAN true"},
		{"real to bool", &Real{Value: 0}, "REAL", "BOOL", "BOOLEAN false"},
		{"string to bool", &String{Value: "TRUE"}, "STRING", "BOOL", "ERROR: conversion from STRING to BOOL is not supported"},
		{"int to byte", &Int{Value: 255}, "INT", "BYTE", "BITSTRING BYTE#16#FF"},
		{"int to byte range", &Int{Value: 256}, "INT", "BYTE", "ERROR: value 256 is out of range for type BYTE (0 to 255)"},
		{"string to byte", &String{Value: "1"}, "STRING", "BYTE", "ERROR: conversion from STRING to BYTE is not supported"},
		// Bit strings convert to integers, other bit strings and BOOL.
		{"byte to int", &BitString{Value: 5, Width: 8}, "BYTE", "INT", "INT 5"},
		{"word to sint range", &BitString{Value: 300, Width: 16}, "WORD", "SINT", "ERROR: SINT overflow: 300"},
		{"byte to word", &BitString{Value: 5, Width: 8}, "BYTE", "WORD", "BITSTRING WORD#16#5"},
		{"word to byte range", &BitString{Value: 300, Width: 16}, "WORD", "BYTE", "ERROR: value 300 is out of range for type BYTE (0 to 255)"},
		{"byte to bool", &BitString{Value: 1, Width: 8}, "BYTE", "BOOL", "BOOLEAN true"},
		{"word from byte type", &BitString{Value: 1, Width: 16}, "BYTE", "INT", "ERROR: type mismatch for BYTE_TO_INT"},
		{"int to bcd", &Int{Value: 1234}, "INT", "BCD", "BITSTRING WORD#16#1234"},
		{"int to bcd range", &Int{Value: 10000}, "INT", "BCD", "ERROR: value 10000 out of range for 4-digit BCD conversion"},
		{"real to bcd", &Real{Value: 1}, "REAL", "BCD", "ERROR: conversion from REAL to BCD is not supported"},
		{"unknown target", &Int{Value: 1}, "INT", "FOO", "ERROR: conversion to type FOO is not supported"},
	}
	for _, tt := range tests {
		checkResult(t, tt.name, ApplyConversion(tt.input, tt.from, tt.to), tt.want)
	}
}

func TestGenericConversionBuiltin(t *testing.T) {
	conv := GenericConversionBuiltin("INT", "REAL")
	checkResult(t, "one argument", conv.Fn(&Int{Value: 2}), "REAL 2.000000")
	checkResult(t, "no arguments", conv.Fn(), "ERROR: wrong number of arguments for INT_TO_REAL. got=0, want=1")
}
