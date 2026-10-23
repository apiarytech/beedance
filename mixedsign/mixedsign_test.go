package mixedsign

import (
	"strings"
	"testing"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

func check(t *testing.T, src string) error {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse %q: %v", src, p.Errors())
	}
	return Check(prog)
}

// TestCheck covers issue #18: signed and unsigned integers neither of which
// holds the other are not mixed without a conversion.
func TestCheck(t *testing.T) {
	refused := []string{
		"VAR u : UDINT := 1; d : DINT := -2; r2 : LINT; END_VAR r2 := u + d;",
		"VAR u : UINT; i : INT; x : INT; END_VAR x := i - u;",
		"VAR u : ULINT; l : LINT; x : LINT; END_VAR x := l * u;",
		"VAR d : DINT; x : DINT; END_VAR x := d / UDINT#2;",
		"PROGRAM P VAR a : USINT; b : SINT; c : INT; END_VAR c := a MOD b; END_PROGRAM",
		"FUNCTION_BLOCK F VAR u : UDINT; END_VAR METHOD M VAR d : DINT; x : LINT; END_VAR x := u + d; END_METHOD END_FUNCTION_BLOCK",
	}
	for _, src := range refused {
		if err := check(t, src); err == nil || !strings.Contains(err.Error(), "mixes signed and unsigned") {
			t.Errorf("%q: %v, want it refused", src, err)
		}
	}
	allowed := []string{
		"VAR a : USINT; i : INT; x : INT; END_VAR x := a + i;",                                                                                 // every USINT is an INT
		"VAR u : UINT; d : DINT; x : DINT; END_VAR x := u + d;",                                                                                // every UINT is a DINT
		"VAR u : UDINT; d : DINT; x : LINT; END_VAR x := UDINT_TO_LINT(u) + d;",                                                                // converted
		"VAR u : UDINT; x : UDINT; END_VAR x := u + 1;",                                                                                        // an untyped literal
		"VAR i : INT; j : DINT; x : DINT; END_VAR x := i + j;",                                                                                 // same signedness
		"VAR u : UDINT; d : DINT; b : BOOL; END_VAR b := u > 0 AND d < 0;",                                                                     // comparisons are not checked
		"FUNCTION_BLOCK F VAR u : UDINT; END_VAR METHOD M VAR u : DINT; d : DINT; x : DINT; END_VAR x := u + d; END_METHOD END_FUNCTION_BLOCK", // the method's u
	}
	for _, src := range allowed {
		if err := check(t, src); err != nil {
			t.Errorf("%q: %v, want it allowed", src, err)
		}
	}
}
