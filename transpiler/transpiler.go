// In beedance/transpiler/transpiler.go
package transpiler

import (
	"beedance/ast"
	"beedance/evaluator"
	"beedance/object"
	"beedance/token"
	"fmt"
	"io"
	"log" // Added for time.Time and time.Duration
	"sort"
	"strings" // For type information if needed
)

// builtInFunctionReturnTypes maps standard IEC 61131-3 function names to their return type family.
// This is used by the IL transpiler to select the correct accumulator.
var builtInFunctionReturnTypes = map[string]string{
	// Type Conversion Functions
	"BOOL_TO_SINT":   "LINT",
	"BOOL_TO_INT":    "LINT",
	"BOOL_TO_DINT":   "LINT",
	"BOOL_TO_LINT":   "LINT",
	"BOOL_TO_USINT":  "LINT",
	"BOOL_TO_UINT":   "LINT",
	"BOOL_TO_UDINT":  "LINT",
	"BOOL_TO_ULINT":  "LINT",
	"BOOL_TO_REAL":   "LREAL",
	"BOOL_TO_LREAL":  "LREAL",
	"BOOL_TO_STRING": "STRING",
	"SINT_TO_BOOL":   "BOOL",
	"SINT_TO_INT":    "LINT",
	"SINT_TO_DINT":   "LINT",
	"SINT_TO_LINT":   "LINT",
	"SINT_TO_USINT":  "LINT",
	"SINT_TO_UINT":   "LINT",
	"SINT_TO_UDINT":  "LINT",
	"SINT_TO_ULINT":  "LINT",
	"SINT_TO_REAL":   "LREAL",
	"SINT_TO_LREAL":  "LREAL",
	"SINT_TO_STRING": "STRING",
	"INT_TO_BOOL":    "BOOL",
	"INT_TO_SINT":    "LINT",
	"INT_TO_DINT":    "LINT",
	"INT_TO_LINT":    "LINT",
	"INT_TO_USINT":   "LINT",
	"INT_TO_UINT":    "LINT",
	"INT_TO_UDINT":   "LINT",
	"INT_TO_ULINT":   "LINT",
	"INT_TO_REAL":    "LREAL",
	"INT_TO_LREAL":   "LREAL",
	"INT_TO_STRING":  "STRING",
	"DINT_TO_BOOL":   "BOOL",
	"DINT_TO_SINT":   "LINT",
	"DINT_TO_INT":    "LINT",
	"DINT_TO_LINT":   "LINT",
	"DINT_TO_USINT":  "LINT",
	"DINT_TO_UINT":   "LINT",
	"DINT_TO_UDINT":  "LINT",
	"DINT_TO_ULINT":  "LINT",
	"DINT_TO_REAL":   "LREAL",
	"DINT_TO_LREAL":  "LREAL",
	"DINT_TO_STRING": "STRING",
	"LINT_TO_BOOL":   "BOOL",
	"LINT_TO_SINT":   "LINT",
	"LINT_TO_INT":    "LINT",
	"LINT_TO_DINT":   "LINT",
	"LINT_TO_USINT":  "LINT",
	"LINT_TO_UINT":   "LINT",
	"LINT_TO_UDINT":  "LINT",
	"LINT_TO_ULINT":  "LINT",
	"LINT_TO_REAL":   "LREAL",
	"LINT_TO_LREAL":  "LREAL",
	"LINT_TO_STRING": "STRING",

	// Numerical Functions
	"ABS":  "LREAL", // Can be any numeric, but LREAL is a safe bet for accumulator
	"SQRT": "LREAL",
	"SIN":  "LREAL",
	"COS":  "LREAL",
	"TAN":  "LREAL",
	"ASIN": "LREAL",
	"ACOS": "LREAL",
	"ATAN": "LREAL",
	"MOD":  "LINT",

	// String Functions
	"LEN":    "LINT",
	"LEFT":   "STRING",
	"RIGHT":  "STRING",
	"MID":    "STRING",
	"CONCAT": "STRING",
	"FIND":   "LINT",
}

// standardFBPrimaryOutputs maps standard IEC 61131-3 function block names to their primary output.
// The primary output is the first VAR_OUTPUT parameter, which is loaded into the IL accumulator after a CAL instruction.
var standardFBPrimaryOutputs = map[string]struct {
	Name string // The name of the output parameter (e.g., "Q", "QU", "Q1")
	Type string // The base type family for the accumulator (e.g., "BOOL", "LINT")
}{
	"TON":    {Name: "Q", Type: "BOOL"},
	"TOF":    {Name: "Q", Type: "BOOL"},
	"TP":     {Name: "Q", Type: "BOOL"},
	"CTU":    {Name: "Q", Type: "BOOL"},
	"CTD":    {Name: "Q", Type: "BOOL"},
	"CTUD":   {Name: "QU", Type: "BOOL"}, // QU is the first output, QD is second
	"R_TRIG": {Name: "Q", Type: "BOOL"},
	"F_TRIG": {Name: "Q", Type: "BOOL"},
	"SR":     {Name: "Q1", Type: "BOOL"},
	"RS":     {Name: "Q1", Type: "BOOL"},
}

// Transpiler holds the state of the code generation process.
type Transpiler struct {
	w                io.Writer
	programVarName   string // The name of the receiver for program methods, e.g., "p"
	currentFunc      *ast.FunctionDeclaration
	currentFuncBlock *ast.FunctionBlockDeclaration   // The current FB being transpiled
	varInfo          map[string]*ast.TypeDeclaration // Maps var names in current scope to their type declaration
	inOutVars        map[string]bool                 // Set of VAR_IN_OUT variable names in the current function scope
	accessVars       map[string]bool                 // Set of VAR_ACCESS variable names in the current scope
	tempVars         map[string]bool                 // Set of VAR_TEMP variable names in the current scope
	ilCurrentCRType  string                          // The data type of the IL Current Result
	locatedVars      map[string]bool                 // Set of VARs with an AT % location
	mainGenerated    bool                            // Flag to ensure main is only generated once
	globalVars       map[string]bool                 // Set of global variable names
	typeInfo         map[string]ast.Node
}

// New creates a new Transpiler instance with the given io.Writer.
func New(w io.Writer) *Transpiler {
	return &Transpiler{
		w:             w,
		locatedVars:   make(map[string]bool),
		mainGenerated: false,
		accessVars:    make(map[string]bool),
		tempVars:      make(map[string]bool),
		inOutVars:     make(map[string]bool),
		globalVars:    make(map[string]bool),
		varInfo:       make(map[string]*ast.TypeDeclaration),
		typeInfo:      make(map[string]ast.Node),
	}
}

// Transpile is the main entry point for the transpilation process. It orchestrates
// a two-pass approach: first expanding macros, then generating Go code.
func (t *Transpiler) Transpile(programAST *ast.Program) error { // Changed parameter name
	// --- Pass 1: Macro Definition & Expansion ---
	// To handle macros correctly, we must expand them before generating Go code.
	// We can reuse the evaluator's macro engine for this.
	macroEnv := object.NewEnvironment()
	evaluator.DefineMacros(programAST, macroEnv)                // Use programAST here
	expandedAST := evaluator.ExpandMacros(programAST, macroEnv) // Use programAST here

	// --- Pass 2: Code Generation ---
	// Now, we transpile the modified AST that has all macros expanded.
	return t.transpileNode(expandedAST)
}

// transpileNode is the recursive heart of the transpiler, handling code generation for each AST node.
func (t *Transpiler) transpileNode(node ast.Node) error {
	// Check if the node has leading comments and transpile them.
	if nodeWithComments, ok := node.(interface{ GetLeadingComments() []string }); ok {
		t.transpileLeadingComments(nodeWithComments.GetLeadingComments())
	}

	switch node := node.(type) {
	case *ast.Program:
		t.buildTypeInfo(node) // First pass to collect type definitions
		t.buildGlobalVarInfo(node)
		for _, stmt := range node.Statements {
			if err := t.transpileNode(stmt); err != nil {
				return err
			}
		}
		return nil
	case *ast.ProgramDeclaration:
		// First, transpile any global var blocks that might exist at the program level
		t.transpileGlobalVarBlocks(node.VarGlobal)
		return t.transpileProgram(node) // This will call transpileNode recursively
	case *ast.BlockStatement:
		return t.transpileBlockStatement(node) // This will call transpileNode recursively
	case *ast.VarDeclStatement:
		t.transpileVarDecl(node)
		return nil
	case *ast.AssignmentStatement:
		return t.transpileAssignmentStatement(node) // This will call transpileNode recursively
	case *ast.IfStatement:
		return t.transpileIfStatement(node) // This will call transpileNode recursively
	case *ast.CaseStatement:
		return t.transpileCaseStatement(node) // This will call transpileNode recursively
	case *ast.ForLoopStatement:
		return t.transpileForLoopStatement(node) // This will call transpileNode recursively
	case *ast.WhileStatement:
		return t.transpileWhileStatement(node) // This will call transpileNode recursively
	case *ast.RepeatStatement:
		return t.transpileRepeatStatement(node) // This will call transpileNode recursively
	case *ast.ReturnStatement:
		return t.transpileReturnStatement(node) // This will call transpileNode recursively
	case *ast.ExitStatement:
		return t.transpileExitStatement(node)
	case *ast.ExpressionStatement:
		// The parser can misinterpret assignments to complex l-values (like array elements)
		// as an InfixExpression with operator `:=`. We detect this here and handle it
		// as a proper assignment statement. This is a workaround for a parser limitation.
		if infix, ok := node.Expression.(*ast.InfixExpression); ok && infix.Operator == ":=" {
			// Create a synthetic AssignmentStatement to be processed by the correct transpiler function.
			assignment := &ast.AssignmentStatement{
				Token: infix.Token,
				Left:  infix.Left,
				Value: infix.Right,
			}
			return t.transpileAssignmentStatement(assignment) // This will call transpileNode recursively
		}
		// For other expression statements (like function block calls), they are expected
		// to handle their own formatting (indentation and newlines).
		return t.transpileExpression(node.Expression) // This will call transpileNode recursively
	case *ast.FunctionBlockDeclaration:
		return t.transpileFunctionBlockDeclaration(node) // This will call transpileNode recursively
	case *ast.InterfaceDeclaration:
		return t.transpileInterfaceDeclaration(node)
	case *ast.TypeBlockDeclaration:
		return t.transpileTypeBlockDeclaration(node) // This will call transpileNode recursively
	case *ast.ConfigurationDeclaration:
		return t.transpileConfigurationDeclaration(node) // This will call transpileNode recursively
	case *ast.SFCProgram:
		// This is handled within transpileProgram/transpileFunctionBlockDeclaration
		// but we add a case to prevent "unhandled type" errors if it appears elsewhere.
		return nil
	case *ast.GlobalVarDeclaration:
		t.transpileGlobalVarBlocks([]*ast.GlobalVarDeclaration{node})
		return nil
	case *ast.FunctionDeclaration:
		return t.transpileFunctionDeclaration(node) // This will call transpileNode recursively
	case *ast.VarBlockDeclaration:
		// This handles VAR blocks that might appear as statements inside a body.
		// We treat them as local variable declarations.
		for _, decl := range node.Declarations {
			t.write("\tvar %s %s", decl.Name.Value, t.mapIecTypeToGo(decl.DataType))
			if decl.Value != nil {
				t.write(" = ")
				// Use a temporary transpiler to avoid carrying over receiver context.
				valT := &Transpiler{w: t.w, typeInfo: t.typeInfo, globalVars: t.globalVars} // This will call transpileNode recursively
				if err := valT.transpileExpression(decl.Value); err != nil {
					return err
				}
			}
			t.write("\n")
		}
		return nil
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
		switch node := stmt.(type) {
		case *ast.GlobalVarDeclaration:
			for _, decl := range node.Vars {
				t.globalVars[decl.Name.Value] = true
			}
		case *ast.ProgramDeclaration:
			for _, globalBlock := range node.VarGlobal {
				for _, decl := range globalBlock.Vars {
					t.globalVars[decl.Name.Value] = true
				}
			}
			// Also treat VAR_EXTERNAL as globals for name resolution purposes.
			for _, externalBlock := range node.VarExternal {
				for _, decl := range externalBlock.Vars {
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
		case *ast.InterfaceDeclaration:
			t.typeInfo[node.Name.Value] = node
		case *ast.TypeBlockDeclaration:
			for _, decl := range node.Declarations {
				t.typeInfo[decl.Name.Value] = decl
			}
		}
	}
}

// transpileGlobalVarBlocks transpiles VAR_GLOBAL blocks into Go package-level variable declarations.
// These variables are accessible throughout the generated Go code.
func (t *Transpiler) transpileGlobalVarBlocks(blocks []*ast.GlobalVarDeclaration) {
	for _, block := range blocks {
		t.write("// --- VAR_GLOBAL ---\n")
		for _, decl := range block.Vars {
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

// transpileConfigurationDeclaration transpiles an IEC 61131-3 CONFIGURATION block into a Go `main` function.
// This `main` function sets up program factories, resources, and tasks, acting as the entry point
// for a `royaljelly`-based runtime.
func (t *Transpiler) transpileConfigurationDeclaration(config *ast.ConfigurationDeclaration) error {
	if t.mainGenerated {
		return nil // Main function already generated
	}
	t.mainGenerated = true

	// Group VAR_CONFIG parameters by their program instance name, as they are now at the config level.
	paramsByInstance := make(map[string][]*ast.VarDeclStatement)
	for _, varConfig := range config.VarConfigs {
		instanceName := varConfig.ProgramInstanceName.Value
		paramsByInstance[instanceName] = varConfig.Declarations
	}

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
		t.transpileResourceDeclaration(res, paramsByInstance)
	}

	t.write("\t\t},\n")
	t.write("\t}\n\n")

	t.write("\t// This is where you would start the royaljelly scheduler with the generated config.\n")
	t.write("\tfmt.Println(\"Configuration loaded and ready to run.\")\n")
	t.write("\t// Example: royaljelly.Start(cfg)\n")

	t.write("}\n\n")
	return nil
}

// transpileResourceDeclaration transpiles an IEC 61131-3 RESOURCE block within a CONFIGURATION.
// It generates Go code to define tasks and program instances associated with that resource.
func (t *Transpiler) transpileResourceDeclaration(res *ast.ResourceDeclaration, paramsByInstance map[string][]*ast.VarDeclStatement) {
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

// transpileVarConfigParams transpiles the variable declarations from a VAR_CONFIG block
// into key-value pairs for a Go map[string]string literal.
func (t *Transpiler) transpileVarConfigParams(params []*ast.VarDeclStatement) {
	if params == nil {
		return
	}
	for _, p := range params {
		if p.Value == nil {
			continue
		}
		var valueStr string
		// We need the raw string representation of the literal value.
		switch v := p.Value.(type) {
		case *ast.StringLiteral:
			valueStr = v.Value // Value field holds the unquoted string
		case *ast.WStringLiteral:
			valueStr = v.Value // Value field holds the unquoted string
		default:
			// For other literals (INT, REAL, BOOL, TIME, etc.), the String() method gives a suitable representation.
			valueStr = p.Value.String()
		}
		t.write("\t\t\t\t\t\t\t%q: %q,\n", p.Name.Value, valueStr)
	}
}

// transpileTaskDeclaration transpiles an IEC 61131-3 TASK definition within a RESOURCE.
// It generates Go code to configure a task's name, priority, interval, and associated programs.
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

// transpileProgram transpiles an IEC 61131-3 PROGRAM POU into a Go struct and associated methods.
// This includes generating the struct definition for program variables, a factory function for instantiation,
// and a `Logic` method that contains the program's executable code (ST, IL, or SFC).
func (t *Transpiler) transpileProgram(prog *ast.ProgramDeclaration) error {
	t.programVarName = "p" // Set the receiver name

	// --- 1. Generate the struct definition ---
	// Build var info for this program's scope
	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	t.buildVarInfo(prog.Vars)

	originalAccessVars := t.accessVars
	t.accessVars = make(map[string]bool)
	t.buildAccessVarInfo(prog.VarAccess)

	originalTempVars := t.tempVars
	t.tempVars = make(map[string]bool)
	t.buildTempVarInfo(prog.VarTemp)
	defer func() { t.tempVars = originalTempVars }()

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(prog.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.accessVars = originalAccessVars }()

	defer func() { t.varInfo = originalVarInfo }() // Restore previous scope

	t.write("type %s struct {\n", prog.Name.Value)
	for _, varDecl := range prog.Vars {
		t.transpileVarDecl(varDecl)
	}
	for _, accessDecl := range prog.VarAccess {
		t.transpileVarAccess(accessDecl)
	}
	// If the body is an SFC program, we need to add state fields to the struct.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		t.write("\tsfcActiveSteps map[string]bool\n")
		t.transpileSFCStateFields(sfc)
		t.transpileSFCActionStateFields(sfc)
	}
	t.write("}\n\n")

	// --- 2. Generate the factory function for initialization ---
	t.write("// New%sFactory creates a new instance of the %s program.\n", prog.Name.Value, prog.Name.Value)
	t.write("func New%sFactory(params map[string]string) (func(time.Time), error) {\n", prog.Name.Value)
	t.write("\tinstance := &%s{}\n", prog.Name.Value)

	// Transpile initial values from VAR declarations
	//t.write("\t// Apply initial values from ST code\n")
	for _, varDecl := range prog.Vars {
		// Skip macro definitions, as they have no runtime initial value.
		if _, ok := varDecl.Value.(*ast.MacroLiteral); ok {
			continue
		}

		if varDecl.Value != nil {
			t.write("\tinstance.%s = ", varDecl.Name.Value)
			// Use a temporary transpiler with the receiver unset to transpile the value expression
			valT := &Transpiler{w: t.w, typeInfo: t.typeInfo, globalVars: t.globalVars}
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

	t.write("\treturn instance.Logic, nil\n")
	t.write("}\n\n")

	// --- 3. Generate the Logic method for the scheduler ---
	t.write("// Link connects the program's located variables to the runtime's I/O manager.\n")
	t.write("func (p *%s) Link(linker config.IOLinker) error {\n", prog.Name.Value)
	for _, varDecl := range prog.Vars {
		if varDecl.Location != nil {
			t.write("\tif err := linker.LinkIO(&p.%s, %q); err != nil {\n", varDecl.Name.Value, varDecl.Location.Location.String())
			t.write("\t\treturn err\n")
			t.write("\t}\n")
		}
	}
	for _, accessBlock := range prog.VarAccess {
		for _, decl := range accessBlock.Vars {
			t.write("\tif err := linker.LinkVar(&p.%s, %q); err != nil {\n", decl.Name.Value, decl.AccessPath.String())
			t.write("\t\treturn err\n\t}\n")
		}
	}
	t.write("\treturn nil\n}\n\n")

	t.write("func (%s *%s) Logic(now time.Time) {\n", t.programVarName, prog.Name.Value)
	// Transpile VAR_TEMP as local variables inside the Logic method.
	for _, tempBlock := range prog.VarTemp {
		for _, decl := range tempBlock.Vars {
			t.transpileVarDeclAsLocal(decl)
		}
	}

	// If the body is an SFC program, transpile it as a state machine.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		if err := t.transpileSFCProgram(sfc); err != nil {
			return err // This will call transpileNode recursively
		}
	} else if block, ok := prog.Body.(*ast.BlockStatement); ok && isIlBlock(block) {
		// It's an IL program
		if err := t.transpileIlProgram(block); err != nil {
			return err
		}
	} else {
		// Otherwise, transpile as a regular ST block of statements.
		if err := t.transpileNode(prog.Body); err != nil {
			return err
		}
	}
	t.write("}\n")
	t.programVarName = "" // Unset after use
	return nil
}

// transpileIlProgram transpiles an IEC 61131-3 Instruction List (IL) program body.
// It sets up typed accumulators and iterates through IL instructions, transpiling each one.
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

// transpileIlInstruction transpiles a single IEC 61131-3 Instruction List (IL) instruction.
// It handles various IL operators like LD, ST, ADD, JMP, CAL, and conditional modifiers.
func (t *Transpiler) transpileIlInstruction(stmt *ast.IlInstructionStatement) error {
	// Transpile the label, if it exists.
	if stmt.Label != nil {
		t.write("%s:\n", stmt.Label.Value)
	}

	// Handle conditional execution (C modifier)
	op := strings.ToUpper(stmt.Operator)
	isConditional := strings.Contains(stmt.Modifier, "C") && op != "JMP"
	if isConditional {
		// Conditional operations in IL are always based on a BOOL accumulator.
		t.write("\tif cr_BOOL { ")
	}

	switch op {
	case "LD":
		// Check for a parenthesized IL sub-program, e.g., LD (LD A ADD B)
		if block, ok := stmt.Operand.(*ast.BlockStatement); ok && len(block.Statements) > 0 && isIlBlock(block) {
			// This is a parenthesized IL sub-program, e.g., LD (LD A ADD B)
			// We transpile it into an anonymous Go function to isolate its accumulator.
			t.write("\tcr_LINT = func() iec.LINT {\n") // Assuming result is LINT for now
			t.transpileIlProgram(block)
			t.write("\t\treturn cr_LINT\n") // Return the sub-program's final CR
			t.write("\t}()\n")              // cspell:disable-line
			t.ilCurrentCRType = "LINT"
		} else {
			// Standard operand (variable or ST expression)
			var baseType string
			if targetTypeDecl := t.resolveAssignmentTargetType(stmt.Operand); targetTypeDecl != nil {
				baseType = t.getBaseTypeFamily(targetTypeDecl.DataType)
			} else {
				if _, isBool := stmt.Operand.(*ast.Boolean); isBool {
					baseType = "BOOL"
				} else {
					baseType = "LINT" // Assume LINT for other literals or ST expressions
				}
			}
			t.ilCurrentCRType = baseType
			t.write("\tcr_%s = iec.%s(", baseType, baseType)
			t.transpileExpression(stmt.Operand)
			t.write(")\n")
		}
	case "ST":
		targetTypeDecl := t.resolveAssignmentTargetType(stmt.Operand)
		if targetTypeDecl == nil {
			return fmt.Errorf("cannot determine type of ST target: %s", stmt.Operand.String())
		}
		targetType := t.mapIecTypeToGo(targetTypeDecl.DataType)
		crType := t.ilCurrentCRType

		t.write("\t")
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
		if strings.Contains(stmt.Modifier, "N") {
			t.write("\tif !cr_BOOL { goto %s; }\n", stmt.Operand.String())
		} else if strings.Contains(stmt.Modifier, "C") {
			t.write("\tif cr_BOOL { goto %s; }\n", stmt.Operand.String())
		} else {
			t.write("\tgoto %s\n", stmt.Operand.String())
		}

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
		t.write(" }\n")
	}

	return nil
}

// transpileIlCalInstruction handles the transpilation of the IL `CAL` instruction.
// It determines whether the call is to a FUNCTION or a FUNCTION_BLOCK and dispatches accordingly.
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

// transpileIlFunctionBlockCall transpiles an IL `CAL` instruction when the operand is a FUNCTION_BLOCK instance.
// It handles input assignments, calls the FB's `Logic` method, and loads the primary output into the IL accumulator.
func (t *Transpiler) transpileIlFunctionBlockCall(callExpr *ast.CallExpression) error {
	// 1. Transpile the input assignments for the function block call.
	for _, arg := range callExpr.Arguments {
		if namedArg, ok := arg.(*ast.NamedArgument); ok {
			isInOut := t.isInOutArgument(callExpr.Function, namedArg.Name.Value)
			if isInOut {
				t.write("\t")
				t.transpileExpression(callExpr.Function)
				t.write(".%s = &", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			} else {
				t.write("\t")
				t.transpileExpression(callExpr.Function)
				t.write(".%s = ", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			}
		}
	}

	// 2. Transpile the call to the Logic() method.
	t.write("\t")
	t.transpileExpression(callExpr.Function)
	t.write(".Logic(now)\n")

	// 3. Identify the primary output (first VAR_OUTPUT) to load into the accumulator.
	fbDef := t.getFunctionBlockDefinition(callExpr.Function)
	if fbDef == nil {
		// This could be a standard library FB like TON. Check our lookup table.
		fbName := ""
		if ident, ok := callExpr.Function.(*ast.Identifier); ok {
			fbName = strings.ToUpper(ident.Value)
		}

		if primaryOut, ok := standardFBPrimaryOutputs[fbName]; ok {
			// It's a known standard FB.
			t.write("\tcr_%s = ", primaryOut.Type)
			t.transpileExpression(callExpr.Function)
			t.write(".%s\n", primaryOut.Name)
			t.ilCurrentCRType = primaryOut.Type
			return nil
		}

		// If not found, fallback to the old warning and assumption.
		log.Printf("Warning: Could not find user definition for FB %s. Assuming primary output 'Q' of type BOOL.", callExpr.Function.String())
		t.write("\tcr_BOOL = ")
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

// transpileIlFunctionCall transpiles an IL `CAL` instruction when the operand is a FUNCTION call.
// It determines the function's return type and loads the result into the appropriate IL accumulator.
func (t *Transpiler) transpileIlFunctionCall(callExpr *ast.CallExpression) error {
	// 1. Determine the return type of the function.
	var funcName string
	if ident, ok := callExpr.Function.(*ast.Identifier); ok {
		funcName = ident.Value
	}

	crType := "" // Start with unknown type

	// Check user-defined functions first
	if defNode, ok := t.typeInfo[funcName]; ok {
		if funcDef, isFunc := defNode.(*ast.FunctionDeclaration); isFunc && funcDef.ReturnType != nil {
			crType = t.getBaseTypeFamily(funcDef.ReturnType)
		}
	}

	// If not found, check built-in functions
	if crType == "" {
		if builtInReturnType, ok := builtInFunctionReturnTypes[strings.ToUpper(funcName)]; ok {
			crType = builtInReturnType
		}
	}

	// If still not found, default to LINT and log a warning.
	if crType == "" {
		crType = "LINT" // Default assumption for unknown functions
		log.Printf("Warning: Could not determine return type for function '%s'. Assuming LINT.", funcName)
	}

	t.ilCurrentCRType = crType
	t.write("\tcr_%s = ", crType)

	// 2. Transpile the call itself.
	t.transpileStandardFunctionCall(callExpr)
	t.write("\n")

	return nil
}

// isIlBlock checks if a given block statement contains IL instructions, indicating it's an IL program body.
func isIlBlock(block *ast.BlockStatement) bool {
	if len(block.Statements) == 0 {
		return false
	}
	_, isIL := block.Statements[0].(*ast.IlInstructionStatement)
	// A more robust check might involve looking at all statements or a specific marker.
	return isIL
}

// getBaseTypeFamily categorizes an IEC data type into a broad family (e.g., LINT, LREAL, BOOL)
// for selecting the correct IL accumulator type.
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

// mapIlOperatorToGo maps IEC 61131-3 IL operators to their corresponding Go language operators.
func (t *Transpiler) mapIlOperatorToGo(op string) string {
	// Maps IL operators to their Go equivalents.
	upperOp := strings.ToUpper(op)
	switch upperOp {
	case "ADD":
		return "+"
	case "SUB":
		return "-"
	case "MUL":
		return "*"
	case "DIV":
		return "/"
	case "AND":
		return "&"
	case "OR":
		return "|"
	case "XOR":
		return "^"
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
	return op // Default case for operators that don't need mapping (though most do).
}

// transpileSFCStateFields generates Go struct fields for managing SFC step states,
// including active status (`_X`), previous active status (`_X_prev`), and activation time (`_T`).
func (t *Transpiler) transpileSFCStateFields(sfc *ast.SFCProgram) {
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			// For Step S1, generate S1_X bool and S1_T time.Duration
			t.write("\t%s_X bool\n", step.Name.Value)
			t.write("\t%s_X_prev iec.BOOL\n", step.Name.Value) // For P qualifier
			t.write("\t%s_T time.Time\n", step.Name.Value)
		}
	}
}

// transpileSFCActionStateFields generates Go struct fields for managing SFC action states,
// including their output (`_Q`), associated timers (`_Timer`), and other qualifier-related data.
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

	// To ensure deterministic output, we sort the action names.
	sortedActions := make([]string, 0, len(uniqueActions))
	for actionName := range uniqueActions {
		sortedActions = append(sortedActions, actionName)
	}
	sort.Strings(sortedActions)

	for _, actionName := range sortedActions {
		t.write("\t%s_Q iec.BOOL\n", actionName)
		// Check if any usage of this action is timed, if so, add a timer field.
		t.write("\t%s_Timer time.Time\n", actionName)
		t.write("\t%s_ActivationCount int\n", actionName)    // For P qualifier
		t.write("\t%s_Qualifier string\n", actionName)       // To store the last active qualifier for non-active step logic
		t.write("\t%s_Duration time.Duration\n", actionName) // For timed qualifiers
	}
}

// findInitialStep finds and returns the initial step of an SFC program.
func (t *Transpiler) findInitialStep(sfc *ast.SFCProgram) *ast.StepStatement {
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok && step.IsInitial {
			return step
		}
	}
	return nil
}

// transpileSFCProgram transpiles an IEC 61131-3 Sequential Function Chart (SFC) program.
// It generates Go code that simulates the SFC execution cycle, including transition evaluation,
// step state updates, and action processing based on qualifiers.
func (t *Transpiler) transpileSFCProgram(sfc *ast.SFCProgram) error {
	// --- Phase 0: Store previous step state for Pulse qualifiers ---
	t.write("\t// --- SFC Phase 0: Store previous step state ---\n")
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			t.write("\tp.%s_X_prev = p.%s_X\n", step.Name.Value, step.Name.Value)
		}
	}
	t.write("\n")

	t.write("\t// --- SFC Phase 1: Evaluate Transitions and collect fired transitions ---\n")
	t.write("\tfiredTransitions := make(map[string]bool)\n\n")
	for i, element := range sfc.Elements {
		if trans, ok := element.(*ast.TransitionStatement); ok {
			transKey := fmt.Sprintf("t%d", i)
			t.write("\t// Check transition %s from %v to %v\n", transKey, trans.From, trans.To)
			t.write("\tif ")
			if len(trans.From) == 0 {
				t.write("false") // A transition must have at least one source step.
			} else {
				for i, fromStep := range trans.From {
					if i > 0 {
						t.write(" && ")
					}
					t.write("p.sfcActiveSteps[%q]", fromStep.Value)
				}
			}
			t.write(" {\n")
			t.write("\t\tif ")
			t.transpileExpression(trans.Condition)
			t.write(" {\n")
			t.write("\t\t\tfiredTransitions[%q] = true\n", transKey)
			t.write("\t\t}\n")
			t.write("\t}\n")
		}
	}
	t.write("\n")

	t.write("\t// --- SFC Phase 2: Update Step States based on fired transitions ---\n")
	t.write("\tnextActiveSteps := make(map[string]bool)\n")
	t.write("\t// Copy current active steps; they will be deactivated if they are a source of a fired transition.\n")
	t.write("\tfor step, active := range p.sfcActiveSteps {\n")
	t.write("\t\tif active { nextActiveSteps[step] = true }\n")
	t.write("\t}\n\n")
	for i, element := range sfc.Elements {
		if trans, ok := element.(*ast.TransitionStatement); ok {
			transKey := fmt.Sprintf("t%d", i)
			t.write("\tif firedTransitions[%q] {\n", transKey)
			for _, fromStep := range trans.From {
				t.write("\t\tdelete(nextActiveSteps, %q)\n", fromStep.Value)
			}
			for _, toStep := range trans.To {
				t.write("\t\tnextActiveSteps[%q] = true\n", toStep.Value)
			}
			t.write("\t}\n")
		}
	}
	t.write("\n")

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
			// Group action associations by name to resolve qualifier priorities (R > S > others). // This will call transpileNode recursively
			actionsInStep := make(map[string][]*ast.ActionBlockStatement)
			for _, actionAssoc := range step.Actions {
				actionName := actionAssoc.ActionName.Value
				actionsInStep[actionName] = append(actionsInStep[actionName], actionAssoc)
			}

			// To ensure deterministic output, sort the action names.
			sortedActionNames := make([]string, 0, len(actionsInStep))
			for name := range actionsInStep {
				sortedActionNames = append(sortedActionNames, name)
			}
			sort.Strings(sortedActionNames)

			t.write("\t// Actions for step %s\n", step.Name.Value)
			t.write("\tif p.sfcActiveSteps[%q] {\n", step.Name.Value)
			// Transpile logic for when the step is ACTIVE.
			for _, actionName := range sortedActionNames {
				associations := actionsInStep[actionName]
				effectiveAssoc := findHighestPriorityAction(associations)
				if effectiveAssoc != nil {
					t.transpileSFCAction(step, effectiveAssoc, true)
				}
			}
			t.write("\t} else {\n")
			// Transpile logic for when the step is NOT ACTIVE (for stored/timed actions).
			for _, actionName := range sortedActionNames {
				associations := actionsInStep[actionName]
				effectiveAssoc := findHighestPriorityAction(associations)
				if effectiveAssoc != nil {
					t.transpileSFCAction(step, effectiveAssoc, false)
				}
			}
			t.write("\t}\n")
		}
	}
	t.write("\n")

	// Finally, execute the bodies of all actions that are currently active.
	t.write("\t// --- SFC Phase 4: Execute Action Bodies ---\n")
	actionDefinitions := make(map[string]*ast.ActionStatement)
	for _, element := range sfc.Elements {
		if action, ok := element.(*ast.ActionStatement); ok {
			actionDefinitions[action.Name.Value] = action
		}
	}

	// Get a unique list of all action names that are actually used in steps.
	usedActions := make(map[string]bool)
	for _, element := range sfc.Elements {
		if step, ok := element.(*ast.StepStatement); ok {
			for _, actionAssoc := range step.Actions {
				usedActions[actionAssoc.ActionName.Value] = true
			}
		}
	}

	// To ensure deterministic output, we sort the action names.
	sortedActions := make([]string, 0, len(usedActions))
	for name := range usedActions {
		sortedActions = append(sortedActions, name)
	}
	sort.Strings(sortedActions)

	for _, name := range sortedActions {
		t.write("\tif p.%s_Q {\n", name)
		if def, ok := actionDefinitions[name]; ok {
			if block, isBlock := def.Body.(*ast.BlockStatement); isBlock {
				// Manually iterate and indent statements within the action body.
				for _, s := range block.Statements {
					t.write("\t") // This will call transpileNode recursively
					if err := t.transpileNode(s); err != nil {
						return err
					}
				}
			}
		}
		t.write("\t}\n")
	}

	return nil
}

// transpileSFCAction transpiles an SFC action block, generating Go code that implements
// the logic for various action qualifiers (N, S, R, P, D, L, SD, DS, SL) based on whether
// the associated step is active or not.
func (t *Transpiler) transpileSFCAction(step *ast.StepStatement, actionBlock *ast.ActionBlockStatement, isStepActive bool) {
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
			t.write("\t\tp.%s_Q = p.%s_X && !p.%s_X_prev\n", actionName, step.Name.Value, step.Name.Value)
		case "D": // Non-stored delayed
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; }\n", actionName, actionName)
			t.write("\t\tp.%s_Q = now.Sub(p.%s_Timer) >= ", actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write("\n")
		case "L": // Non-stored limited
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; }\n", actionName, actionName)
			t.write("\t\tp.%s_Q = now.Sub(p.%s_Timer) < ", actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write("\n")
		case "SD", "DS": // Stored delayed
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; }\n", actionName, actionName)
			// Only set Q to true, never to false. It must be reset by 'R'.
			t.write("\t\tif !p.%s_Q && now.Sub(p.%s_Timer) >= ", actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = true; }\n", actionName)
		case "SL": // Stored limited
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; p.%s_Q = true; }\n", actionName, actionName, actionName) // Set Q true immediately
			// Only set Q to false, never back to true.
			t.write("\t\tif now.Sub(p.%s_Timer) >= ", actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = false; }\n", actionName)
		}
	} else { // Step is not active
		switch qualifier {
		case "N", "R", "P", "D", "L": // For all non-stored qualifiers, deactivate the action and reset its timer.
			t.write("\t\tp.%s_Q = false\n", actionName)
			t.write("\t\tp.%s_Timer = time.Time{}\n", actionName) // Reset timer
		case "SD", "DS":
			// Stored delayed. If timer is running, check if it has elapsed to set Q to true.
			t.write("\t\tif !p.%s_Timer.IsZero() && !p.%s_Q && now.Sub(p.%s_Timer) >= ", actionName, actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = true; }\n", actionName)
		case "SL": // Stored-Limited
			// If timer is running, check if it has elapsed to set Q to false.
			t.write("\t\tif !p.%s_Timer.IsZero() && p.%s_Q && now.Sub(p.%s_Timer) >= ", actionName, actionName, actionName)
			t.transpileExpression(actionBlock.Duration)
			t.write(" { p.%s_Q = false; }\n", actionName)
			// For S, the state is maintained, so we do nothing here.
		}
	}
}

// transpileFunctionBlockDeclaration transpiles an IEC 61131-3 FUNCTION_BLOCK POU into a Go struct
// and a `Logic` method. The struct holds the FB's internal and I/O variables, and the `Logic` method
// contains the FB's executable code, including EN/ENO handling.
func (t *Transpiler) transpileFunctionBlockDeclaration(fb *ast.FunctionBlockDeclaration) error {
	// Set context for the current function block to handle SUPER calls.
	originalFuncBlock := t.currentFuncBlock
	t.currentFuncBlock = fb
	defer func() { t.currentFuncBlock = originalFuncBlock }()

	// 1. Generate the struct for the Function Block.
	t.write("// %s is the transpiled struct for the FUNCTION_BLOCK of the same name.\n", fb.Name.Value)

	t.write("type %s struct {\n", fb.Name.Value)

	// If the FB extends another, embed the parent struct.
	if fb.Extends != nil {
		t.write("\t%s\n", fb.Extends.Value)
	}

	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	t.buildVarInfo(fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars)

	originalTempVars := t.tempVars
	t.tempVars = make(map[string]bool)
	t.buildTempVarInfo(fb.VarTemp)
	defer func() { t.tempVars = originalTempVars }()

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(fb.VarInputs, fb.VarOutputs, fb.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.varInfo = originalVarInfo }()

	// --- Add implicit EN and ENO fields if not extending ---
	// If extending, the parent FB is expected to provide these.
	if fb.Extends == nil {
		t.write("\tEN  iec.BOOL\n")
		t.write("\tENO iec.BOOL\n")
	}

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
	// VAR_EXTERNAL variables are not part of the struct; they are global.
	// VAR_TEMP variables are local to the Logic() call, not fields of the struct.
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

	// Transpile VAR_TEMP as local variables inside the Logic method.
	for _, tempBlock := range fb.VarTemp {
		for _, decl := range tempBlock.Vars {
			t.transpileVarDeclAsLocal(decl)
		}
	}

	if err := t.transpileNode(fb.Body); err != nil {
		t.programVarName = originalProgramVarName // Restore context on error
		return err
	}

	t.write("}\n\n")

	// 3. Generate methods for the Function Block.
	for _, method := range fb.Methods {
		if err := t.transpileMethodDeclaration(fb, method); err != nil {
			return err
		}
	}

	// 4. Add static checks to ensure it implements the specified interfaces.
	for _, iface := range fb.Implements {
		t.write("// Statically assert that %s implements %s.\n", fb.Name.Value, iface.Value)
		t.write("var _ %s = (*%s)(nil)\n\n", iface.Value, fb.Name.Value)
	}

	t.programVarName = originalProgramVarName // Restore context
	return nil
}

// transpileInterfaceDeclaration transpiles an IEC 61131-3 INTERFACE into a Go interface.
func (t *Transpiler) transpileInterfaceDeclaration(iface *ast.InterfaceDeclaration) error {
	t.write("// %s is the transpiled Go interface for the IEC 61131-3 INTERFACE of the same name.\n", iface.Name.Value)
	t.write("type %s interface {\n", iface.Name.Value)

	for _, method := range iface.Methods {
		// Build parameter list string
		params := []string{}
		for _, p := range method.Parameters {
			goType := t.mapIecTypeToGo(p.DataType)
			params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
		}
		paramStr := strings.Join(params, ", ")

		// Build return type string
		returnStr := ""
		if method.ReturnType != nil {
			// Check if return type is VOID, if so, it's an empty return string
			if typeSpec, ok := method.ReturnType.(*ast.TypeSpecifier); !ok || strings.ToUpper(typeSpec.Token.Literal) != "VOID" {
				returnStr = " " + t.mapIecTypeToGo(method.ReturnType)
			}
		}

		t.write("\t%s(%s)%s\n", method.Name.Value, paramStr, returnStr)
	}

	t.write("}\n\n")
	return nil
}

// transpileMethodDeclaration transpiles an IEC 61131-3 METHOD into a Go method on the FB's struct.
func (t *Transpiler) transpileMethodDeclaration(fb *ast.FunctionBlockDeclaration, method *ast.MethodDeclaration) error {
	// Do not generate any code for abstract methods, as they have no body.
	// The Go compiler will enforce implementation through interface satisfaction checks.
	if method.IsAbstract {
		return nil
	}

	// The receiver name is already set in t.programVarName by the caller (transpileFunctionBlockDeclaration).
	receiverName := t.programVarName

	// Track VAR_IN_OUT for this method's scope to handle dereferencing.
	originalInOutVars := t.inOutVars
	t.inOutVars = make(map[string]bool)
	defer func() { t.inOutVars = originalInOutVars }()

	// Build the method signature.
	t.write("// %s is a method on the %s FUNCTION_BLOCK.\n", method.Name.Value, fb.Name.Value)
	t.write("func (%s *%s) %s(", receiverName, fb.Name.Value, method.Name.Value)

	// Transpile parameters (VAR_INPUT, VAR_IN_OUT).
	params := []string{}
	for _, p := range method.VarInputs {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
	}
	for _, p := range method.VarInOuts {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s *%s", p.Name.Value, goType))
		t.inOutVars[p.Name.Value] = true
	}
	t.write("%s", strings.Join(params, ", "))
	t.write(") ")

	// Transpile return type.
	if method.ReturnType != nil {
		// Check for VOID return type, which means no return value in Go.
		if typeSpec, ok := method.ReturnType.(*ast.TypeSpecifier); !ok || strings.ToUpper(typeSpec.Token.Literal) != "VOID" {
			t.write("%s", t.mapIecTypeToGo(method.ReturnType))
		}
	}
	t.write(" {\n")

	// Transpile local variables (VAR_TEMP).
	for _, tempBlock := range method.VarTemp {
		for _, decl := range tempBlock.Vars {
			t.transpileVarDeclAsLocal(decl)
		}
	}

	// Transpile the method body.
	// Set a temporary function context so that `MyMethod := ...` is treated as a return.
	originalFunc := t.currentFunc
	t.currentFunc = &ast.FunctionDeclaration{Name: method.Name, ReturnType: method.ReturnType}
	defer func() { t.currentFunc = originalFunc }()

	if err := t.transpileNode(method.Body); err != nil {
		return err
	}

	t.write("}\n\n")
	return nil
}

// transpileVarDeclAsLocal transpiles a variable declaration as a local `var` statement
// inside a function body, used for VAR_TEMP.
func (t *Transpiler) transpileVarDeclAsLocal(varDecl *ast.VarDeclStatement) {
	t.write("\tvar %s %s", varDecl.Name.Value, t.mapIecTypeToGo(varDecl.DataType))
	if varDecl.Value != nil {
		t.write(" = ")
		// Use a temporary transpiler to avoid carrying over receiver context.
		valT := &Transpiler{w: t.w, typeInfo: t.typeInfo, globalVars: t.globalVars}
		if err := valT.transpileExpression(varDecl.Value); err != nil {
			// This is not ideal as we can't return an error here. Log it.
			log.Printf("Error transpiling initial value for temp var: %v", err)
		}
	}
	t.write("\n")
}

// isTransitionFromStep checks if a given transition statement originates from a specific step.
// This is a helper for SFC transpilation.
func (t *Transpiler) isTransitionFromStep(trans *ast.TransitionStatement, stepName string) bool {
	for _, from := range trans.From {
		if from.Value == stepName {
			return true
		}
	}
	return false
}

// transpileSFCTransition generates Go code for an SFC transition, including
// deactivating source steps and activating destination steps when the transition condition is met.
func (t *Transpiler) transpileSFCTransition(trans *ast.TransitionStatement) {
	t.write("\t\t// Transition from %s to %s\n", trans.From[0].Value, trans.To[0].Value)
	t.write("\t\tif ")
	t.transpileExpression(trans.Condition)
	t.write(" {\n") // This will call transpileNode recursively
	// Deactivate source steps and activate destination steps
	for _, from := range trans.From {
		t.write("\t\t\tdelete(nextActiveSteps, %q)\n", from.Value)
	}
	for _, to := range trans.To {
		t.write("\t\t\tnextActiveSteps[%q] = true\n", to.Value)
	}
	t.write("\t\t}\n")
}

// buildVarInfo populates the transpiler's `varInfo` map with `TypeDeclaration`s for variables
// found in the provided variable blocks. This is used for type resolution during transpilation.
func (t *Transpiler) buildVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			var typeName string
			currentType := varDecl.DataType
			for {
				if arrayDef, ok := currentType.(*ast.ArrayDefinition); ok {
					currentType = arrayDef.DataType
				} else {
					break
				}
			}

			if typeSpec, ok := currentType.(*ast.TypeSpecifier); ok {
				typeName = typeSpec.Token.Literal
			} else if typeIdent, ok := currentType.(*ast.Identifier); ok {
				typeName = typeIdent.Value
			}

			typeDecl := t.getTypeDeclaration(typeName)
			if typeDecl == nil {
				typeDecl = &ast.TypeDeclaration{Name: &ast.Identifier{Value: typeName}, DataType: varDecl.DataType}
			}
			t.varInfo[varDecl.Name.Value] = typeDecl
		}
	}
}

// buildLocatedVarInfo populates the `locatedVars` map with names of variables that have
// an AT location, indicating they are directly mapped to I/O.
func (t *Transpiler) buildLocatedVarInfo(varBlocks ...[]*ast.VarDeclStatement) {
	for _, block := range varBlocks {
		for _, varDecl := range block {
			if varDecl.Location != nil {
				t.locatedVars[varDecl.Name.Value] = true
			}
		}
	}
}

// buildAccessVarInfo populates the `accessVars` map with names of variables declared in VAR_ACCESS blocks.
// These variables represent external access paths.
func (t *Transpiler) buildAccessVarInfo(accessBlocks []*ast.AccessVarDeclaration) {
	for _, block := range accessBlocks {
		for _, decl := range block.Vars {
			t.accessVars[decl.Name.Value] = true
		}
	}
}

// buildTempVarInfo populates the `tempVars` map with names of variables declared in VAR_TEMP blocks.
func (t *Transpiler) buildTempVarInfo(tempBlocks []*ast.TempVarDeclaration) {
	for _, block := range tempBlocks {
		for _, decl := range block.Vars {
			t.tempVars[decl.Name.Value] = true
		}
	}
}

// isFunctionBlockType checks if a given AST expression representing a data type
// refers to a user-defined FUNCTION_BLOCK type.
func (t *Transpiler) isFunctionBlockType(dataType ast.Expression) bool {
	var typeName string
	if typeIdent, ok := dataType.(*ast.Identifier); ok {
		typeName = typeIdent.Value
	} else if typeSpec, ok := dataType.(*ast.TypeSpecifier); ok {
		typeName = typeSpec.TokenLiteral()
	}

	if typeName != "" {
		if typeDef, ok := t.typeInfo[typeName]; ok {
			if _, isFB := typeDef.(*ast.FunctionBlockDeclaration); isFB {
				return true
			}
		}
	}
	return false
}

// transpileFunctionDeclaration transpiles an IEC 61131-3 FUNCTION POU into a Go function.
// It handles input, output, and in-out parameters, as well as local variables and the function body.
// The function's return value is handled by assignments to the function's name within the body.
func (t *Transpiler) transpileFunctionDeclaration(fd *ast.FunctionDeclaration) error {
	// Set context for the duration of this function's transpilation
	originalFunc := t.currentFunc
	t.currentFunc = fd
	defer func() { t.currentFunc = originalFunc }()

	// Standalone functions do not have a receiver like programs or FBs.
	originalProgramVarName := t.programVarName
	t.programVarName = ""
	defer func() { t.programVarName = originalProgramVarName }()

	// Track VAR_IN_OUT variables to handle dereferencing.
	originalInOutVars := t.inOutVars
	t.inOutVars = make(map[string]bool)
	defer func() { t.inOutVars = originalInOutVars }()

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
		t.inOutVars[p.Name.Value] = true
	}
	t.write("%s", strings.Join(params, ", "))
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
		t.write("%s", returns[0])
	}

	t.write(" {\n")

	// --- 2. Transpile local variables (VAR) ---
	for _, v := range fd.Vars {
		t.write("\tvar %s %s\n", v.Name.Value, t.mapIecTypeToGo(v.DataType))
	}
	t.write("\n")

	// --- 3. Transpile the function body ---
	if err := t.transpileNode(fd.Body); err != nil {
		return err
	}

	t.write("}\n\n")
	return nil
}

// transpileVarDecl transpiles a single variable declaration (VAR, VAR_INPUT, VAR_OUTPUT, VAR_TEMP)
// into a Go struct field. It handles located variables by making them pointers.
func (t *Transpiler) transpileVarDecl(varDecl *ast.VarDeclStatement) {
	// Check if we are trying to instantiate an abstract function block.
	var getBaseTypeName func(dt ast.Expression) string
	getBaseTypeName = func(dt ast.Expression) string {
		if arrayDef, ok := dt.(*ast.ArrayDefinition); ok {
			return getBaseTypeName(arrayDef.DataType)
		} else if typeSpec, ok := dt.(*ast.TypeSpecifier); ok {
			return typeSpec.Token.Literal
		} else if typeIdent, ok := dt.(*ast.Identifier); ok {
			return typeIdent.Value
		}
		return ""
	}
	typeName := getBaseTypeName(varDecl.DataType)

	if typeName != "" {
		if typeDef, ok := t.typeInfo[typeName]; ok {
			if fbDef, isFB := typeDef.(*ast.FunctionBlockDeclaration); isFB && fbDef.IsAbstract {
				log.Printf("ERROR: Cannot instantiate abstract function block '%s' for variable '%s'", typeName, varDecl.Name.Value)
				t.write("\t// ERROR: Cannot instantiate abstract function block %s\n", typeName)
				return
			}
		}
	}
	// Transpile any leading comments associated with this variable declaration.
	// If the variable is a macro definition, skip it entirely as it has no
	// runtime equivalent in the transpiled code.
	if _, ok := varDecl.Value.(*ast.MacroLiteral); ok {
		return
	}

	t.transpileLeadingComments(varDecl.LeadingComments)

	// Get the variable name.
	// If it's a located variable, transpile it as a pointer.
	if varDecl.Location != nil {
		goType := t.mapIecTypeToGo(varDecl.DataType)
		t.write("\t%s *%s // AT %s\n", varDecl.Name.Value, goType, varDecl.Location.Location.String())
		return
	}

	// Special handling for FUNCTION type to generate a func signature.
	if typeSpec, ok := varDecl.DataType.(*ast.TypeSpecifier); ok && typeSpec.Token.Type == token.FUNCTION {
		// Infer the signature from the initial value if it's a function literal.
		if fnLit, ok := varDecl.Value.(*ast.FunctionLiteral); ok {
			t.write("\t%s func(", varDecl.Name.Value)
			params := []string{}
			for _, p := range fnLit.Parameters {
				goType := t.mapIecTypeToGo(p.DataType)
				params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
			}
			t.write("%s", strings.Join(params, ", "))
			t.write(")")

			if returnTypeSpec, ok := fnLit.ReturnType.(*ast.TypeSpecifier); !ok || strings.ToUpper(returnTypeSpec.Token.Literal) != "VOID" {
				t.write(" %s", t.mapIecTypeToGo(fnLit.ReturnType))
			}
			t.write("\n")
			return
		}
		// Fallback for a FUNCTION variable without a literal assignment.
		t.write("\t%s func()\n", varDecl.Name.Value)
		return
	}

	name := varDecl.Name.Value

	// Get the Go type for the IEC data type.
	goType := t.mapIecTypeToGo(varDecl.DataType)

	// Write the struct field. e.g., "MyCounter iec.LINT"
	t.write("\t%s %s\n", name, goType)

}

// transpileVarAccess transpiles a VAR_ACCESS declaration. It infers the type of the accessed variable
// and declares a pointer field in the Go struct to represent the access path.
func (t *Transpiler) transpileVarAccess(accessDecl *ast.AccessVarDeclaration) {
	for _, decl := range accessDecl.Vars {
		// Transpile any leading comments associated with this variable declaration.
		t.transpileLeadingComments(decl.LeadingComments)

		localName := decl.Name.Value

		// Infer the type of the target variable.
		targetTypeDecl := t.resolveAssignmentTargetType(decl.AccessPath)
		if targetTypeDecl == nil {
			log.Printf("Warning: Could not resolve type for VAR_ACCESS target: %s", decl.AccessPath.String())
			t.write("\t%s *any // Could not resolve type for %s\n", localName, decl.AccessPath.String())
			continue
		}

		// Get the Go type and declare the field as a pointer to that type.
		goType := t.mapIecTypeToGo(targetTypeDecl.DataType)
		t.write("\t%s *%s\n", localName, goType)
	}
}

// LinkMethodBody is a placeholder for the `Link` method's body in generated Go programs.
// This method is intended for runtime linking of located variables to an I/O manager.
func (t *Transpiler) LinkMethodBody(prog *ast.ProgramDeclaration) string {
	// This is a placeholder for the logic that would be generated inside the Link method.
	return "// Runtime linking logic will be placed here by the royaljelly scheduler.\n"
}

// transpileVarDeclInOut transpiles a VAR_IN_OUT declaration into a Go struct field that is a pointer to the IEC type.
func (t *Transpiler) transpileVarDeclInOut(varDecl *ast.VarDeclStatement) {
	name := varDecl.Name.Value
	goType := t.mapIecTypeToGo(varDecl.DataType)
	// Write the struct field as a pointer, e.g., "MyVar *iec.INT"
	t.write("\t%s *%s\n", name, goType)

}

// transpileCaseStatement transpiles an IEC 61131-3 CASE statement into a Go `switch` statement.
// transpileCaseStatement transpiles an IEC 61131-3 CASE statement to a Go switch statement.
// If the CASE statement includes ranges, it transpiles to an if-else-if chain instead.
func (t *Transpiler) transpileCaseStatement(stmt *ast.CaseStatement) error {
	// Check if any case uses a range. If so, we must generate an if/else if chain.
	hasRange := false
	for _, caseElem := range stmt.Cases {
		for _, val := range caseElem.Values {
			if infix, ok := val.(*ast.InfixExpression); ok && infix.Operator == ".." {
				hasRange = true
				break
			}
		}
		if hasRange {
			break
		}
	}

	if hasRange {
		return t.transpileCaseWithRanges(stmt)
	}

	// Original switch-based implementation for non-range cases.
	t.write("\tswitch ")
	if err := t.transpileExpression(stmt.Expression); err != nil {
		return err
	}
	t.write(" {\n")

	for _, caseElem := range stmt.Cases {
		t.write("\tcase ")
		for i, expr := range caseElem.Values {
			if i > 0 {
				t.write(", ")
			}
			if err := t.transpileExpression(expr); err != nil {
				return err
			}
		}
		t.write(":\n")
		t.transpileNode(caseElem.Consequence)
	}

	if stmt.Alternative != nil {
		t.write("\tdefault:\n")
		t.transpileNode(stmt.Alternative)
	}
	t.write("\t}\n")
	return nil
}

// transpileCaseWithRanges transpiles a CASE statement into a Go if-else-if chain
// to correctly handle range conditions (e.g., 1..10).
func (t *Transpiler) transpileCaseWithRanges(stmt *ast.CaseStatement) error {
	// Store the selector expression in a temporary variable to avoid re-evaluation.
	t.write("\tcaseSelector := ")
	if err := t.transpileExpression(stmt.Expression); err != nil {
		return err
	}
	t.write("\n")

	for i, caseElem := range stmt.Cases {
		if i == 0 {
			t.write("\tif ")
		} else {
			t.write(" else if ")
		}

		// Build the condition for the if/else if
		for j, val := range caseElem.Values {
			if j > 0 {
				t.write(" || ")
			}
			if infix, ok := val.(*ast.InfixExpression); ok && infix.Operator == ".." {
				// Range: (caseSelector >= L && caseSelector <= H)
				t.write("(caseSelector >= ")
				t.transpileExpression(infix.Left)
				t.write(" && caseSelector <= ")
				t.transpileExpression(infix.Right)
				t.write(")")
			} else {
				// Simple value: caseSelector == V
				t.write("(caseSelector == ")
				t.transpileExpression(val)
				t.write(")")
			}
		}
		t.write(" {\n")
		t.transpileNode(caseElem.Consequence)
		t.write("\t}")
	}

	// Handle the final ELSE block
	if stmt.Alternative != nil {
		t.write(" else {\n")
		t.transpileNode(stmt.Alternative)
		t.write("\t}")
	}
	t.write("\n")

	return nil
}

// transpileAssignmentStatement transpiles an IEC 61131-3 assignment (`:=`) into a Go assignment (`=`).
// transpileAssignmentStatement transpiles an IEC `:=` assignment to a Go `=` assignment.
func (t *Transpiler) transpileAssignmentStatement(stmt *ast.AssignmentStatement) error {
	// Special case: Assignment to the function name is a return statement.
	if ident, ok := stmt.Left.(*ast.Identifier); ok && t.currentFunc != nil && ident.Value == t.currentFunc.Name.Value {
		t.write("\treturn ")
		if err := t.transpileExpression(stmt.Value); err != nil {
			return err
		}
		t.write("\n")
		return nil
	}

	t.write("\t")

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

// resolveAssignmentTargetType recursively determines the `TypeDeclaration` of the target
// of an assignment expression. It handles identifiers, array elements, and struct members.
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

		// Get the type name from the variable's type declaration (e.g., "MyStruct")
		structTypeName := structVarType.DataType.String()

		// Look up the actual struct definition using its type name.
		structDefNode, ok := t.typeInfo[structTypeName]
		if !ok {
			return nil
		}

		// Now, find the member's type declaration within the struct's definition.
		return t.findMemberType(structDefNode, e.Member.Value)
	}
	return nil
}

// getTypeDeclaration retrieves a `TypeDeclaration` from the transpiler's `typeInfo` map
// based on the provided type name.
func (t *Transpiler) getTypeDeclaration(typeName string) *ast.TypeDeclaration {
	if typeDef, ok := t.typeInfo[typeName]; ok {
		if typeDecl, ok := typeDef.(*ast.TypeDeclaration); ok {
			return typeDecl
		}
	}
	return nil
}

// findMemberType searches within a `STRUCT` definition (represented by an `ast.Node`)
// for a member with the given name and returns its `TypeDeclaration`.
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

// transpileSubrangeAssignment handles assignments to variables declared with a subrange type.
// It generates Go code that uses a `Clamp` function from the `iec` package to ensure the assigned
// value stays within the defined range.
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

// transpileForLoopStatement transpiles an IEC 61131-3 `FOR` loop into a Go `for` loop.
// transpileForLoopStatement transpiles a FOR loop to a Go `for` loop.
func (t *Transpiler) transpileForLoopStatement(stmt *ast.ForLoopStatement) error {
	// The control variable is an assignment statement itself.
	t.write("\tfor ")
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
	for _, s := range stmt.Body.Statements {
		t.write("\t")
		if err := t.transpileNode(s); err != nil {
			return err
		}
	}
	t.write("\t}\n")
	return nil
}

// transpileBlockStatement iterates through the statements within an AST block and transpiles each one.
// transpileBlockStatement iterates over statements in a block and transpiles them.
func (t *Transpiler) transpileBlockStatement(bs *ast.BlockStatement) error {
	if bs == nil {
		return nil
	}
	for _, stmt := range bs.Statements {
		if err := t.transpileNode(stmt); err != nil {
			return err
		}
	}
	return nil
}

// transpileWhileStatement transpiles an IEC 61131-3 `WHILE` loop into a Go `for` loop.
// transpileWhileStatement transpiles a WHILE loop to a Go `for` loop.
func (t *Transpiler) transpileWhileStatement(stmt *ast.WhileStatement) error {
	t.write("\tfor ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" {\n")
	for _, s := range stmt.Body.Statements {
		t.write("\t")
		if err := t.transpileNode(s); err != nil {
			return err
		}
	}
	t.write("\t}\n")
	return nil
}

// transpileRepeatStatement transpiles an IEC 61131-3 `REPEAT...UNTIL` loop into a Go `for` loop with a `break` condition.
// transpileRepeatStatement transpiles a REPEAT...UNTIL loop to a Go `for` loop.
func (t *Transpiler) transpileRepeatStatement(stmt *ast.RepeatStatement) error {
	t.write("\tfor {\n")
	for _, s := range stmt.Body.Statements {
		t.write("\t")
		if err := t.transpileNode(s); err != nil {
			return err
		}
	}
	t.write("\t\tif ")
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}
	t.write(" { break }\n")
	t.write("\t}\n")
	return nil
}

// transpileReturnStatement transpiles an IEC 61131-3 `RETURN` statement into a Go `return` statement.
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

// transpileExitStatement transpiles an IEC 61131-3 `EXIT` statement into a Go `break` statement.
// transpileExitStatement transpiles an EXIT statement to a `break`.
func (t *Transpiler) transpileExitStatement(stmt *ast.ExitStatement) error {
	t.write("\tbreak\n")
	return nil
}

// transpileExpression dispatches to specific handlers for different expression types.
// It handles identifiers, literals, and various operators, converting them into their
// corresponding Go syntax.
func (t *Transpiler) transpileExpression(exp ast.Expression) error {
	switch exp := exp.(type) {
	case *ast.Identifier:
		// If it's a located or access variable, it's a pointer and must be dereferenced.
		if t.locatedVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.accessVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.inOutVars[exp.Value] {
			// VAR_IN_OUT in a FUNCTION is a pointer and must be dereferenced.
			t.write("(*%s)", exp.Value)
		} else if t.tempVars[exp.Value] {
			// It's a temporary variable, local to the Logic function.
			t.write("%s", exp.Value)
		} else if t.globalVars[exp.Value] {
			// If it's a global variable, write it without a prefix.
			t.write("%s", exp.Value)
		} else if t.programVarName != "" {
			// It's a local/member variable of a PROGRAM or FUNCTION_BLOCK, so prefix it with the receiver.
			t.write("%s.%s", t.programVarName, exp.Value)
		} else {
			// It's a local variable inside a FUNCTION.
			t.write("%s", exp.Value)
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
	case *ast.SuperCallExpression:
		if t.currentFuncBlock == nil || t.currentFuncBlock.Extends == nil {
			return fmt.Errorf("SUPER call used outside of a derived FUNCTION_BLOCK")
		}
		parentTypeName := t.currentFuncBlock.Extends.Value
		receiverName := t.programVarName
		// The call is receiver.ParentType.Method(args...)
		t.write("%s.%s.%s(", receiverName, parentTypeName, exp.Method.Value)
		// Transpile arguments
		for i, arg := range exp.Arguments {
			if i > 0 {
				t.write(", ")
			}
			if err := t.transpileExpression(arg); err != nil {
				return err
			}
		}
		t.write(")")
	case *ast.CallExpression:
		return t.transpileCallExpression(exp)
	case *ast.MemberAccessExpression:
		return t.transpileMemberAccessExpression(exp)
	case *ast.IndexExpression:
		return t.transpileIndexExpression(exp)
	case *ast.EnumeratedValueLiteral:
		return t.transpileEnumeratedValueLiteral(exp)
	case *ast.FunctionLiteral:
		return t.transpileFunctionLiteral(exp)
	case *ast.HashLiteral:
		return t.transpileHashLiteral(exp)
	case *ast.ArrayRepetition:
		// This is handled by transpileArrayLiteral, but we add a case to be safe.
		return t.transpileArrayRepetition(exp)
	case *ast.MacroLiteral:
		// This case should no longer be hit, as macros are expanded before this function is called.
		return fmt.Errorf("unhandled macro literal found during code generation pass")
	case *ast.ArrayLiteral:
		return t.transpileArrayLiteral(exp)

	default:
		log.Printf("Warning: Unhandled expression type in transpiler: %T", exp)
		t.write("/* unhandled_expression: %T */", exp)
	}
	return nil
}

// transpileTypeBlockDeclaration transpiles an IEC 61131-3 `TYPE...END_TYPE` block.
// It generates Go struct definitions for `STRUCT` types, Go `const` blocks for `ENUM` types,
// and Go `type` aliases for subrange types.
func (t *Transpiler) transpileTypeBlockDeclaration(tbd *ast.TypeBlockDeclaration) error {
	for _, decl := range tbd.Declarations {
		// Transpile comments associated with each individual type declaration inside the block.
		t.transpileLeadingComments(decl.LeadingComments)

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

// mapIecTypeToGo converts an AST expression representing an IEC 61131-3 data type
// into its corresponding Go type string, typically prefixed with `iec.` for built-in types.
// mapIecTypeToGo converts an AST expression representing an IEC type
// into the corresponding Go type string from the `iec` package.
func (t *Transpiler) mapIecTypeToGo(dataType ast.Expression) string {
	// The DataType is an expression, which for simple types is an Identifier.
	// The DataType is an expression. For simple types, it's an ast.TypeSpecifier.
	if ts, ok := dataType.(*ast.TypeSpecifier); ok { // e.g., INT, or MyStruct
		typeName := ts.Token.Literal
		// Check if it's a user-defined type (STRUCT, ENUM, etc.) we've registered.
		if _, isUserDefined := t.typeInfo[typeName]; isUserDefined {
			// It's a type we've defined in this package, so just use its name.
			return typeName
		}

		// Convert to uppercase to match standard IEC types (e.g., 'int' -> 'INT').
		iecType := strings.ToUpper(typeName)
		// It's a standard built-in type, so prefix with the 'iec' package.
		return "iec." + iecType
	}
	if arrayDef, ok := dataType.(*ast.ArrayDefinition); ok { // cspell:disable-line
		// Recursively call mapIecTypeToGo for the element type.
		// This correctly handles multi-dimensional arrays (e.g., ARRAY OF ARRAY OF INT).
		elemType := t.mapIecTypeToGo(arrayDef.DataType)
		return "[]" + elemType
	}

	// Handle more complex types like ARRAY or STRUCT here in the future.
	log.Printf("Warning: Unhandled data type expression in transpiler: %s", dataType.String())
	return "any /* unhandled type */"
}

// transpileInfixExpression transpiles an IEC 61131-3 infix expression (e.g., `A + B`, `X AND Y`)
// into a Go infix expression, mapping IEC operators to their Go equivalents.
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

// transpileIfStatement transpiles an IEC 61131-3 `IF...THEN...ELSIF...ELSE...END_IF` statement
// into a Go `if...else if...else` construct.
func (t *Transpiler) transpileIfStatement(stmt *ast.IfStatement) error {
	t.write("\tif ")                      // Add tab for the if statement itself
	t.transpileExpression(stmt.Condition) // A function to convert expressions
	t.write(" {\n")
	// Manually transpile block to add extra indentation
	for _, s := range stmt.Consequence.Statements {
		t.write("\t")
		if err := t.transpileNode(s); err != nil {
			return err
		}
	}
	t.write("\t}")

	// Handle ELSIF and ELSE
	alt := stmt.Alternative
	for alt != nil {
		if elseifStmt, ok := alt.(*ast.IfStatement); ok {
			t.write(" else if ")
			t.transpileExpression(elseifStmt.Condition)
			t.write(" {\n")
			for _, s := range elseifStmt.Consequence.Statements {
				t.write("\t")
				if err := t.transpileNode(s); err != nil {
					return err
				}
			}
			t.write("\t}")
			alt = elseifStmt.Alternative
		} else { // This is the final ELSE block
			t.write(" else {\n")
			if elseBlock, ok := alt.(*ast.BlockStatement); ok {
				for _, s := range elseBlock.Statements {
					t.write("\t")
					if err := t.transpileNode(s); err != nil {
						return err
					}
				}
			}
			t.write("\t}")
			alt = nil
		}
	}
	t.write("\n")
	return nil
}

// transpileTypedLiteral transpiles an IEC 61131-3 typed literal (e.g., `INT#10`, `T#5s`)
// into its corresponding Go representation, often using helper functions from the `iec` package.
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

// transpilePrefixExpression transpiles an IEC 61131-3 prefix expression (e.g., `NOT X`, `-Y`)
// into a Go prefix expression, mapping IEC operators to their Go equivalents.
func (t *Transpiler) transpilePrefixExpression(exp *ast.PrefixExpression) error {
	t.write("(")
	op := exp.Operator
	if strings.ToUpper(op) == "NOT" {
		op = "!"
	}
	t.write("%s", op)
	if err := t.transpileExpression(exp.Right); err != nil {
		return err
	}
	t.write(")")
	return nil
}

// transpileCallExpression transpiles an IEC 61131-3 function call or function block invocation.
// It distinguishes between standard function calls and function block calls (which involve
// setting inputs, calling a `Logic` method, and handling outputs).
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
				t.write("\t")
				// Transpiles to: p.MyFB.InOutVar = &p.MyProgramVar
				t.transpileExpression(exp.Function)
				t.write(".%s = &", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			} else {
				// Pass by value for VAR_INPUT
				t.write("\t")
				// Transpiles to: p.MyTimer.IN = ...
				t.transpileExpression(exp.Function)
				t.write(".%s = ", namedArg.Name.Value)
				t.transpileExpression(namedArg.Value)
				t.write("\n")
			}
		}
	}

	// 2. Call the Logic() method.
	t.write("\t")
	t.transpileExpression(exp.Function) // e.g., p.MyTimer
	t.write(".Logic(now)\n")            // Pass the 'now' timestamp

	// 3. Handle output arguments (e.g., Q => MyVar)
	for _, arg := range exp.Arguments {
		if outArg, ok := arg.(*ast.OutputArgument); ok {
			t.write("\t")
			// Transpile `p.MyVar = p.MyTimer.Q`
			t.transpileExpression(outArg.Target)
			t.write(" = ")
			t.transpileExpression(exp.Function)
			t.write(".%s\n", outArg.Source.Value)
		}
	}

	return nil
}

// isInOutArgument checks if a given argument name for a function block or function
// corresponds to a `VAR_IN_OUT` parameter, indicating it should be passed by reference.
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

// transpileStandardFunctionCall transpiles a standard IEC 61131-3 function call
// (e.g., `SIN(X)`) into a direct Go function call.
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

// transpileEnumDefinition transpiles an IEC 61131-3 enumerated type definition
// into a Go `int` type with associated `const` declarations for its members (using `iota`).
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

// transpileEnumeratedValueLiteral transpiles an IEC 61131-3 enumerated value literal
// (e.g., `COLOR#RED`) into its Go `const` equivalent (e.g., `COLOR_RED`).
func (t *Transpiler) transpileEnumeratedValueLiteral(evl *ast.EnumeratedValueLiteral) error {
	t.write("%s_%s", evl.TypeName.Value, evl.Value.Value)
	return nil
}

func (t *Transpiler) transpileArrayLiteral(al *ast.ArrayLiteral) error {
	// We need to infer the type of the array's elements to generate the correct Go literal.
	// We'll inspect the first element. This is a simplification.
	elemType := ""
	if len(al.Elements) > 0 {
		var firstElem ast.Expression
		firstElem = al.Elements[0]

		// If the first element is a repetition, look at the type of the element being repeated.
		if rep, ok := firstElem.(*ast.ArrayRepetition); ok && len(rep.Elements) > 0 {
			firstElem = rep.Elements[0]
		}

		// If the first element is another array, it's a multi-dimensional array.
		// In this case, we don't specify the inner type, letting it be inferred recursively.
		if _, isArray := firstElem.(*ast.ArrayLiteral); isArray {
			// This will result in `[][]...{...}` which is what we want.
			elemType = ""
		} else if typedLit, ok := firstElem.(*ast.TypedLiteral); ok {
			elemType = "iec." + strings.ToUpper(typedLit.TypeName)
		} else if _, isInt := firstElem.(*ast.IntegerLiteral); isInt {
			// Heuristic: if we see an integer literal, assume the array type is INT.
			// This is not perfect but covers many common cases.
			elemType = "iec.INT"
		}
		// For other literal types (REAL, BOOL, STRING), Go can often infer the type,
	}

	t.write("[]%s{", elemType)
	for i, el := range al.Elements {
		if i > 0 {
			t.write(", ")
		}
		// The expression transpiler will handle ArrayRepetition now.
		if err := t.transpileExpression(el); err != nil {
			// Cannot return error from here easily, log it.
			log.Printf("Error transpiling array element: %v", err)
		}
	}
	t.write("}")
	return nil
}

// findHighestPriorityAction determines the effective action association for an action
// within a step, based on qualifier priority (R > S > others).
func findHighestPriorityAction(associations []*ast.ActionBlockStatement) *ast.ActionBlockStatement {
	if len(associations) == 0 {
		return nil
	}

	var rAssoc, sAssoc, otherAssoc *ast.ActionBlockStatement

	for _, assoc := range associations {
		qualifier := "N"
		if assoc.Qualifier != nil {
			qualifier = assoc.Qualifier.Value
		}

		switch strings.ToUpper(qualifier) {
		case "R":
			rAssoc = assoc
		case "S":
			sAssoc = assoc
		default:
			if otherAssoc == nil {
				otherAssoc = assoc
			}
		}
	}

	if rAssoc != nil {
		return rAssoc // R has highest priority
	}
	if sAssoc != nil {
		return sAssoc // S has second highest
	}
	return otherAssoc // Return any other qualifier, or nil if none
}

// transpileArrayRepetition transpiles an array repetition factor (e.g., `3(0)`)
// by repeating the elements the specified number of times in the Go array literal.
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

// transpileMemberAccessExpression transpiles an IEC 61131-3 member access expression
// (e.g., `MyStruct.Field1`) into a Go struct field access.
func (t *Transpiler) transpileMemberAccessExpression(exp *ast.MemberAccessExpression) error {
	if err := t.transpileExpression(exp.Struct); err != nil {
		return err
	}
	t.write(".%s", exp.Member.Value)
	return nil
}

// transpileIndexExpression transpiles an IEC 61131-3 array index expression
// (e.g., `MyArray[Index]`) into a Go array index access.
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

// getFunctionBlockDefinition retrieves the `FunctionBlockDeclaration` AST node
// for a given function block instance expression, by looking up its type in the symbol table.
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

// transpileFunctionLiteral is a placeholder for transpiling IEC 61131-3 function literals.
// Direct transpilation to Go anonymous functions with the same semantics is complex.
func (t *Transpiler) transpileFunctionLiteral(fl *ast.FunctionLiteral) error {
	// This implementation assumes the AST has been extended to support typed
	// anonymous functions, including Parameters and a ReturnType.
	if fl.Parameters == nil || fl.ReturnType == nil {
		t.write("func() { panic(\"anonymous functions must have explicit types for transpilation\") }")
		return nil
	}

	// --- 1. Build the function signature ---
	t.write("func(")
	params := []string{}
	for _, p := range fl.Parameters {
		goType := t.mapIecTypeToGo(p.DataType)
		params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
	}
	t.write("%s", strings.Join(params, ", "))
	t.write(")")

	// Handle return type. VOID means no return value in Go.
	if returnTypeSpec, ok := fl.ReturnType.(*ast.TypeSpecifier); !ok || strings.ToUpper(returnTypeSpec.Token.Literal) != "VOID" {
		t.write(" %s", t.mapIecTypeToGo(fl.ReturnType))
	}

	t.write(" {\n")

	// --- 2. Transpile the body ---
	// The body is transpiled within the context of the function literal.
	// We need to ensure that variable references inside are not prefixed with the program receiver.
	originalProgramVarName := t.programVarName
	t.programVarName = "" // No receiver inside anonymous function
	defer func() { t.programVarName = originalProgramVarName }()

	t.transpileNode(fl.Body)

	t.write("}") // End of anonymous function
	return nil
}

// transpileHashLiteral is a placeholder for transpiling IEC 61131-3 hash literals.
// It currently generates a Go `map[string]interface{}`.
func (t *Transpiler) transpileHashLiteral(hl *ast.HashLiteral) error {
	// Transpiling hash literals (maps).
	// Since IEC 61131-3 does not have a standard map type, we generate a Go map.
	// We use map[any]any to be flexible, as beedance allows various hashable key types.
	t.write("map[any]any{\n")

	// Sort keys for deterministic output
	keys := make([]ast.Expression, 0, len(hl.Pairs))
	for k := range hl.Pairs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

	for _, key := range keys {
		t.transpileExpression(key)
		t.write(": ")
		t.transpileExpression(hl.Pairs[key])
		t.write(",\n")
	}
	t.write("}")
	return nil
}
