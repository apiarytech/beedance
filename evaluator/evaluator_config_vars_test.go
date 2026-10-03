/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

import (
	"github.com/apiarytech/beedance/object"
	"strings"
	"testing"
	"time"
)

const configVarsPrograms = `
	PROGRAM Prog
		VAR
			COUNT : INT;
			TIME1 : TON;
			Sensor : BOOL;
		END_VAR
	END_PROGRAM
`

// programInstanceEnv returns the environment of a program instance, looking in
// the named resource, or directly in env for the single-resource form.
func programInstanceEnv(t *testing.T, env *object.Environment, resource, instance string) *object.Environment {
	t.Helper()
	scope := env
	if resource != "" {
		resObj, ok := env.Get(resource)
		if !ok {
			t.Fatalf("resource %q not found", resource)
		}
		scope = resObj.(*object.FunctionBlockInstance).Env
	}
	progObj, ok := scope.Get(instance)
	if !ok {
		t.Fatalf("program instance %q not found", instance)
	}
	return progObj.(*object.ProgramInstance).Env
}

func TestVarConfigInitialValues(t *testing.T) {
	input := configVarsPrograms + `
	CONFIGURATION Cell
		RESOURCE Station_1 ON CPU PROGRAM P1 : Prog; END_RESOURCE
		RESOURCE Station_2 ON CPU PROGRAM P1 : Prog; END_RESOURCE
		VAR_CONFIG
			STATION_1.P1.COUNT : INT := 1;
			Station_2.P1.COUNT : INT := 100;
			Station_1.P1.TIME1 : TON := (PT := T#2.5s);
			Station_2.P1.Sensor AT %IX0.5 : BOOL;
		END_VAR
	END_CONFIGURATION`

	ioMap = make(map[string]object.Object) // Reset I/O map
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, input, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}

	// Each instance gets its own value, converted to the declared type.
	for resource, want := range map[string]int64{"Station_1": 1, "Station_2": 100} {
		count, _ := programInstanceEnv(t, env, resource, "P1").Get("COUNT")
		got, ok := count.(*object.Int)
		if !ok || int64(got.Value) != want {
			t.Fatalf("%s.P1.COUNT: expected INT %d, got %T %v", resource, want, count, count)
		}
	}

	// The structure initialization sets PT on the TON instance.
	timerObj, _ := programInstanceEnv(t, env, "Station_1", "P1").Get("TIME1")
	timer, ok := timerObj.(*object.FunctionBlockInstance)
	if !ok {
		t.Fatalf("TIME1: expected a function block instance, got %T", timerObj)
	}
	pt, _ := timer.Env.Get("PT")
	if ptTime, ok := pt.(*object.Time); !ok || ptTime.Value != 2500*time.Millisecond {
		t.Fatalf("TIME1.PT: expected T#2.5s, got %v", pt)
	}

	// The located entry maps Sensor to the I/O address.
	sensor, _ := programInstanceEnv(t, env, "Station_2", "P1").Get("Sensor")
	if ptr, ok := sensor.(*object.Pointer); !ok || ptr.Name != "%IX0.5" {
		t.Fatalf("Sensor: expected a pointer to %%IX0.5, got %v", sensor)
	}
	if _, ok := ioMap["%IX0.5"]; !ok {
		t.Fatalf("expected %%IX0.5 to be registered in the I/O map")
	}
}

func TestSingleResourceConfigurationEvaluation(t *testing.T) {
	input := configVarsPrograms + `
	CONFIGURATION Cell
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		PROGRAM P1 WITH Fast : Prog;
		VAR_CONFIG P1.COUNT : INT := 7; END_VAR
	END_CONFIGURATION`

	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, input, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}

	count, _ := programInstanceEnv(t, env, "", "P1").Get("COUNT")
	if got, ok := count.(*object.Int); !ok || got.Value != 7 {
		t.Fatalf("P1.COUNT: expected INT 7, got %T %v", count, count)
	}

	scheduler, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	if len(scheduler.Tasks) != 1 || scheduler.Tasks[0].Name != "Fast" || len(scheduler.Tasks[0].Programs) != 1 {
		t.Fatalf("expected task Fast running P1, got %+v", scheduler.Tasks)
	}
}

func TestVarConfigErrors(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectedError string
	}{
		{
			name:          "path names no program instance",
			config:        `VAR_CONFIG Res.Nope.COUNT : INT := 1; END_VAR`,
			expectedError: "VAR_CONFIG path 'Res.Nope.COUNT' does not name a variable of a program instance",
		},
		{
			name:          "variable does not exist",
			config:        `VAR_CONFIG Res.P1.Missing : INT := 1; END_VAR`,
			expectedError: "variable 'Missing' in VAR_CONFIG path 'Res.P1.Missing' not found in instance",
		},
		{
			name:          "structure initialization on a non-FB variable",
			config:        `VAR_CONFIG Res.P1.COUNT : INT := (PT := T#1s); END_VAR`,
			expectedError: "VAR_CONFIG structure initialization requires a function block instance or structure, but 'COUNT' is ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := configVarsPrograms + `
			CONFIGURATION Cell
				RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
				` + tt.config + `
			END_CONFIGURATION`
			result := testEvalWithEnv(t, input, object.NewEnvironment())
			errObj, ok := result.(*object.Error)
			if !ok {
				t.Fatalf("expected an error, got %T %v", result, result)
			}
			// Messages start with a source position, e.g. "ERROR (10:4): ".
			if !strings.Contains(errObj.Message, "): "+tt.expectedError) || strings.Contains(errObj.Message, "(0:0)") {
				t.Fatalf("wrong error.\nwant: %s\ngot:  %s", tt.expectedError, errObj.Message)
			}
		})
	}
}
