/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

import (
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
)

// TestTimeFunction checks TIME(), the CODESYS clock OSCAT's T_PLC_MS reads.
func TestTimeFunction(t *testing.T) {
	saved := stdlib.Clock
	defer func() { stdlib.Clock = saved }()
	stdlib.Clock = func() time.Duration { return 1500 * time.Millisecond }

	for input, want := range map[string]bool{
		"TIME() = T#1.5s;": true,
		"FUNCTION T_PLC_MS : DWORD VAR tx : TIME; END_VAR tx := TIME(); T_PLC_MS := TIME_TO_DWORD(tx); END_FUNCTION T_PLC_MS() = 1500;": true,
	} {
		got, ok := testEval(t, input).(*object.Boolean)
		if !ok || got.Value != want {
			t.Errorf("%s = %v, want %v", input, testEval(t, input), want)
		}
	}
}
