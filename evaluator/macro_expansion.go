/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package evaluator

import (
	"beedance/ast"
	"beedance/object"
)

// DefineMacros finds all macro definitions within a program's AST, adds them
// to the given macro environment, and then removes them from the AST. This
// prevents macros from being evaluated as regular variables and prepares them
// for the expansion phase.
func DefineMacros(program *ast.Program, env *object.Environment) {
	var newStmts []ast.Statement

	for _, stmt := range program.Statements {
		if varBlock, ok := stmt.(*ast.VarBlockDeclaration); ok {
			var newDecls []*ast.VarDeclStatement
			// Iterate over declarations within the VAR block
			for _, decl := range varBlock.Declarations {
				if isMacroDefinition(decl) {
					addMacro(decl, env)
					// Do not add the macro declaration to the new list
				} else {
					newDecls = append(newDecls, decl)
				}
			}
			// If the VAR block is not empty after removing macros, keep it.
			if len(newDecls) > 0 {
				varBlock.Declarations = newDecls
				newStmts = append(newStmts, varBlock)
			}
			// If the VAR block becomes empty, it is effectively removed.
		} else {
			// Keep non-VAR block statements
			newStmts = append(newStmts, stmt)
		}
	}
	program.Statements = newStmts
}

// isMacroDefinition checks if a given AST statement is a variable declaration
// that defines a macro (i.e., its value is a MacroLiteral).
func isMacroDefinition(node ast.Statement) bool {
	varDecl, ok := node.(*ast.VarDeclStatement)
	if !ok {
		return false
	}

	_, ok = varDecl.Value.(*ast.MacroLiteral)
	return ok
}

// addMacro creates a new Macro object from a macro definition statement
// and adds it to the specified environment.
func addMacro(stmt ast.Statement, env *object.Environment) {
	varDecl, _ := stmt.(*ast.VarDeclStatement)
	macroLiteral, _ := varDecl.Value.(*ast.MacroLiteral)

	macro := &object.Macro{
		Parameters: macroLiteral.Parameters,
		Env:        env,
		Body:       macroLiteral.Body,
	}

	env.Set(varDecl.Name.Value, macro)
}

// ExpandMacros traverses the AST and replaces any macro call expressions with
// their expanded AST nodes. It uses a modifier function that identifies macro
// calls, evaluates the macro's body with the provided arguments, and substitutes
// the call site with the resulting quoted AST node.
func ExpandMacros(program ast.Node, env *object.Environment) ast.Node {
	return ast.Modify(program, func(node ast.Node) ast.Node {
		callExpression, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}

		macro, ok := isMacroCall(callExpression, env)
		if !ok {
			return node
		}

		args := quoteArgs(callExpression)
		evalEnv := extendMacroEnv(macro, args)

		evaluated := Eval(macro.Body, evalEnv)

		// If the macro body fails to evaluate (e.g., due to a parsing error in the
		// macro definition), we cannot expand it. Return the original macro call
		// node. The test will then fail cleanly on the string comparison.
		if isError(evaluated) {
			return node
		}

		quote, ok := evaluated.(*object.Quote)
		if !ok {
			panic("we only support returning AST-nodes from macros")
		}

		return quote.Node
	})
}

// isMacroCall checks if a given call expression is an invocation of a macro
// defined in the environment. It returns the macro object if found.
func isMacroCall(
	exp *ast.CallExpression,
	env *object.Environment,
) (*object.Macro, bool) {
	identifier, ok := exp.Function.(*ast.Identifier)
	if !ok {
		return nil, false
	}

	obj, ok := env.Get(identifier.Value)
	if !ok {
		return nil, false
	}

	macro, ok := obj.(*object.Macro)
	if !ok {
		return nil, false
	}

	return macro, true
}

// quoteArgs takes the arguments of a macro call expression and wraps each one
// in a Quote object. This prevents the arguments from being evaluated before
// they are passed into the macro's expansion environment.
func quoteArgs(exp *ast.CallExpression) []*object.Quote {
	args := []*object.Quote{}

	for _, a := range exp.Arguments {
		args = append(args, &object.Quote{Node: a})
	}

	return args
}

// extendMacroEnv creates a new, enclosed environment for a macro's execution.
// It binds the macro's parameters to the quoted arguments from the call site,
// making them available within the macro's body.
func extendMacroEnv(
	macro *object.Macro,
	args []*object.Quote,
) *object.Environment {
	extended := object.NewEnclosedEnvironment(macro.Env)

	for paramIdx, param := range macro.Parameters {
		extended.Set(param.Value, args[paramIdx])
	}

	return extended
}
