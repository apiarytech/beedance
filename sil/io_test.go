/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
)

// A fill controller: open the valve below low, close it above high. The
// test passes once the tank has been filled to high and the valve closed.
const tankSource = `
PROGRAM TEST_Fill
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR
	level AT %IW0 : INT;
	full AT %IX0.1 : BOOL;
	valve AT %QX0.0 : BOOL;
	reached : BOOL;
END_VAR
IF level < 20 THEN valve := TRUE; END_IF;
IF level >= 80 THEN valve := FALSE; reached := TRUE; END_IF;
IF full THEN failures := failures + 1; message := 'overflow'; END_IF;
IF reached AND NOT valve THEN done := TRUE; END_IF;
END_PROGRAM
`

// tank is a plant model: the level rises 1 per 10 ms while the valve is
// open, and the full switch closes at 100.
type tank struct {
	mu     sync.Mutex
	level  int
	valve  bool
	begins []string
	points []Point
	ends   int
	failOn string // a method to fail
}

func (k *tank) fail(op string) error {
	if k.failOn == op {
		return errors.New(op + " failed")
	}
	return nil
}

func (k *tank) Begin(_ context.Context, test string, e Engine, points []Point) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.level, k.valve = 0, false
	k.begins = append(k.begins, test+"/"+string(e))
	k.points = points
	return k.fail("begin")
}

func (k *tank) Read(context.Context, time.Duration) (map[string]object.Object, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.valve {
		k.level++
	}
	in := map[string]object.Object{
		"%IW0":   &object.Int{Value: int16(k.level)},
		"%IX0.1": &object.Boolean{Value: k.level >= 100},
	}
	return in, k.fail("read")
}

func (k *tank) Write(_ context.Context, _ time.Duration, out map[string]object.Object) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := out["%QX0.0"].(*object.Boolean); ok {
		k.valve = v.Value
	}
	return k.fail("write")
}

func (k *tank) End(context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.ends++
	return k.fail("end")
}

func TestIOPlantModel(t *testing.T) {
	k := &tank{}
	results, err := Run(context.Background(), tankSource, Options{IO: k, Deterministic: true})
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !r.Passed {
		t.Fatalf("not passed: %+v", r)
	}
	for _, e := range r.Engines {
		// Level 0 on the first scan, then 1 per scan: 80 at scan 81, which
		// closes the valve and is done.
		if n := len(e.Scans); n != 81 {
			t.Errorf("%s: %d scans, want 81", e.Engine, n)
		}
		if got := e.Scans[len(e.Scans)-1]["%QX0.0"]; got != "FALSE" && got != "false" {
			t.Errorf("%s: valve at the end %q", e.Engine, got)
		}
	}
	if got := strings.Join(k.begins, ","); got != "TEST_Fill/eval,TEST_Fill/vm" {
		t.Errorf("begins %s", got)
	}
	if k.ends != 2 {
		t.Errorf("ends %d, want 2", k.ends)
	}
	want := map[string]string{"%IW0": "INT", "%IX0.1": "BOOL", "%QX0.0": "BOOL"}
	if len(k.points) != len(want) {
		t.Errorf("points %v", k.points)
	}
	for _, p := range k.points {
		if want[p.Address] != p.Type {
			t.Errorf("point %v, want type %s", p, want[p.Address])
		}
	}
}

func TestIOErrors(t *testing.T) {
	for _, op := range []string{"begin", "read", "write", "end"} {
		t.Run(op, func(t *testing.T) {
			k := &tank{failOn: op}
			results, err := Run(context.Background(), tankSource, Options{IO: k, Engines: []Engine{VM, Evaluator}})
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range results[0].Engines {
				if e.Passed || !strings.Contains(e.Err, "io "+op) {
					t.Errorf("%s: passed %v, err %q", e.Engine, e.Passed, e.Err)
				}
			}
			if op != "begin" && k.ends != 2 {
				t.Errorf("ends %d, want 2: End runs after a failed run", k.ends)
			}
		})
	}
}

// An input that is not in the input area is refused.
type writesOutput struct{ tank }

func (w *writesOutput) Read(context.Context, time.Duration) (map[string]object.Object, error) {
	return map[string]object.Object{"%QX0.0": &object.Boolean{Value: true}}, nil
}

func TestIOReadOnlyInputs(t *testing.T) {
	results, err := Run(context.Background(), tankSource, Options{IO: &writesOutput{}, Engines: []Engine{VM}})
	if err != nil {
		t.Fatal(err)
	}
	if e := results[0].Engines[0]; !strings.Contains(e.Err, "not an input") {
		t.Errorf("err %q", e.Err)
	}
}

// Without Deterministic the engines are not compared: a rig's inputs
// differ between runs.
type noisy struct {
	tank
	runs int
}

func (n *noisy) Begin(ctx context.Context, test string, e Engine, p []Point) error {
	n.runs++
	return n.tank.Begin(ctx, test, e, p)
}

func (n *noisy) Read(ctx context.Context, t time.Duration) (map[string]object.Object, error) {
	in, err := n.tank.Read(ctx, t)
	if n.runs == 2 { // the second engine sees the level a step later
		in["%IW0"] = &object.Int{Value: int16(n.level + 1)}
	}
	return in, err
}

func TestIONotCompared(t *testing.T) {
	results, err := Run(context.Background(), tankSource, Options{IO: &noisy{}})
	if err != nil {
		t.Fatal(err)
	}
	if r := results[0]; !r.Passed || r.Mismatch != "" {
		t.Errorf("passed %v, mismatch %q", r.Passed, r.Mismatch)
	}
	results, _ = Run(context.Background(), tankSource, Options{IO: &noisy{}, Deterministic: true})
	if r := results[0]; r.Passed || r.Mismatch == "" {
		t.Errorf("Deterministic: passed %v, mismatch %q", r.Passed, r.Mismatch)
	}
}

func TestIOGoEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go code")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no Go toolchain")
	}
	k := &tank{}
	results, err := Run(context.Background(), tankSource, Options{IO: k, Deterministic: true,
		Engines: []Engine{Evaluator, VM, Go}, GoTimeout: 10 * time.Second,
		GoReplace: map[string]string{"github.com/apiarytech/royaljelly": "../../royaljelly"}})
	if err != nil {
		t.Fatal(err)
	}
	r := results[0]
	if !r.Passed {
		for _, e := range r.Engines {
			t.Logf("%s: passed %v, %d scans, err %s", e.Engine, e.Passed, len(e.Scans), e.Err)
		}
		t.Fatalf("not passed, mismatch %q", r.Mismatch)
	}
	if got := strings.Join(k.begins, ","); got != "TEST_Fill/eval,TEST_Fill/vm,TEST_Fill/go" {
		t.Errorf("begins %s", got)
	}
	if g := r.Engines[2]; len(g.Scans) != 81 || !strings.EqualFold(g.Scans[80]["%QX0.0"], "false") {
		t.Errorf("go: %d scans, last %v", len(g.Scans), g.Scans[len(g.Scans)-1])
	}
}

func TestRealTime(t *testing.T) {
	src := `
PROGRAM TEST_Wait
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR t : TON; END_VAR
t(IN := TRUE, PT := T#50ms);
done := t.Q;
END_PROGRAM
`
	start := time.Now()
	results, err := Run(context.Background(), src, Options{RealTime: true, Interval: 5 * time.Millisecond, Engines: []Engine{VM}})
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Passed {
		t.Fatalf("%+v", results[0])
	}
	if took := time.Since(start); took < 50*time.Millisecond {
		t.Errorf("took %v: scans not paced", took)
	}
	if n := len(results[0].Engines[0].Scans); n > 12 {
		t.Errorf("%d scans: the clock is not the wall clock", n)
	}
}

func TestRealTimeCancel(t *testing.T) {
	src := `
PROGRAM TEST_Never
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
END_PROGRAM
`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	results, _ := Run(ctx, src, Options{RealTime: true, Interval: time.Second, Engines: []Engine{VM}})
	if len(results) != 1 || results[0].Passed {
		t.Fatalf("%+v", results)
	}
}

// The rig protocol carries the plant model over TCP.
func TestRig(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	k := &tank{}
	served := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			served <- err
			return
		}
		defer conn.Close()
		served <- ServeRig(context.Background(), conn, k)
	}()
	rig, err := DialRig(context.Background(), "tcp://"+ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	results, err := Run(context.Background(), tankSource, Options{IO: rig, Deterministic: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := results[0]; !r.Passed {
		t.Fatalf("%+v", r)
	}
	rig.Close()
	if err := <-served; err != nil {
		t.Errorf("serve: %v", err)
	}
	if k.ends != 2 || len(k.begins) != 2 {
		t.Errorf("begins %v, ends %d", k.begins, k.ends)
	}
	if len(k.points) != 3 || k.points[0].Type == "" {
		t.Errorf("points over the wire %v", k.points)
	}
}

// A rig's error fails the run with its message.
func TestRigError(t *testing.T) {
	c, s := net.Pipe()
	go ServeRig(context.Background(), s, &tank{failOn: "read"})
	results, err := Run(context.Background(), tankSource, Options{IO: NewRig(c), Engines: []Engine{VM}})
	if err != nil {
		t.Fatal(err)
	}
	if e := results[0].Engines[0]; !strings.Contains(e.Err, "rig read: read failed") {
		t.Errorf("err %q", e.Err)
	}
}

// A client that goes away within a run gets End called on the rig.
func TestRigClientGone(t *testing.T) {
	c, s := net.Pipe()
	k := &tank{}
	done := make(chan struct{})
	go func() { ServeRig(context.Background(), s, k); close(done) }()
	rig := NewRig(c)
	if err := rig.Begin(context.Background(), "T", VM, nil); err != nil {
		t.Fatal(err)
	}
	c.Close()
	<-done
	if k.ends != 1 {
		t.Errorf("ends %d, want 1", k.ends)
	}
}

func TestToObject(t *testing.T) {
	for _, c := range []struct {
		typ  string
		v    any
		want string
	}{
		{"BOOL", true, "TRUE"},
		{"INT", float64(-21), "-21"},
		{"SINT", 127, "127"},
		{"LINT", int64(math.MaxInt64), fmt.Sprint(int64(math.MaxInt64))},
		{"ULINT", uint64(math.MaxUint64), fmt.Sprint(uint64(math.MaxUint64))},
		{"WORD", 65535, ""},
		{"REAL", 1.5, ""},
		{"STRING", "hi", "hi"},
		{"TIME", "T#1.5s", "T#1.5s"},
		{"time", 2 * time.Second, "T#2s"},
	} {
		o, err := ToObject(c.typ, c.v)
		if err != nil {
			t.Errorf("%s %v: %v", c.typ, c.v, err)
			continue
		}
		if c.want != "" && !strings.EqualFold(o.Inspect(), c.want) {
			t.Errorf("%s %v: %s, want %s", c.typ, c.v, o.Inspect(), c.want)
		}
		back, err := FromObject(o)
		if err != nil {
			t.Errorf("%s back: %v", c.typ, err)
		}
		if again, err := ToObject(c.typ, back); err != nil || again.Inspect() != o.Inspect() {
			t.Errorf("%s round trip: %v %v", c.typ, again, err)
		}
	}
	for _, c := range []struct {
		typ string
		v   any
	}{
		{"INT", 40000}, {"INT", 2.5}, {"BOOL", 1}, {"USINT", -1}, {"STRUCT", 1}, {"TIME", "soon"},
	} {
		if o, err := ToObject(c.typ, c.v); err == nil {
			t.Errorf("%s %v: %v, want an error", c.typ, c.v, o)
		}
	}
}

// On the go engine with IO, GoTimeout bounds each scan.
func TestIOGoEngineRunaway(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go code")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no Go toolchain")
	}
	src := `
PROGRAM TEST_Spin
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR x AT %IX0.0 : BOOL; i : DINT; END_VAR
WHILE NOT x DO i := i + 1; END_WHILE;
END_PROGRAM
`
	k := &tank{}
	results, err := Run(context.Background(), src, Options{IO: k, Engines: []Engine{Go}, GoTimeout: time.Second,
		GoReplace: map[string]string{"github.com/apiarytech/royaljelly": "../../royaljelly"}})
	if err != nil {
		t.Fatal(err)
	}
	if e := results[0].Engines[0]; !strings.Contains(e.Err, "scan 1: no reply after 1s") {
		t.Errorf("err %q", e.Err)
	}
	if k.ends != 1 {
		t.Errorf("ends %d, want 1", k.ends)
	}
}

func TestOpenRigAddresses(t *testing.T) {
	ctx := context.Background()
	for _, addr := range []string{"serial:", "serial:/dev/x?speed=9600", "serial:/dev/x?baud=0"} {
		if _, err := OpenRig(ctx, addr); err == nil {
			t.Errorf("%q accepted", addr)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	rig, err := OpenRig(ctx, "tcp://"+ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	rig.Close()
}

// A rig that stops answering fails the run, on a connection with
// deadlines and on one without.
func TestRigTimeout(t *testing.T) {
	for name, wrap := range map[string]func(net.Conn) io.ReadWriteCloser{
		"deadlines":    func(c net.Conn) io.ReadWriteCloser { return c },
		"no deadlines": func(c net.Conn) io.ReadWriteCloser { return struct{ io.ReadWriteCloser }{c} },
	} {
		t.Run(name, func(t *testing.T) {
			c, s := net.Pipe()
			defer s.Close()
			go io.Copy(io.Discard, s) // a rig that reads and never answers
			rig := NewRig(wrap(c))
			rig.Timeout = 50 * time.Millisecond
			start := time.Now()
			err := rig.Begin(context.Background(), "T", VM, nil)
			if err == nil || time.Since(start) > 2*time.Second {
				t.Fatalf("err %v after %v", err, time.Since(start))
			}
		})
	}
}
