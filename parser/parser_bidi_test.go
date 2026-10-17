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
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
)

// A bidirectional control character is refused wherever it is, comments,
// strings and diagram bodies too, once each (security review, 2026-10-14).
func TestBidiControlsRefused(t *testing.T) {
	tests := []struct {
		name, input string
		want        []string // the bidi errors; nil: none
	}{
		{"clean, other Unicode kept", "PROGRAM P\nVAR s : STRING; END_VAR\ns := 'café Ω'; (* 日本 *)\nEND_PROGRAM\n", nil},
		{"in a comment", "PROGRAM P\nVAR x : BOOL; END_VAR\nx := TRUE; (* \u202E ; ESLAF =: x *)\nEND_PROGRAM\n",
			[]string{"bidirectional control character U+202E at row 3, column 15"}},
		{"in a string", "PROGRAM P\nVAR s : STRING; END_VAR\ns := 'ok\u2066';\nEND_PROGRAM\n",
			[]string{"bidirectional control character U+2066 at row 3, column 9"}},
		{"in a ladder rung's comment", "PROGRAM P\nVAR a, b : BOOL; END_VAR\nLD\n  RUNG r (* \u2067 *)\n    a ( b )\nEND_LD\nEND_PROGRAM\n",
			[]string{"bidirectional control character U+2067 at row 4, column 13"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(lexer.New(tt.input))
			p.ParseProgram()
			var got []string
			for _, e := range p.Errors() {
				if strings.HasPrefix(e, "bidirectional control character") {
					got = append(got, e[:strings.Index(e, ":")])
				}
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("bidi errors = %q, want %q (all errors: %q)", got, tt.want, p.Errors())
			}
			if tt.want == nil && len(p.Errors()) > 0 {
				t.Errorf("errors in a clean source: %q", p.Errors())
			}
		})
	}
}
