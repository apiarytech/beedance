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
	"strings"
	"testing"
	"time"
)

const configSource = `PROGRAM COUNT
VAR_EXTERNAL total : INT; END_VAR
VAR n : INT; END_VAR
n := n + 1;
total := total + 1;
END_PROGRAM

PROGRAM SLOW
VAR_EXTERNAL total : INT; END_VAR
VAR seen : INT; END_VAR
seen := total;
END_PROGRAM

PROGRAM OTHER
VAR k : INT; END_VAR
k := k + 1;
END_PROGRAM

CONFIGURATION CELL
VAR_GLOBAL total : INT; END_VAR
RESOURCE R1 ON PLC
TASK FAST(INTERVAL := T#10ms, PRIORITY := 1);
TASK LAZY(INTERVAL := T#30ms, PRIORITY := 2);
PROGRAM P1 WITH FAST : COUNT;
PROGRAM P2 WITH FAST : COUNT;
PROGRAM P3 WITH LAZY : SLOW;
END_RESOURCE
END_CONFIGURATION
`

func loadConfig(t *testing.T, src string, o Options) *Session {
	t.Helper()
	o.Program = "CELL"
	s, err := Load(src, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// A CONFIGURATION runs its instances on their tasks, a scan a cycle; a
// breakpoint on a line of a program stops each instance of it in turn,
// and the body's frame names the instance and lists its variables.
func TestConfigurationBreakpointsByInstance(t *testing.T) {
	s := loadConfig(t, configSource, Options{})
	if !s.Configuration() || s.Program() != "CELL" {
		t.Fatalf("Configuration %v, Program %q", s.Configuration(), s.Program())
	}
	row := line(t, configSource, "n := n + 1;")
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: row}})
	if !bps[0].Verified {
		t.Fatalf("breakpoint not verified: %+v", bps[0])
	}
	ctx := context.Background()
	var names []string
	for i := 0; i < 4; i++ {
		stop, err := s.Continue(ctx)
		if err != nil || stop.Reason != OnBreakpoint || stop.Line != row {
			t.Fatalf("stop %d: %+v %v", i, stop, err)
		}
		fs := s.Frames()
		names = append(names, fs[0].Name)
		locals := s.Locals(0)
		if !hasValue(locals, "n") {
			t.Fatalf("locals of %s: %+v", fs[0].Name, locals)
		}
	}
	if got := strings.Join(names, " "); got != "R1.P1 R1.P2 R1.P1 R1.P2" {
		t.Errorf("frames %s", got)
	}
	// In the second scan P1 has run and P2 stands at its first
	// statement: three counts, the third in P2's copy of the global.
	if got := value(t, s, "R1.P1.n"); got != "2" {
		t.Errorf("R1.P1.n = %s, want 2", got)
	}
	if got := value(t, s, "total"); got != "3" {
		t.Errorf("total = %s, want 3", got)
	}
}

func hasValue(vs []Value, name string) bool {
	for _, v := range vs {
		if strings.EqualFold(v.Name, name) {
			return true
		}
	}
	return false
}

// The tasks' intervals hold: in 6 scans of 10 ms the LAZY task runs twice.
// Globals, resources and instances read and write by path.
func TestConfigurationValues(t *testing.T) {
	s := loadConfig(t, configSource, Options{})
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		if stop, err := s.StepScan(ctx); err != nil || stop.Reason != OnScanEnd {
			t.Fatalf("scan %d: %+v %v", i, stop, err)
		}
	}
	if got := value(t, s, "total"); got != "12" {
		t.Errorf("total = %s, want 12", got)
	}
	if got := value(t, s, "R1.P3.seen"); got != "8" {
		t.Errorf("R1.P3.seen = %s, want 8 (its second run, at 30 ms, after four counts)", got)
	}
	var names []string
	for _, g := range s.Globals() {
		names = append(names, g.Name)
	}
	if got := strings.Join(names, " "); got != "total R1" {
		t.Errorf("globals: %s", got)
	}
	if err := s.Set("R1.P1.n", "100"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("total", "0"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StepScan(ctx); err != nil {
		t.Fatal(err)
	}
	if got := value(t, s, "R1.P1.n"); got != "101" {
		t.Errorf("R1.P1.n = %s, want 101", got)
	}
	if got := value(t, s, "total"); got != "2" {
		t.Errorf("total = %s, want 2", got)
	}
	if v, err := s.Evaluate("R1.P2.n + total", 0); err != nil || v.Inspect() != "9" {
		t.Errorf("Evaluate: %v %v", v, err)
	}
	var watched []string
	for _, w := range s.Watched() {
		watched = append(watched, w.Name)
	}
	if got := strings.Join(watched, " "); got != "total R1.P1.n R1.P2.n R1.P3.seen" {
		t.Errorf("watched: %s", got)
	}
	if err := s.Set("R1.P1", "1"); err == nil {
		t.Error("Set wrote a program instance")
	}
}

// Online change carries each instance's variables; an instance added is
// reported so, one removed too.
func TestConfigurationOnlineChange(t *testing.T) {
	s := loadConfig(t, configSource, Options{})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := s.StepScan(ctx); err != nil {
			t.Fatal(err)
		}
	}
	edited := strings.Replace(configSource, "PROGRAM P2 WITH FAST : COUNT;", "PROGRAM P4 WITH FAST : OTHER;", 1)
	edited = strings.Replace(edited, "n := n + 1;", "n := n + 10;", 1)
	r, err := s.Change(edited)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"R1.P1.n", "R1.P3.seen", "total"} {
		if !contains(r.Kept, want) {
			t.Errorf("%s not kept: %+v", want, r)
		}
	}
	if !contains(r.Added, "R1.P4") || !contains(r.Removed, "R1.P2") {
		t.Errorf("report %+v", r)
	}
	if _, err := s.StepScan(ctx); err != nil {
		t.Fatal(err)
	}
	if got := value(t, s, "R1.P1.n"); got != "13" {
		t.Errorf("R1.P1.n = %s, want 13", got)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// A line of a program the configuration has no instance of says so, and
// the configuration runs on the VM only.
func TestConfigurationRefusals(t *testing.T) {
	s := loadConfig(t, configSource, Options{})
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: line(t, configSource, "k := k + 1;")}})
	if bps[0].Verified || !strings.Contains(bps[0].Message, "has no instance") {
		t.Errorf("breakpoint in OTHER: %+v", bps[0])
	}
	if _, err := Load(configSource, Options{Program: "CELL", Engine: EngineEvaluator}); err == nil ||
		!strings.Contains(err.Error(), "VM engine only") {
		t.Errorf("evaluator: %v", err)
	}
	// A program is still the default.
	p, err := Load(configSource, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Configuration() || p.Program() != "COUNT" {
		t.Errorf("default: %s", p.Program())
	}
}

// An exception in an instance names it, and the next scan starts a fresh
// cycle.
func TestConfigurationException(t *testing.T) {
	src := strings.Replace(configSource, "seen := total;", "seen := 1 / (total - total);", 1)
	s := loadConfig(t, src, Options{Interval: 30 * time.Millisecond})
	stop, err := s.StepScan(context.Background())
	if err != nil || stop.Reason != OnException || !strings.Contains(stop.Message, "P3") {
		t.Fatalf("%+v %v", stop, err)
	}
}
