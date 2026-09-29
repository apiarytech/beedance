/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

// These tests drive the VM with hand-assembled bytecode, to cover runtime
// behavior the compiler does not currently generate or that only malformed
// bytecode reaches.

import (
	"beedance/code"
	"beedance/compiler"
	"beedance/object"
	"strings"
	"testing"
)

func TestGlobalsShareStateAcrossVMs(t *testing.T) {
	globals := make([]object.Object, GlobalsSize)
	constants := []object.Object{&object.LInt{Value: 5}}

	// The first program sets global 0.
	first := NewWithGlobalsStore(&compiler.Bytecode{
		Instructions: concat(code.Make(code.OpConstant, 0), code.Make(code.OpSetGlobal, 0)),
		Constants:    constants,
	}, globals)
	if err := first.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}

	// The second program reads it, as the REPL does between lines.
	second := NewWithGlobalsStore(&compiler.Bytecode{
		Instructions: concat(code.Make(code.OpGetGlobal, 0), code.Make(code.OpPop)),
		Constants:    constants,
	}, globals)
	if err := second.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(5, second.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
	if &second.Globals()[0] != &globals[0] {
		t.Fatalf("expected the VM to use the provided globals store")
	}
}

func TestUnassignedGlobalReadsAsNull(t *testing.T) {
	machine, err := runBytecode(nil, nil, code.Make(code.OpGetGlobal, 7), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if machine.LastPoppedStackElem() != Null {
		t.Fatalf("expected NULL, got %v", machine.LastPoppedStackElem())
	}
}

func TestClosureFreeVariables(t *testing.T) {
	// fn returns its first free variable.
	fn := compiledFunction(0, 0, code.Make(code.OpGetFree, 0), code.Make(code.OpReturnValue))
	constants := []object.Object{&object.LInt{Value: 42}, fn}
	machine, err := runBytecode(constants, nil,
		code.Make(code.OpConstant, 0),   // the value to capture
		code.Make(code.OpClosure, 1, 1), // closure over one free variable
		code.Make(code.OpCall, 0),
		code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(42, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
}

func TestSwapAndDup(t *testing.T) {
	constants := []object.Object{&object.LInt{Value: 1}, &object.LInt{Value: 10}}
	// 1, 10 -> swap -> 10, 1 -> 10 - 1
	machine, err := runBytecode(constants, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpSwap), code.Make(code.OpSub), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(9, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
	// 10 -> dup -> 10, 10 -> 10 * 10
	machine, err = runBytecode(constants, nil,
		code.Make(code.OpConstant, 1), code.Make(code.OpDup), code.Make(code.OpMul), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(100, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
}

// TestFunctionOutputs covers OpReturnValueMulti, which returns a function's
// VAR_OUTPUT values in a hash alongside its return value.
func TestFunctionOutputs(t *testing.T) {
	fn := compiledFunction(1, 0,
		code.Make(code.OpConstant, 0), code.Make(code.OpSetLocal, 0), // out := 5
		code.Make(code.OpConstant, 1), code.Make(code.OpReturnValueMulti)) // return 1
	fn.OutputNames = []string{"out"}
	fn.OutputIndices = []int{0}
	constants := []object.Object{&object.LInt{Value: 5}, &object.LInt{Value: 1}, fn}

	machine, err := runBytecode(constants, nil, code.Make(code.OpClosure, 2, 0), code.Make(code.OpCall, 0), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	result, ok := machine.LastPoppedStackElem().(*object.Hash)
	if !ok {
		t.Fatalf("expected a hash of outputs, got %T", machine.LastPoppedStackElem())
	}
	for name, want := range map[string]int64{"__return__": 1, "out": 5} {
		pair, ok := result.Pairs[(&object.String{Value: name}).HashKey()]
		if !ok {
			t.Fatalf("output %q missing from %s", name, result.Inspect())
		}
		if err := testIntegerObject(want, pair.Value); err != nil {
			t.Fatalf("output %q: %s", name, err)
		}
	}

	// The compiler must supply one index per output name.
	fn.OutputIndices = nil
	if _, err := runBytecode(constants, nil, code.Make(code.OpClosure, 2, 0), code.Make(code.OpCall, 0)); err == nil ||
		err.Error() != "internal vm error: mismatch between output names and indices" {
		t.Fatalf("expected an output mismatch error, got %v", err)
	}
}

func TestInstanceMemberFromClass(t *testing.T) {
	// A class member that is not a method is returned as-is.
	class := newClass(nil, map[string]object.Object{"Limit": &object.LInt{Value: 3}})
	machine, err := runBytecode([]object.Object{newInstance(class), &object.String{Value: "Limit"}}, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpIndex), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(3, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedBytecodeErrors(t *testing.T) {
	tests := []struct {
		name         string
		constants    []object.Object
		instructions [][]byte
		want         string
	}{
		{
			name:         "named argument with a non-string name",
			constants:    []object.Object{&object.LInt{Value: 1}},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpMakeNamedArg, 0)},
			want:         "operand to OpMakeNamedArg must be a string constant",
		},
		{
			name:         "closure over a non-function constant",
			constants:    []object.Object{&object.LInt{Value: 1}},
			instructions: [][]byte{code.Make(code.OpClosure, 0, 0)},
			want:         "not a function: &{Value:1}",
		},
		{
			name:         "pop from an empty stack is reported, not a crash",
			instructions: [][]byte{code.Make(code.OpAdd)},
			want:         "vm runtime error: runtime error: index out of range [-1]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runBytecode(tt.constants, nil, tt.instructions...)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("expected error %q, got %v", tt.want, err)
			}
		})
	}
}

func TestStackOverflow(t *testing.T) {
	runVmErrorTests(t, []vmErrorTestCase{
		{"FUNCTION f : INT VAR_INPUT n : INT; END_VAR f := f(n + 1); END_FUNCTION f(1);", "stack overflow"},
	})
}

// TestOperandTypeChecks calls the typed operation helpers directly with the
// wrong operand types. Their callers check types first, so these guard
// against misuse rather than reachable program states.
func TestOperandTypeChecks(t *testing.T) {
	machine := New(emptyBytecode())
	text := &object.String{Value: "x"}
	one := &object.LInt{Value: 1}
	real := &object.LReal{Value: 1}

	checks := []struct {
		name string
		err  error
		want string
	}{
		{"integer op, left", machine.executeBinaryIntegerOperation(code.OpAdd, text, one), "left operand is not an integer: STRING"},
		{"integer op, right", machine.executeBinaryIntegerOperation(code.OpAdd, one, text), "right operand is not an integer: STRING"},
		{"integer op, operator", machine.executeBinaryIntegerOperation(code.OpEqual, one, one), "unknown integer operator: OpEqual"},
		{"real op, left", machine.executeBinaryRealOperation(code.OpAdd, text, real), "left operand is not a real: STRING"},
		{"real op, right", machine.executeBinaryRealOperation(code.OpAdd, real, text), "right operand is not a real: STRING"},
		{"integer compare, left", machine.executeIntegerComparison(code.OpEqual, text, one), "left operand is not an integer: STRING"},
		{"integer compare, right", machine.executeIntegerComparison(code.OpEqual, one, text), "right operand is not an integer: STRING"},
		{"integer compare, operator", machine.executeIntegerComparison(code.OpAdd, one, one), "unknown integer comparison operator: OpAdd"},
		{"real compare, left", machine.executeRealComparison(code.OpEqual, text, real), "left operand is not a real: STRING"},
		{"real compare, right", machine.executeRealComparison(code.OpEqual, real, text), "right operand is not a real: STRING"},
		{"real compare, operator", machine.executeRealComparison(code.OpAdd, real, real), "unknown real comparison operator: OpAdd"},
		{"string compare, operator", machine.executeStringComparison(code.OpAdd, "a", "b"), "unknown string comparison operator: OpAdd"},
		{"array read, index", machine.executeArrayIndex(&object.Array{}, text), "array index must be an integer, got STRING"},
		{"array write, index", machine.executeArraySetIndex(&object.Array{}, text, one), "array index must be an integer, got STRING"},
	}
	for _, c := range checks {
		if c.err == nil || c.err.Error() != c.want {
			t.Errorf("%s: expected error %q, got %v", c.name, c.want, c.err)
		}
	}

	if got := opcodeName(code.Opcode(250)); got != "opcode 250" {
		t.Errorf("expected an unknown opcode to be named by number, got %q", got)
	}
	if got := stringValue(one); got != "" {
		t.Errorf("expected no text for a non-string, got %q", got)
	}
	if !strings.HasPrefix(opcodeName(code.OpAdd), "OpAdd") {
		t.Errorf("expected OpAdd to be named, got %q", opcodeName(code.OpAdd))
	}
}

// concat joins encoded instructions into one stream.
func concat(parts ...[]byte) code.Instructions {
	var out code.Instructions
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestLocatedVariablesUseTheIOImage(t *testing.T) {
	// An output written by the program appears in the I/O image.
	machine := New(compileForTest(t, "VAR out AT %QW2 : INT := 0; END_VAR out := 7 * 3;"))
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(21, machine.IO()["%QW2"]); err != nil {
		t.Fatalf("%%QW2: %s", err)
	}

	// An input set by the host before the run is read by the program.
	machine = New(compileForTest(t, "VAR in AT %IW1 : INT; END_VAR in + 1;"))
	machine.IO()["%IW1"] = &object.LInt{Value: 41}
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(42, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}

	// An address never written reads as NULL.
	machine = New(compileForTest(t, "VAR in AT %IX0.0 : BOOL; END_VAR in;"))
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if machine.LastPoppedStackElem() != Null {
		t.Fatalf("expected NULL, got %v", machine.LastPoppedStackElem())
	}
}

func TestSetFreeVariable(t *testing.T) {
	// fn sets its free variable to 9, then returns it.
	fn := compiledFunction(0, 0,
		code.Make(code.OpConstant, 1), code.Make(code.OpSetFree, 0),
		code.Make(code.OpGetFree, 0), code.Make(code.OpReturnValue))
	constants := []object.Object{&object.LInt{Value: 1}, &object.LInt{Value: 9}, fn}
	machine, err := runBytecode(constants, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpClosure, 2, 1), code.Make(code.OpCall, 0), code.Make(code.OpPop))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if err := testIntegerObject(9, machine.LastPoppedStackElem()); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownAndMalformedInstructions(t *testing.T) {
	if _, err := runBytecode(nil, nil, []byte{250}); err == nil || err.Error() != "unknown opcode 250 at position 0" {
		t.Fatalf("expected an unknown opcode error, got %v", err)
	}
	constants := []object.Object{&object.LInt{Value: 1}}
	want := "external variable operand must be an address string constant, got *object.LInt"
	if _, err := runBytecode(constants, nil, code.Make(code.OpGetExternal, 0)); err == nil || err.Error() != want {
		t.Fatalf("expected %q, got %v", want, err)
	}
	if _, err := runBytecode(constants, nil, code.Make(code.OpConstant, 0), code.Make(code.OpSetExternal, 0)); err == nil || err.Error() != want {
		t.Fatalf("expected %q, got %v", want, err)
	}
}

// TestUnknownNamedArgument covers the VM's own check. The compiler rejects an
// unknown input name when it knows the callee, so this is assembled directly.
func TestUnknownNamedArgument(t *testing.T) {
	fn := compiledFunction(1, 1, code.Make(code.OpGetLocal, 0), code.Make(code.OpReturnValue))
	fn.ParameterNames = []string{"a"}
	constants := []object.Object{fn, &object.LInt{Value: 1}, &object.String{Value: "z"}}
	_, err := runBytecode(constants, nil,
		code.Make(code.OpClosure, 0, 0),
		code.Make(code.OpConstant, 1), code.Make(code.OpMakeNamedArg, 2),
		code.Make(code.OpCall, 1))
	if err == nil || err.Error() != "unknown named argument: z" {
		t.Fatalf("expected an unknown named argument error, got %v", err)
	}
}
