/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// TestClassInstanceCallIsAnError: a CLASS has no body, so calling an
// instance of one is an error on every engine (IEC-58).
func TestClassInstanceCallIsAnError(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), `
CLASS C58
VAR n : INT; END_VAR
METHOD PUBLIC Inc : INT
n := n + 1;
Inc := n;
END_METHOD
END_CLASS
PROGRAM TEST_IEC58
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR c : C58; END_VAR
c();
done := TRUE;
END_PROGRAM
`, sil.Options{Engines: engines, MaxScans: 3, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	for _, e := range results[0].Engines {
		if !strings.Contains(e.Err, "CLASS C58") {
			t.Errorf("%s: expected an error about CLASS C58, got %q", e.Engine, e.Err)
		}
	}
}
