/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

// Package enables gives EN and ENO, when a POU declares them itself, their
// meaning for every engine alike (IEC-75). IEC 61131-3 provides EN and ENO
// so (2003 2.5.1.2 and 2.5.2.1a; Ed.3 Table 18):
//
//	VAR_INPUT EN : BOOL := 1; END_VAR
//	VAR_OUTPUT ENO : BOOL; END_VAR
//
// and rules: if EN is FALSE when the POU is invoked, its body is not
// executed and ENO is reset to FALSE; otherwise ENO is set TRUE and the body
// is executed, which may set ENO FALSE. A function block's outputs keep
// their values while it is not executed. Lower writes the rules into the
// body:
//
//	ENO := TRUE;
//	IF EN THEN <body> ELSE ENO := FALSE; END_IF;
//
// so EN and ENO are ordinary variables to the engines: EN an input like any
// other, which keeps its value when a call does not pass it. The engines'
// own EN and ENO, for a POU that does not declare them, are left to them.
// An IL body is left as it is: its instructions cannot sit in an ST IF.
package enables

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
)

// marker is the variable a lowered POU gets, so Lower may run twice.
const marker = "_enables_lowered"

// Lower gives the declared EN and ENO of the program's functions and
// function blocks (in namespaces too) their meaning, in place.
func Lower(program *ast.Program) error {
	if program == nil {
		return nil
	}
	return lowerStatements(program.Statements)
}

func lowerStatements(stmts []ast.Statement) error {
	for _, s := range stmts {
		switch d := s.(type) {
		case *ast.FunctionBlockDeclaration:
			vars, err := lower(d.Name, d.VarInputs, d.VarOutputs, d.Vars, d.Body)
			if err != nil {
				return err
			}
			d.Vars = vars
		case *ast.FunctionDeclaration:
			vars, err := lower(d.Name, d.VarInputs, d.VarOutputs, d.Vars, d.Body)
			if err != nil {
				return err
			}
			d.Vars = vars
		case *ast.NamespaceDeclaration:
			if err := lowerStatements(d.Statements); err != nil {
				return err
			}
		}
	}
	return nil
}

// Declares reports whether a POU declares EN among its inputs or ENO among
// its outputs (any case): then their meaning is this package's, and an
// engine leaves its own EN and ENO handling out for it.
func Declares(inputs, outputs []*ast.VarDeclStatement) (en, eno bool) {
	for _, d := range inputs {
		if d != nil && d.Name != nil && strings.EqualFold(d.Name.Value, "EN") {
			en = true
		}
	}
	for _, d := range outputs {
		if d != nil && d.Name != nil && strings.EqualFold(d.Name.Value, "ENO") {
			eno = true
		}
	}
	return en, eno
}

func lower(name *ast.Identifier, inputs, outputs, vars []*ast.VarDeclStatement, b ast.Statement) ([]*ast.VarDeclStatement, error) {
	en, eno := Declares(inputs, outputs)
	body, ok := b.(*ast.BlockStatement)
	if (!en && !eno) || !ok || body == nil || isIL(body) {
		return vars, nil
	}
	for _, d := range vars {
		if d != nil && d.Name != nil && d.Name.Value == marker {
			return vars, nil // lowered already
		}
	}
	enName, enoName := "EN", "ENO"
	for _, d := range inputs {
		if d != nil && d.Name != nil && strings.EqualFold(d.Name.Value, "EN") {
			enName = d.Name.Value
		}
	}
	for _, d := range outputs {
		if d != nil && d.Name != nil && strings.EqualFold(d.Name.Value, "ENO") {
			enoName = d.Name.Value
		}
	}
	var src strings.Builder
	if eno {
		fmt.Fprintf(&src, "%s := TRUE;\n", enoName)
	}
	if en {
		fmt.Fprintf(&src, "IF %s THEN\n_enables_body := 0;\n", enName)
		if eno {
			fmt.Fprintf(&src, "ELSE %s := FALSE;\n", enoName)
		}
		src.WriteString("END_IF;\n")
	}
	stmts, err := parse(src.String())
	if err != nil {
		return vars, fmt.Errorf("%s: %w", name.Value, err)
	}
	// The body's own statements go in the IF, where the placeholder is;
	// its methods stay outside it.
	var own, methods []ast.Statement
	for _, s := range body.Statements {
		if _, isMethod := s.(*ast.MethodImplementation); isMethod {
			methods = append(methods, s)
			continue
		}
		own = append(own, s)
	}
	if en {
		ifStmt := stmts[len(stmts)-1].(*ast.IfStatement)
		ifStmt.Consequence.Statements = own
		own = nil
	}
	body.Statements = append(append(stmts, own...), methods...)
	markerVars, _, err := parseDecls(marker + " : BOOL;")
	if err != nil {
		return vars, err
	}
	return append(vars, markerVars...), nil
}

func isIL(body *ast.BlockStatement) bool {
	for _, s := range body.Statements {
		if _, ok := s.(*ast.IlInstructionStatement); ok {
			return true
		}
	}
	return false
}

// parse parses statements as a program's body.
func parse(stmts string) ([]ast.Statement, error) {
	_, out, err := parseDecls("_enables_body : INT;", stmts)
	return out, err
}

func parseDecls(decls string, stmts ...string) ([]*ast.VarDeclStatement, []ast.Statement, error) {
	src := "PROGRAM _enables\nVAR " + decls + " END_VAR\n" + strings.Join(stmts, "") + "END_PROGRAM\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, nil, fmt.Errorf("enables: %s", strings.Join(errs, "; "))
	}
	pd, ok := prog.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		return nil, nil, fmt.Errorf("enables: not a program")
	}
	var out []ast.Statement
	if blk, ok := pd.Body.(*ast.BlockStatement); ok {
		out = blk.Statements
	}
	return pd.Vars, out, nil
}
