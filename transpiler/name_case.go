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

// IEC 61131-3 names ignore case, and Go's do not: OSCAT declares `DIVI` and
// uses `divi`. This file spells every use of a declared name as it is
// declared: the program's POUs, types and global variables everywhere, a
// POU's variables within it, and the members of its function blocks and
// structures, in member access and named arguments, where the program
// spells a member only one way.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// spellings maps the upper case of names to their declared spelling.
type spellings map[string]string

func (s spellings) add(name *ast.Identifier) {
	if name == nil {
		return
	}
	if _, ok := s[strings.ToUpper(name.Value)]; !ok {
		s[strings.ToUpper(name.Value)] = name.Value
	}
}

func (s spellings) copy() spellings {
	out := spellings{}
	for k, v := range s {
		out[k] = v
	}
	return out
}

// matchNameCase returns the program with the uses of its declared names
// spelled as declared.
func matchNameCase(program *ast.Program) *ast.Program {
	global := spellings{}
	members := map[string]map[string]bool{}
	addMember := func(name *ast.Identifier) {
		if name == nil {
			return
		}
		up := strings.ToUpper(name.Value)
		if members[up] == nil {
			members[up] = map[string]bool{}
		}
		members[up][name.Value] = true
	}
	for _, stmt := range program.Statements {
		global.add(declarationName(stmt))
		switch s := stmt.(type) {
		case *ast.GlobalVarDeclaration:
			for _, v := range s.Vars {
				global.add(v.Name)
			}
		case *ast.ProgramDeclaration:
			for _, g := range s.VarGlobal {
				for _, v := range g.Vars {
					global.add(v.Name)
				}
			}
		}
		ast.Modify(stmt, func(n ast.Node) ast.Node {
			switch v := n.(type) {
			case *ast.FunctionBlockDeclaration:
				for _, block := range [][]*ast.VarDeclStatement{v.VarInputs, v.VarOutputs, v.VarInOuts, v.Vars} {
					for _, d := range block {
						addMember(d.Name)
					}
				}
			case *ast.FunctionDeclaration:
				for _, block := range [][]*ast.VarDeclStatement{v.VarInputs, v.VarOutputs, v.VarInOuts} {
					for _, d := range block {
						addMember(d.Name)
					}
				}
			case *ast.StructDefinition:
				for _, d := range v.Members {
					addMember(d.Name)
				}
			}
			return n
		})
	}
	member := func(id *ast.Identifier) {
		if id == nil {
			return
		}
		spelled := members[strings.ToUpper(id.Value)]
		if len(spelled) != 1 {
			return
		}
		for s := range spelled {
			id.Value = s
		}
	}

	for i, stmt := range program.Statements {
		local := global.copy()
		// The POU's own variables, which hide global names.
		ast.Modify(stmt, func(n ast.Node) ast.Node {
			if v, ok := n.(*ast.VarDeclStatement); ok && v.Name != nil {
				local[strings.ToUpper(v.Name.Value)] = v.Name.Value
			}
			return n
		})
		// Names that are not variables of the POU: members, argument names
		// and the declarations themselves.
		skip := map[*ast.Identifier]bool{}
		types := map[*ast.Identifier]bool{}
		ast.Modify(stmt, func(n ast.Node) ast.Node {
			switch v := n.(type) {
			case *ast.MemberAccessExpression:
				skip[v.Member] = true
				member(v.Member)
			case *ast.NamedArgument:
				skip[v.Name] = true
				member(v.Name)
			case *ast.OutputArgument:
				skip[v.Source] = true
				member(v.Source)
			case *ast.VarDeclStatement:
				skip[v.Name] = true
				// A type named like a variable is the type.
				if id, ok := v.DataType.(*ast.Identifier); ok {
					types[id] = true
				}
			}
			return n
		})
		program.Statements[i] = ast.Modify(stmt, func(n ast.Node) ast.Node {
			switch v := n.(type) {
			case *ast.Identifier:
				if skip[v] {
					return n
				}
				if types[v] {
					if s, ok := global[strings.ToUpper(v.Value)]; ok {
						v.Value = s
					}
					return n
				}
				if s, ok := local[strings.ToUpper(v.Value)]; ok {
					v.Value = s
				}
			case *ast.TypeSpecifier:
				if s, ok := global[strings.ToUpper(v.Token.Literal)]; ok && s != v.Token.Literal {
					spec := *v
					spec.Token.Literal = s
					return &spec
				}
			}
			return n
		}).(ast.Statement)
	}
	return program
}
