/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	"bytes"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
	"strings"
	"testing"
)

const configTestProgram = `
PROGRAM Prog
	VAR COUNT : INT; END_VAR
	COUNT := COUNT + 1;
END_PROGRAM
`

// transpileConfig transpiles input and returns the generated main function.
func transpileConfig(t *testing.T, input string) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	var buf bytes.Buffer
	if err := New(&buf).Transpile(program); err != nil {
		t.Fatalf("transpile error: %s", err)
	}
	out := buf.String()
	start := strings.Index(out, "func main()")
	if start < 0 {
		t.Fatalf("no main function generated:\n%s", out)
	}
	return out[start:]
}

func TestConfigurationOptionalFieldsTranspilation(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		contains []string
		excludes []string
	}{
		{
			name: "program without WITH runs in a background task",
			config: `CONFIGURATION C RESOURCE Res ON CPU
				TASK Tk(INTERVAL := T#10ms, PRIORITY := 1);
				PROGRAM Free : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{
				`res1_task1 := core.NewTask("Tk", core.CyclicTask, 0, time.Duration(iec.TIME(10000000)))`,
				`res1_background := core.NewTask("BACKGROUND", core.CyclicTask, 1, 2*core.DefaultCycle)`,
				`res1_background.AddProgram(&core.Program{Name: "Free", Logic: res1_prog1})`,
			},
		},
		{
			name: "task without PRIORITY or INTERVAL runs every other scan",
			config: `CONFIGURATION C RESOURCE Res ON CPU
				TASK Tk();
				PROGRAM P1 WITH Tk : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{`core.NewTask("Tk", core.CyclicTask, 0, 2*core.DefaultCycle)`, `res1_task1.AddProgram(&core.Program{Name: "P1", Logic: res1_prog1})`},
		},
		{
			name: "SINGLE triggers the task on a rising edge",
			config: `CONFIGURATION C VAR_GLOBAL z : BOOL; END_VAR RESOURCE Res ON CPU
				TASK Tk(SINGLE := z, PRIORITY := 1);
				PROGRAM P1 WITH Tk : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{
				`res1_task1 := core.NewTask("Tk", core.EventDrivenTask, 0, 0)`,
				`if z && !res1_task1_single {`,
				`res1_task1.Trigger()`,
				`res1_task1_single = z`,
			},
		},
		{
			name: "standard VAR_CONFIG paths become instance parameters",
			config: `CONFIGURATION C
				RESOURCE Res ON CPU PROGRAM P1 : Prog; PROGRAM P2 : Prog; END_RESOURCE
				VAR_CONFIG
					Res.P1.COUNT : INT := 5;
					Res.P2.COUNT AT %MW4 : INT;
				END_VAR
			END_CONFIGURATION`,
			contains: []string{`NewProgFactory(map[string]string{"COUNT": "5"})`, `NewProgFactory(map[string]string{})`},
			excludes: []string{"Res.P1.COUNT", "%MW4"},
		},
		{
			name: "single-resource form",
			config: `CONFIGURATION C
				TASK Tk(INTERVAL := T#10ms, PRIORITY := 1);
				PROGRAM P1 WITH Tk : Prog;
				VAR_CONFIG P1.COUNT : INT := 3; END_VAR
			END_CONFIGURATION`,
			contains: []string{`cfg := &core.Configuration{Name: "C"}`, `res1_task1.AddProgram(&core.Program{Name: "P1", Logic: res1_prog1})`, `{"COUNT": "3"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := transpileConfig(t, configTestProgram+tt.config)
			for _, s := range tt.contains {
				if !strings.Contains(out, s) {
					t.Fatalf("expected output to contain %q:\n%s", s, out)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(out, s) {
					t.Fatalf("expected output not to contain %q:\n%s", s, out)
				}
			}
		})
	}
}

func TestConfigurationUnmatchedVarConfigTranspilation(t *testing.T) {
	p := parser.New(lexer.New(configTestProgram + `CONFIGURATION C
		RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
		VAR_CONFIG Res.Nope.COUNT : INT := 1; END_VAR
	END_CONFIGURATION`))
	program := p.ParseProgram()
	var buf bytes.Buffer
	err := New(&buf).Transpile(program)
	want := "VAR_CONFIG path 'Res.Nope.COUNT' does not name a variable of a program instance in configuration 'C'"
	if err == nil || err.Error() != want {
		t.Fatalf("expected error %q, got %v", want, err)
	}
}

// royaljelly needs a different priority for each task of a resource. Tasks
// are ranked by IEC priority, then declaration, so they keep their order.
func TestConfigurationTaskPriorities(t *testing.T) {
	out := transpileConfig(t, configTestProgram+`CONFIGURATION C RESOURCE Res ON CPU
		TASK Slow(INTERVAL := T#100ms, PRIORITY := 5);
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		TASK Also(INTERVAL := T#20ms, PRIORITY := 1);
		PROGRAM P1 WITH Slow : Prog;
	END_RESOURCE END_CONFIGURATION`)
	for _, want := range []string{
		`core.NewTask("Slow", core.CyclicTask, 2,`,
		`core.NewTask("Fast", core.CyclicTask, 0,`,
		`core.NewTask("Also", core.CyclicTask, 1,`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
	// Errors in a configuration are reported.
	for _, tt := range []struct{ config, want string }{
		{`CONFIGURATION C RESOURCE Res ON CPU TASK Tk(INTERVAL := T#10ms, PRIORITY := -1); END_RESOURCE END_CONFIGURATION`, "PRIORITY must be a non-negative constant"},
		{`CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 WITH Nope : Prog; END_RESOURCE END_CONFIGURATION`, "names task 'Nope'"},
		{`CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE VAR_CONFIG Res.P1.COUNT : INT := 1 + 2; END_VAR END_CONFIGURATION`, "must be a literal number, BOOL or string"},
		{`CONFIGURATION C RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE END_CONFIGURATION CONFIGURATION C2 RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE END_CONFIGURATION`, "has only one CONFIGURATION"},
	} {
		p := parser.New(lexer.New(configTestProgram + tt.config))
		program := p.ParseProgram()
		var buf bytes.Buffer
		if err := New(&buf).Transpile(program); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s:\n  expected an error containing %q, got %v", tt.config, tt.want, err)
		}
	}
}
