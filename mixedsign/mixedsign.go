/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package mixedsign refuses arithmetic that mixes a signed and an unsigned
// integer type neither of which holds every value of the other, such as
// UDINT + DINT: strict IEC 61131-3 has no implicit conversion between them,
// and the engines computed different results (the evaluator and the VM -1,
// the transpiled Go 4294967295). The owner chose a compile error (issue
// #18), for every engine alike: the evaluator, the VM's compiler and the
// transpiler run Check before anything else.
//
// What is refused: +, -, *, / or MOD between two operands whose types are
// known here, a variable declared in the POU (or globally) or a typed
// literal (UDINT#5), when one is signed and the other unsigned and neither
// fits the other (IntegerFits). USINT + INT is allowed, since every USINT is
// an INT. An untyped literal, a member, an element, a call or a nested
// expression has no known type and is not checked.
package mixedsign

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// Check returns the first operation in program that mixes signed and
// unsigned integers, as an error with its position, or nil.
func Check(program *ast.Program) error {
	if program == nil {
		return nil
	}
	globals := scope{}
	for _, s := range program.Statements {
		switch d := s.(type) {
		case *ast.VarBlockDeclaration:
			globals.add(d.Declarations)
		case *ast.GlobalVarDeclaration:
			globals.add(d.Vars)
		}
	}
	return checkStatements(program.Statements, globals)
}

// scope maps the upper-cased names of variables to their type names.
type scope map[string]string

func (s scope) add(decls []*ast.VarDeclStatement) {
	for _, d := range decls {
		if d == nil || d.Name == nil || d.DataType == nil {
			continue
		}
		s[strings.ToUpper(d.Name.Value)] = strings.ToUpper(d.DataType.String())
	}
}

func (s scope) with(lists ...[]*ast.VarDeclStatement) scope {
	out := scope{}
	for k, v := range s {
		out[k] = v
	}
	for _, l := range lists {
		out.add(l)
	}
	return out
}

func checkStatements(stmts []ast.Statement, globals scope) error {
	for _, s := range stmts {
		var err error
		switch d := s.(type) {
		case *ast.ProgramDeclaration:
			err = checkBody(d.Body, globals.with(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars))
		case *ast.FunctionDeclaration:
			err = checkBody(d.Body, globals.with(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars))
		case *ast.FunctionBlockDeclaration:
			err = checkFunctionBlock(d, globals)
		case *ast.NamespaceDeclaration:
			err = checkStatements(d.Statements, globals)
		case *ast.VarBlockDeclaration, *ast.GlobalVarDeclaration:
		default:
			err = checkBody(s, globals) // a statement at the top level
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// checkFunctionBlock checks a block's body and each of its methods. A name a
// method declares is left out of the block's scope: the method's own
// variable shadows the block's in its body, which the walk enters too.
func checkFunctionBlock(fb *ast.FunctionBlockDeclaration, globals scope) error {
	own := globals.with(fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars)
	var methods []*ast.MethodImplementation
	if body, ok := fb.Body.(*ast.BlockStatement); ok {
		for _, s := range body.Statements {
			if m, isMethod := s.(*ast.MethodImplementation); isMethod && m != nil {
				methods = append(methods, m)
			}
		}
	}
	blockScope := own.with()
	for _, m := range methods {
		for _, l := range [][]*ast.VarDeclStatement{m.VarInputs, m.VarOutputs, m.VarInOuts, m.Vars} {
			for _, d := range l {
				if d != nil && d.Name != nil {
					delete(blockScope, strings.ToUpper(d.Name.Value))
				}
			}
		}
	}
	if err := checkBody(fb.Body, blockScope); err != nil {
		return err
	}
	for _, m := range methods {
		if err := checkBody(m.Body, own.with(m.VarInputs, m.VarOutputs, m.VarInOuts, m.Vars)); err != nil {
			return err
		}
	}
	return nil
}

// arithmetic are the operators checked.
var arithmetic = map[string]bool{"+": true, "-": true, "*": true, "/": true, "MOD": true}

func checkBody(body ast.Node, names scope) (err error) {
	if body == nil || err != nil {
		return nil
	}
	ast.Modify(body, func(n ast.Node) ast.Node {
		inf, ok := n.(*ast.InfixExpression)
		if !ok || err != nil || !arithmetic[strings.ToUpper(inf.Operator)] {
			return n
		}
		left, right := typeOf(inf.Left, names), typeOf(inf.Right, names)
		if mixes(left, right) {
			err = fmt.Errorf("%s %s %s at row %d, column %d mixes signed and unsigned integers: convert one operand (%s_TO_%s or %s_TO_%s)",
				left, inf.Operator, right, inf.Token.Row, inf.Token.Column, left, right, right, left)
		}
		return n
	})
	return err
}

// typeOf is the integer type name of e when known here, else "".
func typeOf(e ast.Expression, names scope) string {
	switch e := e.(type) {
	case *ast.Identifier:
		return names[strings.ToUpper(e.Value)]
	case *ast.TypedLiteral:
		return strings.ToUpper(e.TypeName)
	}
	return ""
}

// mixes reports whether a and b are integer types of different signedness
// neither of which holds every value of the other.
func mixes(a, b string) bool {
	if !object.IsIntegerType(a) || !object.IsIntegerType(b) {
		return false
	}
	if strings.HasPrefix(a, "U") == strings.HasPrefix(b, "U") {
		return false
	}
	return !object.IntegerFits(a, b) && !object.IntegerFits(b, a)
}
