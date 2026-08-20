// In beedance/transpiler/transpiler.go
package transpiler

import (
	"beedance/ast"
	"fmt"
	"io"
	"log"
	"strings" // For type information if needed
)

// Transpiler holds the state of the code generation process.
type Transpiler struct {
	w              io.Writer
	programVarName string // The name of the receiver for program methods, e.g., "p"
	currentFunc    *ast.FunctionDeclaration
	varInfo        map[string]*ast.TypeDeclaration // Maps var names in current scope to their type declaration
	accessVars     map[string]bool                 // Set of VAR_ACCESS variable names in the current scope
	ilCurrentCRType string                          // The data type of the IL Current Result
	locatedVars    map[string]bool                 // Set of VARs with an AT % location
	mainGenerated  bool                            // Flag to ensure main is only generated once
	globalVars     map[string]bool                 // Set of global variable names
	typeInfo       map[string]ast.Node
}

func New(w io.Writer) *Transpiler {
	return &Transpiler{
		w:             w,
		locatedVars:   make(map[string]bool),
		mainGenerated: false,
		accessVars:    make(map[string]bool),
		globalVars:    make(map[string]bool),
		varInfo:       make(map[string]*ast.TypeDeclaration),
		typeInfo:      make(map[string]ast.Node),
	}
}

// Transpile is the main entry point.
func (t *Transpiler) Transpile(node ast.Node) error {
	// Check if the node has leading comments and transpile them.
	if nodeWithComments, ok := node.(interface{ GetLeadingComments() []string }); ok {
		t.transpileLeadingComments(nodeWithComments.GetLeadingComments())
	}

	switch node := node.(type) {
	case *ast.Program:
		t.buildTypeInfo(node) // First pass to collect type definitions
		t.buildGlobalVarInfo(node)
		for _, stmt := range node.Statements {
			if err := t.Transpile(stmt); err != nil {
				return err
			}
		}
		return nil
	case *ast.ProgramDeclaration:
		// First, transpile any global var blocks that might exist at the program level
		t.transpileGlobalVarBlocks(node.GlobalVars)
		return t.transpileProgram(node)
	case *ast.BlockStatement:
		return t.transpileBlockStatement(node)
	case *ast.VarDeclStatement:
		t.transpileVarDecl(node)
		return nil
	case *ast.AssignmentStatement:
		return t.transpileAssignmentStatement(node)
	case *ast.IfStatement:
		return t.transpileIfStatement(node)
	case *ast.CaseStatement:
		return t.transpileCaseStatement(node)
	case *ast.ForLoopStatement:
		return t.transpileForLoopStatement(node)
	case *ast.WhileStatement:
		return t.transpileWhileStatement(node)
	case *ast.RepeatStatement:
		return t.transpileRepeatStatement(node)
	case *ast.ReturnStatement:
		return t.transpileReturnStatement(node)
	case *ast.ExitStatement:
		return t.transpileExitStatement(node)
	case *ast.ExpressionStatement:
		return t.transpileExpression(node.Expression)
	case *ast.FunctionBlockDeclaration:
		return t.transpileFunctionBlockDeclaration(node)
	case *ast.TypeBlockDeclaration:
		return t.transpileTypeBlockDeclaration(node)
	case *ast.ConfigurationDeclaration:
		return t.transpileConfigurationDeclaration(node)
	case *ast.SFCProgram:
		// This is handled within transpileProgram/transpileFunctionBlockDeclaration
		// but we add a case to prevent "unhandled type" errors if it appears elsewhere.
		return nil
	case *ast.FunctionDeclaration:
		return t.transpileFunctionDeclaration(node)

	default:
		return fmt.Errorf("unhandled AST node type: %T", node)
	}
}

// Helper to write to the buffer
func (t *Transpiler) write(format string, a ...interface{}) {
	fmt.Fprintf(t.w, format, a...)
}

// transpileLeadingComments writes comments to the output buffer, formatting them as Go comments.
func (t *Transpiler) transpileLeadingComments(comments []string) {
	for _, comment := range comments {
		t.write("// %s\n", strings.TrimSpace(comment))
	}
}

// buildGlobalVarInfo performs a pass to collect all VAR_GLOBAL names.
func (t *Transpiler) buildGlobalVarInfo(program *ast.Program) {
	for _, stmt := range program.Statements {
		if prog, ok := stmt.(*ast.ProgramDeclaration); ok {
			for _, globalBlock := range prog.GlobalVars {
				for _, decl := range globalBlock.Declarations {
					t.globalVars[decl.Name.Value] = true
				}
			}
		}
	}
}

// buildTypeInfo performs a first pass over the AST to collect all
// FUNCTION_BLOCK and TYPE declarations into a symbol table for later reference.
func (t *Transpiler) buildTypeInfo(program *ast.Program) {
	for _, stmt := range program.Statements {
		switch node := stmt.(type) {
		case *ast.FunctionBlockDeclaration:
			t.typeInfo[node.Name.Value] = node
		case *ast.FunctionDeclaration:
			t.typeInfo[node.Name.Value] = node
		case *ast.TypeBlockDeclaration:
			for _, decl := range node.Declarations {
				t.typeInfo[decl.Name.Value] = decl
			}
		}
	}
}

func (t *Transpiler) transpileGlobalVarBlocks(blocks []*ast.VarBlockDeclaration) {
	for _, block := range blocks {
		t.write("// --- VAR_GLOBAL ---\n")
		for _, decl := range block.Declarations {
			goType := t.mapIecTypeToGo(decl.DataType)
			t.write("var %s %s", decl.Name.Value, goType)
			if decl.Value != nil {
				t.write(" = ")
				t.transpileExpression(decl.Value)
			}
			t.write("\n")
		}
		t.write("\n")
	}
}

func (t *Transpiler) transpileConfigurationDeclaration(config *ast.ConfigurationDeclaration) error {
	if t.mainGenerated {
		return nil // Main function already generated
	}
	t.mainGenerated = true

	t.write("// --- Generated Main Function from CONFIGURATION ---\n")
	t.write("func main() {\n")

	// Register all program factories that are used in the configuration.
	t.write("\t// Register program factories\n")
	uniqueProgramTypes := make(map[string]bool)
	for _, res := range config.Resources {
		for _, progConfig := range res.Programs {
			uniqueProgramTypes[progConfig.TypeName.Value] = true
		}
	}
	for progType := range uniqueProgramTypes {
		t.write("\tconfig.RegisterProgramFactory(%q, New%sFactory)\n", progType, progType)
	}
	t.write("\n")

	// Build the configuration struct literal
	t.write("\t// Create the configuration from the IEC 61131-3 source\n")
	t.write("\tcfg := &config.Configuration{\n")
	t.write("\t\tName: %q,\n", config.Name.Value)
	t.write("\t\tResources: []*config.Resource{\n")

	for _, res := range config.Resources {
		t.transpileResourceDeclaration(res)
	}

	t.write("\t\t},\n")
	t.write("\t}\n\n")

	t.write("\t// This is where you would start the royaljelly scheduler with the generated config.\n")
	t.write("\tfmt.Println(\"Configuration loaded and ready to run.\")\n")
	t.write("\t// Example: royaljelly.Start(cfg)\n")

	t.write("}\n\n")
	return nil
}

func (t *Transpiler) transpileResourceDeclaration(res *ast.ResourceDeclaration) {
	// Group VAR_CONFIG parameters by their program instance name.
	paramsByInstance := make(map[string][]*ast.VarDeclStatement)
	for _, varConfig := range res.VarConfigs {
		instanceName := varConfig.ProgramInstanceName.Value
		paramsByInstance[instanceName] = varConfig.Declarations
	}

	// Group program instances by their assigned task.
	programsByTask := make(map[string][]*ast.ProgramConfiguration)
	for _, progConfig := range res.Programs {
		taskName := progConfig.TaskName.Value
		programsByTask[taskName] = append(programsByTask[taskName], progConfig)
	}

	t.write("\t\t\t\tPrograms: map[string]*config.ProgramInstance{\n")
	for _, progConfig := range res.Programs {
		t.write("\t\t\t\t\t%q: {\n", progConfig.InstanceName.Value)
		t.write("\t\t\t\t\t\tType: %q,\n", progConfig.TypeName.Value)
		t.write("\t\t\t\t\t\tParams: map[string]string{\n")
		t.transpileVarConfigParams(paramsByInstance[progConfig.InstanceName.Value])
		t.write("\t\t\t\t\t\t},\n\t\t\t\t\t},\n")
	}
	t.write("\t\t\t\t},\n")

	t.write("\t\t\t{\n")
	t.write("\t\t\t\tName: %q,\n", res.Name.Value)
	t.write("\t\t\t\tTasks: []*config.Task{\n")

	for _, task := range res.Tasks {
		// Pass the list of programs for this specific task.
		t.transpileTaskDeclaration(task, programsByTask[task.Name.Value])
	}

	t.write("\t\t\t\t},\n")
	t.write("\t\t\t},\n")
}

func (t *Transpiler) transpileTaskDeclaration(task *ast.TaskDeclaration, programs []*ast.ProgramConfiguration) {
	t.write("\t\t\t\t\t{\n")
	t.write("\t\t\t\t\t\tName: %q,\n", task.Name.Value)
	t.write("\t\t\t\t\t\tPriority: %s,\n", task.Priority.String())
	t.write("\t\t\t\t\t\tInterval: ")
	t.transpileExpression(task.Interval)
	t.write(",\n")

	t.write("\t\t\t\t\t\tPrograms: []string{")
	for i, prog := range programs {
		if i > 0 {
			t.write(", ")
		}
		t.write("%q", prog.InstanceName.Value)
	}
	t.write("},\n")

	t.write("\t\t\t\t\t},\n")
}

func (t *Transpiler) transpileProgram(prog *ast.ProgramDeclaration) error {
	t.programVarName = "p" // Set the receiver name

	// --- 1. Generate the struct definition ---
	// Build var info for this program's scope
	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	t.buildVarInfo(prog.Vars)

	originalAccessVars := t.accessVars
	t.accessVars = make(map[string]bool)
	t.buildAccessVarInfo(prog.AccessVars)

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(prog.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.accessVars = originalAccessVars }()

	defer func() { t.varInfo = originalVarInfo }() // Restore previous scope

	// If the body is an SFC program, we need to add state fields to the struct.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		t.write("\tsfcActiveSteps map[string]bool\n")
		t.transpileSFCStateFields(sfc)
		t.transpileSFCActionStateFields(sfc)
	}

	t.write("type %s struct {\n", prog.Name.Value)
	for _, varDecl := range prog.Vars {
		t.transpileVarDecl(varDecl)
	}
	t.write("}\n\n")
	for _, accessDecl := range prog.AccessVars {
		t.transpileVarAccess(accessDecl)
	}

	// --- 2. Generate the factory function for initialization ---
	t.write("// New%sFactory creates a new instance of the %s program.\n", prog.Name.Value, prog.Name.Value)
	t.write("func New%sFactory(params map[string]string) (func(time.Time), error) {\n", prog.Name.Value)
	t.write("\tinstance := &%s{}\n\n", prog.Name.Value)

	// Transpile initial values from VAR declarations
	t.write("\t// Apply initial values from ST code\n")
	for _, varDecl := range prog.Vars {
		if varDecl.Value != nil {
			t.write("\tinstance.%s = ", varDecl.Name.Value)
			// Use a temporary transpiler with the receiver unset to transpile the value expression
			valT := &Transpiler{w: t.w}
			if err := valT.transpileExpression(varDecl.Value); err != nil {
				return err
			}
			t.write("\n")
		}
	}

	// If it's an SFC program, set the initial step in the factory.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		initialStep := t.findInitialStep(sfc)
		if initialStep != nil {
			t.write("\tinstance.sfcActiveSteps = make(map[string]bool)\n")
			t.write("\tinstance.sfcActiveSteps[%q] = true\n", initialStep.Name.Value)
		}
	}

	// For FBs, initialize their EN input to TRUE by default.
	for _, varDecl := range prog.Vars {
		if t.isFunctionBlockType(varDecl.DataType) {
			t.write("\tinstance.%s.EN = true\n", varDecl.Name.Value)
		}
	}

	t.write("\n\t// TODO: Add logic to override initial values from the 'params' map if needed.\n\n")
	t.write("\treturn instance.Logic, nil\n")
	t.write("}\n\n")

	// --- 3. Generate the Logic method for the scheduler ---
	t.write("// Link connects the program's located variables to the runtime's I/O manager.\n")
	t.write("func (p *%s) Link(linker config.IOLinker) error {\n", prog.Name.Value)
	for _, varDecl := range prog.Vars {
		if varDecl.Location != nil {
			t.write("\tif err := linker.LinkIO(&p.%s, %q); err != nil {\n", varDecl.Name.Value, varDecl.Location.String())
			t.write("\t\treturn err\n")
			t.write("\t}\n")
		}
	}
	t.write("\treturn nil\n}\n\n")

	t.write("func (%s *%s) Logic(now time.Time) {\n", t.programVarName, prog.Name.Value)
	// If the body is an SFC program, transpile it as a state machine.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		if err := t.transpileSFCProgram(sfc); err != nil {
			return err
		}
	} else if block, ok := prog.Body.(*ast.BlockStatement); ok && len(block.Statements) > 0 {
		if _, isIL := block.Statements[0].(*ast.IlInstructionStatement); isIL {
			// It's an IL program
			if err := t.transpileIlProgram(block); err != nil {
				return err
			}
		}
	} else {
		if err := t.transpileSFCProgram(sfc); err != nil {
			return err
		} else {
            // Otherwise, transpile as a regular block of statements.
            if err := t.Transpile(prog.Body); err != nil {
                return err
            }
        }
    }
	t.write("}\n")
	t.programVarName = "" // Unset after use
	return nil
}

func (t *Transpiler) transpileIlProgram(body *ast.BlockStatement) error {
	// Declare typed accumulators for the main data type families.
	t.write("\t// Typed accumulators for IL Current Result (CR)\n")
	t.write("\tvar cr_BOOL iec.BOOL\n")
	t.write("\tvar cr_LINT iec.LINT\n")
	t.write("\tvar cr_LREAL iec.LREAL\n")
	t.write("\tvar cr_TIME iec.TIME\n")
	t.write("\tvar cr_STRING iec.STRING\n")
	t.write("\t_ = cr_BOOL; _ = cr_LINT; _ = cr_LREAL; _ = cr_TIME; _ = cr_STRING // Avoid unused var errors\n\n")

	originalCRType := t.ilCurrentCRType
	defer func() { t.ilCurrentCRType = originalCRType }() // Restore after transpiling

	for _, stmt := range body.Statements {
		if ilStmt, ok := stmt.(*ast.IlInstructionStatement); ok {
			if err := t.transpileIlInstruction(ilStmt); err != nil {
				t.ilCurrentCRType = "" // Reset on error
				return err
			}
		} else {
			// This case should ideally not be hit in a pure IL program.
			log.Printf("Warning: Non-IL statement found in IL program body: %T", stmt)
		}
	}
	t.ilCurrentCRType = "" // Reset after use
	return nil
}

func (t *Transpiler) transpileIlInstruction(stmt *ast.IlInstructionStatement) error {
	// Transpile the label, if it exists.
	if stmt.Label != nil {
		t.write("%s:\n", stmt.Label.Value)
	}

	// Handle conditional execution (C modifier)
	isConditional := strings.Contains(stmt.Modifier, "C")
	if isConditional {
		// Conditional operations in IL are always based on a BOOL accumulator.
		t.write("\tif cr_BOOL {\n")
	}

	op := strings.ToUpper(stmt.Operator)
	switch op {
	case "LD":
		// Check for a parenthesized IL sub-program, e.g., LD (LD A ADD B)
		if block, ok := stmt.Operand.(*ast.BlockStatement); ok && len(block.Statements) > 0 && isIlBlock(block) {
			// This is a parenthesized IL sub-program, e.g., LD (LD A ADD B)
			// We transpile it into an anonymous Go function to isolate its accumulator.
			t.write("\tcr_LINT = func() iec.LINT {\n") // Assuming result is LINT for now
			t.transpileIlProgram(block)
			t.write("\t\treturn cr_LINT\n") // Return the sub-program's final CR
			t.write("\t}()\n") // cspell:disable-line
			t.ilCurrentCRType = "LINT"
		} else {
			// Standard operand (variable or ST expression)
			targetTypeDecl := t.resolveAssignmentTargetType(stmt.Operand)
			if targetTypeDecl == nil {
				// Fallback for literals or unresolved types
				t.ilCurrentCRType = "LINT" // Assume LINT for literals or ST expressions
				t.write("\tcr_LINT = ")
				t.transpileExpression(stmt.Operand)
				t.write("\n")
			} else {
				baseType := t.getBaseTypeFamily(targetTypeDecl.DataType)
				t.ilCurrentCRType = baseType
				t.write("\tcr_%s = %s(", baseType, baseType) // Cast the expression to the target CR type
				t.transpileExpression(stmt.Operand)
				t.write(")\n")
			}
		}
	case "ST":
		targetTypeDecl := t.resolveAssignmentTargetType(stmt.Operand)
		if targetTypeDecl == nil {
			return fmt.Errorf("cannot determine type of ST target: %s", stmt.Operand.String())
		}
		targetType := t.mapIecTypeToGo(targetTypeDecl.DataType)
		crType := t.ilCurrentCRType

		t.transpileExpression(stmt.Operand)
		t.write(" = %s(cr_%s)\n", targetType, crType) // e.g., p.MyInt = iec.INT(cr_LINT)

	case "ADD", "SUB", "MUL", "DIV", "AND", "OR", "XOR":
		// Determine the operator and the correct accumulator to use.
		goOp := t.mapIlOperatorToGo(op)
		crType := t.ilCurrentCRType
		if crType == "" {
			return fmt.Errorf("IL operator '%s' used before accumulator was loaded (LD)", op)
		}
		t.write("\tcr_%s = cr_%s %s ", crType, crType, goOp)
		t.transpileExpression(stmt.Operand)
		t.write("\n")

	case "GT", "LT", "EQ", "NE", "GE", "LE":
		goOp := t.mapIlOperatorToGo(op)
		crType := t.ilCurrentCRType
		if crType == "" {
			return fmt.Errorf("IL operator '%s' used before accumulator was loaded (LD)", op)
		}
		// The result of a comparison is always BOOL.
		t.write("\tcr_BOOL = cr_%s %s ", crType, goOp)
		t.transpileExpression(stmt.Operand)
		t.write("\n")
		// The current result is now a boolean.
		t.ilCurrentCRType = "BOOL"

	case "JMP":
		t.write("\tgoto %s\n", stmt.Operand.String())

	case "CAL":
		return t.transpileIlCalInstruction(stmt)

	case "RET":
		t.write("\treturn\n")

	case "S": // Set
		// 'S' is conditional on the boolean accumulator.
		t.write("\tif cr_BOOL { ")
		t.transpileExpression(stmt.Operand)
		t.write(" = true }\n")

	case "R": // Reset
		// 'R' is conditional on the boolean accumulator.
		t.write("\tif cr_BOOL { ")
		t.transpileExpression(stmt.Operand)
		t.write(" = false }\n")

	default:
		log.Printf("Warning: Unhandled IL operator: %s", op)
		t.write("\t// Unhandled IL operator: %s\n", op)
	}

	if isConditional {
		t.write("\t}\n")
	}

	return nil
}

func (t *Transpiler) transpileIlCalInstruction(stmt *ast.IlInstructionStatement) error {
	callExpr, ok := stmt.Operand.(*ast.CallExpression)
	if !ok {
		return fmt.Errorf("operand for CAL must be a function or function block call, got %T", stmt.Operand)
	}

	// Determine if the call is to a FUNCTION or a FUNCTION_BLOCK instance.
	if ident, ok := callExpr.Function.(*ast.Identifier); ok {
		// Is it a variable in the current scope?
		if varTypeDecl := t.varInfo[ident.Value]; varTypeDecl != nil {
			// Yes. Is its type a FUNCTION_BLOCK?
			if t.isFunctionBlockType(varTypeDecl.DataType) {
				// It's a FUNCTION_BLOCK call.
				return t.transpileIlFunctionBlockCall(callExpr)
			}
		}

		// Is it a globally defined FUNCTION?
		if defNode, ok := t.typeInfo[ident.Value]; ok {
			if _, isFunc := defNode.(*ast.FunctionDeclaration); isFunc {
				// It's a standard FUNCTION call.
				return t.transpileIlFunctionCall(callExpr)
			}
		}
	}

	// Fallback for built-in functions (e.g., SIN, COS) which are not in typeInfo.
	// We'll treat them as standard function calls.
	return t.transpileIlFunctionCall(callExpr)
}

func (t *Transpiler) transpileIlFunctionBlockCall(callExpr *ast.CallExpression) error {
	// 1. Transpile the input assignments for the function block call.
	t.transpileFBInputAssignments(callExpr)

	// 2. Transpile the call to the Logic() method.
	t.transpileExpression(callExpr.Function)
	t.write(".Logic(now)\n")

	// 3. Identify the primary output (first VAR_OUTPUT) to load into the accumulator.
	fbDef := t.getFunctionBlockDefinition(callExpr.Function)
	if fbDef == nil {
		// This could be a standard library FB like TON. We need a way to know their outputs.
		// For now, we'll assume a common output like 'Q' if the definition isn't found in user code.
		log.Printf("Warning: Could not find user definition for FB %s. Assuming primary output 'Q'.", callExpr.Function.String())
		t.write("\tcr_BOOL = ") // Assume BOOL type for 'Q'
		t.transpileExpression(callExpr.Function)
		t.write(".Q\n")
		t.ilCurrentCRType = "BOOL"
		return nil
	}

	if len(fbDef.VarOutputs) == 0 {
		return fmt.Errorf("CAL instruction used on function block with no outputs: %s", callExpr.Function.String())
	}

	primaryOutput := fbDef.VarOutputs[0]
	primaryOutputName := primaryOutput.Name.Value
	baseType := t.getBaseTypeFamily(primaryOutput.DataType)

	// 4. Generate the code to load the primary output into the correct typed accumulator.
	t.write("\tcr_%s = ", baseType)
	t.transpileExpression(callExpr.Function)
	t.write(".%s\n", primaryOutputName)

	// 5. Update the transpiler's state to reflect the new accumulator type.
	t.ilCurrentCRType = baseType
	return nil
}

func (t *Transpiler) transpileIlFunctionCall(callExpr *ast.CallExpression) error {
	// 1. Determine the return type of the function.
	var returnType ast.Expression
	var funcName string
	if ident, ok := callExpr.Function.(*ast.Identifier); ok {
		funcName = ident.Value
	}

	if defNode, ok := t.typeInfo[funcName]; ok {
		if funcDef, isFunc := defNode.(*ast.FunctionDeclaration); isFunc {
			returnType = funcDef.ReturnType
		}
	}

	crType := "LINT" // Default assumption for built-ins
	if returnType != nil {
		crType = t.getBaseTypeFamily(returnType)
	}
	t.ilCurrentCRType = crType
	t.write("\tcr_%s = ", crType)

	// 2. Transpile the call itself.
	t.transpileStandardFunctionCall(callExpr)
	t.write("\n")

	return nil
}

func isIlBlock(block *ast.BlockStatement) bool {
	if len(block.Statements) == 0 {
		return false
	}
	_, isIL := block.Statements[0].(*ast.IlInstructionStatement)
	return isIL
}

func (t *Transpiler) getBaseTypeFamily(dataType ast.Expression) string {
	// This is a simplified function to categorize types into broad families
	// for selecting the correct accumulator.
	typeStr := strings.ToUpper(dataType.String())
	switch {
	case strings.Contains(typeStr, "INT"):
		return "LINT"
	case strings.Contains(typeStr, "REAL"):
		return "LREAL"
	case typeStr == "BOOL":
		return "BOOL"
	case typeStr == "STRING" || typeStr == "WSTRING":
		return "STRING"
	case typeStr == "TIME":
		return "TIME"
	default:
		return "LINT" // Default to integer for enums, etc.
	}
}

func (t *Transpiler) mapIlOperatorToGo(op string) string {
	// Maps IL operators to their Go equivalents.
	upperOp := strings.ToUpper(op)
	switch upperOp {
	case "GT":
		return ">"
	case "LT":
		return "<"
	case "EQ":
		return "=="
	case "NE":
		return "!="
	case "GE":
		return ">="
	case "LE":
		return "<="
	}
	return op // For ADD, MUL, DIV, AND, OR, XOR, the Go operator is the same.
}

func (t *Transpiler) transpileSFCStateFields(sfc *ast.SFCProgram) {
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			// For Step S1, generate S1_X bool and S1_T time.Duration
			t.write("\t%s_X bool\n", step.Name.Value)
			t.write("\t%s_X_prev bool\n", step.Name.Value) // For P qualifier
			t.write("\t%s_T time.Duration\n", step.Name.Value)
		}
	}
}

func (t *Transpiler) transpileSFCActionStateFields(sfc *ast.SFCProgram) {
	// Find all unique action names to create state variables for them.
	uniqueActions := make(map[string]bool)
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			for _, actionBlock := range step.Actions {
				uniqueActions[actionBlock.ActionName.Value] = true
			}
		}
	}

	for actionName := range uniqueActions {
		t.write("\t%s_Q bool\n", actionName)
		// Check if any usage of this action is timed, if so, add a timer field.
		if t.isActionTimed(sfc, actionName) {
			t.write("\t%s_Timer time.Time\n", actionName)
		}
	}
}

func (t *Transpiler) findInitialStep(sfc *ast.SFCProgram) *ast.StepStatement {
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok && step.IsInitial {
			return step
		}
	}
	return nil
}

func (t *Transpiler) transpileSFCProgram(sfc *ast.SFCProgram) error {
	// --- Phase 0: Store previous step state for Pulse qualifiers ---
	t.write("\t// --- SFC Phase 0: Store previous step state ---\n")
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			t.write("\tp.%s_X_prev = p.%s_X\n", step.Name.Value, step.Name.Value)
		}
	}
	t.write("\n")

	t.write("\t// --- SFC Phase 1: Evaluate Transitions ---\n")
	t.write("\tfiredTransitions := []*ast.TransitionStatement{}\n")

	for _, element := range sfc.Elements {
		if trans, ok := element.(*ast.TransitionStatement); ok {
			// Check if all source steps for this transition are active.
			t.write("\t// Check transition from %v\n", trans.From)
			t.write("\tif ")
			for i, fromStep := range trans.From {
				if i > 0 {
					t.write(" && ")
				}
				t.write("p.sfcActiveSteps[%q]", fromStep.Value)
			}
			t.write(" {\n")
			t.write("\t\tif ")
			t.transpileExpression(trans.Condition)
			t.write(" {\n")
			// This is a hack. In a real scenario, we'd pass the AST node itself.
			t.write("\t\t\t// Fired transition from %s to %s\n", trans.From[0].Value, trans.To[0].Value)
			t.write("\t\t}\n")
			t.write("\t}\n")
		}
	}
	t.write("\n")

	t.write("\t// --- SFC Phase 2: Update Step States ---\n")
	t.write("\t// In a real implementation, you would iterate over firedTransitions.\n")
	t.write("\t// For this example, we'll manually code the logic based on the previous switch.\n")
	t.write("\tnextActiveSteps := make(map[string]bool)\n")
	t.write("\tfor step, active := range p.sfcActiveSteps {\n")
	t.write("\t\tif active { nextActiveSteps[step] = true }\n")
	t.write("\t}\n\n")

	t.write("\t// --- SFC Phase 3: Update Step and Action States ---\n")
	t.write("\tp.sfcActiveSteps = nextActiveSteps\n")

	// Reset step active flags and then set them for the current active steps.
	t.write("\t// Reset all step active flags\n")
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			t.write("\tp.%s_X = p.sfcActiveSteps[%q]\n", step.Name.Value, step.Name.Value)
		}
	}
	t.write("\n")

	// Process actions based on the final set of active steps.
	t.write("\t// Process actions for active steps\n")
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			t.write("\t// Actions for step %s\n", step.Name.Value)
			t.write("\tif p.sfcActiveSteps[%q] {\n", step.Name.Value)
			for _, actionBlock := range step.Actions {
				t.transpileSFCAction(actionBlock, true)
			}
			t.write("\t} else {\n")
			// Handle logic for when the step is NOT active (for stored/timed actions)
			for _, actionBlock := range step.Actions {
				t.transpileSFCAction(actionBlock, false)
			}
			t.write("\t}\n")
		}
	}
	t.write("\n")

	// Finally, execute the bodies of all actions that are currently active.
	t.write("\t// --- SFC Phase 4: Execute Action Bodies ---\n")
	uniqueActions := make(map[string]*ast.ActionBlockStatement)
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			for _, actionBlock := range step.Actions {
				uniqueActions[actionBlock.ActionName.Value] = actionBlock
			}
		}
	}
	for name, actionBlock := range uniqueActions {
		t.write("\tif p.%s_Q {\n", name)
		t.transpileBlockStatement(actionBlock.Body)
		t.write("\t}\n")
	}

	return nil
}

func (t *Transpiler) transpileSFCAction(actionBlock *ast.ActionBlockStatement, isStepActive bool) {
	actionName := actionBlock.ActionName.Value
	qualifier := "N"
	if actionBlock.Qualifier != nil {
		qualifier = actionBlock.Qualifier.Value
	}

	if isStepActive {
		switch qualifier {
		case "N":
			t.write("\t\tp.%s_Q = true\n", actionName)
		case "S":
			t.write("\t\tp.%s_Q = true\n", actionName)
		case "R":
			t.write("\t\tp.%s_Q = false\n", actionName)
		case "P":
			// Action is active only if the step just became active (was not active in the previous scan).
			t.write("\t\tp.%s_Q = p.%s_X && !p.%s_X_prev\n", actionName, actionBlock.StepName, actionBlock.StepName)
		case "D", "SD", "DS":
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now }\n", actionName, actionName)
			t.write("\t\tif now.Sub(p.%s_Timer) >= ", actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = true }\n", actionName)
		case "L", "SL":
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now }\n", actionName, actionName)
			t.write("\t\tif now.Sub(p.%s_Timer) < ", actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = true } else { p.%s_Q = false }\n", actionName, actionName)
		}
	} else { // Step is not active
		switch qualifier {
		case "N", "L", "D", "P": // Non-stored qualifiers, including Pulse
			t.write("\t\tp.%s_Q = false\n", actionName)
			t.write("\t\tp.%s_Timer = time.Time{}\n", actionName) // Reset timer
		case "SL": // Stored-Limited also deactivates and stays off
			t.write("\t\tif !p.%s_Timer.IsZero() && now.Sub(p.%s_Timer) >= ", actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = false }\n", actionName)
		// For S and SD/DS, the state is maintained, so we do nothing here.
		}
	}
}

func (t *Transpiler) transpileFunctionBlockDeclaration(fb *ast.FunctionBlockDeclaration) error {
	// 1. Generate the struct for the Function Block.
	t.write("// %s is the transpiled struct for the FUNCTION_BLOCK of the same name.\n", fb.Name.Value)

	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	t.buildVarInfo(fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars)

	originalAccessVars := t.accessVars
	t.accessVars = make(map[string]bool)
	t.buildAccessVarInfo(fb.AccessVars)

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(fb.VarInputs, fb.VarOutputs, fb.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.accessVars = originalAccessVars }()

	defer func() { t.varInfo = originalVarInfo }()

	// --- Add implicit EN and ENO fields ---
	t.write("\tEN  iec.BOOL\n")
	t.write("\tENO iec.BOOL\n")

	t.write("type %s struct {\n", fb.Name.Value)

	// Transpile VAR_INPUT, VAR_OUTPUT, and VAR into struct fields.
	for _, varDecl := range fb.VarInputs {
		t.transpileVarDecl(varDecl)
	}
	for _, varDecl := range fb.VarOutputs {
		t.transpileVarDecl(varDecl)
	}
	for _, varDecl := range fb.VarInOuts {
		t.transpileVarDeclInOut(varDecl)
	}
	for _, varDecl := range fb.Vars {
		t.transpileVarDecl(varDecl)
	}
	for _, accessDecl := range fb.AccessVars {
		t.transpileVarAccess(accessDecl)
	}

	t.write("}\n\n")

	// 2. Generate the Logic method for the Function Block.
	// The receiver name is specific to this FB.
	receiverName := strings.ToLower(fb.Name.Value[:1])
	originalProgramVarName := t.programVarName
	t.programVarName = receiverName // Set context for transpiling the body

	t.write("// Logic executes the logic for the %s FUNCTION_BLOCK.\n", fb.Name.Value)
	t.write("func (%s *%s) Logic(now time.Time) {\n", receiverName, fb.Name.Value)

	// --- EN/ENO wrapper logic ---
	t.write("\tif !%s.EN {\n", receiverName)
	t.write("\t\t%s.ENO = false\n", receiverName)
	t.write("\t\treturn\n")
	t.write("\t}\n")
	t.write("\t%s.ENO = true\n\n", receiverName)

	if err := t.Transpile(fb.Body); err != nil {
		t.programVarName = originalProgramVarName // Restore context on error
		return err
	}

	t.write("}\n\n")

	t.programVarName = originalProgramVarName // Restore context
	return nil
}

func (t *Transpiler) isTransitionFromStep(trans *ast.TransitionStatement, stepName string) bool {
	for _, from := range trans.From {
		if from.Value == stepName {
			return true
		}
	}
	return false
}

func (t *Transpiler) transpileSFCTransition(trans *ast.TransitionStatement) {
	t.write("\t\t// Transition from %s to %s\n", trans.From[0].Value, trans.To[0].Value)
	t.write("\t\tif ")
	t.transpileExpression(trans.Condition)
	t.write(" {\n")
	// Deactivate source steps and activate destination steps
	for _, from := range trans.From {
		t.write("\t\t\tdelete(nextActiveSteps, %q)\n", from.Value)
	}
	for _, to := range trans.To {
		t.write("\t\t\tnextActiveSteps[%q] = true\n", to.Value)
	}
	t.write("\t\t}\n")
}

func (t *Transpiler) buildVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			// We need to resolve the type declaration for this variable.
			var typeName string
			currentType := varDecl.DataType
			// For arrays, we need to get the base element type.
			for {
				if arrayDef, ok := currentType.(*ast.ArrayDefinition); ok {
					currentType = arrayDef.DataType
				} else {
					break
				}
			}
			if typeIdent, ok := currentType.(*ast.Identifier); ok {
				typeName = typeIdent.Value
			}
			t.varInfo[varDecl.Name.Value] = t.getTypeDeclaration(typeName)
		}
	}
}

func (t *Transpiler) buildLocatedVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			if varDecl.Location != nil {
				t.locatedVars[varDecl.Name.Value] = true
			}
		}
	}
}

func (t *Transpiler) buildAccessVarInfo(accessBlocks []*ast.VarAccessDeclaration) {
	for _, block := range accessBlocks {
		// This is a simplified version. A real implementation would have a list of declarations inside the block.
		// Assuming the parser provides a flat list for now.
		// for _, decl := range block.Declarations {
		// 	t.accessVars[decl.LocalName.Value] = true
		// }
	}
}

func (t *Transpiler) isFunctionBlockType(dataType ast.Expression) bool {
	if typeIdent, ok := dataType.(*ast.Identifier); ok {
		if typeDef, ok := t.typeInfo[typeIdent.Value]; ok {
			if _, isFB := typeDef.(*ast.FunctionBlockDeclaration); isFB {
				return true
			}
		}
	}
	return false
}

func (t *Transpiler) transpileFunctionDeclaration(fd *ast.FunctionDeclaration) error {
	// Set context for the duration of this function's transpilation
	originalFunc := t.currentFunc
	t.currentFunc = fd
	defer func() { t.currentFunc = originalFunc }()

	// --- 1. Build the function signature ---
	t.write("func %s(", fd.Name.Value)

	// Parameters (VAR_INPUT, VAR_IN_OUT)
	params := []string{}
	for _, p := range fd.VarInputs {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
	}
	for _, p := range fd.VarInOuts {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s *%s", p.Name.Value, goType))
	}
	t.write(strings.Join(params, ", "))
	t.write(") ")

	// Return values (primary return type + VAR_OUTPUT)
	returns := []string{}
	if fd.ReturnType != nil {
		returns = append(returns, t.mapIecTypeToGo(fd.ReturnType))
	}
	for _, p := range fd.VarOutputs {
		returns = append(returns, t.mapIecTypeToGo(p.DataType))
	}
	if len(returns) > 1 {
		t.write("(%s)", strings.Join(returns, ", "))
	} else if len(returns) == 1 {
		t.write(returns[0])
	}

	t.write(" {\n")

	// --- 2. Transpile local variables (VAR) ---
	for _, v := range fd.Vars {
		t.write("\tvar %s %s\n", v.Name.Value, t.mapIecTypeToGo(v.DataType))
	}
	t.write("\n")

	// --- 3. Transpile the function body ---
	if err := t.Transpile(fd.Body); err != nil {
		return err
	}

	t.write("}\n\n")
	return nil
}

func (t *Transpiler) transpileVarDecl(varDecl *ast.VarDeclStatement) {
	// Get the variable name.
	// If it's a located variable, transpile it as a pointer.
	if varDecl.Location != nil {
		goType := t.mapIecTypeToGo(varDecl.DataType)
		t.write("\t%s *%s // AT %s\n", varDecl.Name.Value, goType, varDecl.Location.String())
		return
	}

	name := varDecl.Name.Value

	// Get the Go type for the IEC data type.
	goType := t.mapIecTypeToGo(varDecl.DataType)

	// Write the struct field. e.g., "MyCounter iec.LINT"
	t.write("\t%s %s\n", name, goType)

}

func (t *Transpiler) transpileVarAccess(accessDecl *ast.VarAccessDeclaration) {
	localName := accessDecl.LocalName.Value

	// Infer the type of the target variable.
	targetTypeDecl := t.resolveAssignmentTargetType(accessDecl.AccessPath)
	if targetTypeDecl == nil {
		log.Printf("Warning: Could not resolve type for VAR_ACCESS target: %s", accessDecl.AccessPath.String())
		t.write("\t%s *any // Could not resolve type for %s\n", localName, accessDecl.AccessPath.String())
		return
	}

	// Get the Go type and declare the field as a pointer to that type.
	goType := t.mapIecTypeToGo(targetTypeDecl.DataType)
	t.write("\t%s *%s\n", localName, goType)
}

func (t *Transpiler) LinkMethodBody(prog *ast.ProgramDeclaration) string {
	// This is a placeholder for the logic that would be generated inside the Link method.
	return "// Runtime linking logic will be placed here by the royaljelly scheduler.\n"
}

// transpileVarDeclInOut transpiles a VAR_IN_OUT declaration to a pointer field.
func (t *Transpiler) transpileVarDeclInOut(varDecl *ast.VarDeclStatement) {
	name := varDecl.Name.Value
	goType := t.mapIecTypeToGo(varDecl.DataType)
	// Write the struct field as a pointer, e.g., "MyVar *iec.INT"
	t.write("\t%s *%s\n", name, goType)

}

// transpileCaseStatement transpiles an IEC 61131-3 CASE statement to a Go switch statement.
func (t *Transpiler) transpileCaseStatement(stmt *ast.CaseStatement) error {
	t.write("switch ")
	if err := t.transpileExpression(stmt.Expression); err != nil {
		return err
	}
	t.write(" {\n")

	for _, caseElem := range stmt.Cases {
		t.write("case ")
		for i, expr := range caseElem.Values {
			if i > 0 {
				t.write(", ")
			}
			if err := t.transpileExpression(expr); err != nil {
				return err
			}
		}
		t.write(":\n")
		if err := t.transpileBlockStatement(caseElem.Consequence); err != nil {
			return err
		}
	}

	if stmt.Alternative != nil {
		t.write("default:\n")
		if err := t.transpileBlockStatement(stmt.Alternative); err != nil {
			return err
		}
	}
	t.write("}\n")
	return nil
}

// transpileAssignmentStatement transpiles an IEC `:=` assignment to a Go `=` assignment.
func (t *Transpiler) transpileAssignmentStatement(stmt *ast.AssignmentStatement) error {
	// Special case: Assignment to the function name is a return statement.
	if ident, ok := stmt.Left.(*ast.Identifier); ok && t.currentFunc != nil && ident.Value == t.currentFunc.Name.Value {
		t.write("return ")
		if err := t.transpileExpression(stmt.Value); err != nil {
			return err
		}
		t.write("\n")
		return nil
	}

	// Check if the left-hand side resolves to a subrange type.
	if typeDecl := t.resolveAssignmentTargetType(stmt.Left); typeDecl != nil && typeDecl.Subrange != nil {
		return t.transpileSubrangeAssignment(stmt, typeDecl)
	}

	// Standard assignment
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")
	if err := t.transpileExpression(stmt.Value); err != nil {
		return err
	}
	t.write("\n")
	return nil
}

// resolveAssignmentTargetType recursively finds the TypeDeclaration for the target of an assignment.
// It can handle simple variables, array elements, and struct members.
func (t *Transpiler) resolveAssignmentTargetType(expr ast.Expression) *ast.TypeDeclaration {
	switch e := expr.(type) {
	case *ast.Identifier:
		// Base case: a simple variable. Look it up in the current scope's varInfo.
		return t.varInfo[e.Value]

	case *ast.IndexExpression:
		// It's an array element. The type of the element is the type of the array's base variable.
		// Recursively resolve the type of the array itself.
		return t.resolveAssignmentTargetType(e.Left)

	case *ast.MemberAccessExpression:
		// It's a struct field. We need to find the type of the struct, then the type of the field.
		structVarType := t.resolveAssignmentTargetType(e.Struct)
		if structVarType == nil {
			return nil // Can't resolve the struct's type.
		}

		// The struct's type definition should be in the main typeInfo map.
		structDefNode, ok := t.typeInfo[structVarType.Name.Value]
		if !ok {
			return nil
		}

		// Now, find the member's type declaration within the struct's definition.
		return t.findMemberType(structDefNode, e.Member.Value)
	}
	return nil
}

func (t *Transpiler) getTypeDeclaration(typeName string) *ast.TypeDeclaration {
	if typeDef, ok := t.typeInfo[typeName]; ok {
		if typeDecl, ok := typeDef.(*ast.TypeDeclaration); ok {
			return typeDecl
		}
	}
	return nil
}

// findMemberType looks inside a STRUCT definition for a specific member and returns its type declaration.
func (t *Transpiler) findMemberType(structDefNode ast.Node, memberName string) *ast.TypeDeclaration {
	typeDecl, ok := structDefNode.(*ast.TypeDeclaration)
	if !ok {
		return nil
	}
	structDef, ok := typeDecl.DataType.(*ast.StructDefinition)
	if !ok {
		return nil
	}

	// This is a simplified lookup. A real implementation would build a symbol table for the struct members.
	return t.getTypeDeclaration(structDef.GetMemberType(memberName))
}

func (t *Transpiler) transpileSubrangeAssignment(stmt *ast.AssignmentStatement, typeDecl *ast.TypeDeclaration) error {
	// Transpile the left side of the assignment (the variable).
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")

	// Get the base type to select the correct Clamp function (e.g., "INT" -> "ClampINT").
	baseType := strings.ToUpper(typeDecl.DataType.String())
	clampFunc := "iec.Clamp" + baseType

	// Get the subrange bounds.
	subrange, ok := typeDecl.Subrange.(*ast.InfixExpression)
	if !ok || subrange.Operator != ".." {
		return fmt.Errorf("invalid subrange definition for type %s", typeDecl.Name.Value)
	}

	// Write the call to the clamp function: iec.ClampINT(value, min, max)
	t.write("%s(", clampFunc)
	if err := t.transpileExpression(stmt.Value); err != nil {
		return err
	}
	t.write(", %s, %s)\n", subrange.Left.String(), subrange.Right.String())

	return nil
}

// transpileForLoopStatement transpiles a FOR loop to a Go `for` loop.
func (t *Transpiler) transpileForLoopStatement(stmt *ast.ForLoopStatement) error {
	// The control variable is an assignment statement itself.
	t.write("for ")
	t.transpileExpression(stmt.ControlVar.Left)
	t.write(" := ")
	t.transpileExpression(stmt.ControlVar.Value)
	t.write("; ")
	t.transpileExpression(stmt.ControlVar.Left)
	t.write(" <= ")
	t.transpileExpression(stmt.EndValue)
	t.write("; ")
	t.transpileExpression(stmt.ControlVar.Left)
	if stmt.StepValue != nil {
		t.write(" += ")
		t.transpileExpression(stmt.StepValue)
	} else {
		t.write("++")
	}
	t.write(" {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("}\n")
	return nil
}

// transpileBlockStatement iterates over statements in a block and transpiles them.
func (t *Transpiler) transpileBlockStatement(bs *ast.BlockStatement) error {
	for _, stmt := range bs.Statements {
		if err := t.Transpile(stmt); err != nil {
			return err
		}
	}
	return nil
}

// transpileWhileStatement transpiles a WHILE loop to a Go `for` loop.
func (t *Transpiler) transpileWhileStatement(stmt *ast.WhileStatement) error {
	t.write("for ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("}\n")
	return nil
}

// transpileRepeatStatement transpiles a REPEAT...UNTIL loop to a Go `for` loop.
func (t *Transpiler) transpileRepeatStatement(stmt *ast.RepeatStatement) error {
	t.write("for {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("if ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" { break }\n")
	t.write("}\n")
	return nil
}

// transpileReturnStatement transpiles a RETURN statement.
func (t *Transpiler) transpileReturnStatement(stmt *ast.ReturnStatement) error {
	t.write("return")
	if stmt.ReturnValue != nil {
		t.write(" ")
		if err := t.transpileExpression(stmt.ReturnValue); err != nil {
			return err
		}
	}
	t.write("\n")
	return nil
}

// transpileExitStatement transpiles an EXIT statement to a `break`.
func (t *Transpiler) transpileExitStatement(stmt *ast.ExitStatement) error {
	t.write("break\n")
	return nil
}

// transpileExpression dispatches to specific handlers for different expression types.
func (t *Transpiler) transpileExpression(exp ast.Expression) error {
	switch exp := exp.(type) {
	case *ast.Identifier:
		// If it's a located or access variable, it's a pointer and must be dereferenced.
		if t.locatedVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.accessVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.globalVars[exp.Value] {
			// If it's a global variable, write it without a prefix.
			if t.accessVars[exp.Value] {
				t.write("(*%s.%s)", t.programVarName, exp.Value)
			} else if t.globalVars[exp.Value] {
				// If it's a global variable, write it without a prefix.
				t.write("%s", exp.Value)
			} else if t.programVarName != "" {
				// Otherwise, it's a local/member variable, so prefix it with the receiver.
				t.write("%s.%s", t.programVarName, exp.Value)
			} else {
				t.write("%s", exp.Value)
			}
		}
	case *ast.IntegerLiteral:
		t.write("%d", exp.Value)
	case *ast.UnsignedIntegerLiteral:
		t.write("%d", exp.Value)
	case *ast.BitStringLiteral:
		// Assuming a helper in iec package to create bitstrings
		t.write("iec.MakeBitString(%d, %d)", exp.Value, exp.Width)
	case *ast.LRealLiteral:
		t.write("%f", exp.Value)
	case *ast.WStringLiteral:
		t.write("%q", exp.Value)
	case *ast.DateLiteral:
		// Assuming an iec helper `iec.MakeDate("D#...")`
		t.write("iec.MakeDate(%q)", exp.Value)
	case *ast.TimeOfDayLiteral:
		// Assuming an iec helper `iec.MakeTOD("TOD#...")`
		t.write("iec.MakeTOD(%q)", exp.Value)
	case *ast.DateAndTimeLiteral:
		// Assuming an iec helper `iec.MakeDT("DT#...")`
		t.write("iec.MakeDT(%q)", exp.Value)
	case *ast.Boolean:
		t.write("%t", exp.Value)
	case *ast.RealLiteral:
		t.write("%f", exp.Value) // This might need more careful formatting for IEC REAL
	case *ast.StringLiteral:
		t.write("%q", exp.Value)
	case *ast.TimeLiteral:
		// Assuming an iec helper `iec.MakeTime("T#5s")`
		t.write("iec.MakeTime(\"%s\")", exp.Value)
	case *ast.TypedLiteral:
		return t.transpileTypedLiteral(exp)
	case *ast.InfixExpression:
		return t.transpileInfixExpression(exp)
	case *ast.PrefixExpression:
		return t.transpilePrefixExpression(exp)
	case *ast.CallExpression:
		return t.transpileCallExpression(exp)
	case *ast.MemberAccessExpression:
		return t.transpileMemberAccessExpression(exp)
	case *ast.IndexExpression:
		return t.transpileIndexExpression(exp)
	case *ast.EnumeratedValueLiteral:
		return t.transpileEnumeratedValueLiteral(exp)
	case *ast.ArrayLiteral:
		return t.transpileArrayLiteral(exp)
	case *ast.IlInstructionStatement, *ast.StepStatement, *ast.TransitionStatement, *ast.ActionStatement, *ast.ActionBlockStatement:
		// These are handled by their parent SFC/IL transpilers, so we can ignore them here.
		return nil

	default:
		log.Printf("Warning: Unhandled expression type in transpiler: %T", exp)
		t.write("/* unhandled_expression: %T */", exp)
	}
	return nil
}

func (t *Transpiler) transpileTypeBlockDeclaration(tbd *ast.TypeBlockDeclaration) error {
	for _, decl := range tbd.Declarations {
		// Check if the type declaration is for a STRUCT
		if structDef, ok := decl.DataType.(*ast.StructDefinition); ok {
			t.write("// %s is the transpiled struct for the user-defined type.\n", decl.Name.Value)
			t.write("type %s struct {\n", decl.Name.Value)
			for _, member := range structDef.Members {
				t.transpileVarDecl(member)
			}
			t.write("}\n\n")
		} else if enumDef, ok := decl.DataType.(*ast.EnumDefinition); ok {
			// Handle ENUM definitions
			t.transpileEnumDefinition(decl.Name, enumDef)
		} else if decl.Subrange != nil {
			// Handle Subrange types
			baseType := t.mapIecTypeToGo(decl.DataType)
			t.write("// %s is a subrange of %s.\n", decl.Name.Value, baseType)
			t.write("type %s %s\n\n", decl.Name.Value, baseType)
			// Note: Runtime range checks are not added by the transpiler at this stage.
		}
	}
	return nil
}

// mapIecTypeToGo converts an AST expression representing an IEC type
// into the corresponding Go type string from the `iec` package.
func (t *Transpiler) mapIecTypeToGo(dataType ast.Expression) string {
	// The DataType is an expression, which for simple types is an Identifier.
	if ident, ok := dataType.(*ast.Identifier); ok { // e.g., INT, or MyStruct
		typeName := ident.Value
		// Check if it's a user-defined type (STRUCT, ENUM, etc.) we've registered.
		if _, isUserDefined := t.typeInfo[typeName]; isUserDefined {
			// It's a type we've defined in this package, so just use its name.
			return typeName
		}

		// Convert to uppercase to match standard IEC types (e.g., 'int' -> 'INT').
		iecType := strings.ToUpper(typeName)
		// It's a standard built-in type, so prefix with the 'iec' package.
		// A more advanced implementation might have a lookup table.
		return "iec." + iecType
	}
	if arrayDef, ok := dataType.(*ast.ArrayDefinition); ok {
		// Recursively call mapIecTypeToGo for the element type.
		// This correctly handles multi-dimensional arrays (e.g., ARRAY OF ARRAY OF INT).
		elemType := t.mapIecTypeToGo(arrayDef.DataType)
		return "[]" + elemType
	}

	// Handle more complex types like ARRAY or STRUCT here in the future.
	log.Printf("Warning: Unhandled data type expression in transpiler: %s", dataType.String())
	return "any /* unhandled type */"
}

func (t *Transpiler) transpileInfixExpression(exp *ast.InfixExpression) error {
	t.write("(")
	t.transpileExpression(exp.Left) // Errors are handled inside

	// Map ST operators to Go operators
	op := exp.Operator
	switch strings.ToUpper(op) {
	case "AND":
		op = "&&"
	case "OR":
		op = "||"
	case "NOT":
		op = "!"
	case "<>":
		op = "!="
	case "=":
		op = "=="
	}

	t.write(" %s ", op)
	t.transpileExpression(exp.Right) // Errors are handled inside
	t.write(")")
	return nil
}

func (t *Transpiler) transpileIfStatement(stmt *ast.IfStatement) error {
	t.write("if ")
	t.transpileExpression(stmt.Condition) // A function to convert expressions
	t.write(" {\n")
	t.transpileBlockStatement(stmt.Consequence)
	t.write("}")

	// Handle ELSIF and ELSE
	alt := stmt.Alternative
	for alt != nil {
		if elseifStmt, ok := alt.(*ast.IfStatement); ok {
			t.write(" else if ")
			t.transpileExpression(elseifStmt.Condition)
			t.write(" {\n")
			t.transpileBlockStatement(elseifStmt.Consequence)
			t.write("}")
			alt = elseifStmt.Alternative
		} else { // This is the final ELSE block
			t.write(" else {\n")
			t.transpileBlockStatement(alt.(*ast.BlockStatement))
			t.write("}")
			alt = nil
		}
	}
	t.write("\n")
	return nil
}

func (t *Transpiler) transpileTypedLiteral(lit *ast.TypedLiteral) error {
	// Map IEC type to Go type from royaljelly
	goType := strings.ToUpper(lit.TypeName) // e.g., "LINT"

	// The value is already a string from the parser
	valueStr := lit.Value.String()

	// Handle TIME literals specifically
	if goType == "TIME" || goType == "T" {
		// The parser gives us the full literal string, e.g., "T#5s"
		// We need to wrap it in quotes for the Go code.
		t.write("iec.MakeTime(%q)", lit.String())
	} else if goType == "STRING" {
		// Ensure string literals are properly quoted in Go
		t.write("%q", valueStr)
	} else {
		t.write("iec.%s(%s)", goType, valueStr)
	}
	return nil
}

func (t *Transpiler) transpilePrefixExpression(exp *ast.PrefixExpression) error {
	t.write("(")
	op := exp.Operator
	if strings.ToUpper(op) == "NOT" {
		op = "!"
	}
	t.write(op)
	if err := t.transpileExpression(exp.Right); err != nil {
		return err
	}
	t.write(")")
	return nil
}

func (t *Transpiler) transpileCallExpression(exp *ast.CallExpression) error {
	// A call expression can be a standard function (e.g., SIN(X)) or a Function Block invocation (e.g., MyTimer(IN:=...)).
	// We'll treat calls with named arguments as potential FB calls.
	isLikelyFB := false
	if len(exp.Arguments) > 0 {
		if _, ok := exp.Arguments[0].(*ast.NamedArgument); ok {
			isLikelyFB = true
		}
	}

	if !isLikelyFB {
		// Standard function call like SIN(X)
		return t.transpileStandardFunctionCall(exp)
	}

	// --- Function Block Invocation ---
	// 1. Set the input parameters.
	for _, arg := range exp.Arguments {
		if namedArg, ok := arg.(*ast.NamedArgument); ok {
			isInOut := t.isInOutArgument(exp.Function, namedArg.Name.Value)

			if isInOut {
				// Pass by reference for VAR_IN_OUT
				// Transpiles to: p.MyFB.InOutVar = &p.MyProgramVar
				t.transpileExpression(exp.Function)
				t.write(".%s = &", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			} else {
				// Pass by value for VAR_INPUT
				// Transpiles to: p.MyTimer.IN = ...
				t.transpileExpression(exp.Function)
				t.write(".%s = ", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			}
		}
	}

	// 2. Call the Logic() method.
	t.transpileExpression(exp.Function) // e.g., p.MyTimer
	t.write(".Logic(now)\n")             // Pass the 'now' timestamp

	// 3. Handle output arguments (e.g., Q => MyVar)
	for _, arg := range exp.Arguments {
		if outArg, ok := arg.(*ast.OutputArgument); ok {
			// Transpile `p.MyVar = p.MyTimer.Q`
			t.transpileExpression(outArg.Target)
			t.write(" = ")
			t.transpileExpression(exp.Function)
			t.write(".%s\n", outArg.Source.Value)
		}
	}

	return nil
}

// isInOutArgument checks the symbol table to determine if an argument for a
// given function block corresponds to a VAR_IN_OUT parameter.
func (t *Transpiler) isInOutArgument(fbExpr ast.Expression, argName string) bool {
	// This is a simplified check. A robust implementation would resolve the type
	// of fbExpr more accurately. Here, we assume it's an identifier.
	if fbIdent, ok := fbExpr.(*ast.Identifier); ok {
		// Look up the FB definition in our type info map.
		if fbDefNode, ok := t.typeInfo[fbIdent.Value]; ok {
			// Check if it's a FUNCTION, which can also have VAR_IN_OUT
			if fDef, ok := fbDefNode.(*ast.FunctionDeclaration); ok {
				for _, inOut := range fDef.VarInOuts {
					if inOut.Name.Value == argName {
						return true
					}
				}
			}
			if fbDef, ok := fbDefNode.(*ast.FunctionBlockDeclaration); ok {
				// Check if the argument name exists in the VarInOuts list.
				for _, inOut := range fbDef.VarInOuts {
					if inOut.Name.Value == argName {
						return true
					}
				}
			}
		}
	}
	// This could also happen if we are calling a FB instance that is a member of another struct.
	// e.g. MyStruct.MyTimer(....)
	return false
}

func (t *Transpiler) transpileStandardFunctionCall(exp *ast.CallExpression) error {
	if err := t.transpileExpression(exp.Function); err != nil {
		return err
	}
	t.write("(")
	for i, arg := range exp.Arguments {
		if i > 0 {
			t.write(", ")
		}
		if err := t.transpileExpression(arg); err != nil {
			return err
		}
	}
	t.write(")")
	return nil
}

func (t *Transpiler) transpileEnumDefinition(name *ast.Identifier, enum *ast.EnumDefinition) {
	typeName := name.Value
	t.write("type %s int\n\n", typeName)
	t.write("const (\n")
	for i, val := range enum.Values {
		// Convention: TypeName_ValueName
		constName := fmt.Sprintf("%s_%s", typeName, val.Value)
		if i == 0 {
			t.write("\t%s %s = iota\n", constName, typeName)
		} else {
			t.write("\t%s\n", constName)
		}
	}
	t.write(")\n\n")
}

func (t *Transpiler) transpileEnumeratedValueLiteral(evl *ast.EnumeratedValueLiteral) error {
	t.write("%s_%s", evl.TypeName.Value, evl.Value.Value)
	return nil
}

func (t *Transpiler) transpileArrayLiteral(al *ast.ArrayLiteral) error {
	// We need to infer the type of the array's elements to generate the correct Go literal.
	// We'll inspect the first element. This is a simplification.
	elemType := ""
	if len(al.Elements) > 0 {
		firstElem := al.Elements[0]
		// If the first element is another array, it's a multi-dimensional array.
		// In this case, we don't specify the inner type, letting it be inferred recursively.
		if _, isArray := firstElem.(*ast.ArrayLiteral); isArray {
			// This will result in `[][]...{...}` which is what we want.
			elemType = ""
		} else if typedLit, ok := firstElem.(*ast.TypedLiteral); ok {
			elemType = "iec." + strings.ToUpper(typedLit.TypeName)
			// For other literals, we let Go infer the type during assignment.
		}
	}

	t.write("[]%s{", elemType)
	for i, el := range al.Elements {
		if i > 0 {
			t.write(", ")
		}
		// Handle ArrayRepetition here
		t.transpileExpression(el)
		if i < len(al.Elements)-1 {
			t.write(", ")
		}
	}
	t.write("}")
	return nil
}

func (t *Transpiler) transpileArrayRepetition(ar *ast.ArrayRepetition) error {
	// We need to evaluate the factor to know how many times to repeat.
	// This is a simplification; a full implementation would need to evaluate the expression.
	// For now, we assume it's an IntegerLiteral.
	factor, ok := ar.Factor.(*ast.IntegerLiteral)
	if !ok {
		return fmt.Errorf("array repetition factor must be an integer literal for transpilation")
	}

	for i := 0; i < int(factor.Value); i++ {
		for j, el := range ar.Elements {
			t.transpileExpression(el)
			if j < len(ar.Elements)-1 || i < int(factor.Value)-1 {
				t.write(", ")
			}
		}
	}

	return nil
}

func (t *Transpiler) transpileMemberAccessExpression(exp *ast.MemberAccessExpression) error {
	if err := t.transpileExpression(exp.Struct); err != nil {
		return err
	}
	t.write(".%s", exp.Member.Value)
	return nil
}

func (t *Transpiler) transpileIndexExpression(exp *ast.IndexExpression) error {
	if err := t.transpileExpression(exp.Left); err != nil {
		return err
	}
	t.write("[")
	if err := t.transpileExpression(exp.Index); err != nil {
		return err
	}
	t.write("]")
	return nil
}
			// Execute the bodies of all active actions for this step.
			for _, actionBlock := range step.Actions {
				// Execute the action's code.
				t.transpileBlockStatement(actionBlock.Body) // Simplified for now
			}
			t.write("\t}\n")
		}
	}

	return nil
}

func (t *Transpiler) transpileFunctionBlockDeclaration(fb *ast.FunctionBlockDeclaration) error {
	// 1. Generate the struct for the Function Block.
	t.write("// %s is the transpiled struct for the FUNCTION_BLOCK of the same name.\n", fb.Name.Value)

	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	t.buildVarInfo(fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars)

	originalAccessVars := t.accessVars
	t.accessVars = make(map[string]bool)
	t.buildAccessVarInfo(fb.AccessVars)

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(fb.VarInputs, fb.VarOutputs, fb.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.accessVars = originalAccessVars }()

	defer func() { t.varInfo = originalVarInfo }()

	t.write("type %s struct {\n", fb.Name.Value)

	// Transpile VAR_INPUT, VAR_OUTPUT, and VAR into struct fields.
	for _, varDecl := range fb.VarInputs {
		t.transpileVarDecl(varDecl)
	}
	for _, varDecl := range fb.VarOutputs {
		t.transpileVarDecl(varDecl)
	}
	for _, varDecl := range fb.VarInOuts {
		t.transpileVarDeclInOut(varDecl)
	}
	for _, varDecl := range fb.Vars {
		t.transpileVarDecl(varDecl)
	}
	for _, accessDecl := range fb.AccessVars {
		t.transpileVarAccess(accessDecl)
	}

	t.write("}\n\n")

	// 2. Generate the Logic method for the Function Block.
	// The receiver name is specific to this FB.
	receiverName := strings.ToLower(fb.Name.Value[:1])
	originalProgramVarName := t.programVarName
	t.programVarName = receiverName // Set context for transpiling the body

	t.write("// Logic executes the logic for the %s FUNCTION_BLOCK.\n", fb.Name.Value)
	t.write("func (%s *%s) Logic() {\n", receiverName, fb.Name.Value)
	if err := t.Transpile(fb.Body); err != nil {
		t.programVarName = originalProgramVarName // Restore context on error
		return err
	}
	t.write("}\n\n")

	t.programVarName = originalProgramVarName // Restore context
	return nil
}

func (t *Transpiler) isTransitionFromStep(trans *ast.TransitionStatement, stepName string) bool {
	for _, from := range trans.From {
		if from.Value == stepName {
			return true
		}
	}
	return false
}

func (t *Transpiler) transpileSFCTransition(trans *ast.TransitionStatement) {
	t.write("\t\t// Transition from %s to %s\n", trans.From[0].Value, trans.To[0].Value)
	t.write("\t\tif ")
	t.transpileExpression(trans.Condition)
	t.write(" {\n")
	// Deactivate source steps and activate destination steps
	for _, from := range trans.From {
		t.write("\t\t\tdelete(nextActiveSteps, %q)\n", from.Value)
	}
	for _, to := range trans.To {
		t.write("\t\t\tnextActiveSteps[%q] = true\n", to.Value)
	}
	t.write("\t\t}\n")
}

func (t *Transpiler) buildVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			// We need to resolve the type declaration for this variable.
			var typeName string
			currentType := varDecl.DataType
			// For arrays, we need to get the base element type.
			for {
				if arrayDef, ok := currentType.(*ast.ArrayDefinition); ok {
					currentType = arrayDef.DataType
				} else {
					break
				}
			}
			if typeIdent, ok := currentType.(*ast.Identifier); ok {
				typeName = typeIdent.Value
			}
			t.varInfo[varDecl.Name.Value] = t.getTypeDeclaration(typeName)
		}
	}
}

func (t *Transpiler) buildLocatedVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			if varDecl.Location != nil {
				t.locatedVars[varDecl.Name.Value] = true
			}
		}
	}
}

func (t *Transpiler) buildAccessVarInfo(accessBlocks []*ast.VarAccessDeclaration) {
	for _, block := range accessBlocks {
		// This is a simplified version. A real implementation would have a list of declarations inside the block.
		// Assuming the parser provides a flat list for now.
		// for _, decl := range block.Declarations {
		// 	t.accessVars[decl.LocalName.Value] = true
		// }
	}
}

func (t *Transpiler) transpileFunctionDeclaration(fd *ast.FunctionDeclaration) error {
	// Set context for the duration of this function's transpilation
	originalFunc := t.currentFunc
	t.currentFunc = fd
	defer func() { t.currentFunc = originalFunc }()

	// --- 1. Build the function signature ---
	t.write("func %s(", fd.Name.Value)

	// Parameters (VAR_INPUT, VAR_IN_OUT)
	params := []string{}
	for _, p := range fd.VarInputs {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
	}
	for _, p := range fd.VarInOuts {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s *%s", p.Name.Value, goType))
	}
	t.write(strings.Join(params, ", "))
	t.write(") ")

	// Return values (primary return type + VAR_OUTPUT)
	returns := []string{}
	if fd.ReturnType != nil {
		returns = append(returns, t.mapIecTypeToGo(fd.ReturnType))
	}
	for _, p := range fd.VarOutputs {
		returns = append(returns, t.mapIecTypeToGo(p.DataType))
	}
	if len(returns) > 1 {
		t.write("(%s)", strings.Join(returns, ", "))
	} else if len(returns) == 1 {
		t.write(returns[0])
	}

	t.write(" {\n")

	// --- 2. Transpile local variables (VAR) ---
	for _, v := range fd.Vars {
		t.write("\tvar %s %s\n", v.Name.Value, t.mapIecTypeToGo(v.DataType))
	}
	t.write("\n")

	// --- 3. Transpile the function body ---
	if err := t.Transpile(fd.Body); err != nil {
		return err
	}

	t.write("}\n\n")
	return nil
}

func (t *Transpiler) transpileVarDecl(varDecl *ast.VarDeclStatement) {
	// Get the variable name.
	// If it's a located variable, transpile it as a pointer.
	if varDecl.Location != nil {
		goType := t.mapIecTypeToGo(varDecl.DataType)
		t.write("\t%s *%s // AT %s\n", varDecl.Name.Value, goType, varDecl.Location.String())
		return
	}

	name := varDecl.Name.Value

	// Get the Go type for the IEC data type.
	goType := t.mapIecTypeToGo(varDecl.DataType)

	// Write the struct field. e.g., "MyCounter iec.LINT"
	t.write("\t%s %s\n", name, goType)

}

func (t *Transpiler) transpileVarAccess(accessDecl *ast.VarAccessDeclaration) {
	localName := accessDecl.LocalName.Value

	// Infer the type of the target variable.
	targetTypeDecl := t.resolveAssignmentTargetType(accessDecl.AccessPath)
	if targetTypeDecl == nil {
		log.Printf("Warning: Could not resolve type for VAR_ACCESS target: %s", accessDecl.AccessPath.String())
		t.write("\t%s *any // Could not resolve type for %s\n", localName, accessDecl.AccessPath.String())
		return
	}

	// Get the Go type and declare the field as a pointer to that type.
	goType := t.mapIecTypeToGo(targetTypeDecl.DataType)
	t.write("\t%s *%s\n", localName, goType)
}

func (t *Transpiler) LinkMethodBody(prog *ast.ProgramDeclaration) string {
	// This is a placeholder for the logic that would be generated inside the Link method.
	return "// Runtime linking logic will be placed here by the royaljelly scheduler.\n"
}

// transpileVarDeclInOut transpiles a VAR_IN_OUT declaration to a pointer field.
func (t *Transpiler) transpileVarDeclInOut(varDecl *ast.VarDeclStatement) {
	name := varDecl.Name.Value
	goType := t.mapIecTypeToGo(varDecl.DataType)
	// Write the struct field as a pointer, e.g., "MyVar *iec.INT"
	t.write("\t%s *%s\n", name, goType)

}

// transpileCaseStatement transpiles an IEC 61131-3 CASE statement to a Go switch statement.
func (t *Transpiler) transpileCaseStatement(stmt *ast.CaseStatement) error {
	t.write("switch ")
	if err := t.transpileExpression(stmt.Expression); err != nil {
		return err
	}
	t.write(" {\n")

	for _, caseElem := range stmt.Cases {
		t.write("case ")
		for i, expr := range caseElem.Values {
			if i > 0 {
				t.write(", ")
			}
			if err := t.transpileExpression(expr); err != nil {
				return err
			}
		}
		t.write(":\n")
		if err := t.transpileBlockStatement(caseElem.Consequence); err != nil {
			return err
		}
	}

	if stmt.Alternative != nil {
		t.write("default:\n")
		if err := t.transpileBlockStatement(stmt.Alternative); err != nil {
			return err
		}
	}
	t.write("}\n")
	return nil
}

// transpileAssignmentStatement transpiles an IEC `:=` assignment to a Go `=` assignment.
func (t *Transpiler) transpileAssignmentStatement(stmt *ast.AssignmentStatement) error {
	// Special case: Assignment to the function name is a return statement.
	if ident, ok := stmt.Left.(*ast.Identifier); ok && t.currentFunc != nil && ident.Value == t.currentFunc.Name.Value {
		t.write("return ")
		if err := t.transpileExpression(stmt.Value); err != nil {
			return err
		}
		t.write("\n")
		return nil
	}

	// Check if the left-hand side resolves to a subrange type.
	if typeDecl := t.resolveAssignmentTargetType(stmt.Left); typeDecl != nil && typeDecl.Subrange != nil {
		return t.transpileSubrangeAssignment(stmt, typeDecl)
	}

	// Standard assignment
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")
	if err := t.transpileExpression(stmt.Value); err != nil {
		return err
	}
	t.write("\n")
	return nil
}

// resolveAssignmentTargetType recursively finds the TypeDeclaration for the target of an assignment.
// It can handle simple variables, array elements, and struct members.
func (t *Transpiler) resolveAssignmentTargetType(expr ast.Expression) *ast.TypeDeclaration {
	switch e := expr.(type) {
	case *ast.Identifier:
		// Base case: a simple variable. Look it up in the current scope's varInfo.
		return t.varInfo[e.Value]

	case *ast.IndexExpression:
		// It's an array element. The type of the element is the type of the array's base variable.
		// Recursively resolve the type of the array itself.
		return t.resolveAssignmentTargetType(e.Left)

	case *ast.MemberAccessExpression:
		// It's a struct field. We need to find the type of the struct, then the type of the field.
		structVarType := t.resolveAssignmentTargetType(e.Struct)
		if structVarType == nil {
			return nil // Can't resolve the struct's type.
		}

		// The struct's type definition should be in the main typeInfo map.
		structDefNode, ok := t.typeInfo[structVarType.Name.Value]
		if !ok {
			return nil
		}

		// Now, find the member's type declaration within the struct's definition.
		return t.findMemberType(structDefNode, e.Member.Value)
	}
	return nil
}

func (t *Transpiler) getTypeDeclaration(typeName string) *ast.TypeDeclaration {
	if typeDef, ok := t.typeInfo[typeName]; ok {
		if typeDecl, ok := typeDef.(*ast.TypeDeclaration); ok {
			return typeDecl
		}
	}
	return nil
}

// findMemberType looks inside a STRUCT definition for a specific member and returns its type declaration.
func (t *Transpiler) findMemberType(structDefNode ast.Node, memberName string) *ast.TypeDeclaration {
	typeDecl, ok := structDefNode.(*ast.TypeDeclaration)
	if !ok {
		return nil
	}
	structDef, ok := typeDecl.DataType.(*ast.StructDefinition)
	if !ok {
		return nil
	}

	// This is a simplified lookup. A real implementation would build a symbol table for the struct members.
	return t.getTypeDeclaration(structDef.GetMemberType(memberName))
}

func (t *Transpiler) transpileSubrangeAssignment(stmt *ast.AssignmentStatement, typeDecl *ast.TypeDeclaration) error {
	// Transpile the left side of the assignment (the variable).
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")

	// Get the base type to select the correct Clamp function (e.g., "INT" -> "ClampINT").
	baseType := strings.ToUpper(typeDecl.DataType.String())
	clampFunc := "iec.Clamp" + baseType

	// Get the subrange bounds.
	subrange, ok := typeDecl.Subrange.(*ast.InfixExpression)
	if !ok || subrange.Operator != ".." {
		return fmt.Errorf("invalid subrange definition for type %s", typeDecl.Name.Value)
	}

	// Write the call to the clamp function: iec.ClampINT(value, min, max)
	t.write("%s(", clampFunc)
	if err := t.transpileExpression(stmt.Value); err != nil {
		return err
	}
	t.write(", %s, %s)\n", subrange.Left.String(), subrange.Right.String())

	return nil
}

// transpileForLoopStatement transpiles a FOR loop to a Go `for` loop.
func (t *Transpiler) transpileForLoopStatement(stmt *ast.ForLoopStatement) error {
	// The control variable is an assignment statement itself.
	t.write("for ")
	t.transpileExpression(stmt.ControlVar.Left)
	t.write(" := ")
	t.transpileExpression(stmt.ControlVar.Value)
	t.write("; ")
	t.transpileExpression(stmt.ControlVar.Left)
	t.write(" <= ")
	t.transpileExpression(stmt.EndValue)
	t.write("; ")
	t.transpileExpression(stmt.ControlVar.Left)
	if stmt.StepValue != nil {
		t.write(" += ")
		t.transpileExpression(stmt.StepValue)
	} else {
		t.write("++")
	}
	t.write(" {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("}\n")
	return nil
}

// transpileBlockStatement iterates over statements in a block and transpiles them.
func (t *Transpiler) transpileBlockStatement(bs *ast.BlockStatement) error {
	for _, stmt := range bs.Statements {
		if err := t.Transpile(stmt); err != nil {
			return err
		}
	}
	return nil
}

// transpileWhileStatement transpiles a WHILE loop to a Go `for` loop.
func (t *Transpiler) transpileWhileStatement(stmt *ast.WhileStatement) error {
	t.write("for ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("}\n")
	return nil
}

// transpileRepeatStatement transpiles a REPEAT...UNTIL loop to a Go `for` loop.
func (t *Transpiler) transpileRepeatStatement(stmt *ast.RepeatStatement) error {
	t.write("for {\n")
	if err := t.transpileBlockStatement(stmt.Body); err != nil {
		return err
	}
	t.write("if ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" { break }\n")
	t.write("}\n")
	return nil
}

// transpileReturnStatement transpiles a RETURN statement.
func (t *Transpiler) transpileReturnStatement(stmt *ast.ReturnStatement) error {
	t.write("return")
	if stmt.ReturnValue != nil {
		t.write(" ")
		if err := t.transpileExpression(stmt.ReturnValue); err != nil {
			return err
		}
	}
	t.write("\n")
	return nil
}

// transpileExitStatement transpiles an EXIT statement to a `break`.
func (t *Transpiler) transpileExitStatement(stmt *ast.ExitStatement) error {
	t.write("break\n")
	return nil
}

// transpileExpression dispatches to specific handlers for different expression types.
func (t *Transpiler) transpileExpression(exp ast.Expression) error {
	switch exp := exp.(type) {
	case *ast.Identifier:
		// If it's a located or access variable, it's a pointer and must be dereferenced.
		if t.locatedVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.accessVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.globalVars[exp.Value] {
			// If it's a global variable, write it without a prefix.
			if t.accessVars[exp.Value] {
				t.write("(*%s.%s)", t.programVarName, exp.Value)
			} else if t.globalVars[exp.Value] {
				// If it's a global variable, write it without a prefix.
				t.write("%s", exp.Value)
			} else if t.programVarName != "" {
				// Otherwise, it's a local/member variable, so prefix it with the receiver.
				t.write("%s.%s", t.programVarName, exp.Value)
			} else {
				t.write("%s", exp.Value)
			}
		}
	case *ast.IntegerLiteral:
		t.write("%d", exp.Value)
	case *ast.UnsignedIntegerLiteral:
		t.write("%d", exp.Value)
	case *ast.BitStringLiteral:
		// Assuming a helper in iec package to create bitstrings
		t.write("iec.MakeBitString(%d, %d)", exp.Value, exp.Width)
	case *ast.LRealLiteral:
		t.write("%f", exp.Value)
	case *ast.WStringLiteral:
		t.write("%q", exp.Value)
	case *ast.DateLiteral:
		// Assuming an iec helper `iec.MakeDate("D#...")`
		t.write("iec.MakeDate(%q)", exp.Value)
	case *ast.TimeOfDayLiteral:
		// Assuming an iec helper `iec.MakeTOD("TOD#...")`
		t.write("iec.MakeTOD(%q)", exp.Value)
	case *ast.DateAndTimeLiteral:
		// Assuming an iec helper `iec.MakeDT("DT#...")`
		t.write("iec.MakeDT(%q)", exp.Value)
	case *ast.Boolean:
		t.write("%t", exp.Value)
	case *ast.RealLiteral:
		t.write("%f", exp.Value) // This might need more careful formatting for IEC REAL
	case *ast.StringLiteral:
		t.write("%q", exp.Value)
	case *ast.TimeLiteral:
		// Assuming an iec helper `iec.MakeTime("T#5s")`
		t.write("iec.MakeTime(\"%s\")", exp.Value)
	case *ast.TypedLiteral:
		return t.transpileTypedLiteral(exp)
	case *ast.InfixExpression:
		return t.transpileInfixExpression(exp)
	case *ast.PrefixExpression:
		return t.transpilePrefixExpression(exp)
	case *ast.CallExpression:
		return t.transpileCallExpression(exp)
	case *ast.MemberAccessExpression:
		return t.transpileMemberAccessExpression(exp)
	case *ast.IndexExpression:
		return t.transpileIndexExpression(exp)
	case *ast.EnumeratedValueLiteral:
		return t.transpileEnumeratedValueLiteral(exp)
	case *ast.ArrayLiteral:
		return t.transpileArrayLiteral(exp)
	case *ast.ArrayRepetition:
		return t.transpileArrayRepetition(exp)
	case *ast.IlInstructionStatement, *ast.StepStatement, *ast.TransitionStatement, *ast.ActionStatement, *ast.ActionBlockStatement:
		// These are handled by their parent SFC/IL transpilers, so we can ignore them here.
		// For IL, the instruction is handled directly by transpileIlProgram.
		return nil

	default:
		log.Printf("Warning: Unhandled expression type in transpiler: %T", exp)
		t.write("/* unhandled_expression: %T */", exp)
	}
	return nil
}

func (t *Transpiler) transpileTypeBlockDeclaration(tbd *ast.TypeBlockDeclaration) error {
	for _, decl := range tbd.Declarations {
		// Check if the type declaration is for a STRUCT
		if structDef, ok := decl.DataType.(*ast.StructDefinition); ok {
			t.write("// %s is the transpiled struct for the user-defined type.\n", decl.Name.Value)
			t.write("type %s struct {\n", decl.Name.Value)
			for _, member := range structDef.Members {
				t.transpileVarDecl(member)
			}
			t.write("}\n\n")
		} else if enumDef, ok := decl.DataType.(*ast.EnumDefinition); ok {
			// Handle ENUM definitions
			t.transpileEnumDefinition(decl.Name, enumDef)
		} else if decl.Subrange != nil {
			// Handle Subrange types
			baseType := t.mapIecTypeToGo(decl.DataType)
			t.write("// %s is a subrange of %s.\n", decl.Name.Value, baseType)
			t.write("type %s %s\n\n", decl.Name.Value, baseType)
			// Note: Runtime range checks are not added by the transpiler at this stage.
		}
	}
	return nil
}

// mapIecTypeToGo converts an AST expression representing an IEC type
// into the corresponding Go type string from the `iec` package.
func (t *Transpiler) mapIecTypeToGo(dataType ast.Expression) string {
	// The DataType is an expression, which for simple types is an Identifier.
	if ident, ok := dataType.(*ast.Identifier); ok { // e.g., INT, or MyStruct
		typeName := ident.Value
		// Check if it's a user-defined type (STRUCT, ENUM, etc.) we've registered.
		if _, isUserDefined := t.typeInfo[typeName]; isUserDefined {
			// It's a type we've defined in this package, so just use its name.
			return typeName
		}

		// Convert to uppercase to match standard IEC types (e.g., 'int' -> 'INT').
		iecType := strings.ToUpper(typeName)
		// It's a standard built-in type, so prefix with the 'iec' package.
		// A more advanced implementation might have a lookup table.
		return "iec." + iecType
	}
	if arrayDef, ok := dataType.(*ast.ArrayDefinition); ok {
		// Recursively call mapIecTypeToGo for the element type.
		// This correctly handles multi-dimensional arrays (e.g., ARRAY OF ARRAY OF INT).
		elemType := t.mapIecTypeToGo(arrayDef.DataType)
		return "[]" + elemType
	}

	// Handle more complex types like ARRAY or STRUCT here in the future.
	log.Printf("Warning: Unhandled data type expression in transpiler: %s", dataType.String())
	return "any /* unhandled type */"
}

func (t *Transpiler) transpileInfixExpression(exp *ast.InfixExpression) error {
	t.write("(")
	t.transpileExpression(exp.Left) // Errors are handled inside

	// Map ST operators to Go operators
	op := exp.Operator
	switch strings.ToUpper(op) {
	case "AND":
		op = "&&"
	case "OR":
		op = "||"
	case "NOT":
		op = "!"
	case "<>":
		op = "!="
	case "=":
		op = "=="
	}

	t.write(" %s ", op)
	t.transpileExpression(exp.Right) // Errors are handled inside
	t.write(")")
	return nil
}

func (t *Transpiler) transpileIfStatement(stmt *ast.IfStatement) error {
	t.write("if ")
	t.transpileExpression(stmt.Condition) // A function to convert expressions
	t.write(" {\n")
	t.transpileBlockStatement(stmt.Consequence)
	t.write("}")

	// Handle ELSIF and ELSE
	alt := stmt.Alternative
	for alt != nil {
		if elseifStmt, ok := alt.(*ast.IfStatement); ok {
			t.write(" else if ")
			t.transpileExpression(elseifStmt.Condition)
			t.write(" {\n")
			t.transpileBlockStatement(elseifStmt.Consequence)
			t.write("}")
			alt = elseifStmt.Alternative
		} else { // This is the final ELSE block
			t.write(" else {\n")
			t.transpileBlockStatement(alt.(*ast.BlockStatement))
			t.write("}")
			alt = nil
		}
	}
	t.write("\n")
	return nil
}

func (t *Transpiler) transpileTypedLiteral(lit *ast.TypedLiteral) error {
	// Map IEC type to Go type from royaljelly
	goType := strings.ToUpper(lit.TypeName) // e.g., "LINT"

	// The value is already a string from the parser
	valueStr := lit.Value.String()

	// Handle TIME literals specifically
	if goType == "TIME" || goType == "T" {
		// The parser gives us the full literal string, e.g., "T#5s"
		// We need to wrap it in quotes for the Go code.
		t.write("iec.MakeTime(%q)", lit.String())
	} else if goType == "STRING" {
		// Ensure string literals are properly quoted in Go
		t.write("%q", valueStr)
	} else {
		t.write("iec.%s(%s)", goType, valueStr)
	}
	return nil
}

func (t *Transpiler) transpilePrefixExpression(exp *ast.PrefixExpression) error {
	t.write("(")
	op := exp.Operator
	if strings.ToUpper(op) == "NOT" {
		op = "!"
	}
	t.write(op)
	if err := t.transpileExpression(exp.Right); err != nil {
		return err
	}
	t.write(")")
	return nil
}

func (t *Transpiler) transpileCallExpression(exp *ast.CallExpression) error {
	// A call expression can be a standard function (e.g., SIN(X)) or a Function Block invocation (e.g., MyTimer(IN:=...)).
	// We'll treat calls with named arguments as potential FB calls.
	isLikelyFB := false
	if len(exp.Arguments) > 0 {
		if _, ok := exp.Arguments[0].(*ast.NamedArgument); ok {
			isLikelyFB = true
		}
	}

	if !isLikelyFB {
		// Standard function call like SIN(X)
		return t.transpileStandardFunctionCall(exp)
	}

	// --- Function Block Invocation ---
	// 1. Set the input parameters.
	for _, arg := range exp.Arguments {
		if namedArg, ok := arg.(*ast.NamedArgument); ok {
			isInOut := t.isInOutArgument(exp.Function, namedArg.Name.Value)

			if isInOut {
				// Pass by reference for VAR_IN_OUT
				// Transpiles to: p.MyFB.InOutVar = &p.MyProgramVar
				t.transpileExpression(exp.Function)
				t.write(".%s = &", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			} else {
				// Pass by value for VAR_INPUT
				// Transpiles to: p.MyTimer.IN = ...
				t.transpileExpression(exp.Function)
				t.write(".%s = ", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			}
		}
	}

	// 2. Call the Logic() method.
	t.transpileExpression(exp.Function)
	t.write(".Logic()\n")

	// 3. Handle output arguments (e.g., Q => MyVar)
	for _, arg := range exp.Arguments {
		if outArg, ok := arg.(*ast.OutputArgument); ok {
			// Transpile `p.MyVar = p.MyTimer.Q`
			t.transpileExpression(outArg.Target)
			t.write(" = ")
			t.transpileExpression(exp.Function)
			t.write(".%s\n", outArg.Source.Value)
		}
	}

	return nil
}

func (t *Transpiler) transpileFBInputAssignments(exp *ast.CallExpression) {
	for _, arg := range exp.Arguments {
		if namedArg, ok := arg.(*ast.NamedArgument); ok {
			isInOut := t.isInOutArgument(exp.Function, namedArg.Name.Value)

			if isInOut {
				// Pass by reference for VAR_IN_OUT
				t.transpileExpression(exp.Function)
				t.write(".%s = &", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			} else {
				// Pass by value for VAR_INPUT
				t.transpileExpression(exp.Function)
				t.write(".%s = ", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			}
		}
	}

	// After setting inputs, call the Logic method.
	t.transpileExpression(exp.Function) // e.g., p.MyTimer
	t.write(".Logic(now)\n")             // Pass the 'now' timestamp
}

// isInOutArgument checks the symbol table to determine if an argument for a
// given function block corresponds to a VAR_IN_OUT parameter.
func (t *Transpiler) isInOutArgument(fbExpr ast.Expression, argName string) bool {
	// This is a simplified check. A robust implementation would resolve the type
	// of fbExpr more accurately. Here, we assume it's an identifier.
	if fbIdent, ok := fbExpr.(*ast.Identifier); ok {
		// Look up the FB definition in our type info map.
		if fbDefNode, ok := t.typeInfo[fbIdent.Value]; ok {
			// Check if it's a FUNCTION, which can also have VAR_IN_OUT
			if fDef, ok := fbDefNode.(*ast.FunctionDeclaration); ok {
				for _, inOut := range fDef.VarInOuts {
					if inOut.Name.Value == argName {
						return true
					}
				}
			}
			if fbDef, ok := fbDefNode.(*ast.FunctionBlockDeclaration); ok {
				// Check if the argument name exists in the VarInOuts list.
				for _, inOut := range fbDef.VarInOuts {
					if inOut.Name.Value == argName {
						return true
					}
				}
			}
		}
	}
	// This could also happen if we are calling a FB instance that is a member of another struct.
	// e.g. MyStruct.MyTimer(....)
	return false
}

func (t *Transpiler) getFunctionBlockDefinition(fbExpr ast.Expression) *ast.FunctionBlockDeclaration {
	// This is a simplified check. A robust implementation would resolve the type
	// of fbExpr more accurately. Here, we assume it's an identifier that refers to a variable.
	if fbIdent, ok := fbExpr.(*ast.Identifier); ok {
		// Look up the variable's type declaration
		if varTypeDecl := t.varInfo[fbIdent.Value]; varTypeDecl != nil {
			// Now look up the type definition itself in the main typeInfo map.
			if fbDefNode, ok := t.typeInfo[varTypeDecl.Name.Value]; ok {
				if fbDef, isFB := fbDefNode.(*ast.FunctionBlockDeclaration); isFB {
					return fbDef
				}
			}
		}
	}
	return nil
}

func (t *Transpiler) transpileStandardFunctionCall(exp *ast.CallExpression) error {
	if err := t.transpileExpression(exp.Function); err != nil {
		return err
	}
	t.write("(")
	for i, arg := range exp.Arguments {
		if i > 0 {
			t.write(", ")
		}
		if err := t.transpileExpression(arg); err != nil {
			return err
		}
	}
	t.write(")")
	return nil
}

func (t *Transpiler) transpileEnumDefinition(name *ast.Identifier, enum *ast.EnumDefinition) {
	typeName := name.Value
	t.write("type %s int\n\n", typeName)
	t.write("const (\n")
	for i, val := range enum.Values {
		// Convention: TypeName_ValueName
		constName := fmt.Sprintf("%s_%s", typeName, val.Value)
		if i == 0 {
			t.write("\t%s %s = iota\n", constName, typeName)
		} else {
			t.write("\t%s\n", constName)
		}
	}
	t.write(")\n\n")
}

func (t *Transpiler) transpileEnumeratedValueLiteral(evl *ast.EnumeratedValueLiteral) error {
	t.write("%s_%s", evl.TypeName.Value, evl.Value.Value)
	return nil
}

func (t *Transpiler) transpileArrayLiteral(al *ast.ArrayLiteral) error {
	// We need to infer the type of the array's elements to generate the correct Go literal.
	// We'll inspect the first element. This is a simplification.
	elemType := ""
	if len(al.Elements) > 0 {
		firstElem := al.Elements[0]
		// If the first element is another array, it's a multi-dimensional array.
		// In this case, we don't specify the inner type, letting it be inferred recursively.
		if _, isArray := firstElem.(*ast.ArrayLiteral); isArray {
			// This will result in `[][]...{...}` which is what we want.
			elemType = ""
		} else if typedLit, ok := firstElem.(*ast.TypedLiteral); ok {
			elemType = "iec." + strings.ToUpper(typedLit.TypeName)
			// For other literals, we let Go infer the type during assignment.
		}
	}

	t.write("[]%s{", elemType)
	for i, el := range al.Elements {
		t.transpileExpression(el)
		if i < len(al.Elements)-1 {
			t.write(", ")
		}
	}
	t.write("}")
	return nil
}

func (t *Transpiler) transpileMemberAccessExpression(exp *ast.MemberAccessExpression) error {
	if err := t.transpileExpression(exp.Struct); err != nil {
		return err
	}
	t.write(".%s", exp.Member.Value)
	return nil
}

func (t *Transpiler) transpileIndexExpression(exp *ast.IndexExpression) error {
	if err := t.transpileExpression(exp.Left); err != nil {
		return err
	}
	t.write("[")
	if err := t.transpileExpression(exp.Index); err != nil {
		return err
	}
	t.write("]")
	return nil
}
