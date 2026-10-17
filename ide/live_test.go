/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ide

import (
	"slices"
	"strings"
	"testing"

	"github.com/apiarytech/beedance/object"
)

func TestWatches(t *testing.T) {
	d := Analyze(ldSrc).Diagrams()[0]
	w := d.Watches()
	for _, n := range []string{"Start", "Run", "Stop", "Done", "Fault", "Mode", "t1.Q"} {
		if !slices.Contains(w, n) {
			t.Errorf("Watches = %v; lacks %s", w, n)
		}
	}
	s := Analyze(sfcSrc).Diagrams()[0].Watches()
	if !slices.Contains(s, "_sfc_Idle_X") || !slices.Contains(s, "_sfc_Busy_X") {
		t.Errorf("an SFC body's watches = %v; lack the step flags", s)
	}
}

func TestLive(t *testing.T) {
	d := Analyze(ldSrc).Diagrams()[0]
	on := &object.Boolean{Value: true}
	off := &object.Boolean{Value: false}
	v := FromMap(map[string]object.Object{"start": on, "Run": off, "Stop": off, "Done": off, "Fault": off, "Mode": &object.Int{Value: 2}, "t1.Q": off})
	live, err := d.Live(v, "")
	if err != nil {
		t.Fatal(err)
	}
	if live.Values["Start"] != "TRUE" || live.Values["Mode"] != "2" {
		t.Errorf("values = %v", live.Values)
	}
	// Each wire is worked out: [ Start | Run ] carries TRUE, and through
	// /Stop the rung powers the coil; EQ(Mode, 2) is TRUE.
	trues := 0
	for x, val := range live.Exprs {
		if val == "TRUE" {
			trues++
		}
		if strings.Contains(x, "EQ(") && val != "TRUE" {
			t.Errorf("%s = %s; want TRUE", x, val)
		}
	}
	if trues < 3 || len(live.Exprs) < 4 {
		t.Errorf("exprs = %v", live.Exprs)
	}
	for _, x := range d.exprs() {
		if _, ok := live.Exprs[x]; !ok {
			t.Errorf("the wire %q was not worked out", x)
		}
	}
}

func TestLiveSources(t *testing.T) {
	d := Analyze(sfcSrc).Diagrams()[0]
	// A controller's answer, as JSON.
	live, err := d.Live(FromJSON(map[string]string{"_sfc_Idle_X": "false", "_sfc_Busy_X": "true", "go": "true", "done": "false", "n": "7"}), "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(live.Steps, []string{"Busy"}) || live.Values["go"] != "TRUE" || live.Exprs["go"] != "TRUE" {
		t.Errorf("live = %+v", live)
	}
	// The evaluator's environment; through an instance, with a prefix.
	env := object.NewEnvironment()
	env.Set("_sfc_Idle_X", &object.Boolean{Value: true})
	live, err = d.Live(FromEvaluator(env), "")
	if err != nil || !slices.Equal(live.Steps, []string{"Idle"}) {
		t.Errorf("on the evaluator: %+v, %v", live, err)
	}
	live, err = d.Live(FromMap(map[string]object.Object{"seq1._sfc_Busy_X": &object.Boolean{Value: true}}), "seq1.")
	if err != nil || !slices.Equal(live.Steps, []string{"Busy"}) {
		t.Errorf("through an instance: %+v, %v", live, err)
	}
}

func TestExprNames(t *testing.T) {
	got := exprNames("GE(L, SP) AND NOT t1.Q OR m.p.x > INT#5 AND T#5S > hold.ET")
	if want := "L SP t1.Q m.p.x hold.ET"; strings.Join(got, " ") != want {
		t.Errorf("exprNames = %v; want %s", got, want)
	}
	if got := replaceName("Run AND Running AND x.Run", "Run", "__v0"); got != "__v0 AND Running AND x.Run" {
		t.Errorf("replaceName = %q", got)
	}
}
