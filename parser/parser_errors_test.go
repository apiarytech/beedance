package parser

import (
	"beedance/lexer"
	"strings"
	"testing"
)

func TestParserErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Missing semicolon",
			input:         `PROGRAM Test; VAR x: INT END_VAR x := 5; END_PROGRAM`,
			expectedError: "expected next token to be ;, got END_VAR",
		},
		{
			name:          "Invalid assignment operator",
			input:         `PROGRAM Test; VAR x: INT; END_VAR x = 5; END_PROGRAM`,
			expectedError: "expected :=, got =",
		},
		{
			name:          "Missing END_IF",
			input:         `PROGRAM Test; VAR x: BOOL; END_VAR IF x THEN x := FALSE;`,
			expectedError: "unexpected end of file, expected END_IF",
		},
		{
			name:          "Missing END_FOR",
			input:         `PROGRAM Test; VAR i: INT; END_VAR FOR i := 1 TO 10 DO i := i + 1;`,
			expectedError: "unexpected end of file, expected END_FOR",
		},
		{
			name:          "Missing UNTIL in REPEAT",
			input:         `PROGRAM Test; VAR x: INT; END_VAR REPEAT x := x + 1; END_REPEAT`,
			expectedError: "expected UNTIL, got END_REPEAT",
		},
		{
			name:          "Incomplete CASE statement",
			input:         `PROGRAM Test; VAR x: INT; END_VAR CASE x OF 1: x := 1; END_CASE`,
			expectedError: "no case branches found in CASE statement",
		},
		{
			name:          "Invalid token in expression",
			input:         `PROGRAM Test; VAR x: INT; END_VAR x := 1 + #; END_PROGRAM`,
			expectedError: "no prefix parse function for # found",
		},
		{
			name:          "Missing expression after operator",
			input:         `PROGRAM Test; VAR x: INT; END_VAR x := 1 +; END_PROGRAM`,
			expectedError: "unexpected token ;",
		},
		{
			name:          "Unclosed parenthesis",
			input:         `PROGRAM Test; VAR x: INT; END_VAR x := (1 + 2; END_PROGRAM`,
			expectedError: "expected next token to be ), got ;",
		},
		{
			name:          "Invalid function block call",
			input:         `PROGRAM Test; VAR fb: TON; END_VAR fb(IN := TRUE, PT: T#5s); END_PROGRAM`,
			expectedError: "expected := or => in function block parameter, got :",
		},
		{
			name:          "Missing transition condition",
			input:         `PROGRAM TestSFC; INITIAL_STEP S1: END_STEP; TRANSITION FROM S1 TO S2; END_TRANSITION; END_PROGRAM`,
			expectedError: "expected := after transition definition",
		},
		{
			name:          "Invalid array declaration",
			input:         `PROGRAM Test; VAR arr: ARRAY[1..10] OF; END_VAR END_PROGRAM`,
			expectedError: "expected a data type after 'OF' in array definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()

			errors := p.Errors()
			if len(errors) == 0 {
				t.Fatalf("expected parser errors, but got none")
			}

			found := false
			for _, err := range errors {
				if strings.Contains(err, tt.expectedError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected error message containing %q, but got:", tt.expectedError)
				for _, err := range errors {
					t.Logf("  - %s", err)
				}
			}
		})
	}
}
