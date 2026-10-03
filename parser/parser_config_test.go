package parser

import (
	"fmt"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"testing"
)

func TestConfigurationDeclaration(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			VAR_GLOBAL
				Global1 : BOOL;
			END_VAR

			RESOURCE Res1 ON PLC1
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
				PROGRAM Prog1 WITH Task1 : ProgType1;
			END_RESOURCE

			VAR_CONFIG Prog1
				Input1 : INT := 42;
			END_VAR
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionWithMultipleVarBlocks", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyConfig" {
		t.Errorf("Configuration name is not 'MyConfig'. got=%s", stmt.Name.Value)
	}

	if len(stmt.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(stmt.Resources))
	}

	if len(stmt.VarConfigs) != 1 {
		t.Fatalf("Expected 1 VAR_CONFIG block. got=%d", len(stmt.VarConfigs))
	}

	cfgVar := stmt.VarConfigs[0]
	if cfgVar.ProgramInstanceName.Value != "Prog1" {
		t.Errorf("VAR_CONFIG instance name is not 'Prog1'. got=%s", cfgVar.ProgramInstanceName.Value)
	}
}

func TestResourceDeclarationErrorRecovery(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				MyBareIdentifier; (* This is an invalid token in this context *)
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	// 1. Check that the specific error for the unexpected identifier was reported.
	expectedError := "unexpected identifier 'MyBareIdentifier' in resource block, use PROGRAM keyword for instantiation"
	assertErrorContains(t, p.Errors(), expectedError)

	// 2. Check that the parser recovered and continued to parse the configuration.
	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements should contain 1 statement after recovery. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource to be parsed after recovery. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	// 3. Verify that the TASK declaration *after* the invalid token was parsed successfully.
	if len(resource.Tasks) != 1 {
		t.Fatalf("Expected 1 task to be parsed after recovery. got=%d", len(resource.Tasks))
	}
	if resource.Tasks[0].Name.Value != "Task1" {
		t.Errorf("Expected task name 'Task1', got %s", resource.Tasks[0].Name.Value)
	}
}

func TestResourceWithGlobalVar(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				VAR_GLOBAL
					GlobalInResource : BOOL;
				END_VAR
				(* This is a comment before the task *)
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestResourceWithGlobalVar", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	// 1. Verify the VAR_GLOBAL block was parsed and attached to the resource.
	if len(resource.GlobalVars) != 1 {
		t.Fatalf("Expected 1 VAR_GLOBAL block in the resource. got=%d", len(resource.GlobalVars))
	}
	globalBlock := resource.GlobalVars[0]
	if len(globalBlock.Vars) != 1 {
		t.Fatalf("Expected 1 variable in the VAR_GLOBAL block. got=%d", len(globalBlock.Vars))
	}
	testVarDeclStatement(t, globalBlock.Vars[0], "GlobalInResource", "BOOL")

	// 2. Verify the parser continued and parsed the TASK declaration after the VAR_GLOBAL block.
	if len(resource.Tasks) != 1 {
		t.Fatalf("Expected 1 task to be parsed after the VAR_GLOBAL block. got=%d", len(resource.Tasks))
	}
}

func TestResourceDeclarationWithComments(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				(* This is a comment before the task *)
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
				// This is another comment between task and program
				PROGRAM Prog1 WITH Task1 : ProgType1;
				(* And one at the end before END_RESOURCE *)
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestResourceDeclarationWithComments", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	// Check that the parser correctly skipped the comments and parsed both the TASK and PROGRAM.
	if len(resource.Tasks) != 1 {
		t.Fatalf("Expected 1 task to be parsed. got=%d", len(resource.Tasks))
	}
	if resource.Tasks[0].Name.Value != "Task1" {
		t.Errorf("Expected task name 'Task1', got %s", resource.Tasks[0].Name.Value)
	}

	if len(resource.Programs) != 1 {
		t.Fatalf("Expected 1 program to be parsed. got=%d", len(resource.Programs))
	}
	if resource.Programs[0].InstanceName.Value != "Prog1" {
		t.Errorf("Expected program instance name 'Prog1', got %s", resource.Programs[0].InstanceName.Value)
	}
}

func TestProgramConfigurationWithParameters(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
				PROGRAM Prog1 WITH Task1 : ProgType1(Input1 := 42, Out1 => GlobalVar);
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramConfigurationWithParameters", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	if len(resource.Programs) != 1 {
		t.Fatalf("Expected 1 program configuration. got=%d", len(resource.Programs))
	}
	progConfig := resource.Programs[0]

	if progConfig.InstanceName.Value != "Prog1" {
		t.Errorf("Program instance name is not 'Prog1'. got=%s", progConfig.InstanceName.Value)
	}

	if len(progConfig.Parameters) != 2 {
		t.Fatalf("Expected 2 parameters in program configuration. got=%d", len(progConfig.Parameters))
	}

	// Check first parameter: Input1 := 42
	namedArg, ok := progConfig.Parameters[0].(*ast.NamedArgument)
	if !ok {
		t.Fatalf("First parameter is not a NamedArgument. got=%T", progConfig.Parameters[0])
	}
	testVarDeclStatement(t, &ast.VarDeclStatement{Token: namedArg.Name.Token, Name: namedArg.Name, Value: namedArg.Value}, "Input1", "")

	// Check second parameter: Out1 => GlobalVar
	outputArg, ok := progConfig.Parameters[1].(*ast.OutputArgument)
	if !ok {
		t.Fatalf("Second parameter is not an OutputArgument. got=%T", progConfig.Parameters[1])
	}
	if outputArg.Source.Value != "Out1" {
		t.Errorf("Output argument source is not 'Out1'. got=%s", outputArg.Source.Value)
	}
	testIdentifier(t, outputArg.Target, "GlobalVar")
}

func TestProgramConfigurationWithRetain(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				TASK Task1 (INTERVAL := T#100ms, PRIORITY := 1);
				PROGRAM RETAIN Prog1 WITH Task1 : ProgType1;
				PROGRAM NON_RETAIN Prog2 WITH Task1 : ProgType2;
				PROGRAM Prog3 WITH Task1 : ProgType3;
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramConfigurationWithRetain", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	if len(resource.Programs) != 3 {
		t.Fatalf("Expected 3 program configurations, got %d", len(resource.Programs))
	}

	prog1 := resource.Programs[0]
	if !prog1.IsRetain {
		t.Errorf("Expected Prog1 to have IsRetain = true")
	}

	prog2 := resource.Programs[1]
	if !prog2.IsNonRetain {
		t.Errorf("Expected Prog2 to have IsNonRetain = true")
	}

	prog3 := resource.Programs[2]
	if prog3.IsRetain || prog3.IsNonRetain {
		t.Errorf("Expected Prog3 to have no retain flags set")
	}
}

func TestProgramConfigurationWithFbTask(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			RESOURCE Res1 ON PLC1
				TASK TaskA (PRIORITY := 1);
				TASK TaskB (PRIORITY := 2);
				PROGRAM MyProg : ProgType(FB1 WITH TaskA, In1 := TRUE, FB2 WITH TaskB);
			END_RESOURCE
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramConfigurationWithFbTask", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.Resources) != 1 {
		t.Fatalf("Expected 1 resource. got=%d", len(config.Resources))
	}
	resource := config.Resources[0]

	if len(resource.Programs) != 1 {
		t.Fatalf("Expected 1 program configuration. got=%d", len(resource.Programs))
	}
	progConfig := resource.Programs[0]

	if len(progConfig.FbTasks) != 2 {
		t.Fatalf("Expected 2 FB-Task associations, got %d", len(progConfig.FbTasks))
	}

	// Check first fb_task
	if progConfig.FbTasks[0].FbName.Value != "FB1" || progConfig.FbTasks[0].TaskName.Value != "TaskA" {
		t.Errorf("Incorrect first FB-Task association parsed. want='FB1 WITH TaskA', got=%q", progConfig.FbTasks[0].String())
	}

	// Check second fb_task
	if progConfig.FbTasks[1].FbName.Value != "FB2" || progConfig.FbTasks[1].TaskName.Value != "TaskB" {
		t.Errorf("Incorrect second FB-Task association parsed. want='FB2 WITH TaskB', got=%q", progConfig.FbTasks[1].String())
	}

	// Check the standard parameter
	if len(progConfig.Parameters) != 1 {
		t.Fatalf("Expected 1 standard parameter (prog_cnxn), got %d", len(progConfig.Parameters))
	}
	if _, ok := progConfig.Parameters[0].(*ast.NamedArgument); !ok {
		t.Errorf("Failed to parse named argument alongside fb_task. got=%T", progConfig.Parameters[0])
	}
}

func TestConfigurationWithVarAccess(t *testing.T) {
	input := `
		CONFIGURATION MyConfig
			VAR_ACCESS
				RemoteVar : OtherProgram.Var READ_ONLY;
			END_VAR
		END_CONFIGURATION
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestConfigurationWithVarAccess", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
	}

	if len(config.AccessVars) != 1 {
		t.Fatalf("Expected 1 VAR_ACCESS block. got=%d", len(config.AccessVars))
	}

	accessBlock := config.AccessVars[0]
	if len(accessBlock.Vars) != 1 {
		t.Fatalf("Expected 1 declaration in VAR_ACCESS block. got=%d", len(accessBlock.Vars))
	}

	decl := accessBlock.Vars[0]
	if decl.Name.Value != "RemoteVar" {
		t.Errorf("decl.Name.Value not 'RemoteVar'. got=%s", decl.Name.Value)
	}
	if decl.AccessPath.String() != "OtherProgram.Var" {
		t.Errorf("decl.AccessPath not 'OtherProgram.Var'. got=%s", decl.AccessPath.String())
	}
	if decl.AccessType != "READ_ONLY" {
		t.Errorf("decl.AccessType not 'READ_ONLY'. got=%s", decl.AccessType)
	}
}

func TestTaskDeclarationParsing(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		check         func(t *testing.T, stmt *ast.TaskDeclaration)
	}{
		{
			name:  "Valid task with all parameters",
			input: `TASK T1 (SINGLE := TRUE, INTERVAL := T#1s, PRIORITY := 1);`,
			check: func(t *testing.T, stmt *ast.TaskDeclaration) {
				if stmt.Name.Value != "T1" {
					t.Errorf("Expected task name 'T1', got %s", stmt.Name.Value)
				}
				if stmt.Single == nil {
					t.Error("Expected SINGLE parameter to be parsed")
				}
				if stmt.Interval == nil {
					t.Error("Expected INTERVAL parameter to be parsed")
				}
				if stmt.Priority == nil {
					t.Error("Expected PRIORITY parameter to be parsed")
				}
			},
		},
		{
			name:  "Task with empty parameters",
			input: `TASK T2 ();`,
			check: func(t *testing.T, stmt *ast.TaskDeclaration) {
				if stmt.Name.Value != "T2" {
					t.Errorf("Expected task name 'T2', got %s", stmt.Name.Value)
				}
				if stmt.Single != nil || stmt.Interval != nil || stmt.Priority != nil {
					t.Error("Expected no parameters to be parsed")
				}
			},
		},
		{
			name:          "Error on missing task name",
			input:         `TASK (PRIORITY := 1);`,
			expectedError: "expected next token to be IDENT, got ( instead",
		},
		{
			name:          "Error on missing left parenthesis",
			input:         `TASK T1 PRIORITY := 1);`,
			expectedError: "expected next token to be (, got PRIORITY instead",
		},
		{
			name:          "Error on missing assignment in SINGLE",
			input:         `TASK T1 (SINGLE TRUE);`,
			expectedError: "expected next token to be :=, got TRUE instead",
		},
		{
			name:          "Error on missing assignment in PRIORITY",
			input:         `TASK T1 (PRIORITY 1);`,
			expectedError: "expected next token to be :=, got INT instead",
		},
		{
			name:          "Error on unexpected token in parameters",
			input:         `TASK T1 (VAR);`,
			expectedError: "unexpected token in task configuration: VAR",
		},
		{
			name:          "Error on missing comma between parameters",
			input:         `TASK T1 (PRIORITY := 1 INTERVAL := T#1s);`,
			expectedError: "expected ',' or ')' in task configuration, got INTERVAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TASK declarations are only valid inside a RESOURCE block.
			// We wrap the input to create a valid program for the parser.
			fullInput := fmt.Sprintf("CONFIGURATION Cfg\nRESOURCE Res ON PLC\n%s\nEND_RESOURCE\nEND_CONFIGURATION", tt.input)
			l := lexer.New(fullInput)
			p := New(l)
			program := p.ParseProgram()

			if tt.expectedError != "" {
				assertErrorContains(t, p.Errors(), tt.expectedError)
				return // Don't check the AST if an error was expected
			}

			checkParserErrors(t, p, tt.name, fullInput)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
			}

			config, ok := program.Statements[0].(*ast.ConfigurationDeclaration)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ConfigurationDeclaration. got=%T", program.Statements[0])
			}
			if len(config.Resources) != 1 {
				t.Fatalf("Expected 1 resource, got %d", len(config.Resources))
			}
			resource := config.Resources[0]

			if len(resource.Tasks) != 1 {
				t.Fatalf("Expected 1 task, got %d", len(resource.Tasks))
			}
			stmt := resource.Tasks[0]

			if tt.check != nil {
				tt.check(t, stmt)
			}
		})
	}
}

func TestParseVarConfigComplex(t *testing.T) {
	input := `
		VAR_CONFIG
			STATION_1.P1.COUNT : INT := 1;
			STATION_1.P1.TIME1 : TON := (PT := T#2.5s);
			STATION_2.P4.FB1.C2 AT %QB25 : BYTE;
		END_VAR
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram() // cspell:disable-line
	checkParserErrors(t, p, "TestParseVarConfigComplex", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ConfigVarDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ConfigVarDeclaration. got=%T", program.Statements[0])
	}

	if len(stmt.Declarations) != 3 {
		t.Fatalf("Expected 3 declarations in VAR_CONFIG block. got=%d", len(stmt.Declarations))
	}

	// --- Check Declaration 1: STATION_1.P1.COUNT : INT := 1; ---
	decl1 := stmt.Declarations[0]
	if decl1.AccessPath.String() != "STATION_1.P1.COUNT" {
		t.Errorf("decl1.AccessPath not 'STATION_1.P1.COUNT'. got=%s", decl1.AccessPath.String())
	}
	if decl1.DataType.String() != "INT" {
		t.Errorf("decl1.DataType not 'INT'. got=%s", decl1.DataType.String())
	}
	if !testIntegerLiteral(t, decl1.Value, 1) {
		t.Errorf("decl1.Value is not 1.")
	}
	if decl1.Location != nil {
		t.Errorf("decl1.Location should be nil.")
	}

	// --- Check Declaration 2: STATION_1.P1.TIME1 : TON := (PT := T#2.5s); ---
	decl2 := stmt.Declarations[1]
	if decl2.AccessPath.String() != "STATION_1.P1.TIME1" {
		t.Errorf("decl2.AccessPath not 'STATION_1.P1.TIME1'. got=%s", decl2.AccessPath.String())
	}
	if decl2.DataType.String() != "TON" {
		t.Errorf("decl2.DataType not 'TON'. got=%s", decl2.DataType.String())
	}
	structLit, ok := decl2.Value.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("decl2.Value is not a StructLiteral. got=%T", decl2.Value)
	}
	if len(structLit.Initializers) != 1 {
		t.Fatalf("Expected 1 initializer in struct literal, got %d", len(structLit.Initializers))
	}
	namedArg, ok := structLit.Initializers[0].(*ast.NamedArgument)
	if !ok {
		t.Fatalf("Initializer is not a NamedArgument. got=%T", structLit.Initializers[0])
	}
	if namedArg.Name.Value != "PT" {
		t.Errorf("Argument name not 'PT'. got=%s", namedArg.Name.Value)
	}
	timeLit, ok := namedArg.Value.(*ast.TimeLiteral)
	if !ok {
		t.Fatalf("exp not *ast.TimeLiteral. got=%T", namedArg.Value)
	}
	if timeLit.Value != "2.5s" {
		t.Errorf("Argument value is not '2.5s'. got=%q", timeLit.Value)
	}
	if decl2.Location != nil {
		t.Errorf("decl2.Location should be nil.")
	}

	// --- Check Declaration 3: STATION_2.P4.FB1.C2 AT %QB25 : BYTE; ---
	decl3 := stmt.Declarations[2]
	if decl3.AccessPath.String() != "STATION_2.P4.FB1.C2" {
		t.Errorf("decl3.AccessPath not 'STATION_2.P4.FB1.C2'. got=%s", decl3.AccessPath.String())
	}
	if decl3.DataType.String() != "BYTE" {
		t.Errorf("decl3.DataType not 'BYTE'. got=%s", decl3.DataType.String())
	}
	if decl3.Value != nil {
		t.Errorf("decl3.Value should be nil.")
	}
	if decl3.Location == nil {
		t.Fatalf("decl3.Location should not be nil.")
	}
	if decl3.Location.Location.Address != "QB25" {
		t.Errorf("decl3.Location address not 'QB25'. got=%s", decl3.Location.Location.Address)
	}
}
