package ide

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

// FuzzAnalyze checks that Analyze (the LSP parses every keystroke) returns, without panicking, on any input. Run it
// with: go test -run '^$' -fuzz '^FuzzAnalyze$' -fuzztime 30s ./ide
func FuzzAnalyze(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		Analyze(src)
	})
}
