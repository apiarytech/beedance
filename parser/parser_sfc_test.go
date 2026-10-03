package parser

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/lexer"
	"testing"
)

func TestProgramWithSFCBody(t *testing.T) {
	input := `
		PROGRAM MySFCProgram
			VAR
				cond : BOOL;
				x : INT := 0;
			END_VAR

			ACTION Step1Action:
				x := x + 1;
				cond := TRUE;
			END_ACTION

			ACTION Step2Action:
				x := x * 2;
				cond := FALSE;
			END_ACTION

			INITIAL_STEP S1: Step1Action(); END_STEP

			TRANSITION FROM S1 TO S2 := cond; END_TRANSITION

			STEP S2: Step2Action(); END_STEP
		END_PROGRAM
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestProgramWithSFCBody", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	progDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	if progDecl.Name.Value != "MySFCProgram" {
		t.Fatalf("Program name is not 'MySFCProgram'. got=%s", progDecl.Name.Value)
	}

	sfcBody, ok := progDecl.Body.(*ast.SFCProgram)
	if !ok {
		t.Fatalf("Program body is not ast.SFCProgram. got=%T", progDecl.Body)
	}

	if len(sfcBody.Elements) != 5 {
		t.Fatalf("SFC body does not have 5 elements. got=%d", len(sfcBody.Elements))
	}

	// --- Detailed check of the first element: INITIAL_STEP S1 ---
	initialStep, ok := sfcBody.Elements[2].(*ast.StepStatement)
	if !ok {
		t.Fatalf("Element 2 is not ast.StepStatement. got=%T", sfcBody.Elements[2])
	}
	if !initialStep.IsInitial {
		t.Error("First step should be initial.")
	}
	if initialStep.Name.Value != "S1" {
		t.Errorf("Initial step name is not 'S1'. got=%s", initialStep.Name.Value)
	}

	// --- Detailed check of the second element: TRANSITION ---
	transition, ok := sfcBody.Elements[3].(*ast.TransitionStatement)
	if !ok {
		t.Fatalf("Element 3 is not ast.TransitionStatement. got=%T", sfcBody.Elements[3])
	}
	if len(transition.From) != 1 || transition.From[0].Value != "S1" {
		t.Errorf("Transition 'FROM' is not 'S1'. got=%v", transition.From)
	}
	if len(transition.To) != 1 || transition.To[0].Value != "S2" {
		t.Errorf("Transition 'TO' is not 'S2'. got=%v", transition.To)
	}
	testIdentifier(t, transition.Condition, "cond")

	// --- Detailed check of the third element: STEP S2 ---
	step2, ok := sfcBody.Elements[4].(*ast.StepStatement)
	if !ok {
		t.Fatalf("Element 4 is not ast.StepStatement. got=%T", sfcBody.Elements[4])
	}
	if step2.Name.Value != "S2" {
		t.Errorf("Step name is not 'S2'. got=%s", step2.Name.Value)
	}
}

func TestFunctionBlockWithSFCBody(t *testing.T) {
	input := `
		FUNCTION_BLOCK MySFC_FB
			VAR
				cond : BOOL;
			END_VAR

			INITIAL_STEP S1:
			END_STEP

			TRANSITION FROM S1 TO S2 := cond; END_TRANSITION

			STEP S2:
			END_STEP
		END_FUNCTION_BLOCK
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestFunctionBlockWithSFCBody", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	fb, ok := program.Statements[0].(*ast.FunctionBlockDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionBlockDeclaration. got=%T", program.Statements[0])
	}

	if fb.Name.Value != "MySFC_FB" {
		t.Errorf("Function block name is not 'MySFC_FB'. got=%s", fb.Name.Value)
	}

	sfcBody, ok := fb.Body.(*ast.SFCProgram)
	if !ok {
		t.Fatalf("Function block body is not *ast.SFCProgram. got=%T", fb.Body)
	}

	if len(sfcBody.Elements) != 3 {
		t.Fatalf("SFC body should have 3 elements. got=%d", len(sfcBody.Elements))
	}

	// Check the first element (INITIAL_STEP)
	initialStep, ok := sfcBody.Elements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("SFC element 0 is not *ast.StepStatement. got=%T", sfcBody.Elements[0])
	}
	if !initialStep.IsInitial {
		t.Error("First step should be an initial step.")
	}
	if initialStep.Name.Value != "S1" {
		t.Errorf("Initial step name is not 'S1'. got=%s", initialStep.Name.Value)
	}
}

func TestActionStatement(t *testing.T) {
	input := `
		ACTION MyAction:
			x := x + 1;
		END_ACTION
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestActionStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ActionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ActionStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyAction" {
		t.Fatalf("Action name is not 'MyAction'. got=%s", stmt.Name.Value)
	}

	body, ok := stmt.Body.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("Action body is not a BlockStatement. got=%T", stmt.Body)
	}

	if len(body.Statements) != 1 {
		t.Fatalf("Action body does not have 1 statement. got=%d", len(body.Statements))
	}

	bodyStmt, ok := body.Statements[0].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Body statement is not ast.AssignmentStatement. got=%T", body.Statements[0])
	}

	testIdentifier(t, bodyStmt.Left, "x")
	testInfixExpression(t, 0, bodyStmt.Value, "x", "+", 1)
}

func TestTransitionStatement(t *testing.T) {
	input := `
		TRANSITION FROM Step1, Step2 TO Step3 := Condition1 AND Condition2; END_TRANSITION
	`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestTransitionStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.TransitionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TransitionStatement. got=%T", program.Statements[0])
	}

	if len(stmt.From) != 2 {
		t.Fatalf("Expected 2 'FROM' identifiers. got=%d", len(stmt.From))
	}
	if stmt.From[0].Value != "Step1" || stmt.From[1].Value != "Step2" {
		t.Errorf("Incorrect 'FROM' identifiers. got=%s, %s", stmt.From[0].Value, stmt.From[1].Value)
	}

	if len(stmt.To) != 1 {
		t.Fatalf("Expected 1 'TO' identifier. got=%d", len(stmt.To))
	}
	if stmt.To[0].Value != "Step3" {
		t.Errorf("Incorrect 'TO' identifier. got=%s", stmt.To[0].Value)
	}

	if !testInfixExpression(t, 0, stmt.Condition, "Condition1", "AND", "Condition2") {
		return
	}
}

func TestStepStatement(t *testing.T) {
	input := `
		STEP MyStep:
			Action1(N);
			Action2(P);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStepStatement", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "MyStep" {
		t.Errorf("Step name is not 'MyStep'. got=%s", stmt.Name.Value)
	}

	if stmt.IsInitial {
		t.Errorf("Step should not be initial")
	}

	if len(stmt.Actions) != 2 {
		t.Fatalf("Expected 2 action associations. got=%d", len(stmt.Actions))
	}

	action1 := stmt.Actions[0]
	if action1.ActionName.Value != "Action1" {
		t.Errorf("Incorrect first action name. Expected 'Action1', got %s", action1.ActionName.Value)
	}
	if action1.Qualifier == nil || action1.Qualifier.Value != "N" {
		t.Errorf("Incorrect first action qualifier. Expected 'N', got %v", action1.Qualifier)
	}

	action2 := stmt.Actions[1]
	if action2.ActionName.Value != "Action2" {
		t.Errorf("Incorrect second action name. Expected 'Action2', got %s", action2.ActionName.Value)
	}
	if action2.Qualifier == nil || action2.Qualifier.Value != "P" {
		t.Errorf("Incorrect second action qualifier. Expected 'P', got %v", action2.Qualifier)
	}
}

func TestInitialStepStatement(t *testing.T) {
	input := `
		INITIAL_STEP InitStep:
			InitAction(N);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestIfStatementWithEmptyBlocks", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if stmt.Name.Value != "InitStep" {
		t.Errorf("Step name is not 'InitStep'. got=%s", stmt.Name.Value)
	}

	if !stmt.IsInitial {
		t.Errorf("Step should be initial")
	}

	if len(stmt.Actions) != 1 {
		t.Fatalf("Expected 1 action association in step body. got=%d", len(stmt.Actions))
	}

	action := stmt.Actions[0]
	if action.ActionName.Value != "InitAction" {
		t.Errorf("Incorrect action name. Expected 'InitAction', got %s", action.ActionName.Value)
	}
	if action.Qualifier == nil || action.Qualifier.Value != "N" {
		t.Errorf("Incorrect action qualifier. Expected 'N', got %v", action.Qualifier)
	}
}

func TestStepStatementWithComments(t *testing.T) {
	input := `
		STEP MyStep:
			Action1(N);
			(* This is a multi-line comment *)
			Action2(P);
			// This is a single-line comment
			Action3(S);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStepStatementWithComments", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if len(stmt.Actions) != 3 {
		t.Fatalf("Expected 3 action associations. got=%d", len(stmt.Actions))
	}

	action1 := stmt.Actions[0]
	if action1.ActionName.Value != "Action1" || action1.Qualifier.Value != "N" {
		t.Errorf("Incorrect first action. Expected 'Action1(N)', got %s", action1.String())
	}

	action2 := stmt.Actions[1]
	if action2.ActionName.Value != "Action2" || action2.Qualifier.Value != "P" {
		t.Errorf("Incorrect second action. Expected 'Action2(P)', got %s", action2.String())
	}

	action3 := stmt.Actions[2]
	if action3.ActionName.Value != "Action3" || action3.Qualifier.Value != "S" {
		t.Errorf("Incorrect third action. Expected 'Action3(S)', got %s", action3.String())
	}
}

func TestStepActionWithDuration(t *testing.T) {
	input := `
		STEP MyStep:
			MyAction(L, T#5s);
		END_STEP
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestStepActionWithDuration", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.StepStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.StepStatement. got=%T", program.Statements[0])
	}

	if len(stmt.Actions) != 1 {
		t.Fatalf("Expected 1 action association. got=%d", len(stmt.Actions))
	}

	action := stmt.Actions[0]
	if action.ActionName.Value != "MyAction" {
		t.Errorf("Action name is not 'MyAction'. got=%s", action.ActionName.Value)
	}
	if action.Qualifier == nil || action.Qualifier.Value != "L" {
		t.Errorf("Action qualifier is not 'L'. got=%v", action.Qualifier)
	}

	if action.Duration == nil {
		t.Fatal("Action duration was not parsed.")
	}

	timeLit, ok := action.Duration.(*ast.TimeLiteral)
	if !ok {
		t.Fatalf("exp not *ast.TimeLiteral. got=%T", action.Duration)
	}
	if timeLit.Value != "5s" {
		t.Errorf("Duration value not '5s'. got=%q", timeLit.Value)
	}
}

func TestSfcErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "Missing RPAREN in action association",
			input:         `STEP MyStep: MyAction(N; END_STEP`,
			expectedError: "expected next token to be ), got ; instead",
		},
		{
			name:          "Missing END_ACTION",
			input:         `ACTION MyAction: x := 1;`,
			expectedError: "expected next token to be END_ACTION, got EOF instead",
		},
		{
			name:          "Missing FROM in TRANSITION",
			input:         `TRANSITION S1 TO S2 := TRUE; END_TRANSITION`,
			expectedError: "expected next token to be FROM, got IDENT instead",
		},
		{
			name:          "Missing TO in TRANSITION",
			input:         `TRANSITION FROM S1 S2 := TRUE; END_TRANSITION`,
			expectedError: "expected next token to be TO, got IDENT instead",
		},
		{
			name:          "Missing assignment in TRANSITION",
			input:         `TRANSITION FROM S1 TO S2 TRUE; END_TRANSITION`,
			expectedError: "expected next token to be :=, got TRUE instead",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()

			if len(p.Errors()) == 0 {
				t.Fatalf("Expected an error but got none")
			}
			assertErrorContains(t, p.Errors(), tt.expectedError)
		})
	}
}

func TestSFCProgramWithMixedElements(t *testing.T) {
	// This test validates the loop in `parseSFCProgram`, specifically covering
	// the skipping of comments and the handling of non-block statements.
	input := `
		PROGRAM MySFC
			INITIAL_STEP S1: END_STEP
			(* A comment between elements *)
			x := 1; // This is a non-block statement
			// Another comment
			TRANSITION FROM S1 TO S2 := TRUE; END_TRANSITION
		END_PROGRAM
	`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, "TestSFCProgramWithMixedElements", input)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d", len(program.Statements))
	}

	progDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ProgramDeclaration. got=%T", program.Statements[0])
	}

	sfcBody, ok := progDecl.Body.(*ast.SFCProgram)
	if !ok {
		t.Fatalf("Program body is not ast.SFCProgram. got=%T", progDecl.Body)
	}

	if len(sfcBody.Elements) != 3 {
		t.Fatalf("SFC body should have 3 elements. got=%d", len(sfcBody.Elements))
	}

	// Check element 1: Step (Block Statement)
	if _, ok := sfcBody.Elements[0].(*ast.StepStatement); !ok {
		t.Errorf("Element 0 should be a StepStatement, got %T", sfcBody.Elements[0])
	}

	// Check element 2: AssignmentStatement (Non-Block Statement)
	assignStmt, ok := sfcBody.Elements[1].(*ast.AssignmentStatement)
	if !ok {
		t.Errorf("Element 1 should be an AssignmentStatement, got %T", sfcBody.Elements[1])
	}
	testAssignmentStatement(t, assignStmt, "x", "1")

	// Check element 3: Transition (Block Statement)
	if _, ok := sfcBody.Elements[2].(*ast.TransitionStatement); !ok {
		t.Errorf("Element 2 should be a TransitionStatement, got %T", sfcBody.Elements[2])
	}
}
