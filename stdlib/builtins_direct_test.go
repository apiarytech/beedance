package stdlib

import (
	"math"
	"strings"
	"testing"
	"time"

	"beedance/object"
)

// These tests call each built-in directly with the argument objects,
// covering argument checks that ST source code cannot easily reach.

type builtinCase struct {
	name string
	args []object.Object
	want string // "TYPE inspect", or "ERROR: <part of the message>"
}

// describeObject returns an object's type and printed form, or
// "ERROR: <message>" for an error.
func describeObject(o object.Object) string {
	if err, ok := o.(*object.Error); ok {
		return "ERROR: " + err.Message
	}
	return string(o.Type()) + " " + o.Inspect()
}

func runBuiltinCases(t *testing.T, cases []builtinCase) {
	t.Helper()
	for _, c := range cases {
		b, ok := object.GetBuiltinByName(c.name)
		if !ok {
			t.Fatalf("built-in %s is not registered", c.name)
		}
		got := describeObject(b.Fn(c.args...))
		if rest, isErr := strings.CutPrefix(c.want, "ERROR:"); isErr {
			if !strings.HasPrefix(got, "ERROR: ") || !strings.Contains(got, strings.TrimSpace(rest)) {
				t.Errorf("%s%s: expected an error containing %q, got %s", c.name, describeArgs(c.args), strings.TrimSpace(rest), got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("%s%s: expected %s, got %s", c.name, describeArgs(c.args), c.want, got)
		}
	}
}

func describeArgs(args []object.Object) string {
	parts := []string{}
	for _, a := range args {
		parts = append(parts, a.Inspect())
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func args(a ...object.Object) []object.Object { return a }

var (
	i1    = &object.Int{Value: 1}
	i2    = &object.Int{Value: 2}
	str   = &object.String{Value: "abc"}
	wstr  = &object.WString{Value: "abc"}
	yes   = &object.Boolean{Value: true}
	sec   = &object.Time{Value: time.Second}
	byte1 = &object.BitString{Value: 0x81, Width: 8}
)

func real(v float64) *object.Real   { return &object.Real{Value: v} }
func lreal(v float64) *object.LReal { return &object.LReal{Value: v} }
func arr(elems ...object.Object) *object.Array {
	return &object.Array{Elements: elems}
}

func TestMathBuiltinArguments(t *testing.T) {
	cases := []builtinCase{}
	// Every one-argument math function checks its argument count and type.
	for _, name := range []string{"SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP", "SQRT", "ROUND", "ABS", "TRUNC"} {
		cases = append(cases,
			builtinCase{name, args(), "ERROR: wrong number of arguments for " + name},
			builtinCase{name, args(str), "ERROR: `" + name + "`"},
		)
	}
	for _, name := range []string{"ATAN2", "ADD", "SUB", "MUL", "DIV", "MOD", "EXPT"} {
		cases = append(cases, builtinCase{name, args(i1), "ERROR: wrong number of arguments for " + name})
	}
	runBuiltinCases(t, cases)
}

func TestMathBuiltinResults(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"SIN", args(real(0)), "REAL 0.000000"},
		{"COS", args(i1), "REAL 0.540302"},
		{"TAN", args(real(0)), "REAL 0.000000"},
		{"ASIN", args(real(1)), "REAL 1.570796"},
		{"ASIN", args(real(2)), "ERROR: must be between -1 and 1"},
		{"ACOS", args(real(1)), "REAL 0.000000"},
		{"ACOS", args(real(-2)), "ERROR: must be between -1 and 1"},
		{"ATAN", args(real(0)), "REAL 0.000000"},
		{"ATAN2", args(i1, i1), "REAL 0.785398"},
		{"ATAN2", args(str, i1), "ERROR: argument 1 to `ATAN2`"},
		{"ATAN2", args(i1, str), "ERROR: argument 2 to `ATAN2`"},
		{"LN", args(real(math.E)), "REAL 1.000000"},
		{"LN", args(real(0)), "ERROR: argument to `LN` must be positive"},
		{"LOG", args(real(100)), "REAL 2.000000"},
		{"LOG", args(real(-1)), "ERROR: argument to `LOG` must be positive"},
		{"EXP", args(real(0)), "REAL 1.000000"},
		{"SQRT", args(real(9)), "REAL 3.000000"},
		{"SQRT", args(real(-1)), "ERROR: must be non-negative"},
		{"ROUND", args(real(2.5)), "LINT 3"},
		{"ROUND", args(lreal(-2.5)), "LINT -3"},
		{"TRUNC", args(real(2.7)), "LINT 2"},
		{"TRUNC", args(lreal(-2.7)), "LINT -2"},
		{"TRUNC", args(i1), "ERROR: argument to `TRUNC` must be REAL"},
	})
}

func TestAbsBuiltin(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"ABS", args(&object.SInt{Value: -3}), "SINT 3"},
		{"ABS", args(&object.SInt{Value: 3}), "SINT 3"},
		{"ABS", args(&object.Int{Value: -3}), "INT 3"},
		{"ABS", args(&object.Int{Value: 3}), "INT 3"},
		{"ABS", args(&object.DInt{Value: -3}), "DINT 3"},
		{"ABS", args(&object.DInt{Value: 3}), "DINT 3"},
		{"ABS", args(&object.LInt{Value: -3}), "LINT 3"},
		{"ABS", args(&object.LInt{Value: 3}), "LINT 3"},
		{"ABS", args(&object.UDInt{Value: 3}), "UDINT 3"},
		{"ABS", args(real(-1.5)), "REAL 1.500000"},
		{"ABS", args(lreal(-1.5)), "LREAL 1.500000"},
	})
}

func TestArithmeticBuiltins(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"ADD", args(i1, i2), "INT 3"},
		{"ADD", args(i1, yes), "ERROR: unsupported argument types for ADD: INT + BOOLEAN"},
		{"SUB", args(i2, i1), "INT 1"},
		{"SUB", args(i1, yes), "ERROR: unsupported argument types for SUB: INT - BOOLEAN"},
		{"MUL", args(i2, i2), "INT 4"},
		{"MUL", args(i1, yes), "ERROR: unsupported argument types for MUL: INT * BOOLEAN"},
		{"DIV", args(i2, i1), "INT 2"},
		{"DIV", args(i1, &object.Int{Value: 0}), "ERROR: division by zero"},
		{"DIV", args(i1, yes), "ERROR: unsupported argument types for DIV: INT / BOOLEAN"},
		{"MOD", args(&object.Int{Value: 7}, i2), "INT 1"},
		{"MOD", args(real(7), i2), "ERROR: arguments to `MOD` must be INTEGER, got REAL and INT"},
		{"EXPT", args(i2, &object.Int{Value: 3}), "REAL 8.000000"},
		{"EXPT", args(str, i2), "ERROR: argument 1 to `EXPT` must be numeric"},
		{"EXPT", args(i2, str), "ERROR: argument 2 to `EXPT` must be numeric"},
	})
}

func TestMinMaxBuiltins(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"MIN", args(), "ERROR: wrong number of arguments for MIN. got=0"},
		{"MIN", args(yes, yes), "ERROR: arguments to `MIN` must be of an orderable elementary type, got BOOLEAN"},
		{"MAX", args(i1, str), "ERROR: all arguments to `MAX` must be INTEGER or REAL, got STRING"},
		{"MAX", args(str, sec), "ERROR: all arguments to `MAX` must be of the same type, got STRING and TIME"},
		// Mixed numeric types compare by value.
		{"MIN", args(i2, real(1.5), &object.LInt{Value: 3}), "REAL 1.500000"},
		{"MAX", args(i2, real(1.5), &object.LInt{Value: 3}), "LINT 3"},
		{"MIN", args(&object.String{Value: "b"}, &object.String{Value: "a"}), "STRING a"},
		{"MAX", args(&object.String{Value: "b"}, &object.String{Value: "c"}), "STRING c"},
		{"MIN", args(&object.Time{Value: 2 * time.Second}, sec), "TIME T#1s"},
		{"MAX", args(sec, &object.Time{Value: 2 * time.Second}), "TIME T#2s"},
		{"MAX", args(&object.WString{Value: "a"}, &object.WString{Value: "b"}), "WSTRING b"},
	})
}

func TestBitShiftBuiltins(t *testing.T) {
	cases := []builtinCase{}
	for _, name := range []string{"SHL", "SHR", "ROL", "ROR"} {
		cases = append(cases,
			builtinCase{name, args(byte1), "ERROR: wrong number of arguments for " + name},
			builtinCase{name, args(i1, i1), "ERROR: argument 1 to `" + name + "` must be a bitstring type"},
			builtinCase{name, args(byte1, str), "ERROR: argument 2 to `" + name + "` must be INTEGER"},
			builtinCase{name, args(byte1, &object.Int{Value: -1}), "ERROR: amount for `" + name + "` must be non-negative"},
		)
	}
	w := func(v uint64, width int) object.Object { return &object.BitString{Value: v, Width: width} }
	cases = append(cases,
		// Shifts drop bits beyond the width.
		builtinCase{"SHL", args(byte1, i1), "BITSTRING BYTE#16#2"},
		builtinCase{"SHL", args(w(1, 64), &object.Int{Value: 63}), "BITSTRING LWORD#16#8000000000000000"},
		builtinCase{"SHR", args(byte1, i1), "BITSTRING BYTE#16#40"},
		builtinCase{"SHR", args(w(2, 64), i1), "BITSTRING LWORD#16#1"},
		// Rotations wrap bits around at every width.
		builtinCase{"ROL", args(byte1, i1), "BITSTRING BYTE#16#3"},
		builtinCase{"ROL", args(w(0x8001, 16), i1), "BITSTRING WORD#16#3"},
		builtinCase{"ROL", args(w(0x80000001, 32), i1), "BITSTRING DWORD#16#3"},
		builtinCase{"ROL", args(w(1<<63|1, 64), i1), "BITSTRING LWORD#16#3"},
		builtinCase{"ROR", args(byte1, i1), "BITSTRING BYTE#16#C0"},
		builtinCase{"ROR", args(w(0x8001, 16), i1), "BITSTRING WORD#16#C000"},
		builtinCase{"ROR", args(w(0x80000001, 32), i1), "BITSTRING DWORD#16#C0000000"},
		builtinCase{"ROR", args(w(1<<63|1, 64), i1), "BITSTRING LWORD#16#C000000000000000"},
	)
	runBuiltinCases(t, cases)
}

func TestBitwiseBuiltins(t *testing.T) {
	b := func(v uint64) object.Object { return &object.BitString{Value: v, Width: 8} }
	lw := func(v uint64) object.Object { return &object.BitString{Value: v, Width: 64} }
	runBuiltinCases(t, []builtinCase{
		{"AND", args(b(1)), "ERROR: wrong number of arguments for AND. got=1, want>=2"},
		{"OR", args(i1, b(1)), "ERROR: all arguments to `OR` must be bit-string types, got INT"},
		{"XOR", args(b(1), i1), "ERROR: all arguments to `XOR` must be bit-string types, got INT"},
		{"AND", args(b(1), lw(1)), "ERROR: all arguments to `AND` must have the same width, got 8 and 64"},
		// Any number of inputs.
		{"AND", args(b(0xF0), b(0x3C), b(0x30)), "BITSTRING BYTE#16#30"},
		{"OR", args(b(0x01), b(0x02), b(0x04)), "BITSTRING BYTE#16#7"},
		{"XOR", args(b(0xFF), b(0x0F)), "BITSTRING BYTE#16#F0"},
		{"NAND", args(b(0xF0), b(0xFF)), "BITSTRING BYTE#16#F"},
		{"NOR", args(b(0xF0), b(0x00)), "BITSTRING BYTE#16#F"},
		{"NAND", args(lw(0), lw(0)), "BITSTRING LWORD#16#FFFFFFFFFFFFFFFF"},
	})
	// The operator is checked inside the shared implementation too.
	if got := describeObject(bitwiseBuiltin("SHL", b(1), b(1))); !strings.Contains(got, "unknown bitwise operator SHL") {
		t.Errorf("bitwiseBuiltin(SHL) = %s", got)
	}
}

func TestComparisonBuiltins(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"GT", args(i2, i1), "BOOLEAN true"},
		{"GE", args(i1, i2), "BOOLEAN false"},
		{"EQ", args(i1, i1), "BOOLEAN true"},
		{"LE", args(i1, i1), "BOOLEAN true"},
		{"LT", args(i2, i1), "BOOLEAN false"},
		{"NE", args(i1, i2), "BOOLEAN true"},
		{"GREAT", args(i2, i1), "BOOLEAN true"},
		{"EQ", args(i1), "ERROR: wrong number of arguments for EQ. got=1, want=2"},
		// Incompatible operands are a type mismatch.
		{"GT", args(sec, i1), "ERROR: type mismatch for comparison: TIME > INT"},
		{"LT", args(i1, str), "ERROR: type mismatch for comparison: INT < STRING"},
	})
}

func TestSelectionBuiltinArguments(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"SEL", args(yes, i1), "ERROR: wrong number of arguments for SEL. got=2, want=3"},
		{"SEL", args(yes, i1, i2), "INT 2"},
		{"MOVE", args(), "ERROR: wrong number of arguments for MOVE. got=0, want=1"},
		{"MOVE", args(str), "STRING abc"},
		{"MUX", args(i1), "ERROR: wrong number of arguments for MUX. got=1, want>=2"},
		{"MUX", args(i1, str, str), "STRING abc"},
		{"LIMIT", args(i1, i2), "ERROR: wrong number of arguments for LIMIT. got=2, want=3"},
		{"LIMIT", args(i1, &object.Int{Value: 5}, &object.Int{Value: 3}), "LINT 3"},
		{"LIMIT", args(real(0), real(5), real(3)), "REAL 3.000000"},
		{"MUX", args(i1, str, i1), "ERROR: all value arguments to `MUX` must be of the same type, got INT but expected STRING"},
	})
}

func TestPutsBuiltin(t *testing.T) {
	runBuiltinCases(t, []builtinCase{{"PUTS", args(str, i1), "NULL null"}})
}

func TestArrayBuiltins(t *testing.T) {
	a := arr(i1, i2, str)
	runBuiltinCases(t, []builtinCase{
		{"LEN", args(), "ERROR: wrong number of arguments for LEN"},
		{"LEN", args(str), "LINT 3"},
		{"PUSH", args(a), "ERROR: wrong number of arguments for PUSH"},
		{"PUSH", args(i1, i1), "ERROR: argument to `PUSH` must be ARRAY, got INT"},
		{"POP", args(), "ERROR: wrong number of arguments for POP"},
		{"POP", args(i1), "ERROR: argument to `POP` must be ARRAY, got INT"},
		{"POP", args(arr()), "NULL null"},
		{"POP", args(a), "ARRAY [1, 2]"},
		{"LOWER_BOUND", args(a), "ERROR: wrong number of arguments for LOWER_BOUND"},
		{"LOWER_BOUND", args(i1, i1), "ERROR: argument 1 to `LOWER_BOUND` must be of type ARRAY"},
		{"LOWER_BOUND", args(a, str), "ERROR: argument 2 to `LOWER_BOUND` must be of type INT"},
		{"LOWER_BOUND", args(a, i2), "ERROR: invalid dimension 2 for 1D array"},
		{"UPPER_BOUND", args(a), "ERROR: wrong number of arguments for UPPER_BOUND"},
		{"UPPER_BOUND", args(i1, i1), "ERROR: argument 1 to `UPPER_BOUND` must be of type ARRAY"},
		{"UPPER_BOUND", args(a, str), "ERROR: argument 2 to `UPPER_BOUND` must be of type INT"},
		{"UPPER_BOUND", args(a, i2), "ERROR: invalid dimension 2 for 1D array"},
		{"FIRST", args(), "ERROR: wrong number of arguments"},
		{"FIRST", args(i1), "ERROR: argument to `FIRST` must be ARRAY"},
		{"LAST", args(), "ERROR: wrong number of arguments"},
		{"LAST", args(i1), "ERROR: argument to `LAST` must be ARRAY"},
		{"REST", args(), "ERROR: wrong number of arguments"},
		{"REST", args(i1), "ERROR: argument to `REST` must be ARRAY"},
		{"REST", args(arr()), "NULL null"},
		{"FIND", args(a, str), "LINT 3"},
		{"FIND", args(a, &object.Int{Value: 9}), "LINT 0"},
	})
}

func TestStringBuiltinArguments(t *testing.T) {
	runBuiltinCases(t, []builtinCase{
		{"INSERT", args(str, str), "ERROR: wrong number of arguments for INSERT"},
		{"INSERT", args(str, str, str), "ERROR: argument 3 to `INSERT` must be INTEGER"},
		{"INSERT", args(str, i1, i1), "ERROR: argument 2 to `INSERT` for strings must be STRING"},
		{"INSERT", args(wstr, str, i1), "ERROR: argument 2 to `INSERT` for wstrings must be WSTRING"},
		{"INSERT", args(i1, i1, i1), "ERROR: argument 1 to `INSERT` must be ARRAY, STRING, or WSTRING"},
		{"DELETE", args(str, i1), "ERROR: wrong number of arguments for DELETE"},
		{"DELETE", args(str, str, i1), "ERROR: argument 2 to `DELETE` must be INTEGER"},
		{"DELETE", args(str, i1, str), "ERROR: argument 3 to `DELETE` must be INTEGER"},
		{"DELETE", args(i1, i1, i1), "ERROR: argument 1 to `DELETE` must be ARRAY, STRING, or WSTRING"},
		{"CONCAT", args(str), "ERROR: wrong number of arguments for CONCAT"},
		{"CONCAT", args(arr(), str), "ERROR: all arguments to `CONCAT` must be of the same type (ARRAY), got STRING"},
		{"CONCAT", args(str, wstr), "ERROR: all arguments to `CONCAT` must be of the same type (STRING), got WSTRING"},
		{"CONCAT", args(wstr, str), "ERROR: all arguments to `CONCAT` must be of the same type (WSTRING), got STRING"},
		{"CONCAT", args(i1, i1), "ERROR: arguments to `CONCAT` must be ARRAY, STRING, or WSTRING, got INT"},
		{"LEFT", args(str), "ERROR: wrong number of arguments for LEFT"},
		{"LEFT", args(str, str), "ERROR: argument 2 to `LEFT` must be INTEGER"},
		{"LEFT", args(i1, i1), "ERROR: argument 1 to `LEFT` must be STRING or WSTRING"},
		{"RIGHT", args(str), "ERROR: wrong number of arguments for RIGHT"},
		{"RIGHT", args(str, str), "ERROR: argument 2 to `RIGHT` must be INTEGER"},
		{"RIGHT", args(i1, i1), "ERROR: argument 1 to `RIGHT` must be STRING or WSTRING"},
		{"MID", args(str, i1), "ERROR: wrong number of arguments for MID"},
		{"MID", args(str, i1, str), "ERROR: argument 3 to `MID` must be INTEGER"},
		{"MID", args(str, str, i1), "ERROR: argument 2 to `MID` must be INTEGER"},
		{"MID", args(i1, i1, i1), "ERROR: argument 1 to `MID` must be STRING or WSTRING"},
		{"FIND", args(str), "ERROR: wrong number of arguments for FIND"},
		{"FIND", args(str, i1), "ERROR: argument 2 to `FIND` for strings must be STRING"},
		{"FIND", args(wstr, str), "ERROR: argument 2 to `FIND` for wstrings must be WSTRING"},
		{"FIND", args(i1, i1), "ERROR: argument 1 to `FIND` must be STRING, WSTRING, or ARRAY"},
		{"REPLACE", args(str, str, i1), "ERROR: wrong number of arguments for REPLACE"},
		{"REPLACE", args(str, str, i1, str), "ERROR: argument 4 to `REPLACE` must be INTEGER"},
		{"REPLACE", args(str, str, str, i1), "ERROR: argument 3 to `REPLACE` must be INTEGER"},
		{"REPLACE", args(str, wstr, i1, i1), "ERROR: argument 2 to `REPLACE` must be STRING"},
		{"REPLACE", args(wstr, str, i1, i1), "ERROR: argument 2 to `REPLACE` must be WSTRING"},
		{"REPLACE", args(i1, str, i1, i1), "ERROR: argument 1 to `REPLACE` must be STRING or WSTRING"},
	})
}

func TestStringBuiltinEdges(t *testing.T) {
	n := func(v int16) object.Object { return &object.Int{Value: v} }
	s := func(v string) object.Object { return &object.String{Value: v} }
	w := func(v string) object.Object { return &object.WString{Value: v} }
	runBuiltinCases(t, []builtinCase{
		// Positions are clamped to the string.
		{"INSERT", args(s("abc"), s("X"), n(0)), "STRING Xabc"},
		{"INSERT", args(s("abc"), s("X"), n(9)), "STRING abcX"},
		{"INSERT", args(w("abc"), w("X"), n(0)), "WSTRING Xabc"},
		{"INSERT", args(w("abc"), w("X"), n(9)), "WSTRING abcX"},
		{"INSERT", args(w("abc"), w("X"), n(2)), "WSTRING aXbc"},
		// DELETE leaves the input as is when nothing is deleted, and stops at the end.
		{"DELETE", args(s("abc"), n(0), n(1)), "STRING abc"},
		{"DELETE", args(s("abc"), n(2), n(9)), "STRING a"},
		{"DELETE", args(w("abc"), n(2), n(1)), "WSTRING ac"},
		{"DELETE", args(w("abc"), n(4), n(1)), "WSTRING abc"},
		{"DELETE", args(w("abc"), n(2), n(9)), "WSTRING a"},
		{"DELETE", args(arr(i1, i2), n(1), n(0)), "ARRAY [1, 2]"},
		{"DELETE", args(arr(i1, i2, str), n(2), n(9)), "ARRAY [1]"},
		{"CONCAT", args(w("ab"), w("cd"), w("e")), "WSTRING abcde"},
		{"LEFT", args(s("abc"), n(0)), "STRING "},
		{"LEFT", args(s("abc"), n(9)), "STRING abc"},
		{"LEFT", args(w("abc"), n(0)), "WSTRING "},
		{"LEFT", args(w("abc"), n(9)), "WSTRING abc"},
		{"LEFT", args(w("abc"), n(2)), "WSTRING ab"},
		{"RIGHT", args(s("abc"), n(-1)), "STRING "},
		{"RIGHT", args(s("abc"), n(9)), "STRING abc"},
		{"RIGHT", args(w("abc"), n(0)), "WSTRING "},
		{"RIGHT", args(w("abc"), n(9)), "WSTRING abc"},
		{"RIGHT", args(w("abc"), n(2)), "WSTRING bc"},
		{"MID", args(s("abcde"), n(0), n(2)), "STRING "},
		{"MID", args(s("abcde"), n(4), n(9)), "STRING de"},
		{"MID", args(w("abcde"), n(2), n(2)), "WSTRING bc"},
		{"MID", args(w("abcde"), n(6), n(2)), "WSTRING "},
		{"MID", args(w("abcde"), n(4), n(9)), "WSTRING de"},
		{"FIND", args(w("abcde"), w("cd")), "LINT 3"},
		// REPLACE clamps its length and position; a position past the end appends.
		{"REPLACE", args(s("abcde"), s("X"), n(0), n(-1)), "STRING Xabcde"},
		{"REPLACE", args(s("abc"), s("X"), n(9), n(1)), "STRING abcX"},
		{"REPLACE", args(s("abc"), s("X"), n(2), n(9)), "STRING aX"},
		{"REPLACE", args(w("abcde"), w("X"), n(2), n(2)), "WSTRING aXde"},
		{"REPLACE", args(w("abc"), w("X"), n(9), n(1)), "WSTRING abcX"},
		{"REPLACE", args(w("abc"), w("X"), n(2), n(9)), "WSTRING aX"},
	})
}

func TestDeleteLeavesInputArrayUnchanged(t *testing.T) {
	input := arr(i1, i2, str)
	b, _ := object.GetBuiltinByName("DELETE")
	got := b.Fn(input, i1, i1)
	if got.Inspect() != "[2, abc]" {
		t.Errorf("DELETE result = %s, want [2, abc]", got.Inspect())
	}
	if input.Inspect() != "[1, 2, abc]" {
		t.Errorf("DELETE changed its input to %s", input.Inspect())
	}
}
