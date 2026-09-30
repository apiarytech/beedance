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
	"testing"

	"beedance/ast"
	"beedance/compiler"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
)

// Function outputs can be written to any assignable target.
func TestFunctionOutputTargets(t *testing.T) {
	const f = "FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 3; F := 1; END_FUNCTION "
	runVmTests(t, []vmTestCase{
		{f + "VAR a : ARRAY[0..1] OF INT; END_VAR F(o => a[0]); a;", []int{3, 0}},
		{"TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE " + f + "VAR p : Pt; END_VAR F(o => p.px); p.px;", 3},
		// Inside a function the temporary is a local of that function.
		{f + "FUNCTION G : INT VAR a : ARRAY[0..1] OF INT; END_VAR F(o => a[1]); G := a[1]; END_FUNCTION G();", 3},
	})
}

// CompileProgram's initialization and cyclic code run as main.go runs them:
// initialization once, then the cyclic code on every scan, sharing globals.
func TestCompiledProgramScanCycles(t *testing.T) {
	input := `PROGRAM Counter
		VAR_INPUT incr : INT := 2; END_VAR
		VAR_OUTPUT total : INT; END_VAR
		VAR_TEMP scratch : INT := 1; END_VAR
		total := total + incr + scratch;
	END_PROGRAM`
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	compiled, err := compiler.NewWithState(compiler.NewSymbolTable(), nil, nil, nil).CompileProgram(program.Statements[0].(*ast.ProgramDeclaration))
	if err != nil {
		t.Fatalf("CompileProgram: %s", err)
	}

	globals := make([]object.Object, GlobalsSize)
	if err := NewWithGlobalsStore(compiled.InitBytecode, globals).Run(); err != nil {
		t.Fatalf("initialization: %s", err)
	}
	for scan := 0; scan < 3; scan++ {
		if err := NewWithGlobalsStore(compiled.CyclicBytecode, globals).Run(); err != nil {
			t.Fatalf("scan %d: %s", scan, err)
		}
	}
	// incr (2) and scratch (1, re-initialized every scan) are added three times.
	found := false
	for _, g := range globals {
		if v, _, ok := object.GetIntegerObjectValue(g); ok && v == 9 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected total = 9 after three scans; globals: %v", globals[:4])
	}
}
