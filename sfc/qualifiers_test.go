/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sfc_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// pulsesAndStores: P1 pulses when its step becomes active and P0 when it
// becomes inactive (IEC 61131-3 3rd edition); DS is stored only if its
// step is still active when its time is up, SD whether it is or not.
// S0 is left after 50 ms: ADS (DS, 100 ms) is cancelled, ASD (SD, 100 ms)
// comes on anyway, ADS2 (DS, 20 ms) was stored in time and runs until R.
const pulsesAndStores = `
PROGRAM TEST_PulsesAndStores
VAR_OUTPUT failures : INT; done : BOOL; scans, p1, p0, ds, sd, ds2 : INT; END_VAR
ACTION Tick: scans := scans + 1; END_ACTION
ACTION AP1: p1 := p1 + 1; END_ACTION
ACTION AP0: p0 := p0 + 1; END_ACTION
ACTION ADS: ds := ds + 1; END_ACTION
ACTION ASD: sd := sd + 1; END_ACTION
ACTION ADS2: ds2 := ds2 + 1; END_ACTION
ACTION Finish: done := TRUE; END_ACTION
INITIAL_STEP S0: Tick(N); AP1(P1); AP0(P0); ADS(DS, T#100MS); ASD(SD, T#100MS); ADS2(DS, T#20MS); END_STEP
TRANSITION FROM S0 TO S1 := scans >= 5; END_TRANSITION
STEP S1: Tick(N); END_STEP
TRANSITION FROM S1 TO S2 := scans >= 25; END_TRANSITION
STEP S2: Tick(N); ASD(R); ADS2(R); AP0(P0); AP1(P1); END_STEP
TRANSITION FROM S2 TO S3 := scans >= 30; END_TRANSITION
STEP S3: Tick(N); Finish(N); END_STEP
END_PROGRAM
`

// TestQualifiersOnEveryEngine runs the qualifiers on the evaluator, the VM
// and the transpiled Go (where the Go toolchain is), which agree scan by
// scan and give IEC 61131-3's results.
func TestQualifiersOnEveryEngine(t *testing.T) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if _, err := exec.LookPath("go"); err == nil && !testing.Short() {
		engines = append(engines, sil.Go)
	}
	results, err := sil.Run(context.Background(), charts+pulsesAndStores, sil.Options{Engines: engines, MaxScans: 500, GoTimeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		for _, e := range r.Engines {
			if e.Err != "" {
				t.Errorf("%s on %s: %s", r.Name, e.Engine, e.Err)
			}
		}
		if r.Mismatch != "" {
			t.Errorf("%s: engines differ: %s", r.Name, r.Mismatch)
		}
		if !r.Passed {
			for _, e := range r.Engines {
				t.Errorf("%s on %s: %v", r.Name, e.Engine, e.Problems())
			}
		}
		if r.Name != "TEST_PulsesAndStores" {
			continue
		}
		for _, e := range r.Engines {
			if len(e.Scans) == 0 {
				continue
			}
			last := e.Scans[len(e.Scans)-1]
			for v, want := range map[string]string{
				"p1": "2", // on entering S0 and S2
				"p0": "2", // on leaving S0 and S2
				"ds": "0", // S0 left before its time: cancelled
			} {
				if last[v] != want {
					t.Errorf("%s on %s: %s = %s, want %s", r.Name, e.Engine, v, last[v], want)
				}
			}
			for _, v := range []string{"sd", "ds2"} {
				if last[v] == "" || last[v] == "0" {
					t.Errorf("%s on %s: %s = %q, want it to have run", r.Name, e.Engine, v, last[v])
				}
			}
			if !strings.EqualFold(last["done"], "true") {
				t.Errorf("%s on %s: not done", r.Name, e.Engine)
			}
		}
	}
}
