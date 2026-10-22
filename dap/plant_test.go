/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

import (
	"testing"
)

const plantProgram = `PROGRAM Main
VAR
    level AT %IW0 : INT;
    fill AT %QX0.0 : BOOL;
END_VAR
fill := level < 100;
END_PROGRAM

PROGRAM PLANT_Tank
VAR
    level AT %IW0 : INT;
    fill AT %QX0.0 : BOOL;
    volume : INT;
END_VAR
IF fill THEN volume := volume + 3; END_IF;
level := volume;
END_PROGRAM
`

// TestPlantTraceSpeed: a plant model runs with the program and shows as a
// scope; the trace records signals every scan; the speed changes.
func TestPlantTraceSpeed(t *testing.T) {
	c := newClient(t)
	c.ok("initialize", map[string]any{"adapterID": "beedance"}, nil)
	c.ok("launch", map[string]any{"program": "main.st", "source": plantProgram, "speed": 0, "stopOnEntry": true}, nil)
	c.event("initialized")
	c.ok("configurationDone", nil, nil)
	c.event("stopped")

	var tr struct {
		Signals []string
		Scans   []int
		TimesMs []float64
		Values  [][]any
	}
	c.ok("beedance/trace", map[string]any{"signals": []string{"level", "fill", "PLANT_Tank.volume"}, "depth": 100}, &tr)
	if len(tr.Scans) != 0 || len(tr.Signals) != 3 {
		t.Fatalf("a new trace %+v", tr)
	}
	if m := c.request("beedance/trace", map[string]any{"signals": []string{"nothere"}}); m.Success {
		t.Error("a trace of nothing there")
	}
	for range 40 {
		c.ok("beedance/stepScan", nil, nil)
		c.event("stopped")
	}
	c.ok("beedance/trace", map[string]any{"scans": 5}, &tr)
	if len(tr.Scans) != 5 || tr.Scans[4] != 40 || tr.Values[2][4] != 102.0 || tr.Values[1][4] != false {
		t.Errorf("trace %+v: want the tank at 102, filling stopped", tr)
	}

	vs, _ := c.vars(c.top().ID, "PLANT PLANT_Tank")
	if vs["volume"].Value != "102" || vs["volume"].EvaluateName != "PLANT_Tank.volume" {
		t.Errorf("the plant's scope %+v", vs)
	}
	_, ref := c.vars(c.top().ID, "PLANT PLANT_Tank")
	if m := c.request("setVariable", map[string]any{"variablesReference": ref, "name": "volume", "value": "5"}); m.Success {
		t.Error("a plant's variable was written")
	}

	var sp struct{ Speed float64 }
	c.ok("beedance/speed", map[string]any{"factor": 2}, &sp)
	if sp.Speed != 2 {
		t.Errorf("speed %v", sp.Speed)
	}
}
