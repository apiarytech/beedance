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

// This file flattens NAMESPACEs before transpiling, since the generated Go is
// one package. A declaration in a namespace takes its path as a prefix, so
// Lib.Inner.Half becomes Lib_Inner_Half, and every reference to it, qualified
// (Lib.Inner.Half) or, within the namespace, unqualified (Half), is renamed.

import (
	"strings"

	"beedance/ast"
)

// namespaceFlattener renames the declarations of namespaces.
type namespaceFlattener struct {
	// renamed maps a declaration's qualified name, in upper case, to its
	// name in the generated code.
	renamed map[string]string
	// names are identifiers that are names rather than references, such as
	// the member of a member access; they are not renamed.
	names map[*ast.Identifier]bool
}

// flattenNamespaces returns the program with its namespaces' declarations
// renamed and moved to the top level.
func flattenNamespaces(program *ast.Program) *ast.Program {
	type placed struct {
		stmt ast.Statement
		path []string // The namespace the statement is declared in.
	}
	f := &namespaceFlattener{renamed: map[string]string{}, names: map[*ast.Identifier]bool{}}
	flat := []placed{}
	var collect func(stmts []ast.Statement, path []string)
	collect = func(stmts []ast.Statement, path []string) {
		for _, stmt := range stmts {
			if ns, ok := stmt.(*ast.NamespaceDeclaration); ok {
				inner := append(append([]string{}, path...), strings.Split(ns.Name.String(), ".")...)
				collect(ns.Statements, inner)
				continue
			}
			flat = append(flat, placed{stmt, path})
			if len(path) == 0 {
				continue
			}
			for _, name := range declaredNames(stmt) {
				qualified := strings.Join(append(append([]string{}, path...), name.Value), ".")
				f.renamed[strings.ToUpper(qualified)] = strings.ReplaceAll(qualified, ".", "_")
			}
		}
	}
	collect(program.Statements, nil)
	if len(f.renamed) == 0 {
		return program
	}

	for _, p := range flat {
		f.collectNames(p.stmt)
	}
	out := &ast.Program{}
	for _, p := range flat {
		path := p.path
		stmt := ast.Modify(p.stmt, func(n ast.Node) ast.Node { return f.rename(n, path) }).(ast.Statement)
		for _, name := range declaredNames(stmt) {
			if renamed, ok := f.resolve(name.Value, path); ok {
				name.Value = renamed
			}
		}
		out.Statements = append(out.Statements, stmt)
	}
	return out
}

// declaredNames returns the names a top-level statement declares.
func declaredNames(stmt ast.Statement) []*ast.Identifier {
	switch s := stmt.(type) {
	case *ast.FunctionDeclaration:
		return []*ast.Identifier{s.Name}
	case *ast.FunctionBlockDeclaration:
		return []*ast.Identifier{s.Name}
	case *ast.ProgramDeclaration:
		return []*ast.Identifier{s.Name}
	case *ast.InterfaceDeclaration:
		return []*ast.Identifier{s.Name}
	case *ast.TypeBlockDeclaration:
		names := []*ast.Identifier{}
		for _, td := range s.Declarations {
			names = append(names, td.Name)
		}
		return names
	case *ast.TypeDeclaration:
		return []*ast.Identifier{s.Name}
	}
	return nil
}

// collectNames records the identifiers of stmt that are names rather than
// references.
func (f *namespaceFlattener) collectNames(stmt ast.Statement) {
	ast.Modify(stmt, func(n ast.Node) ast.Node {
		switch v := n.(type) {
		case *ast.MemberAccessExpression:
			f.names[v.Member] = true
		case *ast.NamedArgument:
			f.names[v.Name] = true
		case *ast.OutputArgument:
			f.names[v.Source] = true
		case *ast.FunctionDeclaration:
			f.names[v.Name] = true
		case *ast.FunctionBlockDeclaration:
			f.names[v.Name] = true
		case *ast.InterfaceDeclaration:
			f.names[v.Name] = true
		case *ast.MethodImplementation:
			f.names[v.Name] = true
		case *ast.MethodDeclaration:
			f.names[v.Name] = true
		case *ast.PropertyDeclaration:
			f.names[v.Name] = true
		}
		return n
	})
}

// resolve returns the generated name of name, qualified or not, referred to
// from the namespace path: the innermost namespace that declares it wins.
func (f *namespaceFlattener) resolve(name string, path []string) (string, bool) {
	for i := len(path); i >= 0; i-- {
		qualified := strings.Join(append(append([]string{}, path[:i]...), name), ".")
		if renamed, ok := f.renamed[strings.ToUpper(qualified)]; ok {
			return renamed, true
		}
	}
	return "", false
}

// rename replaces a reference to a namespace's declaration with its
// generated name.
func (f *namespaceFlattener) rename(n ast.Node, path []string) ast.Node {
	switch v := n.(type) {
	case *ast.Identifier:
		if !f.names[v] {
			if renamed, ok := f.resolve(v.Value, path); ok {
				return &ast.Identifier{Token: v.Token, Value: renamed}
			}
		}
	case *ast.MemberAccessExpression:
		if qualified, ok := dottedName(v); ok {
			if renamed, ok := f.resolve(qualified, path); ok {
				return &ast.Identifier{Token: v.Member.Token, Value: renamed}
			}
		}
	case *ast.TypeSpecifier:
		if renamed, ok := f.resolve(v.Token.Literal, path); ok {
			spec := *v
			spec.Token.Literal = renamed
			return &spec
		}
	case *ast.TypedLiteral:
		if renamed, ok := f.resolve(v.TypeName, path); ok {
			v.TypeName = renamed
		}
	case *ast.EnumeratedValueLiteral:
		if v.TypeName != nil {
			if renamed, ok := f.resolve(v.TypeName.Value, path); ok {
				v.TypeName = &ast.Identifier{Token: v.TypeName.Token, Value: renamed}
			}
		}
	}
	return n
}

// dottedName returns a member access of identifiers, such as Lib.Inner.Half,
// as a dotted name.
func dottedName(e ast.Expression) (string, bool) {
	switch v := e.(type) {
	case *ast.Identifier:
		return v.Value, true
	case *ast.MemberAccessExpression:
		owner, ok := dottedName(v.Struct)
		if !ok || v.Member == nil {
			return "", false
		}
		return owner + "." + v.Member.Value, true
	}
	return "", false
}
