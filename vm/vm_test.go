package vm

import (
	"beedance/ast"
	"beedance/code"
	"beedance/compiler"
	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	_ "beedance/stdlib" // Import for side-effect of registering built-ins
	"fmt"
	"strings"
	"testing"
)

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 * (2 + 10)", 60},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 = 1", true},
		{"1 != 1", false},
		{"1 = 2", false},
		{"1 != 2", true},
		{"true = true", true},
		{"false = false", true},
		{"true = false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) = true", true},
		{"(1 < 2) = false", false},
		{"(1 > 2) = true", false},
		{"(1 > 2) = false", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"IF 1 < 2 THEN 10; ELSE 20; END_IF", 10},
		{"IF true THEN 10; ELSE 20; END_IF", 10},
		{"IF false THEN 10; ELSE 20; END_IF", 20},
		{"IF 1 < 2 THEN 10; END_IF", 10},
		{"IF 1 < 2 THEN 10; ELSE 20; END_IF", 10},
		{"IF 1 > 2 THEN 10; ELSE 20; END_IF", 20},
		{"IF 1 > 2 THEN 10; END_IF", Null},
		{"IF false THEN 10; END_IF", Null},
		{"IF 1 > 2 THEN 99; ELSIF (1 = 1) THEN 42; ELSE 100; END_IF", 42},
		{"IF 1 > 2 THEN 99; ELSIF (1 = 0) THEN 42; ELSE 100; END_IF", 100},
		// Per IEC 61131-3, conditions must be boolean. Other types are not "truthy".
		// The VM's isTruthy function correctly returns false for non-booleans.
		{"IF 1 THEN 10; ELSE 20; END_IF", 20},
	}

	runVmTests(t, tests)
}

func TestGlobalVarStatements(t *testing.T) {
	tests := []vmTestCase{
		// Use VAR_GLOBAL for top-level variable declarations, which is more aligned with IEC 61131-3 structure
		// where global variables are explicitly marked.
		{"VAR_GLOBAL g_one : INT := 1; END_VAR g_one;", 1},
		{"VAR_GLOBAL g_one : INT := 1; g_two : INT := 2; END_VAR g_one + g_two;", 3},
		{`VAR_GLOBAL
			g_one : INT := 1;
			g_two : INT;
		 END_VAR
		 g_two := g_one + g_one; g_one + g_two;`, 3},
	}

	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		// Per IEC 61131-3, STRING literals use single quotes and concatenation
		// is handled by the CONCAT function, not the '+' operator.
		{`'beedance';`, "beedance"},
	}

	runVmTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[];", []int{}},
		{"[1, 2, 3];", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4, 5 + 6];", []int{3, 12, 11}},
	}

	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			"{};", map[object.HashKey]int64{},
		},
		{
			"{1: 2, 2: 3};",
			map[object.HashKey]int64{
				(&object.LInt{Value: 1}).HashKey(): 2,
				(&object.LInt{Value: 2}).HashKey(): 3,
			},
		},
		{
			"{1 + 1: 2 * 2, 3 + 3: 4 * 4};",
			map[object.HashKey]int64{
				(&object.LInt{Value: 2}).HashKey(): 4,
				(&object.LInt{Value: 6}).HashKey(): 16,
			},
		},
	}

	runVmTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1];", 2},
		{"[1, 2, 3][0 + 2];", 3},
		{"[[1, 1, 1]][0][0];", 1},
		{"[][0];", Null},
		{"[1, 2, 3][99];", Null},
		{"[1][-1];", Null},
		{"{1: 1, 2: 2}[1];", 1},
		{"{1: 1, 2: 2}[2];", 2},
		{"{1: 1}[0];", Null},
		{"{}[0];", Null},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION fivePlusTen : INT fivePlusTen := 5 + 10; END_FUNCTION;
					fivePlusTen();`,
			expected: 15,
		},
		{
			input: `FUNCTION one : INT one := 1; END_FUNCTION;
					FUNCTION two : INT two := 2; END_FUNCTION;
					one() + two();`,
			expected: 3,
		},
		{
			input: `FUNCTION a : INT a := 1; END_FUNCTION;
					FUNCTION b : INT b := a() + 1; END_FUNCTION;
					FUNCTION c : INT c := b() + 1; END_FUNCTION;
					c();`,
			expected: 3,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION earlyExit : INT RETURN 99; END_FUNCTION;
					earlyExit();`,
			expected: 99,
		},
		{
			input: `FUNCTION earlyExit : INT RETURN 99; RETURN 100; END_FUNCTION;
					earlyExit();`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

// TestFunctionsWithoutReturnValue checks that a function which never assigns its
// result returns the default value of its return type (0 for INT).
func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION noReturn : INT END_FUNCTION;
					noReturn();`,
			expected: 0,
		},
		{
			input: `FUNCTION noReturn : INT END_FUNCTION;
					FUNCTION noReturnTwo : INT noReturnTwo := noReturn(); END_FUNCTION;
					noReturn();
					noReturnTwo();`,
			expected: 0,
		},
	}

	runVmTests(t, tests)
}
func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			FUNCTION one : INT
				VAR one_local : INT := 1;END_VAR
				one := one_local;
			END_FUNCTION;
			PROGRAM TestProgram VAR_OUTPUT result : INT; END_VAR
				result := one();
			END_PROGRAM;
			TestProgram();
			TestProgram.result;`,
			expected: 1,
		},
		{
			input: `
			FUNCTION oneAndTwo : INT
				VAR one: INT := 1; two: INT := 2; END_VAR
				oneAndTwo := one + two;
			END_FUNCTION;
			PROGRAM TestProgram VAR_OUTPUT result : INT; END_VAR
				result := oneAndTwo();
			END_PROGRAM;
			TestProgram();
			TestProgram.result;`,
			expected: 3,
		},
		{
			input: `
			FUNCTION oneAndTwo : INT
				VAR one:INT:=1; two:INT:=2; END_VAR
				oneAndTwo := one + two;
			END_FUNCTION;
			FUNCTION threeAndFour : INT
				VAR three:INT:=3; four:INT:=4; END_VAR
				threeAndFour := three + four;
			END_FUNCTION;
			PROGRAM TestProgram VAR_OUTPUT result : INT; END_VAR
				result := oneAndTwo() + threeAndFour();
			END_PROGRAM;
			TestProgram();
			TestProgram.result;`,
			expected: 10,
		},
		{
			input: `
			FUNCTION firstFoobar : INT
				VAR foobar : INT := 50; END_VAR
				firstFoobar := foobar;
			END_FUNCTION;
			FUNCTION secondFoobar : INT
				VAR foobar : INT := 100; END_VAR
				secondFoobar := foobar;
			END_FUNCTION;
			PROGRAM TestProgram VAR_OUTPUT result : INT; END_VAR
				result := firstFoobar() + secondFoobar();
			END_PROGRAM;
			TestProgram();
			TestProgram.result;`,
			expected: 150,
		},
		{
			input: `
			VAR_GLOBAL globalSeed : INT := 50; END_VAR;
			// IEC 61131-3 does not allow VAR_EXTERNAL in a FUNCTION, so the
			// PROGRAM binds the global and passes it to the functions.
			FUNCTION minusOne : INT
				VAR_INPUT
					seed : INT;
				END_VAR
				VAR
					num : INT := 1;
				END_VAR
				minusOne := seed - num;
			END_FUNCTION;
			FUNCTION minusTwo : INT
				VAR_INPUT
					seed : INT;
				END_VAR
				VAR
					num : INT := 2;
				END_VAR
				minusTwo := seed - num;
			END_FUNCTION;
			PROGRAM TestProgram VAR_OUTPUT result : INT; END_VAR
				VAR_EXTERNAL
					globalSeed : INT;
				END_VAR
				result := minusOne(globalSeed) + minusTwo(globalSeed);
			END_PROGRAM;
			TestProgram();
			TestProgram.result;`,
			expected: 97,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithArgumentsAndBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION identity : INT 
						VAR_INPUT 
							a:INT; 
						END_VAR 
						identity := a; 
					END_FUNCTION;
					identity(4);`,
			expected: 4,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR sum := a + b; END_FUNCTION;
					sum(1, 2);`,
			expected: 3,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION;
					sum(1, 2);`,
			expected: 3,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION;
					sum(1, 2) + sum(3, 4);`,
			expected: 10,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION;
					FUNCTION outer : INT outer := sum(1, 2) + sum(3, 4); END_FUNCTION;
					outer();`,
			expected: 10,
		},
		{
			input: `
			VAR_GLOBAL globalNum : INT := 10; END_VAR;
			FUNCTION sum : INT
				VAR_INPUT 
					a:INT; 
					b:INT; 
				END_VAR
				VAR 
					c:INT;
				END_VAR
				c := a + b;
				sum := c + globalNum;
			END_FUNCTION;
			FUNCTION outer : INT
				outer := sum(1, 2) + sum(3, 4) + globalNum;
			END_FUNCTION;
			outer() + globalNum;`,
			expected: 50,
		},
	}

	runVmTests(t, tests)
}

// TestCallingFunctionsWithWrongArguments covers non-formal (positional) calls,
// which must supply every input. A formal call such as `f()` or `f(a := 1)` may
// omit inputs, which then take their defaults; see vm_functions_test.go.
func TestCallingFunctionsWithWrongArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input:    `FUNCTION f:INT f:=1; END_FUNCTION; f(1);`,
			expected: `wrong number of arguments: want=0, got=1`,
		},
		{
			input:    `FUNCTION f:INT VAR_INPUT a:INT;b:INT; END_VAR f:=a+b; END_FUNCTION; f(1);`,
			expected: `wrong number of arguments: want=2, got=1`,
		},
	}

	// Finalize built-ins to ensure they are registered and correctly indexed.
	object.FinalizeBuiltins()

	for _, tt := range tests {
		program := parse(t, tt.input)

		// The object.Builtins slice is now correctly indexed, so no sorting is needed.
		comp := compiler.NewCompilerWithBuiltins(object.Builtins)
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vmBuiltins := make([]*object.Builtin, len(object.Builtins))
		for i, entry := range object.Builtins {
			vmBuiltins[i] = entry.Builtin
		}
		vm := NewWithBuiltins(comp.Bytecode(), vmBuiltins)
		err = vm.Run()
		if err == nil {
			t.Fatalf("expected VM error but resulted in none.")
		}

		if err.Error() != tt.expected {
			t.Fatalf("wrong VM error: want=%q, got=%q", tt.expected, err)
		}
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []vmTestCase{
		// --- Monkey-compatible built-ins (adapted for IEC syntax) ---
		{`LEN('');`, 0},             // Test #0
		{`LEN('four');`, 4},         // Test #1
		{`LEN('hello world');`, 11}, // Test #2
		{ // Test #3
			`LEN(1);`,
			&object.Error{
				Message: "BUILTIN ERROR: argument to `LEN` not supported, got LINT",
			},
		},
		{ // Test #4
			`LEN('one', 'two');`,
			&object.Error{
				Message: "BUILTIN ERROR: wrong number of arguments for LEN. got=2, want=1",
			},
		},
		{`LEN([1, 2, 3]);`, 3},          // Test #5
		{`LEN([]);`, 0},                 // Test #6
		{`PUTS('hello world!');`, Null}, // Test #7
		{`FIRST([1, 2, 3]);`, 1},        // Test #8
		{`FIRST([]);`, Null},            // Test #9
		{ // Test #10
			`FIRST(1);`,
			&object.Error{
				Message: "BUILTIN ERROR: argument to `FIRST` must be ARRAY, got LINT",
			},
		},
		{`LAST([1, 2, 3]);`, 3}, // Test #11
		{`LAST([]);`, Null},     // Test #12
		{ // Test #13
			`LAST(1);`,
			&object.Error{
				Message: "BUILTIN ERROR: argument to `LAST` must be ARRAY, got LINT",
			},
		},
		{`REST([1, 2, 3]);`, []int{2, 3}}, // Test #14
		{`REST([]);`, Null},               // Test #15
		{`PUSH([], 1);`, []int{1}},        // Test #16
		{ // Test #17
			`PUSH(1, 1);`,
			&object.Error{
				Message: "BUILTIN ERROR: argument to `PUSH` must be ARRAY, got LINT",
			},
		},

		// --- IEC 61131-3 Standard Built-ins ---
		// String Functions
		{`CONCAT('a', 'b');`, "ab"},       // Test #18
		{`CONCAT('a', 'b', 'c');`, "abc"}, // Test #19
		{ // Test #20
			`CONCAT('a');`,
			&object.Error{
				Message: "BUILTIN ERROR: wrong number of arguments for CONCAT. got=1, want>=2",
			},
		},
		{`LEFT('abcde', 2);`, "ab"},    // Test #21
		{`RIGHT('abcde', 2);`, "de"},   // Test #22
		{`MID('abcde', 2, 3);`, "bcd"}, // Test #23, MID(IN, P, L)
		{`FIND('abcabc', 'b');`, 2},    // Test #24

		// Selection Functions
		{`LIMIT(10, 5, 20);`, 10},         // Test #25
		{`LIMIT(10, 15, 20);`, 15},        // Test #26
		{`LIMIT(10, 25, 20);`, 20},        // Test #27
		{`LIMIT(10.0, 5.5, 20.0);`, 10.0}, // Test #28
		{`MUX(0, 100, 101, 102);`, 100},   // Test #29
		{`MUX(2, 'a', 'b', 'c');`, "c"},   // Test #30
		{`SEL(FALSE, 10, 20);`, 10},       // Test #31
		{`SEL(TRUE, 'a', 'b');`, "b"},     // Test #32
		{`MOVE(123);`, 123},               // Test #33
		{`MOVE('hello');`, "hello"},       // Test #34

		// Math Functions
		{`SQRT(9);`, 3.0},     // Test #35
		{`ABS(-10);`, 10},     // Test #36
		{`ABS(-10.5);`, 10.5}, // Test #37
		{`ROUND(3.5);`, 4},    // Test #38
		{`TRUNC(-3.9);`, -3},  // Test #39
		{`SIN(0);`, 0.0},      // Test #40
		{`COS(0);`, 1.0},      // Test #41
	}

	runVmTests(t, tests)
}
func TestRecursiveFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR;
						IF x = 0 THEN
							countDown := 0;
						ELSE
							countDown := countDown(x - 1);
						END_IF
					END_FUNCTION;
					countDown(1);`,
			expected: 0,
		},
		{
			input: `FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR
						IF x = 0 THEN countDown := 0; ELSE countDown := countDown(x - 1); END_IF
					END_FUNCTION;
					FUNCTION wrapper : INT wrapper := countDown(1); END_FUNCTION;
					wrapper();`,
			expected: 0,
		},
		{
			input: `FUNCTION wrapper : INT;
						FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR;
							IF x = 0 THEN countDown := 0; ELSE countDown := countDown(x - 1); END_IF
						END_FUNCTION;
						wrapper := countDown(1);
					END_FUNCTION;
					wrapper();`,
			expected: 0,
		},
	}

	runVmTests(t, tests)
}

func TestVMMacroExpansion(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			VAR
				my_macro : MACRO := macro(a, b) { EXPR(EVAL(a) + EVAL(b)); };
			END_VAR

			my_macro(1 + 1, 2 + 2);
			`,
			// The macro expands to `(2 + 4)` at compile time.
			// The VM should receive bytecode for `6`.
			expected: 6,
		},
		// {
		// 	input: `
		// 	VAR
		// 		unless : MACRO := macro(condition, consequence, alternative) {
		// 			EXPR(IF NOT (EVAL(condition)) THEN EVAL(consequence); ELSE EVAL(alternative); END_IF);
		// 		};
		// 	END_VAR

		// 	unless(1 > 5, 10, 20);
		// 	`,
		// 	// The macro expands to `IF NOT (1 > 5) THEN 10; ELSE 20; END_IF`, which evaluates to 10.
		// 	expected: 10,
		// },
		{
			input:    `VAR a : MACRO := macro() { EXPR(1); }; END_VAR VAR b : MACRO := macro() { EXPR(2); }; END_VAR a() + b();`,
			expected: 3,
		},
	}

	runVmTestsWithMacros(t, tests)
}

func TestRecursiveFibonacci(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION fibonacci : INT VAR_INPUT x:INT; END_VAR;
						IF x = 0 THEN
							fibonacci := 0;
						ELSIF x = 1 THEN
							fibonacci := 1;
						ELSE
							fibonacci := fibonacci(x - 1) + fibonacci(x - 2);
						END_IF
					END_FUNCTION;
					fibonacci(15);`,
			expected: 610,
		},
	}

	runVmTests(t, tests)
}

type vmTestCase struct {
	input    string
	expected interface{}
}

func runVmTestsWithMacros(t *testing.T, tests []vmTestCase) {
	t.Helper()

	object.FinalizeBuiltins()
	builtinEntries := object.Builtins

	maxIndex := -1
	for _, entry := range builtinEntries {
		if entry.Index > maxIndex {
			maxIndex = entry.Index
		}
	}
	vmBuiltins := make([]*object.Builtin, maxIndex+1)
	for _, entry := range builtinEntries {
		vmBuiltins[entry.Index] = entry.Builtin
	}

	for i, tt := range tests {
		program := parse(t, tt.input)
		env := object.NewEnvironment()
		evaluator.DefineMacros(program, env)
		expanded := evaluator.ExpandMacros(program, env)

		comp := compiler.NewCompilerWithBuiltins(builtinEntries)
		err := comp.Compile(expanded)
		if err != nil {
			t.Fatalf("test #%d/%d on input '%s': compiler error: %s", i, len(tests), tt.input, err)
		}

		vm := NewWithBuiltins(comp.Bytecode(), vmBuiltins)
		err = vm.Run()
		if err != nil {
			t.Fatalf("test #%d/%d on input '%s': vm error: %s", i, len(tests), tt.input, err)
		}

		stackElem := vm.LastPoppedStackElem()
		testExpectedObject(t, i, tt.expected, stackElem)
	}
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	// Finalize built-ins to ensure they are registered from all stdlib packages
	// and sorted by their iota-defined index.
	object.FinalizeBuiltins()

	builtinEntries := object.Builtins

	// Create a slice for the VM's built-ins that is explicitly sized
	// to the highest registered index. This is more robust than relying
	// on the length of the `builtinEntries` slice.
	maxIndex := -1
	for _, entry := range builtinEntries {
		if entry.Index > maxIndex {
			maxIndex = entry.Index
		}
	}
	vmBuiltins := make([]*object.Builtin, maxIndex+1)
	for _, entry := range builtinEntries {
		vmBuiltins[entry.Index] = entry.Builtin
	}

	for i, tt := range tests {
		program := parse(t, tt.input)

		comp := compiler.NewCompilerWithBuiltins(builtinEntries)
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("test #%d/%d on input '%s': compiler error: %s", i, len(tests), tt.input, err)
		}

		vm := NewWithBuiltins(comp.Bytecode(), vmBuiltins)
		err = vm.Run()
		if err != nil {
			t.Fatalf("test #%d/%d on input '%s': vm error: %s", i, len(tests), tt.input, err)
		}

		stackElem := vm.LastPoppedStackElem()

		testExpectedObject(t, i, tt.expected, stackElem)
	}
}

// parse parses input and fails the test immediately if the parser reports any
// errors, so tests never compile a partially parsed AST.
func parse(t testing.TB, input string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser has %d error(s) for input:\n%s\nerrors:\n  %s", len(errs), input, strings.Join(errs, "\n  "))
	}
	return program
}

func testExpectedObject(
	t *testing.T,
	testIndex int,
	expected interface{},
	actual object.Object,
) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("test #%d: testIntegerObject failed: %s", testIndex, err)
		}

	case float64:
		err := testRealObject(expected, actual)
		if err != nil {
			t.Errorf("test #%d: testRealObject failed: %s", testIndex, err)
		}

	case bool:
		err := testBooleanObject(bool(expected), actual)
		if err != nil {
			t.Errorf("test #%d: testBooleanObject failed: %s", testIndex, err)
		}

	case *object.Null:
		if actual != Null {
			t.Errorf("test #%d: object is not Null: %T (%+v)", testIndex, actual, actual)
		}

	case string:
		err := testStringObject(expected, actual)
		if err != nil {
			t.Errorf("test #%d: testStringObject failed: %s", testIndex, err)
		}

	case []int:
		array, ok := actual.(*object.Array)
		if !ok {
			t.Errorf("test #%d: object not Array: %T (%+v)", testIndex, actual, actual)
			return
		}

		if len(array.Elements) != len(expected) {
			t.Errorf("test #%d: wrong num of elements. want=%d, got=%d",
				testIndex, len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("test #%d, element %d: testIntegerObject failed: %s", testIndex, i, err)
			}
		}

	case map[object.HashKey]int64:
		hash, ok := actual.(*object.Hash)
		if !ok {
			t.Errorf("test #%d: object is not Hash. got=%T (%+v)", testIndex, actual, actual)
			return
		}

		if len(hash.Pairs) != len(expected) {
			t.Errorf("test #%d: hash has wrong number of Pairs. want=%d, got=%d",
				testIndex, len(expected), len(hash.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := hash.Pairs[expectedKey]
			if !ok {
				t.Errorf("test #%d: no pair for given key in Pairs", testIndex)
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("test #%d: testIntegerObject failed: %s", testIndex, err)
			}
		}

	case *object.Error:
		errObj, ok := actual.(*object.Error)
		if !ok {
			t.Errorf("test #%d: object is not Error: %T (%+v)", testIndex, actual, actual)
			return
		}
		if errObj.Message != expected.Message {
			t.Errorf("test #%d: wrong error message. expected=%q, got=%q",
				testIndex, expected.Message, errObj.Message)
		}
	}
}

func testIntegerObject(expected int64, actual object.Object) error {
	result, ok := actual.(*object.LInt)
	if !ok {
		return fmt.Errorf("object is not LInt. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
	}

	return nil
}

func testRealObject(expected float64, actual object.Object) error {
	var result float64
	var ok bool
	switch o := actual.(type) {
	case *object.Real:
		result = o.Value
		ok = true
	case *object.LReal:
		result = o.Value
		ok = true
	}
	if !ok {
		return fmt.Errorf("object is not Real or LReal. got=%T (%+v)", actual, actual)
	}

	// Use a small tolerance for float comparison
	if diff := result - expected; diff < -0.000001 || diff > 0.000001 {
		return fmt.Errorf("object has wrong value. got=%f, want=%f", result, expected)
	}

	return nil
}

func testBooleanObject(expected bool, actual object.Object) error {
	result, ok := actual.(*object.Boolean)
	if !ok {
		return fmt.Errorf("object is not Boolean. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%t, want=%t",
			result.Value, expected)
	}

	return nil
}

func testStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.String)
	if !ok {
		return fmt.Errorf("object is not String. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%q, want=%q",
			result.Value, expected)
	}

	return nil
}

// vmErrorTestCase is an input that compiles but must fail when run.
type vmErrorTestCase struct {
	input         string
	expectedError string
}

// runVmErrorTests compiles each input and checks that running it fails with
// exactly the expected error.
func runVmErrorTests(t *testing.T, tests []vmErrorTestCase) {
	t.Helper()
	object.FinalizeBuiltins()
	for i, tt := range tests {
		comp := compiler.New()
		if err := comp.Compile(parse(t, tt.input)); err != nil {
			t.Fatalf("test #%d on input %q: compiler error: %s", i, tt.input, err)
		}
		err := New(comp.Bytecode()).Run()
		if err == nil {
			t.Fatalf("test #%d on input %q: expected VM error %q, got none", i, tt.input, tt.expectedError)
		}
		if err.Error() != tt.expectedError {
			t.Fatalf("test #%d on input %q: wrong VM error.\nwant: %s\ngot:  %s", i, tt.input, tt.expectedError, err)
		}
	}
}

// runBytecode runs hand-assembled instructions with the given constants and
// built-ins. It returns the VM so tests can inspect its state.
func runBytecode(constants []object.Object, builtins []*object.Builtin, instructions ...[]byte) (*VM, error) {
	var ins code.Instructions
	for _, i := range instructions {
		ins = append(ins, i...)
	}
	machine := NewWithBuiltins(&compiler.Bytecode{Instructions: ins, Constants: constants}, builtins)
	return machine, machine.Run()
}

// compiledFunction wraps instructions in a CompiledFunction constant.
func compiledFunction(numLocals, numParams int, instructions ...[]byte) *object.CompiledFunction {
	var ins code.Instructions
	for _, i := range instructions {
		ins = append(ins, i...)
	}
	return &object.CompiledFunction{Instructions: ins, NumLocals: numLocals, NumParameters: numParams}
}

// compileForTest compiles input, failing the test on a parser or compiler error.
func compileForTest(t *testing.T, input string) *compiler.Bytecode {
	t.Helper()
	object.FinalizeBuiltins()
	comp := compiler.New()
	if err := comp.Compile(parse(t, input)); err != nil {
		t.Fatalf("compiler error for %q: %s", input, err)
	}
	return comp.Bytecode()
}

// emptyBytecode returns an empty program, for tests that drive VM internals directly.
func emptyBytecode() *compiler.Bytecode {
	return &compiler.Bytecode{Instructions: code.Instructions{}, Constants: []object.Object{}}
}
