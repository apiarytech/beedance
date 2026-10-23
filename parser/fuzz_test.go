package parser

import (
	"github.com/apiarytech/beedance/lexer"
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

// FuzzParse checks that ParseProgram returns, without panicking, on any input. Run it
// with: go test -run '^$' -fuzz '^FuzzParse$' -fuzztime 30s ./parser
func FuzzParse(f *testing.F) {
	fuzzSeeds(f)
	f.Fuzz(func(t *testing.T, src string) {
		p := New(lexer.New(src))
		p.ParseProgram()
		_ = p.Errors()
	})
}
