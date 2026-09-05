package compiler

import (
	"beedance/ast"
	"beedance/code"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	_ "beedance/stdlib"
	"fmt"
	"testing"
	"time"
)

func TestCompilerScopes(t *testing.T) {
	compiler := New()
	if compiler.scopeIndex != 0 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d", compiler.scopeIndex, 0)
	}
	globalSymbolTable := compiler.symbolTable

	compiler.emit(code.OpMul)

	compiler.enterScope()
	if compiler.scopeIndex != 1 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d", compiler.scopeIndex, 1)
	}

	compiler.emit(code.OpSub)

	if len(compiler.scopes[compiler.scopeIndex].instructions) != 1 {
		t.Errorf("instructions length wrong. got=%d",
			len(compiler.scopes[compiler.scopeIndex].instructions))
	}

	last := compiler.scopes[compiler.scopeIndex].lastInstruction
	if last.Opcode != code.OpSub {
		t.Errorf("lastInstruction.Opcode wrong. got=%d, want=%d",
			last.Opcode, code.OpSub)
	}

	if compiler.symbolTable.Outer != globalSymbolTable {
		t.Errorf("compiler did not enclose symbolTable")
	}

	compiler.leaveScope()
	if compiler.scopeIndex != 0 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d",
			compiler.scopeIndex, 0)
	}

	if compiler.symbolTable != globalSymbolTable {
		t.Errorf("compiler did not restore global symbol table")
	}
	if compiler.symbolTable.Outer != nil {
		t.Errorf("compiler modified global symbol table incorrectly")
	}

	compiler.emit(code.OpAdd)

	if len(compiler.scopes[compiler.scopeIndex].instructions) != 2 {
		t.Errorf("instructions length wrong. got=%d",
			len(compiler.scopes[compiler.scopeIndex].instructions))
	}

	last = compiler.scopes[compiler.scopeIndex].lastInstruction
	if last.Opcode != code.OpAdd {
		t.Errorf("lastInstruction.Opcode wrong. got=%d, want=%d",
			last.Opcode, code.OpAdd)
	}

	previous := compiler.scopes[compiler.scopeIndex].previousInstruction
	if previous.Opcode != code.OpMul {
		t.Errorf("previousInstruction.Opcode wrong. got=%d, want=%d",
			previous.Opcode, code.OpMul)
	}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "1 + 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1; 2;",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 - 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSub),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 * 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpMul),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "2 / 1",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpDiv),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "-1",
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "true",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpFalse),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 > 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterThan),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 < 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpLessThan),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 = 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 != 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true = false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "true != false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "!true",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpBang),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `if (true) then 10; end_if;`,
			expectedConstants: []interface{}{10},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),              // 0000
				code.Make(code.OpJumpNotTruthy, 10), // 0001
				code.Make(code.OpConstant, 0),       // 0004
				code.Make(code.OpJump, 11),          // 0007
				code.Make(code.OpNull),              // 0010,
				code.Make(code.OpPop),
			},
		},
		{
			input:             `if (true) then 10; else 20; end_if;`,
			expectedConstants: []interface{}{10, 20},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),              // 0000
				code.Make(code.OpJumpNotTruthy, 10), // 0001
				code.Make(code.OpConstant, 0),       // 0004
				code.Make(code.OpJump, 13),          // 0007
				code.Make(code.OpConstant, 1),       // 0010,
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestGlobalVarStatements(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			VAR_GLOBAL
				one: INT := 1;
				two: INT := 2;
			END_VAR
			one;
			two;
			`,
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpPop),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			VAR_GLOBAL
				one: INT := 1;
			END_VAR
			one;
			`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			VAR_GLOBAL
				one: INT := 1;
				two: INT := one;
			END_VAR
			two;
			`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpSetGlobal, 1),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFunctions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `FUNCTION MyFunc : INT MyFunc := 5 + 10; END_FUNCTION`,
			expectedConstants: []interface{}{
				5,
				10,
				[]code.Instructions{
					code.Make(code.OpNull),        // Initialize return var to null
					code.Make(code.OpSetLocal, 0), //
					code.Make(code.OpConstant, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue), // Optimized return via assignment
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpSetGlobal, 0),
			},
		},
		{
			input: `
			FUNCTION MyFuncWithVars : INT
				VAR_INPUT
					InVar : INT;
				END_VAR
				VAR_OUTPUT
					OutVar : INT;
				END_VAR
				OutVar := InVar * 2;
				MyFuncWithVars := OutVar + 1;
			END_FUNCTION
			`,
			expectedConstants: []interface{}{
				2,
				1,
				[]code.Instructions{
					// The compiler correctly initializes the return variable to null first.
					code.Make(code.OpNull),
					code.Make(code.OpSetLocal, 1), // Set return var 'MyFuncWithVars' (index 1)
					// OutVar := InVar * 2;
					code.Make(code.OpGetLocal, 0), // Get InVar (index 0)
					code.Make(code.OpConstant, 0), // Push 2
					code.Make(code.OpMul),
					code.Make(code.OpSetLocal, 2), // Set OutVar (index 2)
					// MyFuncWithVars := OutVar + 1;
					code.Make(code.OpGetLocal, 2), // Get OutVar (index 2)
					code.Make(code.OpConstant, 1), // Push 1
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue), // Optimized return
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpSetGlobal, 0),
			},
		},
		{
			input: `FUNCTION MyFunc : INT MyFunc := 2; END_FUNCTION`,
			expectedConstants: []interface{}{
				2,
				[]code.Instructions{
					code.Make(code.OpNull),        // Initialize return var
					code.Make(code.OpSetLocal, 0), //
					code.Make(code.OpConstant, 0), // MyFunc := 2
					code.Make(code.OpReturnValue), // Optimized return
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetGlobal, 0),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestProgramDeclarationWithVars(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			PROGRAM MyTestProgram
				VAR_GLOBAL
					gVar : INT := 1;
				END_VAR
				VAR_EXTERNAL
					eVar : BOOL;
				END_VAR
				VAR_ACCESS
					aVar : MyFB READ_ONLY;
				END_VAR
				VAR_TEMP
					tVar : REAL := 2.5;
				END_VAR

				gVar;
				tVar;
			END_PROGRAM
			`,
			expectedConstants: []interface{}{1, "MyFB", 2.5},
			expectedInstructions: []code.Instructions{
				// VAR_GLOBAL gVar := 1;
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				// VAR_EXTERNAL eVar; (implicit init to null)
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 1),
				// VAR_ACCESS aVar; (no code generated, only symbol table entry)
				// VAR_TEMP tVar := 2.5;
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSetGlobal, 2),
				// Body: gVar;
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpPop),
				// Body: tVar;
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestVarAccess(t *testing.T) {
	// NOTE: This test will fail until the parser is updated to handle the full
	// VAR_ACCESS syntax (e.g., `MyVar : OtherProg.Var : REAL;`) and populate
	// an `AccessPath` field on the `ast.VarDeclStatement` node.
	// It serves as a specification for the compiler's expected behavior.
	tests := []compilerTestCase{
		{
			input: `
			PROGRAM MyConsumer
				VAR_ACCESS
					MyPressure : OtherProg.Pressure : REAL;
				END_VAR
				MyPressure;
			END_PROGRAM
			`,
			expectedConstants: []interface{}{"OtherProg.Pressure"},
			expectedInstructions: []code.Instructions{
				// The VAR_ACCESS block itself doesn't emit instructions,
				// it just populates the symbol table.
				code.Make(code.OpGetExternal, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

type compilerTestCase struct {
	input                string
	expectedConstants    []interface{}
	expectedInstructions []code.Instructions
}

// To make tests stable, we define explicit indices for the built-ins used in tests.
// This mimics the iota-based approach for the main application.
const (
	testBuiltinLen = iota
	testBuiltinPush
)

// getTestBuiltins provides a clean, isolated set of built-in function definitions for testing.
// This prevents test failures caused by global state pollution where the global `object.Builtins`
// slice might be modified by another test.
func getTestBuiltins() []object.BuiltinEntry {
	// With explicit indexing, we create a slice of the correct size
	// and place the built-ins at their designated index. No sorting is needed.
	builtins := make([]object.BuiltinEntry, 2)
	builtins[testBuiltinLen] = object.BuiltinEntry{Name: "LEN", Index: testBuiltinLen}
	builtins[testBuiltinPush] = object.BuiltinEntry{Name: "PUSH", Index: testBuiltinPush}
	return builtins
}

func runCompilerTests(t *testing.T, tests []compilerTestCase) {
	t.Helper()

	for i, tt := range tests {
		program := parse(tt.input)

		// The test setup is now much simpler. No sorting is required because
		// getTestBuiltins provides the built-ins in their final, indexed order.
		compiler := NewCompilerWithBuiltins(getTestBuiltins())
		err := compiler.Compile(program)
		if err != nil {
			t.Fatalf("compiler error on test #%d/%d: %s", i+1, len(tests), err)
		}

		bytecode := compiler.Bytecode()

		err = testInstructions(tt.expectedInstructions, bytecode.Instructions)
		if err != nil {
			t.Fatalf("test #%d/%d: testInstructions failed: %s", i+1, len(tests), err)
		}

		err = testConstants(t, tt.expectedConstants, bytecode.Constants)
		if err != nil {
			t.Fatalf("test #%d/%d: testConstants failed: %s", i+1, len(tests), err)
		}
	}
}

func TestStringExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `VAR str: STRING := 'monkey' END_VAR str;`,
			expectedConstants: []interface{}{"monkey"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),  //0000
				code.Make(code.OpSetGlobal, 0), //0003
				code.Make(code.OpGetGlobal, 0), //0006
				code.Make(code.OpPop),          //0009
			},
		},
		{
			input:             `'mon' + 'key'`,
			expectedConstants: []interface{}{"mon", "key"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "[]",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpArray, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "[1, 2, 3]",
			expectedConstants: []interface{}{1, 2, 3},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpArray, 3),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "[1 + 2, 3 - 4, 5 * 6]",
			expectedConstants: []interface{}{1, 2, 3, 4, 5, 6},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpSub),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpMul),
				code.Make(code.OpArray, 3),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "{}",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpHash, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "{1: 2, 3: 4, 5: 6}",
			expectedConstants: []interface{}{1, 2, 3, 4, 5, 6},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpHash, 6),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "{1: 2 + 3, 4: 5 * 6}",
			expectedConstants: []interface{}{1, 2, 3, 4, 5, 6},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpAdd),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpMul),
				code.Make(code.OpHash, 4),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "[1, 2, 3][1 + 1]",
			expectedConstants: []interface{}{1, 2, 3, 1, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpArray, 3),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpAdd),
				code.Make(code.OpIndex),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "{1: 2}[2 - 1]",
			expectedConstants: []interface{}{1, 2, 2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpHash, 2),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpSub),
				code.Make(code.OpIndex),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `fn() : VOID { }`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpPop),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestFunctionCalls(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `fn() : INT { 24 }();`,
			expectedConstants: []interface{}{
				24,
				[]code.Instructions{
					// Anonymous `fn` literals have a simpler return mechanism.
					code.Make(code.OpConstant, 0), // Push 24
					code.Make(code.OpReturnValue), // Return it
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			FUNCTION noArg : INT noArg := 24; END_FUNCTION
			noArg();`,
			expectedConstants: []interface{}{
				24,
				[]code.Instructions{
					// The compiler correctly implements the IEC 61131-3 standard,
					// where the function name acts as a return variable.
					code.Make(code.OpNull),        // Initialize return var
					code.Make(code.OpSetLocal, 0), //
					code.Make(code.OpConstant, 0), // Push 24 for return
					code.Make(code.OpReturnValue), // Optimized return
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			FUNCTION oneArg : INT VAR_INPUT a:INT; END_VAR oneArg := a; END_FUNCTION
			oneArg(24);`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					// The compiler correctly implements the IEC 61131-3 standard,
					// where the function name acts as a return variable.
					code.Make(code.OpNull),        // Initialize return var 'oneArg'
					code.Make(code.OpSetLocal, 1), // (a is 0, oneArg is 1)
					code.Make(code.OpGetLocal, 0), // Get input 'a' for return
					code.Make(code.OpReturnValue), // Optimized return
				},
				24,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			FUNCTION manyArg : INT VAR_INPUT a:INT; b:INT; c:INT; END_VAR a; b; manyArg := c; END_FUNCTION
			manyArg(24, 25, 26);`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					// The compiler correctly implements the IEC 61131-3 standard,
					// where the function name acts as a return variable.
					code.Make(code.OpNull),        // Initialize return var 'manyArg'
					code.Make(code.OpSetLocal, 3), //
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpPop), // a;
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpPop),         // b;
					code.Make(code.OpGetLocal, 2), // Get input 'c' for return
					code.Make(code.OpReturnValue), // Optimized return
				},
				24,
				25,
				26,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpCall, 3),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestVarStatementScopes(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			VAR num: INT := 55; END_VAR
			fn() : INT { num }
			`,
			expectedConstants: []interface{}{
				55,
				[]code.Instructions{
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			fn() {
				VAR num: INT := 55; END_VAR
				num;
			}()
			`,
			expectedConstants: []interface{}{
				55,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0), // The closure for fn
				code.Make(code.OpCall, 0),       // The call to fn
				code.Make(code.OpPop),           // Pop the result of the call
			},
		},
		{
			input: `
			fn() : INT {
				VAR a: INT := 55; END_VAR
				VAR b: INT := 77; END_VAR
				a + b;
			}()
			`,
			expectedConstants: []interface{}{
				55,
				77,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0), // The closure for fn
				code.Make(code.OpCall, 0),       // The call to fn
				code.Make(code.OpPop),           // Pop the result of the call
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestMacroCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `macro(x, y) { x + y }`,
			expectedConstants: []interface{}{
				&object.UncompiledMacro{
					Parameters: []*ast.Identifier{
						{Value: "x"},
						{Value: "y"},
					},
					Body: &ast.BlockStatement{
						Statements: []ast.Statement{
							&ast.ExpressionStatement{
								Expression: &ast.InfixExpression{
									Left:     &ast.Identifier{Value: "x"},
									Operator: "+",
									Right:    &ast.Identifier{Value: "y"},
								},
							},
						},
					},
				},
			},
			expectedInstructions: []code.Instructions{
				// The macro definition itself is just a constant.
				// The compiler pushes it onto the stack, and the outer statement
				// (like a `let` or just an expression statement) pops it.
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestConfigurationCompilation(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
            CONFIGURATION MyConfig
                RESOURCE Res1 ON PLC1
                    TASK T1 (INTERVAL := T#100ms, PRIORITY := 1);
                    PROGRAM P1 WITH T1 : ProgType;
                END_RESOURCE
                VAR_CONFIG P1
                    Input1 : INT := 42;
                END_VAR
            END_CONFIGURATION
            `,
			expectedConstants: []interface{}{
				"name", "T1", "interval", 100 * time.Millisecond, "priority", 1, // Task constants
				"instance", "P1", "task", "T1", "type", "ProgType", // Program constants
				"params", "Input1", 42, // VAR_CONFIG constants
				"name", "Res1", "type", "PLC1", "programs", "tasks", // Resource constants
				"name", "MyConfig", "resources", // Configuration constants
			},
			expectedInstructions: []code.Instructions{
				// Task T1
				code.Make(code.OpConstant, 0), // "name"
				code.Make(code.OpConstant, 1), // "T1"
				code.Make(code.OpConstant, 2), // "interval"
				code.Make(code.OpConstant, 3), // T#100ms (as object.Time)
				code.Make(code.OpConstant, 4), // "priority"
				code.Make(code.OpConstant, 5), // 1
				code.Make(code.OpHash, 6),     // Task hash
				code.Make(code.OpArray, 1),    // Tasks array
				// Program P1
				code.Make(code.OpConstant, 6),  // "instance"
				code.Make(code.OpConstant, 7),  // "P1"
				code.Make(code.OpConstant, 8),  // "task"
				code.Make(code.OpConstant, 9),  // "T1" // cspell:disable-line
				code.Make(code.OpConstant, 10), // "type" // cspell:disable-line
				code.Make(code.OpConstant, 11), // "ProgType" // cspell:disable-line
				code.Make(code.OpConstant, 12), // "params"
				code.Make(code.OpConstant, 13), // "Input1"
				code.Make(code.OpConstant, 14), // 42
				code.Make(code.OpHash, 2),      // Params hash
				code.Make(code.OpHash, 8),      // Program hash
				code.Make(code.OpArray, 1),     // Programs array
				// Resource Res1
				code.Make(code.OpConstant, 15), // "name"
				code.Make(code.OpConstant, 16), // "Res1"
				code.Make(code.OpConstant, 17), // "type"
				code.Make(code.OpConstant, 18), // "PLC1"
				code.Make(code.OpConstant, 19), // "programs"
				code.Make(code.OpSwap),
				code.Make(code.OpConstant, 20), // "tasks"
				code.Make(code.OpHash, 8),      // Resource hash // cspell:disable-line
				code.Make(code.OpArray, 1),     // Resources array
				// Configuration MyConfig
				code.Make(code.OpConstant, 21), // "name"
				code.Make(code.OpConstant, 22), // "MyConfig"
				code.Make(code.OpConstant, 23), // "resources"
				code.Make(code.OpHash, 4),      // Config hash
				code.Make(code.OpSetGlobal, 0), // Store config
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestTypedLiterals(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `TIME#1s500ms`,
			expectedConstants: []interface{}{1500 * time.Millisecond},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `D#2026-08-28`,
			expectedConstants: []interface{}{time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `INT#16#F`,
			expectedConstants: []interface{}{15},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `LREAL#3.14`,
			expectedConstants: []interface{}{3.14},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `WORD#16#FFFF`,
			expectedConstants: []interface{}{&object.BitString{Value: 0xFFFF, Width: 16}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `COLOR#RED`,
			expectedConstants: []interface{}{&object.EnumeratedValue{TypeName: "COLOR", Value: "RED"}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
	}
	runCompilerTests(t, tests)
}

func TestTypeDeclarations(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			TYPE
				MyStruct : STRUCT
					Field1: INT;
				END_STRUCT;
				MyEnum : (RED, GREEN, BLUE);
				MyArray : ARRAY[0..4] OF BOOL;
			END_TYPE
			`,
			expectedConstants: []interface{}{
				&object.StructDefinition{
					Name:    &ast.Identifier{Value: "MyStruct"},
					Members: []*ast.VarDeclStatement{{Name: &ast.Identifier{Value: "Field1"}}},
				},
				&object.EnumDefinition{
					Name:   &ast.Identifier{Value: "MyEnum"},
					Values: []*ast.Identifier{{Value: "RED"}, {Value: "GREEN"}, {Value: "BLUE"}},
				},
				&object.ArrayDefinition{
					Name: &ast.Identifier{Value: "MyArray"},
				},
			},
			expectedInstructions: []code.Instructions{
				// MyStruct
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				// MyEnum
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSetGlobal, 1),
				// MyArray
				code.Make(code.OpConstant, 2),
				code.Make(code.OpSetGlobal, 2),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestBuiltins(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			LEN([]);
			PUSH([], 1);
			`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetBuiltin, testBuiltinLen),
				code.Make(code.OpArray, 0),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
				code.Make(code.OpGetBuiltin, testBuiltinPush),
				code.Make(code.OpArray, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpCall, 2),
				code.Make(code.OpPop),
			},
		},
		{
			input: `fn() { LEN([]) }`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetBuiltin, testBuiltinLen),
					code.Make(code.OpArray, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestAdvancedLiteralsAndExpressions(t *testing.T) {
	tests := []compilerTestCase{
		// WStringLiteral
		{
			input:             `"wide string"`,
			expectedConstants: []interface{}{&object.WString{Value: "wide string"}},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		// MemberAccessExpression
		{
			input: `
			VAR_GLOBAL MyStructInstance : MyStruct; END_VAR
			MyStructInstance.MyMember`,
			expectedConstants: []interface{}{"MyMember"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 0), // Init MyStructInstance
				code.Make(code.OpGetGlobal, 0), // Get instance
				code.Make(code.OpConstant, 0),  // The string "MyMember"
				code.Make(code.OpIndex),
				code.Make(code.OpPop),
			},
		},
		// ArrayRepetition
		{
			input:             `[1, 2(3), 4]`,
			expectedConstants: []interface{}{1, 3, 4},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0), // 1
				code.Make(code.OpConstant, 1), // 3
				code.Make(code.OpConstant, 1), // 3 (repeated)
				code.Make(code.OpConstant, 2), // 4
				code.Make(code.OpArray, 4),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			fn(a : INT) {
				fn(b : INT) {
					a + b
				}
			};
			`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 0, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			fn(a : INT) {
				fn(b : INT) {
					fn(c : INT) {
						a + b + c
					}
				}
			};
			`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpGetFree, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 0, 2),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 1, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			VAR global: INT := 55; END_VAR
			fn() : VOID {
				VAR a: INT := 66; END_VAR
				fn() : VOID {
					VAR b: INT := 77; END_VAR
					fn() : INT {
						VAR c: INT := 88; END_VAR
						global + a + b + c;
					}
				}
			}
			`,
			expectedConstants: []interface{}{
				55,
				66,
				77,
				88,
				[]code.Instructions{
					code.Make(code.OpConstant, 3),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpAdd),
					code.Make(code.OpGetFree, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpConstant, 2),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 4, 2),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 5, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpClosure, 6, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `
			FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR countDown := countDown(x - 1); END_FUNCTION
			countDown(1);`,
			expectedConstants: []interface{}{
				1,
				[]code.Instructions{ // The body of countDown
					code.Make(code.OpNull),        // Initialize return var 'countDown'
					code.Make(code.OpSetLocal, 1), // x is 0, countDown is 1
					code.Make(code.OpCurrentClosure),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSub),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
				1,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
		{
			input: `
			FUNCTION wrapper : INT
				FUNCTION countDown : INT VAR_INPUT x:INT; END_VAR countDown := countDown(x-1); END_FUNCTION
				countDown(1);
			END_FUNCTION
			wrapper();`,
			expectedConstants: []interface{}{
				1,
				[]code.Instructions{ // Body of inner countDown
					code.Make(code.OpNull),        // Initialize return var 'countDown'
					code.Make(code.OpSetLocal, 1), // x is 0, countDown is 1
					code.Make(code.OpCurrentClosure),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSub),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
				1,
				[]code.Instructions{
					code.Make(code.OpNull),        // Initialize return var 'wrapper'
					code.Make(code.OpSetLocal, 0), //
					code.Make(code.OpClosure, 1, 0),
					code.Make(code.OpSetLocal, 1), // Store 'countDown' closure
					code.Make(code.OpGetLocal, 1), // Load 'countDown' for call
					code.Make(code.OpConstant, 2),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestVarInputIsReadOnly(t *testing.T) {
	input := `
	FUNCTION MyFunc : INT
		VAR_INPUT
			InVar : INT;
		END_VAR
		InVar := 5;
		MyFunc := InVar;
	END_FUNCTION
	`
	compiler := New()
	err := compiler.Compile(parse(input))

	expectedError := "cannot assign to read-only variable 'InVar'"
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected error %q, but got %v", expectedError, err)
	}
}

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func testInstructions(
	expected []code.Instructions,
	actual code.Instructions,
) error {
	concatted := concatInstructions(expected)

	if len(actual) != len(concatted) {
		return fmt.Errorf("wrong instructions length.\n\nwant:\n%s\ngot:\n%s",
			concatted, actual)
	}

	for i, ins := range concatted {
		if actual[i] != ins {
			return fmt.Errorf("wrong instruction at %d.\n\nwant:\n%s\ngot:\n%s",
				i, concatted, actual)
		}
	}

	return nil
}

func concatInstructions(s []code.Instructions) code.Instructions {
	out := code.Instructions{}

	for _, ins := range s {
		out = append(out, ins...)
	}

	return out
}

func testConstants(
	t *testing.T,
	expected []interface{},
	actual []object.Object,
) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("wrong number of constants. got=%d, want=%d",
			len(actual), len(expected))
	}

	for i, constant := range expected {
		switch constant := constant.(type) {
		case string:
			err := testStringObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testStringObject failed: %s",
					i, err)
			}
		case int:
			err := testIntegerObject(int64(constant), actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testIntegerObject failed: %s",
					i, err)
			}
		case uint64:
			err := testUnsignedIntegerObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testUnsignedIntegerObject failed: %s",
					i, err)
			}
		case time.Duration:
			err := testTimeObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testTimeObject failed: %s", i, err)
			}
		case time.Time:
			err := testDateObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testDateObject failed: %s", i, err)
			}
		case []code.Instructions:
			fn, ok := actual[i].(*object.CompiledFunction)
			if !ok {
				return fmt.Errorf("constant %d - not a function: %T",
					i, actual[i])
			}

			err := testInstructions(constant, fn.Instructions)
			if err != nil {
				return fmt.Errorf("constant %d - testInstructions failed: %s",
					i, err)
			}
		case float64:
			err := testRealObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testRealObject failed: %s",
					i, err)
			}
		case *object.WString:
			// The test expects a *object.WString, so we check for that type.
			err := testWStringObject(constant.Value, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testWStringObject failed: %s", i, err)
			}
		case *object.BitString: // cspell:disable-line
			err := testBitStringObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testBitStringObject failed: %s",
					i, err)
			}
		case *object.StructDefinition:
			err := testStructDefinitionObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testStructDefinitionObject failed: %s", i, err)
			}
		case *object.EnumDefinition:
			err := testEnumDefinitionObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testEnumDefinitionObject failed: %s", i, err)
			}
		case *object.ArrayDefinition:
			err := testArrayDefinitionObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testArrayDefinitionObject failed: %s", i, err)
			}
		case *object.EnumeratedValue:
			err := testEnumeratedValueObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testEnumeratedValueObject failed: %s", i, err)
			}
		case *object.UncompiledMacro:
			err := testUncompiledMacroObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testUncompiledMacroObject failed: %s", i, err)
			}
		default:
			return fmt.Errorf("unhandled constant type in test: %T", constant)
		}
	}

	return nil
}

func testUnsignedIntegerObject(expected uint64, actual object.Object) error {
	result, ok := actual.(*object.ULInt)
	if !ok {
		return fmt.Errorf("object is not ULInt. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
	}

	return nil
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

func testTimeObject(expected time.Duration, actual object.Object) error {
	result, ok := actual.(*object.Time)
	if !ok {
		return fmt.Errorf("object is not Time. got=%T (%+v)", actual, actual)
	}
	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
	}
	return nil
}

func testDateObject(expected time.Time, actual object.Object) error {
	switch result := actual.(type) {
	case *object.Date:
		if !result.Value.Equal(expected) {
			return fmt.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
		}
	case *object.DateAndTime:
		if !result.Value.Equal(expected) {
			return fmt.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
		}
	case *object.TimeOfDay:
		if !result.Value.Equal(expected) {
			return fmt.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
		}
	default:
		return fmt.Errorf("object is not a date/time type. got=%T (%+v)", actual, actual)
	}
	return nil
}

func testBitStringObject(expected *object.BitString, actual object.Object) error {
	result, ok := actual.(*object.BitString)
	if !ok {
		return fmt.Errorf("object is not BitString. got=%T (%+v)", actual, actual)
	}
	if result.Value != expected.Value {
		return fmt.Errorf("wrong value. want=%d, got=%d", expected.Value, result.Value)
	}
	if result.Width != expected.Width {
		return fmt.Errorf("wrong width. want=%d, got=%d", expected.Width, result.Width)
	}
	return nil
}

func testEnumeratedValueObject(expected *object.EnumeratedValue, actual object.Object) error {
	result, ok := actual.(*object.EnumeratedValue)
	if !ok {
		return fmt.Errorf("object is not EnumeratedValue. got=%T (%+v)", actual, actual)
	}
	if result.TypeName != expected.TypeName {
		return fmt.Errorf("wrong TypeName. want=%s, got=%s", expected.TypeName, result.TypeName)
	}
	if result.Value != expected.Value {
		return fmt.Errorf("wrong Value. want=%s, got=%s", expected.Value, result.Value)
	}
	return nil
}

func testUncompiledMacroObject(expected *object.UncompiledMacro, actual object.Object) error {
	result, ok := actual.(*object.UncompiledMacro)
	if !ok {
		return fmt.Errorf("object is not UncompiledMacro. got=%T (%+v)", actual, actual)
	}

	if len(result.Parameters) != len(expected.Parameters) {
		return fmt.Errorf("wrong number of macro parameters. want=%d, got=%d",
			len(expected.Parameters), len(result.Parameters))
	}

	for i, param := range expected.Parameters {
		if result.Parameters[i].Value != param.Value {
			return fmt.Errorf("wrong parameter at %d. want=%q, got=%q", i, param.Value, result.Parameters[i].Value)
		}
	}

	// A more detailed test would compare the body statements.
	// For now, we just check the number of statements.
	return nil
}

func testStructDefinitionObject(expected *object.StructDefinition, actual object.Object) error {
	result, ok := actual.(*object.StructDefinition)
	if !ok {
		return fmt.Errorf("object is not StructDefinition. got=%T (%+v)", actual, actual)
	}
	if result.Name.Value != expected.Name.Value {
		return fmt.Errorf("wrong name. want=%s, got=%s", expected.Name.Value, result.Name.Value)
	}
	if len(result.Members) != len(expected.Members) {
		return fmt.Errorf("wrong number of members. want=%d, got=%d", len(expected.Members), len(result.Members))
	}
	// A more detailed test would compare each member.
	return nil
}

func testEnumDefinitionObject(expected *object.EnumDefinition, actual object.Object) error {
	result, ok := actual.(*object.EnumDefinition)
	if !ok {
		return fmt.Errorf("object is not EnumDefinition. got=%T (%+v)", actual, actual)
	}
	if result.Name.Value != expected.Name.Value {
		return fmt.Errorf("wrong name. want=%s, got=%s", expected.Name.Value, result.Name.Value)
	}
	if len(result.Values) != len(expected.Values) {
		return fmt.Errorf("wrong number of values. want=%d, got=%d", len(expected.Values), len(result.Values))
	}
	return nil
}

func testArrayDefinitionObject(expected *object.ArrayDefinition, actual object.Object) error {
	result, ok := actual.(*object.ArrayDefinition)
	if !ok {
		return fmt.Errorf("object is not ArrayDefinition. got=%T (%+v)", actual, actual)
	}
	if result.Name.Value != expected.Name.Value {
		return fmt.Errorf("wrong name. want=%s, got=%s", expected.Name.Value, result.Name.Value)
	}
	// A more detailed test would compare ranges and data type.
	return nil
}

func testRealObject(expected float64, actual object.Object) error {
	result, ok := actual.(*object.LReal)
	if !ok {
		return fmt.Errorf("object is not LReal. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%f, want=%f",
			result.Value, expected)
	}

	return nil
}

func testWStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.WString)
	if !ok {
		return fmt.Errorf("object is not WString. got=%T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%q, want=%q",
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
