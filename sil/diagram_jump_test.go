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

	"github.com/apiarytech/beedance/plcopen"
	"github.com/apiarytech/beedance/sil"
)

// diagramJumps (IEC-71): jumps, labels and returns in LD and FBD, in the
// text form: a rung ending in >>LABEL past a rung when not enabled, a
// label, a rung ending in RETURN; in FBD a loop by a backward jump and a
// conditional return. They lower to ST jumps (IEC-68).
const diagramJumps = `
FUNCTION_BLOCK FB_LdJumps
VAR_INPUT enable, a : BOOL; END_VAR
VAR_OUTPUT q, seen, last : BOOL; END_VAR
LD
  RUNG skip_when_off
    /enable >>SKIP
  RUNG
    a ( q )
  SKIP:
  RUNG
    a ( seen )
  RUNG
    /a RETURN
  RUNG
    enable ( last )
END_LD
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FbdJumps
VAR_INPUT k : INT; stop : BOOL; END_VAR
VAR_OUTPUT i : INT; after : INT; END_VAR
FBD
  i := 0
  AGAIN:
  i := ADD(i, 1)
  >>AGAIN(LT(i, k))
  RETURN(stop)
  after := i
END_FBD
END_FUNCTION_BLOCK

PROGRAM TEST_DiagramJumps
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR ld : FB_LdJumps; fbd : FB_FbdJumps; n : INT; END_VAR
n := n + 1;
IF n = 1 THEN
  ld(enable := FALSE, a := TRUE);   (* q skipped, seen, last := enable *)
  IF ld.q OR NOT ld.seen OR ld.last THEN failures := failures + 1; END_IF;
  fbd(k := 4, stop := FALSE);
  IF fbd.i <> 4 OR fbd.after <> 4 THEN failures := failures + 1; END_IF;
ELSIF n = 2 THEN
  ld(enable := TRUE, a := TRUE);
  IF NOT ld.q OR NOT ld.seen OR NOT ld.last THEN failures := failures + 1; END_IF;
  fbd(k := 7, stop := TRUE);    (* returns before after := i *)
  IF fbd.i <> 7 OR fbd.after <> 4 THEN failures := failures + 1; END_IF;
ELSIF n = 3 THEN
  ld(enable := TRUE, a := FALSE);   (* returns before last := enable: last stays TRUE *)
  IF ld.q OR ld.seen OR NOT ld.last THEN failures := failures + 1; END_IF;
  done := TRUE;
END_IF;
END_PROGRAM
`

// TestDiagramJumpsOnEveryEngine (IEC-71) runs them on the evaluator, the VM
// and the transpiled Go (where the Go toolchain is), which agree.
func TestDiagramJumpsOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), diagramJumps, sil.Options{Engines: engines, MaxScans: 10, GoTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("%d tests, want 1", len(results))
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) {
			t.Errorf("%s ran on %d engines, want %d", r.Name, len(r.Engines), len(engines))
		}
		if !r.Passed {
			t.Errorf("%s failed: mismatch %q", r.Name, r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v", e.Engine, e.Problems())
			}
		}
	}
}

// TestDiagramJumpsThroughPLCopen (IEC-71): exported as PLCopen XML (graphical
// LD and FBD bodies with jump, label and return elements) and imported back,
// as diagrams in the text form and as ST, the source runs as before.
func TestDiagramJumpsThroughPLCopen(t *testing.T) {
	x, err := plcopen.ExportSourceToXML(diagramJumps, "Jumps")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<jump ", `label="SKIP"`, "<label ", "<return "} {
		if !strings.Contains(string(x), want) {
			t.Errorf("exported XML lacks %q", want)
		}
	}
	for name, keep := range map[string]bool{"as diagrams": true, "as ST": false} {
		src, err := plcopen.ImportToIECTextOptions(x, plcopen.ImportOptions{KeepDiagrams: keep})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if keep && (!strings.Contains(src, ">>SKIP") || !strings.Contains(src, "SKIP:")) {
			t.Errorf("%s: the jumps are not in the text form:\n%s", name, src)
		}
		results, err := sil.Run(context.Background(), src, sil.Options{Engines: []sil.Engine{sil.Evaluator, sil.VM}, MaxScans: 10})
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, src)
		}
		for _, r := range results {
			if !r.Passed {
				t.Errorf("%s: %s failed: %q", name, r.Name, r.Mismatch)
			}
		}
	}
}
