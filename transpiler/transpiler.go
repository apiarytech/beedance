// In beedance/transpiler/transpiler.go
package transpiler

import (
	"fmt"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/token"
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
	w io.Writer
	// PreferOSCAT makes a name that both OSCAT BASIC (beebread) and royaljelly
	// define, such as ROUND or CEIL, OSCAT's. It is royaljelly's otherwise.
	PreferOSCAT bool
	// HostBinding generates New<Program>() and Variables() for each PROGRAM
	// (see Options.HostBinding).
	HostBinding      bool
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
	globalDecls      []*ast.VarDeclStatement         // The global variables' declarations
	typeInfo         map[string]ast.Node
	currentSetter    *ast.PropertyDeclaration               // The current property setter being transpiled
	isDereferencing  bool                                   // Flag to prevent double-dereferencing
	macroDefinitions map[string]*ast.MacroLiteral           // Stores macro definitions for expansion
	arrayDecls       map[string]*ast.ArrayDefinition        // Declared array types of the variables in scope, for their bounds
	localVars        map[string]bool                        // Parameters, results and locals of the current function or method
	initVisiting     map[*ast.FunctionBlockDeclaration]bool // Function blocks whose Init is being generated
	expectedGoType   string                                 // The elementary Go type the expression being transpiled is assigned to, or ""
	usesStdValue     bool                                   // The output calls stdValue, so it is written at the end
	usesStringCut    bool                                   // The output calls stringCut, so it is written at the end
	usesPlcTime      bool                                   // The output calls plcTime, so it is written at the end
	usesProcessImage bool                                   // The output uses processImage, so it is declared at the end
	configuredVars   map[string][]string                    // The variables VAR_CONFIG sets, by program type
}

// New creates a new Transpiler instance with the given io.Writer.
func New(w io.Writer) *Transpiler {
	return &Transpiler{
		w:                w,
		locatedVars:      make(map[string]bool),
		mainGenerated:    false,
		accessVars:       make(map[string]bool),
		tempVars:         make(map[string]bool),
		inOutVars:        make(map[string]bool),
		globalVars:       make(map[string]bool),
		varInfo:          make(map[string]*ast.TypeDeclaration),
		typeInfo:         make(map[string]ast.Node),
		currentSetter:    nil,
		isDereferencing:  false,
		macroDefinitions: make(map[string]*ast.MacroLiteral),
	}
}

// Transpile is the main entry point for the transpilation process. It orchestrates
// a two-pass approach: first expanding macros, then generating Go code.
func (t *Transpiler) Transpile(programAST *ast.Program) error { // Changed parameter name
	// The main logic, including macro expansion, is now handled within the
	// *ast.Program case in transpileNode.
	return t.transpileNode(programAST)
}

// transpileNode is the recursive heart of the transpiler, handling code generation for each AST node.
func (t *Transpiler) transpileNode(node ast.Node) error {
	// Check if the node has leading comments and transpile them.
	if nodeWithComments, ok := node.(interface{ GetLeadingComments() []string }); ok {
		t.transpileLeadingComments(nodeWithComments.GetLeadingComments())
	}

	switch node := node.(type) {
	case *ast.Program:
		// This is the top-level entry point. Perform macro expansion as the first pass.
		macroEnv := object.NewEnvironment()
		evaluator.DefineMacros(node, macroEnv)
		expandedAST := evaluator.ExpandMacros(node, macroEnv).(*ast.Program)
		// Go has no namespaces within a package; see namespaces.go.
		expandedAST = flattenNamespaces(expandedAST)
		// Uses of a name spelled as it is declared; see name_case.go.
		expandedAST = matchNameCase(expandedAST)
		// Names Go cannot use, such as `go` or `len`; see go_names.go.
		expandedAST = renameForGo(expandedAST)

		// Now, continue with code generation on the expanded AST.
		t.buildTypeInfo(expandedAST) // First pass to collect type definitions
		t.buildGlobalVarInfo(expandedAST)
		t.collectConfiguredVars(expandedAST)
		for _, stmt := range expandedAST.Statements {
			if err := t.transpileNode(stmt); err != nil {
				return err
			}
		}
		if t.usesStdValue {
			t.write("%s", stdValueHelper)
		}
		if t.usesStringCut {
			t.write("%s", stringCutHelper)
		}
		if t.usesPlcTime {
			t.write("%s", plcTimeHelper)
		}
		if t.usesProcessImage {
			t.write("%s", processImageDecl)
		}
		return nil
	case *ast.ProgramDeclaration:
		t.write("\n")
		// First, transpile any global var blocks that might exist at the program level
		if err := t.transpileGlobalVarBlocks(node.VarGlobal); err != nil {
			return err
		}
		return t.transpileProgram(node) // This will call transpileNode recursively
	case *ast.BlockStatement:
		return t.transpileBlockStatement(node) // This will call transpileNode recursively
	case *ast.VarDeclStatement:
		return t.transpileVarDecl(node)
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
		t.write("\t")
		if err := t.transpileExpression(node.Expression); err != nil {
			return err
		}
		t.write("\n")
		return nil

	case *ast.FunctionBlockDeclaration:
		t.write("\n")
		return t.transpileFunctionBlockDeclaration(node) // This will call transpileNode recursively
	case *ast.InterfaceDeclaration:
		t.write("\n")
		return t.transpileInterfaceDeclaration(node)
	case *ast.TypeBlockDeclaration:
		t.write("\n")
		return t.transpileTypeBlockDeclaration(node) // This will call transpileNode recursively
	case *ast.ConfigurationDeclaration:
		t.write("\n")
		return t.transpileConfigurationDeclaration(node) // This will call transpileNode recursively
	case *ast.SFCProgram:
		// This is handled within transpileProgram/transpileFunctionBlockDeclaration
		// but we add a case to prevent "unhandled type" errors if it appears elsewhere.
		return nil
	case *ast.GlobalVarDeclaration:
		t.write("\n")
		return t.transpileGlobalVarBlocks([]*ast.GlobalVarDeclaration{node})
	case *ast.FunctionDeclaration:
		t.write("\n")
		return t.transpileFunctionDeclaration(node) // This will call transpileNode recursively
	case *ast.VarBlockDeclaration:
		// This handles VAR blocks that might appear as statements inside a body.
		// We treat them as local variable declarations.
		for _, decl := range node.Declarations {
			if err := t.transpileLocalVar(decl); err != nil {
				return err
			}
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
		// Each line of a comment that spans several lines is a Go comment.
		for _, line := range strings.Split(strings.TrimSpace(comment), "\n") {
			t.write("// %s\n", strings.TrimRight(line, " \t\r"))
		}
	}
}

// buildGlobalVarInfo performs a pass to collect all VAR_GLOBAL names.
func (t *Transpiler) buildGlobalVarInfo(program *ast.Program) {
	for _, stmt := range program.Statements {
		switch node := stmt.(type) {
		case *ast.GlobalVarDeclaration:
			for _, decl := range node.Vars {
				t.globalVars[decl.Name.Value] = true
				t.globalDecls = append(t.globalDecls, decl)
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
func (t *Transpiler) transpileGlobalVarBlocks(blocks []*ast.GlobalVarDeclaration) error {
	for _, block := range blocks {
		t.write("// --- VAR_GLOBAL ---\n")
		for _, decl := range block.Vars {
			goType := t.mapIecTypeToGo(decl.DataType)
			t.write("var %s %s", decl.Name.Value, goType)
			// Global starting values have no receiver.
			originalReceiver := t.programVarName
			t.programVarName = ""
			v, err := t.initialValue(decl.DataType, decl.Value)
			t.programVarName = originalReceiver
			if err != nil {
				return fmt.Errorf("initial value of global '%s': %w", decl.Name.Value, err)
			}
			if v != "" {
				t.write(" = %s", v)
			}
			t.write("\n")
		}
		t.write("\n")
	}
	return nil
}

// transpileProgram transpiles an IEC 61131-3 PROGRAM POU into a Go struct and associated methods.
// This includes generating the struct definition for program variables, a factory function for instantiation,
// and a `Logic` method that contains the program's executable code (ST, IL, or SFC).
func (t *Transpiler) transpileProgram(prog *ast.ProgramDeclaration) error {
	t.programVarName = "p" // Set the receiver name

	// --- 1. Generate the struct definition ---
	// --- Scan for inferred variables from short assignment statements (:=) ---
	// This needs to happen before the struct definition is written.
	// Build var info for this program's scope first, so that inferred variables
	// are correctly identified (i.e., not already explicitly declared).
	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	originalArrayDecls := t.arrayDecls
	t.arrayDecls = map[string]*ast.ArrayDefinition{}
	defer func() { t.arrayDecls = originalArrayDecls }()
	t.buildVarInfo(prog.VarInputs, prog.VarOutputs, prog.VarInOuts, prog.Vars)
	originalInOutVars := t.inOutVars
	t.inOutVars = map[string]bool{}
	for _, decl := range prog.VarInOuts {
		t.inOutVars[decl.Name.Value] = true
	}
	defer func() { t.inOutVars = originalInOutVars }()

	// Build temp var info first, so we can exclude them from inferred struct fields.
	originalTempVars := t.tempVars
	t.tempVars = make(map[string]bool)
	t.buildTempVarInfo(prog.VarTemp)
	defer func() { t.tempVars = originalTempVars }()

	inferredVars := make(map[string]ast.Expression)
	if body, ok := prog.Body.(*ast.BlockStatement); ok {
		for _, stmt := range body.Statements {
			if assign, ok := stmt.(*ast.AssignmentStatement); ok && assign.Token.Type == token.ASSIGN {
				if ident, ok := assign.Left.(*ast.Identifier); ok {
					// If the variable is not already declared (in VAR, VAR_INPUT, etc.) and is not a VAR_TEMP,
					// then it's an inferred variable that needs to be a field on the struct.
					if _, exists := t.varInfo[ident.Value]; !exists {
						if _, isTemp := t.tempVars[ident.Value]; !isTemp {
							if _, isGlobal := t.globalVars[ident.Value]; !isGlobal {
								// We can't know the exact type, so we'll use 'any' in the Go struct.
								inferredVars[ident.Value] = nil // Using nil as a placeholder for 'any'
							}
						}
					}
				}
			}
		}
	}

	originalAccessVars := t.accessVars
	t.accessVars = make(map[string]bool)
	t.buildAccessVarInfo(prog.VarAccess)

	originalLocatedVars := t.locatedVars
	t.locatedVars = make(map[string]bool)
	t.buildLocatedVarInfo(prog.VarInputs, prog.VarOutputs, prog.Vars)
	defer func() { t.locatedVars = originalLocatedVars }()

	defer func() { t.accessVars = originalAccessVars }()

	defer func() { t.varInfo = originalVarInfo }() // Restore previous scope

	allVarBlocks := [][]*ast.VarDeclStatement{prog.VarInputs, prog.VarOutputs, prog.Vars}

	t.write("type %s struct {\n", prog.Name.Value)
	for _, varBlock := range allVarBlocks {
		for _, varDecl := range varBlock {
			if err := t.transpileVarDecl(varDecl); err != nil {
				return err
			}
		}
	}
	for _, varDecl := range prog.VarInOuts {
		t.transpileVarDeclInOut(varDecl)
	}
	for _, accessDecl := range prog.VarAccess {
		t.transpileVarAccess(accessDecl)
	}
	// Add inferred variables to the struct definition
	// Sort keys for deterministic output in tests
	inferredVarNames := make([]string, 0, len(inferredVars))
	for name := range inferredVars {
		inferredVarNames = append(inferredVarNames, name)
	}
	sort.Strings(inferredVarNames)
	for _, varName := range inferredVarNames {
		t.write("\t%s any // Inferred\n", varName)
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

	// Set starting values in declaration order. Initial values may refer to
	// variables declared before them, which are fields of instance.
	t.programVarName = "instance"
	for _, varBlock := range allVarBlocks {
		for _, varDecl := range varBlock {
			if err := t.transpileProgramVarInit(varDecl); err != nil {
				t.programVarName = "p"
				return err
			}
		}
	}
	t.programVarName = "p"
	// Values a configuration gives its variables (VAR_CONFIG).
	if err := t.transpileConfiguredVars(prog); err != nil {
		return err
	}

	// If it's an SFC program, set the initial step in the factory.
	if sfc, ok := prog.Body.(*ast.SFCProgram); ok {
		initialStep := t.findInitialStep(sfc)
		if initialStep != nil {
			t.write("\tinstance.sfcActiveSteps = make(map[string]bool)\n")
			t.write("\tinstance.sfcActiveSteps[%q] = true\n", initialStep.Name.Value)
		}
	}

	t.write("\treturn instance.Logic, nil\n")
	t.write("}\n\n")

	// --- 3. Generate the Logic method for the scheduler ---
	if err := t.transpileLinkAccess(prog); err != nil {
		return err
	}

	t.write("func (%s *%s) Logic(now time.Time) {\n", t.programVarName, prog.Name.Value)
	if err := t.transpileProcessImage(t.programVarName, allVarBlocks...); err != nil {
		return err
	}
	// Transpile VAR_TEMP as local variables inside the Logic method.
	for _, tempBlock := range prog.VarTemp {
		for _, decl := range tempBlock.Vars {
			if err := t.transpileLocalVar(decl); err != nil {
				return err
			}
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
	if t.HostBinding {
		if err := t.transpileHostBinding(prog, allVarBlocks); err != nil {
			return err
		}
	}
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
			if err := t.transpileIlProgram(block); err != nil {
				return err
			}
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
			if err := t.transpileExpression(stmt.Operand); err != nil {
				return err
			}
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
		if err := t.transpileExpression(stmt.Operand); err != nil {
			return err
		}
		t.write(" = %s(cr_%s)\n", targetType, crType) // e.g., p.MyInt = iec.INT(cr_LINT)

	case "ADD", "SUB", "MUL", "DIV", "AND", "OR", "XOR":
		// Determine the operator and the correct accumulator to use.
		goOp := t.mapIlOperatorToGo(op)
		crType := t.ilCurrentCRType
		if crType == "" {
			return fmt.Errorf("IL operator '%s' used before accumulator was loaded (LD)", op)
		}
		// The operand takes the accumulator's type, as it may be declared narrower.
		// Go has no bitwise operators on booleans; BOOL uses the logical ones.
		if crType == "BOOL" {
			goOp = map[string]string{"AND": "&&", "OR": "||", "XOR": "!="}[op]
		}
		t.write("\tcr_%s = cr_%s %s iec.%s(", crType, crType, goOp, crType)
		if err := t.transpileExpression(stmt.Operand); err != nil {
			return err
		}
		t.write(")\n")

	case "GT", "LT", "EQ", "NE", "GE", "LE":
		goOp := t.mapIlOperatorToGo(op)
		crType := t.ilCurrentCRType
		if crType == "" {
			return fmt.Errorf("IL operator '%s' used before accumulator was loaded (LD)", op)
		}
		// The result of a comparison is always BOOL.
		t.write("\tcr_BOOL = cr_%s %s iec.%s(", crType, goOp, crType)
		if err := t.transpileExpression(stmt.Operand); err != nil {
			return err
		}
		t.write(")\n")
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
		if err := t.transpileExpression(stmt.Operand); err != nil {
			return err
		}
		t.write(" = true }\n")

	case "R": // Reset
		// 'R' is conditional on the boolean accumulator.
		t.write("\tif cr_BOOL { ")
		if err := t.transpileExpression(stmt.Operand); err != nil {
			return err
		}
		t.write(" = false }\n")
	case "NOT":
		// NOT negates the current result: logically for BOOL, bitwise otherwise.
		crType := t.ilCurrentCRType
		if crType == "" {
			return fmt.Errorf("IL operator 'NOT' used before accumulator was loaded (LD)")
		}
		if crType == "BOOL" {
			t.write("\tcr_BOOL = !cr_BOOL\n")
		} else {
			t.write("\tcr_%s = ^cr_%s\n", crType, crType)
		}
	default:
		// Check if the operator is a standard built-in function (e.g., ABS, SQRT).
		if returnType, ok := builtInFunctionReturnTypes[op]; ok {
			crType := t.ilCurrentCRType
			if crType == "" {
				return fmt.Errorf("IL function '%s' used before accumulator was loaded (LD)", op)
			}
			// The result of a built-in function updates the accumulator.
			arg := "cr_" + crType
			call := op + "(" + arg + ")"
			if fn, isStd := royaljellyFunctions[op]; isStd {
				switch {
				case paramType(fn, 0) != "":
					arg = paramType(fn, 0) + "(" + arg + ")"
				case realOnlyFunctions[op] && crType != "LREAL":
					arg = "iec.LREAL(" + arg + ")"
				}
				call = goPackageAlias(fn.pkg) + "." + op + "(" + arg + ")"
				if fn.err {
					t.usesStdValue = true
					call = "stdValue(" + call + ")"
				}
			}
			t.write("\tcr_%s = iec.%s(%s)\n", returnType, returnType, call)
			t.ilCurrentCRType = returnType
		} else {
			// An operator that cannot be transpiled must not be skipped silently.
			return fmt.Errorf("IL operator '%s' is not supported by the transpiler", op)
		}
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
				if err := t.transpileExpression(callExpr.Function); err != nil {
					return err
				}
				t.write(".%s = &", t.memberName(callExpr.Function, namedArg.Name.Value))
				if err := t.transpileExpression(namedArg.Value); err != nil {
					return err
				}
				t.write("\n")
			} else {
				t.write("\t")
				if err := t.transpileExpression(callExpr.Function); err != nil {
					return err
				}
				t.write(".%s = ", t.memberName(callExpr.Function, namedArg.Name.Value))
				if err := t.transpileValue(namedArg.Value, t.inputGoType(callExpr.Function, namedArg.Name)); err != nil {
					return err
				}
				t.write("\n")
			}
		}
	}

	// 2. Transpile the call to the Logic() method.
	t.write("\t")
	if err := t.transpileExpression(callExpr.Function); err != nil {
		return err
	}
	t.write(".%s\n", t.functionBlockRun(callExpr.Function))

	// 3. Identify the primary output (first VAR_OUTPUT) to load into the accumulator.
	fbDef := t.getFunctionBlockDefinition(callExpr.Function)
	if fbDef == nil {
		// This could be a standard library FB like TON. Check our lookup table.
		fbTypeName := ""
		if ident, ok := callExpr.Function.(*ast.Identifier); ok {
			// It's an instance variable like 'my_timer'. We need its type.
			if varTypeDecl := t.varInfo[ident.Value]; varTypeDecl != nil {
				// The DataType is an expression. For simple types, it's an ast.TypeSpecifier.
				// The String() method on the expression node gives us the type name.
				fbTypeName = strings.ToUpper(varTypeDecl.DataType.String())
			}
		}

		if primaryOut, ok := standardFBPrimaryOutputs[fbTypeName]; ok {
			// It's a known standard FB.
			t.write("\tcr_%s = ", primaryOut.Type)
			if err := t.transpileExpression(callExpr.Function); err != nil {
				return err
			}
			t.write(".%s\n", primaryOut.Name)
			t.ilCurrentCRType = primaryOut.Type
			return nil
		}

		// If not found, fallback to the old warning and assumption.
		log.Printf("Warning: Could not find user definition for FB %s. Assuming primary output 'Q' of type BOOL.", callExpr.Function.String())
		t.write("\tcr_BOOL = ")
		if err := t.transpileExpression(callExpr.Function); err != nil {
			return err
		}
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
	if err := t.transpileExpression(callExpr.Function); err != nil {
		return err
	}
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
	// The result takes the accumulator's type, as the function may return a
	// narrower one.
	t.write("\tcr_%s = iec.%s(", crType, crType)

	// 2. Transpile the call itself. A user-defined function takes its
	// arguments in declaration order, with omitted inputs defaulted.
	if sig, ok := t.lookupCallSignature(callExpr.Function); ok {
		if err := t.transpileUserCall(callExpr, sig); err != nil {
			return err
		}
	} else if err := t.transpileStandardFunctionCall(callExpr); err != nil {
		return err
	}
	t.write(")\n")

	return nil
}

// isIlBlock checks if a given block statement contains IL instructions, indicating it's an IL program body.
func isIlBlock(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	// A more robust check involves looking at all statements, as a function
	// declaration might appear before the first IL instruction.
	for _, stmt := range block.Statements {
		if _, isIL := stmt.(*ast.IlInstructionStatement); isIL {
			return true
		}
	}
	return false
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
			t.write("\t%s_X_prev bool\n", step.Name.Value) // For P qualifier
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
			if err := t.transpileExpression(trans.Condition); err != nil {
				return err
			}
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
					if err := t.transpileSFCAction(step, effectiveAssoc, true); err != nil {
						return err
					}
				}
			}
			t.write("\t} else {\n")
			// Transpile logic for when the step is NOT ACTIVE (for stored/timed actions).
			for _, actionName := range sortedActionNames {
				associations := actionsInStep[actionName]
				effectiveAssoc := findHighestPriorityAction(associations)
				if effectiveAssoc != nil {
					if err := t.transpileSFCAction(step, effectiveAssoc, false); err != nil {
						return err
					}
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
func (t *Transpiler) transpileSFCAction(step *ast.StepStatement, actionBlock *ast.ActionBlockStatement, isStepActive bool) error {
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
			t.write("\t\tp.%s_Q = iec.BOOL(p.%s_X && !p.%s_X_prev)\n", actionName, step.Name.Value, step.Name.Value)
		case "D": // Non-stored delayed
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; }\n", actionName, actionName)
			t.write("\t\tp.%s_Q = iec.TIME(now.Sub(p.%s_Timer)) >= ", actionName, actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write("\n")
		case "L": // Non-stored limited
			t.write("\t\tif p.%s_Timer.IsZero() { p.%s_Timer = now; }\n", actionName, actionName)
			t.write("\t\tp.%s_Q = iec.TIME(now.Sub(p.%s_Timer)) < ", actionName, actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write("\n")
		case "SD", "DS": // Stored delayed
			t.write("\t\tif p.%s_Timer.IsZero() {\n\t\t\tp.%s_Timer = now\n\t\t}\n", actionName, actionName)
			// Only set Q to true, never to false. It must be reset by 'R'.
			t.write("\t\tif !p.%s_Q && iec.TIME(now.Sub(p.%s_Timer)) >= ", actionName, actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write(" {\n\t\t\tp.%s_Q = true\n\t\t}\n", actionName)
		case "SL": // Stored limited
			t.write("\t\tif p.%s_Timer.IsZero() {\n", actionName)
			t.write("\t\t\tp.%s_Timer = now\n", actionName)
			t.write("\t\t\tp.%s_Q = true\n", actionName)
			t.write("\t\t}\n")
			// Only set Q to false, never back to true.
			t.write("\t\tif iec.TIME(now.Sub(p.%s_Timer)) >= ", actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write(" {\n\t\t\tp.%s_Q = false\n\t\t}\n", actionName)
		}
	} else { // Step is not active
		switch qualifier {
		case "N", "R", "P", "D", "L": // For all non-stored qualifiers, deactivate the action and reset its timer.
			t.write("\t\tp.%s_Q = false\n", actionName)
			t.write("\t\tp.%s_Timer = time.Time{}\n", actionName) // Reset timer
		case "SD", "DS":
			// Stored delayed. If timer is running, check if it has elapsed to set Q to true.
			t.write("\t\tif !p.%s_Timer.IsZero() && !bool(p.%s_Q) && iec.TIME(now.Sub(p.%s_Timer)) >= ", actionName, actionName, actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write(" {\n\t\t\tp.%s_Q = true\n\t\t}\n", actionName)
		case "SL": // Stored-Limited
			// If timer is running, check if it has elapsed to set Q to false.
			t.write("\t\tif !p.%s_Timer.IsZero() && bool(p.%s_Q) && iec.TIME(now.Sub(p.%s_Timer)) >= ", actionName, actionName, actionName)
			if err := t.transpileExpression(actionBlock.Duration); err != nil {
				return err
			}
			t.write(" {\n\t\t\tp.%s_Q = false\n\t\t}\n", actionName)
			// For S, the state is maintained, so we do nothing here.
		}
	}
	return nil
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
		t.write("\t%s\n", fb.Extends.String())
	}

	originalVarInfo := t.varInfo
	t.varInfo = make(map[string]*ast.TypeDeclaration)
	originalArrayDecls := t.arrayDecls
	t.arrayDecls = map[string]*ast.ArrayDefinition{}
	defer func() { t.arrayDecls = originalArrayDecls }()
	t.buildVarInfo(fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars)
	originalInOutVars := t.inOutVars
	t.inOutVars = map[string]bool{}
	for _, decl := range fb.VarInOuts {
		t.inOutVars[decl.Name.Value] = true
	}
	defer func() { t.inOutVars = originalInOutVars }()

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
		if err := t.transpileVarDecl(varDecl); err != nil {
			return err
		}
	}
	for _, varDecl := range fb.VarOutputs {
		if err := t.transpileVarDecl(varDecl); err != nil {
			return err
		}
	}
	for _, varDecl := range fb.VarInOuts {
		t.transpileVarDeclInOut(varDecl)
	}
	for _, varDecl := range fb.Vars {
		if err := t.transpileVarDecl(varDecl); err != nil {
			return err
		}
	}
	// VAR_EXTERNAL variables are not part of the struct; they are global.
	// VAR_TEMP variables are local to the Logic() call, not fields of the struct.
	t.write("}\n\n")

	// The receiver name is specific to this FB.
	receiverName := receiverOf(fb.Name.Value)

	// Generate the Init method, which sets the starting values of an instance.
	if err := t.transpileFunctionBlockInit(fb, receiverName); err != nil {
		return err
	}

	// 2. Generate the Logic method for the Function Block.
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
	if err := t.transpileProcessImage(receiverName, fb.VarInputs, fb.VarOutputs, fb.Vars); err != nil {
		return err
	}

	// If this FB extends another, call the parent's Logic method first.
	if fb.Extends != nil {
		t.write("\t%s.%s.Logic(now)\n\n", receiverName, fb.Extends.String())
	}

	// Transpile VAR_TEMP as local variables inside the Logic method.
	for _, tempBlock := range fb.VarTemp {
		for _, decl := range tempBlock.Vars {
			if err := t.transpileLocalVar(decl); err != nil {
				return err
			}
		}
	}

	// Filter out MethodImplementations from the main body for the Logic method.
	mainLogicStatements := []ast.Statement{}
	if bodyBlock, ok := fb.Body.(*ast.BlockStatement); ok {
		for _, stmt := range bodyBlock.Statements {
			if _, isMethod := stmt.(*ast.MethodImplementation); !isMethod {
				mainLogicStatements = append(mainLogicStatements, stmt)
			}
		}
	}
	// Create a new BlockStatement for the main logic.
	mainLogicBody := &ast.BlockStatement{Statements: mainLogicStatements} // This will call transpileNode recursively

	if err := t.transpileNode(mainLogicBody); err != nil {
		t.programVarName = originalProgramVarName // Restore context on error
		return err
	}

	t.write("}\n\n")

	// 3. Generate methods for the Function Block. // This will call transpileNode recursively
	if body, ok := fb.Body.(*ast.BlockStatement); ok {
		for _, stmt := range body.Statements {
			if method, ok := stmt.(*ast.MethodImplementation); ok { // This will call transpileNode recursively
				if err := t.transpileMethodDeclaration(fb, method); err != nil {
					return err
				}
			}
		}
	}

	// 3.5. Generate properties for the Function Block. // This will call transpileNode recursively
	for _, prop := range fb.Properties {
		if err := t.transpilePropertyDeclaration(fb, prop); err != nil {
			return err
		}
	}

	// 4. Add static checks to ensure it implements the specified interfaces. // This will call transpileNode recursively
	if !fb.IsAbstract {
		// Collect all interfaces implemented by this FB and its parents.
		allInterfaces := []ast.Expression{}
		allInterfaces = append(allInterfaces, fb.Implements...)

		// Start with the current FB and walk up the inheritance chain.
		// A temporary variable `currentExtends` is used to traverse the chain.
		if fb.Extends != nil {
			currentExtendsExpr := fb.Extends
			for currentExtendsExpr != nil {
				parentFBDef := t.getFunctionBlockDefinitionFromTypeInfo(currentExtendsExpr.String())
				if parentFBDef == nil {
					break // Parent not found, stop traversing.
				}
				allInterfaces = append(allInterfaces, parentFBDef.Implements...)
				currentExtendsExpr = parentFBDef.Extends // Move to the next parent.
			}
		}

		uniqueInterfaces := make(map[string]ast.Expression)
		for _, iface := range allInterfaces {
			uniqueInterfaces[iface.String()] = iface
		}
		for _, iface := range uniqueInterfaces {
			t.write("// Statically assert that %s implements %s.\n", fb.Name.Value, iface.String())
			t.write("var _ %s = (*%s)(nil)\n\n", iface.String(), fb.Name.Value)
		}
	}

	t.programVarName = originalProgramVarName // Restore context
	return nil
}

// transpilePropertyDeclaration transpiles an IEC 61131-3 PROPERTY into Go getter/setter methods. // This will call transpileNode recursively
func (t *Transpiler) transpilePropertyDeclaration(fb *ast.FunctionBlockDeclaration, prop *ast.PropertyDeclaration) error {
	// Do not generate any code for abstract properties, as they have no body.
	if prop.IsAbstract { // This will call transpileNode recursively
		return nil
	}

	receiverName := receiverOf(fb.Name.Value)
	propName := prop.Name.Value
	goType := t.mapIecTypeToGo(prop.DataType)

	// Transpile the GET block
	if prop.Getter != nil {
		t.write("// Get%s is the getter for the %s property.\n", propName, propName)
		t.write("func (%s *%s) Get%s() (%s %s) {\n", receiverName, fb.Name.Value, propName, propName, goType)

		// Temporarily set the current function context so that assignments to the
		// property name set the getter's result, a named Go result.
		originalFunc := t.currentFunc
		t.currentFunc = &ast.FunctionDeclaration{Name: prop.Name}

		err := t.transpileResultValues(prop.Name, prop.DataType, nil)
		if err == nil {
			err = t.transpileNode(prop.Getter.Body) // This will call transpileNode recursively
		}
		t.currentFunc = originalFunc
		if err != nil {
			return err
		}
		t.write("\treturn\n")
		t.write("}\n\n")
	}

	// Transpile the SET block
	if prop.Setter != nil {
		t.write("// Set%s is the setter for the %s property.\n", propName, propName)
		t.write("func (%s *%s) Set%s(value %s) {\n", receiverName, fb.Name.Value, propName, goType)

		// In a SET block, the property name is an implicit input variable.
		// We set the currentSetter context to handle this during expression transpilation.
		originalSetter := t.currentSetter
		t.currentSetter = prop

		if err := t.transpileNode(prop.Setter.Body); err != nil { // This will call transpileNode recursively
			t.currentSetter = originalSetter // Restore on error
			return err
		}

		t.currentSetter = originalSetter
		t.write("}\n\n")
	}
	return nil
}

// transpileInterfaceDeclaration transpiles an IEC 61131-3 INTERFACE into a Go interface. // This will call transpileNode recursively
func (t *Transpiler) transpileInterfaceDeclaration(iface *ast.InterfaceDeclaration) error {
	t.write("// %s is the transpiled Go interface for the IEC 61131-3 INTERFACE of the same name.\n", iface.Name.Value)
	t.write("type %s interface {\n", iface.Name.Value)

	// Embed parent interfaces. Assumes parser adds `Extends` to `ast.InterfaceDeclaration`.
	if iface.Extends != nil {
		for _, parent := range iface.Extends {
			t.write("\t%s\n", parent.String())
		}
	}

	for _, method := range iface.Methods {
		// Build parameter list string for VAR_INPUT and VAR_IN_OUT.
		params := []string{}
		for _, p := range method.VarInputs {
			goType := t.mapIecTypeToGo(p.DataType)
			params = append(params, fmt.Sprintf("%s %s", p.Name.Value, goType))
		}
		for _, p := range method.VarInOuts {
			goType := t.mapIecTypeToGo(p.DataType)
			params = append(params, fmt.Sprintf("%s *%s", p.Name.Value, goType))
		}
		paramStr := strings.Join(params, ", ")

		// Build return type string for the primary return type and all VAR_OUTPUTs.
		returns := []string{}
		if method.ReturnType != nil {
			// A non-nil return type is a valid type to be returned.
			returns = append(returns, t.mapIecTypeToGo(method.ReturnType))
		}
		for _, p := range method.VarOutputs {
			returns = append(returns, t.mapIecTypeToGo(p.DataType))
		}

		returnStr := ""
		if len(returns) > 1 {
			returnStr = fmt.Sprintf(" (%s)", strings.Join(returns, ", "))
		} else if len(returns) == 1 {
			returnStr = " " + returns[0]
		}

		t.write("\t%s(%s)%s\n", method.Name.Value, paramStr, returnStr)
	}

	// Transpile properties into Get/Set methods
	for _, prop := range iface.Properties {
		goType := t.mapIecTypeToGo(prop.DataType)
		t.write("\tGet%s() %s\n", prop.Name.Value, goType)
		t.write("\tSet%s(value %s)\n", prop.Name.Value, goType)
	}

	t.write("}\n\n")
	return nil // This will call transpileNode recursively
}

// transpileMethodDeclaration transpiles an IEC 61131-3 METHOD into a Go method on the FB's struct.
func (t *Transpiler) transpileMethodDeclaration(fb *ast.FunctionBlockDeclaration, method *ast.MethodImplementation) error {
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

	// Named results, as for a function: the method's value, named after the
	// method, then its VAR_OUTPUTs.
	var returnType ast.Expression
	if method.ReturnType != nil {
		returnType = method.ReturnType
	}
	hasResults := t.transpileResultDeclarations(method.Name.Value, returnType, method.VarOutputs)
	t.write("{\n")

	// Set a temporary function context so that `MyMethod := ...` sets the result.
	originalFunc := t.currentFunc
	t.currentFunc = &ast.FunctionDeclaration{Name: method.Name, ReturnType: method.ReturnType}
	defer func() { t.currentFunc = originalFunc }()

	// A method sees its function block's variables as well as its own.
	defer t.withVarInfo(true, method.VarInputs, method.VarOutputs, method.VarInOuts, method.Vars)()
	return t.withLocals(declNames(method.VarInputs, method.VarOutputs, method.VarInOuts, method.Vars), func() error {
		// Starting values of the results and local variables (VAR).
		if err := t.transpileResultValues(method.Name, returnType, method.VarOutputs); err != nil {
			return err
		}
		for _, varDecl := range method.Vars {
			if err := t.transpileLocalVar(varDecl); err != nil {
				return err
			}
		}

		if err := t.transpileNode(method.Body); err != nil { // This will call transpileNode recursively
			return err
		}
		if hasResults && !endsWithReturn(method.Body) {
			t.write("\treturn\n")
		}
		t.write("}\n\n")
		return nil
	})
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
			// Store macro definitions
			if macroLit, ok := varDecl.Value.(*ast.MacroLiteral); ok {
				t.macroDefinitions[varDecl.Name.Value] = macroLit
				continue // Don't treat macros as regular variables for type info
			}
			if typeDecl == nil {
				typeDecl = &ast.TypeDeclaration{Name: &ast.Identifier{Value: typeName}, DataType: varDecl.DataType}
			}
			// A STRING(n) variable keeps its length, which an assignment cuts to.
			if varDecl.StringLength != nil && typeDecl.StringLength == nil {
				sized := *typeDecl
				sized.StringLength = varDecl.StringLength
				typeDecl = &sized
			}
			t.varInfo[varDecl.Name.Value] = typeDecl
			if def := t.arrayDefinitionOf(varDecl.DataType); def != nil {
				if t.arrayDecls == nil {
					t.arrayDecls = map[string]*ast.ArrayDefinition{}
				}
				t.arrayDecls[varDecl.Name.Value] = def
			}
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
		if _, ok := standardFBPrimaryOutputs[strings.ToUpper(typeName)]; ok {
			return true
		}
		if b, ok := t.beebreadTypeOf(dataType); ok && b.isFB {
			return true
		}
	}
	return false
}

// isReferenceType checks if a given AST expression representing a data type
// is a REFERENCE TO type.
func (t *Transpiler) isReferenceType(dataType ast.Expression) bool {
	_, isRef := dataType.(*ast.ReferenceType)
	return isRef
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

	// Named results: the function's value, named after the function, then its
	// VAR_OUTPUTs. Assigning to the function's name sets its value without
	// returning; RETURN, or the end of the body, returns every result.
	var returnType ast.Expression
	if fd.ReturnType != nil {
		returnType = fd.ReturnType
	}
	hasResults := t.transpileResultDeclarations(fd.Name.Value, returnType, fd.VarOutputs)
	t.write("{\n")

	// The declared types of parameters and locals, for references and subranges.
	defer t.withVarInfo(false, fd.VarInputs, fd.VarOutputs, fd.VarInOuts, fd.Vars)()
	return t.withLocals(declNames(fd.VarInputs, fd.VarOutputs, fd.VarInOuts, fd.Vars), func() error {
		// --- 2. Starting values of the results and local variables (VAR) ---
		if err := t.transpileResultValues(fd.Name, returnType, fd.VarOutputs); err != nil {
			return err
		}
		for _, v := range fd.Vars {
			if err := t.transpileLocalVar(v); err != nil {
				return err
			}
		}
		// Go does not allow a local that is never used.
		used := usedNames(fd.Body)
		for _, v := range fd.Vars {
			if !used[v.Name.Value] {
				t.write("\t_ = %s\n", v.Name.Value)
			}
		}
		t.write("\n")

		// --- 3. Transpile the function body ---
		if err := t.transpileNode(fd.Body); err != nil {
			return err
		}
		if hasResults && !endsWithReturn(fd.Body) {
			t.write("\treturn\n")
		}
		t.write("}\n\n")
		return nil
	})
}

// transpileVarDecl transpiles a single variable declaration (VAR, VAR_INPUT, VAR_OUTPUT, VAR_TEMP)
// into a Go struct field. It handles located variables by making them pointers.
func (t *Transpiler) transpileVarDecl(varDecl *ast.VarDeclStatement) error {
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
				return fmt.Errorf("cannot instantiate abstract function block '%s' for variable '%s'", typeName, varDecl.Name.Value)
			}
		}
	}
	// Transpile any leading comments associated with this variable declaration.
	// If the variable is a macro definition, skip it entirely as it has no
	// runtime equivalent in the transpiled code.
	if _, ok := varDecl.Value.(*ast.MacroLiteral); ok {
		return nil
	}

	t.transpileLeadingComments(varDecl.LeadingComments)

	// Get the variable name.
	// A located variable is copied from and to the process image each scan.
	if varDecl.Location != nil {
		goType := t.mapIecTypeToGo(varDecl.DataType)
		t.write("\t%s %s // AT %s\n", varDecl.Name.Value, goType, varDecl.Location.Location.String())
		return nil
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

			if fnLit.ReturnType != nil {
				t.write(" %s", t.mapIecTypeToGo(fnLit.ReturnType))
			}
			t.write("\n")
			return nil
		}
		// Fallback for a FUNCTION variable without a literal assignment.
		t.write("\t%s func()\n", varDecl.Name.Value)
		return nil
	}

	name := varDecl.Name.Value

	// Get the Go type for the IEC data type.
	goType := t.mapIecTypeToGo(varDecl.DataType)

	// Write the struct field. e.g., "MyCounter iec.LINT"
	t.write("\t%s %s\n", name, goType)

	return nil
}

// transpileVarAccess transpiles a VAR_ACCESS declaration. It infers the type of the accessed variable
// and declares a pointer field in the Go struct to represent the access path.
func (t *Transpiler) transpileVarAccess(accessDecl *ast.AccessVarDeclaration) {
	for _, decl := range accessDecl.Vars {
		// Transpile any leading comments associated with this variable declaration.
		t.transpileLeadingComments(decl.LeadingComments)

		// A pointer to the variable the access path names; see LinkAccess.
		t.write("\t%s *%s // VAR_ACCESS %s\n", decl.Name.Value, t.accessVarGoType(decl), decl.AccessPath.String())
	}
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
		if err := t.transpileNode(caseElem.Consequence); err != nil {
			return err
		}
	}

	if stmt.Alternative != nil {
		t.write("\tdefault:\n")
		if err := t.transpileNode(stmt.Alternative); err != nil {
			return err
		}
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
				if err := t.transpileExpression(infix.Left); err != nil {
					return err
				}
				t.write(" && caseSelector <= ")
				if err := t.transpileExpression(infix.Right); err != nil {
					return err
				}
				t.write(")")
			} else {
				// Simple value: caseSelector == V
				t.write("(caseSelector == ")
				if err := t.transpileExpression(val); err != nil {
					return err
				}
				t.write(")")
			}
		}
		t.write(" {\n")
		if err := t.transpileNode(caseElem.Consequence); err != nil {
			return err
		}
		t.write("\t}")
	}

	// Handle the final ELSE block
	if stmt.Alternative != nil {
		t.write(" else {\n")
		if err := t.transpileNode(stmt.Alternative); err != nil {
			return err
		}
		t.write("\t}")
	}
	t.write("\n")

	return nil
}

// isBitwiseType is a helper to infer if an expression is likely to be a bitwise type (WORD, BYTE, etc.).
// This is a heuristic for the transpiler to differentiate between logical (&&) and bitwise (&) operators.
func (t *Transpiler) isBitwiseType(expr ast.Expression) bool {
	// An integer or bit string of a type the transpiler knows.
	if typ := t.exprGoType(expr); typ != "" {
		return isIntegerGoType(typ)
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if typeDecl, ok := t.varInfo[e.Value]; ok {
			typeStr := strings.ToUpper(typeDecl.DataType.String())
			return typeStr == "BYTE" || typeStr == "WORD" || typeStr == "DWORD" || typeStr == "LWORD"
		}
		return false
	case *ast.InfixExpression:
		// If it's an infix expression, its "bitwiseness" depends on its operands.
		// If either operand is bitwise, the operation is bitwise.
		return t.isBitwiseType(e.Left) || t.isBitwiseType(e.Right)
	case *ast.PrefixExpression:
		// The "bitwiseness" of a prefix expression depends on its operand.
		return t.isBitwiseType(e.Right)
	default:
		return false // Cannot infer type for other complex expressions.
	}
}

// transpileAssignmentStatement transpiles an IEC 61131-3 assignment (`:=`) into a Go assignment (`=`).
// transpileAssignmentStatement transpiles an IEC `:=` assignment to a Go `=` assignment.
func (t *Transpiler) transpileAssignmentStatement(stmt *ast.AssignmentStatement) error {
	// A bit assignment, flags.3 := value.
	if bit, ok := stmt.Left.(*ast.BitAccessExpression); ok {
		return t.transpileBitWrite(bit, stmt.Value)
	}
	// Assignment to the function's name sets its result, which is a named Go
	// result of the same name; it does not return.
	if ident, ok := stmt.Left.(*ast.Identifier); ok && t.currentFunc != nil && ident.Value == t.currentFunc.Name.Value {
		t.write("\t%s = ", ident.Value)
		if lit, isLit := stmt.Value.(*ast.StructLiteral); isLit && t.currentFunc.ReturnType != nil {
			v, err := t.initialValue(t.currentFunc.ReturnType, lit)
			if err != nil {
				return err
			}
			t.write("%s\n", v)
			return nil
		}
		// A value of another numeric type is converted to the result's.
		target := ""
		if t.currentFunc.ReturnType != nil {
			target = iecOnly(t.mapIecTypeToGo(t.currentFunc.ReturnType))
		}
		if err := t.transpileValue(stmt.Value, target); err != nil {
			return err
		}
		t.write("\n")
		return nil
	}

	t.write("\t")

	// Check if the left-hand side is a property access, which requires a setter call.
	if memberAccess, ok := stmt.Left.(*ast.MemberAccessExpression); ok {
		if targetTypeDecl := t.resolveAssignmentTargetType(memberAccess.Struct); targetTypeDecl != nil {
			typeName := targetTypeDecl.DataType.String() // e.g., "DCMotor"
			fbDef := t.getFunctionBlockDefinitionFromTypeInfo(typeName)
			if prop := t.findPropertyOnFBChain(fbDef, memberAccess.Member.Value); prop != nil {
				// It's a property SET.
				if err := t.transpileExpression(memberAccess.Struct); err != nil {
					return err
				}
				t.write(".Set%s(", prop.Name.Value)
				if err := t.transpileExpression(stmt.Value); err != nil {
					return err
				}
				t.write(")\n")
				return nil
			}
		}
	}

	// Check if the left-hand side resolves to a subrange type.
	typeDecl := t.resolveAssignmentTargetType(stmt.Left)
	if typeDecl != nil && typeDecl.Subrange != nil {
		return t.transpileSubrangeAssignment(stmt, typeDecl)
	}

	// Check if this is a reference assignment.
	if typeDecl != nil {
		if _, ok := typeDecl.DataType.(*ast.ReferenceType); ok {
			// Manually transpile the LHS to avoid dereferencing.
			if ident, ok := stmt.Left.(*ast.Identifier); ok {
				if t.programVarName != "" && !t.globalVars[ident.Value] {
					t.write("%s.%s", t.programVarName, ident.Value)
				} else {
					t.write("%s", ident.Value)
				}
				t.write(" = &")
				if err := t.transpileExpression(stmt.Value); err != nil {
					return err
				}
				t.write("\n")
				return nil
			}
			// For now, only simple identifiers are supported as reference assignment targets.
		}
	}

	// A structure initializer takes its type from the assignment's target.
	if lit, ok := stmt.Value.(*ast.StructLiteral); ok {
		if typeDecl == nil {
			return fmt.Errorf("cannot determine the type of %s for the initializer %s", stmt.Left.String(), lit.String())
		}
		dataType := ast.Expression(typeDecl.Name)
		if _, isStruct := typeDecl.DataType.(*ast.StructDefinition); !isStruct {
			dataType = typeDecl.DataType
		}
		v, err := t.initialValue(dataType, lit)
		if err != nil {
			return err
		}
		if v == "" {
			return fmt.Errorf("%s is not a structure, so it cannot be assigned the initializer %s", stmt.Left.String(), lit.String())
		}
		if err := t.transpileExpression(stmt.Left); err != nil {
			return err
		}
		t.write(" = %s\n", v)
		return nil
	}

	// An array literal takes its type from the array it is assigned to.
	if lit, ok := stmt.Value.(*ast.ArrayLiteral); ok {
		if def, dim := t.indexedArray(stmt.Left); def != nil && dim < len(def.Ranges) {
			if dim > 0 { // A row of a multi-dimensional array.
				def = &ast.ArrayDefinition{Token: def.Token, Ranges: def.Ranges[dim:], DataType: def.DataType}
			}
			v, err := t.arrayValue(def, lit, map[ast.Node]bool{})
			if err != nil {
				return err
			}
			if err := t.transpileExpression(stmt.Left); err != nil {
				return err
			}
			t.write(" = %s\n", v)
			return nil
		}
	}

	// Standard assignment
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")
	// A value of another numeric type is converted to the target's type,
	// as IEC 61131-3 widens it implicitly.
	var err error
	if n := t.stringLength(stmt.Left); n > 0 {
		// A STRING(n) keeps the first n characters of a longer value.
		t.usesStringCut = true
		t.write("stringCut(iec.STRING(")
		err = t.transpileExpression(stmt.Value)
		t.write("), %d)", n)
	} else {
		err = t.transpileValue(stmt.Value, t.exprGoType(stmt.Left))
	}
	if err != nil {
		return err
	}
	t.write("\n")
	return nil
}

// getFunctionBlockDefinitionFromTypeInfo gets the FB definition from the typeInfo map.
func (t *Transpiler) getFunctionBlockDefinitionFromTypeInfo(typeName string) *ast.FunctionBlockDeclaration {
	if typeDef, ok := t.typeInfo[typeName]; ok {
		if fbDef, isFB := typeDef.(*ast.FunctionBlockDeclaration); isFB {
			return fbDef
		}
	}
	return nil
}

// findPropertyOnFBChain recursively searches for a property declaration starting from a given
// function block and traversing up its inheritance chain.
func (t *Transpiler) findPropertyOnFBChain(fbDef *ast.FunctionBlockDeclaration, propName string) *ast.PropertyDeclaration {
	if fbDef == nil {
		return nil
	}

	// Search for the property in the current FB's definition.
	for _, prop := range fbDef.Properties {
		if prop.Name.Value == propName {
			return prop // Found it.
		}
	}

	// If not found, recurse to the parent.
	if fbDef.Extends != nil {
		parentDef := t.getFunctionBlockDefinitionFromTypeInfo(fbDef.Extends.String())
		return t.findPropertyOnFBChain(parentDef, propName)
	}

	return nil // Reached the top of the chain without finding the property.
}

// resolveAssignmentTargetType recursively determines the `TypeDeclaration` of the target
// of an assignment expression. It handles identifiers, array elements, and struct members.
// resolveAssignmentTargetType recursively finds the TypeDeclaration for the target of an assignment.
// It can handle simple variables, array elements, and struct members.
func (t *Transpiler) resolveAssignmentTargetType(expr ast.Expression) *ast.TypeDeclaration {
	switch e := expr.(type) {
	case *ast.Identifier:
		// Base case: a simple variable. Look it up in the current scope's varInfo.
		if td := t.varInfo[e.Value]; td != nil {
			return td
		}
		// A global variable the scope does not declare.
		if decl := t.globalDecl(e.Value); decl != nil {
			return &ast.TypeDeclaration{Name: decl.Name, DataType: decl.DataType, StringLength: decl.StringLength}
		}
		return nil

	case *ast.IndexExpression:
		// It's an array element. The type of the element is the type of the array's base variable.
		// Recursively resolve the type of the array itself.
		return t.resolveAssignmentTargetType(e.Left)

	case *ast.MemberAccessExpression:
		// Any member of a structure or function block, however it is reached.
		if decl := t.memberDecl(e.Struct, e.Member.Value); decl != nil {
			return &ast.TypeDeclaration{Name: decl.Name, DataType: decl.DataType, StringLength: decl.StringLength}
		}
		// It's a struct field. We need to find the type of the struct, then the type of the field.
		structVarType := t.resolveAssignmentTargetType(e.Struct)
		if structVarType == nil {
			return nil // Can't resolve the struct's type.
		}

		// A structure variable's type declaration is the structure type itself.
		if _, isStruct := structVarType.DataType.(*ast.StructDefinition); isStruct {
			return t.findMemberType(structVarType, e.Member.Value)
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
	// A function block's variable, such as an output read as fb.Q.
	if fb, isFB := structDefNode.(*ast.FunctionBlockDeclaration); isFB {
		if decl := t.findFunctionBlockVar(fb, memberName); decl != nil {
			return &ast.TypeDeclaration{Name: decl.Name, DataType: decl.DataType}
		}
		return nil
	}
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
// The value is clamped to the subrange with Go's built-in min and max, and
// converted to the subrange type, e.g. `p.x = SMALL(min(max(v, -100), 100))`.
func (t *Transpiler) transpileSubrangeAssignment(stmt *ast.AssignmentStatement, typeDecl *ast.TypeDeclaration) error {
	// Transpile the left side of the assignment (the variable).
	if err := t.transpileExpression(stmt.Left); err != nil {
		return err
	}
	t.write(" = ")

	// Get the subrange bounds.
	subrange, ok := typeDecl.Subrange.(*ast.InfixExpression)
	if !ok || subrange.Operator != ".." {
		return fmt.Errorf("invalid subrange definition for type %s", typeDecl.Name.Value)
	}

	t.write("%s(min(max(", t.mapIecTypeToGo(typeDecl.Name))
	if err := t.transpileExpression(stmt.Value); err != nil {
		return err
	}
	t.write(", %s), %s))\n", subrange.Left.String(), subrange.Right.String())

	return nil
}

// transpileForLoopStatement transpiles an IEC 61131-3 `FOR` loop into a Go `for` loop.
// transpileForLoopStatement transpiles a FOR loop to a Go `for` loop.
func (t *Transpiler) transpileForLoopStatement(stmt *ast.ForLoopStatement) error {
	// The control variable is a declared variable, so it is assigned rather
	// than declared, and keeps its final value after the loop. The loop runs
	// up to the end value, or down to it for a negative step.
	control, err := t.expressionString(stmt.ControlVar.Left)
	if err != nil {
		return err
	}
	// The start, end and step take the control variable's type.
	ctype := t.exprGoType(stmt.ControlVar.Left)
	end, err := t.capture(func() error { return t.transpileConverted(stmt.EndValue, ctype) })
	if err != nil {
		return err
	}
	condition := fmt.Sprintf("%s <= %s", control, end)
	if stmt.StepValue != nil {
		if step, isConst := constantInteger(stmt.StepValue); isConst && step < 0 {
			condition = fmt.Sprintf("%s >= %s", control, end)
		} else if !isConst {
			step, err := t.capture(func() error { return t.transpileConverted(stmt.StepValue, ctype) })
			if err != nil {
				return err
			}
			condition = fmt.Sprintf("(%s >= 0 && %s <= %s) || (%s < 0 && %s >= %s)", step, control, end, step, control, end)
		}
	}
	t.write("\tfor %s = ", control)
	if err := t.transpileValue(stmt.ControlVar.Value, ctype); err != nil {
		return err
	}
	t.write("; %s; ", condition)
	if err := t.transpileExpression(stmt.ControlVar.Left); err != nil {
		return err
	}
	if stmt.StepValue != nil {
		t.write(" += ")
		if err := t.transpileConverted(stmt.StepValue, ctype); err != nil {
			return err
		}
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
	case *ast.BitAccessExpression:
		return t.transpileBitRead(exp)
	case *ast.Identifier:
		// NULL, the reference to nothing, unless a variable is called NULL.
		if t.isNull(exp) {
			t.write("nil")
			return nil
		}
		// A global variable of OSCAT BASIC, such as MATH.
		if g, ok := t.beebreadGlobal(exp); ok {
			t.write("%s", g)
			return nil
		}
		// If we are inside a property setter, check if the identifier is the
		// property name itself or `value`, which act as the implicit input variable.
		if t.currentSetter != nil && (exp.Value == t.currentSetter.Name.Value || strings.EqualFold(exp.Value, "value")) {
			t.write("value") // The name of the setter's parameter
			return nil       // This will call transpileNode recursively
		}
		// A parameter, result or local of the current function or method.
		if t.localVars[exp.Value] || (t.currentFunc != nil && exp.Value == t.currentFunc.Name.Value) {
			if t.inOutVars[exp.Value] {
				t.write("(*%s)", exp.Value)
			} else {
				t.write("%s", exp.Value)
			}
			return nil
		}
		// An access variable is a pointer and must be dereferenced. (A located
		// variable is a plain field, copied from and to the process image.)
		if t.accessVars[exp.Value] {
			t.write("(*%s.%s)", t.programVarName, exp.Value)
		} else if t.inOutVars[exp.Value] {
			// A VAR_IN_OUT is a pointer and must be dereferenced; in a program or
			// function block it is a field of the receiver.
			if t.programVarName != "" {
				t.write("(*%s.%s)", t.programVarName, exp.Value)
			} else {
				t.write("(*%s)", exp.Value)
			}
		} else if varDecl, ok := t.varInfo[exp.Value]; ok && t.isReferenceType(varDecl.DataType) {
			// If this is a reference type, it's a pointer. We need to dereference it
			// to get the value, UNLESS it's already being dereferenced by the '^' operator.
			if !t.isDereferencing {
				t.write("(*")
			}
			if t.programVarName != "" && !t.globalVars[exp.Value] {
				t.write("%s.%s", t.programVarName, exp.Value)
			} else {
				t.write("%s", exp.Value)
			}
			if !t.isDereferencing {
				t.write(")")
			}
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
		t.write("%s(%d)", bitStringGoType(exp.Width), exp.Value)
	case *ast.LRealLiteral:
		t.write("%s", realLiteral(float64(exp.Value)))
	case *ast.WStringLiteral:
		t.write("%q", exp.Value)
	case *ast.DateLiteral:
		return t.writeTimeDate(exp.Value, "DATE")
	case *ast.TimeOfDayLiteral:
		return t.writeTimeDate(exp.Value, "TOD")
	case *ast.DateAndTimeLiteral:
		return t.writeTimeDate(exp.Value, "DT")
	case *ast.Boolean:
		t.write("%t", exp.Value)
	case *ast.RealLiteral:
		t.write("%s", realLiteral(float64(exp.Value)))
	case *ast.StringLiteral:
		t.write("%q", exp.Value)
	case *ast.TimeLiteral:
		return t.writeTimeDate(exp.Value, "TIME")
	case *ast.InfixExpression:
		return t.transpileInfixExpression(exp)
	case *ast.PrefixExpression:
		return t.transpilePrefixExpression(exp)
	case *ast.DereferenceExpression:
		return t.transpileDereferenceExpression(exp)
	case *ast.CallExpression:
		return t.transpileCallExpression(exp)
	case *ast.MemberAccessExpression:
		return t.transpileMemberAccessExpression(exp)
	case *ast.IndexExpression:
		return t.transpileIndexExpression(exp)
	case *ast.EnumeratedValueLiteral:
		return t.transpileEnumeratedValueLiteral(exp)
	case *ast.StructLiteral:
		return t.transpileStructLiteral(exp)
	case *ast.FunctionLiteral:
		return t.transpileFunctionLiteral(exp)
	case *ast.HashLiteral:
		return t.transpileHashLiteral(exp)
	case *ast.TypedLiteral:
		// Check if it's an enum value (e.g., MyColor#RED)
		if typeDef, ok := t.typeInfo[exp.TypeName]; ok {
			if typeDecl, isTypeDecl := typeDef.(*ast.TypeDeclaration); isTypeDecl {
				if _, isEnumDef := typeDecl.DataType.(*ast.EnumDefinition); isEnumDef {
					// It's an enum type, so transpile as an enumerated value literal
					return t.transpileEnumeratedValueLiteral(&ast.EnumeratedValueLiteral{
						TypeName: &ast.Identifier{Value: exp.TypeName},
						Value:    exp.Value.(*ast.Identifier), // Assuming parseIecLiteralValue returns Identifier
					})
				}
			}
		}
		return t.transpileTypedLiteral(exp)
	case *ast.ArrayLiteral:
		return t.transpileArrayLiteral(exp)
	case *ast.ArrayRepetition:
		// This is handled by transpileArrayLiteral, but we add a case to be safe.
		return t.transpileArrayRepetition(exp)
	case *ast.MacroLiteral:
		// This case should no longer be hit, as macros are expanded before this function is called.
		return fmt.Errorf("unhandled macro literal found during code generation pass")
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
				if err := t.transpileVarDecl(member); err != nil {
					return err
				}
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
		} else if decl.DataType != nil {
			// An alias of another type, such as `MyInt : INT := 42`. Its initial
			// value is the starting value of its variables.
			t.write("// %s is an alias of %s.\n", decl.Name.Value, decl.DataType.String())
			t.write("type %s %s\n\n", decl.Name.Value, t.mapIecTypeToGo(decl.DataType))
		}
	}
	return nil
}

// mapIecTypeToGo converts an AST expression representing an IEC 61131-3 data type
// into its corresponding Go type string, typically prefixed with `iec.` for built-in types.
// mapIecTypeToGo converts an AST expression representing an IEC type
// into the corresponding Go type string from the `iec` package.
func (t *Transpiler) mapIecTypeToGo(dataType ast.Expression) string {
	var typeName string
	switch dt := dataType.(type) {
	case *ast.Identifier:
		typeName = dt.Value
	case *ast.TypeSpecifier:
		typeName = dt.Token.Literal
	case *ast.MemberAccessExpression:
		// For qualified names like MyLib.MyType, transpile to mylib.MyType.
		// The String() method on MemberAccessExpression already produces the correct dot-separated path.
		// We just need to lowercase the namespace part to follow Go conventions.
		return t.transpileQualifiedIdentifier(dt)
	case *ast.ArrayDefinition:
		// A Go array, which is assigned and passed by value as IEC 61131-3
		// arrays are. Bounds that are not constant give a slice.
		dims := ""
		if lengths, ok := arrayLengths(dt); ok {
			for _, n := range lengths {
				dims += fmt.Sprintf("[%d]", n)
			}
		} else {
			dims = strings.Repeat("[]", len(dt.Ranges))
		}
		elemType := t.mapIecTypeToGo(dt.DataType)
		return dims + elemType
	case *ast.RefToType:
		// REF_TO T is *T; a reference to an array is a slice; see references.go.
		return t.refToGoType(dt)
	case *ast.ReferenceType:
		baseType := t.mapIecTypeToGo(dt.BaseType)
		return "*" + baseType
	case *ast.StructDefinition:
		// A structure declared in place is a Go anonymous struct.
		fields := make([]string, len(dt.Members))
		for i, m := range dt.Members {
			fields[i] = m.Name.Value + " " + t.mapIecTypeToGo(m.DataType)
		}
		return "struct{ " + strings.Join(fields, "; ") + " }"
	default:
		log.Printf("Warning: Unhandled data type expression in transpiler: %s", dataType.String())
		return "any /* unhandled type */"
	}

	if typeName != "" {
		// Check if it's a user-defined type (STRUCT, ENUM, etc.) we've registered.
		if _, isUserDefined := t.typeInfo[typeName]; isUserDefined {
			// It's a type we've defined in this package, so just use its name.
			return typeName
		}
		// The same type written in another case, as IEC 61131-3 allows.
		for declared := range t.typeInfo {
			if strings.EqualFold(declared, typeName) {
				return declared
			}
		}
		// A standard function block is royaljelly's.
		if fb, ok := standardFunctionBlocks[strings.ToUpper(typeName)]; ok {
			return fb.goType
		}
		// A function block or structured type of OSCAT BASIC is beebread's.
		if b, ok := t.beebreadTypeOf(dataType); ok {
			return b.goType
		}
		// Convert to uppercase to match standard IEC types (e.g., 'int' -> 'INT').
		iecType := strings.ToUpper(typeName)
		// royaljelly names DATE_AND_TIME only by its short form.
		if iecType == "DATE_AND_TIME" {
			iecType = "DT"
		}
		// It's a standard built-in type, so prefix with the 'iec' package.
		return "iec." + iecType
	}
	log.Printf("Warning: Unhandled data type expression in transpiler: %s", dataType.String())
	return "any /* unhandled type */"
}

// transpileQualifiedIdentifier converts an IEC qualified name (MyLib.MyType)
// into a Go qualified name (mylib.MyType).
func (t *Transpiler) transpileQualifiedIdentifier(expr *ast.MemberAccessExpression) string {
	// Recursively build the path.
	var buildPath func(e ast.Expression) string
	buildPath = func(e ast.Expression) string {
		if ident, ok := e.(*ast.Identifier); ok {
			return ident.Value
		}
		if member, ok := e.(*ast.MemberAccessExpression); ok {
			return buildPath(member.Struct) + "." + member.Member.Value
		}
		return ""
	}
	return buildPath(expr)
}

// transpileInfixExpression transpiles an IEC 61131-3 infix expression (e.g., `A + B`, `X AND Y`)
// into a Go infix expression, mapping IEC operators to their Go equivalents.
func (t *Transpiler) transpileInfixExpression(exp *ast.InfixExpression) error {
	// DATE, DT and TOD values compare as points in time.
	if done, err := t.transpileTimeComparison(exp); done || err != nil {
		return err
	}
	if done, err := t.transpileTimeArithmetic(exp); done || err != nil {
		return err
	}
	t.write("(")
	// Operands of different numeric types meet at the wider type.
	common := ""
	lt, rt := t.exprGoType(exp.Left), t.exprGoType(exp.Right)
	if lt != "" && rt != "" && lt != rt {
		common = wider(lt, rt)
	}
	// An integer literal too big for the other operand's type, such as
	// 60000 with an INT, makes the operation as wide as the literal needs.
	// Bit strings keep their width.
	if !isBitStringGoType(lt) && !isBitStringGoType(rt) {
		for _, side := range [][2]string{{literalFit(exp.Right), lt}, {literalFit(exp.Left), rt}} {
			if need, known := side[0], side[1]; need != "" && known != "" && typeRank[need] > typeRank[wider(known, common)] {
				common = need
			}
		}
	}
	// A function whose result type comes only from literal arguments, such
	// as SEL(g, 0, 255), takes the other operand's type.
	operand := func(e ast.Expression, other string) error {
		if t.isLibraryCall(e) && common == "" && other != "" {
			expected := t.expectedGoType
			t.expectedGoType = other
			defer func() { t.expectedGoType = expected }()
		}
		return t.transpileConverted(e, common)
	}
	if err := operand(exp.Left, rt); err != nil {
		return err
	}

	// Map ST operators to Go operators
	isBitwise := t.isBitwiseType(exp.Left) || t.isBitwiseType(exp.Right)
	op := exp.Operator
	switch strings.ToUpper(op) {
	case "AND", "&":
		if isBitwise {
			op = "&"
		} else {
			op = "&&"
		}
	case "MOD":
		op = "%"
	case "OR":
		if isBitwise {
			op = "|"
		} else {
			op = "||"
		}
	case "XOR":
		// Go uses ^ for bitwise XOR. For booleans, != is equivalent.
		if isBitwise {
			op = "^"
		} else {
			op = "!="
		}
	case "<>":
		op = "!="
	case "=":
		op = "=="
	}

	t.write(" %s ", op)
	if err := operand(exp.Right, lt); err != nil {
		return err
	}

	t.write(")")
	return nil
}

// transpileIfStatement transpiles an IEC 61131-3 `IF...THEN...ELSIF...ELSE...END_IF` statement
// into a Go `if...else if...else` construct.
func (t *Transpiler) transpileIfStatement(stmt *ast.IfStatement) error {
	t.write("\tif ") // Add tab for the if statement itself
	if err := t.transpileExpression(stmt.Condition); err != nil {
		return err
	}

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
			if err := t.transpileExpression(elseifStmt.Condition); err != nil {
				return err
			}
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
		return t.writeTimeDate(valueStr, "TIME")
	} else if goType == "STRING" {
		// Ensure string literals are properly quoted in Go
		t.write("%q", valueStr)
	} else if goType == "BOOL" {
		switch strings.ToUpper(valueStr) {
		case "TRUE", "1":
			t.write("iec.BOOL(true)")
		case "FALSE", "0":
			t.write("iec.BOOL(false)")
		default:
			return fmt.Errorf("invalid BOOL literal: %s", lit.String())
		}
	} else {
		t.write("iec.%s(%s)", goType, goNumber(valueStr))
	}
	return nil
}

// goNumber converts an IEC 61131-3 number, which may be based (2#1010,
// 8#17, 16#FF) and contain underscores (1_000), to Go syntax.
func goNumber(value string) string {
	value = strings.ReplaceAll(value, "_", "")
	base, digits, isBased := strings.Cut(value, "#")
	if !isBased {
		return value
	}
	switch base {
	case "2":
		return "0b" + digits
	case "8":
		return "0o" + digits
	case "16":
		return "0x" + digits
	}
	return digits
}

// transpilePrefixExpression transpiles an IEC 61131-3 prefix expression (e.g., `NOT X`, `-Y`)
// into a Go prefix expression, mapping IEC operators to their Go equivalents.
func (t *Transpiler) transpilePrefixExpression(exp *ast.PrefixExpression) error {
	isBitwise := t.isBitwiseType(exp.Right)
	op := exp.Operator
	if strings.ToUpper(op) == "NOT" {
		if isBitwise {
			op = "^"
			// Bitwise NOT in Go has lower precedence than arithmetic operators,
			// so it's safer to wrap the operand.
			t.write("%s(", op)
			if err := t.transpileExpression(exp.Right); err != nil {
				return err
			}
			t.write(")")
			return nil
		} else {
			op = "!"
		}
	}
	t.write("(%s", op)
	if err := t.transpileExpression(exp.Right); err != nil {
		return err
	}
	t.write(")")
	return nil
}

// transpileStructLiteral transpiles a struct literal e.g., `(A := 1, B := TRUE)`
func (t *Transpiler) transpileStructLiteral(lit *ast.StructLiteral) error {
	// An initializer takes its type from where it is used: a variable's
	// declaration or an assignment's target, which transpile it themselves.
	return fmt.Errorf("cannot determine the type of the initializer %s here", lit.String())
}

// transpileCallExpression transpiles an IEC 61131-3 function call or function block invocation.
// It distinguishes between standard function calls and function block calls (which involve
// setting inputs, calling a `Logic` method, and handling outputs).
func (t *Transpiler) transpileCallExpression(exp *ast.CallExpression) error {
	// REF(x) and ADR(x); see references.go.
	if t.isReferenceCall(exp) {
		return t.transpileReference(exp)
	}
	// --- Check for SUPER call ---
	if memberAccess, ok := exp.Function.(*ast.MemberAccessExpression); ok {
		if deref, ok := memberAccess.Struct.(*ast.DereferenceExpression); ok {
			if _, ok := deref.Pointer.(*ast.SuperExpression); ok {
				// This is a SUPER call, e.g., SUPER^.MyMethod()
				if t.currentFuncBlock == nil || t.currentFuncBlock.Extends == nil {
					return fmt.Errorf("SUPER call used outside of a derived FUNCTION_BLOCK")
				}

				// Find which parent in the hierarchy implements the method.
				methodName := memberAccess.Member.Value
				var implementingParent *ast.FunctionBlockDeclaration
				currentParent := t.getFunctionBlockDefinitionFromTypeInfo(t.currentFuncBlock.Extends.String())

				for currentParent != nil {
					if currentParent.HasMethod(methodName) {
						implementingParent = currentParent
						break
					}
					if currentParent.Extends == nil {
						break
					}
					currentParent = t.getFunctionBlockDefinitionFromTypeInfo(currentParent.Extends.String())
				}

				if implementingParent == nil {
					return fmt.Errorf("could not find method '%s' in any parent for SUPER call", methodName)
				}

				receiverName := t.programVarName
				t.write("%s.%s.%s(", receiverName, implementingParent.Name.Value, methodName)
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
				return nil // SUPER call handled.
			}
		}
	}
	// A call to a user-defined function or method puts its arguments in
	// declaration order and fills omitted inputs with their defaults.
	if sig, ok := t.lookupCallSignature(exp.Function); ok {
		return t.transpileUserCall(exp, sig)
	}
	// TIME(), the CODESYS clock, is the time since the program started.
	if isClockCall(exp) {
		t.usesPlcTime = true
		t.write("plcTime()")
		return nil
	}
	// A call expression can be a standard function (e.g., SIN(X)) or a Function Block invocation (e.g., MyTimer(IN:=...)).
	// We'll treat calls with named arguments as potential FB calls.
	isLikelyFB := false
	if len(exp.Arguments) > 0 {
		if _, ok := exp.Arguments[0].(*ast.NamedArgument); ok {
			isLikelyFB = true
		}
	}
	// A call of a variable declared as a function block, such as `f()`, or
	// of a standard function, such as SQRT.
	if td := t.resolveAssignmentTargetType(exp.Function); td != nil {
		isLikelyFB = t.isFunctionBlockType(td.DataType)
	} else if ident, ok := exp.Function.(*ast.Identifier); ok {
		if _, isStd := royaljellyFunctions[strings.ToUpper(ident.Value)]; isStd {
			isLikelyFB = false
		} else if _, _, isOSCAT := t.beebreadFunctionOf(ident); isOSCAT {
			isLikelyFB = false
		}
	}

	if !isLikelyFB {
		// Standard function call like SIN(X)
		return t.transpileStandardFunctionCall(exp)
	}

	// --- Multi-line statement generation from a single expression ---
	// The ExpressionStatement handler adds the initial tab and final newline.
	// We generate a sequence of statements for a single FB call expression.
	hasWrittenStmt := false

	// Separate arguments into inputs and outputs first, as their order matters.
	inputArgs := []*ast.NamedArgument{}
	outputArgs := []*ast.OutputArgument{}
	for _, arg := range exp.Arguments {
		if na, ok := arg.(*ast.NamedArgument); ok {
			inputArgs = append(inputArgs, na)
		} else if oa, ok := arg.(*ast.OutputArgument); ok {
			outputArgs = append(outputArgs, oa)
		}
	}

	// --- Function Block Invocation ---
	// 1. Set the input parameters.
	for _, namedArg := range inputArgs {
		if hasWrittenStmt {
			t.write("\n\t") // Newline and tab for subsequent statements
		}
		isInOut := t.isInOutArgument(exp.Function, namedArg.Name.Value)
		if err := t.transpileExpression(exp.Function); err != nil {
			return err
		}
		if isInOut {
			t.write(".%s = &", t.memberName(exp.Function, namedArg.Name.Value))
			if err := t.transpileExpression(namedArg.Value); err != nil {
				return err
			}
		} else {
			t.write(".%s = ", t.memberName(exp.Function, namedArg.Name.Value))
			if err := t.transpileValue(namedArg.Value, t.inputGoType(exp.Function, namedArg.Name)); err != nil {
				return err
			}
		}
		hasWrittenStmt = true
	}

	// 2. Call the Logic() method.
	if hasWrittenStmt {
		t.write("\n\t")
	}
	if err := t.transpileExpression(exp.Function); err != nil {
		return err
	}

	t.write(".%s", t.functionBlockRun(exp.Function)) // Logic(now), or a standard function block's own call
	hasWrittenStmt = true

	// 3. Handle output arguments (e.g., Q => MyVar)
	for _, outArg := range outputArgs {
		t.write("\n\t")
		if err := t.transpileExpression(outArg.Target); err != nil {
			return err
		}
		t.write(" = ")
		if err := t.transpileExpression(exp.Function); err != nil {
			return err
		}
		t.write(".%s", t.memberName(exp.Function, outArg.Source.Value))
	}
	return nil
}

// isInOutArgument checks if a given argument name for a function block or function
// corresponds to a `VAR_IN_OUT` parameter, indicating it should be passed by reference.
// isInOutArgument checks the symbol table to determine if an argument for a
// given function block corresponds to a VAR_IN_OUT parameter.
func (t *Transpiler) isInOutArgument(fbExpr ast.Expression, argName string) bool {
	// The callee is a FUNCTION by name, or a function block instance whose
	// declared type names the function block.
	var inOuts []*ast.VarDeclStatement
	if ident, ok := fbExpr.(*ast.Identifier); ok {
		if fd, ok := t.lookupType(ident.Value).(*ast.FunctionDeclaration); ok {
			inOuts = fd.VarInOuts
		}
	}
	if inOuts == nil {
		if typeDecl := t.resolveAssignmentTargetType(fbExpr); typeDecl != nil && typeDecl.DataType != nil {
			if fb := t.lookupFunctionBlock(typeDecl.DataType); fb != nil {
				inOuts = fb.VarInOuts
			}
		}
	}
	return indexOfDecl(inOuts, argName) >= 0 || t.isBeebreadInOut(fbExpr, argName)
}

// transpileStandardFunctionCall transpiles a standard IEC 61131-3 function call
// (e.g., `SIN(X)`) into a direct Go function call.
func (t *Transpiler) transpileStandardFunctionCall(exp *ast.CallExpression) error {
	// If the function is a simple identifier, it's a global/standard function.
	if ident, ok := exp.Function.(*ast.Identifier); ok {
		// Macros are expanded before transpiling (see transpileNode), so a macro
		// call that is still here could not be expanded.
		if _, isMacro := t.macroDefinitions[ident.Value]; isMacro {
			return fmt.Errorf("the call to macro '%s' could not be expanded", ident.Value)
		}
	}
	// A function of OSCAT BASIC, such as DEAD_BAND.
	if name, fn, ok := t.beebreadFunctionOf(exp.Function); ok {
		return t.transpileBeebreadCall(name, fn, exp)
	}
	// A royaljelly standard function, such as SQRT or CONCAT.
	if ident, ok := exp.Function.(*ast.Identifier); ok {
		name := strings.ToUpper(ident.Value)
		if fn, isStd := royaljellyFunctions[name]; isStd {
			return t.transpileRoyaljellyCall(name, fn, exp)
		}
	}
	// Don't transpile it as an expression, which would add a receiver prefix.
	if _, ok := exp.Function.(*ast.Identifier); ok {
		t.write("%s(", exp.Function.String())
	} else {
		// If it's a more complex expression (like a member access for a method call),
		// transpile it fully to include the receiver.
		if err := t.transpileExpression(exp.Function); err != nil {
			return err
		}
		t.write("(")
	}
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
	// The parser gives us the type name and the value name.
	// The convention is to transpile `COLOR#RED` to `COLOR_RED`.
	typeName := evl.TypeName.Value
	valueName := evl.Value.Value
	t.write("%s_%s", typeName, valueName)
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

	// Arrays are Go arrays; an array literal whose target is not known here
	// has the length of its elements.
	if elemType != "" {
		t.write("[...]%s{", elemType)
	} else {
		t.write("[]%s{", elemType)
	}
	for i, el := range al.Elements {
		if i > 0 {
			t.write(", ")
		}
		// The expression transpiler will handle ArrayRepetition now.
		if err := t.transpileExpression(el); err != nil {
			return err
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
			if err := t.transpileExpression(el); err != nil {
				return err
			}
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
	// Check if this is a property GET access.
	if targetTypeDecl := t.resolveAssignmentTargetType(exp.Struct); targetTypeDecl != nil {
		// This is a simplification. A robust solution would handle nested structs.
		typeName := targetTypeDecl.DataType.String()
		fbDef := t.getFunctionBlockDefinitionFromTypeInfo(typeName)
		if prop := t.findPropertyOnFBChain(fbDef, exp.Member.Value); prop != nil {
			// It's a property GET.
			if err := t.transpileExpression(exp.Struct); err != nil {
				return err
			}
			t.write(".Get%s()", prop.Name.Value)
			return nil
		}
	}

	// Standard member access
	if err := t.transpileExpression(exp.Struct); err != nil {
		return err
	}
	t.write(".%s", t.memberName(exp.Struct, exp.Member.Value))
	return nil
}

// transpileDereferenceExpression transpiles a pointer dereference `^`.
func (t *Transpiler) transpileDereferenceExpression(exp *ast.DereferenceExpression) error {
	// Special handling for THIS^ which refers to the current FB instance.
	if _, ok := exp.Pointer.(*ast.ThisExpression); ok {
		if t.programVarName == "" {
			return fmt.Errorf("THIS used outside of a PROGRAM or FUNCTION_BLOCK context")
		}
		t.write("%s", t.programVarName)
		return nil
	}
	// A reference to an array is a slice, used as it is.
	if t.refToArray(exp.Pointer) != nil {
		return t.transpileExpression(exp.Pointer)
	}

	// Set a flag to prevent the identifier transpiler from adding another dereference.
	originalIsDereferencing := t.isDereferencing
	t.isDereferencing = true
	defer func() { t.isDereferencing = originalIsDereferencing }()

	// For other pointers, generate a standard Go dereference.
	t.write("(*")
	if err := t.transpileExpression(exp.Pointer); err != nil {
		return err
	}
	t.write(")")
	return nil
}

// transpileIndexExpression transpiles an IEC 61131-3 array index expression
// (e.g., `MyArray[Index]`) into a Go array index access.
func (t *Transpiler) transpileIndexExpression(exp *ast.IndexExpression) error {
	if err := t.transpileExpression(exp.Left); err != nil {
		return err
	}
	// Go slices start at 0, so the declared lower bound is subtracted, e.g.
	// a[i] on ARRAY[1..3] becomes a[(i - 1)].
	low := t.indexLowerBound(exp.Left)
	if index, isConst := constantInteger(exp.Index); isConst && low != 0 {
		t.write("[%d]", index-low)
		return nil
	}
	t.write("[")
	if low != 0 {
		t.write("(")
	}
	if err := t.transpileExpression(exp.Index); err != nil {
		return err
	}
	switch {
	case low > 0:
		t.write(" - %d)", low)
	case low < 0:
		t.write(" + %d)", -low)
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
	// This is a non-standard extension from Monkey. The parser creates a
	// FunctionLiteral for `fn() {}`. We transpile this to a simple Go
	// anonymous function.
	t.write("func() {}")
	return nil
	/* This implementation assumes the AST has been extended to support typed
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

	if err := t.transpileNode(fl.Body); err != nil {
		return err
	}

	t.write("}") // End of anonymous function
	return nil*/
}

// transpileHashLiteral is a placeholder for transpiling IEC 61131-3 hash literals.
// It currently generates a Go `map[string]interface{}`.
func (t *Transpiler) transpileHashLiteral(hl *ast.HashLiteral) error {
	// Transpiling hash literals (maps).
	// Since IEC 61131-3 does not have a standard map type, we generate a Go map.
	// We use map[any]any to be flexible, as beedance allows various hashable key types.
	t.write("map[any]any {\n")

	// Sort keys for deterministic output
	keys := make([]ast.Expression, 0, len(hl.Pairs))
	for k := range hl.Pairs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

	for _, key := range keys {
		t.write("\t\t")
		if err := t.transpileExpression(key); err != nil {
			return err
		}
		t.write(": ")
		if err := t.transpileExpression(hl.Pairs[key]); err != nil {
			return err
		}
		t.write(",\n")
	}
	t.write("\t")
	t.write("}")
	return nil
}
