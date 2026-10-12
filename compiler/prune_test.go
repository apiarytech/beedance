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
	"os"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// A program compiles only the library POUs it reaches, and each OSCAT block
// and function compiles with what it reaches: nothing it needs is left out.
func TestReachableOSCAT(t *testing.T) {
	object.FinalizeBuiltins()
	var lib string
	for _, f := range []string{oscatPath, "../reference/beedance_building_100.st"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Skip(err)
		}
		lib += string(b) + "\n"
	}
	var blocks, funcs []string
	for _, s := range parse(t, lib).Statements {
		switch n := s.(type) {
		case *ast.FunctionBlockDeclaration:
			blocks = append(blocks, n.Name.Value)
		case *ast.FunctionDeclaration:
			funcs = append(funcs, n.Name.Value)
		}
	}
	compile := func(prog string) error {
		c := NewCompilerWithBuiltins(object.Builtins)
		c.AllowInputWrites = true // OSCAT is CODESYS's dialect
		_, err := c.CompileProgramUnit(parse(t, prog+"\n"+lib), "P")
		return err
	}
	// These bound an array by a constant declared after it, which beedance does
	// not resolve yet, pruned or not.
	known := map[string]bool{"FIFO_16": true, "FIFO_32": true, "STACK_16": true, "STACK_32": true, "TIMER_P4": true}
	for _, b := range blocks {
		if known[b] {
			continue
		}
		if err := compile("PROGRAM P\nVAR x : " + b + "; END_VAR\nx();\nEND_PROGRAM"); err != nil {
			t.Errorf("block %s: %v", b, err)
		}
	}
	// A function is reached through a block whose body names it.
	for _, f := range funcs {
		src := "FUNCTION_BLOCK W\nVAR r : BOOL; END_VAR\nIF r THEN r := " + f + " = " + f + "; END_IF;\nEND_FUNCTION_BLOCK\nPROGRAM P\nVAR w : W; END_VAR\nw();\nEND_PROGRAM"
		if err := compile(src); err != nil && strings.Contains(err.Error(), "undefined") {
			t.Errorf("function %s: %v", f, err)
		}
	}
	// C_TO_F alone leaves the rest of OSCAT out.
	pous := parse(t, "PROGRAM P\nVAR r : REAL; END_VAR\nr := C_TO_F(1.0);\nEND_PROGRAM\n"+lib).Statements
	kept := reachable(pous[0], pous[1:])
	if len(kept) > 30 {
		t.Errorf("C_TO_F kept %d statements of %d", len(kept), len(pous)-1)
	}
}
