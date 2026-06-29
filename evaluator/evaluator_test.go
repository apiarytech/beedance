package evaluator

import (
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"math"
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
		testIntegerObject(t, evaluated, tt.expected)
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
		testBooleanObject(t, evaluated, tt.expected)
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
		testBooleanObject(t, testEval(t, tt.input), tt.expected)
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
		{"NOT 5;", "ERROR (1:1): unknown operator: NOTINT"},
		{"NOT 3.14;", "ERROR (1:1): unknown operator: NOTLREAL"},
		//{`NOT "hello";`, "ERROR (1:1): unknown operator: NOTSTRING"},
		// Nested NOT on invalid type should also error
		{"NOT NOT 5;", "ERROR (1:5): unknown operator: NOTINT"},
		// ANY_BIT bitwise tests
		// {"NOT BYTE#16#A5;", uint64(0x5A)}, // NOT 10100101 -> 01011010
		// {"NOT WORD#16#FF00;", uint64(0x00FF)},
		// {"NOT DWORD#16#FFFF0000;", uint64(0x0000FFFF)},
		// {"NOT LWORD#16#FFFFFFFF00000000;", uint64(0x00000000FFFFFFFF)},
		// Double NOT on bitstrings
		// {"NOT NOT BYTE#16#A5;", uint64(0xA5)},
		// {"NOT NOT WORD#16#FF00;", uint64(0xFF00)},
		// {"NOT NOT DWORD#16#FFFF0000;", uint64(0xFFFF0000)},
		// {"NOT NOT LWORD#16#FFFFFFFF00000000;", uint64(0xFFFFFFFF00000000)},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case bool:
			testBooleanObject(t, evaluated, expected)
		case string:
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Errorf("object is not Error. got=%T(%+v)", evaluated, evaluated)
				continue
			}
			if errObj.Message != expected {
				t.Errorf("wrong error message. expected=%q, got=%q", expected, errObj.Message)
			}
		case uint64:
			bs, ok := evaluated.(*object.BitString)
			if !ok {
				t.Errorf("object is not BitString. got=%T (%+v)", evaluated, evaluated)
				continue
			}
			if bs.Value != expected {
				t.Errorf("wrong bitstring value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
			}
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
				testBitStringObject(t, evaluated, expectedUint)
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
		{"BYTE#10#256;", `value "256" is out of range for type BYTE`},
		{"BYTE#16#100;", `value "100" is out of range for type BYTE`}, // 0x100 = 256
		{"BYTE#8#400;", `value "400" is out of range for type BYTE`},  // 400 octal = 256 decimal
		{"BYTE#2#100000000;", `value "100000000" is out of range for type BYTE`},

		// WORD (16-bit) overflows (max 65535)
		{"WORD#10#65536;", `value "65536" is out of range for type WORD`},
		{"WORD#16#10000;", `value "10000" is out of range for type WORD`},
		{"WORD#8#200000;", `value "200000" is out of range for type WORD`},
		{"WORD#2#10000000000000000;", `value "10000000000000000" is out of range for type WORD`},

		// DWORD (32-bit) overflows (max 4294967295)
		{"DWORD#10#4294967296;", `value "4294967296" is out of range for type DWORD`},
		{"DWORD#16#100000000;", `value "100000000" is out of range for type DWORD`},

		// LWORD (64-bit) overflows (max 18446744073709551615)
		{"LWORD#10#18446744073709551616;", `value "18446744073709551616" is out of range for type LWORD`},
		{"LWORD#16#10000000000000000;", `value "10000000000000000" is out of range for type LWORD`},

		// Invalid format
		{"BYTE#FFF;", `value "FFF" is out of range for type BYTE (width 8)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// For expected error cases, we allow parser errors.
			_, pErrors := testEvalWithParserErrors(t, tt.input)
			if len(pErrors) == 0 {
				t.Fatalf("expected parser errors, but got none")
			}
			testParserErrorContains(t, pErrors, tt.expected)
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
		{"IF 1 THEN 10; END_IF", nil},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
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
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			"5 + true;",
			"ERROR (1:3): type mismatch: INT + BOOLEAN",
		},
		{
			"5 + true; 5;",
			"ERROR (1:3): type mismatch: INT + BOOLEAN",
		},
		{
			"-true;",
			"ERROR (1:1): unknown operator: -BOOLEAN",
		},
		{
			"true + false;",
			"ERROR (1:6): unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"true + false + true + false;",
			"ERROR (1:6): unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"5; true + false; 5;",
			"ERROR (1:9): unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			`'Hello' - 'World';`,
			"ERROR (1:9): unknown operator: STRING - STRING",
		},
		{
			`"Hello" - "World";`,
			"ERROR (1:9): unknown operator: WSTRING - WSTRING",
		},
		{
			"IF (10 > 1) THEN true + false; END_IF",
			"ERROR (1:23): unknown operator: BOOLEAN + BOOLEAN",
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
			"ERROR (6:18): unknown operator: BOOLEAN + BOOLEAN",
		},
		{
			"foobar;",
			"ERROR (1:1): identifier not found: foobar",
		},
		{
			`{"name": "beedance"}[fn(x) { x ;}];`,
			"ERROR (1:21): unusable as hash key: FUNCTION",
		},
		{
			`NOT 'string';`,
			"ERROR (1:1): unknown operator: NOTSTRING",
		},
		{
			`NOT "string";`,
			"ERROR (1:1): unknown operator: NOTWSTRING",
		},
		{
			`999[1];`,
			"ERROR (1:4): index operator not supported: INT",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		errObj, ok := evaluated.(*object.Error)
		if !ok {
			t.Errorf("no error object returned. got=%T(%+v)",
				evaluated, evaluated)
		}

		if errObj.Message != tt.expectedMessage {
			t.Errorf("wrong error message. expected=%q, got=%q",
				tt.expectedMessage, errObj.Message)
		}
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
		{"SINT#64 * SINT#3;", "SINT overflow: 192"},
		{"-SINT#65 * SINT#2;", "SINT underflow: -130"},

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

			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Errorf("no error object returned for input '%s'. got=%T(%+v)",
					tt.input, evaluated, evaluated)
			}

			// We check for a substring because the error message includes line/column info.
			if !strings.Contains(errObj.Message, tt.expectedMessage) {
				t.Errorf("wrong error message for input '%s'.\nexpected to contain: %q\ngot: %q",
					tt.input, tt.expectedMessage, errObj.Message)
			}
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
				// In a real program, this might be an assignment, and the result would be the assigned value.
				// For this test, we check if the evaluated object is the expected integer.
				testIntegerObject(t, evaluated, expected)
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
			"CASE 1 OF 'a': 10; END_CASE",
			"type mismatch for comparison: INT = STRING",
		},
		{
			"CASE 'a' OF 1: 10; END_CASE",
			"type mismatch for comparison: STRING = INT",
		},
		{`
			 TYPE COLOR : (RED, GREEN, BLUE); END_TYPE
			 VAR myColor : INT := 1; END_VAR
			 CASE myColor OF 
			 	COLOR#RED: 1; 
			 	COLOR#GREEN: 2; 
			 	COLOR#BLUE: 3; 
			 END_CASE
		`, "ERROR (5:12): identifier not found: RED",
		},
		{
			`TYPE COLOR : (RED, GREEN, BLUE); END_TYPE
			 CASE COLOR#RED OF 1: 1; END_CASE`,
			"ERROR (2:16): identifier not found: RED",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testErrorObjectContains(t, evaluated, tt.expectedMessage)
	}
}

func TestSubrangeTypeErrors(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			`TYPE MyRange : INT(1.0..10); END_TYPE`,
			"subrange bounds must be integers",
		},
		{
			`TYPE MyRange : INT(1..10.5); END_TYPE`,
			"subrange bounds must be integers",
		},
		{
			`TYPE MyRange : INT('a'..'z'); END_TYPE`,
			"subrange bounds must be integers",
		},
		{
			`TYPE MyRange : REAL(0..100); END_TYPE`,
			"ERROR (1:6): subrange bounds must be integers",
		},
		{
			`TYPE MyRange : BOOL(0..1); END_TYPE`,
			"ERROR (1:6): subrange bounds must be integers",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testErrorObjectContains(t, evaluated, tt.expectedMessage)
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
		testIntegerObject(t, evaluated, expectedInt)
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
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.expected)
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
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestSFCExecution(t *testing.T) {
	input := `
		PROGRAM TestSFC
			VAR
				x : INT := 0;
				cond1 : BOOL := FALSE;
				cond2 : BOOL := FALSE;
			END_VAR

			ACTION Step1Action: x := 1; END_ACTION
			ACTION Step2Action: x := x + 10; END_ACTION
			ACTION Step3Action: x := x + 100; END_ACTION

			INITIAL_STEP S1: Step1Action(); END_STEP

			TRANSITION FROM S1 TO S2 := cond1; END_TRANSITION

			STEP S2: Step2Action(); END_STEP

			TRANSITION FROM S2 TO S3 := cond2; END_TRANSITION

			STEP S3: Step3Action(); END_STEP

		END_PROGRAM
	`

	// We need to manage the environment manually for this test to check variables across cycles.
	env := object.NewEnvironment()
	sfcObj := testEvalWithEnv(t, input, env)

	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("TestSFC is not an SFC object, got %T", sfcObj)
	}

	// Ensure transition conditions are false initially to control the test flow.
	env.Set("cond1", FALSE)
	env.Set("cond2", FALSE)

	// --- Cycle 1: Initial state ---
	// S1 is active. The action "x := 1" should execute.
	evalSFCCycle(sfc, env)
	if !sfc.Steps["S1"].IsActive {
		t.Fatal("S1 should be active initially")
	}
	testIntegerObject(t, mustGet(env, "x"), 1)

	// --- Cycle 2: Transition to S2 ---
	// Set the condition and run the cycle. The transition should clear,
	// S1 should deactivate, and S2 should activate and execute its action.
	env.Set("cond1", TRUE)
	evalSFCCycle(sfc, env)
	if sfc.Steps["S1"].IsActive || !sfc.Steps["S2"].IsActive {
		t.Fatal("Should have transitioned to S2")
	}
	testIntegerObject(t, mustGet(env, "x"), 11) // S1's action ran in cycle 1 (x=1). S2's action runs in cycle 2 (x=1+10).

	// --- Cycle 3: Still in S2 ---
	// The transition condition `cond2` is FALSE. S2 remains active.
	// The action "x := x + 10" executes again.
	evalSFCCycle(sfc, env)
	testIntegerObject(t, mustGet(env, "x"), 21) // S2's action runs again: 11 + 10
}

func TestSFCActionQualifiers(t *testing.T) {
	input := `
		PROGRAM TestSFCQualifiers
		VAR
			// Action variables
			ActionN, ActionS, ActionP : BOOL;
			// Conditions
			GoToS2, GoToS3, GoToS4, Reset : BOOL;
		END_VAR

		INITIAL_STEP S1:
			ActionN(N);
			ActionS(S);
		END_STEP

		TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION

		STEP S2:
			ActionP(P);
		END_STEP

		TRANSITION FROM S2 TO S3 := GoToS3; END_TRANSITION

		STEP S3:
			ActionS(R); // Reset the 'ActionS' variable
		END_STEP

		TRANSITION FROM S3 TO S1 := Reset; END_TRANSITION
		END_PROGRAM
	`

	env := object.NewEnvironment()
	progInstance := testEvalWithEnv(t, input, env)
	sfc, ok := progInstance.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", progInstance)
	}

	// --- Cycle 1: Initial state ---
	// S1 is active. ActionN and ActionS should be TRUE. ActionP and ActionR_S are FALSE.
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionN"), true)
	testBooleanObject(t, mustGet(env, "ActionS"), true)
	testBooleanObject(t, mustGet(env, "ActionP"), false)

	// --- Cycle 2: Transition from S1 to S2 ---
	// Set condition and cycle. S1 becomes inactive, S2 becomes active.
	env.Set("GoToS2", TRUE)
	evalSFCCycle(sfc, env)
	// ActionN (Non-stored) becomes FALSE as S1 is no longer active.
	// ActionS (Set) remains TRUE.
	// ActionP (Pulse) becomes TRUE for this one cycle.
	testBooleanObject(t, mustGet(env, "ActionN"), false)
	testBooleanObject(t, mustGet(env, "ActionS"), true)
	testBooleanObject(t, mustGet(env, "ActionP"), true)

	// --- Cycle 3: S2 is active ---
	// Reset condition. Cycle again.
	env.Set("GoToS2", FALSE)
	evalSFCCycle(sfc, env)
	// ActionP (Pulse) should now be FALSE again.
	// ActionS remains TRUE.
	testBooleanObject(t, mustGet(env, "ActionP"), false)
	testBooleanObject(t, mustGet(env, "ActionS"), true)

	// --- Cycle 4: Transition from S2 to S3 ---
	// Set condition and cycle. S2 becomes inactive, S3 becomes active.
	env.Set("GoToS3", TRUE)
	evalSFCCycle(sfc, env)
	// S3 is now active. It has an 'R' qualifier for 'ActionS'.
	// This should force 'ActionS' to become FALSE, even though it was 'Set' before.
	testBooleanObject(t, mustGet(env, "ActionS"), false)
}

// Mockable time for testing
var mockTime time.Time

func advanceMockTime(d time.Duration) {
	mockTime = mockTime.Add(d)
}

func TestSFCDivergenceConvergence(t *testing.T) {
	input := `
		PROGRAM TestSFCBranching
			VAR
				// Action variables
				PathA_Active, PathB_Active, PathC_Active, Merged_Active : BOOL;
				// Conditions
				SelectA, SelectB, Fork, Join : BOOL;
			END_VAR

			INITIAL_STEP S1: END_STEP

			// Selection Divergence
			TRANSITION FROM S1 TO S2 := SelectA; END_TRANSITION
			TRANSITION FROM S1 TO S3 := SelectB; END_TRANSITION

			STEP S2: PathA_Active(N); END_STEP
			STEP S3: PathB_Active(N); END_STEP

			// Selection Convergence
			TRANSITION FROM S2 TO S4 := TRUE; END_TRANSITION
			TRANSITION FROM S3 TO S4 := TRUE; END_TRANSITION

			STEP S4: Merged_Active(N); END_STEP

			// Simultaneous Divergence (Fork)
			TRANSITION FROM S4 TO (S5, S6) := Fork; END_TRANSITION

			STEP S5: PathA_Active(S); END_STEP // Use Set to see state over cycles
			STEP S6: PathB_Active(S); END_STEP

			// Some intermediate steps
			TRANSITION FROM S5 TO S7 := TRUE; END_TRANSITION
			STEP S7: END_STEP
			TRANSITION FROM S6 TO S8 := TRUE; END_TRANSITION
			STEP S8: END_STEP

			// Simultaneous Convergence (Join)
			TRANSITION FROM (S7, S8) TO S9 := Join; END_TRANSITION

			STEP S9: PathC_Active(N); END_STEP

		END_PROGRAM
	`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkEvaluatorErrors(t, p, "TestSFCBranching", input)

	env := object.NewEnvironment()
	// This will declare the PROGRAM POU and its variables
	Eval(program, env)

	// Now, instantiate the program to get the SFC object
	progInstance := testEval(t, "TestSFCBranching")
	sfc, ok := progInstance.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", progInstance)
	}

	// --- Cycle 1: Initial state ---
	evalSFCCycle(sfc, env)
	if !sfc.Steps["S1"].IsActive {
		t.Fatal("S1 should be active initially")
	}

	// --- Cycle 2: Test Selection Divergence ---
	env.Set("SelectA", TRUE) // Choose path A
	evalSFCCycle(sfc, env)
	if sfc.Steps["S1"].IsActive || !sfc.Steps["S2"].IsActive || sfc.Steps["S3"].IsActive {
		t.Fatal("Selection divergence failed: S1 should be inactive, S2 active, S3 inactive")
	}

	// --- Cycle 3: Test Selection Convergence ---
	evalSFCCycle(sfc, env)
	if sfc.Steps["S2"].IsActive || !sfc.Steps["S4"].IsActive {
		t.Fatal("Selection convergence failed: S2 should be inactive, S4 active")
	}

	// --- Cycle 4: Test Simultaneous Divergence (Fork) ---
	env.Set("Fork", TRUE)
	evalSFCCycle(sfc, env)
	if sfc.Steps["S4"].IsActive || !sfc.Steps["S5"].IsActive || !sfc.Steps["S6"].IsActive {
		t.Fatal("Simultaneous divergence failed: S4 should be inactive, S5 and S6 should be active")
	}

	// --- Cycle 5 & 6: Let parallel paths advance ---
	evalSFCCycle(sfc, env) // S5->S7, S6->S8
	if !sfc.Steps["S7"].IsActive || !sfc.Steps["S8"].IsActive {
		t.Fatal("Parallel paths did not advance correctly")
	}

	// --- Cycle 7: Test Simultaneous Convergence (Join) ---
	env.Set("Join", TRUE)
	evalSFCCycle(sfc, env)
	if sfc.Steps["S7"].IsActive || sfc.Steps["S8"].IsActive || !sfc.Steps["S9"].IsActive {
		t.Fatal("Simultaneous convergence failed: S7/S8 should be inactive, S9 active")
	}
}

func TestSFCActionQualifiersTimed(t *testing.T) {
	// Initialize mock time for this test
	mockTime = time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC)
	// Override the global nowFunc in evaluator package for testing
	originalNowFunc := nowFunc
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }() // Restore original nowFunc after test

	input := `
		PROGRAM TestSFCTimedQualifiers
			VAR
				// Action variables
				ActionD_Q, ActionL_Q, ActionSD_Q, ActionDS_Q, ActionSL_Q : BOOL;
				// Conditions
				GoToS2, GoToS3, GoToS4, GoToS5 : BOOL;
			END_VAR

			INITIAL_STEP S1:
				ActionD_Q(D, T#5s);
				ActionL_Q(L, T#3s);
			END_STEP

			TRANSITION FROM S1 TO S2 := GoToS2; END_TRANSITION

			STEP S2:
				ActionSD_Q(SD, T#2s);
				ActionDS_Q(DS, T#4s);
			END_STEP

			TRANSITION FROM S2 TO S3 := GoToS3; END_TRANSITION

			STEP S3:
				ActionSL_Q(SL, T#6s);
			END_STEP

			TRANSITION FROM S3 TO S4 := GoToS4; END_TRANSITION

			STEP S4:
				(* Final step *)
			END_STEP
		END_PROGRAM
	`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkEvaluatorErrors(t, p, "TestSFCTimedQualifiers", input)

	env := object.NewEnvironment()
	Eval(program, env) // Declare the PROGRAM POU

	progInstance := testEval(t, "TestSFCTimedQualifiers")
	sfc, ok := progInstance.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", progInstance)
	}

	// --- Cycle 1: Initial state (S1 active) ---
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionD_Q"), false) // D: Not active yet
	testBooleanObject(t, mustGet(env, "ActionL_Q"), true)  // L: Active immediately

	// --- Cycle 2: Advance time by 2s ---
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionD_Q"), false) // D: Still not active
	testBooleanObject(t, mustGet(env, "ActionL_Q"), true)  // L: Still active

	// --- Cycle 3: Advance time by another 2s (total 4s) ---
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionD_Q"), true)  // D: Now active (5s delay passed)
	testBooleanObject(t, mustGet(env, "ActionL_Q"), false) // L: Now inactive (3s limit passed)

	// --- Cycle 4: Transition S1 -> S2 (GoToS2 = TRUE) ---
	env.Set("GoToS2", TRUE)
	advanceMockTime(1 * time.Second) // Advance time to ensure transitions are processed
	evalSFCCycle(sfc, env)
	// S1 is inactive, S2 is active.
	// ActionD_Q and ActionL_Q should be reset to FALSE.
	testBooleanObject(t, mustGet(env, "ActionD_Q"), false)
	testBooleanObject(t, mustGet(env, "ActionL_Q"), false)
	testBooleanObject(t, mustGet(env, "ActionSD_Q"), false) // SD: Not active yet
	testBooleanObject(t, mustGet(env, "ActionDS_Q"), false) // DS: Not active yet

	// --- Cycle 5: Advance time by 2s (total 7s) ---
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionSD_Q"), true)  // SD: Now active (2s delay passed)
	testBooleanObject(t, mustGet(env, "ActionDS_Q"), false) // DS: Still not active

	// --- Cycle 6: Advance time by another 2s (total 9s) ---
	advanceMockTime(2 * time.Second)
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionSD_Q"), true)  // SD: Remains active
	testBooleanObject(t, mustGet(env, "ActionDS_Q"), true)  // DS: Now active (4s delay passed)
	testBooleanObject(t, mustGet(env, "ActionSL_Q"), false) // SL: Not active yet (S3 not active)

	// --- Cycle 7: Transition S2 -> S3 (GoToS3 = TRUE) ---
	env.Set("GoToS3", TRUE)
	advanceMockTime(1 * time.Second) // Advance time
	evalSFCCycle(sfc, env)
	// S2 inactive, S3 active. ActionSD_Q and ActionDS_Q remain TRUE (stored).
	testBooleanObject(t, mustGet(env, "ActionSD_Q"), true)
	testBooleanObject(t, mustGet(env, "ActionDS_Q"), true)
	testBooleanObject(t, mustGet(env, "ActionSL_Q"), true) // SL: Active immediately

	// --- Cycle 8: Advance time by 6s (total 16s) ---
	advanceMockTime(6 * time.Second)
	evalSFCCycle(sfc, env)
	testBooleanObject(t, mustGet(env, "ActionSL_Q"), false) // SL: Now inactive (6s limit passed)
}

func TestSFCActionWithSTBody(t *testing.T) {
	input := `
		PROGRAM TestSFC_ST_Action
			VAR
				Counter : INT := 0;
				GoToStep2 : BOOL := FALSE;
				GoToStep1 : BOOL := FALSE;
			END_VAR

			ACTION IncrementCounter:
				Counter := Counter + 1;
			END_ACTION

			INITIAL_STEP S1:
				(* Do nothing *)
			END_STEP

			TRANSITION FROM S1 TO S2 := GoToStep2;
			END_TRANSITION

			STEP S2:
				IncrementCounter(N); (* Non-stored action *)
			END_STEP

			TRANSITION FROM S2 TO S1 := GoToStep1;
			END_TRANSITION

		END_PROGRAM
	`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkEvaluatorErrors(t, p, "TestSFCActionWithSTBody", input)

	env := object.NewEnvironment()
	// This will declare the PROGRAM POU and its variables
	Eval(program, env)

	// Now, instantiate the program to get the SFC object
	progInstance := testEval(t, "TestSFC_ST_Action")
	sfc, ok := progInstance.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", progInstance)
	}

	// --- Cycle 1: Initial state (S1 active) ---
	evalSFCCycle(sfc, env)
	testIntegerObject(t, mustGet(env, "Counter"), 0) // Action body should not have run

	// --- Cycle 2: Transition to S2 ---
	env.Set("GoToStep2", TRUE)
	evalSFCCycle(sfc, env)
	testIntegerObject(t, mustGet(env, "Counter"), 1) // Action body runs for the first time

	// --- Cycle 3: Still in S2 ---
	env.Set("GoToStep2", FALSE) // Prevent immediate re-transition
	evalSFCCycle(sfc, env)
	testIntegerObject(t, mustGet(env, "Counter"), 2) // Action body runs again
}

func TestFunctionObject(t *testing.T) {
	input := "fn(x) { x + 2; };"

	evaluated := testEval(t, input)
	fn, ok := evaluated.(*object.Function)
	if !ok {
		t.Fatalf("object is not Function. got=%T (%+v)", evaluated, evaluated)
	}

	if len(fn.VarInputs) != 1 {
		t.Fatalf("function has wrong parameters. Parameters=%+v",
			fn.VarInputs)
	}

	if fn.VarInputs[0].Name.Value != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", fn.VarInputs[0].Name.Value)
	}

	expectedBody := "(x + 2);"

	if fn.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, fn.Body.String())
	}
}

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"fn(x) { x; }(5);", 5},
	}

	for _, tt := range tests {
		testIntegerObject(t, testEval(t, tt.input), tt.expected)
	}
}

func TestStringLiteral(t *testing.T) {
	input := `"Hello World!";`

	evaluated := testEval(t, input)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestStringConcatenation(t *testing.T) {
	input := `"Hello" + " " + "World!";`

	evaluated := testEval(t, input)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`len("");`, 0},
		{`len("four");`, 4},
		{`len("hello world");`, 11},
		{`len(1);`, "argument to `len` not supported, got INTEGER"},
		{`len("one", "two");`, "wrong number of arguments. got=2, want=1"},
		{`len([1, 2, 3]);`, 3},
		{`len([]);`, 0},
		{`puts("hello", "world!");`, nil},
		{`first([1, 2, 3]);`, 1},
		{`first([]);`, nil},
		{`first(1);`, "argument to `first` must be ARRAY, got INTEGER"},
		{`last([1, 2, 3]);`, 3},
		{`last([]);`, nil},
		{`last(1);`, "argument to `last` must be ARRAY, got INTEGER"},
		{`rest([1, 2, 3]);`, []int{2, 3}},
		{`rest([]);`, nil},
		{`push([], 1);`, []int{1}},
		{`push(1, 1);`, "argument to `push` must be ARRAY, got INTEGER"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case nil:
			testNullObject(t, evaluated)
		case string:
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Errorf("object is not Error. got=%T (%+v)",
					evaluated, evaluated)
			}
			if errObj.Message != expected {
				t.Errorf("wrong error message. expected=%q, got=%q",
					expected, errObj.Message)
			}
		case []int:
			array, ok := evaluated.(*object.Array)
			if !ok {
				t.Errorf("obj not Array. got=%T (%+v)", evaluated, evaluated)
			}

			if len(array.Elements) != len(expected) {
				t.Errorf("wrong num of elements. want=%d, got=%d",
					len(expected), len(array.Elements))
				continue
			}

			for i, expectedElem := range expected {
				testIntegerObject(t, array.Elements[i], int64(expectedElem))
			}
		}
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

	testIntegerObject(t, result.Elements[0], 1)
	testIntegerObject(t, result.Elements[1], 4)
	testIntegerObject(t, result.Elements[2], 6)
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

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
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
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestBuiltinAddSub(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, time.Duration, time.Time, or string for error
	}{
		// ADD operations
		{"ADD(T#1s, T#2s);", 3 * time.Second},
		{"ADD(TIME#1m, TIME#30s);", (1 * time.Minute) + (30 * time.Second)},
		{"ADD(TOD#10:00:00, T#1h);", time.Date(0, 1, 1, 11, 0, 0, 0, time.UTC)},               // TOD + TIME
		{"ADD(TOD#23:00:00, T#2h);", time.Date(0, 1, 2, 1, 0, 0, 0, time.UTC)},                // TOD + TIME with wrap-around
		{"ADD(DT#2026-04-30-10:00:00, T#1h);", time.Date(2026, 4, 30, 11, 0, 0, 0, time.UTC)}, // DT + TIME
		{"ADD(10, 20);", int64(30)},
		{"ADD(1.5, 2.5);", 4.0},
		{"ADD(10, 2.5);", 12.5}, // INT + REAL promotion
		{"ADD(1.5, 20);", 21.5}, // REAL + INT promotion

		// SUB operations
		{"SUB(T#5s, T#2s);", 3 * time.Second},
		{"SUB(TIME#2m, TIME#30s);", (1 * time.Minute) + (30 * time.Second)},
		{"SUB(D#2026-04-30, D#2026-04-29);", 24 * time.Hour},                                 // DATE - DATE -> TIME
		{"SUB(TOD#10:00:00, T#1h);", time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC)},               // TOD - TIME
		{"SUB(TOD#01:00:00, T#2h);", time.Date(0, 1, 0, 23, 0, 0, 0, time.UTC)},              // TOD - TIME with wrap-around
		{"SUB(TOD#10:00:00, TOD#09:00:00);", 1 * time.Hour},                                  // TOD - TOD -> TIME
		{"SUB(DT#2026-04-30-10:00:00, T#1h);", time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)}, // DT - TIME
		{"SUB(DT#2026-04-30-10:00:00, DT#2026-04-30-09:00:00);", 1 * time.Hour},              // DT - DT -> TIME
		{"SUB(20, 10);", int64(10)},
		{"SUB(4.0, 1.5);", 2.5},
		{"SUB(10, 2.5);", 7.5},
		{"SUB(4.0, 2);", 2.0},

		// Error cases
		{"ADD(T#1s, D#2026-04-30);", "unsupported argument types for ADD: TIME + DATE"},
		{"SUB(T#1s, D#2026-04-30);", "unsupported argument types for SUB: TIME - DATE"},
		{"ADD(10, TRUE);", "unsupported argument types for ADD: INTEGER + BOOLEAN"},
		{"SUB(10, TRUE);", "unsupported argument types for SUB: INTEGER + BOOLEAN"},
		{"ADD(T#1s);", "wrong number of arguments for ADD. got=1, want=2"},
		{"SUB(T#1s);", "wrong number of arguments for SUB. got=1, want=2"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		switch expected := tt.expected.(type) {
		case time.Duration:
			timeObj, ok := evaluated.(*object.Time)
			if !ok {
				t.Errorf("input: %q, object is not Time. got=%T (%+v)", tt.input, evaluated, evaluated)
				continue
			}
			if timeObj.Value != expected {
				t.Errorf("input: %q, wrong time duration value. want=%v, got=%v", tt.input, expected, timeObj.Value)
			}
		case time.Time:
			switch evaluated := evaluated.(type) {
			case *object.Date:
				if !evaluated.Value.Equal(expected) {
					t.Errorf("input: %q, wrong date value. want=%v, got=%v", tt.input, expected, evaluated.Value)
				}
			case *object.TimeOfDay:
				// For TOD, we only compare the time part, not the date part.
				// The expected time.Time will have a default date (Jan 1, year 0000 or Jan 2, year 0000 for wrap-around).
				// We need to compare only the time components.
				if evaluated.Value.Hour() != expected.Hour() ||
					evaluated.Value.Minute() != expected.Minute() ||
					evaluated.Value.Second() != expected.Second() ||
					evaluated.Value.Nanosecond() != expected.Nanosecond() {
					t.Errorf("input: %q, wrong time of day value. want=%v, got=%v", tt.input, expected.Format("15:04:05.999999999"), evaluated.Value.Format("15:04:05.999999999"))
				}
			case *object.DateAndTime:
				if !evaluated.Value.Equal(expected) {
					t.Errorf("input: %q, wrong date and time value. want=%v, got=%v", tt.input, expected, evaluated.Value)
				}
			default:
				t.Errorf("input: %q, object is not a known time.Time-based type. got=%T (%+v)", tt.input, evaluated, evaluated)
			}
		case int64:
			testIntegerObject(t, evaluated, expected)
		case float64:
			realObj, ok := evaluated.(*object.Real)
			if !ok {
				t.Errorf("input: %q, object is not Real. got=%T (%+v)", tt.input, evaluated, evaluated)
				continue
			}
			// Use a small tolerance for float comparison
			const epsilon = 1e-9
			if diff := realObj.Value - expected; diff < -epsilon || diff > epsilon {
				t.Errorf("input: %q, wrong real value. want=%v, got=%v", tt.input, expected, realObj.Value)
			}
		case string: // For error messages
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Errorf("input: %q, object is not Error. got=%T (%+v)", tt.input, evaluated, evaluated)
				continue
			}
			if errObj.Message != expected {
				t.Errorf("input: %q, wrong error message. expected=%q, got=%q", tt.input, expected, errObj.Message)
			}
		default:
			t.Errorf("input: %q, unhandled expected type: %T", tt.input, tt.expected)
		}
	}
}

func TestBuiltinMulDiv(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, time.Duration, or string for error
	}{
		// MUL operations
		{"MUL(10, 20);", int64(200)},
		{"MUL(1.5, 2.0);", 3.0},
		{"MUL(10, 2.5);", 25.0}, // INT * REAL promotion
		{"MUL(1.5, 10);", 15.0}, // REAL * INT promotion
		{"MUL(T#10s, 2);", 20 * time.Second},
		{"MUL(2, T#10s);", 20 * time.Second},
		{"MUL(T#1m, 1.5);", 90 * time.Second},

		// DIV operations
		{"DIV(20, 10);", int64(2)},
		{"DIV(5.0, 2.0);", 2.5},
		{"DIV(10, 2.5);", 4.0},
		{"DIV(5.0, 2);", 2.5},
		{"DIV(T#20s, 2);", 10 * time.Second},
		{"DIV(T#1m, 2.5);", 24 * time.Second},

		// Error cases
		{"MUL(10, TRUE);", "unsupported argument types for MUL: INTEGER * BOOLEAN"},
		{"DIV(10, TRUE);", "unsupported argument types for DIV: INTEGER / BOOLEAN"},
		{"MUL(T#1s, T#2s);", "unsupported argument types for MUL: TIME * TIME"},
		{"DIV(10, 0);", "division by zero"},
		{"DIV(10.0, 0);", "division by zero"},
		{"DIV(T#10s, 0);", "division by zero"},
		{"MUL(T#1s);", "wrong number of arguments for MUL. got=1, want=2"},
		{"DIV(T#1s);", "wrong number of arguments for DIV. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case time.Duration:
				timeObj, ok := evaluated.(*object.Time)
				if !ok {
					t.Errorf("object is not Time. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if timeObj.Value != expected {
					t.Errorf("wrong time duration value. want=%v, got=%v", expected, timeObj.Value)
				}
			case string: // Error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			}
		})
	}
}

func TestBuiltinModExpt(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be int64, float64, or string for error
	}{
		// MOD operations
		{"MOD(10, 3);", int64(1)},
		{"MOD(10, 2);", int64(0)},
		{"MOD(-10, 3);", int64(-1)},
		{"MOD(10, -3);", int64(1)},
		{"MOD(10, 0);", "division by zero in MOD"},
		{"MOD(10.5, 2);", "arguments to `MOD` must be INTEGER, got REAL and INTEGER"},
		{"MOD(10, 2.5);", "arguments to `MOD` must be INTEGER, got INTEGER and REAL"},
		{"MOD(10);", "wrong number of arguments for MOD. got=1, want=2"},

		// EXPT operations
		{"EXPT(2, 3);", 8.0},
		{"EXPT(2.0, 3.0);", 8.0},
		{"EXPT(4, 0.5);", 2.0},
		{"EXPT(10, -1);", 0.1},
		{"EXPT(-2, 3);", -8.0},
		{"EXPT(9, 0.5);", 3.0},
		{"EXPT(2, 3.5);", math.Pow(2, 3.5)},
		{"EXPT(TRUE, 2);", "argument 1 to `EXPT` must be numeric, got BOOLEAN"},
		{"EXPT(2, TRUE);", "argument 2 to `EXPT` must be numeric, got BOOLEAN"},
		{"EXPT(2);", "wrong number of arguments for EXPT. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case string: // Error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
					return
				}
				if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			}
		})
	}
}

func TestBuiltinComparisonFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// GT (Greater Than)
		{"GT(20, 10);", true},
		{"GT(10, 20);", false},
		{"GT(10, 10);", false},
		{"GT(20.5, 10);", true},
		{"GT('b', 'a');", true},

		// GE (Greater Than or Equal)
		{"GE(20, 10);", true},
		{"GE(10, 20);", false},
		{"GE(10, 10);", true},
		{"GE(10.0, 10);", true},
		{"GE('b', 'a');", true},
		{"GE('a', 'a');", true},

		// EQ (Equal)
		{"EQ(10, 10);", true},
		{"EQ(10, 20);", false},
		{"EQ(10.0, 10);", true},
		{"EQ('a', 'a');", true},
		{"EQ('a', 'b');", false},
		{"EQ(TRUE, TRUE);", true},
		{"EQ(FALSE, FALSE);", true},
		{"EQ(TRUE, FALSE);", false},
		{"EQ(T#1s, T#1s);", true},
		{"EQ(T#1s, T#2s);", false},
		{"EQ(D#2026-01-01, D#2026-01-01);", true},
		{"EQ(D#2026-01-01, D#2026-01-02);", false},
		{"EQ(TOD#10:00:00, TOD#10:00:00);", true},
		{"EQ(TOD#10:00:00, TOD#10:00:01);", false},
		{"EQ(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", true},
		{"EQ(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", false},

		// LE (Less Than or Equal)
		{"LE(10, 20);", true},
		{"LE(20, 10);", false},
		{"LE(10, 10);", true},
		{"LE(10, 10.0);", true},
		{"LE(10.0, 10.0);", true},
		{"LE('a', 'b');", true},
		{"LE('a', 'a');", true},
		{"LE(T#1s, T#1s);", true},
		{"LE(T#1s, T#2s);", true},
		{"LE(D#2026-01-01, D#2026-01-01);", true},
		{"LE(D#2026-01-01, D#2026-01-02);", true},
		{"LE(TOD#10:00:00, TOD#10:00:00);", true},
		{"LE(TOD#10:00:00, TOD#10:00:01);", true},
		{"LE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", true},
		{"LE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},

		// LT (Less Than)
		{"LT(10, 20);", true},
		{"LT(20, 10);", false},
		{"LT(10, 10);", false},
		{"LT(10, 20.5);", true},
		{"LT(10.0, 20.5);", true},
		{"LT('a', 'b');", true},
		{"LT(T#1s, T#2s);", true},
		{"LT(T#2s, T#1s);", false},
		{"LT(D#2026-01-01, D#2026-01-02);", true},
		{"LT(D#2026-01-02, D#2026-01-01);", false},
		{"LT(TOD#10:00:00, TOD#10:00:01);", true},
		{"LT(TOD#10:00:01, TOD#10:00:00);", false},
		{"LT(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},
		{"LT(DT#2026-01-01-10:00:01, DT#2026-01-01-10:00:00);", false},

		// NE (Not Equal)
		{"NE(10, 20);", true},
		{"NE(10, 10);", false},
		{"NE(10.0, 10);", false},
		{"NE('a', 'b');", true},
		{"NE('a', 'a');", false},
		{"NE(TRUE, FALSE);", true},
		{"NE(TRUE, TRUE);", false},
		{"NE(T#1s, T#2s);", true},
		{"NE(T#1s, T#1s);", false},
		{"NE(D#2026-01-01, D#2026-01-01);", false},
		{"NE(D#2026-01-01, D#2026-01-02);", true},
		{"NE(TOD#10:00:00, TOD#10:00:00);", false},
		{"NE(TOD#10:00:00, TOD#10:00:01);", true},
		{"NE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:00);", false},
		{"NE(DT#2026-01-01-10:00:00, DT#2026-01-01-10:00:01);", true},

		// Error cases
		{"GT(10, 'a');", "BUILTIN ERROR: type mismatch for comparison: INTEGER GT STRING"},
		{"LT(TRUE, 1);", "BUILTIN ERROR: type mismatch for comparison: BOOLEAN LT INTEGER"},
		{"GT(T#1s, 1);", "BUILTIN ERROR: unsupported operand types for GT: TIME"},
		{"GT(10);", "BUILTIN ERROR: wrong number of arguments for GT. got=1, want=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				testErrorObject(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinSQRTAndROUND(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// SQRT
		{"SQRT(9);", 3.0},
		{"SQRT(2.25);", 1.5},
		{"SQRT(0);", 0.0},
		{"SQRT(-1);", "argument to `SQRT` must be non-negative, got -1"},
		{"SQRT(-2.25);", "argument to `SQRT` must be non-negative, got -2.250000"},
		{"SQRT(TRUE);", "argument to `SQRT` not supported, got BOOLEAN"},
		{"SQRT();", "wrong number of arguments for SQRT. got=0, want=1"},
		{"SQRT(1, 2);", "wrong number of arguments for SQRT. got=2, want=1"},

		// ROUND
		{"ROUND(2.4);", int64(2)},
		{"ROUND(2.5);", int64(2)}, // round half to even
		{"ROUND(2.6);", int64(3)},
		{"ROUND(3.5);", int64(4)}, // round half to even
		{"ROUND(-2.4);", int64(-2)},
		{"ROUND(-2.5);", int64(-2)}, // round half to even
		{"ROUND(-2.6);", int64(-3)},
		{"ROUND(0.0);", int64(0)},
		{"ROUND(5);", "argument to `ROUND` must be REAL, got INTEGER"},
		{"ROUND(TRUE);", "argument to `ROUND` must be REAL, got BOOLEAN"},
		{"ROUND();", "wrong number of arguments for ROUND. got=0, want=1"},
		{"ROUND(1, 2);", "wrong number of arguments for ROUND. got=2, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case int64:
			testIntegerObject(t, evaluated, expected)
		case float64:
			testRealObject(t, evaluated, expected)
		case string:
			testErrorObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinTypeConversionFunctions(t *testing.T) { // Replaced with more detailed tests
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// --- To Integer Types ---
		{"REAL_TO_INT(5.4);", int64(5)},
		{"REAL_TO_INT(5.5);", int64(6)}, // Round half to even (2.5->2, 3.5->4)
		{"REAL_TO_INT(5.6);", int64(6)},
		{"REAL_TO_INT(6.5);", int64(6)},
		{"INT_TO_DINT(123);", int64(123)},
		{"STRING_TO_INT('42');", int64(42)},
		{"STRING_TO_INT('abc');", "BUILTIN ERROR: could not parse string to integer: abc"},

		// --- To Real Types ---
		{"INT_TO_REAL(10);", 10.0},
		{"STRING_TO_REAL('123.45');", 123.45},
		{"STRING_TO_REAL('xyz');", "BUILTIN ERROR: could not parse string to real: xyz"},

		// --- To String Type ---
		{"INT_TO_STRING(123);", "123"},
		{"REAL_TO_STRING(1.23);", "1.230000"},
		{"BOOL_TO_STRING(TRUE);", "TRUE"},

		// --- To BCD ---
		{"INT_TO_BCD(1234)", uint64(0x1234)},
		{"INT_TO_BCD(10000)", "BUILTIN ERROR: value 10000 out of range for 4-digit BCD conversion (0-9999)"},
		{"BCD_TO_INT(WORD#16#1234)", int64(1234)},
		{"BCD_TO_INT(WORD#16#1A2B)", "BUILTIN ERROR: invalid BCD format: nibble 2 has value 10 > 9"},

		// --- Overflow/Underflow Checks for Integer Types ---
		// SINT (-128 to 127)
		{"INT_TO_SINT(127);", int64(127)},
		{"INT_TO_SINT(128);", "BUILTIN ERROR: value 128 is out of range for type SINT (-128 to 127)"},
		{"INT_TO_SINT(-128);", int64(-128)},
		{"INT_TO_SINT(-129);", "BUILTIN ERROR: value -129 is out of range for type SINT (-128 to 127)"},

		// USINT (0 to 255)
		{"INT_TO_USINT(255);", int64(255)},
		{"INT_TO_USINT(256);", "BUILTIN ERROR: value 256 is out of range for type USINT (0 to 255)"},
		{"INT_TO_USINT(-1);", "BUILTIN ERROR: value -1 is out of range for type USINT (0 to 255)"},

		// INT (-32768 to 32767)
		{"DINT_TO_INT(32767);", int64(32767)},
		{"DINT_TO_INT(32768);", "BUILTIN ERROR: value 32768 is out of range for type INT (-32768 to 32767)"},

		// UINT (0 to 65535)
		{"DINT_TO_UINT(65535);", int64(65535)},
		{"DINT_TO_UINT(65536);", "BUILTIN ERROR: value 65536 is out of range for type UINT (0 to 65535)"},

		// --- Overflow Checks for Bit-String Types ---
		// BYTE (0 to 255)
		{"INT_TO_BYTE(255);", uint64(255)},
		{"INT_TO_BYTE(256);", "BUILTIN ERROR: value 256 is out of range for type BYTE (0 to 255)"},
		{"INT_TO_BYTE(-1);", "BUILTIN ERROR: value -1 is out of range for type BYTE (0 to 255)"},

		// WORD (0 to 65535)
		{"INT_TO_WORD(65535);", uint64(65535)},
		{"INT_TO_WORD(65536);", "BUILTIN ERROR: value 65536 is out of range for type WORD (0 to 65535)"},

		// --- General Error Cases ---
		{"INT_TO_BOOL(1);", "BUILTIN ERROR: conversion to type BOOL is not supported"},
		{"REAL_TO_TIME(1.0);", "BUILTIN ERROR: conversion to type TIME is not supported"},
		{"INT_TO_REAL();", "BUILTIN ERROR: wrong number of arguments for INT_TO_REAL. got=0, want=1"},
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
					t.Errorf("wrong value. want=0x%X, got=0x%X", expected, bs.Value)
				}
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				realObj, ok := evaluated.(*object.Real)
				if !ok {
					t.Errorf("input: %q, object is not Real. got=%T (%+v)", tt.input, evaluated, evaluated)
				}
				const epsilon = 1e-9
				if diff := realObj.Value - expected; diff < -epsilon || diff > epsilon {
					t.Errorf("input: %q, wrong real value. want=%v, got=%v", tt.input, expected, realObj.Value)
				}
			case string: // For error messages
				errObj, ok := evaluated.(*object.Error)
				if !ok {
					// Not an error, so it must be a string result
					testStringObject(t, evaluated, expected)
				} else if !strings.Contains(errObj.Message, expected) {
					t.Errorf("wrong error message. expected to contain %q, got %q", expected, errObj.Message)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinAbsFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer tests
		{"ABS(10);", int64(10)},
		{"ABS(-10);", int64(10)},
		{"ABS(0);", int64(0)},

		// Real tests
		{"ABS(10.5);", 10.5},
		{"ABS(-10.5);", 10.5},

		// Error cases
		{"ABS(TRUE);", "argument to `ABS` not supported, got BOOLEAN"},
		{"ABS();", "wrong number of arguments for ABS. got=0, want=1"},
		{"ABS(1, 2);", "wrong number of arguments for ABS. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case string:
				testErrorObject(t, evaluated, expected)
			}
		})
	}
}

func TestBuiltinTruncFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Positive and negative values
		{"TRUNC(5.7);", int64(5)},
		{"TRUNC(-5.7);", int64(-5)},
		{"TRUNC(5.2);", int64(5)},
		{"TRUNC(-5.2);", int64(-5)},
		{"TRUNC(0.0);", int64(0)},
		{"TRUNC(5.0);", int64(5)},

		// Error cases
		{"TRUNC(5);", "argument to `TRUNC` must be REAL, got INTEGER"},
		{"TRUNC(TRUE);", "argument to `TRUNC` must be REAL, got BOOLEAN"},
		{"TRUNC();", "wrong number of arguments for TRUNC. got=0, want=1"},
		{"TRUNC(1.0, 2.0);", "wrong number of arguments for TRUNC. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case string:
				testErrorObject(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinMoveFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Test with various literal types
		{"MOVE(5);", int64(5)},
		{"MOVE(10.5);", 10.5},
		{"MOVE(TRUE);", true},
		{`MOVE("hello");`, "hello"},
		{"MOVE(T#5s);", 5 * time.Second},
		{"MOVE(BYTE#16#FF);", uint64(0xFF)},

		// Test with an expression
		{"MOVE(5 + 5);", int64(10)},

		// Error cases for wrong number of arguments
		{"MOVE();", "BUILTIN ERROR: wrong number of arguments for MOVE. got=0, want=1"},
		{"MOVE(1, 2);", "BUILTIN ERROR: wrong number of arguments for MOVE. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				// Can be a string result or an error message
				if _, ok := evaluated.(*object.Error); ok {
					testErrorObject(t, evaluated, expected)
				} else {
					testStringObject(t, evaluated, expected)
				}
			case time.Duration:
				testTimeObject(t, evaluated, expected)
			case uint64:
				if bs, ok := evaluated.(*object.BitString); !ok || bs.Value != expected {
					t.Errorf("object is not correct BitString. want=%d, got=%v", expected, evaluated)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinMinMaxFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// MIN function tests
		{"MIN(10, 20);", int64(10)},
		{"MIN(20, 10);", int64(10)},
		{"MIN(10, 20, 5, 30);", int64(5)},
		{"MIN(-10, -20);", int64(-20)},
		{"MIN(10.5, 10.6);", 10.5},
		{"MIN(10, 20.5);", 10.0}, // Type promotion to REAL
		{"MIN(1, 2.5, -3.0, 4);", -3.0},
		{"MIN(10);", int64(10)},
		{"MIN();", "BUILTIN ERROR: wrong number of arguments for MIN. got=0, want>=1"},
		{"MIN(1, TRUE);", "BUILTIN ERROR: all arguments to `MIN` must be INTEGER or REAL, got BOOLEAN"},

		// MAX function tests
		{"MAX(10, 20);", int64(20)},
		{"MAX(20, 10);", int64(20)},
		{"MAX(10, 20, 5, 30);", int64(30)},
		{"MAX(-10, -20);", int64(-10)},
		{"MAX(10.5, 10.6);", 10.6},
		{"MAX(10, 20.5);", 20.5}, // Type promotion to REAL
		{"MAX(1, 2.5, -3.0, 4);", 4.0},
		{"MAX(10);", int64(10)},
		{"MAX();", "BUILTIN ERROR: wrong number of arguments for MAX. got=0, want>=1"},
		{"MAX(1, TRUE);", "BUILTIN ERROR: all arguments to `MAX` must be INTEGER or REAL, got BOOLEAN"},
	}

	// Register MIN and MAX in builtins for testing if they aren't already
	if _, ok := builtins["MIN"]; !ok { // cspell:disable-line
		builtins["MIN"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
			return minMaxBuiltin("MIN", args...)
		}}
	}
	if _, ok := builtins["MAX"]; !ok {
		builtins["MAX"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
			return minMaxBuiltin("MAX", args...)
		}}
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case string:
				// Can be a string result or an error message
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitwiseFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// AND function
		{"AND(BYTE#16#F0, BYTE#16#A5);", uint64(0xA0)},
		{"AND(WORD#16#1234, WORD#16#00FF, WORD#16#FFFF);", uint64(0x0034)},
		{"AND(BYTE#16#FF, BYTE#16#FF);", uint64(0xFF)},

		// OR function
		{"OR(BYTE#16#A0, BYTE#16#05);", uint64(0xA5)},
		{"OR(WORD#16#1200, WORD#16#0034);", uint64(0x1234)},
		{"OR(BYTE#16#F0, BYTE#16#0F, BYTE#16#AA);", uint64(0xFF)},

		// XOR function
		{"XOR(BYTE#16#A5, BYTE#16#F0);", uint64(0x55)},
		{"XOR(WORD#16#FFFF, WORD#16#1234);", uint64(0xEDCB)},
		{"XOR(BYTE#16#FF, BYTE#16#FF, BYTE#16#FF);", uint64(0xFF)}, // A ^ A ^ A = A

		// Error cases
		{"AND(BYTE#16#FF);", "BUILTIN ERROR: wrong number of arguments for AND. got=1, want>=2"},
		{"OR(BYTE#16#FF, WORD#16#FF);", "BUILTIN ERROR: all arguments to `OR` must have the same width, got 8 and 16"},
		{"XOR(BYTE#16#FF, 10);", "BUILTIN ERROR: all arguments to `XOR` must be bit-string types, got INTEGER"},

		// NAND function
		{"NAND(BYTE#16#F0, BYTE#16#A5);", uint64(0x5F)},             // NOT(A0) -> 5F
		{"NAND(WORD#16#1234, WORD#16#00FF);", uint64(0xFFCB)},       // NOT(0034) -> FFCB
		{"NAND(BYTE#16#FF, BYTE#16#FF, BYTE#16#FF);", uint64(0x00)}, // NOT(FF) -> 00

		// NOR function
		{"NOR(BYTE#16#A0, BYTE#16#05);", uint64(0x5A)},       // NOT(A5) -> 5A
		{"NOR(WORD#16#1200, WORD#16#0034);", uint64(0xEDCB)}, // NOT(1234) -> EDCB
		{"NOR(BYTE#16#00, BYTE#16#00);", uint64(0xFF)},       // NOT(00) -> FF
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

func TestBuiltinTrigFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// SIN
		{"SIN(0);", 0.0},
		{"SIN(PI / 2);", 1.0}, // Assuming PI is a variable in the environment
		{"SIN(PI);", math.Sin(math.Pi)},
		{"SIN(1.570796);", math.Sin(1.570796)},
		{"SIN(TRUE);", "argument to `SIN` must be INTEGER or REAL, got BOOLEAN"},
		{"SIN();", "wrong number of arguments for SIN. got=0, want=1"},

		// COS
		{"COS(0);", 1.0},
		{"COS(PI / 2);", math.Cos(math.Pi / 2)},
		{"COS(PI);", -1.0},
		{"COS(TRUE);", "argument to `COS` must be INTEGER or REAL, got BOOLEAN"},

		// TAN
		{"TAN(0);", 0.0},
		{"TAN(PI / 4);", 1.0},
		{"TAN(TRUE);", "argument to `TAN` must be INTEGER or REAL, got BOOLEAN"},
	}

	for _, tt := range tests {
		evaluated := testEvalWithPi(t, tt.input) // Use a helper that defines PI
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, expected)
		case string:
			testErrorObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinInverseTrigFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// ASIN
		{"ASIN(0);", 0.0},
		{"ASIN(1);", math.Pi / 2},
		{"ASIN(-1);", -math.Pi / 2},
		{"ASIN(0.5);", math.Asin(0.5)},
		{"ASIN(2);", "argument to `ASIN` must be between -1 and 1, got 2.000000"},
		{"ASIN(-2.0);", "argument to `ASIN` must be between -1 and 1, got -2.000000"},
		{"ASIN(TRUE);", "argument to `ASIN` must be INTEGER or REAL, got BOOLEAN"},
		{"ASIN();", "wrong number of arguments for ASIN. got=0, want=1"},

		// ACOS
		{"ACOS(1);", 0.0},
		{"ACOS(-1);", math.Pi},
		{"ACOS(0);", math.Pi / 2},
		{"ACOS(0.5);", math.Acos(0.5)},
		{"ACOS(2);", "argument to `ACOS` must be between -1 and 1, got 2.000000"},
		{"ACOS(TRUE);", "argument to `ACOS` must be INTEGER or REAL, got BOOLEAN"},

		// ATAN
		{"ATAN(0);", 0.0},
		{"ATAN(1);", math.Pi / 4},
		{"ATAN(-1);", -math.Pi / 4},
		{"ATAN(100);", math.Atan(100)},
		{"ATAN(TRUE);", "argument to `ATAN` must be INTEGER or REAL, got BOOLEAN"},

		// ATAN2
		{"ATAN2(1, 1);", math.Pi / 4},        // Quadrant 1
		{"ATAN2(1, -1);", 3 * math.Pi / 4},   // Quadrant 2
		{"ATAN2(-1, -1);", -3 * math.Pi / 4}, // Quadrant 3
		{"ATAN2(-1, 1);", -math.Pi / 4},      // Quadrant 4
		{"ATAN2(1.0, 0.0);", math.Pi / 2},
		{"ATAN2(1, 0);", math.Pi / 2},
		{"ATAN2(TRUE, 1);", "argument 1 to `ATAN2` must be INTEGER or REAL, got BOOLEAN"},
		{"ATAN2(1, TRUE);", "argument 2 to `ATAN2` must be INTEGER or REAL, got BOOLEAN"},
		{"ATAN(TRUE);", "argument to `ATAN` must be INTEGER or REAL, got BOOLEAN"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, expected)
		case string:
			testErrorObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinLogFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// LN (Natural Log)
		{"LN(E);", 1.0}, // Assuming E is a variable in the environment
		{"LN(1);", 0.0},
		{"LN(10);", math.Log(10)},
		{"LN(0);", "argument to `LN` must be positive, got 0.000000"},
		{"LN(-1);", "argument to `LN` must be positive, got -1.000000"},
		{"LN(TRUE);", "argument to `LN` must be INTEGER or REAL, got BOOLEAN"},
		{"LN();", "wrong number of arguments for LN. got=0, want=1"},

		// LOG (Base-10 Log)
		{"LOG(10);", 1.0},
		{"LOG(100);", 2.0},
		{"LOG(1);", 0.0},
		{"LOG(0.1);", -1.0},
		{"LOG(0);", "argument to `LOG` must be positive, got 0.000000"},
		{"LOG(-10);", "argument to `LOG` must be positive, got -10.000000"},
		{"LOG(TRUE);", "argument to `LOG` must be INTEGER or REAL, got BOOLEAN"},
		{"LOG();", "wrong number of arguments for LOG. got=0, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEvalWithBuiltinVars(t, tt.input) // Use a helper that defines E
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, expected)
		case string:
			testErrorObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinExpFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // float64 or string for error
	}{
		// EXP
		{"EXP(1);", math.E},
		{"EXP(0);", 1.0},
		{"EXP(2);", math.Exp(2)},
		{"EXP(-1);", math.Exp(-1)},
		{"EXP(2.5);", math.Exp(2.5)},
		{"EXP(TRUE);", "argument to `EXP` must be INTEGER or REAL, got BOOLEAN"},
		{"EXP();", "wrong number of arguments for EXP. got=0, want=1"},
		{"EXP(1, 2);", "wrong number of arguments for EXP. got=2, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case float64:
			testRealObject(t, evaluated, expected)
		case string:
			testErrorObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinStringFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // string or error message
	}{
		// LEN
		{`LEN("");`, int64(0)},
		{`LEN("four");`, int64(4)},
		{`LEN("hello world");`, int64(11)},
		{`LEN(1);`, "argument to `len` not supported, got INTEGER"},

		// LEFT
		{`LEFT("abcdef", 2);`, "ab"},
		{`LEFT("abc", 5);`, "abc"},
		{`LEFT("abc", 3);`, "abc"},
		{`LEFT("abc", 0);`, ""},
		{`LEFT("abc", -1);`, ""},
		{`LEFT(123, 1);`, "argument 1 to `LEFT` must be STRING, got INTEGER"},
		{`LEFT("abc", "a");`, "argument 2 to `LEFT` must be INTEGER, got STRING"},
		{`LEFT("abc");`, "wrong number of arguments for LEFT. got=1, want=2"},

		// RIGHT
		{`RIGHT("abcdef", 2);`, "ef"},
		{`RIGHT("abc", 5);`, "abc"},
		{`RIGHT("abc", 3);`, "abc"},
		{`RIGHT("abc", 0);`, ""},
		{`RIGHT("abc", -1);`, ""},
		{`RIGHT(123, 1);`, "argument 1 to `RIGHT` must be STRING, got INTEGER"},
		{`RIGHT("abc", "a");`, "argument 2 to `RIGHT` must be INTEGER, got STRING"},
		{`RIGHT("abc");`, "wrong number of arguments for RIGHT. got=1, want=2"},

		// MID
		{`MID("abcdef", 2, 3);`, "cd"},    // MID(IN, L, P) -> MID("abcdef", Length=2, Pos=3)
		{`MID("abcdef", 3, 4);`, "def"},   // MID("abcdef", L=3, P=4)
		{`MID("abcdef", 5, 4);`, "def"},   // Length goes past end of string
		{`MID("abcdef", 1, 2);`, "b"},     // MID("abcdef", L=1, P=2)
		{`MID("abcdef", 1, 10);`, ""},     // Position out of bounds
		{`MID("abcdef", 7, 2);`, "bcdef"}, // Length goes past end of string
		{`MID("abcdef", 2, 0);`, ""},      // Position out of bounds (<=0)
		{`MID("abcdef", 0, 2);`, ""},      // Length is 0
		{`MID("abcdef", -1, 2);`, ""},     // Length is negative
		{`MID(123, 1, 1);`, "argument 1 to `MID` must be STRING, got INTEGER"},
		{`MID("abc", "a", 1);`, "argument 2 to `MID` must be INTEGER, got STRING"},
		{`MID("abc", 1, "a");`, "argument 3 to `MID` must be INTEGER, got STRING"},
		{`MID("abc", 1);`, "wrong number of arguments for MID. got=2, want=3"},

		// FIND
		{`FIND("abcdef", "cd");`, int64(3)},
		{`FIND("abcdef", "a");`, int64(1)},
		{`FIND("abcdef", "f");`, int64(6)},
		{`FIND("abcdef", "xyz");`, int64(0)},
		{`FIND("abcdef", "");`, int64(1)},
		{`FIND("", "a");`, int64(0)},
		{`FIND("abc", 1);`, "argument 2 to `FIND` must be STRING, got INTEGER"},
		{`FIND("abc");`, "wrong number of arguments for FIND. got=2, want=2"},

		// REPLACE
		{`REPLACE("abcdef", "XX", 2, 3);`, "abXXef"},   // Standard replace
		{`REPLACE("abcdef", "XX", 0, 3);`, "abXXcdef"}, // Insert (L=0)
		{`REPLACE("abcdef", "", 2, 3);`, "abef"},       // Delete (IN2 is empty)
		{`REPLACE("abc", "XX", 4, 2);`, "aXX"},         // L is larger than remaining string
		{`REPLACE("abc", "XX", 2, 1);`, "XXc"},         // Replace at start
		{`REPLACE("abc", "XX", 2, 4);`, "abcXX"},       // P is out of bounds (append)
		{`REPLACE("abc", "XX", 2, 0);`, "XXc"},         // P < 1, treated as P=1
		{`REPLACE("abc", "XX", -1, 2);`, "aXXbc"},      // L < 0, treated as L=0 (insert)
		{`REPLACE(1, "a", 1, 1);`, "argument 1 to `REPLACE` must be STRING, got INTEGER"},
		{`REPLACE("a", 1, 1, 1);`, "argument 2 to `REPLACE` must be STRING, got INTEGER"},
		{`REPLACE("a", "b", "c", 1);`, "argument 3 to `REPLACE` must be INTEGER, got STRING"},
		{`REPLACE("a", "b", 1, "d");`, "argument 4 to `REPLACE` must be INTEGER, got STRING"},
		{`REPLACE("a", "b", 1);`, "wrong number of arguments for REPLACE. got=3, want=4"},

		// CONCAT (for strings)
		{`CONCAT("a", "b", "c");`, "abc"},
		{`CONCAT("Hello, ", "World!");`, "Hello, World!"},
		{`CONCAT("one");`, "one"},
		{`CONCAT("", "a");`, "a"},
		{`CONCAT("a", "");`, "a"},
		{`CONCAT("", "");`, ""},
		{`CONCAT("a", 1);`, "all arguments to `CONCAT` must be of the same type (STRING), got INTEGER"},

		// DELETE (for strings)
		{`DELETE("abcdef", 2, 3);`, "abef"},
		{`DELETE("abcdef", 10, 2);`, "a"},
		{`DELETE("abcdef", 2, 1);`, "cdef"},
		{`DELETE("abcdef", 2, 6);`, "abcde"},
		{`DELETE("abc", 1, 4);`, "abc"}, // Position out of bounds
		{`DELETE("abc", 0, 1);`, "abc"}, // Length is 0
		{`DELETE(123, 1, 1);`, "argument 1 to `DELETE` must be ARRAY or STRING, got INTEGER"},
		{`DELETE("abc", "a", 1);`, "argument 2 to `DELETE` must be INTEGER, got STRING"},

		// INSERT (for strings)
		{`INSERT("abcdef", "XX", 3);`, "abXXcdef"},
		{`INSERT("abc", "XX", 1);`, "XXabc"},
		{`INSERT("abc", "XX", 4);`, "abcXX"},
		{`INSERT("abc", "XX", 5);`, "abcXX"}, // Position > length, appends
		{`INSERT("abc", "XX", 0);`, "XXabc"}, // Position < 1, prepends
		{`INSERT(123, "a", 1);`, "argument 1 to `INSERT` must be ARRAY or STRING, got INTEGER"},
		{`DELETE("abc", "a", 1);`, "argument 2 to `DELETE` must be INTEGER, got STRING"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		switch expected := tt.expected.(type) {
		case string:
			if expected == "" {
				// Could be an empty string object or an error message for an empty string
				if _, ok := evaluated.(*object.Error); ok {
					testErrorObject(t, evaluated, expected)
				} else {
					strObj, ok := evaluated.(*object.String)
					if !ok {
						t.Errorf("input: %q, object is not String. got=%T (%+v)", tt.input, evaluated, evaluated)
						continue
					}
					if strObj.Value != expected {
						t.Errorf("input: %q, wrong string value. want=%q, got=%q", tt.input, expected, strObj.Value)
					}
				}
			} else {
				testStringObject(t, evaluated, expected)
			}
		case int64:
			testIntegerObject(t, evaluated, expected)
		}
	}
}

func TestBuiltinMinMax(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // int64, float64, or string for error
	}{
		// MIN function
		{"MIN(10, 20)", int64(10)},
		{"MIN(20, 10)", int64(10)},
		{"MIN(10, 20, 5, 30)", int64(5)},
		{"MIN(-10, -20)", int64(-20)},
		{"MIN(10)", int64(10)},
		{"MIN(10.5, 10.6)", 10.5},
		{"MIN(10, 20.5)", 10.0},
		{"MIN(10.5, 20)", 10.5},
		{"MIN(1, 2.5, -3.0, 4)", -3.0},
		{"MIN()", "wrong number of arguments for MIN. got=0, want>=1"},
		{"MIN(1, TRUE)", "all arguments to `MIN` must be INTEGER or REAL, got BOOLEAN"},

		// MAX function
		{"MAX(10, 20)", int64(20)},
		{"MAX(20, 10)", int64(20)},
		{"MAX(10, 20, 5, 30)", int64(30)},
		{"MAX(-10, -20)", int64(-10)},
		{"MAX(10)", int64(10)},
		{"MAX(10.5, 10.6)", 10.6},
		{"MAX(10, 20.5)", 20.5},
		{"MAX(10.5, 20)", 20.0},
		{"MAX(1, 2.5, -3.0, 4)", 4.0},
		{"MAX()", "wrong number of arguments for MAX. got=0, want>=1"},
		{"MAX(1, TRUE)", "all arguments to `MAX` must be INTEGER or REAL, got BOOLEAN"},
	}

	// Register MIN and MAX in builtins for testing
	builtins["MIN"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		return minMaxBuiltin("MIN", args...)
	}}
	builtins["MAX"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		return minMaxBuiltin("MAX", args...)
	}}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case string:
				testErrorObject(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitShiftFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be uint64 or string for error
	}{
		// SHL (Shift Left)
		{"SHL(BYTE#2#1010_0101, 1)", uint64(0x4A)}, // 0xA5 << 1 = 0x14A, masked to 8 bits is 0x4A
		{"SHL(BYTE#16#A5, 1)", uint64(0x4A)},       // Same as above
		{"SHL(WORD#16#FF00, 8)", uint64(0x0000)},
		{"SHL(WORD#16#00FF, 8)", uint64(0xFF00)},
		{"SHL(DWORD#16#1, 31)", uint64(1 << 31)},
		{"SHL(DWORD#16#1, 32)", uint64(0)}, // Shifted out
		{"SHL(10, 2)", "argument 1 to `SHL` must be a bitstring type, got INTEGER"},
		{"SHL(BYTE#16#10, -1)", "shift amount for `SHL` must be non-negative, got -1"},
		{"SHL(BYTE#16#10)", "wrong number of arguments for SHL. got=1, want=2"},

		// SHR (Shift Right)
		{"SHR(BYTE#2#1010_0101, 1)", uint64(0x52)}, // 0xA5 >> 1 = 0x52 (82)
		{"SHR(BYTE#16#A5, 1)", uint64(0x52)},       // Same as above
		{"SHR(WORD#16#FF00, 8)", uint64(0x00FF)},
		{"SHR(WORD#16#00FF, 8)", uint64(0x0000)},
		{"SHR(DWORD#16#80000000, 31)", uint64(1)},
		{"SHR(DWORD#16#FFFFFFFF, 32)", uint64(0)}, // Shifted out
		{"SHR(BYTE#16#10, 2.5)", "argument 2 to `SHR` must be INTEGER, got REAL"},
		{"SHR(BYTE#16#10, -1)", "shift amount for `SHR` must be non-negative, got -1"},
		{"SHR()", "wrong number of arguments for SHR. got=0, want=2"},
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
				// Mask the result to the width of the bitstring to handle overflow cases in tests correctly
				mask := uint64(math.MaxUint64)
				if bs.Width < 64 {
					mask = (1 << bs.Width) - 1
				}
				if (bs.Value & mask) != expected {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
				}
			case string:
				if !testErrorObjectContains(t, evaluated, expected) {
					t.Errorf("error message did not contain expected text")
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinBitRotateFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{} // Can be uint64 or string for error
	}{
		// ROL (Rotate Left)
		{"ROL(BYTE#2#1010_0101, 1)", uint64(0x4B)}, // ROL(0xA5, 1) -> 0x4B
		{"ROL(BYTE#16#A5, 1)", uint64(0x4B)},
		{"ROL(WORD#16#FF00, 8)", uint64(0x00FF)},
		{"ROL(WORD#16#C0F0, 4)", uint64(0x0F0C)},
		{"ROL(DWORD#16#1, 31)", uint64(1 << 31)},
		{"ROL(DWORD#16#1, 32)", uint64(1)}, // Rotated full circle
		{"ROL(10, 2)", "argument 1 to `ROL` must be a bitstring type, got INTEGER"},
		{"ROL(BYTE#16#10, -1)", "rotate amount for `ROL` must be non-negative, got -1"},
		{"ROL(BYTE#16#10)", "wrong number of arguments for ROL. got=1, want=2"},

		// ROR (Rotate Right)
		{"ROR(BYTE#2#1010_0101, 1)", uint64(0xD2)}, // ROR(0xA5, 1) -> 0xD2
		{"ROR(BYTE#16#A5, 1)", uint64(0xD2)},
		{"ROR(WORD#16#FF00, 8)", uint64(0x00FF)},
		{"ROR(WORD#16#0F0C, 4)", uint64(0xC0F0)},
		{"ROR(DWORD#16#80000000, 31)", uint64(1)},
		{"ROR(DWORD#16#FFFFFFFF, 32)", uint64(0xFFFFFFFF)}, // Rotated full circle
		{"ROR(BYTE#16#10, 2.5)", "argument 2 to `ROR` must be INTEGER, got REAL"},
		{"ROR(BYTE#16#10, -1)", "rotate amount for `ROR` must be non-negative, got -1"},
		{"ROR()", "wrong number of arguments for ROR. got=0, want=2"},
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
				// Mask the result to the width of the bitstring to handle overflow cases in tests correctly
				mask := uint64(math.MaxUint64)
				if bs.Width < 64 {
					mask = (1 << bs.Width) - 1
				}
				if (bs.Value & mask) != (expected & mask) {
					t.Errorf("wrong value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value&mask, bs.Value&mask)
				}
			case string:
				if !testErrorObjectContains(t, evaluated, expected) {
					t.Errorf("error message did not contain expected text")
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestBuiltinArrayFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// INSERT
		{`INSERT([1, 2, 3], 99, 2);`, []int{1, 99, 2, 3}},
		{`INSERT([1, 2, 3], 99, 1);`, []int{99, 1, 2, 3}},
		{`INSERT([1, 2, 3], 99, 4);`, []int{1, 2, 3, 99}},
		{`INSERT([], 99, 1);`, []int{99}},
		{`INSERT([1], 99, 5);`, []int{1, 99}}, // Position > length, appends
		{`INSERT([1], 99, 0);`, []int{99, 1}}, // Position < 1, prepends
		{`INSERT(1, 2, 3);`, "argument 1 to `INSERT` must be ARRAY, got INTEGER"},
		{`INSERT([], 1, "a");`, "argument 3 to `INSERT` must be INTEGER, got STRING"},
		{`INSERT([], 1);`, "wrong number of arguments for INSERT. got=2, want=3"},

		// DELETE
		{`DELETE([1, 2, 3, 4], 2, 2);`, []int{1, 4}},
		{`DELETE([1, 2, 3], 1, 1);`, []int{2, 3}},
		{`DELETE([1, 2, 3], 3, 1);`, []int{}},
		{`DELETE([1, 2, 3], 5, 1);`, []int{}}, // Length > array size
		{`DELETE([1, 2, 3], 2, 3);`, []int{1, 2}},
		{`DELETE([1, 2, 3], 1, 4);`, []int{1, 2, 3}},  // Position out of bounds
		{`DELETE([1, 2, 3], 1, 0);`, []int{1, 2, 3}},  // Position out of bounds
		{`DELETE([1, 2, 3], 0, 1);`, []int{1, 2, 3}},  // Length is 0
		{`DELETE([1, 2, 3], -1, 1);`, []int{1, 2, 3}}, // Length is negative
		{`DELETE(1, 2, 3);`, "argument 1 to `DELETE` must be ARRAY, got INTEGER"},
		{`DELETE([], "a", 1);`, "argument 2 to `DELETE` must be INTEGER, got STRING"},
		{`DELETE([], 1, "a");`, "argument 3 to `DELETE` must be INTEGER, got STRING"},
		{`DELETE([], 1);`, "wrong number of arguments for DELETE. got=2, want=3"},

		// CONCAT
		{`CONCAT([1, 2], [3, 4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1], [2], [3], [4]);`, []int{1, 2, 3, 4}},
		{`CONCAT([1, 2]);`, []int{1, 2}},
		{`CONCAT([], [1]);`, []int{1}},
		{`CONCAT([1], []);`, []int{1}},
		{`CONCAT([], []);`, []int{}},
		{`CONCAT();`, "wrong number of arguments for CONCAT. got=0, want>=1"},
		{`CONCAT([1], 2);`, "all arguments to `CONCAT` must be ARRAY, got INTEGER"},

		// FIND (for arrays)
		{`FIND([1, 2, 3], 2);`, int64(2)},
		{`FIND([1, 2, 3], 4);`, int64(0)},
		{`FIND(["a", "b", "c"], "b");`, int64(2)},
		{`FIND(["a", "b", "c"], "d");`, int64(0)},
		{`FIND([], 1);`, int64(0)},
		{`FIND([1, 2, 3], "a");`, int64(0)}, // Type mismatch, not found
		{`FIND(1, 1);`, "BUILTIN ERROR: argument 1 to `FIND` must be STRING or ARRAY, got INTEGER"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case string:
				testErrorObject(t, evaluated, expected)
			case int64:
				testIntegerObject(t, evaluated, expected)
			case []int:
				arr, ok := evaluated.(*object.Array)
				if !ok {
					t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
				}
				if len(arr.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. want=%d, got=%d", len(expected), len(arr.Elements))
				}
				for i, expectedElem := range expected {
					testIntegerObject(t, arr.Elements[i], int64(expectedElem))
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestMuxFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"MUX(0, 100, 101, 102);", int64(100)},
		{"MUX(1, 100, 101, 102);", int64(101)},
		{"MUX(2, 100, 101, 102);", int64(102)},

		// Real selection
		{"MUX(1, 10.5, 20.5, 30.5);", 20.5},

		// String selection
		{`MUX(0, "a", "b", "c");`, "a"},
		{`MUX(2, "a", "b", "c");`, "c"},

		// Boolean selection
		{"MUX(1, FALSE, TRUE);", true},

		// Selection with expression as selector
		{"MUX(1+1, 10, 20, 30);", int64(30)},

		// Error cases
		{"MUX(3, 100, 101, 102);", "index 3 out of bounds for MUX with 3 inputs"},
		{"MUX(-1, 100, 101, 102);", "index -1 out of bounds for MUX"},
		{"MUX(0.5, 100, 101);", "argument 1 to `MUX` must be INTEGER, got REAL"},
		{`MUX(0, 100, "a");`, "all value arguments to `MUX` must be of the same type, got STRING but expected INTEGER"},
		{"MUX(0);", "wrong number of arguments for MUX. got=1, want>=2"},
		{"MUX();", "wrong number of arguments for MUX. got=0, want>=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}

func TestSelFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// Integer selection
		{"SEL(FALSE, 10, 20);", int64(10)},
		{"SEL(TRUE, 10, 20);", int64(20)},

		// Real selection
		{"SEL(FALSE, 10.5, 20.5);", 10.5},
		{"SEL(TRUE, 10.5, 20.5);", 20.5},

		// Boolean selection
		{"SEL(FALSE, TRUE, FALSE);", true},
		{"SEL(TRUE, TRUE, FALSE);", false},

		// String selection
		{`SEL(FALSE, "hello", "world");`, "hello"},
		{`SEL(TRUE, "hello", "world");`, "world"},

		// Selection with expressions
		{"SEL(1 > 0, 5+5, 10+10);", int64(20)},
		{"SEL(1 < 0, 5+5, 10+10);", int64(10)},

		// Error cases
		{"SEL(1, 10, 20);", "argument 1 to `SEL` must be BOOLEAN, got INTEGER"},
		{`SEL(TRUE, 10, "world");`, "arguments 2 and 3 to `SEL` must be of the same type, got INTEGER and STRING"},
		{"SEL(TRUE, 10);", "wrong number of arguments for SEL. got=2, want=3"},
		{"SEL(TRUE, 10, 20, 30);", "wrong number of arguments for SEL. got=4, want=3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
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
		{"NOT 123;", "ERROR (1:1): unknown operator: NOTLINT"},

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
		{"BYTE#16#A5 AND WORD#16#F0;", "ERROR (1:1): type mismatch: bitstring operands must have same width, got 8 and 16"},
		{"BYTE#16#A5 OR 10;", "ERROR (1:1): type mismatch: BITSTRING OR INTEGER"},
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

func TestBuiltinMove(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"MOVE(5);", int64(5)},
		{"MOVE(10.5);", 10.5},
		{"MOVE(TRUE);", true},
		{`MOVE("hello");`, "hello"},
		{"MOVE(T#5s);", 5 * time.Second},
		{"MOVE(BYTE#16#FF);", uint64(0xFF)},
		{"MOVE();", "BUILTIN ERROR: wrong number of arguments for MOVE. got=0, want=1"},
		{"MOVE(1, 2);", "BUILTIN ERROR: wrong number of arguments for MOVE. got=2, want=1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
			case string:
				if err, ok := evaluated.(*object.Error); ok {
					if !strings.Contains(err.Message, expected) {
						t.Errorf("wrong error message. expected to contain %q, got %q", expected, err.Message)
					}
				} else {
					testStringObject(t, evaluated, expected)
				}
			case time.Duration:
				// This is a simplified check for the test case.
			case uint64:
				// This is a simplified check for the test case.
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
	resultVar, ok := env.Get("ResultVar")
	if !ok {
		t.Fatalf("ResultVar not found in environment")
	}
	testIntegerObject(t, resultVar, 30)

	// 2. Check the value of the variable connected to the output parameter
	// Expected: NamedOut is `temp` * 2 = 30 * 2 = 60. This should be assigned to OutputVar.
	outputVar, ok := env.Get("OutputVar")
	if !ok {
		t.Fatalf("OutputVar not found in environment")
	}
	testIntegerObject(t, outputVar, 60)
}

func TestStandardFunctionBlocks(t *testing.T) {
	// Mock time for timer tests
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	// Helper to advance mock time
	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	t.Run("TON - Timer On-Delay", func(t *testing.T) {
		input := `
			PROGRAM TestTON
				VAR
					MyTimer : TON;
					Start : BOOL;
					TimerDone : BOOL;
					ET : TIME;
				END_VAR

				MyTimer(IN := Start, PT := T#5s, Q => TimerDone, ET => ET);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		// First, evaluate the whole program to set up the environment
		testEvalWithEnv(t, input, env)

		// Helper to run one "scan"
		runScan := func() {
			// In a real app, you'd re-evaluate the program body.
			// For this test, we just need to evaluate the FB call.
			testEvalWithEnv(t, `MyTimer(IN := Start, PT := T#5s, Q => TimerDone, ET => ET);`, env)
		}

		// --- Cycle 1: Initial state ---
		env.Set("Start", FALSE)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), false)
		testTimeObject(t, mustGet(env, "ET"), 0)

		// --- Cycle 2: Rising edge on IN ---
		env.Set("Start", TRUE)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), false) // Q is still false
		testTimeObject(t, mustGet(env, "ET"), 0)               // ET is still 0 on the first scan

		// --- Cycle 3: Time advances (3s) ---
		advanceTime(3 * time.Second)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), false) // Q is still false
		testTimeObject(t, mustGet(env, "ET"), 3*time.Second)

		// --- Cycle 4: Time reaches PT (5s) ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), true) // Q is now true
		testTimeObject(t, mustGet(env, "ET"), 5*time.Second)  // ET is capped at PT

		// --- Cycle 5: IN is still true, time advances further ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), true) // Q remains true
		testTimeObject(t, mustGet(env, "ET"), 5*time.Second)  // ET remains capped at PT

		// --- Cycle 6: Falling edge on IN ---
		env.Set("Start", FALSE)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerDone"), false) // Q resets to false
		testTimeObject(t, mustGet(env, "ET"), 0)               // ET resets to 0
	})

	t.Run("CTU - Counter Up", func(t *testing.T) {
		input := `
			PROGRAM TestCTU
				VAR
					MyCounter : CTU;
					CountUp : BOOL;
					Reset : BOOL;
					IsDone : BOOL;
					CurrentValue : INT;
				END_VAR

				MyCounter(CU := CountUp, R := Reset, PV := 3, Q => IsDone, CV => CurrentValue);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)

		runScan := func() {
			testEvalWithEnv(t, `MyCounter(CU := CountUp, R := Reset, PV := 3, Q => IsDone, CV => CurrentValue);`, env)
		}

		// --- Cycle 1: Initial state ---
		env.Set("CountUp", FALSE)
		env.Set("Reset", FALSE)
		runScan()
		testBooleanObject(t, mustGet(env, "IsDone"), false)
		testIntegerObject(t, mustGet(env, "CurrentValue"), 0)

		// --- Cycle 2: First rising edge on CU ---
		env.Set("CountUp", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 1)
		testBooleanObject(t, mustGet(env, "IsDone"), false)

		// --- Cycle 3: CU is still high (no change) ---
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 1)

		// --- Cycle 4: Falling edge on CU ---
		env.Set("CountUp", FALSE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 1)

		// --- Cycle 5: Second rising edge ---
		env.Set("CountUp", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 2)
		env.Set("CountUp", FALSE)
		runScan()

		// --- Cycle 6: Third rising edge (reaches PV) ---
		env.Set("CountUp", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 3)
		testBooleanObject(t, mustGet(env, "IsDone"), true) // Q is now true

		// --- Cycle 7: Fourth rising edge (CV does not exceed PV in this implementation) ---
		env.Set("CountUp", FALSE)
		runScan()
		env.Set("CountUp", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 3) // CV is capped
		testBooleanObject(t, mustGet(env, "IsDone"), true)

		// --- Cycle 8: Reset ---
		env.Set("CountUp", FALSE)
		env.Set("Reset", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 0)
		testBooleanObject(t, mustGet(env, "IsDone"), false)
	})

	t.Run("TOF - Timer Off-Delay", func(t *testing.T) {
		input := `
			PROGRAM TestTOF
				VAR
					MyTimer : TOF;
					Input : BOOL;
					TimerActive : BOOL;
					ET : TIME;
				END_VAR

				MyTimer(IN := Input, PT := T#5s, Q => TimerActive, ET => ET);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)

		runScan := func() {
			testEvalWithEnv(t, `MyTimer(IN := Input, PT := T#5s, Q => TimerActive, ET => ET);`, env)
		}

		// --- Cycle 1: IN is high ---
		env.Set("Input", TRUE)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerActive"), true)
		testTimeObject(t, mustGet(env, "ET"), 0)

		// --- Cycle 2: Falling edge on IN ---
		env.Set("Input", FALSE)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerActive"), true) // Q remains true
		testTimeObject(t, mustGet(env, "ET"), 0)

		// --- Cycle 3: Time advances (3s) ---
		advanceTime(3 * time.Second)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerActive"), true) // Q still true
		testTimeObject(t, mustGet(env, "ET"), 3*time.Second)

		// --- Cycle 4: Time reaches PT (5s) ---
		advanceTime(2 * time.Second)
		runScan()
		testBooleanObject(t, mustGet(env, "TimerActive"), false) // Q is now false
		testTimeObject(t, mustGet(env, "ET"), 5*time.Second)     // ET is capped
	})

	t.Run("CTD - Counter Down", func(t *testing.T) {
		input := `
			PROGRAM TestCTD
				VAR
					MyCounter : CTD;
					CountDown : BOOL;
					Load : BOOL;
					IsDone : BOOL;
					CurrentValue : INT;
				END_VAR

				MyCounter(CD := CountDown, LD := Load, PV := 3, Q => IsDone, CV => CurrentValue);
			END_PROGRAM
		`
		env := object.NewEnvironment()
		testEvalWithEnv(t, input, env)

		runScan := func() {
			testEvalWithEnv(t, `MyCounter(CD := CountDown, LD := Load, PV := 3, Q => IsDone, CV => CurrentValue);`, env)
		}

		// --- Cycle 1: Load the counter ---
		env.Set("Load", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 3)
		testBooleanObject(t, mustGet(env, "IsDone"), false)

		// --- Cycle 2: First rising edge on CD ---
		env.Set("Load", FALSE)
		env.Set("CountDown", TRUE)
		runScan()
		testIntegerObject(t, mustGet(env, "CurrentValue"), 2)
		env.Set("CountDown", FALSE)
		runScan()

		// --- Cycle 3: Count down to 0 ---
		env.Set("CountDown", TRUE)
		runScan() // CV = 1
		env.Set("CountDown", FALSE)
		runScan()
		env.Set("CountDown", TRUE)
		runScan() // CV = 0
		testIntegerObject(t, mustGet(env, "CurrentValue"), 0)
		testBooleanObject(t, mustGet(env, "IsDone"), true) // Q is now true
	})
}

func TestTP_PulseTimer(t *testing.T) {
	// Mock time for timer tests
	originalNowFunc := nowFunc
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockTime }
	defer func() { nowFunc = originalNowFunc }()

	// Helper to advance mock time
	advanceTime := func(d time.Duration) {
		mockTime = mockTime.Add(d)
	}

	input := `
		PROGRAM TestTP
			VAR
				MyPulse : TP;
				Trigger : BOOL;
				PulseOut : BOOL;
				ET : TIME;
			END_VAR

			MyPulse(IN := Trigger, PT := T#5s, Q => PulseOut, ET => ET);
		END_PROGRAM
	`
	env := object.NewEnvironment()
	testEvalWithEnv(t, input, env)

	runScan := func() {
		testEvalWithEnv(t, `MyPulse(IN := Trigger, PT := T#5s, Q => PulseOut, ET => ET);`, env)
	}

	// --- Cycle 1: Initial state ---
	env.Set("Trigger", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "PulseOut"), false)
	testTimeObject(t, mustGet(env, "ET"), 0)

	// --- Cycle 2: Rising edge on IN, pulse starts ---
	env.Set("Trigger", TRUE)
	runScan()
	testBooleanObject(t, mustGet(env, "PulseOut"), true)
	testTimeObject(t, mustGet(env, "ET"), 0)

	// --- Cycle 3: IN goes low, but pulse continues ---
	advanceTime(2 * time.Second)
	env.Set("Trigger", FALSE)
	runScan()
	testBooleanObject(t, mustGet(env, "PulseOut"), true)
	testTimeObject(t, mustGet(env, "ET"), 2*time.Second)

	// --- Cycle 4: Time reaches PT, pulse ends ---
	advanceTime(3 * time.Second) // Total elapsed time is now 5s
	runScan()
	testBooleanObject(t, mustGet(env, "PulseOut"), false)
	testTimeObject(t, mustGet(env, "ET"), 5*time.Second)

	// --- Cycle 5: State after pulse completion ---
	advanceTime(1 * time.Second)
	runScan()
	testBooleanObject(t, mustGet(env, "PulseOut"), false)
	testTimeObject(t, mustGet(env, "ET"), 0)
}

func TestFunctionBlockWithSFCBody(t *testing.T) {
	input := `
		FUNCTION_BLOCK MySFC_FB
			VAR_INPUT
				EnableTransition : BOOL;
			END_VAR
			VAR_OUTPUT
				ActiveStepOut : INT;
			END_VAR

			INITIAL_STEP S1:
				ActiveStepOut := 1;
			END_STEP

			TRANSITION FROM S1 TO S2 := EnableTransition;
			END_TRANSITION

			STEP S2:
				ActiveStepOut := 2;
			END_STEP
		END_FUNCTION_BLOCK

		PROGRAM TestSFCinFB
			VAR
				myFb : MySFC_FB;
				doTransition : BOOL := FALSE;
				currentActiveStep : INT;
			END_VAR

			myFb(EnableTransition := doTransition, ActiveStepOut => currentActiveStep);
		END_PROGRAM
	`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkEvaluatorErrors(t, p, "TestFunctionBlockWithSFCBody", input)

	env := object.NewEnvironment()
	// Evaluate the entire program to declare the FB type and the main program POU.
	Eval(program, env)

	// Helper to run one "scan" by re-evaluating the call to the FB instance.
	runScan := func() {
		// In a real PLC, the program body would be re-evaluated.
		// For this test, we just need to re-evaluate the FB call.
		testEvalWithEnv(t, `myFb(EnableTransition := doTransition, ActiveStepOut => currentActiveStep);`, env)
	}

	// --- Cycle 1: Initial state ---
	// The FB is called with doTransition = FALSE.
	// The SFC should be in S1, and ActiveStepOut should be 1.
	runScan()
	testIntegerObject(t, mustGet(env, "currentActiveStep"), 1)

	// --- Cycle 2: Still in S1 ---
	// doTransition is still FALSE, so no transition should occur.
	runScan()
	testIntegerObject(t, mustGet(env, "currentActiveStep"), 1)

	// --- Cycle 3: Transition to S2 ---
	// Set the input condition to TRUE and run the scan.
	env.Set("doTransition", TRUE)
	runScan()
	testIntegerObject(t, mustGet(env, "currentActiveStep"), 2)
}

func checkEvaluatorErrors(t *testing.T, p *parser.Parser, testName string, input string) {
	t.Helper()
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("FAIL: %s - parser has %d errors for input:\n%s", testName, len(errors), input)
	for _, msg := range errors {
		t.Errorf("Evaluator error: %q", msg)
	}
	t.FailNow()
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

	// After the first call `OuterFunc(OuterVar := OriginalVar)`:
	// OriginalVar starts at 5.
	// OuterFunc passes it to InnerFunc.
	// InnerFunc modifies it to 5 * 2 = 10.
	// This change should propagate all the way back to OriginalVar.
	testIntegerObject(t, mustGet(env, "OriginalVar"), 10)

	// After the second call `InnerFunc(InnerVar := OriginalVar)`:
	// OriginalVar is now 10.
	// InnerFunc modifies it to 10 * 2 = 20.
	testIntegerObject(t, mustGet(env, "OriginalVar"), 20)
}

func TestPumpControlSFC(t *testing.T) {
	input := `
		PROGRAM PumpControlProgram
			VAR
				StartButton: BOOL;
				TankHighSensor: BOOL;
				PumpMotor: BOOL;
				TimerDone: BOOL;
			END_VAR

			ACTION IdleAction:
				PumpMotor := FALSE;
			END_ACTION

			ACTION RunningAction:
				PumpMotor := TRUE;
			END_ACTION

			INITIAL_STEP Idle:
				IdleAction();
			END_STEP

			TRANSITION FROM Idle TO Running := StartButton AND NOT TankHighSensor;
			END_TRANSITION

			STEP Running:
				RunningAction();
			END_STEP

			TRANSITION FROM Running TO Idle := TankHighSensor OR TimerDone;
			END_TRANSITION
		END_PROGRAM
	`

	env := object.NewEnvironment()
	// Evaluate the program to declare the POU and its variables.
	// testEvalWithEnv will parse and evaluate the program, returning the SFC object.
	sfcObj := testEvalWithEnv(t, input, env)
	sfc, ok := sfcObj.(*object.SFC)
	if !ok {
		t.Fatalf("Evaluation did not return an SFC object. got=%T", sfcObj)
	}

	// Helper to run a scan cycle
	runScan := func() {
		evalSFCCycle(sfc, env)
	}

	// --- Cycle 1: Initial State ---
	// System is idle, pump should be off.
	env.Set("StartButton", FALSE)
	env.Set("TankHighSensor", FALSE)
	env.Set("TimerDone", FALSE)
	runScan()

	if !sfc.Steps["Idle"].IsActive {
		t.Fatal("SFC should be in 'Idle' step initially.")
	}
	testBooleanObject(t, mustGet(env, "PumpMotor"), false)

	// --- Cycle 2: Transition to Running ---
	// Press the start button. Tank is not high. Pump should turn on.
	env.Set("StartButton", TRUE)
	runScan()

	if !sfc.Steps["Running"].IsActive {
		t.Fatal("SFC should have transitioned to 'Running' step.")
	}
	testBooleanObject(t, mustGet(env, "PumpMotor"), true)

	// --- Cycle 3: Transition back to Idle via TankHighSensor ---
	// Release start button, sensor indicates tank is full. Pump should turn off.
	env.Set("StartButton", FALSE)
	env.Set("TankHighSensor", TRUE)
	runScan()

	if !sfc.Steps["Idle"].IsActive {
		t.Fatal("SFC should have transitioned back to 'Idle' step.")
	}
	testBooleanObject(t, mustGet(env, "PumpMotor"), false)

	// --- Cycle 4 & 5: Transition to Running, then stop via TimerDone ---
	env.Set("TankHighSensor", FALSE)
	env.Set("StartButton", TRUE)
	runScan() // Go to Running
	env.Set("StartButton", FALSE)
	env.Set("TimerDone", TRUE)
	runScan() // Go to Idle
	testBooleanObject(t, mustGet(env, "PumpMotor"), false)
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
		{"INVALID_TYPE#10;", "identifier not found: INVALID_TYPE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, evaluated, expected)
			case float64:
				testRealObject(t, evaluated, expected)
			case bool:
				testBooleanObject(t, evaluated, expected)
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
