package parser

import (
	"fmt"
	"testing"
	"time"

	"github.com/apiarytech/beedance/lexer"
)

// parseWithin parses src and fails the test if ParseProgram panics or does
// not return within a few seconds.
func parseWithin(t *testing.T, src string) *Parser {
	t.Helper()
	done := make(chan *Parser, 1)
	panicked := make(chan any, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicked <- r
			}
		}()
		p := New(lexer.New(src))
		p.ParseProgram()
		done <- p
	}()
	select {
	case p := <-done:
		return p
	case r := <-panicked:
		t.Fatalf("ParseProgram(%q) panicked: %v", src, r)
	case <-time.After(5 * time.Second):
		t.Fatalf("ParseProgram(%q) did not return", src)
	}
	return nil
}

// TestBareILMnemonicTerminatesIssue8 covers issue #8: a bare IL mnemonic
// before END_PROGRAM, or a PROPERTY without END_PROPERTY, made the parser
// loop for ever.
func TestBareILMnemonicTerminatesIssue8(t *testing.T) {
	for _, op := range []string{"S", "R", "ST", "LD", "ADD", "NOT"} {
		parseWithin(t, fmt.Sprintf("PROGRAM P VAR x:INT; END_VAR x := 1;\n%s\nEND_PROGRAM\n", op))
	}
	p := parseWithin(t, "FUNCTION_BLOCK F METHOD M END_METHOD PROPERTY S")
	if len(p.Errors()) == 0 {
		t.Error("a PROPERTY without END_PROPERTY parsed without errors")
	}
}

// TestMethodWithoutNameIssue9 covers issue #9: a METHOD without a name is a
// parse error, not a nil pointer dereference.
func TestMethodWithoutNameIssue9(t *testing.T) {
	for _, src := range []string{"PROGRAM P\nmethod\nEND_PROGRAM", "FUNCTION A method", "method"} {
		if p := parseWithin(t, src); len(p.Errors()) == 0 {
			t.Errorf("%q parsed without errors", src)
		}
	}
}
