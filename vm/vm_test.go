package vm

import (
	"beedance/ast"
	"beedance/compiler"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"fmt"
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
		{"!(if (false) then { 5; } end_if)", true},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"IF TRUE THEN 10; END_IF", 10},
		{"IF TRUE THEN 10; ELSE 20; END_IF", 10},
		{"IF FALSE THEN 10; ELSE 20; END_IF", 20},
		{"IF 1 < 2 THEN 10; END_IF", 10},
		{"IF 1 < 2 THEN 10; ELSE 20; END_IF", 10},
		{"IF 1 > 2 THEN 10; ELSE 20; END_IF", 20},
		{"IF 1 > 2 THEN 10; END_IF", Null},
		{"IF FALSE THEN 10; END_IF", Null},
		{"IF 1 > 2 THEN 99; ELSIF 1 = 1 THEN 42; ELSE 100; END_IF", 42},
		{"IF 1 > 2 THEN 99; ELSIF 1 = 0 THEN 42; ELSE 100; END_IF", 100},
		// Non-boolean conditions should evaluate to false.
		{"IF 1 THEN 10; ELSE 20; END_IF", 20},
	}

	runVmTests(t, tests)
}

func TestGlobalVarStatements(t *testing.T) {
	tests := []vmTestCase{
		{"VAR one : INT; END_VAR one := 1; one;", 1},
		{"VAR one, two : INT; END_VAR one := 1; two := 2; one + two;", 3},
		{"VAR one, two : INT; END_VAR one := 1; two := one + one; one + two;", 3},
	}

	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{`"beedance"`, "beedance"},
		{`"mon" + "key"`, "monkey"},
		{`"mon" + "key" + "banana"`, "monkeybanana"},
	}

	runVmTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[]", []int{}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4, 5 + 6]", []int{3, 12, 11}},
	}

	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			"{}", map[object.HashKey]int64{},
		},
		{
			"{1: 2, 2: 3}",
			map[object.HashKey]int64{
				(&object.LInt{Value: 1}).HashKey(): 2,
				(&object.LInt{Value: 2}).HashKey(): 3,
			},
		},
		{
			"{1 + 1: 2 * 2, 3 + 3: 4 * 4}",
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
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][0 + 2]", 3},
		{"[[1, 1, 1]][0][0]", 1},
		{"[][0]", Null},
		{"[1, 2, 3][99]", Null},
		{"[1][-1]", Null},
		{"{1: 1, 2: 2}[1]", 1},
		{"{1: 1, 2: 2}[2]", 2},
		{"{1: 1}[0]", Null},
		{"{}[0]", Null},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION fivePlusTen : INT fivePlusTen := 5 + 10; END_FUNCTION
					fivePlusTen();`,
			expected: 15,
		},
		{
			input: `FUNCTION one : INT one := 1; END_FUNCTION
					FUNCTION two : INT two := 2; END_FUNCTION
					one() + two()`,
			expected: 3,
		},
		{
			input: `FUNCTION a : INT a := 1; END_FUNCTION
					FUNCTION b : INT b := a() + 1; END_FUNCTION
					FUNCTION c : INT c := b() + 1; END_FUNCTION
					c();`,
			expected: 3,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION earlyExit : INT RETURN 99; END_FUNCTION
					earlyExit();`,
			expected: 99,
		},
		{
			input: `FUNCTION earlyExit : INT RETURN 99; RETURN 100; END_FUNCTION
					earlyExit();`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION noReturn : INT END_FUNCTION
					noReturn();`,
			expected: Null,
		},
		{
			input: `FUNCTION noReturn : INT END_FUNCTION
					FUNCTION noReturnTwo : INT noReturnTwo := noReturn(); END_FUNCTION
					noReturn();
					noReturnTwo();`,
			expected: Null,
		},
	}

	runVmTests(t, tests)
}

func TestFirstClassFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
		let returnsOne = fn() { 1; };
		let returnsOneReturner = fn() { returnsOne; };
		returnsOneReturner()();
		`,
			expected: 1,
		},
		{
			input: `
		let returnsOneReturner = fn() {
			let returnsOne = fn() { 1; };
			returnsOne;
		};
		returnsOneReturner()();
		`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION one : INT VAR one_local : INT := 1; END_VAR one := one_local; END_FUNCTION
					one();`,
			expected: 1,
		},
		{
			input: `
			FUNCTION oneAndTwo : INT
				VAR one: INT := 1; two: INT := 2; END_VAR
				oneAndTwo := one + two;
			END_FUNCTION
			oneAndTwo();`,
			expected: 3,
		},
		{
			input: `
			FUNCTION oneAndTwo : INT VAR one:INT:=1; two:INT:=2; END_VAR oneAndTwo := one + two; END_FUNCTION
			FUNCTION threeAndFour : INT VAR three:INT:=3; four:INT:=4; END_VAR threeAndFour := three + four; END_FUNCTION
			oneAndTwo() + threeAndFour();`,
			expected: 10,
		},
		{
			input: `
			FUNCTION firstFoobar : INT
				VAR foobar : INT := 50; END_VAR
				firstFoobar := foobar;
			END_FUNCTION
			FUNCTION secondFoobar : INT
				VAR foobar : INT := 100; END_VAR
				secondFoobar := foobar;
			END_FUNCTION
			firstFoobar() + secondFoobar();`,
			expected: 150,
		},
		{
			input: `
			VAR globalSeed : INT := 50; END_VAR
			FUNCTION minusOne : INT
				VAR num : INT := 1; END_VAR
				minusOne := globalSeed - num;
			END_FUNCTION
			FUNCTION minusTwo : INT
				VAR num : INT := 2; END_VAR
				minusTwo := globalSeed - num;
			END_FUNCTION
			minusOne() + minusTwo();`,
			expected: 97,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithArgumentsAndBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION identity : INT VAR_INPUT a:INT; END_VAR identity := a; END_FUNCTION
					identity(4);`,
			expected: 4,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR sum := a + b; END_FUNCTION
					sum(1, 2);`,
			expected: 3,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION
					sum(1, 2);`,
			expected: 3,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION
					sum(1, 2) + sum(3, 4);`,
			expected: 10,
		},
		{
			input: `FUNCTION sum : INT VAR_INPUT a:INT; b:INT; END_VAR VAR c:INT; END_VAR c := a + b; sum := c; END_FUNCTION
					FUNCTION outer : INT outer := sum(1, 2) + sum(3, 4); END_FUNCTION
					outer();`,
			expected: 10,
		},
		{
			input: `
			VAR globalNum : INT := 10; END_VAR
			FUNCTION sum : INT
				VAR_INPUT a:INT; b:INT; END_VAR
				VAR c:INT; END_VAR
				c := a + b;
				sum := c + globalNum;
			END_FUNCTION
			FUNCTION outer : INT
				outer := sum(1, 2) + sum(3, 4) + globalNum;
			END_FUNCTION
			outer() + globalNum;`,
			expected: 50,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithWrongArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input:    `FUNCTION f:INT f:=1; END_FUNCTION f(1);`,
			expected: `wrong number of arguments: want=0, got=1`,
		},
		{
			input:    `FUNCTION f:INT VAR_INPUT a:INT; END_VAR f:=a; END_FUNCTION f();`,
			expected: `wrong number of arguments: want=1, got=0`,
		},
		{
			input:    `FUNCTION f:INT VAR_INPUT a:INT;b:INT; END_VAR f:=a+b; END_FUNCTION f(1);`,
			expected: `wrong number of arguments: want=2, got=1`,
		},
	}

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
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
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{
			`len(1)`,
			&object.Error{
				Message: "argument to `len` not supported, got INTEGER",
			},
		},
		{`len("one", "two")`,
			&object.Error{
				Message: "wrong number of arguments. got=2, want=1",
			},
		},
		{`len([1, 2, 3])`, 3},
		{`len([])`, 0},
		{`puts("hello", "world!")`, Null},
		{`first([1, 2, 3])`, 1},
		{`first([])`, Null},
		{`first(1)`,
			&object.Error{
				Message: "argument to `first` must be ARRAY, got INTEGER",
			},
		},
		{`last([1, 2, 3])`, 3},
		{`last([])`, Null},
		{`last(1)`,
			&object.Error{
				Message: "argument to `last` must be ARRAY, got INTEGER",
			},
		},
		{`rest([1, 2, 3])`, []int{2, 3}},
		{`rest([])`, Null},
		{`push([], 1)`, []int{1}},
		{`push(1, 1)`,
			&object.Error{
				Message: "argument to `push` must be ARRAY, got INTEGER",
			},
		},
	}

	runVmTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
		let newClosure = fn(a) {
			fn() { a; };
		};
		let closure = newClosure(99);
		closure();
		`,
			expected: 99,
		},
		{
			input: `
		let newAdder = fn(a, b) {
			fn(c) { a + b + c };
		};
		let adder = newAdder(1, 2);
		adder(8);
		`,
			expected: 11,
		},
		{
			input: `
		let newAdder = fn(a, b) {
			let c = a + b;
			fn(d) { c + d };
		};
		let adder = newAdder(1, 2);
		adder(8);
		`,
			expected: 11,
		},
		{
			input: `
		let newAdderOuter = fn(a, b) {
			let c = a + b;
			fn(d) {
				let e = d + c;
				fn(f) { e + f; };
			};
		};
		let newAdderInner = newAdderOuter(1, 2)
		let adder = newAdderInner(3);
		adder(8);
		`,
			expected: 14,
		},
		{
			input: `
		let a = 1;
		let newAdderOuter = fn(b) {
			fn(c) {
				fn(d) { a + b + c + d };
			};
		};
		let newAdderInner = newAdderOuter(2)
		let adder = newAdderInner(3);
		adder(8);
		`,
			expected: 14,
		},
		{
			input: `
		let newClosure = fn(a, b) {
			let one = fn() { a; };
			let two = fn() { b; };
			fn() { one() + two(); };
		};
		let closure = newClosure(9, 90);
		closure();
		`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR
						IF x = 0 THEN
							countDown := 0;
						ELSE
							countDown := countDown(x - 1);
						END_IF
					END_FUNCTION
					countDown(1);`,
			expected: 0,
		},
		{
			input: `FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR
						IF x = 0 THEN countDown := 0; ELSE countDown := countDown(x - 1); END_IF
					END_FUNCTION
					FUNCTION wrapper : INT wrapper := countDown(1); END_FUNCTION
					wrapper();`,
			expected: 0,
		},
		{
			input: `FUNCTION wrapper : INT
						FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR
							IF x = 0 THEN countDown := 0; ELSE countDown := countDown(x - 1); END_IF
						END_FUNCTION
						wrapper := countDown(1);
					END_FUNCTION
					wrapper();`,
			expected: 0,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFibonacci(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `FUNCTION fibonacci : INT VAR_INPUT x:INT; END_VAR
						IF x = 0 THEN
							fibonacci := 0;
						ELSIF x = 1 THEN
							fibonacci := 1;
						ELSE
							fibonacci := fibonacci(x - 1) + fibonacci(x - 2);
						END_IF
					END_FUNCTION
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

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err != nil {
			t.Fatalf("vm error: %s", err)
		}

		stackElem := vm.LastPoppedStackElem()

		testExpectedObject(t, tt.expected, stackElem)
	}
}

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func testExpectedObject(
	t *testing.T,
	expected interface{},
	actual object.Object,
) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("testIntegerObject failed: %s", err)
		}

	case bool:
		err := testBooleanObject(bool(expected), actual)
		if err != nil {
			t.Errorf("testBooleanObject failed: %s", err)
		}

	case *object.Null:
		if actual != Null {
			t.Errorf("object is not Null: %T (%+v)", actual, actual)
		}

	case string:
		err := testStringObject(expected, actual)
		if err != nil {
			t.Errorf("testStringObject failed: %s", err)
		}

	case []int:
		array, ok := actual.(*object.Array)
		if !ok {
			t.Errorf("object not Array: %T (%+v)", actual, actual)
			return
		}

		if len(array.Elements) != len(expected) {
			t.Errorf("wrong num of elements. want=%d, got=%d",
				len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	case map[object.HashKey]int64:
		hash, ok := actual.(*object.Hash)
		if !ok {
			t.Errorf("object is not Hash. got=%T (%+v)", actual, actual)
			return
		}

		if len(hash.Pairs) != len(expected) {
			t.Errorf("hash has wrong number of Pairs. want=%d, got=%d",
				len(expected), len(hash.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := hash.Pairs[expectedKey]
			if !ok {
				t.Errorf("no pair for given key in Pairs")
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	case *object.Error:
		errObj, ok := actual.(*object.Error)
		if !ok {
			t.Errorf("object is not Error: %T (%+v)", actual, actual)
			return
		}
		if errObj.Message != expected.Message {
			t.Errorf("wrong error message. expected=%q, got=%q",
				expected.Message, errObj.Message)
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
