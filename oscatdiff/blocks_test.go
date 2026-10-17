/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package oscatdiff

import (
	"flag"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

var scans = flag.Int("scans", 300, "the scans of each function block")

var follow = flag.String("follow", "", "log the state of both sides of this function block after each scan")

// scanSteps are the times between two scans: a block sees fast scans, slow
// ones and pauses long enough for its timers to run out; OSCAT expects
// cycles of milliseconds, and a longer pause overflows its arithmetic.
var scanSteps = []time.Duration{time.Millisecond, 10 * time.Millisecond, 100 * time.Millisecond, time.Second, 7 * time.Second}

// timeInputs are the values of a TIME input of a block: short enough for
// the scans to reach them.
var timeInputs = []time.Duration{0, 50 * time.Millisecond, time.Second, 5 * time.Second, time.Minute}

// port is an input or output of a block: its name, its IEC type and the
// field of the port.
type pin struct {
	name, st string
	field    int
}

// pins returns the inputs or outputs of a block the harness drives or
// compares, and those it leaves alone.
func pins(decls []*ast.VarDeclStatement, port reflect.Type) (ok []pin, left []string) {
	for _, d := range decls {
		name := strings.ToUpper(d.Name.Value)
		st, supported := iecType(d.DataType.String())
		f, found := port.FieldByName(name)
		if !supported || !found || len(f.Index) != 1 || !matches(st, f.Type) {
			left = append(left, name)
			continue
		}
		ok = append(ok, pin{name, st, f.Index[0]})
	}
	return ok, left
}

// run runs f, turning a panic into an error, and gives up on it after
// portTimeout.
func run(f func()) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		f()
		done <- nil
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(portTimeout):
		return errHang
	}
}

// TestFunctionBlocks runs each OSCAT function block the evaluator can run
// through the same scans as its port, with inputs that change from scan to
// scan on a simulated clock, and compares the outputs after each scan.
func TestFunctionBlocks(t *testing.T) {
	env := library(t)
	var now time.Time
	start := time.Date(2026, 10, 13, 8, 30, 0, 0, time.UTC)
	simulate(t, start, &now)

	skipped := map[string][]string{}
	partly := map[string][]string{}
	tested := 0
	rep := newReport()
	for _, name := range slices.Sorted(maps.Keys(Blocks)) {
		obj, ok := env.Get(name)
		def, isFB := obj.(*object.FunctionBlock)
		if !ok || !isFB {
			skipped["commented out"] = append(skipped["commented out"], name)
			continue
		}
		if len(def.VarInOuts) > 0 {
			skipped["VAR_IN_OUT"] = append(skipped["VAR_IN_OUT"], name)
			continue
		}
		portType := reflect.TypeOf(Blocks[name]()).Elem()
		ins, leftIn := pins(def.VarInputs, portType)
		outs, leftOut := pins(def.VarOutputs, portType)
		if len(outs) == 0 {
			skipped["no output compared"] = append(skipped["no output compared"], name)
			continue
		}
		if len(leftIn)+len(leftOut) > 0 {
			partly[name] = append(leftIn, leftOut...)
		}
		tested++
		rep.tested[name] = true
		t.Run(name, func(t *testing.T) {
			now = start
			inst := "dut_" + name
			if r := eval(env, "VAR "+inst+" : "+name+"; END_VAR"); r != nil && r.Type() == object.ERROR_OBJ {
				rep.add(t, name, evalError, "declaring "+name+": "+r.Inspect())
				return
			}
			o, _ := env.Get(inst)
			evInst, ok := o.(*object.FunctionBlockInstance)
			if !ok {
				t.Fatalf("%s is a %T", inst, o)
			}
			port := Blocks[name]()
			pv := reflect.ValueOf(port).Elem()
			if err := run(port.INIT); err != nil {
				rep.add(t, name, mismatch, "INIT: "+err.Error())
				return
			}
			// The outputs before the first scan.
			if !compareOutputs(t, rep, name, "before the first scan", outs, pv, evInst) {
				return
			}
			r := rand.New(rand.NewPCG(seed(name), 1))
			var history []string
			for i := range *scans {
				now = now.Add(scanSteps[r.IntN(len(scanSteps))])
				var assigns []string
				for _, in := range ins {
					// Each input changes now and then, a BOOL more often.
					if i > 0 && r.IntN(4) != 0 && !(in.st == "BOOL" && r.IntN(2) == 0) {
						continue
					}
					v := genValue(r, in.st, pv.Field(in.field).Type(), i)
					if in.st == "TIME" {
						d := timeInputs[r.IntN(len(timeInputs))]
						v.goV.SetInt(int64(d))
						v.lit = fmt.Sprintf("T#%dms", d.Milliseconds())
					}
					pv.Field(in.field).Set(v.goV)
					assigns = append(assigns, in.name+" := "+v.lit)
				}
				call := fmt.Sprintf("%s(%s);", inst, strings.Join(assigns, ", "))
				history = append(history, fmt.Sprintf("%s %s", now.Sub(start), call))
				if len(history) > 5 {
					history = history[1:]
				}
				err := run(func() { port.Execute(now) })
				ev := eval(env, call)
				failed := ev != nil && ev.Type() == object.ERROR_OBJ
				switch {
				case err != nil && failed:
					rep.add(t, name, bothFail, fmt.Sprintf("scan %d: port %v; evaluator %s", i, err, ev.Inspect()))
					return
				case err != nil:
					rep.add(t, name, mismatch, fmt.Sprintf("scan %d: port %v; scans:\n\t%s", i, err, strings.Join(history, "\n\t")))
					return
				case failed:
					rep.add(t, name, evalError, fmt.Sprintf("%s: %s", call, ev.Inspect()))
					return
				}
				if *follow == name {
					t.Logf("%s\n\tport: %+v\n\tevaluator: %s", history[len(history)-1], port, state(def, evInst))
				}
				if !compareOutputs(t, rep, name, fmt.Sprintf("after scan %d of\n\t%s\n", i, strings.Join(history, "\n\t")), outs, pv, evInst) {
					return
				}
			}
		})
	}
	t.Logf("%d function blocks compared", tested)
	if tested == 0 {
		t.Error("no function block compared")
	}
	for _, why := range slices.Sorted(maps.Keys(skipped)) {
		t.Logf("not compared, %s: %d %v", why, len(skipped[why]), skipped[why])
	}
	for _, name := range slices.Sorted(maps.Keys(partly)) {
		t.Logf("partly compared, %s: not %v", name, partly[name])
	}
	rep.log(t)
}

// compareOutputs compares the outputs of a block's port and evaluator
// instance, and reports whether its comparison goes on: a block's state
// differs after its first mismatch, so the rest would say little.
func compareOutputs(t *testing.T, rep *report, name, when string, outs []pin, pv reflect.Value, inst *object.FunctionBlockInstance) bool {
	t.Helper()
	for _, out := range outs {
		ev, ok := inst.Env.Get(out.name)
		if !ok {
			rep.add(t, name, evalError, fmt.Sprintf("%s: no output %s", name, out.name))
			return false
		}
		got := pv.Field(out.field)
		v, text := compare(out.st, got, ev)
		switch v {
		case agree, wide:
			continue
		case evalError:
			text = name + "." + out.name + ": " + text
		default:
			text = fmt.Sprintf("%s.%s = %s, evaluator %s, %s", name, out.name, goString(got), text, when)
		}
		rep.add(t, name, v, text)
		if v == mismatch {
			return false
		}
	}
	return true
}

// state writes the variables of an evaluator instance of a block, for
// -follow.
func state(def *object.FunctionBlock, inst *object.FunctionBlockInstance) string {
	var s []string
	for _, decls := range [][]*ast.VarDeclStatement{def.VarInputs, def.VarOutputs, def.Vars} {
		for _, d := range decls {
			if v, ok := inst.Env.Get(d.Name.Value); ok && v != nil {
				if fb, ok := v.(*object.FunctionBlockInstance); ok && fb.Definition != nil {
					s = append(s, d.Name.Value+":{"+state(fb.Definition, fb)+"}")
					continue
				}
				s = append(s, d.Name.Value+":"+v.Inspect())
			}
		}
	}
	return strings.Join(s, " ")
}
