/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package embedded

import (
	"bytes"
	"fmt"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	beeparser "github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/vm"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestGenerateEmbeddedGoSizesGlobals checks that the generated program asks
// the VM for only the globals its bytecode uses, not vm.GlobalsSize, which
// does not fit in a microcontroller's RAM.
func TestGenerateEmbeddedGoSizesGlobals(t *testing.T) {
	input := "VAR_GLOBAL a : INT := 1; b : INT := 2; END_VAR a + b;"
	p := beeparser.New(lexer.New(input))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %s", strings.Join(errs, "; "))
	}
	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		t.Fatalf("compiler error: %s", err)
	}
	bytecode := comp.Bytecode()

	var out bytes.Buffer
	if err := GenerateEmbeddedGo(&out, bytecode); err != nil {
		t.Fatalf("GenerateEmbeddedGo: %s", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", out.Bytes(), 0); err != nil {
		t.Fatalf("generated program does not parse: %s\n%s", err, out.String())
	}

	n := vm.GlobalsNeeded(bytecode)
	if n == 0 || n >= vm.GlobalsSize {
		t.Fatalf("GlobalsNeeded = %d, want between 1 and %d", n, vm.GlobalsSize-1)
	}
	want := fmt.Sprintf("vm.New(bytecode, vm.WithGlobalsSize(%d))", n)
	if !strings.Contains(out.String(), want) {
		t.Errorf("generated program does not contain %q:\n%s", want, out.String())
	}
}
