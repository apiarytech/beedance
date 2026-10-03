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

import (
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
	"testing"
)

func TestArrayElementAssignment(t *testing.T) {
	tests := []vmTestCase{
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR a[1] := 9; a[1];", 9},
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; END_VAR a[0] := a[2] * 10; a;", []int{30, 2, 3}},
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; i : INT := 2; END_VAR a[i] := 7; a[i];", 7},
	}
	runVmTests(t, tests)
}

func TestArrayReadOutOfRangeIsNull(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][3];", Null},
		{"[1, 2, 3][-1];", Null},
		{"[][0];", Null},
	}
	runVmTests(t, tests)
}

func TestArrayAssignmentOutOfRange(t *testing.T) {
	runVmErrorTests(t, []vmErrorTestCase{
		// A constant index is checked at compile time; a variable index at run time.
		{"VAR a : ARRAY[0..2] OF INT := [1, 2, 3]; i : INT := 5; END_VAR a[i] := 9;", "array index out of bounds: 5"},
	})
}

func TestHashElementAccess(t *testing.T) {
	tests := []vmTestCase{
		{`{"a": 1, "b": 2}["b"];`, 2},
		{`{"a": 1}["missing"];`, Null},
		{`{1: 10, 2: 20}[1 + 1];`, 20},
	}
	runVmTests(t, tests)
}

// TestInvalidIndexing covers index operations on values that cannot be indexed,
// which the compiler does not rule out. They are assembled directly.
func TestInvalidIndexing(t *testing.T) {
	array := &object.Array{Elements: []object.Object{&object.LInt{Value: 1}}}
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{}}
	one := &object.LInt{Value: 1}
	text := &object.String{Value: "x"}

	tests := []struct {
		name         string
		constants    []object.Object
		instructions [][]byte
		want         string
	}{
		{
			name:         "read from an integer",
			constants:    []object.Object{one, one},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpIndex)},
			want:         "index operator not supported: LINT",
		},
		{
			name:         "read an array with a string index",
			constants:    []object.Object{array, text},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpIndex)},
			want:         "index operator not supported: ARRAY",
		},
		{
			name:         "read a hash with an unhashable key",
			constants:    []object.Object{hash, array},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpIndex)},
			want:         "unusable as hash key: ARRAY",
		},
		{
			name:         "assign into an integer",
			constants:    []object.Object{one, one},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpConstant, 1), code.Make(code.OpSetIndex)},
			want:         "index operator not supported for setting on: LINT",
		},
		{
			name:         "assign into a hash with an unhashable key",
			constants:    []object.Object{hash, array, one},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpConstant, 2), code.Make(code.OpSetIndex)},
			want:         "unusable as hash key: ARRAY",
		},
		{
			name:         "build a hash with an unhashable key",
			constants:    []object.Object{array, one},
			instructions: [][]byte{code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpHash, 2)},
			want:         "unusable as hash key: ARRAY",
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

// TestArraySetIndexUsesIntegerTypes checks that any integer type can index an array.
func TestArraySetIndexUsesIntegerTypes(t *testing.T) {
	array := &object.Array{Elements: []object.Object{Null, Null}}
	index := &object.Int{Value: 1}
	value := &object.LInt{Value: 5}
	_, err := runBytecode([]object.Object{array, index, value}, nil,
		code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1), code.Make(code.OpConstant, 2), code.Make(code.OpSetIndex))
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}
	if array.Elements[1] != value {
		t.Fatalf("expected element 1 to be set, got %v", array.Elements[1])
	}
}
