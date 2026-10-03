/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// This file generates, with Options.HostBinding, what a host needs to bind a
// program's variables to its own data: New<Program>() returns a program with
// its initial values, and Variables() lists its variables of elementary types
// as royaljelly core.Variable, with closures that read and write each field
// without reflection. A host such as beehive's logic service binds them to
// tags and keeps RETAIN variables across restarts.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

func (t *Transpiler) transpileHostBinding(prog *ast.ProgramDeclaration, blocks [][]*ast.VarDeclStatement) error {
	name := prog.Name.Value

	// The constructor: the factory's initialization, returning the program.
	t.write("\n// New%s returns a %s program with its initial values.\n", name, name)
	t.write("func New%s() *%s {\n", name, name)
	t.write("\tinstance := &%s{}\n", name)
	t.programVarName = "instance"
	for _, block := range blocks {
		for _, decl := range block {
			if err := t.transpileProgramVarInit(decl); err != nil {
				t.programVarName = "p"
				return err
			}
		}
	}
	t.programVarName = "p"
	if err := t.transpileConfiguredVars(prog); err != nil {
		return err
	}
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		if initialStep := t.findInitialStep(sfc); initialStep != nil {
			t.write("\tinstance.sfcActiveSteps = make(map[string]bool)\n")
			t.write("\tinstance.sfcActiveSteps[%q] = true\n", initialStep.Name.Value)
		}
	}
	t.write("\treturn instance\n}\n\n")

	// The variable table.
	t.write("// Variables lists %s's variables of elementary types for its host.\n", name)
	t.write("func (p *%s) Variables() []core.Variable {\n", name)
	t.write("\treturn []core.Variable{\n")
	blockNames := []string{"VAR_INPUT", "VAR_OUTPUT", "VAR"}
	for i, block := range blocks {
		for _, decl := range block {
			if decl.Name == nil || decl.DataType == nil {
				continue
			}
			if _, macro := decl.Value.(*ast.MacroLiteral); macro {
				continue
			}
			goType := t.mapIecTypeToGo(decl.DataType)
			if !strings.HasPrefix(goType, "iec.") || strings.ContainsAny(goType, "[]*") {
				continue // a function block, structure or array: not one value
			}
			field := decl.Name.Value
			address := ""
			if decl.Location != nil {
				address = decl.Location.Location.String()
			}
			t.write("\t\t{Name: %q, Block: %q, Type: %q, Address: %q, Retain: %t, Constant: %t,\n",
				field, blockNames[i], strings.TrimPrefix(goType, "iec."), address, decl.IsRetain, decl.IsConstant)
			t.write("\t\t\tGet: func() any { return p.%s },\n", field)
			t.write("\t\t\tSet: func(v any) bool { x, ok := v.(%s); if ok { p.%s = x }; return ok }},\n", goType, field)
		}
	}
	t.write("\t}\n}\n")
	return nil
}
