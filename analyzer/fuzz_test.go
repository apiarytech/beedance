/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// fuzzSeeds adds the example programs and inputs that once hung or crashed
// the parser to f's corpus.
func fuzzSeeds(f *testing.F) {
	for _, pattern := range []string{"../examples/*/*.st", "../embedded/bench/st/*.st", "../embedded/rig/pico/*.st"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil {
				f.Add(string(data))
			}
		}
	}
	for _, src := range []string{
		"PROGRAM P VAR x:INT; END_VAR x := 1;\nS\nEND_PROGRAM", // issue #8
		"FUNCTION_BLOCK F METHOD M END_METHOD PROPERTY S",      // issue #8
		"PROGRAM P\nmethod\nEND_PROGRAM",                       // issue #9
		"FUNCTION A method",                                    // issue #9
		"",
	} {
		f.Add(src)
	}
}

// FuzzLint checks that CheckSource (the lint entry point) returns, without panicking, on any input. Run it
// with: go test -run '^$' -fuzz '^FuzzLint$' -fuzztime 30s ./analyzer
func FuzzLint(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		CheckSource(src, Options{})
	})
}
