/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package token

import "testing"

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"VAR", VAR},
		{"var", VAR},
		{"vAr", VAR},
		{"FUNCTION_BLOCK", FUNCTION_BLOCK},
		{"REAL", REAL},
		{"IF", IF},
		{"ELSE", ELSE},
		{"END_IF", END_IF},
		{"my_variable", IDENT},
		{"variable1", IDENT},
		{"_internal", IDENT},
		{"NOT", NOT},
		{"AND", AND},
		{"OR", OR},
		{"XOR", XOR},
		{"MOD", MOD},
		{"RETURN", RETURN},
		{"PROGRAM", PROGRAM},
		{"TRUE", TRUE},
		{"FALSE", FALSE},
		{"ARRAY", ARRAY},
		{"STRUCT", STRUCT},
		{"DT", DATE_AND_TIME},
		{"TOD", TIME_OF_DAY},
	}

	for _, tt := range tests {
		tok := LookupIdent(tt.input)
		if tok != tt.expected {
			t.Errorf("LookupIdent(%q) wrong. expected=%q, got=%q", tt.input, tt.expected, tok)
		}
	}
}
