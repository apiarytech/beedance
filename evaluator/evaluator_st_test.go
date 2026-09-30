package evaluator

import (
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"strings"
	"testing"
	"time"
)

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5;", 5},
		{"10;", 10},
		{"-5;", -5},
		{"-10;", -10},
		{"5 + 5 + 5 + 5 - 10;", 10},
		{"2 * 2 * 2 * 2 * 2;", 32},
		{"-50 + 100 + -50;", 0},
		{"5 * 2 + 10;", 20},
		{"5 + 2 * 10;", 25},
		{"20 + 2 * -10;", 0},
		{"50 / 2 * 2 + 10;", 60},
		{"2 * (5 + 10);", 30},
		{"3 * 3 * 3 + 10;", 37},
		{"3 * (3 * 3) + 10;", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10;", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.input, tt.expected)
	}
}

func TestEvalUnsignedIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"USINT#255;", 255},
		{"UINT#65535;", 65535},
		{"UDINT#4294967295;", 4294967295},
		{"ULINT#18446744073709551615;", 18446744073709551615},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		ul, ok := evaluated.(*object.ULInt)
		if !ok {
			// It might be evaluated as a different integer type that gets promoted.
			// Let's use the helper to check the value.
			val, _, ok := object.GetIntegerObjectValue(evaluated)
			if !ok {
				t.Fatalf("object is not an integer type. got=%T (%+v)", evaluated, evaluated)
			}
			if uint64(val) != tt.expected {
				t.Errorf("object has wrong value. got=%d, want=%d", val, tt.expected)
			}
			continue
		}
		if ul.Value != tt.expected {
			t.Errorf("object has wrong value. got=%d, want=%d", ul.Value, tt.expected)
		}
	}
}

func TestEvalBooleanExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true;", true},
		{"false;", false},
		{"1 < 2;", true},
		{"1 > 2;", false},
		{"1 < 1;", false},
		{"1 > 1;", false},
		{"1 = 1;", true},
		{"1 != 1;", false},
		{"1 = 2;", false},
		{"1 != 2;", true},
		{"true = true;", true},
		{"false = false;", true},
		{"true = false;", false},
		{"true != false;", true},
		{"false != true;", true},
		{"(1 < 2) = true;", true},
		{"(1 < 2) = false;", false},
		{"(1 > 2) = true;", false},
		{"(1 > 2) = false;", true},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testBooleanObject(t, evaluated, tt.input, tt.expected) // Assuming testBooleanObject is in helper
	}
}

func TestEvalBooleanLogicalExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// AND operator
		{"TRUE AND TRUE;", true},
		{"TRUE AND FALSE;", false},
		{"FALSE AND TRUE;", false},
		{"FALSE AND FALSE;", false},
		{"TRUE & TRUE;", true}, // Test '&' alias
		{"(1 < 2) AND (3 > 1);", true},
		{"(1 > 2) AND (3 > 1);", false},

		// OR operator
		{"TRUE OR TRUE;", true},
		{"TRUE OR FALSE;", true},
		{"FALSE OR TRUE;", true},
		{"FALSE OR FALSE;", false},
		{"(1 > 2) OR (3 > 1);", true},
		{"(1 > 2) OR (3 < 1);", false},

		// XOR operator
		{"TRUE XOR TRUE;", false},
		{"TRUE XOR FALSE;", true},
		{"FALSE XOR TRUE;", true},
		{"FALSE XOR FALSE;", false},
		{"(1 < 2) XOR (3 > 1);", false}, // true XOR true -> false
		{"(1 > 2) XOR (3 > 1);", true},  // false XOR true -> true
	}

	for _, tt := range tests {
		testBooleanObject(t, testEval(t, tt.input), tt.input, tt.expected) // Assuming testBooleanObject is in helper
	}
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"NOT TRUE;", false},
		{"NOT FALSE;", true},
		{"NOT NOT TRUE;", true},
		{"NOT NOT FALSE;", false},
		{"!TRUE;", false},
		// Error cases for non-boolean/non-bitstring types
		{"NOT 5;", "unknown operator: NOT LINT"},
		{"NOT 3.14;", "unknown operator: NOT LREAL"},
		{`NOT 'hello';`, "unknown operator: NOT STRING"},
		// Nested NOT on invalid type should also error
		{"NOT NOT 5;", "unknown operator: NOT LINT"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case bool:
			testBooleanObject(t, evaluated, "bool", expected) // Assuming testBooleanObject is in helper
		case string:
			testErrorObjectContains(t, evaluated, expected)
		case uint64:
			bs, ok := evaluated.(*object.BitString)
			if !ok {
				t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
			}
			testBitStringObject(t, bs, expected)
		}
	}
}

func TestTypedBitStringLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// BYTE (8-bit)
		{"BYTE#16#A5;", uint64(0xA5)},
		{"BYTE#8#245;", uint64(0xA5)}, // 245 octal = 165 decimal = A5 hex
		{"BYTE#2#1010_0101;", uint64(0xA5)},
		{"BYTE#10#165;", uint64(0xA5)},

		// WORD (16-bit)
		{"WORD#16#1234;", uint64(0x1234)},
		{"WORD#10#32767;", uint64(32767)},

		// DWORD (32-bit)
		{"DWORD#16#ABCDEF12;", uint64(0xABCDEF12)},

		// LWORD (64-bit)
		{"LWORD#16#1234567890ABCDEF;", uint64(0x1234567890ABCDEF)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			if expectedUint, ok := tt.expected.(uint64); ok {
				testBitStringObject(t, evaluated, expectedUint) // Assuming testBitStringObject is in helper
			} else if expectedErr, ok := tt.expected.(string); ok {
				testErrorObjectContains(t, evaluated, expectedErr)
			}
		})
	}
}

func TestTypedBitStringLiteralsWithErrors(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// BYTE (8-bit) overflows (max 255)
		{"BYTE#10#256;", "value 256 is out of range for type BYTE"},
		{"BYTE#16#100;", "value 100 is out of range for type BYTE"}, // 0x100 = 256
		{"BYTE#8#400;", "value 400 is out of range for type BYTE"},  // 400 octal = 256 decimal
		{"BYTE#2#100000000;", "value 100000000 is out of range for type BYTE"},

		// WORD (16-bit) overflows (max 65535)
		{"WORD#10#65536;", "value 65536 is out of range for type WORD"},
		{"WORD#16#10000;", "value 10000 is out of range for type WORD"},
		{"WORD#8#200000;", "value 200000 is out of range for type WORD"},
		{"WORD#2#10000000000000000;", "value 10000000000000000 is out of range for type WORD"},

		// DWORD (32-bit) overflows (max 4294967295)
		{"DWORD#10#4294967296;", "value 4294967296 is out of range for type DWORD"},
		{"DWORD#16#100000000;", "value 100000000 is out of range for type DWORD"},

		// LWORD (64-bit) overflows (max 18446744073709551615)
		{"LWORD#10#18446744073709551616;", "value 18446744073709551616 is out of range for type LWORD"},
		{"LWORD#16#10000000000000000;", "value 10000000000000000 is out of range for type LWORD"},

		// Invalid format
		{"BYTE#FFF;", "could not parse \"FFF\" as BYTE (base 10)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// These are evaluator errors, not parser errors.
			evaluated := testEval(t, tt.input)
			testErrorObjectContains(t, evaluated, tt.expected)
		})
	}
}

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"IF TRUE THEN 10; END_IF", 10},
		{"IF FALSE THEN 10; END_IF", nil},
		{"IF 1 = 1 THEN 10; END_IF", 10}, // Non-boolean conditions are false
		{"IF 1 < 2 THEN 10; END_IF", 10},
		{"IF 1 > 2 THEN 10; END_IF", nil},
		{"IF 1 > 2 THEN 10; ELSE 20; END_IF", 20},
		{"IF 1 < 2 THEN 10; ELSE 20; END_IF", 10},
		{"IF 1 THEN 10; END_IF", 10},
		{"IF 0 THEN 10; END_IF", nil},
	}

	for i, tt := range tests {
		t.Logf("Test %d/%d", i+1, len(tests))
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, tt.input, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"return 10;", 10},
		{"return 10; 9;", 10},
		{"return 2 * 5; 9;", 10},
		{"9; return 2 * 5; 9;", 10},
		{"IF (10 > 1) THEN RETURN 10; END_IF", 10},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.input, tt.expected) // Assuming testIntegerObject is in helper
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			"5 + true;",
			"unsupported operator '+' for types LINT and BOOLEAN",
		},
		{
			"5 + true; 5;",
			"unsupported operator '+' for types LINT and BOOLEAN",
		},
		{
			"-true;",
			"ERROR (1:1): unknown operator: - BOOLEAN",
		},
		{
			"true + false;",
			"unknown operator for BOOLEAN: +",
		},
		{
			"true + false + true + false;",
			"unknown operator for BOOLEAN: +",
		},
		{
			"5; true + false; 5;",
			"unknown operator for BOOLEAN: +",
		},
		{
			`'Hello' - 'World';`, // String subtraction is not supported
			"unsupported operator '-' for types STRING and STRING",
		},
		{
			`"Hello" - "World";`, // WString subtraction is not supported
			"unsupported operator '-' for types WSTRING and WSTRING",
		},
		{
			"IF (10 > 1) THEN true + false; END_IF",
			"unknown operator for BOOLEAN: +",
		},
		{
			`
			IF (10 < 1) THEN
				// DO NOTHING
			ELSE
				IF (10 > 1) THEN
					RETURN true + false;
				END_IF
				RETURN 1;
			END_IF
			`,
			"unknown operator for BOOLEAN: +",
		},
		{
			"foobar;",
			"ERROR (1:1): identifier not found: foobar",
		},
		{` // cspell:disable-line
			FUNCTION MyFunc : INT VAR_INPUT x : INT; END_VAR MyFunc := x; END_FUNCTION
			{"name": "beedance"}[MyFunc];`,
			"unusable as hash key: FUNCTION",
		},
		{
			`NOT 'string';`,
			"unknown operator: NOT STRING",
		},
		{
			`NOT "string";`,
			"ERROR (1:1): unknown operator: NOT WSTRING",
		},
		{
			`999[1];`,
			"ERROR (1:4): index operator not supported: LINT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testErrorObjectContains(t, evaluated, tt.expectedMessage)
		})
	}
}

func TestIntegerOverflowErrors(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		// SINT (-128 to 127)
		{"SINT#127 + SINT#1;", "SINT overflow: 128"},
		{"-SINT#127 - SINT#2;", "SINT underflow: -129"},
		{"SINT#64 * SINT#3;", "SINT overflow"},
		{"-SINT#65 * SINT#2;", "SINT underflow"},

		// // INT (-32768 to 32767)
		{"INT#32767 + INT#1;", "INT overflow: 32768"},
		{"-INT#32767 - INT#2;", "INT underflow: -32769"},
		{"INT#16384 * INT#3;", "INT overflow: 49152"},

		// // DINT (-2147483648 to 2147483647)
		{"DINT#2147483647 + INT#1;", "DINT overflow: 2147483648"},
		{"-DINT#2147483647 - INT#2;", "DINT underflow: -2147483649"},

		// // LINT (-9,223,372,036,854,775,808 to 9,223,372,036,854,775,807)
		{"LINT#9223372036854775807 + LINT#1;", "signed integer overflow"},
		{"-LINT#9223372036854775807 - LINT#2;", "signed integer underflow"},

		// // USINT (0 to 255)
		{"USINT#255 + USINT#1;", "USINT overflow: 256"},
		{"USINT#0 - USINT#1;", "unsigned integer underflow"},

		// // UINT (0 to 65535)
		{"UINT#65535 + UINT#1;", "UINT overflow: 65536"},
		{"UINT#0 - UINT#1;", "unsigned integer underflow"},

		// // UDINT (0 to 4294967295)
		{"UDINT#4294967295 + UINT#1;", "UDINT overflow: 4294967296"},
		{"UDINT#0 - UINT#1;", "unsigned integer underflow"},
		{"ULINT#18446744073709551615 + ULINT#1;", "unsigned integer overflow"},
		{"ULINT#0 - ULINT#1;", "unsigned integer underflow"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			testErrorObjectContains(t, evaluated, tt.expectedMessage)
		})
	}
}

func TestEvalCaseStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`
			VAR myVar : INT := 2; END_VAR
			CASE myVar OF
				1: 10;
				2: 20;
				3: 30;
				ELSE 40;
			END_CASE
		`, int64(20)},
		{`
			VAR myVar : INT := 3; END_VAR
			CASE myVar OF
				1: 10;
				2: 20;
				3: 30;
				ELSE 40;
			END_CASE
		`, int64(30)},
		{`
			VAR myVar : INT := 5; END_VAR
			CASE myVar OF
				1: 10;
				2: 20;
				3: 30;
				ELSE 40;
			END_CASE
		`, int64(40)},
		{`
			VAR myVar : INT := 1; END_VAR
			CASE myVar OF
				1: 10;
				ELSE 0;
			END_CASE
		`, int64(10)},
		{`
			VAR myVar : INT := 2; END_VAR
				CASE myVar OF
					1, 2: 10;
					ELSE 0;
				END_CASE
		`, int64(10)},
		{`
			VAR myVar : INT := 3; END_VAR
			CASE myVar OF
				1, 2: 100;
				3, 4: 200;
			END_CASE
		`, int64(200)},
		{`
			VAR myVar : INT := 10; END_VAR
			CASE myVar OF
				1..5: 100;
				6..10: 200;
			END_CASE
		`, int64(200)},
		{`
			CASE 'b' OF
				'a': 1;
				'b': 2;
				'c': 3;
			END_CASE
		`, int64(2)},
		{`
			CASE 2 OF
				1.0..2.0: 10;
			END_CASE
		`, int64(10)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				// The result of a CASE statement is the result of the executed statement.
				// Our test statements are just integer literals.
				// In a real program, this might be an assignment, and the result would be the assigned value. // cspell:disable-line
				// For this test, we check if the evaluated object is the expected integer.
				testIntegerObject(t, evaluated, tt.input, expected)
			case nil:
				testNullObject(t, evaluated)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestCaseStatementErrors(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			`CASE 1 OF 'a': 10; END_CASE`,
			"type mismatch for comparison: LINT = STRING",
		},
		{
			`CASE 'a' OF 1: 10; END_CASE`,
			"type mismatch for comparison: STRING = LINT",
		},
		{`
			 TYPE COLOR : (RED, GREEN, BLUE); END_TYPE
			 VAR myColor : INT := 1; END_VAR
			 CASE myColor OF
			 	COLOR#RED: 1;
			 	COLOR#GREEN: 2;
			 	COLOR#BLUE: 3;
			 END_CASE
		`, "type mismatch for comparison: INT = ENUMERATED_VALUE",
		},
		{
			`TYPE COLOR : (RED, GREEN, BLUE); END_TYPE
			 CASE COLOR#RED OF 1: 1; END_CASE`,
			"type mismatch for comparison: ENUMERATED_VALUE = LINT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testErrorObjectContains(t, evaluated, tt.expectedMessage)
		})
	}
}

func TestSubrangeTypeErrors(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			`TYPE MyRange : INT(1.0..10); END_TYPE`,
			"subrange bounds must be integers, got LREAL and LINT",
		},
		{
			`TYPE MyRange : INT(1..10.5); END_TYPE`,
			"subrange bounds must be integers, got LINT and LREAL",
		},
		{
			`TYPE MyRange : INT('a'..'z'); END_TYPE`,
			"subrange bounds must be integers, got STRING and STRING",
		},
		{
			`TYPE MyRange : REAL(0..100); END_TYPE`,
			"subrange base type must be an integer type",
		},
		{
			`TYPE MyRange : BOOL(0..1); END_TYPE`,
			"subrange base type must be an integer type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testErrorObjectContains(t, evaluated, tt.expectedMessage)
		})
	}
}

func TestForLoopStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`
			VAR total : INT := 0; END_VAR
			FOR i := 1 TO 5 DO
				total := total + i;
			END_FOR
			RETURN total;
		`, int64(15)}, // 1 + 2 + 3 + 4 + 5
		{`
			VAR total : INT := 10; END_VAR
			FOR i := 5 TO 1 BY -1 DO
				total := total - i;
			END_FOR
			RETURN total;
		`, int64(-5)}, // 10 - 5 - 4 - 3 - 2 - 1
		{`
			VAR total : INT := 0; END_VAR
			FOR i := 1 TO 10 DO
				total := total + 1;
				IF i = 5 THEN
					EXIT;
				END_IF
			END_FOR
			RETURN total;
		`, int64(5)}, // 1 + 2 + 3 + 4 + 5
		{`
			FOR i := 1 TO 1 DO
				RETURN 99;
			END_FOR
		`, int64(99)},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		expectedInt, _ := tt.expected.(int64)
		testIntegerObject(t, evaluated, tt.input, expectedInt) // Assuming testIntegerObject is in helper
	}
}

func TestWhileLoopStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`
			VAR x : INT := 0; END_VAR
			WHILE x < 5 DO
				x := x + 1;
			END_WHILE
			RETURN x;
		`, 5},
		{`
			VAR x : INT := 10; END_VAR
			WHILE x > 0 DO
				x := x - 2;
			END_WHILE
			RETURN x;
		`, 0},
		{`
			VAR x : INT := 0; END_VAR
			WHILE x < 10 DO
				x := x + 1;
				IF x = 7 THEN
					EXIT;
				END_IF
			END_WHILE
			RETURN x;
		`, 7},
		{`
			VAR x : INT := 10; END_VAR
			WHILE x > 10 DO
				x := x + 1;
			END_WHILE
			RETURN x;
		`, 10}, // Loop body should not execute
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			testIntegerObject(t, evaluated, tt.input, tt.expected)
		})
	}
}

func TestRepeatLoopStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`
			VAR x : INT := 0; END_VAR
			REPEAT
				x := x + 1;
			UNTIL x >= 5 END_REPEAT
			RETURN x;
		`, 5},
		{`
			VAR x : INT := 10; END_VAR
			REPEAT
				x := x + 1;
			UNTIL x > 10 END_REPEAT
			RETURN x;
		`, 11}, // Loop body executes at least once
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.input, tt.expected) // Assuming testIntegerObject is in helper
	}
}

func TestFunctionObject(t *testing.T) {
	// The original test used a non-standard "fn(x){...}" syntax for anonymous functions.
	// To comply with IEC 61131-3, functions must be declared using the formal
	// FUNCTION...END_FUNCTION syntax, which is named and explicitly typed.
	input := `
		FUNCTION TestFunc : INT
			VAR_INPUT
				x : INT;
			END_VAR
			TestFunc := x + 2;
		END_FUNCTION
	`

	evaluated := testEval(t, input)
	fn, ok := evaluated.(*object.Function)
	if !ok {
		t.Fatalf("object is not Function. got=%T (%+v)", evaluated, evaluated) // cspell:disable-line
	}

	if len(fn.VarInputs) != 1 {
		t.Fatalf("function has wrong parameters. Parameters=%+v",
			fn.VarInputs)
	}

	param := fn.VarInputs[0]
	if param.Name.Value != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", param.Name.Value)
	}
	if param.DataType.String() != "INT" {
		t.Fatalf("parameter type is not 'INT'. got=%q", param.DataType.String())
	}

	expectedBody := "TestFunc := (x + 2);"

	if fn.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, fn.Body.String())
	}
}

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`
			FUNCTION identity : INT
				VAR_INPUT 
					x : INT; 
				END_VAR
				identity := x;
			END_FUNCTION
			identity(5);
		`, 5},
		{`
			FUNCTION double : INT
				VAR_INPUT 
					x : INT; 
				END_VAR
				double := x * 2;
			END_FUNCTION
			double(5);
		`, 10},
		{`
			FUNCTION adder1 : INT
				VAR_INPUT
					x : INT;
					y : INT;
				END_VAR
				adder1 := x + y;
			END_FUNCTION
			adder1(5, 5);
		`, 10},
		{`
			FUNCTION adder2 : INT
				VAR_INPUT
					x : INT;
					y : INT;
				END_VAR
				adder2 := x + y;
			END_FUNCTION
			adder2(6, 5);
		`, 11},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.input, tt.expected) // Assuming testIntegerObject is in helper
	}
}

func TestStringLiteral(t *testing.T) {
	input := `"Hello World!";`

	evaluated := testEval(t, input)
	str, ok := evaluated.(*object.WString)
	if !ok {
		t.Fatalf("object is not WString. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("WString has wrong value. got=%q", str.Value)
	}
}

func TestStringConcatenation(t *testing.T) {
	input := `"Hello" + " " + "World!";`

	evaluated := testEval(t, input)
	str, ok := evaluated.(*object.WString)
	if !ok {
		t.Fatalf("object is not WString. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("WString has wrong value. got=%q", str.Value)
	}
}

func TestArrayLiterals(t *testing.T) {
	input := "[1, 2 * 2, 3 + 3];"

	evaluated := testEval(t, input)
	result, ok := evaluated.(*object.Array)
	if !ok {
		t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
	}

	if len(result.Elements) != 3 {
		t.Fatalf("array has wrong num of elements. got=%d",
			len(result.Elements))
	}

	testIntegerObject(t, result.Elements[0], "1", 1)
	testIntegerObject(t, result.Elements[1], "4", 4)
	testIntegerObject(t, result.Elements[2], "6", 6)
}

func TestArrayIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			"[1, 2, 3][0];",
			1,
		},
		{
			"[1, 2, 3][1];",
			2,
		},
		{
			"[1, 2, 3][2];",
			3,
		},
		{
			"[1, 2, 3][1 + 1];",
			3,
		},
		{
			"[1, 2, 3][3];",
			nil,
		},
		{
			"[1, 2, 3][-1];",
			nil,
		},
	}

	for i, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Logf("Test %d/%d", i+1, len(tests))
			evaluated := testEval(t, tt.input)
			integer, ok := tt.expected.(int)
			if ok {
				testIntegerObject(t, evaluated, tt.input, int64(integer))
			} else {
				testNullObject(t, evaluated)
			}
		})
	}
}

func TestHashIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			`{"foo": 5}["foo"];`,
			5,
		},
		{
			`{"foo": 5}["bar"];`,
			nil,
		},
		{
			`{}["foo"];`,
			nil,
		},
		{
			`{5: 5}[5];`,
			5,
		},
		{
			`{true: 5}[true];`,
			5,
		},
		{
			`{false: 5}[false];`,
			5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			integer, ok := tt.expected.(int)
			if ok {
				testIntegerObject(t, evaluated, tt.input, int64(integer)) // Assuming testIntegerObject is in helper
			} else {
				testNullObject(t, evaluated)
			}
		})
	}
}

func TestTypedLiteralEvaluation(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Valid conversions
		{"SINT#10;", int64(10)},
		{"INT#-100;", int64(-100)},
		{"DINT#123456;", int64(123456)},
		{"USINT#255;", int64(255)},
		{"UINT#65535;", int64(65535)},
		{"REAL#1.5;", 1.5},
		{"LREAL#1.23E-4;", 0.000123},
		{"BOOL#1;", true},
		{"BOOL#0;", false},
		{"BYTE#16#F0;", uint64(0xF0)},
		{"DATE#2026-05-21;", time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)},

		// Out-of-range errors
		{"SINT#128;", "value 128 is out of range for type SINT"},
		{"SINT#-129;", "value -129 is out of range for type SINT"},
		{"USINT#256;", "value 256 is out of range for type USINT"},
		{"USINT#-1;", "value -1 is out of range for type USINT"},
		{"BYTE#256;", "value 256 is out of range for type BYTE"},

		// Type mismatch errors
		{"INVALID_TYPE#10;", "unknown type: INVALID_TYPE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64: // cspell:disable-line
				testIntegerObject(t, evaluated, tt.input, expected)
			case float64: // cspell:disable-line
				testRealObject(t, evaluated, tt.input, expected)
			case bool:
				testBooleanObject(t, evaluated, tt.input, expected)
			case uint64:
				bs, ok := evaluated.(*object.BitString)
				if !ok {
					t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				}
				if bs.Value != expected {
					t.Errorf("wrong value. want=%d, got=%d", expected, bs.Value)
				}
			case string:
				testErrorObjectContains(t, evaluated, expected)
			case time.Time:
				dateObj, ok := evaluated.(*object.Date)
				if !ok {
					t.Fatalf("object is not Date. got=%T (%+v)", evaluated, evaluated)
				}
				if !dateObj.Value.Equal(expected) {
					t.Errorf("wrong date value. want=%v, got=%v", expected, dateObj.Value)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestLocatedVariables(t *testing.T) {
	input := `
		PROGRAM TestLocatedVars
			VAR
				myInput AT %IX0.1 : BOOL;
				myOutput AT %QX0.2 : BOOL;
			END_VAR

			myOutput := myInput;
		END_PROGRAM
	`
	ioMap = make(map[string]object.Object) // Reset for clean test
	env := object.NewEnvironment()
	// Manually set a value in the I/O map to simulate hardware input
	ioMap["%IX0.1"] = TRUE

	// Evaluate the program
	testEvalWithEnv(t, input, env) // Defines the program
	// Execute the program to run its logic
	testEvalWithEnv(t, "TestLocatedVars();", env)

	// Check the I/O map to see if the output was set correctly
	outputVal, ok := ioMap["%QX0.2"]
	if !ok {
		t.Fatalf("Output %%QX0.2 not found in I/O map")
	}

	if outputVal != TRUE {
		t.Errorf("Expected output to be TRUE, got %v", outputVal)
	}

	// Test reading back
	progObj, ok := env.Get("TestLocatedVars")
	if !ok {
		t.Fatalf("Program 'TestLocatedVars' not found in environment")
	}
	progEnv := progObj.(*object.Program).Env

	myInputObj, _ := progEnv.Get("myInput")
	if myInputObj.(*object.Pointer).Env != nil { // Located vars have nil Env
		t.Errorf("Expected myInput to be a located variable pointer")
	}
	// We need to evaluate the identifier to get the value from ioMap
	readVal := testEvalWithEnv(t, "myInput;", progEnv)
	if readVal != TRUE {
		t.Errorf("Expected reading myInput to be TRUE, got %v", readVal)
	}
}

func TestAssignmentToConstantError(t *testing.T) {
	input := `
		VAR CONSTANT MyConst : INT := 5; END_VAR
		MyConst := 10;
	`
	evaluated := testEval(t, input)
	testErrorObjectContains(t, evaluated, "cannot assign to constant variable 'MyConst'")
}

func TestMinusPrefixOnUnsigned(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"-USINT#10;", -10},
		{"-UINT#1000;", -1000},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		// Negating an unsigned results in a signed integer of at least the same size.
		// The evaluator promotes to SInt/Int which are then handled as LInt in tests.
		testIntegerObject(t, evaluated, tt.input, tt.expected)
	}
}

func TestOtherVarBlocks(t *testing.T) {
	t.Run("VAR_GLOBAL", func(t *testing.T) {
		input := `
			VAR_GLOBAL
				g_var : INT := 99;
			END_VAR
			g_var + 1;
		`
		evaluated := testEval(t, input)
		testIntegerObject(t, evaluated, "g_var + 1", 100)
	})

	t.Run("VAR_EXTERNAL", func(t *testing.T) {
		// External vars are just declarations. The evaluator creates a placeholder.
		// We can't resolve them without a global context, but we can test assignment.
		input := `
			VAR_EXTERNAL g_var : INT; END_VAR
			g_var := 123;
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)
		// Check that the variable was created in the environment and assigned to.
		testIntegerObjectInEnv(t, env, "g_var", 123)
	})

	t.Run("VAR_ACCESS", func(t *testing.T) {
		// Access vars are for linking to other programs. The evaluator treats them
		// like placeholders.
		input := `
			VAR_ACCESS MyAccess : Other.Var; END_VAR
			MyAccess := 456;
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)
		testIntegerObjectInEnv(t, env, "MyAccess", 456)
	})

	t.Run("VAR_TEMP in a single pass", func(t *testing.T) {
		// VAR_TEMP are re-initialized every cycle. In a single eval pass, they act like VAR.
		input := `
			PROGRAM TestTemp VAR_OUTPUT out : INT; END_VAR
				VAR_TEMP
					temp_var : INT := 5;
				END_VAR
				out := temp_var * 2;
			END_PROGRAM
			TestTemp(); TestTemp.out;
		`
		evaluated := testEval(t, input)
		testIntegerObject(t, evaluated, "TestTemp", 10)
	})

	t.Run("VAR_TEMP re-initialization", func(t *testing.T) {
		input := `
			PROGRAM TestTempReinit VAR_OUTPUT out : INT; END_VAR
				VAR_TEMP
					temp_var : INT := 0;
				END_VAR
				temp_var := temp_var + 1;
				out := temp_var;
			END_PROGRAM
		`
		env := object.NewEnvironment()
		// First, evaluate the program definition
		testEvalWithEnv(t, input, env)

		// First call
		evaluated1 := testEvalWithEnv(t, "TestTempReinit(); TestTempReinit.out;", env)
		testIntegerObject(t, evaluated1, "first call", 1)

		// Second call - temp_var should be re-initialized to 0, then incremented to 1 again.
		evaluated2 := testEvalWithEnv(t, "TestTempReinit(); TestTempReinit.out;", env)
		testIntegerObject(t, evaluated2, "second call", 1)
	})

	t.Run("VAR in PROGRAM should be static", func(t *testing.T) {
		input := `
			PROGRAM TestStaticVar VAR_OUTPUT out : INT; END_VAR
				VAR
					static_var : INT := 0;
				END_VAR
				static_var := static_var + 1;
				out := static_var;
			END_PROGRAM
		`
		env := object.NewEnvironment()
		// First, evaluate the program definition
		testEvalWithEnv(t, input, env)

		// First call
		evaluated1 := testEvalWithEnv(t, "TestStaticVar(); TestStaticVar.out;", env)
		testIntegerObject(t, evaluated1, "first call", 1)

		// Second call - static_var should retain its value and be incremented to 2.
		evaluated2 := testEvalWithEnv(t, "TestStaticVar(); TestStaticVar.out;", env)
		testIntegerObject(t, evaluated2, "second call", 2)
	})
}

func TestFunctionLiteralApplication(t *testing.T) {
	// This test assumes the parser supports the `fn(...)` syntax for anonymous functions,
	// which has been removed in favor of standard FUNCTION declarations.
	// The test is updated to reflect the standard syntax.
	input := `
		FUNCTION add : INT VAR_INPUT x, y : INT; END_VAR
			add := x + y;
		END_FUNCTION
		add(5, 10);
	`
	evaluated := testEval(t, input)
	testIntegerObject(t, evaluated, "add(5, 10)", 15)
}

func TestHashLiteralEvaluation(t *testing.T) {
	input := `{"one": 10 - 9, "two": 1 + 1, "three": 6 / 2};`

	evaluated := testEval(t, input)
	result, ok := evaluated.(*object.Hash)
	if !ok {
		t.Fatalf("Eval didn't return Hash. got=%T (%+v)", evaluated, evaluated)
	}

	// The input uses double quotes, so the keys are WStrings.
	expected := map[object.HashKey]int64{
		(&object.WString{Value: "one"}).HashKey():   1,
		(&object.WString{Value: "two"}).HashKey():   2,
		(&object.WString{Value: "three"}).HashKey(): 3,
	}
	if len(result.Pairs) != len(expected) {
		t.Fatalf("Hash has wrong num of pairs. want=%d, got=%d",
			len(expected), len(result.Pairs))
	}

	// Create a map of string keys to expected values for easier lookup.
	expectedValues := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	for _, pair := range result.Pairs {
		var keyStr string
		switch k := pair.Key.(type) {
		case *object.String:
			keyStr = k.Value
		case *object.WString:
			keyStr = k.Value
		default:
			t.Errorf("key is not a string type. got=%T", pair.Key)
			continue
		}
		expectedValue, ok := expectedValues[keyStr]
		if !ok {
			t.Errorf("unexpected key in hash: %s", keyStr)
		}
		testIntegerObject(t, pair.Value, keyStr, expectedValue)
	}
}

func TestBitwiseOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// NOT
		{"NOT BYTE#16#A5;", uint64(0x5A)},
		{"NOT WORD#16#FF00;", uint64(0x00FF)},
		{"NOT DWORD#16#FFFF0000;", uint64(0x0000FFFF)},
		{"NOT LWORD#16#FFFFFFFF00000000;", uint64(0x00000000FFFFFFFF)},
		{"NOT 123;", "unknown operator: NOT LINT"},

		// AND
		{"BYTE#16#A5 AND BYTE#16#F0;", uint64(0xA0)},
		{"WORD#16#1234 AND WORD#16#FFFF;", uint64(0x1234)},
		{"WORD#16#1234 & WORD#16#00FF;", uint64(0x0034)}, // Test with '&' alias

		// OR
		{"BYTE#16#A5 OR BYTE#16#F0;", uint64(0xF5)},
		{"WORD#16#1234 OR WORD#16#00FF;", uint64(0x12FF)},

		// XOR
		{"BYTE#16#A5 XOR BYTE#16#F0;", uint64(0x55)},
		{"WORD#16#1234 XOR WORD#16#FFFF;", uint64(0xEDCB)},

		// NAND
		{"BYTE#16#A5 NAND BYTE#16#F0;", uint64(0x5F)}, // NOT (A5 & F0) -> NOT(A0) -> 5F

		// NOR
		{"BYTE#16#A5 NOR BYTE#16#F0;", uint64(0x0A)}, // NOT (A5 | F0) -> NOT(F5) -> 0A

		// Combinations
		{"(BYTE#16#A5 OR BYTE#16#0F) AND BYTE#16#F0;", uint64(0xA0)},
		{"NOT (BYTE#16#A5 AND BYTE#16#F0);", uint64(0x5F)},

		// Error cases
		{"BYTE#16#A5 AND WORD#16#F0;", "type mismatch: bitstring operands must have same width, got 8 and 16"},
		{"BYTE#16#A5 OR 10;", "type mismatch: BITSTRING OR LINT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case uint64:
				bs, ok := evaluated.(*object.BitString)
				if !ok {
					t.Fatalf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				}
				if bs.Value != expected {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
				}
			case string:
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)",
						evaluated, evaluated)
					return
				}

				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q",
						expected, errObj.Message)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestFunctionCallWithMixedArguments(t *testing.T) {
	input := `
		FUNCTION MyFunc : INT
			VAR_INPUT
				PosIn : INT;
				NamedIn : INT;
			END_VAR
			VAR_OUTPUT
				NamedOut : INT;
			END_VAR
			VAR
				temp: INT;
			END_VAR

			temp := PosIn + NamedIn;
			NamedOut := temp * 2;
			MyFunc := temp;
		END_FUNCTION

		VAR
			ResultVar : INT;
			OutputVar : INT;
		END_VAR

		ResultVar := MyFunc(10, NamedIn := 20, NamedOut => OutputVar);
	`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()

	// Evaluate the program to declare the function and variables, and execute the call
	Eval(program, env)

	// 1. Check the primary return value of the function call
	// Expected: MyFunc returns `temp`, which is PosIn (10) + NamedIn (20) = 30
	_, ok1 := env.Get("ResultVar")
	if !ok1 {
		t.Fatalf("ResultVar not found in environment")
	}
	testIntegerObjectInEnv(t, env, "ResultVar", 30)

	// 2. Check the value of the variable connected to the output parameter
	// Expected: NamedOut is `temp` * 2 = 30 * 2 = 60. This should be assigned to OutputVar.
	_, ok2 := env.Get("OutputVar")
	if !ok2 {
		t.Fatalf("OutputVar not found in environment")
	}
	testIntegerObjectInEnv(t, env, "OutputVar", 60)
}

func TestNestedInOutVarPassing(t *testing.T) {
	input := `
		// Inner function that modifies the IN_OUT variable
		FUNCTION InnerFunc : INT
			VAR_IN_OUT
				InnerVar : INT;
			END_VAR
			InnerVar := InnerVar * 2;
			InnerFunc := InnerVar;
		END_FUNCTION

		// Outer function that calls the inner function
		FUNCTION OuterFunc : INT
			VAR_IN_OUT
				OuterVar : INT;
			END_VAR
			// Pass the IN_OUT variable to the inner function
			OuterFunc := InnerFunc(InnerVar := OuterVar);
		END_FUNCTION

		PROGRAM TestProg
			VAR
				OriginalVar : INT := 5;
				Result1 : INT;
				Result2 : INT;
			END_VAR

			// Call the outer function, which calls the inner one
			Result1 := OuterFunc(OuterVar := OriginalVar);
			// For verification, call the inner function directly
			Result2 := InnerFunc(InnerVar := OriginalVar);
		END_PROGRAM
	`

	env := object.NewEnvironment()
	evaluated := testEvalWithEnv(t, input, env)
	if err, ok := evaluated.(*object.Error); ok {
		t.Fatalf("Evaluator error: %s", err.Message)
	}

	// Execute the program to run its logic
	testEvalWithEnv(t, "TestProg();", env)

	progObj, _ := env.Get("TestProg")
	progEnv := progObj.(*object.Program).Env

	// After the first call `OuterFunc(OuterVar := OriginalVar)`:
	// OriginalVar starts at 5.
	// OuterFunc passes it to InnerFunc.
	// InnerFunc modifies it to 5 * 2 = 10.
	// The program then calls InnerFunc again, making OriginalVar 20.
	// So, after the entire program evaluation, OriginalVar should be 20.
	testIntegerObjectInEnv(t, progEnv, "OriginalVar", 20)
}
