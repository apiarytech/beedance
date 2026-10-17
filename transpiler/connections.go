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

// A configuration connects a program instance's inputs and outputs to its
// globals (IEC 61131-3, Table 62, 8a to 9b):
//
//	PROGRAM P1 WITH T1 : MAIN(k := g, out => cg);
//
// Each run of the instance reads its connected inputs first, and writes its
// connected outputs after: main wraps the instance's Logic so.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// markConnectedPrograms records the program types the configurations of
// program connect (t.connectedPrograms), which get new<Program>.
func (t *Transpiler) markConnectedPrograms(program *ast.Program) {
	t.connectedPrograms = map[string]bool{}
	t.programDecls = map[string]*ast.ProgramDeclaration{}
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok && pd.Name != nil {
			t.programDecls[strings.ToUpper(pd.Name.Value)] = pd
		}
		config, ok := s.(*ast.ConfigurationDeclaration)
		if !ok {
			continue
		}
		for _, res := range config.Resources {
			for _, p := range res.Programs {
				if len(p.Parameters) > 0 && p.TypeName != nil {
					t.connectedPrograms[strings.ToUpper(p.TypeName.Value)] = true
				}
			}
		}
	}
}

// transpileConnectedProgram writes the instance of a program configuration
// that connects its inputs and outputs, and logicVar, its Logic wrapped to
// make the connections.
func (t *Transpiler) transpileConnectedProgram(progConfig *ast.ProgramConfiguration, logicVar string, params []string, res *ast.ResourceDeclaration) error {
	pd := t.programDecls[strings.ToUpper(progConfig.TypeName.Value)]
	if pd == nil {
		return fmt.Errorf("program '%s': no PROGRAM %s", progConfig.InstanceName.Value, progConfig.TypeName.Value)
	}
	instVar := logicVar + "_instance"
	t.write("\t%s, err := new%s(map[string]string{%s})\n", instVar, pd.Name.Value, strings.Join(params, ", "))
	t.write("\tif err != nil {\n\t\tlog.Fatalf(\"program %s: %%v\", err)\n\t}\n", progConfig.InstanceName.Value)
	var before, after []string
	for _, param := range progConfig.Parameters {
		switch p := param.(type) {
		case *ast.NamedArgument:
			decl := findVarDecl(p.Name.Value, pd.VarInputs, pd.VarInOuts)
			if decl == nil {
				return fmt.Errorf("program '%s': %s has no input '%s'", progConfig.InstanceName.Value, pd.Name.Value, p.Name.Value)
			}
			value, err := t.expressionString(p.Value)
			if err != nil {
				return err
			}
			before = append(before, fmt.Sprintf("%s.%s = %s(%s)", instVar, decl.Name.Value, t.mapIecTypeToGo(decl.DataType), value))
		case *ast.OutputArgument:
			decl := findVarDecl(p.Source.Value, pd.VarOutputs)
			if decl == nil {
				return fmt.Errorf("program '%s': %s has no output '%s'", progConfig.InstanceName.Value, pd.Name.Value, p.Source.Value)
			}
			target, ok := p.Target.(*ast.Identifier)
			if !ok {
				return fmt.Errorf("program '%s': the output %s must connect to a global variable", progConfig.InstanceName.Value, p.Source.Value)
			}
			global := t.configGlobal(res, target.Value)
			if global == nil {
				return fmt.Errorf("program '%s': %s is not a global variable of the configuration", progConfig.InstanceName.Value, target.Value)
			}
			after = append(after, fmt.Sprintf("%s = %s(%s.%s)", global.Name.Value, t.mapIecTypeToGo(global.DataType), instVar, decl.Name.Value))
		}
	}
	t.write("\t%s := func(now time.Time) {\n", logicVar)
	for _, s := range before {
		t.write("\t\t%s\n", s)
	}
	t.write("\t\t%s.Logic(now)\n", instVar)
	for _, s := range after {
		t.write("\t\t%s\n", s)
	}
	t.write("\t}\n")
	return nil
}

// findVarDecl returns the variable called name of blocks, or nil.
func findVarDecl(name string, blocks ...[]*ast.VarDeclStatement) *ast.VarDeclStatement {
	for _, block := range blocks {
		for _, d := range block {
			if d.Name != nil && strings.EqualFold(d.Name.Value, name) {
				return d
			}
		}
	}
	return nil
}

// configGlobal returns the global variable called name of a resource or its
// configuration (t.currentConfig), or nil.
func (t *Transpiler) configGlobal(res *ast.ResourceDeclaration, name string) *ast.VarDeclStatement {
	blocks := append([]*ast.GlobalVarDeclaration{}, res.GlobalVars...)
	if t.currentConfig != nil {
		blocks = append(blocks, t.currentConfig.GlobalVars...)
	}
	for _, b := range blocks {
		if d := findVarDecl(name, b.Vars); d != nil {
			return d
		}
	}
	return nil
}
