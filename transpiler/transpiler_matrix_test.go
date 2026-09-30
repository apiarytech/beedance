/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	"bytes"
	"strings"
	"testing"

	"beedance/lexer"
	"beedance/parser"
)

func transpileString(t *testing.T, input string) string {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors for input:\n%s\nErrors: %v", input, p.Errors())
	}

	var buf bytes.Buffer
	tr := New(&buf)
	if err := tr.Transpile(program); err != nil {
		t.Fatalf("transpilation failed for input:\n%s\nError: %v", input, err)
	}
	return buf.String()
}

func TestTranspileCommaSeparatedIndexExpressions_2D(t *testing.T) {
	input := `
PROGRAM Matrix2DProg
	VAR
		Matrix : ARRAY [0..1, 0..2] OF INT;
		val : INT;
		r : INT := 1;
		c : INT := 2;
	END_VAR

	val := Matrix[1, 2];
	Matrix[0, 1] := 99;
	Matrix[r, c] := val * 2;
	Matrix[r + 1, c - 1] := 10;
END_PROGRAM
`
	output := transpileString(t, input)

	expectedSubstrings := []string{
		"type Matrix2DProg struct {",
		"Matrix [2][3]iec.INT",
		"p.val = p.Matrix[1][2]",
		"p.Matrix[0][1] = 99",
		"p.Matrix[p.r][p.c] = (p.val * 2)",
		"p.Matrix[(p.r + 1)][(p.c - 1)] = 10",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected transpiled output to contain %q, but got:\n%s", expected, output)
		}
	}
}

func TestTranspileCommaSeparatedIndexExpressions_3D(t *testing.T) {
	input := `
PROGRAM Tensor3DProg
	VAR
		Tensor : ARRAY [0..1, 0..1, 0..1] OF INT;
		val : INT;
		i : INT := 0;
		j : INT := 1;
		k : INT := 0;
	END_VAR

	Tensor[1, 0, 1] := 42;
	val := Tensor[i, j, k] + 1;
END_PROGRAM
`
	output := transpileString(t, input)

	expectedSubstrings := []string{
		"Tensor [2][2][2]iec.INT",
		"p.Tensor[1][0][1] = 42",
		"p.val = (p.Tensor[p.i][p.j][p.k] + 1)",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected transpiled output to contain %q, but got:\n%s", expected, output)
		}
	}
}

func TestTranspileCommaSeparatedIndexInFunction(t *testing.T) {
	input := `
FUNCTION Lookup2D : INT
	VAR_INPUT
		arr : ARRAY [0..1, 0..2] OF INT;
		row : INT;
		col : INT;
	END_VAR

	Lookup2D := arr[row, col];
END_FUNCTION
`
	output := transpileString(t, input)

	expectedSubstrings := []string{
		"func Lookup2D(arr [2][3]iec.INT, row iec.INT, col iec.INT) (Lookup2D iec.INT)",
		"Lookup2D = arr[row][col]",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected transpiled output to contain %q, but got:\n%s", expected, output)
		}
	}
}
