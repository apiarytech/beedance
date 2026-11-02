/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmconfig

import (
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
)

func load(t *testing.T, source string) *Configuration {
	t.Helper()
	c, err := LoadSource(source)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	return c
}

func cycle(t *testing.T, c *Configuration, now time.Duration) {
	t.Helper()
	if err := c.Cycle(now); err != nil {
		t.Fatalf("Cycle(%v): %v", now, err)
	}
}

func integer(t *testing.T, o object.Object, ok bool) int64 {
	t.Helper()
	if !ok || o == nil {
		t.Fatalf("no value")
	}
	n, _, isInt := object.GetIntegerObjectValue(o)
	if !isInt {
		t.Fatalf("%s is not an integer", o.Inspect())
	}
	return n
}

func global(t *testing.T, c *Configuration, name string) int64 {
	t.Helper()
	v, ok := c.Global(name)
	return integer(t, v, ok)
}

func value(t *testing.T, c *Configuration, path, name string) int64 {
	t.Helper()
	inst := c.Instance(path)
	if inst == nil {
		t.Fatalf("no instance %s", path)
	}
	v, ok := inst.Value(name)
	return integer(t, v, ok)
}

// The globals of a configuration and its resource are shared, and an
// instance's connections are made on every run.
func TestGlobalsAndConnections(t *testing.T) {
	c := load(t, `
PROGRAM MAIN
VAR_INPUT k : INT; END_VAR
VAR_OUTPUT out : INT; END_VAR
VAR_EXTERNAL rg : INT; END_VAR
out := rg + k;
rg := rg + 1;
END_PROGRAM
CONFIGURATION CF
VAR_GLOBAL cg : INT; g : INT := 9; END_VAR
RESOURCE R1 ON PLC
VAR_GLOBAL rg : INT := 7; END_VAR
TASK T1(INTERVAL := T#10ms, PRIORITY := 1);
PROGRAM P1 WITH T1 : MAIN(k := g, out => cg);
END_RESOURCE
END_CONFIGURATION`)
	cycle(t, c, 0)
	if got := global(t, c, "cg"); got != 16 {
		t.Fatalf("cg = %d, want 16 (rg 7 + g 9)", got)
	}
	if got := global(t, c, "rg"); got != 8 {
		t.Fatalf("rg = %d, want 8", got)
	}
	// An input reads its global every run.
	c.SetGlobal("g", &object.LInt{Value: 100})
	cycle(t, c, 10*time.Millisecond)
	if got := global(t, c, "cg"); got != 108 {
		t.Fatalf("cg = %d, want 108 (rg 8 + g 100)", got)
	}
}

// Tasks run when due, in PRIORITY order; SINGLE runs on a rising edge; a
// program with no task runs every cycle.
func TestTasks(t *testing.T) {
	c := load(t, `
PROGRAM COUNT
VAR_OUTPUT n : INT; END_VAR
VAR_EXTERNAL trace : DINT; END_VAR
n := n + 1;
END_PROGRAM
PROGRAM FIRST
VAR_EXTERNAL trace : DINT; END_VAR
trace := trace * 10 + 1;
END_PROGRAM
PROGRAM SECOND
VAR_EXTERNAL trace : DINT; END_VAR
trace := trace * 10 + 2;
END_PROGRAM
CONFIGURATION CF
VAR_GLOBAL trig : BOOL; trace : DINT; END_VAR
RESOURCE R1 ON PLC
TASK SLOW(INTERVAL := T#100ms, PRIORITY := 5);
TASK FAST(INTERVAL := T#10ms, PRIORITY := 1);
TASK EV(SINGLE := trig, PRIORITY := 2);
PROGRAM PS WITH SLOW : COUNT;
PROGRAM PF WITH FAST : COUNT;
PROGRAM PE WITH EV : COUNT;
PROGRAM PB : COUNT;
PROGRAM P2 WITH SLOW : SECOND;
PROGRAM P1 WITH FAST : FIRST;
END_RESOURCE
END_CONFIGURATION`)
	cycle(t, c, 0)
	// FAST (priority 1) ran before SLOW (5): 1, then 2.
	if got := global(t, c, "trace"); got != 12 {
		t.Errorf("trace = %d, want 12: FIRST before SECOND", got)
	}
	for i := 1; i < 10; i++ {
		cycle(t, c, time.Duration(i)*10*time.Millisecond)
	}
	for path, want := range map[string]int64{"R1.PS": 1, "R1.PF": 10, "R1.PE": 0, "R1.PB": 10} {
		if got := value(t, c, path, "n"); got != want {
			t.Errorf("%s ran %d times, want %d", path, got, want)
		}
	}
	c.SetGlobal("trig", &object.Boolean{Value: true})
	cycle(t, c, 100*time.Millisecond)
	cycle(t, c, 110*time.Millisecond) // still TRUE: no new edge
	if got := value(t, c, "R1.PE", "n"); got != 1 {
		t.Errorf("PE ran %d times on one rising edge, want 1", got)
	}
}

// VAR_CONFIG gives initial values, members of instances and addresses, and
// the instances share one process image; timers follow the cycle's time.
func TestVarConfigAndProcessImage(t *testing.T) {
	c := load(t, `
PROGRAM WRITER
VAR y AT %Q* : INT; t : TON; k : INT := 1; END_VAR
VAR_OUTPUT done : BOOL; END_VAR
t(IN := TRUE);
y := k;
done := t.Q;
END_PROGRAM
PROGRAM READER
VAR x AT %QW10 : INT; END_VAR
VAR_OUTPUT seen : INT; END_VAR
seen := x;
END_PROGRAM
CONFIGURATION CF
RESOURCE R1 ON PLC
TASK T1(INTERVAL := T#10ms, PRIORITY := 1);
PROGRAM W WITH T1 : WRITER;
PROGRAM RD WITH T1 : READER;
END_RESOURCE
VAR_CONFIG
  R1.W.y AT %QW10 : INT;
  R1.W.k : INT := 42;
  R1.W.t.PT : TIME := T#30ms;
END_VAR
END_CONFIGURATION`)
	cycle(t, c, 0)
	if got := value(t, c, "R1.RD", "seen"); got != 42 {
		t.Fatalf("READER saw %d at %%QW10, want 42", got)
	}
	done := func() bool {
		v, _ := c.Instance("R1.W").Value("done")
		return object.IsTruthy(v)
	}
	cycle(t, c, 20*time.Millisecond)
	if done() {
		t.Fatal("the timer of PT T#30ms is done after 20ms")
	}
	cycle(t, c, 40*time.Millisecond)
	if !done() {
		t.Fatal("the timer of PT T#30ms is not done after 40ms")
	}
}

// Errors in a configuration are reported.
func TestErrors(t *testing.T) {
	prog := "PROGRAM MAIN VAR_INPUT k : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := k; END_PROGRAM\n"
	for config, want := range map[string]string{
		"": "no CONFIGURATION",
		"CONFIGURATION C RESOURCE RES ON PLC PROGRAM P WITH NOPE : MAIN; END_RESOURCE END_CONFIGURATION":                                          "does not declare",
		"CONFIGURATION C RESOURCE RES ON PLC PROGRAM P : MAIN(z := 1); END_RESOURCE END_CONFIGURATION":                                            "has no input z",
		"CONFIGURATION C RESOURCE RES ON PLC PROGRAM P : MAIN(o => nowhere); END_RESOURCE END_CONFIGURATION":                                      "not a global variable",
		"CONFIGURATION C RESOURCE RES ON PLC TASK T(INTERVAL := T#10ms, PRIORITY := -1); PROGRAM P WITH T : MAIN; END_RESOURCE END_CONFIGURATION": "PRIORITY",
	} {
		if _, err := LoadSource(prog + config); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: expected an error containing %q, got %v", config, want, err)
		}
	}
}

// A cycle run instance by instance (Begin, Next, In, Out) runs what Cycle
// runs, in its order, with the tasks' triggers read as Cycle reads them.
func TestCycleRunMatchesCycle(t *testing.T) {
	const source = `
PROGRAM COUNT
VAR_EXTERNAL n : INT; trig : BOOL; END_VAR
n := n + 1;
trig := (n MOD 3) = 0;
END_PROGRAM
PROGRAM SLOW
VAR_EXTERNAL n : INT; s : INT; END_VAR
s := s * 10 + n MOD 10;
END_PROGRAM
PROGRAM ONCE
VAR_EXTERNAL o : INT; END_VAR
o := o + 1;
END_PROGRAM
PROGRAM BG
VAR_EXTERNAL b : INT; n : INT; END_VAR
b := b + n;
END_PROGRAM
CONFIGURATION CF
VAR_GLOBAL n : INT; trig : BOOL; s : INT; o : INT; b : INT; END_VAR
RESOURCE R1 ON PLC
TASK FAST(INTERVAL := T#10ms, PRIORITY := 1);
TASK EDGE(SINGLE := trig, PRIORITY := 2);
PROGRAM P1 WITH FAST : COUNT;
PROGRAM P2 WITH EDGE : ONCE;
PROGRAM P3 : BG;
END_RESOURCE
RESOURCE R2 ON PLC
TASK LAZY(INTERVAL := T#30ms, PRIORITY := 0);
PROGRAM P4 WITH LAZY : SLOW;
END_RESOURCE
END_CONFIGURATION
`
	a, b := load(t, source), load(t, source)
	var order []string
	for i := 0; i < 12; i++ {
		now := time.Duration(i) * 10 * time.Millisecond
		cycle(t, a, now)
		cy := b.Begin(now)
		for inst := cy.Next(); inst != nil; inst = cy.Next() {
			if i == 0 || i == 2 {
				task := ""
				if cy.Task() != nil {
					task = cy.Task().Name
				}
				order = append(order, cy.Resource().Name+"/"+task+"/"+inst.Path())
			}
			inst.In()
			if err := inst.VM().Run(); err != nil {
				t.Fatal(cy.Wrap(err))
			}
			inst.Out()
		}
		for _, name := range []string{"n", "s", "o", "b"} {
			if x, y := global(t, a, name), global(t, b, name); x != y {
				t.Fatalf("cycle %d: %s is %d after Cycle, %d run by instance", i, name, x, y)
			}
		}
	}
	want := "R1/FAST/R1.P1 R1//R1.P3 R2/LAZY/R2.P4 R1/FAST/R1.P1 R1/EDGE/R1.P2 R1//R1.P3"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order:\n got %s\nwant %s", got, want)
	}
}

// LoadOptions.AllowInputWrites reaches the compiler (IEC-69): a function
// that assigns its own input loads with it and is refused without it.
func TestAllowInputWrites(t *testing.T) {
	const source = `
FUNCTION BUMP : INT
VAR_INPUT x : INT; END_VAR
x := x + 1;
BUMP := x;
END_FUNCTION
PROGRAM MAIN
VAR k : INT; END_VAR
k := BUMP(k);
END_PROGRAM
CONFIGURATION CF
RESOURCE R1 ON PLC
PROGRAM P1 : MAIN;
END_RESOURCE
END_CONFIGURATION
`
	if _, err := LoadSource(source); err == nil {
		t.Fatal("LoadSource accepted a write to an input")
	}
	c, err := LoadSourceWith(source, LoadOptions{AllowInputWrites: true})
	if err != nil {
		t.Fatalf("LoadSourceWith: %v", err)
	}
	cycle(t, c, 0)
	cycle(t, c, 10*time.Millisecond)
	if got := value(t, c, "R1.P1", "k"); got != 2 {
		t.Errorf("k = %d, want 2", got)
	}
}
