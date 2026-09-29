/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast_test

import (
	"beedance/ast"
	"beedance/lexer"
	"beedance/parser"
	"testing"
)

func parseConfiguration(t *testing.T, input string) *ast.ConfigurationDeclaration {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	for _, s := range program.Statements {
		if cfg, ok := s.(*ast.ConfigurationDeclaration); ok {
			return cfg
		}
	}
	t.Fatalf("no CONFIGURATION found")
	return nil
}

func TestResolveConfigVars(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  map[string]string // "instance: relative path" -> value
		unmatched int
	}{
		{
			name: "standard form with resources, including an FB path",
			input: `CONFIGURATION C
				RESOURCE Station_1 ON CPU PROGRAM P1 : Prog; END_RESOURCE
				RESOURCE Station_2 ON CPU PROGRAM P1 : Prog; PROGRAM P4 : Prog; END_RESOURCE
				VAR_CONFIG
					STATION_1.P1.COUNT : INT := 1;
					Station_2.P1.COUNT : INT := 100;
					Station_2.P4.FB1.C2 AT %QB25 : BYTE;
				END_VAR
			END_CONFIGURATION`,
			expected: map[string]string{
				"Station_1.P1: COUNT":  "1",
				"Station_2.P1: COUNT":  "100",
				"Station_2.P4: FB1.C2": "",
			},
		},
		{
			name: "single-resource form",
			input: `CONFIGURATION C
				TASK Tk(INTERVAL := T#10ms, PRIORITY := 1);
				PROGRAM P1 WITH Tk : Prog;
				VAR_CONFIG P1.COUNT : INT := 7; END_VAR
			END_CONFIGURATION`,
			expected: map[string]string{"C.P1: COUNT": "7"},
		},
		{
			name: "program-scoped form",
			input: `CONFIGURATION C
				RESOURCE Res ON CPU PROGRAM Motor1 : Prog; PROGRAM Motor2 : Prog; END_RESOURCE
				VAR_CONFIG Motor1
					Speed : INT := 500;
				END_VAR
			END_CONFIGURATION`,
			expected: map[string]string{"Res.Motor1: Speed": "500"},
		},
		{
			name: "unmatched paths are reported",
			input: `CONFIGURATION C
				RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
				VAR_CONFIG
					Res.Nope.COUNT : INT := 1;
					Res.P1 : INT := 2;
				END_VAR
			END_CONFIGURATION`,
			expected:  map[string]string{},
			unmatched: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseConfiguration(t, tt.input)
			entries, unmatched := cfg.ResolveConfigVars()
			if len(unmatched) != tt.unmatched {
				t.Fatalf("expected %d unmatched entries, got %d", tt.unmatched, len(unmatched))
			}
			got := map[string]string{}
			for _, e := range entries {
				value := ""
				if e.Decl.Value != nil {
					value = e.Decl.Value.String()
				}
				got[e.Resource.Name.Value+"."+e.Program.InstanceName.Value+": "+e.RelativePath()] = value
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("expected entries %v, got %v", tt.expected, got)
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Fatalf("entry %q: expected value %q, got %q (all: %v)", k, v, got[k], got)
				}
			}
		})
	}
}
