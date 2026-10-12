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

	"github.com/apiarytech/beedance/object"
)

// A POU may not declare one name twice, in any case: `Pump : BOOL` beside
// `pump : ACTUATOR_PUMP` compiled, and the program faulted at run time.
func TestDuplicateNames(t *testing.T) {
	object.FinalizeBuiltins()
	for _, c := range []struct{ src, want string }{
		{"PROGRAM P\nVAR_OUTPUT Pump : BOOL; END_VAR\nVAR pump : INT; END_VAR\nPump := TRUE;\nEND_PROGRAM", "'Pump' and 'pump' are one name"},
		{"PROGRAM P\nVAR a : BOOL; a : INT; END_VAR\na := TRUE;\nEND_PROGRAM", "'a' is declared twice"},
		{"FUNCTION_BLOCK B\nVAR_INPUT x : BOOL; END_VAR\nVAR X : INT; END_VAR\nEND_FUNCTION_BLOCK\nPROGRAM P\nVAR b : B; END_VAR\nb();\nEND_PROGRAM", "B: 'x' and 'X'"},
		{"FUNCTION F : INT\nVAR_INPUT n : INT; END_VAR\nVAR N : INT; END_VAR\nF := n;\nEND_FUNCTION\nPROGRAM P\nVAR r : INT; END_VAR\nr := F(1);\nEND_PROGRAM", "F: 'n' and 'N'"},
	} {
		_, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, c.src), "P")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v, want %q\n%s", err, c.want, c.src)
		}
	}
	ok := "FUNCTION_BLOCK B\nVAR_INPUT x : BOOL; END_VAR\nVAR y : INT; END_VAR\nEND_FUNCTION_BLOCK\nPROGRAM P\nVAR b : B; on : BOOL; END_VAR\nb();\nEND_PROGRAM"
	if _, err := NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(parse(t, ok), "P"); err != nil {
		t.Errorf("distinct names refused: %v", err)
	}
}
