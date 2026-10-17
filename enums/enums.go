/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package enums lets a program name an enumeration's value without its
// type, where that is clear (IEC 61131-3):
//
//	TYPE COLOR : (RED, GREEN, BLUE); END_TYPE
//	CASE c OF RED: ... GREEN: ... END_CASE
//	IF c = BLUE THEN c := RED; END_IF
//
// A name that the POU does not declare, and that is a value of exactly one
// enumeration, is that value (COLOR#RED) for every engine alike. A name
// two enumerations share is left as it is, to be written in full.
package enums

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/token"
)

// Qualify gives the unqualified enumeration values in the program's POU
// bodies their type, in place.
func Qualify(program *ast.Program) {
	if program == nil {
		return
	}
	values := map[string]string{} // value, upper case -> its type; "" when shared
	globals := map[string]bool{}  // names declared outside the POUs, upper case
	var collect func(stmts []ast.Statement, prefix string)
	collect = func(stmts []ast.Statement, prefix string) {
		for _, s := range stmts {
			switch d := s.(type) {
			case *ast.TypeBlockDeclaration:
				for _, td := range d.Declarations {
					addType(values, td, prefix)
					globals[strings.ToUpper(td.Name.Value)] = true
				}
			case *ast.TypeDeclaration:
				addType(values, d, prefix)
				globals[strings.ToUpper(d.Name.Value)] = true
			case *ast.GlobalVarDeclaration:
				for _, v := range d.Vars {
					if v != nil && v.Name != nil {
						globals[strings.ToUpper(v.Name.Value)] = true
					}
				}
			case *ast.FunctionDeclaration:
				globals[strings.ToUpper(d.Name.Value)] = true
			case *ast.FunctionBlockDeclaration:
				globals[strings.ToUpper(d.Name.Value)] = true
			case *ast.ProgramDeclaration:
				globals[strings.ToUpper(d.Name.Value)] = true
			case *ast.NamespaceDeclaration:
				name := d.Name.String()
				if prefix != "" {
					name = prefix + "." + name
				}
				collect(d.Statements, name)
			}
		}
	}
	collect(program.Statements, "")
	if len(values) == 0 {
		return
	}

	var qualify func(stmts []ast.Statement)
	qualify = func(stmts []ast.Statement) {
		for _, s := range stmts {
			switch d := s.(type) {
			case *ast.ProgramDeclaration:
				local := declared(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars)
				for _, g := range d.VarGlobal {
					addNames(local, g.Vars)
				}
				for _, t := range d.VarTemp {
					addNames(local, t.Vars)
				}
				body(d.Body, values, globals, local)
			case *ast.FunctionBlockDeclaration:
				local := declared(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars)
				for _, t := range d.VarTemp {
					addNames(local, t.Vars)
				}
				body(d.Body, values, globals, local)
			case *ast.FunctionDeclaration:
				local := declared(d.VarInputs, d.VarOutputs, d.VarInOuts, d.Vars)
				body(d.Body, values, globals, local)
			case *ast.NamespaceDeclaration:
				qualify(d.Statements)
			}
		}
	}
	qualify(program.Statements)
}

// addType adds the values of an enumeration type to values.
func addType(values map[string]string, td *ast.TypeDeclaration, prefix string) {
	def, ok := td.DataType.(*ast.EnumDefinition)
	if !ok {
		return
	}
	typeName := td.Name.Value
	if prefix != "" {
		typeName = prefix + "." + typeName
	}
	for _, v := range def.Values {
		key := strings.ToUpper(v.Value)
		if other, seen := values[key]; seen && !strings.EqualFold(other, typeName) {
			values[key] = "" // shared by two enumerations: not clear
			continue
		}
		values[key] = typeName
	}
}

func declared(blocks ...[]*ast.VarDeclStatement) map[string]bool {
	out := map[string]bool{}
	for _, b := range blocks {
		addNames(out, b)
	}
	return out
}

func addNames(out map[string]bool, decls []*ast.VarDeclStatement) {
	for _, d := range decls {
		if d != nil && d.Name != nil {
			out[strings.ToUpper(d.Name.Value)] = true
		}
	}
}

// body qualifies the enumeration values a POU body names. A name is left
// alone where it is not a value read (a member, an argument's name, a call,
// an assignment's target), and where the POU or the program declares it.
func body(b ast.Node, values map[string]string, globals, local map[string]bool) {
	if b == nil {
		return
	}
	// The identifiers that are not values read.
	kept := map[*ast.Identifier]bool{}
	ast.Modify(b, func(n ast.Node) ast.Node {
		switch x := n.(type) {
		case *ast.MemberAccessExpression:
			kept[x.Member] = true
		case *ast.NamedArgument:
			kept[x.Name] = true
		case *ast.OutputArgument:
			kept[x.Source] = true
			if id, ok := x.Target.(*ast.Identifier); ok {
				kept[id] = true
			}
		case *ast.CallExpression:
			if id, ok := x.Function.(*ast.Identifier); ok {
				kept[id] = true
			}
		case *ast.AssignmentStatement:
			if id, ok := x.Left.(*ast.Identifier); ok {
				kept[id] = true
			}
		}
		return n
	})
	ast.Modify(b, func(n ast.Node) ast.Node {
		id, ok := n.(*ast.Identifier)
		if !ok || kept[id] {
			return n
		}
		key := strings.ToUpper(id.Value)
		typeName, isValue := values[key]
		if !isValue || typeName == "" || local[key] || globals[key] {
			return n
		}
		tok := token.Token{Type: token.IDENT, Literal: typeName, Row: id.Token.Row, Column: id.Token.Column}
		return &ast.EnumeratedValueLiteral{Token: tok, TypeName: &ast.Identifier{Token: tok, Value: typeName}, Value: id}
	})
}
