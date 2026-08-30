/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package compiler

import (
	"beedance/ast"
	"beedance/code"
	"beedance/object"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CompiledProgram holds the separated bytecode for a program's
// initialization and cyclic execution phases.
type CompiledProgram struct {
	InitBytecode   *Bytecode
	CyclicBytecode *Bytecode
}

// Compiler holds the state of the compilation process, including the symbol table,
// constant pool, and compilation scopes for managing instructions and scopes.
type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int

	functionStack []*ast.FunctionDeclaration
	loopCtxStack  []*loopContext
}

// loopContext holds information about a loop being compiled, such as the
// positions of `EXIT` statements (`break` in Go) that need to be patched.
type loopContext struct {
	breakPositions []int
}

// New creates and initializes a new Compiler instance. It sets up the main
// compilation scope and defines the built-in functions in the global symbol table.
func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()

	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	return &Compiler{
		constants:     []object.Object{},
		symbolTable:   symbolTable,
		scopes:        []CompilationScope{mainScope},
		scopeIndex:    0,
		functionStack: []*ast.FunctionDeclaration{},
		loopCtxStack:  []*loopContext{},
	}
}

// NewWithState creates a new Compiler with a pre-existing symbol table and
// constant pool, which is useful for testing or for a REPL environment.
func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

// pushFunction adds a function declaration to the top of the function stack.
// This is used to track the context of the current function being compiled,
// which is necessary for handling recursion and return values correctly.
func (c *Compiler) pushFunction(fn *ast.FunctionDeclaration) {
	c.functionStack = append(c.functionStack, fn)
}

// popFunction removes the current function declaration from the top of the function stack.
func (c *Compiler) popFunction() {
	if len(c.functionStack) > 0 {
		c.functionStack = c.functionStack[:len(c.functionStack)-1]
	}
}

// currentFunction returns the function declaration currently being compiled,
// or nil if not inside a function.
func (c *Compiler) currentFunction() *ast.FunctionDeclaration {
	if len(c.functionStack) == 0 {
		return nil
	}
	return c.functionStack[len(c.functionStack)-1]
}

// enterLoop pushes a new loop context onto the loop context stack. This is
// called at the beginning of compiling any loop structure (FOR, WHILE, REPEAT).
func (c *Compiler) enterLoop() {
	c.loopCtxStack = append(c.loopCtxStack, &loopContext{})
}

// leaveLoop pops and returns the current loop context from the stack. This is
// called after a loop has been fully compiled, allowing the compiler to patch
// any `EXIT` statements.
func (c *Compiler) leaveLoop() *loopContext {
	if len(c.loopCtxStack) == 0 {
		return nil
	}
	last := len(c.loopCtxStack) - 1
	ctx := c.loopCtxStack[last]
	c.loopCtxStack = c.loopCtxStack[:last]
	return ctx
}

// currentLoop returns the loop context for the innermost loop currently being compiled.
func (c *Compiler) currentLoop() *loopContext {
	if len(c.loopCtxStack) == 0 {
		return nil
	}
	return c.loopCtxStack[len(c.loopCtxStack)-1]
}

// CompileProgram orchestrates the compilation of a PROGRAM POU into two distinct
// parts: an initialization section and a cyclic section. This separation mirrors
// the execution model of a PLC, where initialization runs once and the main logic
// runs repeatedly in a scan cycle. It is renamed to match the call in main.go.
func (c *Compiler) CompiledProgram(node *ast.ProgramDeclaration) (*CompiledProgram, error) {
	// --- Initialization Phase ---
	// Compile all variable declaration blocks (VAR, VAR_GLOBAL, etc.).
	// This populates the symbol table and generates bytecode to set initial values.
	varDecls := []ast.Statement{}
	for _, b := range node.VarGlobal {
		varDecls = append(varDecls, b)
	}
	for _, d := range node.Vars {
		varDecls = append(varDecls, d)
	}
	for _, b := range node.VarExternal {
		varDecls = append(varDecls, b)
	}
	for _, b := range node.VarAccess {
		varDecls = append(varDecls, b)
	}
	for _, b := range node.VarTemp {
		varDecls = append(varDecls, b)
	}

	for _, decl := range varDecls {
		if err := c.Compile(decl); err != nil {
			return nil, err
		}
	}
	initBytecode := c.Bytecode()

	// --- Cyclic Phase ---
	// Create a new, clean compiler for the cyclic part to ensure it doesn't
	// re-declare variables. It shares the same symbol table and constants.
	cyclicCompiler := NewWithState(c.symbolTable, c.constants)
	if err := cyclicCompiler.Compile(node.Body); err != nil {
		return nil, err
	}

	// After compiling the body, we ensure a value is left on the stack for the VM to report.
	if len(cyclicCompiler.currentInstructions()) == 0 {
		cyclicCompiler.emit(code.OpNull)
	} else if cyclicCompiler.lastInstructionIs(code.OpPop) {
		// If the last statement was an expression, its result was popped.
		// Remove the pop to leave the expression's value on the stack.
		cyclicCompiler.removeLastPop()
	} else {
		// If the last statement was an assignment, it doesn't leave a value on the stack.
		// We push Null so there's something to inspect.
		lastOp := cyclicCompiler.scopes[cyclicCompiler.scopeIndex].lastInstruction.Opcode
		if lastOp == code.OpSetGlobal || lastOp == code.OpSetLocal {
			cyclicCompiler.emit(code.OpNull)
		}
	}

	return &CompiledProgram{
		InitBytecode:   initBytecode,
		CyclicBytecode: cyclicCompiler.Bytecode(),
	}, nil
}

// Compile is the main entry point for the compilation process. It traverses the
// AST recursively, dispatching to specific compilation methods based on the node type.
func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	// A Program is the root of the AST, consisting of a series of statements.
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
			// Only IfStatements leave a value on the stack at the top level
			// that needs to be popped. ExpressionStatements handle their own pop.
			// Declarations do not leave values.
			if _, ok := s.(*ast.IfStatement); ok {
				c.emit(code.OpPop)
			}
		}

	// A Configuration block is compiled into a data structure representing the system setup.
	case *ast.ConfigurationDeclaration:
		return c.compileConfiguration(node)

	// VAR_CONFIG is only valid within a CONFIGURATION block and is handled there.
	case *ast.ConfigVarDeclaration:
		return fmt.Errorf("VAR_CONFIG is only valid inside a CONFIGURATION block")

	// A TYPE block contains one or more type declarations.
	case *ast.TypeBlockDeclaration:
		for _, decl := range node.Declarations {
			if err := c.Compile(decl); err != nil {
				return err
			}
		}

	// A TypeDeclaration defines a new user-defined type (STRUCT, ENUM, etc.).
	// It is compiled into an object representing the type definition and stored as a global constant.
	case *ast.TypeDeclaration:
		// A type declaration defines a "template" that gets stored in a global variable.
		// The actual instantiation happens when a VAR of this type is declared.
		symbol := c.symbolTable.Define(node.Name.Value)

		var typeDefObject object.Object

		switch dt := node.DataType.(type) {
		case *ast.StructDefinition:
			typeDefObject = &object.StructDefinition{
				Name:    node.Name,
				Members: dt.Members,
			}
		case *ast.EnumDefinition:
			typeDefObject = &object.EnumDefinition{
				Name:   node.Name,
				Values: dt.Values,
			}
		case *ast.ArrayDefinition:
			typeDefObject = &object.ArrayDefinition{
				Name:     node.Name,
				Ranges:   dt.Ranges,
				DataType: dt.DataType,
			}
		default:
			// This handles simple type aliases like `TYPE MyInt : INT; END_TYPE`.
			// For now, we don't create a special object for this, as the compiler
			// doesn't have a full type system to resolve aliases yet.
			// We'll just emit a null to have something to store.
			c.emit(code.OpNull)
		}

		if typeDefObject != nil {
			constIndex := c.addConstant(typeDefObject)
			c.emit(code.OpConstant, constIndex)
		}
		c.emit(code.OpSetGlobal, symbol.Index)

	// A FunctionBlockDeclaration is compiled into a callable closure, similar to a function,
	// representing the FB "template" or class.
	case *ast.FunctionBlockDeclaration:
		// This treats an FB declaration similarly to a function declaration.
		// It compiles the body into a callable unit and defines the FB's name globally.
		// The resulting object is a "template" or "class" that can later be instantiated.
		symbol := c.symbolTable.Define(node.Name.Value)

		c.enterScope()

		// Define all variables in the FB's scope so the body can be compiled correctly.
		for _, p := range node.VarInputs {
			c.symbolTable.Define(p.Name.Value)
		}
		for _, p := range node.VarOutputs {
			c.symbolTable.Define(p.Name.Value)
		}
		for _, p := range node.VarInOuts {
			c.symbolTable.Define(p.Name.Value)
		}

		// Compile local variable declarations to handle initial values.
		for _, decl := range node.Vars {
			if err := c.Compile(decl); err != nil {
				return err
			}
		}
		// Note: VAR_TEMP and VAR_EXTERNAL would also be handled here in a full implementation.

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		// Ensure the FB logic ends with a return, even if empty.
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.VarInputs),
		}
		fnIndex := c.addConstant(compiledFn)

		// For now, we compile it into a closure, just like a function.
		// A more advanced VM would need a dedicated FunctionBlock object.
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	// An SFCProgram is compiled into a static data structure (a hash).
	case *ast.SFCProgram:
		return c.compileSFCProgram(node)

	case *ast.ActionStatement:
		// An Action is like a parameter-less function.
		// Compile its body into a callable unit.
		symbol := c.symbolTable.Define(node.Name.Value)

		c.enterScope()

		if err := c.Compile(node.Body); err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		fn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: 0, // Actions have no parameters
		}
		fnIndex := c.addConstant(fn)

		c.emit(code.OpClosure, fnIndex, len(freeSymbols))
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	// Step and Transition statements are only valid within an SFC program body.
	case *ast.StepStatement, *ast.TransitionStatement:
		// These nodes are now handled exclusively within compileSFCProgram.
		// If they are encountered here, it's a structural error in the AST
		// or an unsupported use case.
		return fmt.Errorf("%T is only valid inside a PROGRAM with an SFC body", node)

	// A ProgramDeclaration compiles all its variable blocks and then its body.
	case *ast.ProgramDeclaration:
		// This case handles compiling a PROGRAM POU. It processes all variable
		// declarations first to populate the symbol table, then compiles the program body.
		// This is a single-pass compilation suitable for the current test setup.
		varDecls := []ast.Statement{}
		for _, b := range node.VarGlobal {
			varDecls = append(varDecls, b)
		}
		for _, d := range node.VarInputs {
			varDecls = append(varDecls, d)
		}
		for _, d := range node.VarOutputs {
			varDecls = append(varDecls, d)
		}
		for _, d := range node.VarInOuts {
			varDecls = append(varDecls, d)
		}
		for _, d := range node.Vars {
			varDecls = append(varDecls, d)
		}
		for _, b := range node.VarExternal {
			varDecls = append(varDecls, b)
		}
		for _, b := range node.VarAccess {
			varDecls = append(varDecls, b)
		}
		for _, b := range node.VarTemp {
			varDecls = append(varDecls, b)
		}

		for _, decl := range varDecls {
			if err := c.Compile(decl); err != nil {
				return err
			}
		}

		return c.Compile(node.Body)

	// A FunctionDeclaration is compiled into a closure object. This process involves
	// setting up a new scope, defining parameters, handling the implicit return
	// variable, compiling the body, and then packaging it all into a closure.
	case *ast.FunctionDeclaration:
		// This is a statement that defines a function in the current scope.
		// First, define the function name in the current scope so it can be captured in a closure.
		symbol := c.symbolTable.Define(node.Name.Value)

		// Then, compile the function body itself.
		c.enterScope()
		c.pushFunction(node)

		c.symbolTable.DefineFunctionName(node.Name.Value) // For recursion

		// Define input parameters first, as they are the first locals in the stack frame.
		for _, p := range node.VarInputs {
			c.symbolTable.DefineVarInput(p.Name.Value)
		}

		// Define the function name as a local variable to hold the return value.
		returnSymbol := c.symbolTable.Define(node.Name.Value)

		// Initialize the return variable to Null. This ensures that if no explicit
		// return value is assigned, the function implicitly returns Null.
		c.emit(code.OpNull)
		c.emit(code.OpSetLocal, returnSymbol.Index)

		for _, p := range node.VarOutputs {
			c.symbolTable.Define(p.Name.Value)
		}

		// Compile local variable declarations (VAR ... END_VAR) to define them
		// in the function's scope.
		for _, decl := range node.Vars {
			if err := c.Compile(decl); err != nil {
				return err
			}
		}
		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		// After the body, load the return value from its variable and return it.
		// This handles the `FunctionName := ...` return style of IEC 61131-3.
		returnSymbol, ok := c.symbolTable.Resolve(node.Name.Value)
		if !ok {
			// This should not happen if we defined it above.
			return fmt.Errorf("internal compiler error: could not resolve function return variable %s", node.Name.Value)
		}
		c.loadSymbol(returnSymbol)
		c.emit(code.OpReturnValue)

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		c.popFunction()

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.VarInputs),
		}
		fnIndex := c.addConstant(compiledFn)

		// Create the closure and assign it to the variable in the outer scope.
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	// An ExpressionStatement's result is unused, so it's popped from the stack.
	// An ExpressionStatement's result is unused, so it's popped from the stack.
	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		// An expression statement's value is not used at the top level,
		// so we pop its result off the stack to keep the stack clean.
		// This is the single source of truth for this behavior.
		c.emit(code.OpPop)

	case *ast.AssignmentStatement:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		symbol, ok := c.symbolTable.Resolve(node.Left.(*ast.Identifier).Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Left.(*ast.Identifier).Value)
		}
		err = c.setSymbol(symbol)
		if err != nil {
			return err
		}

	// A VarBlockDeclaration simply triggers compilation of each declaration within it.
	case *ast.VarBlockDeclaration:
		for _, decl := range node.Declarations {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	// GlobalVarDeclaration compiles each global variable.
	case *ast.GlobalVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	// ExternalVarDeclaration compiles each external variable.
	case *ast.ExternalVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	// AccessVarDeclaration defines an external access path, storing the path
	// string as a constant and creating an EXTERNAL symbol.
	case *ast.AccessVarDeclaration:
		// This block handles VAR_ACCESS declarations.
		// We assume the parser has been updated to place the full access path
		// (e.g., "MyProgram.MyVariable") into an `AccessPath` field on the VarDeclStatement.
		for _, decl := range node.Vars {
			if decl.AccessPath == nil {
				return fmt.Errorf("internal compiler error: VAR_ACCESS for '%s' is missing an access path", decl.Name.Value)
			}
			pathIndex := c.addConstant(&object.String{Value: decl.AccessPath.String()})
			c.symbolTable.DefineExternal(decl.Name.Value, pathIndex)
		}

	// TempVarDeclaration compiles each temporary variable.
	case *ast.TempVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	// A VarDeclStatement defines a symbol and compiles its initial value (or null).
	// It then emits an instruction to store that value in the correct scope.
	case *ast.VarDeclStatement:
		symbol := c.symbolTable.Define(node.Name.Value)

		if node.Value != nil {
			err := c.Compile(node.Value)
			if err != nil {
				return err
			}
		} else {
			// If no initial value is provided, push null onto the stack.
			c.emit(code.OpNull)
		}

		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	// An InfixExpression compiles the left and right sides, then emits the operator instruction.
	case *ast.InfixExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "**":
			c.emit(code.OpExponent)
		case "MOD":
			c.emit(code.OpMod)
		case ">":
			c.emit(code.OpGreaterThan)
		case "<":
			c.emit(code.OpLessThan)
		case ">=":
			c.emit(code.OpGreaterThanOrEqual)
		case "<=":
			c.emit(code.OpLessThanOrEqual)
		case "==":
			c.emit(code.OpEqual)
		case "=":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		case "<>":
			c.emit(code.OpNotEqual)
		case "AND":
			c.emit(code.OpAnd)
		case "OR":
			c.emit(code.OpOr)
		case "XOR":
			c.emit(code.OpXor)
		case "NAND":
			c.emit(code.OpNand)
		case "NOR":
			c.emit(code.OpNor)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	// An IntegerLiteral is added to the constant pool and an OpConstant instruction is emitted.
	case *ast.IntegerLiteral:
		lint := &object.LInt{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(lint))

	// A RealLiteral is added to the constant pool and an OpConstant instruction is emitted.
	case *ast.RealLiteral:
		lreal := &object.LReal{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(lreal))

	// A Boolean literal emits either OpTrue or OpFalse directly.
	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	// A PrefixExpression compiles its right side, then emits the prefix operator instruction.
	case *ast.PrefixExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "NOT":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	// An IfStatement compiles the condition, then uses jump instructions to control
	// execution flow between the consequence and alternative blocks.
	case *ast.IfStatement:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		// Emit an `OpJumpNotTruthy` with a bogus value
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			// The consequence of an IF is an expression. We want its value to
			// be left on the stack, so we remove the final OpPop that an
			// ExpressionStatement would normally have.
			c.removeLastPop()
		}

		// Emit an `OpJump` with a bogus value
		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		if node.Alternative == nil {
			c.emit(code.OpNull)
		} else {
			err := c.Compile(node.Alternative)
			if err != nil {
				return err
			}

			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterAlternativePos)

	// A MemberAccessExpression compiles the base struct/FB and then treats the member
	// name as a string key for an OpIndex operation.
	case *ast.MemberAccessExpression:
		// Compile the struct/FB instance on the left
		if err := c.Compile(node.Struct); err != nil {
			return err
		}
		// Compile the member name as a string constant for indexing.
		// The VM's OpIndex will need to handle member access on structs.
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: node.Member.Value}))
		c.emit(code.OpIndex)

	// An UnsignedIntegerLiteral is added to the constant pool.
	case *ast.UnsignedIntegerLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.ULInt{Value: node.Value}))

	// An LRealLiteral is added to the constant pool.
	case *ast.LRealLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.LReal{Value: node.Value}))

	// A WStringLiteral is added to the constant pool.
	case *ast.WStringLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.WString{Value: node.Value}))

	// A BitStringLiteral is added to the constant pool.
	case *ast.BitStringLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.BitString{Value: node.Value, Width: node.Width}))

	// An EnumeratedValueLiteral is added to the constant pool.
	case *ast.EnumeratedValueLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.EnumeratedValue{
			TypeName: node.TypeName.Value,
			Value:    node.Value.Value,
		}))

	// A TypedLiteral is parsed and converted into the appropriate object.Object,
	// which is then added to the constant pool.
	case *ast.TypedLiteral:
		return c.compileTypedLiteral(node)

	// A ForLoopStatement is compiled into a sequence of initialization, condition
	// check, body, increment, and jump instructions to create the loop structure.
	// It uses its own scope for the loop control variable.
	case *ast.ForLoopStatement:
		c.enterScope() // Scope for the control variable
		c.enterLoop()

		// 1. Initialization
		controlVarName := node.ControlVar.Left.(*ast.Identifier).Value
		if err := c.Compile(node.ControlVar.Value); err != nil {
			return err
		}
		symbol := c.symbolTable.Define(controlVarName)
		c.emit(code.OpSetLocal, symbol.Index)

		loopStartPos := len(c.currentInstructions())

		// 2. Condition check (control_var <= end_value)
		c.loadSymbol(symbol)
		if err := c.Compile(node.EndValue); err != nil {
			return err
		}
		// Assuming positive step for now. A full implementation would check the step value.
		c.emit(code.OpLessThanOrEqual)
		jumpToEndPos := c.emit(code.OpJumpNotTruthy, 9999)

		// 3. Body
		if err := c.Compile(node.Body); err != nil {
			return err
		}

		// 4. Increment
		c.loadSymbol(symbol)
		if node.StepValue != nil {
			if err := c.Compile(node.StepValue); err != nil {
				return err
			}
		} else {
			c.emit(code.OpConstant, c.addConstant(&object.LInt{Value: 1}))
		}
		c.emit(code.OpAdd)
		if err := c.setSymbol(symbol); err != nil {
			return err
		}

		// 5. Jump back to start
		c.emit(code.OpJump, loopStartPos)

		// 6. After loop
		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpToEndPos, afterLoopPos)

		loop := c.leaveLoop()
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, afterLoopPos)
		}

		c.leaveScope() // Pop control variable scope

	// A WhileStatement is compiled into a condition check, a jump to the end if
	// the condition is false, the loop body, and a jump back to the start.
	case *ast.WhileStatement:
		c.enterLoop()
		loopStartPos := len(c.currentInstructions())

		if err := c.Compile(node.Condition); err != nil {
			return err
		}
		jumpToEndPos := c.emit(code.OpJumpNotTruthy, 9999)

		if err := c.Compile(node.Body); err != nil {
			return err
		}

		c.emit(code.OpJump, loopStartPos)

		afterLoopPos := len(c.currentInstructions())
		c.changeOperand(jumpToEndPos, afterLoopPos)

		loop := c.leaveLoop()
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, afterLoopPos)
		}

	// A RepeatStatement is compiled into the loop body followed by a condition
	// check. If the condition is false, it jumps back to the start of the body.
	// This creates a do-while style loop.
	case *ast.RepeatStatement:
		c.enterLoop()
		loopStartPos := len(c.currentInstructions())

		if err := c.Compile(node.Body); err != nil {
			return err
		}

		if err := c.Compile(node.Condition); err != nil {
			return err
		}
		c.emit(code.OpJumpNotTruthy, loopStartPos)

		afterLoopPos := len(c.currentInstructions())
		loop := c.leaveLoop()
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, afterLoopPos)
		}

	// A CaseStatement compiles the selector expression, then for each branch, it
	// compares the selector to the case value and jumps to the branch's body if
	// they are equal. It also handles the optional ELSE block.
	case *ast.CaseStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}

		exitJumps := []int{}
		nextCaseJumpPos := -1

		for _, branch := range node.Cases {
			if nextCaseJumpPos != -1 {
				c.changeOperand(nextCaseJumpPos, len(c.currentInstructions()))
			}
			// Simplified: only handles one value per case branch for now.
			c.emit(code.OpDup)
			if err := c.Compile(branch.Values[0]); err != nil {
				return err
			}
			c.emit(code.OpEqual)
			nextCaseJumpPos = c.emit(code.OpJumpNotTruthy, 9999)

			c.emit(code.OpPop) // Pop selector
			if err := c.Compile(branch.Consequence); err != nil {
				return err
			}
			exitJumps = append(exitJumps, c.emit(code.OpJump, 9999))
		}

		if nextCaseJumpPos != -1 {
			c.changeOperand(nextCaseJumpPos, len(c.currentInstructions()))
		}
		c.emit(code.OpPop) // Pop selector if no cases matched

		if node.Alternative != nil {
			if err := c.Compile(node.Alternative); err != nil {
				return err
			}
		}

		afterCasePos := len(c.currentInstructions())
		for _, pos := range exitJumps {
			c.changeOperand(pos, afterCasePos)
		}

	// An ExitStatement finds the current loop context and emits a jump to after the loop.
	case *ast.ExitStatement:
		loop := c.currentLoop()
		if loop == nil {
			return fmt.Errorf("EXIT statement not within a loop")
		}
		pos := c.emit(code.OpJump, 9999)
		loop.breakPositions = append(loop.breakPositions, pos)

	// A BlockStatement compiles each of its inner statements.
	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	// An Identifier resolves the symbol and emits an instruction to load it.
	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.loadSymbol(symbol)

	// A StringLiteral is added to the constant pool.
	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	// An ArrayLiteral compiles all its elements and then emits an OpArray instruction.
	case *ast.ArrayLiteral:
		numElements := 0
		for _, el := range node.Elements {
			// Handle ArrayRepetition within an ArrayLiteral
			if rep, ok := el.(*ast.ArrayRepetition); ok {
				count, err := c.compileArrayRepetition(rep)
				if err != nil {
					return err
				}
				numElements += count
			} else {
				err := c.Compile(el)
				if err != nil {
					return err
				}
				numElements++
			}
		}
		c.emit(code.OpArray, numElements)

	// A HashLiteral compiles its key-value pairs and emits an OpHash instruction.
	case *ast.HashLiteral:
		keys := []ast.Expression{}
		for k := range node.Pairs {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			err := c.Compile(k)
			if err != nil {
				return err
			}
			err = c.Compile(node.Pairs[k])
			if err != nil {
				return err
			}
		}

		c.emit(code.OpHash, len(node.Pairs)*2)

	// An IndexExpression compiles the left side and the index, then emits OpIndex.
	case *ast.IndexExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	// A FunctionLiteral is compiled into a closure, similar to a named function.
	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			if len(c.currentInstructions()) == 0 || !c.lastInstructionIs(code.OpReturn) {
				c.emit(code.OpReturn)
			}
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}

		fnIndex := c.addConstant(compiledFn)
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))

	// A ReturnStatement compiles the return value and emits OpReturnValue.
	case *ast.ReturnStatement:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	// A CallExpression compiles the function/callable and all arguments, then
	// emits an OpCall instruction.
	case *ast.CallExpression:
		// Check for recursive call
		isRecursive := false
		if ident, ok := node.Function.(*ast.Identifier); ok {
			if currentFn := c.currentFunction(); currentFn != nil && ident.Value == currentFn.Name.Value {
				isRecursive = true
			}
		}

		if isRecursive {
			c.emit(code.OpCurrentClosure)
		} else {
			if err := c.Compile(node.Function); err != nil {
				return err
			}
		}

		for _, a := range node.Arguments {
			err := c.Compile(a)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))

	}

	return nil
}

// Bytecode returns the final compiled instructions and constant pool.
func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

// addConstant adds an object to the compiler's constant pool and returns its index.
func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// emit creates a bytecode instruction and adds it to the current scope's
// instruction list, returning the position of the new instruction.
func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

// addInstruction appends a slice of bytes (a complete instruction) to the
// current scope's instructions.
func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	return posNewInstruction
}

// setLastInstruction updates the record of the last and previous instructions emitted.
func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

// lastInstructionIs checks if the last emitted instruction has the given opcode.
func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

// removeLastPop removes the most recently emitted OpPop instruction. This is
// useful for expression statements where the value needs to be kept on the stack.
func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	new := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = new
	c.scopes[c.scopeIndex].lastInstruction = previous
}

// replaceInstruction overwrites the instruction at a given position with a new one.
func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

// changeOperand modifies the operand of an existing instruction. This is used
// for patching jump addresses after the target position is known.
func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

// currentInstructions returns the instruction slice for the current compilation scope.
func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

// enterScope creates a new, nested compilation scope and symbol table, pushing
// it onto the scope stack.
func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

// leaveScope pops the current compilation scope off the stack, returning its
// compiled instructions, and restores the outer symbol table.
func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions
}

// replaceLastPopWithReturn replaces the last emitted OpPop instruction with an
// OpReturnValue, which is a common pattern for function bodies.
func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

// loadSymbol emits the correct `Get` instruction based on the symbol's scope.
func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	case ExternalScope:
		c.emit(code.OpGetExternal, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

// setSymbol emits the correct `Set` instruction based on the symbol's scope.
func (c *Compiler) setSymbol(s Symbol) error {
	if s.IsReadOnly {
		return fmt.Errorf("cannot assign to read-only variable '%s'", s.Name)
	}
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpSetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpSetLocal, s.Index)
	case FreeScope:
		c.emit(code.OpSetFree, s.Index)
	case ExternalScope:
		c.emit(code.OpSetExternal, s.Index)
	default:
		return fmt.Errorf("cannot set symbol with scope %s", s.Scope)
	}
	return nil
}

// compileSFCProgram compiles a Sequential Function Chart into a static data
// structure (a hash object) that the VM can interpret.
func (c *Compiler) compileSFCProgram(node *ast.SFCProgram) error {
	// This function compiles an SFC graph into a static data structure (a hash)
	// that the VM can interpret to execute the SFC logic.

	// 1. Pre-compile all ACTION and TRANSITION bodies into closures.
	// This ensures they are added to the constant pool first, matching the test's expectation.
	actionClosures := make(map[string]int)
	for _, stmt := range node.Elements {
		if action, ok := stmt.(*ast.ActionStatement); ok {
			c.enterScope()
			c.Compile(action.Body)
			if c.lastInstructionIs(code.OpPop) {
				c.replaceLastPopWithReturn()
			}
			c.emit(code.OpReturn)
			instructions := c.leaveScope()
			fnIndex := c.addConstant(&object.CompiledFunction{Instructions: instructions})
			actionClosures[action.Name.Value] = fnIndex
		}
	}

	transitionClosures := make(map[*ast.TransitionStatement]int)
	for _, stmt := range node.Elements {
		if trans, ok := stmt.(*ast.TransitionStatement); ok {
			c.enterScope()
			if err := c.Compile(trans.Condition); err != nil {
				return err
			}
			c.emit(code.OpReturnValue)
			instructions := c.leaveScope()
			fnIndex := c.addConstant(&object.CompiledFunction{Instructions: instructions})
			transitionClosures[trans] = fnIndex
		}
	}

	// Helper to reuse string constants for step names, etc.
	stringConstants := make(map[string]int)
	addStringConst := func(s string) int {
		if idx, ok := stringConstants[s]; ok {
			return idx
		}
		idx := c.addConstant(&object.String{Value: s})
		stringConstants[s] = idx
		return idx
	}

	// 2. Generate bytecode to build the final SFC definition hash.

	// Key: "initial_step"
	var initialStepName string
	for _, stmt := range node.Elements {
		if step, ok := stmt.(*ast.StepStatement); ok && step.IsInitial {
			initialStepName = step.Name.Value
			break
		}
	}
	c.emit(code.OpConstant, addStringConst("initial_step"))
	c.emit(code.OpConstant, addStringConst(initialStepName))

	// Key: "actions"
	c.emit(code.OpConstant, addStringConst("actions"))
	for name, fnIndex := range actionClosures {
		c.emit(code.OpConstant, addStringConst(name))
		c.emit(code.OpClosure, fnIndex, 0)
	}
	c.emit(code.OpHash, len(actionClosures)*2)

	// Key: "transitions"
	c.emit(code.OpConstant, addStringConst("transitions"))
	numTransitions := 0
	for _, stmt := range node.Elements {
		if trans, ok := stmt.(*ast.TransitionStatement); ok {
			numTransitions++
			fnIndex := transitionClosures[trans]

			c.emit(code.OpConstant, addStringConst("condition"))
			c.emit(code.OpClosure, fnIndex, 0)
			c.emit(code.OpConstant, addStringConst("from"))
			for _, from := range trans.From {
				c.emit(code.OpConstant, addStringConst(from.Value))
			}
			c.emit(code.OpArray, len(trans.From))
			c.emit(code.OpConstant, addStringConst("to"))
			for _, to := range trans.To {
				c.emit(code.OpConstant, addStringConst(to.Value))
			}
			c.emit(code.OpArray, len(trans.To))
			c.emit(code.OpHash, 6)
		}
	}
	c.emit(code.OpArray, numTransitions)

	// Key: "steps"
	c.emit(code.OpConstant, addStringConst("steps"))
	numSteps := 0
	for _, stmt := range node.Elements {
		if step, ok := stmt.(*ast.StepStatement); ok {
			numSteps++
			c.emit(code.OpConstant, addStringConst(step.Name.Value))
			c.emit(code.OpConstant, addStringConst("actions"))
			for _, actionAssoc := range step.Actions {
				c.emit(code.OpConstant, addStringConst("name"))
				c.emit(code.OpConstant, addStringConst(actionAssoc.ActionName.Value))

				qualifier := "N"
				if actionAssoc.Qualifier != nil {
					qualifier = actionAssoc.Qualifier.Value
				}
				c.emit(code.OpConstant, addStringConst("qualifier"))
				c.emit(code.OpConstant, addStringConst(qualifier))
				c.emit(code.OpHash, 4)
			}
			c.emit(code.OpArray, len(step.Actions))
			c.emit(code.OpHash, 2)
		}
	}
	c.emit(code.OpHash, numSteps*2)

	// Finally, build the main SFC hash object from the 4 key-value pairs now on the stack.
	c.emit(code.OpHash, 8) // 4 key-value pairs: initial_step, actions, transitions, steps

	c.emit(code.OpPop)

	return nil
}

// compileConfiguration compiles a CONFIGURATION block into a nested hash object
// representing the entire system configuration.
func (c *Compiler) compileConfiguration(config *ast.ConfigurationDeclaration) error {
	// Define a global symbol for the configuration itself.
	symbol := c.symbolTable.Define(config.Name.Value)

	// Compile any global vars defined directly in the configuration.
	for _, gv := range config.GlobalVars {
		if err := c.Compile(gv); err != nil {
			return err
		}
	}

	// Compile each resource, leaving a resource hash object on the stack.
	for _, res := range config.Resources {
		// Find all VAR_CONFIG blocks that are relevant to this resource.
		// This is a simplification; a real implementation might need to map
		// program instances to resources more explicitly if names are not unique.
		// For now, we pass all configs down.
		// A better approach would be to pre-process configs into a map.
		// For this implementation, we will pass all varConfigs to the resource compiler.
		if err := c.compileResource(res, config.VarConfigs); err != nil {
			return err
		}
	}
	// Create an array of resource hashes.
	c.emit(code.OpArray, len(config.Resources))

	// Build the final configuration hash object.
	// Key: "name"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "name"}))
	// Value: config.Name.Value
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: config.Name.Value}))

	// Key: "resources"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "resources"}))
	// Value: The array of resources is already on the stack.

	c.emit(code.OpHash, 2*2) // 2 key-value pairs

	// Store the final configuration hash in its global variable.
	c.emit(code.OpSetGlobal, symbol.Index)

	return nil
}

// compileResource compiles a RESOURCE block into a hash object containing its
// tasks and program instances.
func (c *Compiler) compileResource(res *ast.ResourceDeclaration, varConfigs []*ast.ConfigVarDeclaration) error {
	// Compile tasks, leaving task hashes on the stack.
	for _, task := range res.Tasks {
		if err := c.compileTask(task); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(res.Tasks)) // Array of task hashes

	// Compile program configurations, leaving program hashes on the stack.
	for _, prog := range res.Programs {
		var matchingConfig *ast.ConfigVarDeclaration
		for _, vc := range varConfigs {
			if vc.ProgramInstanceName.Value == prog.InstanceName.Value {
				matchingConfig = vc
				break
			}
		}
		if err := c.compileProgramConfig(prog, matchingConfig); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(res.Programs)) // Array of program hashes

	// Build the resource hash object.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "name"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: res.Name.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "type"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: res.ResourceType.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "programs"})) // Key for programs array
	// The programs array is on top of the stack, tasks array is below it. Swap them.
	c.emit(code.OpSwap)

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "tasks"})) // Key for tasks array
	// The tasks array is now on top.

	c.emit(code.OpHash, 4*2) // 4 key-value pairs
	return nil
}

// compileTask compiles a TASK declaration into a hash object.
func (c *Compiler) compileTask(task *ast.TaskDeclaration) error { // cspell:disable-line
	// Build the task hash object.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "name"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: task.Name.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "interval"}))
	if err := c.Compile(task.Interval); err != nil {
		return err
	}

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "priority"}))
	if err := c.Compile(task.Priority); err != nil {
		return err
	}

	c.emit(code.OpHash, 3*2) // 3 key-value pairs
	return nil
}

// compileProgramConfig compiles a PROGRAM configuration instance into a hash
// object, including its parameters from any associated VAR_CONFIG block.
func (c *Compiler) compileProgramConfig(prog *ast.ProgramConfiguration, varConfig *ast.ConfigVarDeclaration) error {
	// Build the program configuration hash object.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "instance"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: prog.InstanceName.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "task"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: prog.TaskName.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "type"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: prog.TypeName.Value}))

	// Add the parameters from VAR_CONFIG as a nested hash.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "params"}))
	if varConfig != nil {
		for _, decl := range varConfig.Declarations {
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: decl.Name.Value}))
			if err := c.Compile(decl.Value); err != nil {
				return err
			}
		}
		c.emit(code.OpHash, len(varConfig.Declarations)*2)
	} else {
		c.emit(code.OpHash, 0) // Empty hash if no VAR_CONFIG
	}

	c.emit(code.OpHash, 4*2) // 4 key-value pairs
	return nil
}

// compileTypedLiteral compiles a typed literal (e.g., `INT#10`, `T#5s`) by
// parsing its value and creating the corresponding object.Object.
func (c *Compiler) compileTypedLiteral(node *ast.TypedLiteral) error {
	typeName := strings.ToUpper(node.TypeName)
	valueStr := node.Value.String() // This is an ast.Identifier with the value part

	var obj object.Object
	var err error

	switch typeName {
	case "TIME", "T":
		// IEC duration can have underscores, Go's time.ParseDuration does not support them.
		durationStr := strings.ReplaceAll(valueStr, "_", "")
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid TIME literal '%s': %w", valueStr, err)
		}
		obj = &object.Time{Value: d}
	case "DATE", "D":
		t, err := time.Parse("2006-01-02", valueStr)
		if err != nil {
			return fmt.Errorf("invalid DATE literal '%s': %w", valueStr, err)
		}
		obj = &object.Date{Value: t}
	case "TIME_OF_DAY", "TOD":
		t, err := time.Parse("15:04:05.999", valueStr)
		if err != nil {
			// try without milliseconds
			t, err = time.Parse("15:04:05", valueStr)
			if err != nil {
				return fmt.Errorf("invalid TIME_OF_DAY literal '%s': %w", valueStr, err)
			}
		}
		obj = &object.TimeOfDay{Value: t}
	case "DATE_AND_TIME", "DT":
		t, err := time.Parse("2006-01-02-15:04:05.999", valueStr)
		if err != nil {
			// try without milliseconds
			t, err = time.Parse("2006-01-02-15:04:05", valueStr)
			if err != nil {
				return fmt.Errorf("invalid DATE_AND_TIME literal '%s': %w", valueStr, err)
			}
		}
		obj = &object.DateAndTime{Value: t}

	// Integer types
	case "SINT", "INT", "DINT", "LINT":
		val, err := c.parseBasedInteger(valueStr)
		if err != nil {
			return err
		}
		// The VM uses LINT for all integer operations for simplicity.
		obj = &object.LInt{Value: val}
	case "USINT", "UINT", "UDINT", "ULINT":
		val, err := c.parseBasedUnsignedInteger(valueStr)
		if err != nil {
			return err
		}
		obj = &object.ULInt{Value: val}

	// Real types
	case "REAL", "LREAL":
		val, err := strconv.ParseFloat(strings.ReplaceAll(valueStr, "_", ""), 64)
		if err != nil {
			return fmt.Errorf("invalid REAL/LREAL literal '%s': %w", valueStr, err)
		}
		obj = &object.LReal{Value: val}

	// Bit-string types
	case "BYTE", "WORD", "DWORD", "LWORD":
		var width int
		switch typeName {
		case "BYTE":
			width = 8
		case "WORD":
			width = 16
		case "DWORD":
			width = 32
		case "LWORD":
			width = 64
		}
		val, err := c.parseBasedUnsignedInteger(valueStr)
		if err != nil {
			return err
		}
		obj = &object.BitString{Value: val, Width: width}

	default:
		// Fallback for enum types like `COLOR#RED` or other user-defined types.
		obj = &object.EnumeratedValue{TypeName: node.TypeName, Value: valueStr}
	}

	if obj != nil {
		c.emit(code.OpConstant, c.addConstant(obj))
	} else {
		err = fmt.Errorf("unhandled typed literal: %s", node.String())
	}
	return err
}

// parseBasedInteger is a helper function to parse an integer string that may have a base prefix (e.g., "16#FF").
func (c *Compiler) parseBasedInteger(literal string) (int64, error) {
	literal = strings.ReplaceAll(literal, "_", "")
	base := 10
	valueStr := literal

	if strings.Contains(literal, "#") {
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err != nil || (parsedBase != 2 && parsedBase != 8 && parsedBase != 10 && parsedBase != 16) {
				return 0, fmt.Errorf("invalid base in literal: %s", literal)
			}
			base = parsedBase
			valueStr = parts[1]
		}
	}

	return strconv.ParseInt(valueStr, base, 64)
}

// parseBasedUnsignedInteger is a helper function to parse an unsigned integer string that may have a base prefix.
func (c *Compiler) parseBasedUnsignedInteger(literal string) (uint64, error) {
	literal = strings.ReplaceAll(literal, "_", "")
	base := 10
	valueStr := literal

	if strings.Contains(literal, "#") {
		parts := strings.SplitN(literal, "#", 2)
		if len(parts) == 2 {
			parsedBase, err := strconv.Atoi(parts[0])
			if err != nil || (parsedBase != 2 && parsedBase != 8 && parsedBase != 10 && parsedBase != 16) {
				return 0, fmt.Errorf("invalid base in literal: %s", literal)
			}
			base = parsedBase
			valueStr = parts[1]
		}
	}

	return strconv.ParseUint(valueStr, base, 64)
}

// compileArrayRepetition handles the `N(v1, v2, ...)` syntax within an array
// literal, emitting the repeated element values onto the stack.
func (c *Compiler) compileArrayRepetition(ar *ast.ArrayRepetition) (int, error) {
	factor, ok := ar.Factor.(*ast.IntegerLiteral)
	if !ok {
		// For now, we only support constant integer literals as repetition factors.
		// A more advanced implementation could try to evaluate a constant expression.
		return 0, fmt.Errorf("array repetition factor must be a constant integer literal")
	}

	count := int(factor.Value)
	numElementsPerRep := len(ar.Elements)
	totalElements := count * numElementsPerRep

	for i := 0; i < count; i++ {
		for _, el := range ar.Elements {
			if err := c.Compile(el); err != nil {
				return 0, err
			}
		}
	}

	return totalElements, nil
}

// Bytecode holds the compiled instructions and the constant pool for a program or function.
type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

// EmittedInstruction represents an instruction that has been emitted by the
// compiler, tracking its opcode and position in the bytecode stream.
type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

// CompilationScope represents a single level of scope during compilation,
// holding the instructions and instruction history for that scope.
type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}
