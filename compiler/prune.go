/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// reachable keeps, of pous, the functions, function blocks and interfaces
// that target uses, directly or through one another, and every other
// statement (types, globals, namespaces). A program beside a library such
// as OSCAT (over 500 POUs) then compiles the few it calls, not all of them.
func reachable(target ast.Node, pous []ast.Statement) []ast.Statement {
	byName := map[string][]int{}
	for i, s := range pous {
		if name := pouName(s); name != "" {
			byName[name] = append(byName[name], i)
		}
	}
	keep := make([]bool, len(pous))
	var queue []ast.Node
	visit := func(n ast.Node) {
		for name := range names(n) {
			for _, i := range byName[name] {
				if !keep[i] {
					keep[i] = true
					queue = append(queue, pous[i])
				}
			}
		}
	}
	visit(target)
	for i, s := range pous {
		if pouName(s) == "" {
			keep[i] = true
			visit(s) // a global's type, a type's members
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		visit(n)
	}
	var out []ast.Statement
	for i, s := range pous {
		if keep[i] {
			out = append(out, s)
		}
	}
	return out
}

// pouName is the upper-case name of a function, function block or
// interface; empty for any other statement.
func pouName(s ast.Statement) string {
	switch n := s.(type) {
	case *ast.FunctionDeclaration:
		return strings.ToUpper(n.Name.Value)
	case *ast.FunctionBlockDeclaration:
		return strings.ToUpper(n.Name.Value)
	case *ast.InterfaceDeclaration:
		return strings.ToUpper(n.Name.Value)
	}
	return ""
}

// names are the upper-case identifiers in n.
func names(n ast.Node) map[string]bool {
	out := map[string]bool{}
	ast.Modify(n, func(x ast.Node) ast.Node {
		switch id := x.(type) {
		case *ast.Identifier:
			if id != nil {
				out[strings.ToUpper(id.Value)] = true
			}
		case *ast.TypeSpecifier:
			if id != nil {
				out[strings.ToUpper(id.Token.Literal)] = true // a function's return type
			}
		}
		return x
	})
	return out
}
