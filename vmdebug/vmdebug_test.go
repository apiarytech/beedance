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
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const source = `FUNCTION_BLOCK Counter
VAR_INPUT step : INT; END_VAR
VAR_OUTPUT total : INT; END_VAR
VAR tmp : INT; END_VAR
tmp := total + step;
total := tmp;
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    n : INT;
    c : Counter;
    t : TON;
    done : BOOL;
    x AT %IX0.0 : BOOL;
    y AT %QX0.0 : BOOL;
END_VAR
n := n + 1;
c(step := 2);
t(IN := TRUE, PT := T#50ms);
done := t.Q;
y := x;
END_PROGRAM
`

// line is the line of source that starts with text.
func line(t *testing.T, src, text string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), text) {
			return i + 1
		}
	}
	t.Fatalf("no line %q", text)
	return 0
}

func load(t *testing.T, src string) *Session {
	t.Helper()
	s, err := Load(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// value is the text of name's value, as Globals or Watched show it.
func value(t *testing.T, s *Session, name string) string {
	t.Helper()
	path := strings.Split(name, ".")
	s.mu.Lock()
	o, ok := s.lookup(path)
	s.mu.Unlock()
	if !ok {
		t.Fatalf("no %s", name)
	}
	return Format(o)
}

// stopped is where a run stopped, failing t on an error.
func stopped(t *testing.T) func(Stop, error) Stop {
	return func(stop Stop, err error) Stop {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return stop
	}
}

var ctx = context.Background()

func TestBreakpointStopsBeforeTheStatement(t *testing.T) {
	s := load(t, source)
	ln := line(t, source, "n := n + 1")
	bps := s.SetBreakpoints([]BreakpointSpec{{Line: ln}})
	if !bps[0].Verified || bps[0].Actual != ln {
		t.Fatalf("breakpoint %+v", bps[0])
	}
	for scan := 1; scan <= 3; scan++ {
		st := stopped(t)(s.Continue(ctx))
		if st.Reason != OnBreakpoint || st.Line != ln || st.Scan != scan {
			t.Fatalf("scan %d: stop %+v", scan, st)
		}
		if got, want := value(t, s, "n"), []string{"0", "1", "2"}[scan-1]; got != want {
			t.Errorf("scan %d: n = %s before the statement, want %s", scan, got, want)
		}
	}
	if fs := s.Frames(); len(fs) != 1 || fs[0].Name != "Main" || fs[0].Line != ln {
		t.Errorf("frames %+v", fs)
	}
}

func TestBreakpointMovesToTheNextStatement(t *testing.T) {
	s := load(t, source)
	bps := s.SetBreakpoints([]BreakpointSpec{
		{Line: line(t, source, "VAR tmp")},      // a declaration: on to the body
		{Line: line(t, source, "END_PROGRAM")},  // nothing after it
		{Line: line(t, source, "PROGRAM Main")}, // the program's declarations, then its body
	})
	if !bps[0].Verified || bps[0].Actual != line(t, source, "tmp := total") {
		t.Errorf("in Counter: %+v", bps[0])
	}
	if bps[1].Verified {
		t.Errorf("after the end: %+v", bps[1])
	}
	if !bps[2].Verified || bps[2].Actual != line(t, source, "n := n + 1") {
		t.Errorf("in Main: %+v", bps[2])
	}
}

func TestConditionalBreakpoint(t *testing.T) {
	s := load(t, source)
	ln := line(t, source, "c(step")
	s.SetBreakpoints([]BreakpointSpec{{Line: ln, Condition: "n = 3 AND c.total = 4"}})
	st := stopped(t)(s.Continue(ctx))
	if st.Reason != OnBreakpoint || st.Scan != 3 {
		t.Fatalf("stop %+v", st)
	}
	if bad := s.SetBreakpoints([]BreakpointSpec{{Line: ln, Condition: "n ="}}); bad[0].Verified || bad[0].Message == "" {
		t.Errorf("a condition that does not parse: %+v", bad[0])
	}
}

func TestBreakpointInAFunctionBlock(t *testing.T) {
	s := load(t, source)
	ln := line(t, source, "total := tmp")
	s.SetBreakpoints([]BreakpointSpec{{Line: ln}})
	st := stopped(t)(s.Continue(ctx))
	if st.Reason != OnBreakpoint || st.Line != ln {
		t.Fatalf("stop %+v", st)
	}
	fs := s.Frames()
	if len(fs) != 2 || fs[0].Name != "Counter" || fs[1].Name != "Main" || fs[1].Line != line(t, source, "c(step") {
		t.Fatalf("frames %+v", fs)
	}
	locals := map[string]string{}
	for _, v := range s.Locals(0) {
		locals[v.Name] = v.Text()
	}
	if locals["tmp"] != "2" || locals["total"] != "0" || locals["step"] != "2" {
		t.Errorf("locals %v", locals)
	}
	if o, err := s.Evaluate("tmp * 10 + n", 0); err != nil || Format(o) != "21" {
		t.Errorf("evaluate in Counter: %v, %v", o, err)
	}
}

func TestStepping(t *testing.T) {
	s := load(t, source)
	call := line(t, source, "c(step")
	s.SetBreakpoints([]BreakpointSpec{{Line: call}})
	stopped(t)(s.Continue(ctx))

	st := stopped(t)(s.StepIn(ctx))
	if st.Reason != OnStep || st.Line != line(t, source, "tmp := total") || s.Frames()[0].Name != "Counter" {
		t.Fatalf("step in: %+v %+v", st, s.Frames())
	}
	st = stopped(t)(s.StepOver(ctx))
	if st.Line != line(t, source, "total := tmp") {
		t.Fatalf("step over in Counter: %+v", st)
	}
	st = stopped(t)(s.StepOut(ctx))
	if st.Line != line(t, source, "t(IN") || len(s.Frames()) != 1 {
		t.Fatalf("step out: %+v %+v", st, s.Frames())
	}
	if value(t, s, "c.total") != "2" {
		t.Errorf("c.total = %s after the call", value(t, s, "c.total"))
	}

	// Over the call, on the next scan.
	stopped(t)(s.Continue(ctx))
	st = stopped(t)(s.StepOver(ctx))
	if st.Line != line(t, source, "t(IN") || st.Scan != 2 {
		t.Fatalf("step over the call: %+v", st)
	}
	// Past the body's last statement into the next scan's first.
	for _, want := range []string{"done :=", "y := x", "n := n + 1"} {
		st = stopped(t)(s.StepOver(ctx))
		if st.Line != line(t, source, want) {
			t.Fatalf("step to %q: %+v", want, st)
		}
	}
	if st.Scan != 3 {
		t.Errorf("stepped into scan %d, want 3", st.Scan)
	}
}

func TestStepScan(t *testing.T) {
	s := load(t, source)
	for i := 1; i <= 3; i++ {
		st := stopped(t)(s.StepScan(ctx))
		if st.Reason != OnScanEnd || st.Scan != i || st.Line != 0 {
			t.Fatalf("scan %d: %+v", i, st)
		}
	}
	if value(t, s, "n") != "3" || s.Elapsed() != 30*time.Millisecond {
		t.Errorf("n = %s, elapsed %v", value(t, s, "n"), s.Elapsed())
	}
	// From inside a scan, to its end.
	s.SetBreakpoints([]BreakpointSpec{{Line: line(t, source, "c(step")}})
	stopped(t)(s.Continue(ctx))
	st := stopped(t)(s.StepScan(ctx))
	if st.Reason != OnScanEnd || st.Scan != 4 || value(t, s, "c.total") != "8" {
		t.Errorf("to the end of scan 4: %+v, c.total = %s", st, value(t, s, "c.total"))
	}
}

// A run paused at every statement ends as one that never paused.
func TestPausingChangesNothing(t *testing.T) {
	plain, paused := load(t, source), load(t, source)
	var all []BreakpointSpec
	for l := 1; l <= strings.Count(source, "\n"); l++ {
		all = append(all, BreakpointSpec{Line: l})
	}
	paused.SetBreakpoints(all)
	stops := 0
	for paused.Scans() < 40 || paused.midScan {
		st := stopped(t)(paused.Continue(ctx))
		if st.Reason != OnBreakpoint {
			t.Fatalf("stop %+v", st)
		}
		stops++
		if paused.Scans() == 40 {
			paused.SetBreakpoints(nil)
			stopped(t)(paused.StepScan(ctx))
		}
	}
	for range 40 {
		stopped(t)(plain.StepScan(ctx))
	}
	if stops < 40*5 {
		t.Errorf("%d stops in 40 scans", stops)
	}
	for _, name := range []string{"n", "c.total", "t.ET", "t.Q", "done"} {
		if a, b := value(t, plain, name), value(t, paused, name); a != b {
			t.Errorf("%s: %s without pausing, %s paused", name, a, b)
		}
	}
}

func TestPause(t *testing.T) {
	s := load(t, source)
	s.Pause()
	st := stopped(t)(s.Continue(ctx))
	if st.Reason != OnPause || st.Scan != 0 {
		t.Fatalf("paused before the first scan: %+v", st)
	}
	done := make(chan Stop)
	go func() {
		st, _ := s.Continue(ctx)
		done <- st
	}()
	time.Sleep(20 * time.Millisecond)
	s.Pause()
	select {
	case st = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Pause did not stop Continue")
	}
	if st.Reason != OnPause || st.Scan == 0 {
		t.Errorf("stop %+v", st)
	}
	c, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if st = stopped(t)(s.Continue(c)); st.Reason != OnPause {
		t.Errorf("at the context's end: %+v", st)
	}
}

func TestForce(t *testing.T) {
	s := load(t, source)
	if err := s.Force("%IX0.0", "TRUE"); err != nil {
		t.Fatal(err)
	}
	if err := s.Force("n", "100"); err != nil {
		t.Fatal(err)
	}
	s.SetBreakpoints([]BreakpointSpec{{Line: line(t, source, "c(step")}})
	stopped(t)(s.Continue(ctx))
	if value(t, s, "n") != "101" {
		t.Errorf("in the scan, n = %s: the program reads the force, then writes", value(t, s, "n"))
	}
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "n") != "100" || value(t, s, "y") != "TRUE" {
		t.Errorf("after the scan: n = %s, y = %s", value(t, s, "n"), value(t, s, "y"))
	}
	forced := map[string]bool{}
	for _, v := range s.Globals() {
		forced[v.Name] = v.Forced
	}
	if !forced["n"] || !forced["x"] || forced["y"] {
		t.Errorf("forced %v", forced)
	}
	if err := s.Unforce("n"); err != nil {
		t.Fatal(err)
	}
	s.SetBreakpoints(nil)
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "n") != "101" {
		t.Errorf("unforced, n = %s", value(t, s, "n"))
	}
	if err := s.Force("n", "TRUE"); err == nil {
		t.Error("forced an INT with TRUE")
	}
	if err := s.Force("nope", "1"); err == nil {
		t.Error("forced a variable that is not there")
	}
}

func TestSet(t *testing.T) {
	s := load(t, source)
	for name, v := range map[string]string{"n": "16#10", "c.total": "INT#40", "t.PT": "T#1s"} {
		if err := s.Set(name, v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "n") != "17" || value(t, s, "c.total") != "42" {
		t.Errorf("n = %s, c.total = %s", value(t, s, "n"), value(t, s, "c.total"))
	}
}

func TestException(t *testing.T) {
	src := `PROGRAM Main
VAR i : INT; END_VAR
i := i + 1;
WHILE TRUE DO
    i := i + 1;
END_WHILE;
END_PROGRAM
`
	s, err := Load(src, Options{Budget: 10_000})
	if err != nil {
		t.Fatal(err)
	}
	st := stopped(t)(s.Continue(ctx))
	if st.Reason != OnException || !strings.Contains(st.Message, "budget") || st.Line == 0 {
		t.Fatalf("stop %+v", st)
	}
}

func TestChangeKeepsTheValues(t *testing.T) {
	s := load(t, source)
	for range 3 {
		stopped(t)(s.StepScan(ctx))
	}
	etBefore := value(t, s, "t.ET")
	edited := strings.Replace(source, "n := n + 1;", "n := n + 10;", 1)
	edited = strings.Replace(edited, "total := tmp;", "total := tmp * 2;", 1)
	edited = strings.Replace(edited, "    done : BOOL;", "    done : INT;\n    m : INT := 7;", 1)
	edited = strings.Replace(edited, "done := t.Q;", "done := BOOL_TO_INT(t.Q);", 1)
	s.Force("n", "5")
	s.Force("done", "TRUE")
	r, err := s.Change(edited)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Kept, "n") || !slices.Contains(r.Kept, "c") || !slices.Contains(r.Kept, "t") ||
		!slices.Contains(r.Reset, "done") || !slices.Equal(r.Added, []string{"m"}) || len(r.Removed) != 0 ||
		!slices.Equal(r.Unforced, []string{"done"}) {
		t.Errorf("report %+v", r)
	}
	s.Unforce("n")
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "n") != "15" || value(t, s, "c.total") != "16" || value(t, s, "m") != "7" {
		t.Errorf("after the change: n = %s, c.total = %s, m = %s", value(t, s, "n"), value(t, s, "c.total"), value(t, s, "m"))
	}
	if etAfter := value(t, s, "t.ET"); etAfter <= etBefore {
		t.Errorf("the timer started over: ET %s before, %s after", etBefore, etAfter)
	}
	if !strings.Contains(s.Source(), "n + 10") {
		t.Error("Source is the old one")
	}

	// Breakpoints follow the new source; mid-scan, a change waits.
	s.SetBreakpoints([]BreakpointSpec{{Line: line(t, edited, "c(step")}})
	stopped(t)(s.Continue(ctx))
	if _, err := s.Change(source); !errors.Is(err, ErrMidScan) {
		t.Errorf("a change in a scan: %v", err)
	}
	if _, err := s.Change("PROGRAM Other END_PROGRAM"); err == nil {
		t.Error("a source without the program changed it")
	}
}

func TestChangeResetsAMemberWhoseTypeChanged(t *testing.T) {
	s := load(t, source)
	for range 2 {
		stopped(t)(s.StepScan(ctx))
	}
	edited := strings.Replace(source, "VAR tmp : INT; END_VAR", "VAR tmp : REAL; END_VAR", 1)
	edited = strings.Replace(edited, "tmp := total + step;\ntotal := tmp;", "tmp := INT_TO_REAL(total + step);\ntotal := REAL_TO_INT(tmp);", 1)
	if _, err := s.Change(edited); err != nil {
		t.Fatal(err)
	}
	if value(t, s, "c.total") != "4" || !strings.HasPrefix(value(t, s, "c.tmp"), "0") {
		t.Errorf("c.total = %s (kept), c.tmp = %s (reset)", value(t, s, "c.total"), value(t, s, "c.tmp"))
	}
	stopped(t)(s.StepScan(ctx))
	if value(t, s, "c.total") != "6" {
		t.Errorf("c.total = %s after a scan of the new code", value(t, s, "c.total"))
	}
}

func TestDisassemblyAndInstructionSteps(t *testing.T) {
	s := load(t, source)
	code := s.Disassembly()
	byAddr := map[int]Instruction{}
	pous := map[string]bool{}
	for i, in := range code {
		if i > 0 && in.Address <= code[i-1].Address {
			t.Fatalf("addresses out of order at %+v", in)
		}
		byAddr[in.Address] = in
		pous[in.POU] = true
	}
	if !pous["Main"] || !pous["Counter"] || pous["TON"] {
		t.Errorf("code of %v; want Main and Counter, not the standard TON", pous)
	}
	found := false
	for _, in := range code {
		if strings.Contains(in.Text, "OpSetGlobal") && strings.Contains(in.Text, "(n)") && in.Line == line(t, source, "n := n + 1") {
			found = true
		}
	}
	if !found {
		t.Error("no OpSetGlobal (n) on the line of n := n + 1")
	}

	s.SetBreakpoints([]BreakpointSpec{{Line: line(t, source, "n := n + 1")}})
	stopped(t)(s.Continue(ctx))
	at := s.Frames()[0].Address
	if byAddr[at].Line != line(t, source, "n := n + 1") {
		t.Fatalf("stopped at %#x, %+v", at, byAddr[at])
	}
	for range 3 {
		st := stopped(t)(s.StepInstruction(ctx))
		next := s.Frames()[0].Address
		if st.Reason != OnStep || next <= at || byAddr[next].Text == "" {
			t.Fatalf("step from %#x to %#x: %+v", at, next, st)
		}
		at = next
	}
	// Into the call: the next instructions of Counter's body.
	s.SetBreakpoints([]BreakpointSpec{{Line: line(t, source, "c(step")}})
	stopped(t)(s.Continue(ctx))
	for range 50 {
		stopped(t)(s.StepInstruction(ctx))
		if s.Frames()[0].Name == "Counter" {
			return
		}
	}
	t.Error("50 instructions did not step into Counter")
}

func TestChangeWhileRunning(t *testing.T) {
	s := load(t, source)
	c, cancel := context.WithCancel(ctx)
	done := make(chan Stop)
	go func() {
		st, _ := s.Continue(c)
		done <- st
	}()
	time.Sleep(20 * time.Millisecond)
	edited := strings.Replace(source, "n := n + 1;", "n := n - 1;", 1)
	r, err := s.Change(edited)
	if err != nil || !slices.Contains(r.Kept, "n") {
		t.Fatalf("change while running: %+v, %v", r, err)
	}
	// Forces and writes while running reach the program at the next scan.
	if err := s.Force("c.total", "500"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("%IX0.0", "TRUE"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	live := s.Live()
	time.Sleep(20 * time.Millisecond)
	live = s.Live()
	cancel()
	<-done
	if live.Scan == 0 || len(live.Values) == 0 {
		t.Fatalf("live %+v", live)
	}
	stopped(t)(s.StepScan(ctx)) // to the end of the scan the run stopped in
	n, _ := strconv.Atoi(value(t, s, "n"))
	stopped(t)(s.StepScan(ctx))
	if after, _ := strconv.Atoi(value(t, s, "n")); after != n-1 {
		t.Errorf("n went from %d to %d, want %d: the new code counts down", n, after, n-1)
	}
	if value(t, s, "c.total") != "500" || value(t, s, "y") != "TRUE" {
		t.Errorf("c.total = %s (forced 500), y = %s (x set TRUE)", value(t, s, "c.total"), value(t, s, "y"))
	}
}
