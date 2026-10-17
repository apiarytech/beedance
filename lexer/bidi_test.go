/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package lexer

import (
	"reflect"
	"testing"
)

func TestBidiControls(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Bidi
	}{
		{"none", "x := 1; (* café, Ω, 日本 *)", nil},
		{"in a comment", "x := 1; (* \u202E} *)", []Bidi{{0x202E, 1, 12}}},
		{"in a string, after a multi-byte rune", "s := 'é\u2066';", []Bidi{{0x2066, 1, 9}}},
		{"rows and columns", "a;\n\u200Fb;\r\n  \u061C\u2069", []Bidi{{0x200F, 2, 1}, {0x061C, 3, 3}, {0x2069, 3, 5}}},
		{"each of them", "\u061C\u200E\u200F\u202A\u202B\u202C\u202D\u202E\u2066\u2067\u2068\u2069",
			[]Bidi{{0x061C, 1, 1}, {0x200E, 1, 3}, {0x200F, 1, 6}, {0x202A, 1, 9}, {0x202B, 1, 12}, {0x202C, 1, 15}, {0x202D, 1, 18},
				{0x202E, 1, 21}, {0x2066, 1, 24}, {0x2067, 1, 27}, {0x2068, 1, 30}, {0x2069, 1, 33}}},
		{"neighbours are not", "\u200D\u2028\u2065\u206A\u202F", nil},
		{"invalid UTF-8", "\xff\xfe x", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BidiControls(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BidiControls(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// The column BidiControls gives is the lexer's: a token after the control
// character starts where the lexer says.
func TestBidiColumnsMatchLexer(t *testing.T) {
	input := "(* é *) \u202E"
	l := New(input)
	l.NextToken() // the comment
	illegal := l.NextToken()
	got := BidiControls(input)
	if len(got) != 1 || got[0].Row != illegal.Row || got[0].Column != illegal.Column {
		t.Errorf("BidiControls = %v, the lexer puts it at row %d, column %d", got, illegal.Row, illegal.Column)
	}
}
