/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package native_test

import (
	"fmt"
	"time"

	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/native"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
)

// scaler is a function block implemented in Go: OUT := IN * K.
type scaler struct {
	IN, K int16 // INT
	OUT   int16
}

func (s *scaler) Execute(time.Time) { s.OUT = s.IN * s.K }

// A host gives a program a block implemented in Go: it makes a Registry of
// the blocks it allows, puts the registry's Source in front of the program
// and binds the registry's built-ins. On the VM, the host compiles with the
// same Source and calls SetBuiltin with each of Builtins instead of Bind.
func Example() {
	r, err := native.NewRegistry(time.Now, "", native.Block{
		Name:    "Scaler",
		Inputs:  []native.Field{{Name: "IN", Type: "INT"}, {Name: "K", Type: "INT"}},
		Outputs: []native.Field{{Name: "OUT", Type: "INT"}},
		New:     func() native.Instance { return &scaler{} },
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	src := r.Source() + `VAR s : Scaler; y : INT; END_VAR
s(IN := 21, K := 2);
y := s.OUT;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		fmt.Println(errs)
		return
	}
	env := object.NewEnvironment()
	r.Bind(env)
	if res := evaluator.Eval(prog, env); res != nil && res.Type() == object.ERROR_OBJ {
		fmt.Println(res.Inspect())
		return
	}
	y, _ := env.Get("y")
	fmt.Println(y.Inspect())
	// Output: 42
}
