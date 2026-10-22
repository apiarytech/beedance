/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A tank the program fills to a level: the plant model integrates the
// valve's flow on the simulated clock.
const tank = `PROGRAM Control
VAR
    level AT %IW0 : INT;      (* tenths of a percent *)
    valve AT %QX0.0 : BOOL;
    target : INT := 500;
END_VAR
valve := level < target;
END_PROGRAM

PROGRAM PLANT_Tank
VAR
    level_out AT %IW0 : INT;
    valve_in AT %QX0.0 : BOOL;
    level : REAL := 100.0;    (* tenths of a percent *)
    last : TIME;
    dt : REAL;
END_VAR
dt := TIME_TO_REAL(TIME() - last) / 1000.0;
last := TIME();
IF valve_in THEN
    level := level + 200.0 * dt;   (* 20 % a second *)
END_IF;
level := level - 20.0 * dt;        (* the drain: 2 % a second *)
level_out := REAL_TO_INT(level);
END_PROGRAM
`

func TestPlant(t *testing.T) {
	s, err := Load(tank, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Program() != "Control" || strings.Join(s.Plants(), ",") != "PLANT_Tank" {
		t.Fatalf("program %s, plants %v", s.Program(), s.Plants())
	}
	ctx := context.Background()
	if err := s.SetTrace([]string{"level", "valve", "PLANT_Tank.level", "%QX0.0"}, 50); err != nil {
		t.Fatal(err)
	}
	for range 300 { // 3 s
		stopped(t)(s.StepScan(ctx))
	}
	if lvl := num(t, value(t, s, "level")); lvl < 480 || lvl > 520 {
		t.Errorf("level %v after 3 s, want about 500: the program holds it", lvl)
	}
	tr := s.TraceData(10)
	if len(tr.Scans) != 10 || tr.Scans[9] != 300 || tr.TimesMs[9] != 3000 || len(tr.Values) != 4 {
		t.Fatalf("trace %+v", tr)
	}
	if real, ok := tr.Values[2][9].(float64); !ok || real < 480 || real > 520 {
		t.Errorf("PLANT_Tank.level traced %v", tr.Values[2][9])
	}
	if tr.Values[1][9] != tr.Values[3][9] {
		t.Errorf("valve %v and %%QX0.0 %v differ", tr.Values[1][9], tr.Values[3][9])
	}
	all := s.TraceData(0)
	if len(all.Scans) != 50 || all.Scans[0] != 251 {
		t.Errorf("the trace keeps %d scans from %v, want the last 50", len(all.Scans), all.Scans[:1])
	}
	vals := map[string]string{}
	for _, v := range s.PlantGlobals(0) {
		vals[v.Name] = v.Text()
	}
	if vals["valve_in"] == "" || vals["level"] == "" {
		t.Errorf("plant globals %v", vals)
	}

	// Online change: a new target; the plant keeps its level.
	changed := strings.Replace(tank, "target : INT := 500;", "target : INT := 800;", 1)
	changed = strings.Replace(changed, "valve := level < target;", "valve := level < 800;", 1)
	if _, err := s.Change(changed); err != nil {
		t.Fatal(err)
	}
	stopped(t)(s.StepScan(ctx))
	if v := s.PlantGlobals(0); v[2].Name != "level" || num(t, v[2].Text()) < 480 {
		t.Errorf("the plant's %s after online change: %s, want it kept near 500", v[2].Name, v[2].Text())
	}
	for range 300 {
		stopped(t)(s.StepScan(ctx))
	}
	if lvl := num(t, value(t, s, "level")); lvl < 780 || lvl > 820 {
		t.Errorf("level %v after the change, want about 800", lvl)
	}
	if tr := s.TraceData(1); len(tr.Values) != 4 || tr.Values[0][0] == nil {
		t.Errorf("the trace after online change: %+v", tr)
	}
	if err := s.SetTrace([]string{"nothere"}, 0); err == nil {
		t.Error("tracing a variable that is not there")
	}
	if err := s.SetTrace([]string{"PLANT_Nope.x"}, 0); err == nil {
		t.Error("tracing a plant that is not there")
	}
}

// num is a value's text as a number.
func num(t *testing.T, text string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		t.Fatalf("%q is not a number", text)
	}
	return f
}

// TestPlantFails: a plant model that fails stops the run as an exception.
func TestPlantFails(t *testing.T) {
	src := "PROGRAM Main\nVAR x : INT; END_VAR\nx := x + 1;\nEND_PROGRAM\nPROGRAM PLANT_Bad\nVAR a : ARRAY[1..2] OF INT; i : INT := 5; END_VAR\na[i] := 1;\nEND_PROGRAM\n"
	s, err := Load(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := stopped(t)(s.StepScan(context.Background()))
	if st.Reason != OnException || !strings.Contains(st.Message, "PLANT_Bad") {
		t.Errorf("stop %+v", st)
	}
}

// TestSpeed: at a speed, scans follow the wall clock; at 0, as fast as they go.
func TestSpeed(t *testing.T) {
	s := load(t, source)
	if s.Speed() != 0 {
		t.Fatalf("speed %v without Realtime", s.Speed())
	}
	count := func(speed float64) int {
		s.SetSpeed(speed)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		before := s.Scans()
		stopped(t)(s.Continue(ctx))
		return s.Scans() - before
	}
	real, half, fast := count(1), count(0.5), count(0)
	if real < 12 || real > 24 || half < 6 || half > 13 || fast < 200 {
		t.Errorf("scans in 200 ms: %d at real time (10 ms scans), %d at half, %d as fast as they go", real, half, fast)
	}
	if s2, _ := Load(source, Options{Realtime: true}); s2.Speed() != 1 {
		t.Errorf("Realtime loads at speed %v", s2.Speed())
	}
}

// TestBreakpointElsewhere: a breakpoint in a PROGRAM that does not run, or
// in a plant model, says so.
func TestBreakpointElsewhere(t *testing.T) {
	src := "PROGRAM Main\nVAR n : INT; END_VAR\nn := n + 1;\nEND_PROGRAM\n" +
		"PROGRAM Second\nVAR m : INT; END_VAR\nm := m + 1;\nEND_PROGRAM\n" +
		"PROGRAM PLANT_P\nVAR k : INT; END_VAR\nk := k + 1;\nEND_PROGRAM\n"
	s := load(t, src)
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: 3}, {Line: 7}, {Line: 11}})
	if !bps[0].Verified || bps[1].Verified || bps[2].Verified {
		t.Fatalf("breakpoints %+v", bps)
	}
	if !strings.Contains(bps[1].Message, "PROGRAM Second does not run in this session: it runs PROGRAM Main") {
		t.Errorf("in Second: %q", bps[1].Message)
	}
	if !strings.Contains(bps[2].Message, "PLANT_P is a plant model") {
		t.Errorf("in the plant: %q", bps[2].Message)
	}
}
