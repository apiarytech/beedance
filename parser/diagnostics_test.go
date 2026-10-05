package parser

import (
	"testing"

	"github.com/apiarytech/beedance/lexer"
)

func TestDiagnostics(t *testing.T) {
	for _, c := range []struct {
		src      string
		row, col int
	}{
		{"PROGRAM P\nVAR x : INT; END_VAR\nx := ;\nEND_PROGRAM\n", 3, 0},
		{"PROGRAM P\nLD\n  RUNG a\n    A B\nEND_LD\nEND_PROGRAM\n", 3, 3},
	} {
		p := New(lexer.New(c.src))
		p.ParseProgram()
		ds := p.Diagnostics()
		if len(ds) == 0 {
			t.Fatalf("%q parsed", c.src)
		}
		if ds[0].Row != c.row || (c.col != 0 && ds[0].Column != c.col) {
			t.Errorf("%q: %+v, want row %d", c.src, ds[0], c.row)
		}
	}
}
