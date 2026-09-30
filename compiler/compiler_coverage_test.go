/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"strings"
	"testing"

	"beedance/ast"
	"beedance/code"
)

// checkCompiles compiles each input and fails on any error. The behaviour
// of the compiled code is checked by the VM tests.
func checkCompiles(t *testing.T, inputs []string) {
	t.Helper()
	for _, input := range inputs {
		if err := New().Compile(parse(t, input)); err != nil {
			t.Errorf("%s:\n  unexpected compiler error: %s", input, err)
		}
	}
}

// checkCompileErrors compiles each input and expects an error containing want.
func checkCompileErrors(t *testing.T, cases []struct{ input, want string }) {
	t.Helper()
	for _, c := range cases {
		err := New().Compile(parse(t, c.input))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", c.input, c.want, err)
		}
	}
}

// hasOpcode reports whether ins contains op at an instruction boundary.
func hasOpcode(ins code.Instructions, op code.Opcode) bool {
	for i := 0; i < len(ins); {
		def, err := code.Lookup(ins[i])
		if err != nil {
			return false
		}
		if code.Opcode(ins[i]) == op {
			return true
		}
		_, read := code.ReadOperands(def, ins[i+1:])
		i += 1 + read
	}
	return false
}

func TestCompileProgramPhases(t *testing.T) {
	compileProgram := func(input string) (*CompiledProgram, error) {
		prog := parse(t, input).Statements[0].(*ast.ProgramDeclaration)
		return NewWithState(NewSymbolTable(), nil, nil, nil).CompileProgram(prog)
	}

	// Inputs, outputs and in-outs are variables of a program run on its own.
	cp, err := compileProgram("PROGRAM Pg VAR_INPUT i : INT := 2; END_VAR VAR_OUTPUT o : INT; END_VAR VAR_IN_OUT io : INT; END_VAR VAR a : INT; END_VAR VAR_TEMP t1 : INT := 3; END_VAR o := i + t1 + io + a; END_PROGRAM")
	if err != nil {
		t.Fatalf("CompileProgram: %s", err)
	}
	if !hasOpcode(cp.InitBytecode.Instructions, code.OpSetGlobal) {
		t.Errorf("initialization code sets no variables:\n%s", cp.InitBytecode.Instructions)
	}
	// An assignment leaves nothing on the stack, so the cyclic code pushes NULL.
	if ins := cp.CyclicBytecode.Instructions; code.Opcode(ins[len(ins)-1]) != code.OpNull {
		t.Errorf("cyclic code should end with OpNull:\n%s", ins)
	}

	// A final expression's value is left on the stack.
	cp, err = compileProgram("PROGRAM Pg VAR a : INT; END_VAR a + 1; END_PROGRAM")
	if err != nil {
		t.Fatalf("CompileProgram: %s", err)
	}
	if hasOpcode(cp.CyclicBytecode.Instructions, code.OpPop) {
		t.Errorf("the final expression's value should not be popped:\n%s", cp.CyclicBytecode.Instructions)
	}

	// An empty body pushes NULL.
	cp, err = compileProgram("PROGRAM Pg VAR a : INT; END_VAR END_PROGRAM")
	if err != nil {
		t.Fatalf("CompileProgram: %s", err)
	}
	if ins := cp.CyclicBytecode.Instructions; len(ins) != 1 || code.Opcode(ins[0]) != code.OpNull {
		t.Errorf("an empty body should compile to OpNull, got:\n%s", ins)
	}

	for _, c := range []struct{ input, want string }{
		{"PROGRAM Pg VAR a : INT := nope; END_VAR END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR_TEMP t1 : INT := nope; END_VAR END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR a := nope; END_PROGRAM", "undefined variable nope"},
	} {
		if _, err := compileProgram(c.input); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: expected an error containing %q, got %v", c.input, c.want, err)
		}
	}
}

func TestNewWithStateSharesState(t *testing.T) {
	symbols := NewSymbolTable()
	first := NewWithState(symbols, nil, nil, nil)
	if err := first.Compile(parse(t, "VAR shared : INT := 1; END_VAR")); err != nil {
		t.Fatalf("first compile: %s", err)
	}
	// A second compiler with the same symbol table and constants sees the variable.
	second := NewWithState(symbols, first.Bytecode().Constants, nil, nil)
	if err := second.Compile(parse(t, "shared + 1;")); err != nil {
		t.Fatalf("second compile: %s", err)
	}
}

func TestCompileSuccessPaths(t *testing.T) {
	checkCompiles(t, []string{
		// Located variables, with and without an initial value.
		"VAR x AT %IX0.0 : BOOL; END_VAR x;",
		"VAR x AT %QW0 : INT := 5; END_VAR x;",
		// Array elements, including outputs written to them.
		"VAR a : ARRAY[0..2] OF INT; END_VAR a[1] := 5; a[1];",
		"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 3; F := 1; END_FUNCTION VAR a : ARRAY[0..1] OF INT; END_VAR F(o => a[0]);",
		"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 3; F := 1; END_FUNCTION VAR p : Pt; END_VAR F(o => p.px);",
		"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 3; F := 1; END_FUNCTION FUNCTION G : INT VAR a : ARRAY[0..1] OF INT; END_VAR F(o => a[1]); G := a[1]; END_FUNCTION",
		// RETURN in a function returns its result and outputs.
		"FUNCTION F : INT RETURN; END_FUNCTION F();",
		"FUNCTION F : INT RETURN 5; END_FUNCTION F();",
		"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 2; RETURN 5; END_FUNCTION VAR r2 : INT; END_VAR F(o => r2);",
		// Properties read and written by name inside methods.
		"FUNCTION_BLOCK Fb PROPERTY Pr : INT GET Pr := 3; END_GET SET END_SET END_PROPERTY METHOD M : INT M := Pr; END_METHOD END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Fb VAR v : INT; END_VAR PROPERTY Pr : INT GET Pr := v; END_GET SET v := value; END_SET END_PROPERTY METHOD M : INT Pr := 9; THIS.Pr := 4; M := v + Pr + 1; END_METHOD END_FUNCTION_BLOCK",
		"FUNCTION_BLOCK Dv METHOD M : INT M := THIS^.Q(); END_METHOD METHOD Q : INT Q := 4; END_METHOD END_FUNCTION_BLOCK",
		// Literals and expressions typed without a declared variable.
		"1.5;", "BYTE#16#FF;", "TYPE Color : (Red, Green); END_TYPE Color#Green;",
		"VAR x : LREAL; END_VAR x := 1.5 + 2;",
		"VAR x : INT; END_VAR x := BYTE#16#F AND 1;",
		"TYPE Color : (Red, Green); END_TYPE VAR x : BOOL; END_VAR x := Color#Red = Color#Green;",
		"TYPE Pt : STRUCT px : INT; a : ARRAY[0..1] OF INT; END_STRUCT; END_TYPE VAR p : Pt; x : INT; END_VAR x := p.px + 1; x := p.a[0] + 1;",
		"FUNCTION G : INT G := 1; END_FUNCTION FUNCTION_BLOCK Fb METHOD M : INT M := 1; END_METHOD METHOD N END_METHOD END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR x := G() + 1; x := f.M() + 1;",
		// FOR defines an undeclared control variable.
		"VAR x : INT; END_VAR FOR x := 1 TO 3 BY 1 DO END_FOR x;",
		"FOR y := 1 TO 3 DO END_FOR y;",
		// Constant expressions as array bounds.
		"VAR a : ARRAY[0..(8 / 2)] OF INT; END_VAR a;",
		"VAR a : ARRAY[0..-1 + 3] OF INT; END_VAR a;",
		"VAR a : ARRAY[0..3] OF INT := [0(1), 2]; END_VAR",
		// Tasks without an interval or a priority.
		"CONFIGURATION C RESOURCE Res ON CPU TASK T1(PRIORITY := 1); END_RESOURCE END_CONFIGURATION",
		"CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := T#1s); END_RESOURCE END_CONFIGURATION",
		// A VAR_CONFIG entry with only a location.
		"PROGRAM Prg VAR c1 : BOOL; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prg; END_RESOURCE VAR_CONFIG Res.P1.c1 AT %IX0.0 : BOOL; END_VAR END_CONFIGURATION",
		// A program compiled as a statement declares its inputs, outputs and in-outs.
		"PROGRAM Pg VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR VAR_IN_OUT io : INT; END_VAR o := i; END_PROGRAM",
		// VAR_EXTERNAL refers to an existing global.
		"VAR_GLOBAL g : INT; END_VAR PROGRAM Pg VAR_EXTERNAL g : INT; END_VAR g := 1; END_PROGRAM",
		// Function block initializers and FB variables read in methods.
		"FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb := (v := 9); END_VAR",
		"FUNCTION_BLOCK Fb VAR v : INT; END_VAR METHOD M : INT M := v + 1; END_METHOD END_FUNCTION_BLOCK",
		// A subclass may read a PROTECTED member.
		"FUNCTION_BLOCK A VAR PROTECTED pv : INT; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK B EXTENDS A METHOD M : INT VAR a1 : A; END_VAR M := a1.pv; END_METHOD END_FUNCTION_BLOCK",
		// IL programs.
		"PROGRAM Pg VAR a : INT; END_VAR LD 1 ADD 2 ST a END_PROGRAM",
		"PROGRAM Pg VAR a : INT; END_VAR LD 1 NOT END_PROGRAM",
	})
}

func TestInitialValueSuccessPaths(t *testing.T) {
	checkCompiles(t, []string{
		// Defaults of every elementary type family.
		"VAR t : TIME; d : DATE; tod1 : TOD; dt1 : DT; b : WORD; END_VAR b;",
		// Integer constants take REAL and bit-string types.
		"VAR x : LREAL := 3; END_VAR x;",
		"VAR x : WORD := 5; END_VAR x;",
		// A structure type's own initializer, and a structure copied from another.
		"TYPE Pt : STRUCT px : INT; py : INT; END_STRUCT := (px := 3); END_TYPE VAR p : Pt; END_VAR p.px;",
		"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR q : Pt; p : Pt := q; END_VAR p.px;",
		// Array types, with and without an initial value.
		"TYPE Row : ARRAY[0..2] OF INT; END_TYPE VAR r1 : Row; END_VAR r1;",
		"TYPE Row : ARRAY[0..2] OF INT := [7]; END_TYPE VAR r1 : Row; END_VAR r1;",
		// Subranges start at their lower limit; a variable's value is checked when it runs.
		"TYPE Rng : INT(3..9); END_TYPE VAR rv : Rng; END_VAR rv;",
		"TYPE Rng : INT(3..9); END_TYPE VAR k : INT := 4; rv : Rng := k; END_VAR rv;",
		// Enumerations: a one-value type, and a value copied from another variable.
		"TYPE E0 : (Only); END_TYPE VAR e : E0; END_VAR e;",
		"TYPE Color : (Red, Green); END_TYPE VAR c2 : Color := Green; c : Color := c2; END_VAR c;",
		// Multi-dimensional arrays, and an array copied from another.
		"VAR a : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3, 4]]; END_VAR a;",
		"VAR a : ARRAY[0..1, 0..1] OF INT; END_VAR a;",
		"VAR b : ARRAY[0..1] OF INT := [5, 6]; a : ARRAY[0..1] OF INT := b; END_VAR a;",
		// Formal calls fill in omitted inputs, matching names case-insensitively.
		"FUNCTION F : INT VAR_INPUT a : INT := 1; b : INT := 2; END_VAR F := a * 10 + b; END_FUNCTION F(A := 5);",
		// Unknown types compile, taking no default.
		"FUNCTION F : Nope F := 1; END_FUNCTION",
		"FUNCTION_BLOCK Fb VAR v : Nope2; END_VAR METHOD M : Nope3 END_METHOD PROPERTY Pr : Nope3 GET END_GET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR",
	})
}

func TestCompileErrorPaths(t *testing.T) {
	checkCompileErrors(t, []struct{ input, want string }{
		// Errors inside nested declarations are reported.
		{"NAMESPACE N FUNCTION F : INT F := nope; END_FUNCTION END_NAMESPACE", "undefined variable nope"},
		{"TYPE Pt : STRUCT px : INT := nope; END_STRUCT; END_TYPE VAR p : Pt; END_VAR", "undefined variable nope"},
		{"FUNCTION F : INT F := nope; END_FUNCTION", "undefined variable nope"},
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR z : INT := nope; END_VAR M := 1; END_METHOD END_FUNCTION_BLOCK", "undefined variable nope"},
		// Inheritance.
		{"INTERFACE I EXTENDS Nope END_INTERFACE", "parent interface 'NOPE' not found"},
		{"FUNCTION_BLOCK X END_FUNCTION_BLOCK INTERFACE I EXTENDS X END_INTERFACE", "parent 'X' is not an interface"},
		{"FUNCTION_BLOCK Fb EXTENDS Nope END_FUNCTION_BLOCK", "parent function block 'NOPE' definition not found"},
		{"INTERFACE X END_INTERFACE FUNCTION_BLOCK Fb EXTENDS X END_FUNCTION_BLOCK", "parent 'X' is not a function block"},
		{"FUNCTION_BLOCK Base METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base METHOD M : BOOL M := TRUE; END_METHOD END_FUNCTION_BLOCK", "return type mismatch for method 'M': derived is 'BOOL', parent is 'INT'"},
		{"FUNCTION_BLOCK Base METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base METHOD M M := 1; END_METHOD END_FUNCTION_BLOCK", "return type mismatch for method 'M': derived has no return type, parent has INT"},
		{"FUNCTION_BLOCK Base METHOD M VAR_INPUT a : INT; END_VAR END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base METHOD M VAR_INPUT a : BOOL; END_VAR END_METHOD END_FUNCTION_BLOCK", "parameter type mismatch for method 'M' at index 0: derived is 'BOOL', parent is 'INT'"},
		{"FUNCTION_BLOCK ABSTRACT Ab END_FUNCTION_BLOCK TYPE Pt : STRUCT a : Ab; END_STRUCT; END_TYPE VAR p : Pt; END_VAR", "cannot instantiate abstract function block 'Ab'"},
		// Assignments inside methods.
		{"FUNCTION_BLOCK Fb PROPERTY Pr : INT GET Pr := 3; END_GET END_PROPERTY METHOD M : INT Pr := 9; M := 1; END_METHOD END_FUNCTION_BLOCK", "property 'Pr' is read-only"},
		{"FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR METHOD M : INT i := 3; M := i; END_METHOD END_FUNCTION_BLOCK", "cannot assign to read-only variable 'i'"},
		{"FUNCTION_BLOCK Fb VAR FINAL k : INT := 1; END_VAR METHOD M : INT k := 3; M := k; END_METHOD END_FUNCTION_BLOCK", "cannot assign to FINAL variable 'k' from function block 'Fb'"},
		// Calls and their arguments.
		{"FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR io := io + 1; Inc := io; END_FUNCTION Inc(5);", "argument for VAR_IN_OUT 'io' of function 'Inc' must be a variable, got 5"},
		{"FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR io := io + 1; Inc := io; END_FUNCTION Inc();", "the call to 'Inc' must supply its VAR_IN_OUT 'io'"},
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION F(o => nope);", "undefined variable nope"},
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION VAR CONSTANT k : INT := 1; END_VAR F(o => k);", "cannot assign to a constant variable 'k'"},
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR F := 1; END_FUNCTION F(o => nope[0]);", "undefined variable nope"},
		{"FUNCTION F : INT VAR_INPUT a : INT := 1; b : INT := 2; END_VAR F := a * 10 + b; END_FUNCTION F(b := nope);", "undefined variable nope"},
		{"FUNCTION F : INT VAR_INPUT a : INT := nope; END_VAR F := a; END_FUNCTION F();", "undefined variable nope"},
		// Type checking of member access.
		{"FUNCTION_BLOCK Fb METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR x := f.M + 1;", "cannot take value of method 'M'"},
		{"FUNCTION_BLOCK Fb END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR x := f.nope + 1;", "member 'nope' not found on function block 'Fb'"},
		{"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR p : Pt; x : INT; END_VAR x := p.nope + 1;", "member 'nope' not found on structure 'Pt'"},
		{"VAR i : INT; x : INT; END_VAR x := i.q + 1;", "type definition not found for 'INT'"},
		{"FUNCTION_BLOCK Fb METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR x := f.Q() + 1;", "method 'Q' not found on type 'FB'"},
		{"VAR x : INT; END_VAR x := nope3.M() + 1;", "undefined identifier: nope3"},
		{"FUNCTION_BLOCK Fb VAR v : INT; END_VAR PROPERTY Pv : INT GET Pv := v; END_GET PRIVATE SET v := value; END_SET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.Pv := 1;", "cannot access setter for property 'Pv': member is private"},
		{"FUNCTION_BLOCK Fb PROPERTY Ro : INT GET Ro := 1; END_GET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.Ro := 1;", "property 'Ro' is read-only"},
		{"nope.x := 1;", "undefined variable nope"},
		{"FUNCTION_BLOCK Fb VAR x : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.x := nope;", "undefined variable nope"},
		{"VAR b : BOOL; END_VAR b := nope AND TRUE;", "undefined identifier: nope"},
		{"VAR b : BOOL; END_VAR b := TRUE AND nope;", "undefined identifier: nope"},
		{"FUNCTION_BLOCK Fb METHOD M : INT M := SUPER^.M() + 1; END_METHOD END_FUNCTION_BLOCK", "SUPER used outside of a derived function block"},
		{"FUNCTION_BLOCK Fb METHOD N END_METHOD END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR x := f.N() + 1;", "operator '+' not defined for types NULL and LINT"},
		{"FUNCTION_BLOCK Fb IMPLEMENTS Nope END_FUNCTION_BLOCK", "interface 'NOPE' not found"},
		{"FUNCTION_BLOCK X END_FUNCTION_BLOCK FUNCTION_BLOCK Fb IMPLEMENTS X END_FUNCTION_BLOCK", "'X' is not an interface"},
		{"FUNCTION_BLOCK Fb VAR inner : Fb; END_VAR END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "function block 'Fb' contains an instance of itself"},
		{"FUNCTION_BLOCK Fb VAR v : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb := (v := nope); END_VAR", "undefined variable nope"},
		// PROTECTED members are checked when written as well as read, including
		// through a variable of the enclosing function block used in a method.
		{"FUNCTION_BLOCK A VAR PROTECTED pv : INT; END_VAR END_FUNCTION_BLOCK VAR a1 : A; END_VAR a1.pv := 1;", "cannot assign to member variable 'pv': member is protected"},
		{"FUNCTION_BLOCK A VAR PROTECTED pv : INT; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK B VAR a1 : A; END_VAR METHOD M : INT a1.pv := 1; M := 1; END_METHOD END_FUNCTION_BLOCK", "cannot assign to member variable 'pv': member is protected"},
		{"FUNCTION_BLOCK A VAR PROTECTED pv : INT; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK B VAR a1 : A; END_VAR METHOD M : INT M := a1.pv; END_METHOD END_FUNCTION_BLOCK", "cannot access member variable 'pv': member is protected"},
		{"FUNCTION_BLOCK Z END_FUNCTION_BLOCK FUNCTION_BLOCK A VAR PROTECTED pv : INT; END_VAR END_FUNCTION_BLOCK FUNCTION_BLOCK B EXTENDS Z METHOD M : INT VAR a1 : A; END_VAR M := a1.pv; END_METHOD END_FUNCTION_BLOCK", "cannot access member variable 'pv': member is protected"},
		{"INTERFACE FINAL I1 METHOD M : INT; END_INTERFACE INTERFACE I2 EXTENDS I1 END_INTERFACE", "cannot extend from FINAL interface 'I1'"},
		// Array bounds and repetitions.
		{"VAR a : ARRAY[0..1] OF INT; END_VAR a[5] := 1;", "array index out of bounds: index is 5 but bounds are 0..1"},
		{"VAR a : ARRAY[0..1] OF INT := [x(1)]; END_VAR", "undefined variable x"},
		{"VAR a : ARRAY[0..3] OF INT := [2(nope)]; END_VAR", "undefined variable nope"},
		{"VAR a : ARRAY[0..3] OF STRING := [1]; END_VAR", "cannot initialize 'a' of type STRING with a value of type LINT"},
		{"VAR a : ARRAY[0..(nope + 1)] OF INT; END_VAR", "array 'a' must have constant bounds"},
		{"VAR a : ARRAY[0..(1 + nope)] OF INT; END_VAR", "array 'a' must have constant bounds"},
		{"VAR a : ARRAY[0..(1 MOD 2)] OF INT; END_VAR", "array 'a' must have constant bounds"},
		{"VAR a : ARRAY[0..-nope] OF INT; END_VAR", "array 'a' must have constant bounds"},
		{"VAR a : ARRAY[0..NOT 1] OF INT; END_VAR", "array 'a' must have constant bounds"},
		// Initial values.
		{"TYPE Rng : INT(3..9); END_TYPE VAR rv : Rng := 12; END_VAR", "initial value of 'rv': 12 is out of range for subrange type 'Rng' (3..9)"},
		{"TYPE Color : (Red, Green); END_TYPE TYPE Shade : (Light, Dark); END_TYPE VAR c : Color := Shade#Dark; END_VAR", "cannot initialize 'c' of type Color with a value of type Shade"},
		{"TYPE Color : (Red, Green); END_TYPE VAR c : Color := Purple; END_VAR", "initial value of 'c': 'Purple' is not a value of enumeration 'Color'"},
		{"UDINT#16#FFFFFFFFFFFFFFFFFF;", "invalid UDINT literal '16#FFFFFFFFFFFFFFFFFF'"},
		// Errors in the defaults of array elements and function results.
		{"FUNCTION_BLOCK Fb VAR v : INT := nope; END_VAR END_FUNCTION_BLOCK VAR a : ARRAY[0..1] OF Fb; END_VAR", "undefined variable nope"},
		{"FUNCTION_BLOCK Fb VAR v : INT := nope; END_VAR END_FUNCTION_BLOCK VAR a : ARRAY[0..1, 0..1] OF Fb; END_VAR", "undefined variable nope"},
		{"TYPE Pt : STRUCT px : INT; py : INT := nope; END_STRUCT; END_TYPE VAR a : ARRAY[0..2] OF Pt := [(px := 1)]; END_VAR", "undefined variable nope"},
		{"FUNCTION_BLOCK ABSTRACT Ab END_FUNCTION_BLOCK VAR a : ARRAY[0..1] OF Ab; END_VAR", "cannot instantiate abstract function block 'Ab'"},
		{"FUNCTION F : Pt2 F := 1; END_FUNCTION TYPE Pt2 : STRUCT px : INT := nope; END_STRUCT; END_TYPE", "undefined variable nope"},
		{"FUNCTION_BLOCK Fb METHOD M : Pt2 END_METHOD END_FUNCTION_BLOCK TYPE Pt2 : STRUCT px : INT := nope; END_STRUCT; END_TYPE", "undefined variable nope"},
		// Configurations.
		{"PROGRAM Prg VAR c1 : INT; END_VAR END_PROGRAM CONFIGURATION C VAR_GLOBAL g : INT := nope; END_VAR RESOURCE Res ON CPU PROGRAM P1 : Prg; END_RESOURCE END_CONFIGURATION", "undefined variable nope"},
		{"PROGRAM Prg VAR c1 : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prg; END_RESOURCE VAR_CONFIG Res.Nope.c1 : INT := 1; END_VAR END_CONFIGURATION", "VAR_CONFIG path 'Res.Nope.c1' does not name a variable of a program instance in configuration 'C'"},
		{"PROGRAM Prg VAR c1 : INT; END_VAR END_PROGRAM CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prg; END_RESOURCE VAR_CONFIG Res.P1.c1 : INT := nope; END_VAR END_CONFIGURATION", "undefined variable nope"},
		{"CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := nope, PRIORITY := 1); END_RESOURCE END_CONFIGURATION", "undefined variable nope"},
		{"CONFIGURATION C RESOURCE Res ON CPU TASK T1(INTERVAL := T#1s, PRIORITY := nope); END_RESOURCE END_CONFIGURATION", "undefined variable nope"},
		// IL.
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 ST 5 END_PROGRAM", "operand for ST must be a variable identifier"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 ST nope END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : BOOL; END_VAR LD TRUE S nope END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD nope END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 ADD nope END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : BOOL; END_VAR LD TRUE AND nope END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 JMP 5 END_PROGRAM", "operand for JMP must be a label identifier"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 CAL nope() END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 SIN END_PROGRAM", "IL operator not yet supported by compiler: SIN"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 ADD( LD nope ) END_PROGRAM", "undefined variable nope"},
		{"PROGRAM Pg VAR a : INT; END_VAR LD 1 ADD END_PROGRAM", "ADD instruction requires an operand"},
	})
}

// Compiled functions are only shared in the constant pool when their
// parameters and outputs match, not just their instructions.
func TestConstantDeduplicationHelpers(t *testing.T) {
	stringCases := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
	}
	for _, c := range stringCases {
		if got := equalStrings(c.a, c.b); got != c.want {
			t.Errorf("equalStrings(%v, %v) = %v", c.a, c.b, got)
		}
	}
	intCases := []struct {
		a, b []int
		want bool
	}{
		{[]int{1, 2}, []int{1, 2}, true},
		{[]int{1}, []int{1, 2}, false},
		{[]int{1, 2}, []int{1, 3}, false},
	}
	for _, c := range intCases {
		if got := equalInts(c.a, c.b); got != c.want {
			t.Errorf("equalInts(%v, %v) = %v", c.a, c.b, got)
		}
	}
}

// setSymbol stores into free and external variables, and rejects scopes that
// cannot be assigned.
func TestSetSymbolScopes(t *testing.T) {
	tests := []struct {
		scope SymbolScope
		op    code.Opcode
	}{
		{FreeScope, code.OpSetFree},
		{ExternalScope, code.OpSetExternal},
	}
	for _, tt := range tests {
		c := New()
		if err := c.setSymbol(Symbol{Name: "v", Scope: tt.scope, Index: 0}); err != nil {
			t.Fatalf("setSymbol(%s): %s", tt.scope, err)
		}
		if !hasOpcode(c.currentInstructions(), tt.op) {
			t.Errorf("setSymbol(%s) did not emit opcode %d", tt.scope, tt.op)
		}
	}
	if err := New().setSymbol(Symbol{Name: "LEN", Scope: BuiltinScope}); err == nil || !strings.Contains(err.Error(), "cannot set symbol with scope BUILTIN") {
		t.Errorf("expected an error for a built-in, got %v", err)
	}
}

func TestFunctionBlockCallErrors(t *testing.T) {
	const fb = "FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_IN_OUT io : INT; END_VAR VAR_OUTPUT o : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; x : INT; END_VAR "
	checkCompileErrors(t, []struct{ input, want string }{
		{fb + "f(nope := 1);", "function block 'Fb' has no input 'nope'"},
		{fb + "f(nope => x);", "function block 'Fb' has no output 'nope'"},
		{fb + "f(1, x, 3);", "function block 'Fb' takes 2 inputs, got more"},
		{fb + "f(io := 5);", "argument for VAR_IN_OUT 'io' of function block 'Fb' must be a variable, got 5"},
	})
	// A function block's own variables hide built-ins of the same name.
	checkCompiles(t, []string{
		"FUNCTION_BLOCK Fb VAR first : INT; END_VAR first := first + 1; END_FUNCTION_BLOCK",
	})
}

func TestProgramErrors(t *testing.T) {
	checkCompileErrors(t, []struct{ input, want string }{
		// Errors in a program's body, variables or call are reported.
		{"PROGRAM P VAR n : INT; END_VAR LD 1 ST missing END_PROGRAM", "undefined variable missing"},
		{"PROGRAM P VAR n : INT; END_VAR LD TRUE S missing END_PROGRAM", "undefined variable missing"},
		{"PROGRAM P VAR CONSTANT k : INT := 1; END_VAR k := 2; END_PROGRAM", "cannot assign to a constant variable 'k'"},
		{"PROGRAM P VAR a : ARRAY[1..3] OF INT; END_VAR a[4] := 1; END_PROGRAM", "array index out of bounds: index is 4 but bounds are 1..3"},
		{"PROGRAM P VAR_GLOBAL g : INT := missing; END_VAR END_PROGRAM", "undefined variable missing"},
		{"PROGRAM P VAR_EXTERNAL e : INT := missing; END_VAR END_PROGRAM", "undefined variable missing"},
		{"PROGRAM P VAR_INPUT i : INT; END_VAR END_PROGRAM P(nope := 1);", "program 'P' has no input 'nope'"},
	})
}

// The programs the VM runs in its tests of function block and program calls,
// in-outs, array bounds and copies compile; the VM tests check what they do.
func TestCallAndValueFeaturesCompile(t *testing.T) {
	const pt = "TYPE Pt : STRUCT x : INT; y : INT; END_STRUCT; END_TYPE "
	const twice = "FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_FUNCTION_BLOCK "
	checkCompiles(t, []string{
		twice + "VAR f : Fb; res : INT; ok : BOOL; END_VAR f(EN := FALSE, i := 3, o => res, ENO => ok); f(3, o => res);",
		"FUNCTION_BLOCK Acc VAR_INPUT amount : INT; END_VAR VAR_IN_OUT total : INT; END_VAR total := total + amount; END_FUNCTION_BLOCK " +
			"VAR a : Acc; arr : ARRAY[0..1] OF INT; sm : INT; END_VAR a(amount := 3, total := arr[1]); a(4, sm);",
		"FUNCTION_BLOCK Base VAR_INPUT i : INT; END_VAR VAR_OUTPUT seen : INT; END_VAR seen := i; END_FUNCTION_BLOCK " +
			"FUNCTION_BLOCK Dv EXTENDS Base VAR_OUTPUT dbl : INT; END_VAR dbl := seen * 2; END_FUNCTION_BLOCK VAR d : Dv; res : INT; END_VAR d(i := 7, dbl => res, seen => res);",
		"FUNCTION_BLOCK Inner VAR_OUTPUT q : INT; END_VAR q := q + 10; END_FUNCTION_BLOCK " +
			"FUNCTION_BLOCK Outer VAR sub : Inner; END_VAR VAR_OUTPUT total : INT; END_VAR sub(q => total); THIS.sub(); END_FUNCTION_BLOCK " +
			"VAR o : Outer; res : INT; END_VAR o(); o.sub(q => res);",
		"FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR io := io + 1; Inc := io; END_FUNCTION " +
			"FUNCTION_BLOCK Fb VAR_OUTPUT n : INT; END_VAR VAR a : ARRAY[0..1] OF INT; END_VAR Inc(n); Inc(a[1]); Inc(io := n); END_FUNCTION_BLOCK",
		// Arrays with lower bounds, as variables, members, parameters and types.
		"TYPE Row : ARRAY[1..3] OF INT; END_TYPE TYPE Rec : STRUCT row1 : Row; END_STRUCT; END_TYPE " +
			"FUNCTION Second : INT VAR_INPUT v : Row; END_VAR Second := v[2]; END_FUNCTION " +
			"FUNCTION_BLOCK Fb VAR buf : ARRAY[1..2] OF INT; END_VAR buf := [4, 5]; END_FUNCTION_BLOCK " +
			"VAR x : Rec; m : ARRAY[1..2, 1..3] OF INT; r1 : Row; END_VAR x.row1 := [1, 2, 3]; m[2][3] := 9; r1 := x.row1; Second(r1);",
		// Copies of arrays and structures.
		pt + "VAR p : Pt; q : Pt; ps : ARRAY[0..1] OF Pt; m : ARRAY[0..1, 0..1] OF INT; row : ARRAY[0..1] OF INT; n : INT; END_VAR " +
			"q := p; ps[0] := q; m[0] := row; row[0] := n; n := row[0];",
		pt + "FUNCTION_BLOCK Fb VAR_INPUT p : Pt; END_VAR METHOD M : INT VAR_INPUT q : Pt; END_VAR M := q.x; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; q : Pt; END_VAR f(p := q); f.M(q);",
		// Programs.
		"PROGRAM Pg VAR_INPUT i : INT; END_VAR VAR_IN_OUT io : INT; END_VAR VAR_OUTPUT o : INT; END_VAR VAR_TEMP t1 : INT; END_VAR o := i * 2; io := t1; END_PROGRAM " +
			"VAR res : INT; v : INT; END_VAR Pg(i := 4, io := v, o => res);",
		"PROGRAM Pi VAR n : INT; q : BOOL; END_VAR LD n ADD 1 ST n LD n GT 1 S q R q END_PROGRAM Pi();",
	})
	checkCompileErrors(t, []struct{ input, want string }{
		{twice + "VAR f : Fb; END_VAR f(o := 1);", "function block 'Fb' has no input 'o'"},
		{"FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR Inc := io; END_FUNCTION Inc();", "the call to 'Inc' must supply its VAR_IN_OUT 'io'"},
		{"FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR Inc := io; END_FUNCTION Inc(io := 1 + 2);", "argument for VAR_IN_OUT 'io' of function 'Inc' must be a variable"},
		{"FUNCTION_BLOCK Acc VAR_IN_OUT io : INT; END_VAR END_FUNCTION_BLOCK VAR a : Acc; END_VAR a(1 + 2);", "argument for VAR_IN_OUT 'io' of function block 'Acc' must be a variable"},
		{"FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; END_VAR f(i := missing);", "undefined variable missing"},
	})
}

func TestCallAndValueFeatureEdges(t *testing.T) {
	const twice = "FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_FUNCTION_BLOCK "
	checkCompiles(t, []string{
		// ENO without EN is TRUE.
		twice + "VAR f : Fb; ok : BOOL; END_VAR f(ENO => ok);",
		// IL stores into a function's own variables and result.
		"FUNCTION F : INT VAR x : INT; END_VAR LD 1 ST x ST F END_FUNCTION",
		// Too many indices for an array is not a copy of an array.
		"VAR a : ARRAY[0..1] OF INT; x : INT; END_VAR a[0] := x;",
		// A function block inside a structure is called through it.
		twice + "TYPE Holder : STRUCT f : Fb; END_STRUCT; END_TYPE VAR h : Holder; END_VAR h.f(i := 1);",
	})
	checkCompileErrors(t, []struct{ input, want string }{
		{twice + "VAR f : Fb; END_VAR f(EN := missing);", "undefined variable missing"},
		{twice + "VAR f : Fb; END_VAR f(o => missing);", "missing"},
		{twice + "VAR f : Fb; END_VAR f(ENO => missing);", "missing"},
	})
}

func TestEnumAndStructureInitializerEdges(t *testing.T) {
	const types = "TYPE Color : (Red, Green); END_TYPE TYPE Shade : (Dark); END_TYPE TYPE Pt : STRUCT x : INT; END_STRUCT; END_TYPE "
	checkCompiles(t, []string{types + "VAR c : Color := Color#Green; END_VAR"})
	checkCompileErrors(t, []struct{ input, want string }{
		{types + "VAR c : Color := Shade#Dark; END_VAR", "cannot initialize 'c' of type Color with a value of type Shade"},
		{types + "VAR p : Pt := (x := 1, 2); END_VAR", "a structure initializer must list members as name := value"},
	})
}

func TestIlCompileErrors(t *testing.T) {
	checkCompileErrors(t, []struct{ input, want string }{
		{"PROGRAM P VAR x : INT; END_VAR l1: LD 1 l1: ST x END_PROGRAM", "duplicate label defined: l1"},
		{"PROGRAM P VAR x : INT; END_VAR LD 1 ST 5 END_PROGRAM", "operand for ST must be a variable identifier"},
		{"PROGRAM P VAR x : INT; END_VAR LD TRUE S 5 END_PROGRAM", "operand for S must be a variable identifier"},
		{"PROGRAM P VAR x : INT; END_VAR LD TRUE JMP 5 END_PROGRAM", "operand for JMP must be a label identifier"},
		{"PROGRAM P VAR x : INT; END_VAR LD 1 JMP nowhere END_PROGRAM", "undefined jump label: nowhere"},
		{"PROGRAM P VAR x : INT; END_VAR LD missing END_PROGRAM", "undefined variable missing"},
	})
	// A jump back to a label already compiled, and an unconditional jump.
	checkCompiles(t, []string{"PROGRAM P VAR i : INT; END_VAR again: LD i ADD 1 ST i LT 3 JMPC again JMP done done: LD 0 END_PROGRAM"})
}
