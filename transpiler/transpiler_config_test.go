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
	"beedance/lexer"
	"beedance/parser"
	"bytes"
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
			name: "program without WITH is listed but not scheduled",
			config: `CONFIGURATION C RESOURCE Res ON CPU
				TASK Tk(INTERVAL := T#10ms, PRIORITY := 1);
				PROGRAM Free : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{`"Free": {`, `Programs: []string{},`},
		},
		{
			name: "task without PRIORITY or INTERVAL",
			config: `CONFIGURATION C RESOURCE Res ON CPU
				TASK Tk();
				PROGRAM P1 WITH Tk : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{"Priority: 0,", "Interval: 0,"},
			excludes: []string{"Interval: ,"},
		},
		{
			name: "SINGLE trigger is kept as a comment",
			config: `CONFIGURATION C VAR_GLOBAL z : BOOL; END_VAR RESOURCE Res ON CPU
				TASK Tk(SINGLE := z, PRIORITY := 1);
				PROGRAM P1 WITH Tk : Prog;
			END_RESOURCE END_CONFIGURATION`,
			contains: []string{"// SINGLE := z (event trigger is not supported by the generated configuration)"},
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
			contains: []string{`"COUNT": "5",`},
			excludes: []string{"Res.P1.COUNT", "%MW4"},
		},
		{
			name: "single-resource form",
			config: `CONFIGURATION C
				TASK Tk(INTERVAL := T#10ms, PRIORITY := 1);
				PROGRAM P1 WITH Tk : Prog;
				VAR_CONFIG P1.COUNT : INT := 3; END_VAR
			END_CONFIGURATION`,
			contains: []string{`Name: "C",`, `Programs: []string{"P1"},`, `"COUNT": "3",`},
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
