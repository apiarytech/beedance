/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"testing"
)

// The compiler turns an SFC program into a description of the chart: its
// steps, actions and transitions, with transition conditions and actions
// compiled as functions. The VM builds that description; it does not step
// through the chart. These tests check the description and run the compiled
// conditions and actions against the program's variables.

const trafficLightSFC = `
	PROGRAM Lights
		VAR go : BOOL := TRUE; count : INT := 0; END_VAR
		INITIAL_STEP Red:
			Stop(N);
		END_STEP
		TRANSITION FROM Red TO Green := go;
		END_TRANSITION
		STEP Green:
			Go(S);
		END_STEP
		TRANSITION FROM Green TO Red := NOT go;
		END_TRANSITION
		ACTION Stop:
			count := count + 1;
		END_ACTION
		ACTION Go:
			count := count + 10;
		END_ACTION
	END_PROGRAM`

// runSFC runs the program and returns the VM and the chart description.
func runSFC(t *testing.T) (*VM, *compiler.Bytecode, *object.Hash) {
	t.Helper()
	bytecode := compileForTest(t, trafficLightSFC)
	machine := New(bytecode)
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	chart, ok := machine.LastPoppedStackElem().(*object.Hash)
	if !ok {
		t.Fatalf("expected the chart description hash, got %T", machine.LastPoppedStackElem())
	}
	return machine, bytecode, chart
}

func TestSFCChartDescription(t *testing.T) {
	_, _, chart := runSFC(t)

	if initial := field(t, chart, "initial_step").(*object.String).Value; initial != "Red" {
		t.Fatalf("expected initial step Red, got %s", initial)
	}

	steps := field(t, chart, "steps").(*object.Hash)
	for _, name := range []string{"Red", "Green"} {
		field(t, steps, name)
	}

	actions := field(t, chart, "actions").(*object.Hash)
	for _, name := range []string{"Stop", "Go"} {
		if _, ok := field(t, actions, name).(*object.Closure); !ok {
			t.Fatalf("action %s should be compiled as a function", name)
		}
	}

	transitions := field(t, chart, "transitions").(*object.Array).Elements
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	first := transitions[0].(*object.Hash)
	from := field(t, first, "from").(*object.Array).Elements
	to := field(t, first, "to").(*object.Array).Elements
	if len(from) != 1 || from[0].Inspect() != "Red" || len(to) != 1 || to[0].Inspect() != "Green" {
		t.Fatalf("expected the first transition Red -> Green, got %s", first.Inspect())
	}
}

// callWithGlobals runs a compiled chart function against the program's globals.
// The function refers to the program's constant pool, so the call uses that
// pool with the function appended.
func callWithGlobals(t *testing.T, program *compiler.Bytecode, globals []object.Object, fn object.Object) object.Object {
	t.Helper()
	constants := append(append([]object.Object{}, program.Constants...), fn)
	machine := NewWithGlobalsStore(&compiler.Bytecode{
		Instructions: concat(code.Make(code.OpConstant, len(constants)-1), code.Make(code.OpCall, 0), code.Make(code.OpPop)),
		Constants:    constants,
	}, globals)
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	return machine.LastPoppedStackElem()
}

func TestSFCTransitionConditions(t *testing.T) {
	machine, program, chart := runSFC(t)
	transitions := field(t, chart, "transitions").(*object.Array).Elements
	toGreen := field(t, transitions[0].(*object.Hash), "condition")
	toRed := field(t, transitions[1].(*object.Hash), "condition")

	// go is TRUE: Red -> Green can fire, Green -> Red cannot.
	if got := callWithGlobals(t, program, machine.Globals(), toGreen); got != True {
		t.Fatalf("expected Red -> Green to be enabled, got %v", got.Inspect())
	}
	if got := callWithGlobals(t, program, machine.Globals(), toRed); got != False {
		t.Fatalf("expected Green -> Red to be disabled, got %v", got.Inspect())
	}
}

func TestSFCActionsUpdateProgramVariables(t *testing.T) {
	machine, program, chart := runSFC(t)
	actions := field(t, chart, "actions").(*object.Hash)

	callWithGlobals(t, program, machine.Globals(), field(t, actions, "Stop"))
	callWithGlobals(t, program, machine.Globals(), field(t, actions, "Go"))
	callWithGlobals(t, program, machine.Globals(), field(t, actions, "Go"))

	// count is the program's second variable.
	if err := testIntegerObject(21, machine.Globals()[1]); err != nil {
		t.Fatalf("count after Stop, Go, Go: %s", err)
	}
}
