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
		// Test case-insensitivity for a few examples
		{"var", VAR},   // lowercase
		{"vAr", VAR},   // mixed case
		{"TRUE", TRUE}, // uppercase
		{"true", TRUE}, // lowercase

		// Test all keywords from the map
		{"FN", FUNCTION},
		{"MACRO", MACRO},
		{"VAR", VAR},
		{"END_VAR", END_VAR},
		{"VAR_INPUT", VAR_INPUT},
		{"VAR_OUTPUT", VAR_OUTPUT},
		{"VAR_IN_OUT", VAR_IN_OUT},
		{"VAR_EXTERNAL", VAR_EXTERNAL},
		{"VAR_GLOBAL", VAR_GLOBAL},
		{"VAR_ACCESS", VAR_ACCESS},
		{"VAR_TEMP", VAR_TEMP},
		{"VAR_CONFIG", VAR_CONFIG},
		{"DIRECT_VAR", DIRECT_VAR},
		{"RETAIN", RETAIN},
		{"NON_RETAIN", NON_RETAIN},
		{"CONSTANT", CONSTANT},
		{"READ_ONLY", READ_ONLY},
		{"READ_WRITE", READ_WRITE},
		{"IF", IF},
		{"THEN", THEN},
		{"ELSE", ELSE},
		{"ELSIF", ELSIF},
		{"END_IF", END_IF},
		{"FOR", FOR},
		{"TO", TO},
		{"BY", BY},
		{"DO", DO},
		{"END_FOR", END_FOR},
		{"WHILE", WHILE},
		{"END_WHILE", END_WHILE},
		{"REPEAT", REPEAT},
		{"UNTIL", UNTIL},
		{"END_REPEAT", END_REPEAT},
		{"EXIT", EXIT},
		{"R_EDGE", R_EDGE},
		{"F_EDGE", F_EDGE},
		{"OF", OF},
		{"TYPE", TYPE},
		{"END_TYPE", END_TYPE},
		{"STRUCT", STRUCT},
		{"END_STRUCT", END_STRUCT},
		{"CASE", CASE},
		{"END_CASE", END_CASE},
		{"PROGRAM", PROGRAM},
		{"NOT", NOT},
		{"AND", AND},
		{"OR", OR},
		{"XOR", XOR},
		{"NAND", NAND},
		{"LD", LD},
		{"ST", ST},
		{"S", S},
		{"CAL", CAL},
		{"JMP", JMP},
		{"RET", RET},
		{"R", R},
		{"MOD", MOD},
		{"NOR", NOR},
		{"RETURN", RETURN},
		{"END_PROGRAM", END_PROGRAM},
		{"FUNCTION", FUNCTION},
		{"END_FUNCTION", END_FUNCTION},
		{"FUNCTION_BLOCK", FUNCTION_BLOCK},
		{"END_FUNCTION_BLOCK", END_FUNCTION_BLOCK},
		{"ACTION", ACTION},
		{"END_ACTION", END_ACTION},
		{"TRANSITION", TRANSITION},
		{"END_TRANSITION", END_TRANSITION},
		{"INITIAL_STEP", INITIAL_STEP},
		{"STEP", STEP},
		{"END_STEP", END_STEP},
		{"AT", AT},
		{"CONFIGURATION", CONFIGURATION},
		{"END_CONFIGURATION", END_CONFIGURATION},
		{"RESOURCE", RESOURCE},
		{"END_RESOURCE", END_RESOURCE},
		{"ON", ON},
		{"WITH", WITH},
		{"EXTENDS", EXTENDS},
		{"IMPLEMENTS", IMPLEMENTS},
		{"INTERFACE", INTERFACE},
		{"END_INTERFACE", END_INTERFACE},
		{"METHOD", METHOD},
		{"END_METHOD", END_METHOD},
		{"TASK", TASK},
		{"MOVE", MOVE},
		{"NIL", NIL},
		{"SINGLE", SINGLE},
		{"INTERVAL", INTERVAL},
		{"PRIORITY", PRIORITY},
		{"BOOL", BOOL},
		{"INT", INT},
		{"SINT", SINT},
		{"DINT", DINT},
		{"LINT", LINT},
		{"USINT", USINT},
		{"UINT", UINT},
		{"UDINT", UDINT},
		{"ULINT", ULINT},
		{"REAL", REAL},
		{"LREAL", LREAL},
		{"TIME", TIME},
		{"DATE", DATE},
		{"DATE_AND_TIME", DATE_AND_TIME},
		{"TIME_OF_DAY", TIME_OF_DAY},
		{"WSTRING", WSTRING},
		{"BYTE", BYTE},
		{"WORD", WORD},
		{"DWORD", DWORD},
		{"LWORD", LWORD},
		{"ARRAY", ARRAY},
		{"STRING", STRING},
		{"RANGE", RANGE},

		// Test aliases
		{"T", TIME},
		{"DT", DATE_AND_TIME},
		{"D", DATE},
		{"TOD", TIME_OF_DAY},

		// Test non-keywords
		{"my_variable", IDENT},
		{"variable1", IDENT},
		{"_internal", IDENT},
	}

	for _, tt := range tests {
		tok := LookupIdent(tt.input)
		if tok != tt.expected {
			t.Errorf("LookupIdent(%q) wrong. expected=%q, got=%q", tt.input, tt.expected, tok)
		}
	}
}
