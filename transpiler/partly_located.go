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

// A partly located variable, x AT %I* : INT, gets its address from its
// configuration's VAR_CONFIG (IEC 61131-3, Table 16 and Table 62, 11a):
//
//	VAR_CONFIG R1.P1.x AT %IW10 : INT; END_VAR
//
// Go has one struct for a program type, which reads its located variables
// from fixed entries of the process image, so the address is the program
// type's: each instance of the type must be given the same one. A variable
// no VAR_CONFIG locates is an ordinary variable, as on the other engines.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// isPartlyLocated reports whether an address leaves its place to VAR_CONFIG
// (%I*, %Q*, %M*).
func isPartlyLocated(address string) bool { return strings.Contains(address, "*") }

// resolvePartlyLocated gives each partly located variable of a program the
// address the configurations give it, in place.
func resolvePartlyLocated(program *ast.Program) error {
	programs := map[string]*ast.ProgramDeclaration{}
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok && pd.Name != nil {
			programs[strings.ToUpper(pd.Name.Value)] = pd
		}
	}
	chosen := map[*ast.VarDeclStatement]string{}
	for _, s := range program.Statements {
		config, ok := s.(*ast.ConfigurationDeclaration)
		if !ok {
			continue
		}
		entries, _ := config.ResolveConfigVars()
		for _, entry := range entries {
			if entry.Decl == nil || entry.Decl.Location == nil || entry.Decl.Location.Location == nil || len(entry.Path) != 1 || entry.Program.TypeName == nil {
				continue
			}
			pd := programs[strings.ToUpper(entry.Program.TypeName.Value)]
			if pd == nil {
				continue
			}
			decl := partlyLocatedVar(pd, entry.Path[0])
			if decl == nil {
				continue
			}
			address := entry.Decl.Location.Location.Address
			if previous, ok := chosen[decl]; ok && !strings.EqualFold(previous, address) {
				return fmt.Errorf("%s.%s is at %%%s and at %%%s: Go gives every instance of a program type the same address", pd.Name.Value, decl.Name.Value, previous, address)
			}
			chosen[decl] = address
			located := *decl.Location
			located.Location = &ast.DirectVariable{Token: entry.Decl.Location.Location.Token, Address: address}
			decl.Location = &located
		}
	}
	return nil
}

// partlyLocatedVar returns the variable of a program called name that is
// partly located, or nil.
func partlyLocatedVar(pd *ast.ProgramDeclaration, name string) *ast.VarDeclStatement {
	for _, block := range [][]*ast.VarDeclStatement{pd.Vars, pd.VarInputs, pd.VarOutputs} {
		for _, d := range block {
			if d.Name != nil && strings.EqualFold(d.Name.Value, name) && d.Location != nil && d.Location.Location != nil && isPartlyLocated(d.Location.Location.Address) {
				return d
			}
		}
	}
	return nil
}
