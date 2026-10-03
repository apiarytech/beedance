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
	"os"
	"testing"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// TestOscatLibraryRuns compiles the converted OSCAT BASIC library with a
// program that calls some of its functions, and runs it. It is skipped
// without the library.
func TestOscatLibraryRuns(t *testing.T) {
	library, err := os.ReadFile("../reference/beedance_oscat_basic.st")
	if err != nil {
		t.Skipf("OSCAT library not present: %v", err)
	}
	object.FinalizeBuiltins()
	builtins := make([]*object.Builtin, len(object.Builtins))
	for _, entry := range object.Builtins {
		builtins[entry.Index] = entry.Builtin
	}
	for call, want := range map[string]interface{}{
		"GCD(12, 18)":                         6,
		"BIT_COUNT(DWORD#16#F0F0)":            8,
		"WORK_WEEK(D#2024-05-06)":             19,
		"DST(DT#2024-07-01-12:00:00)":         true,
		"DST(DT#2024-01-15-12:00:00)":         false,
		"BCDC_TO_INT(BYTE#16#42)":             42,
		"FIB(10)":                             55,
		"YEAR_OF_DATE(D#2024-05-06)":          2024,
		"DAY_OF_WEEK(D#2024-05-06)":           1,
		"LEAP_YEAR(2024)":                     true,
		"SET_DATE(2024, 3, 1) = D#2024-03-01": true,
		"HOUR(TOD#13:45:00)":                  13,
		"EXPN(2.0, 10) = 1024.0":              true,
		"DEG_TO_DIR(90, 2, 0) = 'E'":          true,
	} {
		p := parser.New(lexer.New(string(library) + "\n" + call + ";"))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("%s: %v", call, p.Errors()[0])
		}
		comp := compiler.NewCompilerWithBuiltins(object.Builtins)
		if err := comp.Compile(program); err != nil {
			t.Errorf("%s: compiler error: %v", call, err)
			continue
		}
		machine := NewWithBuiltins(comp.Bytecode(), builtins)
		if err := machine.Run(); err != nil {
			t.Errorf("%s: %v", call, err)
			continue
		}
		got := machine.LastPoppedStackElem()
		switch want := want.(type) {
		case int:
			if n, _, ok := object.GetIntegerObjectValue(got); !ok || n != int64(want) {
				t.Errorf("%s = %s, want %d", call, got.Inspect(), want)
			}
		case bool:
			if b, ok := got.(*object.Boolean); !ok || b.Value != want {
				t.Errorf("%s = %s, want %v", call, got.Inspect(), want)
			}
		}
	}
}
