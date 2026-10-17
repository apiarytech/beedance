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

import "unicode/utf8"

// Bidi is one Unicode bidirectional control character in a source: its
// rune and where it is, the row and column counted as the lexer counts
// them (1-based, in bytes).
type Bidi struct {
	Rune        rune
	Row, Column int
}

// BidiControls finds the bidirectional control characters in input
// (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069). In a
// comment or a string they can make the text read differently from how it
// compiles, the "Trojan Source" attack (CVE-2021-42574), so the parser
// refuses them anywhere. Identifiers are ASCII, so look-alike letters
// cannot hide in a name.
func BidiControls(input string) []Bidi {
	var out []Bidi
	row, col := 1, 0
	for i := 0; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])
		col++
		if isBidiControl(r) {
			out = append(out, Bidi{Rune: r, Row: row, Column: col})
		}
		if input[i] == '\n' {
			row, col = row+1, 0
		}
		col += size - 1 // the lexer counts bytes
		i += size
	}
	return out
}

func isBidiControl(r rune) bool {
	return r == 0x061C || r == 0x200E || r == 0x200F || 0x202A <= r && r <= 0x202E || 0x2066 <= r && r <= 0x2069
}
