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

// This file transpiles a CONFIGURATION into the main function of a
// royaljelly application: a core.Configuration of Resources, whose Tasks run
// the program instances, started with Configuration.Run until the process is
// interrupted.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// transpileConfigurationDeclaration transpiles a CONFIGURATION into main.
func (t *Transpiler) transpileConfigurationDeclaration(config *ast.ConfigurationDeclaration) error {
	if t.mainGenerated {
		return fmt.Errorf("configuration '%s': a program has only one CONFIGURATION", config.Name.Value)
	}
	t.mainGenerated = true

	// Match each VAR_CONFIG entry to the program instance it configures.
	configEntries, unmatched := config.ResolveConfigVars()
	if len(unmatched) > 0 {
		return fmt.Errorf("VAR_CONFIG path '%s' does not name a variable of a program instance in configuration '%s'", unmatched[0].AccessPath.String(), config.Name.Value)
	}

	// The configuration's globals, which its programs and tasks share.
	if err := t.transpileGlobalVarBlocks(config.GlobalVars); err != nil {
		return err
	}

	t.write("// main runs configuration %s until the process is interrupted.\n", config.Name.Value)
	t.write("func main() {\n")

	// Register the program factories, so the configuration can also be
	// loaded from a file with config.LoadConfigurationFromFile.
	programTypes := []string{}
	seen := map[string]bool{}
	for _, res := range config.Resources {
		for _, progConfig := range res.Programs {
			if !seen[progConfig.TypeName.Value] {
				seen[progConfig.TypeName.Value] = true
				programTypes = append(programTypes, progConfig.TypeName.Value)
			}
		}
	}
	sort.Strings(programTypes)
	for _, progType := range programTypes {
		t.write("\tconfig.RegisterProgramFactory(%q, New%sFactory)\n", progType, progType)
	}

	t.write("\tcfg := &core.Configuration{Name: %q}\n", config.Name.Value)
	for i, res := range config.Resources {
		if err := t.transpileResourceDeclaration(res, fmt.Sprintf("res%d", i+1), configEntries); err != nil {
			return err
		}
	}
	t.write("\tctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)\n")
	t.write("\tdefer stop()\n")
	t.write("\tif err := cfg.Run(ctx); err != nil {\n\t\tlog.Fatal(err)\n\t}\n")
	t.write("}\n\n")
	return nil
}

// transpileResourceDeclaration writes the statements that build a RESOURCE,
// named res in the generated code, and add it to cfg.
func (t *Transpiler) transpileResourceDeclaration(res *ast.ResourceDeclaration, resVar string, configEntries []*ast.ConfigVarEntry) error {
	t.write("\n\t// RESOURCE %s\n", res.Name.Value)
	t.write("\t%s := &core.Resource{Name: %q}\n", resVar, res.Name.Value)

	// royaljelly needs a different priority for each task of a resource, as
	// IEC 61131-3 does not. Tasks keep their order: a task's priority is its
	// rank by IEC priority, then by declaration.
	priorities := make([]int64, len(res.Tasks))
	for i, task := range res.Tasks {
		if task.Priority != nil {
			p, ok := constantInteger(task.Priority)
			if !ok || p < 0 {
				return fmt.Errorf("task '%s': PRIORITY must be a non-negative constant, got %s", task.Name.Value, task.Priority.String())
			}
			priorities[i] = p
		}
	}
	order := make([]int, len(res.Tasks))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return priorities[order[a]] < priorities[order[b]] })
	ranks := make([]int, len(res.Tasks))
	for rank, i := range order {
		ranks[i] = rank
	}

	// Tasks, by name.
	taskVars := map[string]string{}
	for i, task := range res.Tasks {
		taskVar := fmt.Sprintf("%s_task%d", resVar, i+1)
		taskVars[strings.ToUpper(task.Name.Value)] = taskVar
		if err := t.transpileTaskDeclaration(task, taskVar, resVar, ranks[i], i+1); err != nil {
			return err
		}
	}
	maxPriority := len(res.Tasks) - 1

	// A program instance runs in its task; one without a task runs in a
	// background task of the lowest priority, as often as it can.
	background := ""
	for i, progConfig := range res.Programs {
		taskVar := ""
		if progConfig.TaskName != nil {
			var ok bool
			if taskVar, ok = taskVars[strings.ToUpper(progConfig.TaskName.Value)]; !ok {
				return fmt.Errorf("program '%s' of resource '%s' names task '%s', which the resource does not declare", progConfig.InstanceName.Value, res.Name.Value, progConfig.TaskName.Value)
			}
		} else {
			if background == "" {
				background = resVar + "_background"
				t.write("\t%s := core.NewTask(%q, core.CyclicTask, %d, 2*core.DefaultCycle)\n", background, "BACKGROUND", maxPriority+1)
				t.write("\t%s.AddTask(%s)\n", resVar, background)
			}
			taskVar = background
		}

		params := []string{}
		for _, entry := range configEntries {
			if entry.Resource == res && entry.Program == progConfig && entry.Decl.Value != nil {
				value, err := configValueString(entry.Decl.Value)
				if err != nil {
					return fmt.Errorf("VAR_CONFIG %s.%s: %w", progConfig.InstanceName.Value, entry.RelativePath(), err)
				}
				params = append(params, fmt.Sprintf("%q: %q", entry.RelativePath(), value))
			}
		}
		logicVar := fmt.Sprintf("%s_prog%d", resVar, i+1)
		t.write("\t%s, err := New%sFactory(map[string]string{%s})\n", logicVar, progConfig.TypeName.Value, strings.Join(params, ", "))
		t.write("\tif err != nil {\n\t\tlog.Fatalf(\"program %s: %%v\", err)\n\t}\n", progConfig.InstanceName.Value)
		t.write("\t%s.AddProgram(&core.Program{Name: %q, Logic: %s})\n", taskVar, progConfig.InstanceName.Value, logicVar)
	}
	t.write("\tcfg.AddResource(%s)\n", resVar)
	return nil
}

// transpileTaskDeclaration writes the statements that create a TASK, named
// taskVar, with the given royaljelly priority, and add it to its resource.
//
// A task with an INTERVAL is cyclic. A task with SINGLE is event-driven: a
// watcher task of a higher priority than any task's triggers it on each
// rising edge of the SINGLE variable. A task with neither runs every other
// scan of its resource.
func (t *Transpiler) transpileTaskDeclaration(task *ast.TaskDeclaration, taskVar, resVar string, priority, n int) error {
	switch {
	case task.Single != nil:
		t.write("\t%s := core.NewTask(%q, core.EventDrivenTask, %d, 0)\n", taskVar, task.Name.Value, priority)
		single, err := t.expressionString(task.Single)
		if err != nil {
			return err
		}
		// Watchers run first, with priorities below any task's.
		t.write("\tvar %s_single iec.BOOL // SINGLE := %s\n", taskVar, task.Single.String())
		t.write("\t%s.AddTask(core.NewTask(%q, core.CyclicTask, %d, 2*core.DefaultCycle).WithProgram(&core.Program{Name: %q, Logic: func(time.Time) {\n", resVar, task.Name.Value+"_SINGLE", -n, task.Name.Value+"_SINGLE")
		t.write("\t\tif %s && !%s_single {\n\t\t\t%s.Trigger()\n\t\t}\n", single, taskVar, taskVar)
		t.write("\t\t%s_single = %s\n\t}}))\n", taskVar, single)
	case task.Interval != nil:
		interval, err := t.expressionString(task.Interval)
		if err != nil {
			return err
		}
		t.write("\t%s := core.NewTask(%q, core.CyclicTask, %d, time.Duration(%s))\n", taskVar, task.Name.Value, priority, interval)
	default:
		t.write("\t%s := core.NewTask(%q, core.CyclicTask, %d, 2*core.DefaultCycle)\n", taskVar, task.Name.Value, priority)
	}
	t.write("\t%s.AddTask(%s)\n", resVar, taskVar)
	return nil
}

// configValueString returns the text a VAR_CONFIG value is passed to a
// program factory as.
func configValueString(value ast.Expression) (string, error) {
	switch v := value.(type) {
	case *ast.StringLiteral:
		return v.Value, nil
	case *ast.WStringLiteral:
		return v.Value, nil
	case *ast.IntegerLiteral, *ast.RealLiteral, *ast.Boolean:
		return value.String(), nil
	case *ast.PrefixExpression:
		if v.Operator == "-" {
			s, err := configValueString(v.Right)
			return "-" + s, err
		}
	case *ast.TypedLiteral:
		return configValueString(v.Value)
	}
	return "", fmt.Errorf("the value %s must be a literal number, BOOL or string", value.String())
}

// collectConfiguredVars records, for each program type, the variables that
// VAR_CONFIG sets on its instances, so that its factory applies them.
func (t *Transpiler) collectConfiguredVars(program *ast.Program) {
	t.configuredVars = map[string][]string{}
	for _, stmt := range program.Statements {
		cd, ok := stmt.(*ast.ConfigurationDeclaration)
		if !ok {
			continue
		}
		entries, _ := cd.ResolveConfigVars()
		for _, entry := range entries {
			if entry.Decl.Value == nil {
				continue
			}
			progType := strings.ToUpper(entry.Program.TypeName.Value)
			path := entry.RelativePath()
			found := false
			for _, p := range t.configuredVars[progType] {
				found = found || strings.EqualFold(p, path)
			}
			if !found {
				t.configuredVars[progType] = append(t.configuredVars[progType], path)
			}
		}
	}
}

// transpileConfiguredVars writes, in a program's factory, the statements that
// set the variables VAR_CONFIG gives values, from the factory's params.
func (t *Transpiler) transpileConfiguredVars(prog *ast.ProgramDeclaration) error {
	for _, path := range t.configuredVars[strings.ToUpper(prog.Name.Value)] {
		parts := strings.Split(path, ".")
		var target ast.Expression = &ast.Identifier{Value: parts[0]}
		for _, part := range parts[1:] {
			target = &ast.MemberAccessExpression{Struct: target, Member: &ast.Identifier{Value: part}}
		}
		goType := t.elementaryGoType(target)
		field := "instance." + path
		switch {
		case goType == "iec.BOOL":
			t.write("\tif s, ok := params[%q]; ok {\n\t\t%s = iec.BOOL(strings.EqualFold(s, \"TRUE\") || s == \"1\")\n\t}\n", path, field)
		case goType == "iec.STRING" || goType == "iec.WSTRING":
			t.write("\tif s, ok := params[%q]; ok {\n\t\t%s = %s(s)\n\t}\n", path, field, goType)
		case isRealGoType(goType):
			t.write("\tif _, ok := params[%q]; ok {\n\t\tvar v iec.REAL\n", path)
			t.write("\t\tif err := config.ParseREAL(params, %q, &v); err != nil {\n\t\t\treturn nil, err\n\t\t}\n", path)
			t.write("\t\t%s = %s(v)\n\t}\n", field, goType)
		case isNumericGoType(goType) || goType == "iec.BYTE" || goType == "iec.WORD" || goType == "iec.DWORD" || goType == "iec.LWORD":
			t.write("\tif _, ok := params[%q]; ok {\n\t\tvar v iec.LINT\n", path)
			t.write("\t\tif err := config.ParseLINT(params, %q, &v); err != nil {\n\t\t\treturn nil, err\n\t\t}\n", path)
			t.write("\t\t%s = %s(v)\n\t}\n", field, goType)
		default:
			return fmt.Errorf("VAR_CONFIG cannot set %s.%s: it must be a BOOL, number or string variable", prog.Name.Value, path)
		}
	}
	return nil
}
