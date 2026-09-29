/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package parser

import (
	"beedance/ast"
	"beedance/lexer"
	"strings"
	"testing"
)

func TestSingleResourceConfiguration(t *testing.T) {
	input := `
	CONFIGURATION Cell
		VAR_GLOBAL w : UINT; END_VAR
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		TASK Slow(INTERVAL := T#50ms, PRIORITY := 2);
		PROGRAM P1 WITH Fast : Prog;
		PROGRAM P2 WITH Slow : Prog;
	END_CONFIGURATION`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestSingleResourceConfiguration", input)

	cfg, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("expected *ast.ConfigurationDeclaration, got %T", program.Statements[0])
	}
	if len(cfg.Resources) != 1 {
		t.Fatalf("expected 1 implicit resource, got %d", len(cfg.Resources))
	}
	res := cfg.Resources[0]
	if !res.IsImplicit {
		t.Fatalf("expected the resource to be implicit")
	}
	if res.Name.Value != "Cell" || res.ResourceType != nil {
		t.Fatalf("implicit resource should be named after the configuration and have no type, got %q / %v", res.Name.Value, res.ResourceType)
	}
	if len(res.Tasks) != 2 || len(res.Programs) != 2 {
		t.Fatalf("expected 2 tasks and 2 programs, got %d and %d", len(res.Tasks), len(res.Programs))
	}
	if res.Programs[1].TaskName.Value != "Slow" {
		t.Fatalf("expected P2 to run WITH Slow, got %s", res.Programs[1].TaskName.Value)
	}
	if strings.Contains(cfg.String(), "RESOURCE") {
		t.Fatalf("single-resource form should print without a RESOURCE block:\n%s", cfg.String())
	}
}

func TestConfigurationFormErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name: "mixing RESOURCE blocks with direct TASK/PROGRAM",
			input: `CONFIGURATION C
				RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
				PROGRAM P2 : Prog;
			END_CONFIGURATION`,
			expectedError: "a CONFIGURATION must use either RESOURCE blocks or TASK/PROGRAM declarations directly, not both",
		},
		{
			name: "unexpected token in CONFIGURATION",
			input: `CONFIGURATION C
				FOO;
				RESOURCE Res ON CPU PROGRAM P1 : Prog; END_RESOURCE
			END_CONFIGURATION`,
			expectedError: "unexpected token 'FOO' in CONFIGURATION block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(lexer.New(tt.input))
			p.ParseProgram()
			for _, err := range p.Errors() {
				if strings.HasPrefix(err, tt.expectedError) {
					return
				}
			}
			t.Fatalf("expected an error starting with %q, got %v", tt.expectedError, p.Errors())
		})
	}
}
