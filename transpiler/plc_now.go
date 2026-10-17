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

// A function block's Logic and a timer's Execute take now, the scan's time.
// A Logic method has it; a FUNCTION, a METHOD or a property does not, so a
// function block it calls (an instance passed VAR_IN_OUT, or one of the
// method's function block) runs with plcNow, which each Logic sets. Only a
// program that needs it gets plcNow.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// needsPlcNow reports whether a function, a method or a property of the
// program calls a function block instance.
func (t *Transpiler) needsPlcNow(program *ast.Program) bool {
	for _, stmt := range program.Statements {
		switch d := stmt.(type) {
		case *ast.FunctionDeclaration:
			if t.callsInstance(d.Body, d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars) {
				return true
			}
		case *ast.FunctionBlockDeclaration:
			own := [][]*ast.VarDeclStatement{d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars}
			if body, ok := d.Body.(*ast.BlockStatement); ok {
				for _, s := range body.Statements {
					if m, isMethod := s.(*ast.MethodImplementation); isMethod {
						blocks := append([][]*ast.VarDeclStatement{m.VarInputs, m.VarOutputs, m.VarInOuts, m.Vars}, own...)
						if t.callsInstance(m.Body, blocks...) {
							return true
						}
					}
				}
			}
			for _, p := range d.Properties {
				if p.Getter != nil && p.Getter.Body != nil && t.callsInstance(p.Getter.Body, own...) {
					return true
				}
				if p.Setter != nil && p.Setter.Body != nil && t.callsInstance(p.Setter.Body, own...) {
					return true
				}
			}
		}
	}
	return false
}

// callsInstance reports whether body calls a function block instance
// declared in blocks, by name, member or element (f(), f.inner(), fs[i]()).
func (t *Transpiler) callsInstance(body ast.Node, blocks ...[]*ast.VarDeclStatement) bool {
	if body == nil {
		return false
	}
	instances := map[string]bool{}
	for _, block := range blocks {
		for _, decl := range block {
			dataType := decl.DataType
			if def, isArray := dataType.(*ast.ArrayDefinition); isArray && def.DataType != nil {
				dataType = def.DataType
			}
			if dataType != nil && t.isFunctionBlockType(dataType) {
				instances[strings.ToUpper(decl.Name.Value)] = true
			}
		}
	}
	if len(instances) == 0 {
		return false
	}
	found := false
	ast.Modify(body, func(n ast.Node) ast.Node {
		if call, ok := n.(*ast.CallExpression); ok && !found {
			root := call.Function
			for {
				switch r := root.(type) {
				case *ast.IndexExpression:
					root = r.Left
					continue
				case *ast.MemberAccessExpression:
					root = r.Struct
					continue
				}
				break
			}
			if id, ok := root.(*ast.Identifier); ok && instances[strings.ToUpper(id.Value)] {
				found = true
			}
		}
		return n
	})
	return found
}
