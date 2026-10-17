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
	"errors"
	"flag"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	oscat "github.com/apiarytech/beebread/basic"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/stdlib"
)

var cases = flag.Int("cases", 200, "the calls of each function")

// sources are the OSCAT sources of the ports, in the beebread module.
var sources = []string{"doc/beedance_basic.st", "doc/beedance_building.st", "doc/beedance_network.st"}

// library returns an environment with the OSCAT sources evaluated.
func library(t *testing.T) *object.Environment {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/apiarytech/beebread").Output()
	if err != nil {
		t.Fatalf("finding beebread: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	var src strings.Builder
	for _, s := range sources {
		data, err := os.ReadFile(filepath.Join(dir, s))
		if err != nil {
			t.Fatal(err)
		}
		// The ports hold OSCAT's ISO 8859-1 strings as UTF-8 (see beebread's
		// package basic), and so does the evaluator then. A source that is
		// UTF-8 already lost its other characters when it was cleaned.
		if !utf8.Valid(data) {
			data = []byte(latin1(string(data)))
		}
		src.Write(data)
		src.WriteString("\n")
	}
	p := parser.New(lexer.New(src.String()))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("the sources do not parse: %v", p.Errors()[0])
	}
	// OSCAT is written for CODESYS, which lets a POU assign its own inputs
	// (ACTUATOR_PUMP clears RST; IEC-69).
	evaluator.SetAllowInputWrites(true)
	t.Cleanup(func() { evaluator.SetAllowInputWrites(false) })
	env := object.NewEnvironment()
	if r := evaluator.Eval(program, env); r != nil && r.Type() == object.ERROR_OBJ {
		t.Fatalf("the sources do not evaluate: %s", r.Inspect())
	}
	return env
}

// simulate runs both sides on the same simulated clock, which starts at
// start: the evaluator's TIME() and standard timers, and the ports' T_PLC_MS
// and T_PLC_US. It runs the evaluator in a session, so that eval can bound
// what it executes.
func simulate(t *testing.T, start time.Time, now *time.Time) {
	*now = start
	end := evaluator.Session(func() time.Time { return *now })
	oldClock := stdlib.Clock
	stdlib.Clock = func() time.Duration { return now.Sub(start) }
	oscat.SetEpoch(start)
	oscat.SetClock(func() time.Time { return *now })
	t.Cleanup(func() {
		end()
		stdlib.Clock = oldClock
		oscat.SetEpoch(time.Now())
		oscat.SetClock(time.Now)
	})
}

// budget bounds the statements and loop iterations of one evaluation, so
// that a call that loops for ever fails instead of hanging the test.
const budget = 1_000_000

// eval evaluates ST in env, in a simulate session.
func eval(env *object.Environment, src string) object.Object {
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return &object.Error{Message: "parse: " + p.Errors()[0]}
	}
	evaluator.SetBudget(budget)
	return evaluator.Eval(program, env)
}

// portTimeout is how long a port may take before call gives up on it.
const portTimeout = 2 * time.Second

// errHang is the error of a port that does not return.
var errHang = errors.New("does not return")

// call calls a port, turning a panic into an error. A port that does not
// return within portTimeout is left running, as Go cannot stop it.
func call(fn reflect.Value, args []reflect.Value) (reflect.Value, error) {
	type result struct {
		v   reflect.Value
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: fmt.Errorf("panic: %v", r)}
			}
		}()
		done <- result{v: fn.Call(args)[0]}
	}()
	select {
	case r := <-done:
		return r.v, r.err
	case <-time.After(portTimeout):
		return reflect.Value{}, errHang
	}
}

// signature returns the IEC types of a function's inputs and result, or
// why the harness cannot call it.
func signature(f *object.Function, port reflect.Type) (params []string, result string, why string) {
	if len(f.VarInOuts) > 0 || len(f.VarOutputs) > 0 {
		return nil, "", "VAR_IN_OUT or VAR_OUTPUT"
	}
	if port.NumIn() != len(f.VarInputs) {
		return nil, "", fmt.Sprintf("%d inputs, the port %d", len(f.VarInputs), port.NumIn())
	}
	for i, in := range f.VarInputs {
		st, ok := iecType(in.DataType.String())
		if !ok {
			return nil, "", "input type " + in.DataType.String()
		}
		if !matches(st, port.In(i)) {
			return nil, "", fmt.Sprintf("input %s is %s, the port's %s", in.Name.Value, st, port.In(i))
		}
		params = append(params, st)
	}
	if f.ReturnType == nil {
		return nil, "", "no result"
	}
	result, ok := iecType(f.ReturnType.String())
	if !ok {
		return nil, "", "result type " + f.ReturnType.String()
	}
	if !matches(result, port.Out(0)) {
		return nil, "", fmt.Sprintf("result is %s, the port's %s", result, port.Out(0))
	}
	return params, result, ""
}

// TestFunctions calls each OSCAT function the evaluator can run, with
// arguments of its input types, in the evaluator and as its port, and
// compares the results.
func TestFunctions(t *testing.T) {
	env := library(t)
	var now time.Time
	simulate(t, time.Date(2026, 10, 13, 8, 30, 0, 0, time.UTC), &now)

	skipped := map[string][]string{}
	tested := 0
	rep := newReport()
	for _, name := range slices.Sorted(maps.Keys(Functions)) {
		obj, ok := env.Get("_function_" + name) // the evaluator keeps functions apart from variables
		f, isFn := obj.(*object.Function)
		if !ok || !isFn {
			skipped["commented out"] = append(skipped["commented out"], name)
			continue
		}
		port := reflect.ValueOf(Functions[name])
		params, result, why := signature(f, port.Type())
		if why != "" {
			skipped[why] = append(skipped[why], name)
			continue
		}
		tested++
		rep.tested[name] = true
		t.Run(name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed(name), 0))
			for i := range *cases {
				args := make([]reflect.Value, len(params))
				lits := make([]string, len(params))
				for k, st := range params {
					v := genValue(r, st, port.Type().In(k), i)
					args[k], lits[k] = v.goV, v.lit
				}
				if in, ok := domain[name]; ok && !in(args) {
					continue
				}
				src := name + "(" + strings.Join(lits, ", ") + ")"
				got, err := call(port, args)
				ev := eval(env, src+";")
				switch {
				case err != nil:
					v := mismatch
					if _, failed := ev.(*object.Error); failed && err != errHang {
						v = bothFail
					}
					if !rep.add(t, name, v, fmt.Sprintf("%s: port %v; evaluator %s", src, err, ev.Inspect())) || err == errHang {
						return
					}
				default:
					v, text := compare(result, got, ev)
					if v == evalError {
						text = src + ": " + text
					} else {
						text = fmt.Sprintf("%s = %s, evaluator %s", src, goString(got), text)
					}
					if !rep.add(t, name, v, text) {
						return
					}
				}
			}
		})
	}
	t.Logf("%d functions compared", tested)
	if tested == 0 {
		t.Error("no function compared")
	}
	for _, why := range slices.Sorted(maps.Keys(skipped)) {
		t.Logf("not compared, %s: %d %v", why, len(skipped[why]), skipped[why])
	}
	rep.log(t)
}
