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
	"strings"
	"testing"
)

// TIME(), the CODESYS clock, reads the time since the program started.
func TestTimeFunction(t *testing.T) {
	out, err := GoFile(parseForTest(t, "FUNCTION T_PLC_MS : DWORD VAR tx : TIME; END_VAR tx := TIME(); T_PLC_MS := TIME_TO_DWORD(tx); END_FUNCTION"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tx = plcTime()", "func plcTime() iec.TIME", "var plcStarted = time.Now()", `"time"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %s in:\n%s", want, out)
		}
	}
	// A program without TIME() has no clock.
	out, err = GoFile(parseForTest(t, "FUNCTION F : TIME F := TIME#1s; END_FUNCTION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "plcTime") {
		t.Errorf("unexpected clock in:\n%s", out)
	}
}
