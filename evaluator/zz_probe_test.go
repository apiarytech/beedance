package evaluator

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
)

func TestZZProbe(t *testing.T) {
	data, _ := os.ReadFile(os.Getenv("PROBE"))
	for _, in := range strings.Split(string(data), "\n----\n") {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		p := parser.New(lexer.New(in))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			fmt.Printf("%-60q => PARSE: %s\n", in, p.Errors()[0])
			continue
		}
		r := Eval(prog, object.NewEnvironment())
		s := "<nil>"
		if r != nil {
			s = string(r.Type()) + " " + r.Inspect()
		}
		fmt.Printf("%-60q => %s\n", in, s)
	}
}
