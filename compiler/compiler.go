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
	"bytes"
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
// outputTemp names the hidden variable that holds an output argument on its way
// to a target other than a variable, such as `o => a[1]`. IEC 61131-3
// identifiers cannot contain two underscores in a row, so it never clashes.
const outputTemp = "__output_value"

type Compiler struct {
	// stampingArray is set while an assignment to an array with declared
	// lower bounds is compiled, before the bounds are set on the result.
	stampingArray bool
	constants     []object.Object

	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int

	functionStack   []*ast.FunctionDeclaration
	functionResults []functionResult // Parallel to functionStack.
	loopCtxStack    []*loopContext
	currentFB       *ast.FunctionBlockDeclaration
	typeInfo        map[string]ast.Node
	pouNamespaces   map[string]*ast.NamespaceDeclaration // Maps POU name to its namespace
	currentNS       *ast.NamespaceDeclaration            // The namespace currently being compiled
	rootProgram     *ast.Program                         // Reference to the root program node
	// predefined holds the symbols predefineGlobals made, by declaration.
	predefined map[ast.Node]Symbol
}

// NewCompilerWithBuiltins creates a new compiler with a specific set of built-in functions.
// This is useful for testing to avoid dependency on global state.
func NewCompilerWithBuiltins(builtins []object.BuiltinEntry) *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
		varDecls:            make(map[string]*ast.VarDeclStatement),
	}

	symbolTable := NewSymbolTable()
	for _, v := range builtins {
		symbolTable.DefineBuiltin(v.Index, v.Name) // Use the explicit Index from BuiltinEntry
	}

	return &Compiler{
		constants:     []object.Object{},
		symbolTable:   symbolTable,
		scopes:        []CompilationScope{mainScope},
		scopeIndex:    0,
		functionStack: []*ast.FunctionDeclaration{},
		loopCtxStack:  []*loopContext{},
		currentFB:     nil,
		typeInfo:      make(map[string]ast.Node),
		pouNamespaces: make(map[string]*ast.NamespaceDeclaration),
		rootProgram:   nil,
	}
}

// New creates and initializes a new Compiler instance. It sets up the main
// compilation scope and defines the built-in functions in the global symbol table.
func New() *Compiler {
	// The object.Builtins slice is now pre-indexed by FinalizeBuiltins,
	// so we can use it directly without sorting to create the compiler.
	return NewCompilerWithBuiltins(object.Builtins)
}

// NewWithState creates a new Compiler with a pre-existing symbol table and
// constant pool, type info, and namespace info, which is useful for testing or for a REPL environment.
func NewWithState(s *SymbolTable, constants []object.Object, typeInfo map[string]ast.Node, pouNamespaces map[string]*ast.NamespaceDeclaration) *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
		varDecls:            make(map[string]*ast.VarDeclStatement),
	}
	// Ensure typeInfo and pouNamespaces are not nil, even if passed as nil.
	// This allows sharing existing maps or creating new ones if none are provided.
	if typeInfo == nil {
		typeInfo = make(map[string]ast.Node)
	}
	if pouNamespaces == nil {
		pouNamespaces = make(map[string]*ast.NamespaceDeclaration)
	}

	return &Compiler{
		constants:     constants,
		symbolTable:   s,
		scopes:        []CompilationScope{mainScope},
		scopeIndex:    0,
		functionStack: []*ast.FunctionDeclaration{},
		loopCtxStack:  []*loopContext{},
		currentFB:     nil,
		typeInfo:      typeInfo,
		pouNamespaces: pouNamespaces,
		rootProgram:   nil,
	}
}

// loopContext holds information about a loop being compiled, such as the
// positions of `EXIT` statements (`break` in Go) that need to be patched.
type loopContext struct {
	breakPositions []int
}

// pushFunction adds a function declaration to the top of the function stack.
// This is used to track the context of the current function being compiled,
// which is necessary for handling recursion and return values correctly.
func (c *Compiler) pushFunction(fn *ast.FunctionDeclaration) {
	c.functionStack = append(c.functionStack, fn)
	c.functionResults = append(c.functionResults, functionResult{})
}

// popFunction removes the current function declaration from the top of the function stack.
func (c *Compiler) popFunction() {
	if len(c.functionStack) > 0 {
		c.functionStack = c.functionStack[:len(c.functionStack)-1]
		c.functionResults = c.functionResults[:len(c.functionResults)-1]
	}
}

// functionResult describes how the function being compiled returns its result.
type functionResult struct {
	symbol     Symbol // The local variable named after the function, if hasResult.
	hasResult  bool
	hasOutputs bool // The function has VAR_OUTPUTs, returned alongside the result.
}

// setFunctionResult records the current function's result variable and
// whether it also returns VAR_OUTPUT values.
func (c *Compiler) setFunctionResult(symbol Symbol, hasOutputs bool) {
	c.functionResults[len(c.functionResults)-1] = functionResult{symbol: symbol, hasResult: true, hasOutputs: hasOutputs}
}

// emitFunctionReturn returns from the current function. In IEC 61131-3,
// assigning to the function's name only sets the result; the function returns
// that value when it ends or at RETURN. Functions with VAR_OUTPUTs return the
// result and the outputs together, and the caller unpacks them.
func (c *Compiler) emitFunctionReturn() {
	if len(c.functionResults) == 0 || !c.functionResults[len(c.functionResults)-1].hasResult {
		c.emit(code.OpReturn)
		return
	}
	result := c.functionResults[len(c.functionResults)-1]
	c.loadSymbol(result.symbol)
	if result.hasOutputs {
		c.emit(code.OpReturnValueMulti)
	} else {
		c.emit(code.OpReturnValue)
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

// emitForCondition emits the test that decides whether a FOR loop runs another
// iteration, leaving a BOOL on the stack. The loop continues while the control
// variable has not passed the end value in the direction of the step: `<=` for
// a positive step (the default) and `>=` for a negative one. A step that is
// not a constant has its sign checked at runtime.
func (c *Compiler) emitForCondition(control Symbol, node *ast.ForLoopStatement) error {
	compare := func(op code.Opcode) error {
		c.loadSymbol(control)
		if err := c.Compile(node.EndValue); err != nil {
			return err
		}
		c.emit(op)
		return nil
	}

	if node.StepValue == nil {
		return compare(code.OpLessThanOrEqual)
	}
	if step, err := c.evaluateConstantInteger(node.StepValue); err == nil {
		if step < 0 {
			return compare(code.OpGreaterThanOrEqual)
		}
		return compare(code.OpLessThanOrEqual)
	}

	// step >= 0 ? control <= end : control >= end
	if err := c.Compile(node.StepValue); err != nil {
		return err
	}
	c.emitConstant(c.addConstant(&object.LInt{Value: 0}))
	c.emit(code.OpGreaterThanOrEqual)
	negativeStep := c.emit(code.OpJumpNotTruthy, 9999)
	if err := compare(code.OpLessThanOrEqual); err != nil {
		return err
	}
	done := c.emit(code.OpJump, 9999)
	c.changeOperand(negativeStep, len(c.currentInstructions()))
	if err := compare(code.OpGreaterThanOrEqual); err != nil {
		return err
	}
	c.changeOperand(done, len(c.currentInstructions()))
	return nil
}

// emitCaseValueTest emits a test of one CASE label against the selector on top
// of the stack, jumping to the branch body when it matches and falling through
// otherwise. The selector stays on the stack either way. A label is a value or
// a range `low..high`. It returns the positions of jumps to the body.
func (c *Compiler) emitCaseValueTest(value ast.Expression) ([]int, error) {
	if rng, ok := value.(*ast.InfixExpression); ok && rng.Operator == ".." {
		// selector >= low AND selector <= high
		c.emit(code.OpDup)
		if err := c.Compile(rng.Left); err != nil {
			return nil, err
		}
		c.emit(code.OpGreaterThanOrEqual)
		belowRange := c.emit(code.OpJumpNotTruthy, 9999)
		c.emit(code.OpDup)
		if err := c.Compile(rng.Right); err != nil {
			return nil, err
		}
		c.emit(code.OpLessThanOrEqual)
		aboveRange := c.emit(code.OpJumpNotTruthy, 9999)
		toBody := c.emit(code.OpJump, 9999)
		next := len(c.currentInstructions())
		c.changeOperand(belowRange, next)
		c.changeOperand(aboveRange, next)
		return []int{toBody}, nil
	}

	c.emit(code.OpDup)
	if err := c.Compile(value); err != nil {
		return nil, err
	}
	c.emit(code.OpEqual)
	noMatch := c.emit(code.OpJumpNotTruthy, 9999)
	toBody := c.emit(code.OpJump, 9999)
	c.changeOperand(noMatch, len(c.currentInstructions()))
	return []int{toBody}, nil
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
func (c *Compiler) CompileProgram(node *ast.ProgramDeclaration) (*CompiledProgram, error) {
	// --- Initialization Phase ---
	// Compile all variable declaration blocks (VAR, VAR_GLOBAL, etc.).
	// This populates the symbol table and generates bytecode to set initial values.
	varDecls := []ast.Statement{}
	for _, b := range node.VarGlobal {
		varDecls = append(varDecls, b)
	}
	// A program run on its own has no caller, so its inputs, outputs and
	// in-outs are ordinary variables that start at their initial values.
	for _, block := range [][]*ast.VarDeclStatement{node.VarInputs, node.VarOutputs, node.VarInOuts, node.Vars} {
		for _, d := range block {
			varDecls = append(varDecls, d)
		}
	}
	for _, b := range node.VarExternal {
		varDecls = append(varDecls, b)
	}
	for _, b := range node.VarAccess {
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
	cyclicCompiler := NewWithState(c.symbolTable, c.constants, c.typeInfo, c.pouNamespaces)

	// Compile VAR_TEMP at the start of the cyclic code so they are re-initialized on each scan.
	for _, tempBlock := range node.VarTemp {
		if err := cyclicCompiler.Compile(tempBlock); err != nil {
			return nil, err
		}
	}

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
		// First pass: recursively build POU info, including namespaces.
		c.buildPouInfo(node)
		c.rootProgram = node // Set the root program node
		// The standard function blocks the program uses are compiled with it.
		stmts := c.withStandardFBs(node)
		c.predefineGlobals(stmts)
		c.predefineFunctionBlocks(stmts)

		// Second pass: compile all statements. Function blocks are ordered so that a
		// parent is always compiled before any FB that EXTENDS it, because the derived
		// FB's hash references the parent's hash at runtime.
		for _, s := range c.orderByInheritance(stmts) {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	// A Configuration block is compiled into a data structure representing the system setup.
	case *ast.ConfigurationDeclaration:
		return c.compileConfiguration(node)

	case *ast.NamespaceDeclaration:
		originalNS := c.currentNS
		c.currentNS = node
		for _, s := range c.orderByInheritance(node.Statements) {
			if err := c.Compile(s); err != nil {
				return err
			}
		}
		c.currentNS = originalNS
		return nil

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
		symbol := c.symbolTable.Define(node.Name.Value, false)

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
			c.emitConstant(c.addConstant(typeDefObject))
		}
		return c.setSymbol(symbol)

	case *ast.InterfaceDeclaration:
		// Define the interface name in the symbol table and store its AST node for type checking.
		symbol := c.symbolTable.Define(node.Name.Value, false)

		// Check for extending a FINAL interface.
		if node.Extends != nil {
			for _, parentIdent := range node.Extends {
				parentName := c.flattenExpressionToString(parentIdent)
				parentDefNode, ok := c.resolveTypeNode(parentIdent)
				if !ok {
					return fmt.Errorf("parent interface '%s' not found", parentName)
				}
				parentIface, ok := parentDefNode.(*ast.InterfaceDeclaration)
				if !ok {
					return fmt.Errorf("parent '%s' is not an interface", parentName)
				}
				if parentIface.IsFinal {
					return fmt.Errorf("cannot extend from FINAL interface '%s'", parentIface.Name.Value)
				}
			}
		}
		// Interfaces don't generate executable code, they are for compile-time checks.
		// We emit a null and set it to a global var to have a placeholder.
		c.emit(code.OpNull)
		return c.setSymbol(symbol)

	// A FunctionBlockDeclaration is compiled into a callable closure, similar to a function,
	// representing the FB "template" or class.
	case *ast.FunctionBlockDeclaration:
		// --- Circular Inheritance Check ---
		visited := make(map[string]bool)
		current := node
		for current != nil {
			if visited[current.Name.Value] {
				return fmt.Errorf("circular function block inheritance detected: '%s' is already in the inheritance path", current.Name.Value)
			}
			visited[current.Name.Value] = true

			if current.Extends == nil {
				break
			}

			parentObj, ok := c.resolveTypeNode(current.Extends)
			if !ok {
				// This error will be caught later, but we can break here.
				break
			}
			parentFB, ok := parentObj.(*ast.FunctionBlockDeclaration)
			if !ok {
				break
			}
			current = parentFB
		}

		// An FB declaration compiles into a hash of its constituent parts:
		// the main logic body, and each method. Its global symbol may already
		// have been pre-defined so that it can be instantiated before this point.
		symbol, predefined := c.symbolTable.ResolveClass(node)
		if !predefined || c.scopeIndex != 0 {
			symbol = c.symbolTable.Define(node.Name.Value, false)
			c.symbolTable.DefineClass(node, symbol)
		}

		// If the FB extends another, we need to add a "__parent__" key to the hash.
		// The VM will use this to walk the inheritance chain.
		var parentSymbol *Symbol
		var parentFB *ast.FunctionBlockDeclaration
		if node.Extends != nil {
			// --- FINAL Function Block Check ---
			parentName := c.flattenExpressionToString(node.Extends)
			parentDefNode, ok := c.resolveTypeNode(node.Extends)
			if !ok {
				return fmt.Errorf("parent function block '%s' definition not found", parentName)
			}
			parentFB, ok = parentDefNode.(*ast.FunctionBlockDeclaration)
			if !ok {
				return fmt.Errorf("parent '%s' is not a function block", parentName)
			}
			if parentFB.IsFinal {
				return fmt.Errorf("cannot extend from FINAL function block '%s'", parentFB.Name.Value)
			}
			// Resolve the parent symbol by its flattened name
			resolved, ok := c.symbolTable.Resolve(parentName)
			if !ok {
				return fmt.Errorf("parent function block '%s' not found", parentName)
			}
			parentSymbol = &resolved
		}

		// Set context for compiling methods, allowing them to resolve FB members.
		c.currentFB = node
		defer func() { c.currentFB = nil }()

		// Filter out methods from the main body logic.
		mainLogicStmts := []ast.Statement{}
		methods := []*ast.MethodImplementation{}
		if bodyBlock, ok := node.Body.(*ast.BlockStatement); ok {
			for _, stmt := range bodyBlock.Statements {
				if method, isMethod := stmt.(*ast.MethodImplementation); isMethod {
					methods = append(methods, method)
				} else if varDecl, isVarDecl := stmt.(*ast.VarDeclStatement); isVarDecl {
					// The parser may place individual VAR declarations in the body.
					// Add it to the FB's list of instance variables for later resolution.
					node.Vars = append(node.Vars, varDecl)
				} else if varBlock, isVarBlock := stmt.(*ast.VarBlockDeclaration); isVarBlock {
					// The parser may place VAR blocks in the body. Add their contents
					// to the FB's list of instance variables for later resolution.
					node.Vars = append(node.Vars, varBlock.Declarations...)
				} else {
					mainLogicStmts = append(mainLogicStmts, stmt)
				}
			}
		}
		mainBody := &ast.BlockStatement{Statements: mainLogicStmts}

		// --- FINAL Method Override Check ---
		if parentFB != nil {
			for _, derivedMethod := range methods {
				if parentMethod, ownerFB := c.findMethodOnFBChain(parentFB, derivedMethod.Name.Value); parentMethod != nil {
					// The 'ownerFB' returned here is the specific function block in the
					// inheritance chain that defines the parent method. This is crucial for
					// providing clear error messages, e.g., "cannot override FINAL method 'X'
					// from function block 'Y'", where 'Y' is ownerFB.Name.Value.
					if parentMethod.IsFinal {
						return fmt.Errorf("cannot override FINAL Method '%s' from function block '%s'", parentMethod.Name.Value, ownerFB.Name.Value)
					}
					// --- Method Signature Validation ---
					if err := c.validateMethodOverride(derivedMethod, parentMethod); err != nil {
						return err
					}
				}
			}
		}

		// --- FINAL Property Override Check ---
		if parentFB != nil {
			for _, derivedProp := range node.Properties {
				parentProp, ownerFB := c.findPropertyOnFBChain(parentFB, derivedProp.Name.Value)
				if parentProp != nil && parentProp.IsFinal {
					return fmt.Errorf("cannot override FINAL property '%s' from function block '%s'", parentProp.Name.Value, ownerFB.Name.Value)
				}
			}
		}

		// --- FINAL Variable Override Check ---
		// A derived FB may not redeclare a variable that an ancestor declared FINAL.
		if parentFB != nil {
			for _, derivedVar := range node.Vars {
				parentVar, ownerFB := c.findVarDeclOnFBChain(parentFB, derivedVar.Name.Value)
				if parentVar != nil && parentVar.IsFinal {
					return fmt.Errorf("cannot override FINAL variable '%s' from function block '%s'", parentVar.Name.Value, ownerFB.Name.Value)
				}
			}
		}

		// --- Abstract Member Implementation Check ---
		// A concrete class must implement all abstract members from its parents.
		if err := c.checkAbstractImplementation(node); err != nil {
			return err
		}

		// Compile methods
		methodConstants := make(map[string]int)
		for _, prop := range node.Properties {
			if prop.Getter != nil {
				getter, err := c.compilePropertyAccessor(prop, prop.Getter.Body, true)
				if err != nil {
					return err
				}
				methodConstants["get_"+prop.Name.Value] = c.addConstant(getter)
			}
			if prop.Setter != nil {
				setter, err := c.compilePropertyAccessor(prop, prop.Setter.Body, false)
				if err != nil {
					return err
				}
				methodConstants["set_"+prop.Name.Value] = c.addConstant(setter)
			}
		}
		for _, method := range methods {
			compiledMethod, err := c.compileMethod(method)
			if err != nil {
				return err
			}
			methodConstants[method.Name.Value] = c.addConstant(compiledMethod)
		}

		// Compile the main body, which runs when an instance is called, e.g.
		// `f(IN := x)`. Like a method, it takes the instance as THIS, and every
		// variable of the function block (inputs, outputs, in-outs and VARs)
		// is a field of the instance reached through THIS, so values persist
		// between calls.
		c.enterScope()
		thisSymbol := c.symbolTable.Define("THIS", false) // 'THIS' is implicitly local 0
		// A derived function block runs its parent's body first.
		if parentSymbol != nil {
			c.loadSymbol(*parentSymbol)
			c.emitConstant(c.addConstant(&object.String{Value: "main"}))
			c.emit(code.OpIndex)
			c.loadSymbol(thisSymbol)
			c.emit(code.OpCall, 1)
			c.emit(code.OpPop)
		}
		for _, tempBlock := range node.VarTemp {
			// VAR_TEMP starts afresh on every call.
			if err := c.Compile(tempBlock); err != nil {
				return err
			}
		}
		if err := c.Compile(mainBody); err != nil {
			return err
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		numLocals := c.symbolTable.numDefinitions
		mainLogicInstructions := c.leaveScope()
		mainFn := &object.CompiledFunction{
			Instructions:   mainLogicInstructions,
			NumLocals:      numLocals,
			NumParameters:  1, // THIS
			ParameterNames: []string{"THIS"},
		}
		mainFnIndex := c.addConstant(mainFn)

		// --- Compile-time Interface Implementation Check ---
		if !node.IsAbstract {
			if err := c.checkInterfaceImplementation(node); err != nil {
				return err
			}
		}

		// Add main to the method map to be sorted and compiled with others.
		methodConstants["main"] = mainFnIndex

		numHashPairs := len(methodConstants)
		if parentSymbol != nil {
			numHashPairs++
		}

		// Create the hash that represents the FB class.
		// The parent, if it exists, is always added first and is not part of the sorted keys.
		if parentSymbol != nil {
			c.emitConstant(c.addConstant(&object.String{Value: "__parent__"}))
			c.loadSymbol(*parentSymbol) // Load the parent FB's hash
		}
		// Sort the method/property names to ensure deterministic bytecode generation,
		// which is crucial for stable testing.
		type methodPair struct { // cspell:disable-line
			name  string // cspell:disable-line
			index int    // cspell:disable-line
		}
		pairs := make([]methodPair, 0, len(methodConstants)) // cspell:disable-line
		for k, v := range methodConstants {                  // cspell:disable-line
			pairs = append(pairs, methodPair{name: k, index: v}) // cspell:disable-line
		}
		sort.Slice(pairs, func(i, j int) bool { // cspell:disable-line
			return pairs[i].name < pairs[j].name // cspell:disable-line
		})

		for _, pair := range pairs { // cspell:disable-line
			// By pairing the name and index together before sorting, we ensure
			// that the key-value pairs for the hash are emitted correctly and
			// deterministically, preventing scrambled outputs.
			c.emitConstant(c.addConstant(&object.String{Value: pair.name})) // cspell:disable-line
			c.emit(code.OpClosure, pair.index, 0)                           // cspell:disable-line
		}
		c.emit(code.OpHash, numHashPairs*2)
		return c.setSymbol(symbol)

	// An SFCProgram is compiled into a static data structure (a hash).
	case *ast.SFCProgram:
		return c.compileSFCProgram(node)

	case *ast.ActionStatement:
		// An Action is like a parameter-less function.
		// Compile its body into a callable unit.
		symbol := c.symbolTable.Define(node.Name.Value, false)

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
		return c.setSymbol(symbol)

	// Step and Transition statements are only valid within an SFC program body.
	case *ast.StepStatement, *ast.TransitionStatement:
		// These nodes are now handled exclusively within compileSFCProgram.
		// If they are encountered here, it's a structural error in the AST
		// or an unsupported use case.
		return fmt.Errorf("%T is only valid inside a PROGRAM with an SFC body", node)

	// A ProgramDeclaration compiles all its variable blocks and then its body.
	case *ast.ProgramDeclaration:
		// When a PROGRAM is encountered at the top level (scopeIndex 0), it's treated as a
		// POU definition if a CONFIGURATION block is also present at the root. In that
		// case, we only register its type information. It doesn't generate executable
		// code itself; it's a template for instances.
		if c.scopeIndex == 0 {
			hasConfig := false // cspell:disable-line
			if c.rootProgram != nil {
				for _, stmt := range c.rootProgram.Statements {
					if _, ok := stmt.(*ast.ConfigurationDeclaration); ok {
						hasConfig = true
						break
					}
				}
			}

			if hasConfig {
				c.typeInfo[strings.ToUpper(node.Name.Value)] = node
				return nil
			}
			// Declaring a program defines it; a call runs it.
			if _, isSFC := node.Body.(*ast.SFCProgram); !isSFC {
				return c.compileCallableProgram(node)
			}
		}

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
		// A VAR_IN_OUT is passed by copy-in/copy-out: it is a parameter after
		// the VAR_INPUTs, and is returned with the outputs so the caller can
		// copy its final value back into the argument.
		// First, define the function name in the current scope so it can be captured in a closure.
		symbol := c.definePredefined(node, node.Name.Value, false)

		// Then, compile the function body itself.
		c.enterScope()
		c.pushFunction(node)

		c.symbolTable.DefineFunctionName(node.Name.Value) // For recursion

		params := append(append([]*ast.VarDeclStatement{}, node.VarInputs...), node.VarInOuts...)
		paramNames := make([]string, len(params))
		inOutIndices := make([]int, len(node.VarInOuts))
		// Define input parameters first, as they are the first locals in the stack frame.
		for i, p := range params {
			typeName := c.flattenExpressionToString(p.DataType)
			param := c.symbolTable.DefineVarInput(p.Name.Value, typeName)
			paramNames[i] = p.Name.Value
			if i >= len(node.VarInputs) {
				// Unlike an input, an in-out may be assigned.
				param.IsReadOnly = false
				c.symbolTable.store[strings.ToUpper(p.Name.Value)] = param
				inOutIndices[i-len(node.VarInputs)] = param.Index
			}
		}
		if err := c.prepareParameters(params); err != nil {
			return err
		}

		// Define the function name as a local variable to hold the return value.
		returnType := ""
		if node.ReturnType != nil {
			returnType = c.flattenExpressionToString(node.ReturnType)
		}
		returnSymbol := c.symbolTable.Define(node.Name.Value, false, returnType)

		outputNames := make([]string, len(node.VarOutputs))
		outputIndices := make([]int, len(node.VarOutputs))
		for i, p := range node.VarOutputs {
			outputNames[i] = p.Name.Value
			// The index will be populated when the symbol is defined below.
		}

		// The result starts at the default of the return type, so a function
		// that never assigns it returns e.g. 0 for INT, not NULL.
		if err := c.compileStartingValue(node.Name.Value, node.ReturnType, nil, map[ast.Node]bool{}); err != nil {
			return err
		}
		c.emit(code.OpSetLocal, returnSymbol.Index)

		// VAR_OUTPUTs start at their initial value or their type's default.
		for i, p := range node.VarOutputs {
			if err := c.compileVarValue(p); err != nil {
				return err
			}
			symbol := c.symbolTable.Define(p.Name.Value, false)
			outputIndices[i] = symbol.Index
			c.emit(code.OpSetLocal, symbol.Index)
		}
		for i, p := range node.VarInOuts {
			outputNames = append(outputNames, p.Name.Value)
			outputIndices = append(outputIndices, inOutIndices[i])
		}
		c.setFunctionResult(returnSymbol, len(outputNames) > 0)

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

		if block, ok := node.Body.(*ast.BlockStatement); ok && isIlBlock(block) {
			// An IL function that falls off the end returns its current result,
			// the value left on the stack.
			if !c.lastInstructionIs(code.OpReturnValue) {
				if len(c.currentInstructions()) > 0 && !c.lastInstructionIs(code.OpReturn) {
					c.emit(code.OpReturnValue)
				} else if !c.lastInstructionIs(code.OpReturn) {
					c.emit(code.OpReturn)
				}
			}
		} else {
			// Return the function's result variable (and any VAR_OUTPUTs).
			c.emitFunctionReturn()
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		c.popFunction()
		compiledFn := &object.CompiledFunction{
			Instructions:   instructions,
			NumLocals:      numLocals,
			NumParameters:  len(params),
			ParameterNames: paramNames,
			OutputNames:    outputNames,
			OutputIndices:  outputIndices,
		}
		fnIndex := c.addConstant(compiledFn)

		// Create the closure and assign it to the variable in the outer scope.
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))
		return c.setSymbol(symbol)

	// An ExpressionStatement's result is unused, so it's popped from the stack.
	// An ExpressionStatement's result is unused, so it's popped from the stack.
	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		// The result of an expression statement is not used, so we must
		// pop it from the stack to prevent stack corruption.
		c.emit(code.OpPop)

	case *ast.AssignmentStatement:
		// Arrays and structures are assigned by value.
		// A bit assignment, flags.3 := value, stores the whole target.
		node = assignBit(node)
		node = c.copyAssignedValue(node)
		// An array assigned to a variable declared with lower bounds, e.g.
		// ARRAY[1..3], takes those bounds, whatever array it came from.
		if !c.stampingArray {
			if def := c.declaredArrayType(node.Left); def != nil {
				if _, nonZero := c.arrayLowerBounds(def); nonZero {
					c.stampingArray = true
					err := c.Compile(node)
					c.stampingArray = false
					if err != nil {
						return err
					}
					if err := c.Compile(node.Left); err != nil {
						return err
					}
					if err := c.emitArrayBounds(def); err != nil {
						return err
					}
					c.emit(code.OpPop)
					return nil
				}
			}
		}
		// Use a type switch to safely handle different kinds of assignment targets
		// and prevent panics if the parser produces an unexpected AST node.
		switch target := node.Left.(type) {
		case *ast.Identifier:
			// An assignment to the current function's name sets its result. It
			// does not return: the rest of the body still runs, and the function
			// returns the result when it ends or at RETURN.
			if currentFn := c.currentFunction(); currentFn != nil && strings.EqualFold(target.Value, currentFn.Name.Value) {
				result := c.functionResults[len(c.functionResults)-1]
				if result.hasResult {
					if err := c.Compile(node.Value); err != nil {
						return err
					}
					return c.setSymbol(result.symbol)
				}
			}

			// Check if this is an assignment to an instance variable (VAR) or a parameter (VAR_INPUT/VAR_OUTPUT/VAR_IN_OUT)
			// of the current function block.
			if c.currentFB != nil {
				if varDecl, ownerFB := c.findVarDeclOnFBChain(c.currentFB, target.Value); varDecl != nil {
					// Check for FINAL variable override.
					if varDecl.IsFinal {
						return fmt.Errorf("cannot assign to FINAL variable '%s' from function block '%s'", varDecl.Name.Value, ownerFB.Name.Value)
					}
					if varDecl.IsConstant {
						return fmt.Errorf("cannot assign to a constant variable '%s'", varDecl.Name.Value)
					}
					// Check for read-only (VAR_INPUT)
					if varDecl.Scope == "VAR_INPUT" {
						return fmt.Errorf("cannot assign to read-only variable '%s'", target.Value)
					}

					// If it's an instance variable (VAR, VAR_TEMP, VAR_GLOBAL in FB), compile as THIS.variable := value
					// VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT are handled as local symbols.
					local, isLocal := c.symbolTable.Resolve(target.Value)
					isLocal = isLocal && !c.hidesSymbol(local, target.Value)
					if varDecl.Scope == "" || varDecl.Scope == "VAR" || varDecl.Scope == "VAR_TEMP" || varDecl.Scope == "VAR_GLOBAL" || !isLocal {
						// Compile as `THIS.variable := value`. Inside a method, the FB's
						// outputs are also instance fields rather than locals.
						thisSymbol, _ := c.symbolTable.Resolve("THIS")
						c.loadSymbol(thisSymbol)
						c.emitConstant(c.addConstant(&object.String{Value: target.Value}))
						if err := c.Compile(node.Value); err != nil {
							return err
						}
						c.emit(code.OpSetIndex)
						return nil
					}
				}

				// Inside a function block, assigning to one of its properties by
				// name calls the property's setter on THIS, just as reading it
				// by name calls the getter.
				if propDecl, _ := c.findPropertyOnFBChain(c.currentFB, target.Value); propDecl != nil {
					if propDecl.Setter == nil {
						return fmt.Errorf("property '%s' is read-only", propDecl.Name.Value)
					}
					return c.compilePropertySet(&ast.ThisExpression{Token: target.Token}, propDecl.Name.Value, node.Value)
				}
			}

			// Otherwise, it's a regular variable assignment.
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			symbol, ok := c.symbolTable.Resolve(target.Value)
			if !ok {
				return fmt.Errorf("undefined variable %s", target.Value)
			}
			return c.setSymbol(symbol)

		case *ast.IndexExpression:
			if err := c.checkArrayBounds(target.Left, target.Index); err != nil {
				return err
			}
			// Compile the array/hash (LHS of index expression)
			if err := c.Compile(target.Left); err != nil {
				return err
			}
			// Compile the index
			if err := c.Compile(target.Index); err != nil {
				return err
			}
			// Compile the value to be assigned (RHS)
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			c.emit(code.OpSetIndex)
			return nil
		case *ast.MemberAccessExpression:
			// Check if this is an assignment to a property.
			// --- Start of new access control logic for assignment ---
			if structTypeName, ok := c.getExpressionTypeName(target.Struct); ok {
				if structTypeNode, ok := c.resolveTypeName(structTypeName); ok {
					if fbDef, isFB := structTypeNode.(*ast.FunctionBlockDeclaration); isFB {
						member := target.Member.Value

						// Check if it's a variable write
						if varDecl, ownerDef := c.findVarDeclOnFBChain(fbDef, member); varDecl != nil {
							if err := c.checkAccessPermission(varDecl.AccessSpecifier, ownerDef); err != nil {
								return fmt.Errorf("cannot assign to member variable '%s': %w", member, err)
							}
						} else if propDecl, ownerDef := c.findPropertyOnFBChain(fbDef, member); propDecl != nil {
							// It's a property write (SET). Check setter permission.
							if propDecl.Setter != nil {
								accessSpecifier := propDecl.Setter.AccessSpecifier
								if accessSpecifier == "" {
									accessSpecifier = propDecl.AccessSpecifier
								}
								if err := c.checkAccessPermission(accessSpecifier, ownerDef); err != nil {
									return fmt.Errorf("cannot access setter for property '%s': %w", member, err)
								}
							} else {
								return fmt.Errorf("property '%s' is read-only", member)
							}
						}
						// Method assignment is not possible, so we don't check methods here.
					}
				}
			}
			// --- End of new access control logic ---

			isProperty := false
			if c.currentFB != nil {
				// This is a simplification. A real implementation would need to know the type
				// of `target.Struct` to check its properties. For now, we assume access on `THIS`.
				if _, ok := target.Struct.(*ast.ThisExpression); ok {
					for _, prop := range c.currentFB.Properties {
						if prop.Name.Value == target.Member.Value {
							isProperty = true
							break
						}
					}
				}
			}

			if isProperty {
				// It's a property set. Compile as a call to the setter method.
				// e.g., `p.MyProp := 5` becomes a call to `p.set_MyProp(5)`
				setterCall := &ast.CallExpression{
					Function: &ast.MemberAccessExpression{
						Struct: target.Struct,
						Member: &ast.Identifier{Value: "set_" + target.Member.Value},
					},
					Arguments: []ast.Expression{node.Value},
				}
				return c.Compile(setterCall)
			} else {
				// It's a regular field assignment.
				// Before we assume it's a field, check if it's a SUPER call.
				if deref, ok := target.Struct.(*ast.DereferenceExpression); ok { // cspell:disable-line
					if _, ok := deref.Pointer.(*ast.SuperExpression); ok { // cspell:disable-line
						// This is an assignment to a SUPER member. Assume it's a property set.
						return c.compilePropertySet(target.Struct, target.Member.Value, node.Value)
					}
				}

				// Check if the left-hand side is a property access, which requires a setter call.
				if isProperty, propName := c.isPropertyAccess(target); isProperty {
					// It's a property SET.
					return c.compilePropertySet(target.Struct, propName, node.Value)
				}

				// It's a regular field assignment.
				if err := c.Compile(target.Struct); err != nil {
					return err
				}
				c.emitConstant(c.addConstant(&object.String{Value: target.Member.Value}))
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				c.emit(code.OpSetIndex)
				return nil
			}

		default:
			// This default case catches any other type, including `nil`,
			// preventing the panic and providing a clear error message.
			return fmt.Errorf("unsupported assignment target: %T", node.Left)
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

	// ExternalVarDeclaration compiles each external variable. A VAR_EXTERNAL
	// refers to a VAR_GLOBAL, so when a matching global already exists it is
	// bound to that symbol and no code is emitted. Otherwise a placeholder
	// variable is defined, to be bound later (e.g. by a configuration).
	case *ast.ExternalVarDeclaration:
		for _, decl := range node.Vars {
			if existing, ok := c.symbolTable.Resolve(decl.Name.Value); ok && existing.Scope == GlobalScope {
				continue
			}
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
		// If we are inside a function block definition (but not inside a method/function scope),
		// this is an instance variable. Its initial value should not be compiled as executable code.
		// The VM handles instance variable initialization.
		if c.currentFB != nil && c.symbolTable.Outer == nil {
			// This condition identifies a VarDecl that is part of an FB's structure
			// rather than part of executable logic (like a local var in a method).
			return nil
		}

		// If the variable is a macro definition, skip it entirely as it has no
		// runtime equivalent in the compiled code.
		if _, ok := node.Value.(*ast.MacroLiteral); ok {
			return nil
		}

		// Handle located variables (AT %) by treating them as external symbols.
		// The VM will be responsible for mapping the address string to physical I/O.
		if node.Location != nil {
			address := node.Location.Location.String()
			constIndex := c.addConstant(&object.String{Value: address})
			symbol := c.symbolTable.DefineExternal(node.Name.Value, constIndex)

			// If an initial value is provided, compile it and emit OpSetExternal.
			if node.Value != nil {
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				c.emit(code.OpSetExternal, symbol.Index)
			}
			return nil // This declaration is fully handled.
		}

		var typeName string
		if node.DataType != nil {
			// Get the string representation of the type, e.g., "INT", "MyStruct".
			typeName = c.flattenExpressionToString(node.DataType)
		}

		// The starting value is compiled before the name is defined, so that an
		// initial value cannot refer to the variable being declared, and a
		// variable named like its type (`counter : Counter`) still sees the type.
		if err := c.compileVarValue(node); err != nil {
			return err
		}
		symbol := c.definePredefined(node, node.Name.Value, node.IsConstant, typeName)
		c.scopes[c.scopeIndex].varDecls[node.Name.Value] = node

		// A declaration's initial value is not an assignment, so it also
		// initializes CONSTANT variables.
		return c.initSymbol(symbol)

	// An InfixExpression compiles the left and right sides, then emits the operator instruction.
	case *ast.InfixExpression:
		// --- Start of new type-checking logic ---
		// First, determine the types of the left and right operands.
		leftType, err := c.getExpressionType(node.Left)
		if err != nil {
			return err
		}
		rightType, err := c.getExpressionType(node.Right)
		if err != nil {
			return err
		}
		leftType, rightType = literalOperandTypes(node, leftType, rightType)
		// Then, check if the operator is valid for these types.
		if _, err := c.getResultingType(node.Operator, leftType, rightType); err != nil {
			return fmt.Errorf("type error in expression '%s': %w", node.String(), err)
		}
		// --- End of new type-checking logic ---

		err = c.Compile(node.Left)
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
		c.emitConstant(c.addConstant(lint))

	// A RealLiteral is added to the constant pool and an OpConstant instruction is emitted.
	case *ast.RealLiteral:
		lreal := &object.LReal{Value: node.Value}
		c.emitConstant(c.addConstant(lreal))

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
		// IF is a statement: its branches leave nothing on the stack, so an IF
		// in a loop or at the top level keeps the stack balanced whichever
		// branch runs.
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		if err := c.Compile(node.Consequence); err != nil {
			return err
		}

		jumpPos := c.emit(code.OpJump, 9999)
		c.changeOperand(jumpNotTruthyPos, len(c.currentInstructions()))
		if node.Alternative == nil {
			// No branch ran: the IF's value, as the REPL shows it, is NULL.
			c.emit(code.OpNull)
			c.emit(code.OpPop)
		} else if err := c.Compile(node.Alternative); err != nil {
			return err
		}
		c.changeOperand(jumpPos, len(c.currentInstructions()))

	case *ast.UnsignedIntegerLiteral:
		c.emitConstant(c.addConstant(&object.ULInt{Value: node.Value}))

	// An LRealLiteral is added to the constant pool.
	case *ast.LRealLiteral:
		c.emitConstant(c.addConstant(&object.LReal{Value: node.Value}))

	// A WStringLiteral is added to the constant pool.
	case *ast.WStringLiteral:
		c.emitConstant(c.addConstant(&object.WString{Value: node.Value}))

	// A BitStringLiteral is added to the constant pool.
	case *ast.BitStringLiteral:
		c.emitConstant(c.addConstant(&object.BitString{Value: node.Value, Width: node.Width}))

	// An EnumeratedValueLiteral is added to the constant pool.
	case *ast.EnumeratedValueLiteral:
		c.emitConstant(c.addConstant(&object.EnumeratedValue{
			TypeName: node.TypeName.Value,
			Value:    node.Value.Value,
		}))

	// A TypedLiteral is parsed and converted into the appropriate object.Object,
	// which is then added to the constant pool.
	case *ast.TypedLiteral:
		return c.compileTypedLiteral(node)

	// Specific time/date literals are parsed into their own AST nodes.
	// We reconstruct a generic TypedLiteral to reuse the parsing logic.
	case *ast.TimeLiteral:
		return c.compileTypedLiteral(&ast.TypedLiteral{
			Token:    node.Token,
			TypeName: "TIME",
			Value:    &ast.Identifier{Token: node.Token, Value: node.Value},
		})
	case *ast.DateLiteral:
		return c.compileTypedLiteral(&ast.TypedLiteral{
			Token:    node.Token,
			TypeName: "DATE",
			Value:    &ast.Identifier{Token: node.Token, Value: node.Value},
		})
	case *ast.TimeOfDayLiteral:
		return c.compileTypedLiteral(&ast.TypedLiteral{
			Token:    node.Token,
			TypeName: "TIME_OF_DAY",
			Value:    &ast.Identifier{Token: node.Token, Value: node.Value},
		})
	case *ast.DateAndTimeLiteral:
		return c.compileTypedLiteral(&ast.TypedLiteral{
			Token:    node.Token,
			TypeName: "DATE_AND_TIME",
			Value:    &ast.Identifier{Token: node.Token, Value: node.Value},
		})

	case *ast.ThisExpression:
		thisSymbol, ok := c.symbolTable.Resolve("THIS")
		if !ok {
			return fmt.Errorf("cannot use THIS outside of a function block context")
		}
		c.loadSymbol(thisSymbol)

	case *ast.DereferenceExpression:
		// The '^' operator is for dereferencing pointers. In the context of
		// `THIS^`, `THIS` is already the instance reference, so the dereference
		// is effectively a no-op for the compiler's purpose. We just compile
		// the expression being pointed to.
		return c.Compile(node.Pointer)

	// A ForLoopStatement is compiled into a sequence of initialization, condition
	// check, body, increment, and jump instructions to create the loop structure.
	// It uses its own scope for the loop control variable.
	case *ast.ForLoopStatement:
		c.enterLoop()

		// 1. Initialization. The control variable is an ordinary declared
		// variable; one that was not declared is defined here.
		controlIdent, ok := node.ControlVar.Left.(*ast.Identifier)
		if !ok {
			return fmt.Errorf("FOR control variable must be an identifier, got %T", node.ControlVar.Left)
		}
		if err := c.Compile(node.ControlVar.Value); err != nil {
			return err
		}
		symbol, ok := c.symbolTable.Resolve(controlIdent.Value)
		if !ok {
			symbol = c.symbolTable.Define(controlIdent.Value, false)
		}
		if err := c.setSymbol(symbol); err != nil {
			return err
		}

		loopStartPos := len(c.currentInstructions())

		// 2. Condition check: control_var <= end for a positive step, and
		// control_var >= end for a negative one.
		if err := c.emitForCondition(symbol, node); err != nil {
			return err
		}
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
			c.emitConstant(c.addConstant(&object.LInt{Value: 1}))
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
		c.patchLoopBreaks(afterLoopPos)

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
		c.patchLoopBreaks(afterLoopPos)

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
		c.patchLoopBreaks(afterLoopPos)

	// A CaseStatement compiles the selector expression, then for each branch, it
	// compares the selector to the case value and jumps to the branch's body if
	// they are equal. It also handles the optional ELSE block.
	case *ast.CaseStatement:
		// The selector stays on the stack while each branch's labels are
		// tested. A branch runs when any of its labels (values or ranges)
		// matches; otherwise the ELSE block, if any, runs.
		if err := c.Compile(node.Expression); err != nil {
			return err
		}

		exitJumps := []int{}
		for _, branch := range node.Cases {
			toBody := []int{}
			for _, value := range branch.Values {
				jumps, err := c.emitCaseValueTest(value)
				if err != nil {
					return err
				}
				toBody = append(toBody, jumps...)
			}
			// No label matched: skip this branch's body.
			nextBranch := c.emit(code.OpJump, 9999)

			bodyPos := len(c.currentInstructions())
			for _, pos := range toBody {
				c.changeOperand(pos, bodyPos)
			}
			c.emit(code.OpPop) // Pop selector
			if err := c.Compile(branch.Consequence); err != nil {
				return err
			}
			exitJumps = append(exitJumps, c.emit(code.OpJump, 9999))
			c.changeOperand(nextBranch, len(c.currentInstructions()))
		}

		c.emit(code.OpPop) // Pop selector: no branch matched
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
		// Check if this is an IL program body by inspecting the first statement.
		isIlBlock := false
		for _, stmt := range node.Statements {
			if _, ok := stmt.(*ast.IlInstructionStatement); ok {
				isIlBlock = true
				break
			}
		}

		if isIlBlock {
			return c.compileIlProgram(node)
		}

		// Otherwise, it's a standard ST block.
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	// An Identifier resolves the symbol and emits an instruction to load it.
	case *ast.Identifier:
		// First, try to resolve as a local/global/free variable.
		// This is important to correctly find parameters like 'value' in a setter
		// before attempting to interpret it as an implicit property access on THIS.
		symbol, ok := c.symbolTable.Resolve(node.Value)
		// A function block's own variable hides a built-in function or global of
		// the same name, such as a variable called `first` or `counter`.
		if ok && c.hidesSymbol(symbol, node.Value) {
			ok = false
		}
		if ok {
			c.loadSymbol(symbol)
			return nil // Symbol found and loaded, we are done.
		}

		// If not found, check if it's an implicit THIS access (property or instance var).
		// Check if this identifier refers to an instance variable of the current function block.
		// If so, implicitly transform it into a `THIS.Identifier` access.
		if c.currentFB != nil {
			// Any variable of the FB that is not a local symbol here (e.g. an
			// FB input read inside a method) is a field of the instance.
			if varDecl, _ := c.findVarDeclOnFBChain(c.currentFB, node.Value); varDecl != nil {
				thisExpr := &ast.ThisExpression{Token: node.Token}
				memberAccess := &ast.MemberAccessExpression{Struct: thisExpr, Member: node}
				return c.Compile(memberAccess)
			}

			// Check for properties. If it's a property, implicitly transform to THIS.Property.
			if propDecl, _ := c.findPropertyOnFBChain(c.currentFB, node.Value); propDecl != nil {
				thisExpr := &ast.ThisExpression{Token: node.Token}
				memberAccess := &ast.MemberAccessExpression{Struct: thisExpr, Member: node}
				return c.Compile(memberAccess)
			}
		}

		// If we are here, the symbol was not found in any scope and is not an implicit THIS member.
		return fmt.Errorf("undefined variable %s", node.Value)

	// A StringLiteral is added to the constant pool.
	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emitConstant(c.addConstant(str))

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
		// New bounds checking logic
		if err := c.checkArrayBounds(node.Left, node.Index); err != nil {
			return err
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	case *ast.MemberAccessExpression:
		// --- Start of new access control logic for read access ---
		if structTypeName, ok := c.getExpressionTypeName(node.Struct); ok {
			if structTypeNode, ok := c.resolveTypeName(structTypeName); ok {
				if fbDef, isFB := structTypeNode.(*ast.FunctionBlockDeclaration); isFB {
					member := node.Member.Value

					// Check if it's a variable read
					if varDecl, ownerDef := c.findVarDeclOnFBChain(fbDef, member); varDecl != nil {
						if err := c.checkAccessPermission(varDecl.AccessSpecifier, ownerDef); err != nil {
							return fmt.Errorf("cannot access member variable '%s': %w", member, err)
						}
					} else if propDecl, ownerDef := c.findPropertyOnFBChain(fbDef, member); propDecl != nil {
						// It's a property read (GET). Check getter permission.
						if propDecl.Getter != nil {
							accessSpecifier := propDecl.Getter.AccessSpecifier
							if accessSpecifier == "" {
								accessSpecifier = propDecl.AccessSpecifier
							}
							if err := c.checkAccessPermission(accessSpecifier, ownerDef); err != nil {
								return fmt.Errorf("cannot access getter for property '%s': %w", member, err)
							}
						} else {
							return fmt.Errorf("property '%s' is write-only", member)
						}
					} else if methodDef, ownerDef := c.findMethodOnFBChain(fbDef, member); methodDef != nil {
						// Check method access permission
						if err := c.checkAccessPermission(methodDef.AccessSpecifier, ownerDef); err != nil {
							return fmt.Errorf("cannot access method '%s': %w", member, err)
						}
					}
				}
			}
		}
		// --- End of new access control logic ---

		// Check if this is a property access (read).
		if isProperty, propName := c.isPropertyAccess(node); isProperty {
			// It's a property get. Compile as a call to the getter method.
			getterCall := &ast.CallExpression{
				Function: &ast.MemberAccessExpression{
					Struct: node.Struct,
					Member: &ast.Identifier{Value: "get_" + propName},
				},
				Arguments: []ast.Expression{}, // Getter has no arguments
			}
			return c.Compile(getterCall)
		}

		// Handle SUPER calls for METHODS (not properties, which are handled above).
		if deref, ok := node.Struct.(*ast.DereferenceExpression); ok {
			if _, ok := deref.Pointer.(*ast.SuperExpression); ok {
				thisSymbol, ok := c.symbolTable.Resolve("THIS")
				if !ok {
					return fmt.Errorf("cannot use SUPER outside of a function block context")
				}
				c.loadSymbol(thisSymbol)
				c.emitConstant(c.addConstant(&object.String{Value: node.Member.Value}))
				c.emit(code.OpSuperIndex)
				return nil
			}
		}

		// It's a regular field access.
		return c.compileMemberAccess(node)

	// A FunctionLiteral is compiled into a closure, similar to a named function.
	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Name.Value, false)
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

	// A MacroLiteral is treated as a data structure at compile time. It's not
	// executed but is stored as a constant to be expanded by the evaluator/VM later.
	case *ast.MacroLiteral:
		// The macro's body and parameters are stored in an UncompiledMacro
		// object. The compiler does not have a runtime environment to capture,
		// so the Env field is set to nil. The VM/evaluator will handle expansion.
		macro := &object.UncompiledMacro{
			Parameters: node.Parameters,
			Body:       node.Body,
		}
		c.emitConstant(c.addConstant(macro))

	// A ReturnStatement compiles the return value and emits OpReturnValue.
	case *ast.ReturnStatement:
		inFunction := len(c.functionResults) > 0 && c.functionResults[len(c.functionResults)-1].hasResult
		if node.ReturnValue == nil {
			// RETURN in a function returns its current result.
			c.emitFunctionReturn()
			return nil
		}
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		if inFunction {
			// `RETURN value` sets the result first, so VAR_OUTPUTs are still returned.
			if err := c.setSymbol(c.functionResults[len(c.functionResults)-1].symbol); err != nil {
				return err
			}
			c.emitFunctionReturn()
			return nil
		}
		c.emit(code.OpReturnValue)

	// A bit of an integer or bit string, e.g. flags.3.
	case *ast.BitAccessExpression:
		return c.compileBitAccess(node, nil)
	case *bitSetValue:
		return c.compileBitAccess(node.BitAccessExpression, node.value)

	// An assigned array or structure is copied; see copyAssignedValue.
	case *copiedValue:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		c.emit(code.OpCopy)

	// A CallExpression compiles the function/callable and all arguments, then
	// emits an OpCall instruction.
	case *ast.CallExpression:
		// Calling a function block instance runs its body; see compileFunctionBlockCall.
		if fbDef := c.calleeFunctionBlock(node.Function); fbDef != nil {
			return c.compileFunctionBlockCall(node, fbDef)
		}

		// Separate arguments into inputs (positional/named) and outputs (=>).
		inputArgs := []ast.Expression{}
		outputArgs := []*ast.OutputArgument{}

		for _, arg := range node.Arguments {
			if out, ok := arg.(*ast.OutputArgument); ok {
				outputArgs = append(outputArgs, out)
			} else {
				inputArgs = append(inputArgs, arg)
			}
		}

		// --- Part 1: Compile the function/callable object ---
		// This is a standard function call, recursive call, or method call.
		// We compile the function expression itself, which will leave a callable
		// object (Closure, Builtin) on the stack. The MemberAccessExpression
		// compiler case will handle resolving methods, including SUPER calls.
		isRecursive := false
		if ident, ok := node.Function.(*ast.Identifier); ok {
			if currentFn := c.currentFunction(); currentFn != nil && ident.Value == currentFn.Name.Value {
				isRecursive = true
			}
		}
		if isRecursive {
			c.emit(code.OpCurrentClosure)
		} else if err := c.Compile(node.Function); err != nil {
			return err
		}

		// Compile input arguments.
		for _, arg := range inputArgs {
			if named, ok := arg.(*ast.NamedArgument); ok {
				// Compile the value of the named argument.
				if err := c.Compile(named.Value); err != nil {
					return err
				}
				// Add the name as a constant and emit the new opcode.
				nameIndex := c.addConstant(&object.String{Value: named.Name.Value}) // cspell:disable-line
				c.emit(code.OpMakeNamedArg, nameIndex)
			} else {
				// It's a positional argument.
				if err := c.Compile(arg); err != nil {
					return err
				}
			}
		}

		// A user function with VAR_OUTPUTs returns a hash holding its result
		// ("__return__") and each output. Check the `=>` names against it.
		var calleeFn *ast.FunctionDeclaration
		if ident, ok := node.Function.(*ast.Identifier); ok {
			if def, found := c.resolveTypeName(ident.Value); found {
				calleeFn, _ = def.(*ast.FunctionDeclaration)
			}
		}
		if calleeFn != nil {
			for _, out := range outputArgs {
				declared := false
				for _, o := range calleeFn.VarOutputs {
					if strings.EqualFold(o.Name.Value, out.Source.Value) {
						declared = true
						break
					}
				}
				if !declared {
					return fmt.Errorf("function '%s' has no output '%s'", calleeFn.Name.Value, out.Source.Value)
				}
			}
			// A VAR_IN_OUT's argument is passed in by value and its final value
			// copied back from the result, as if it were an output `io => arg`.
			copyBacks, err := c.functionInOutCopyBacks(calleeFn, inputArgs, node)
			if err != nil {
				return err
			}
			outputArgs = append(outputArgs, copyBacks...)
		}

		// Inputs the call omits take their initial value or type default.
		numArgs := len(inputArgs)
		if inputs, known := c.calleeInputs(node.Function); known {
			// Named in-outs were checked above; only inputs take defaults.
			inputOnly := inputArgs
			if calleeFn != nil && len(calleeFn.VarInOuts) > 0 {
				inputOnly = []ast.Expression{}
				for _, arg := range inputArgs {
					if named, ok := arg.(*ast.NamedArgument); ok && findParameter(calleeFn.VarInOuts, named.Name.Value) != nil {
						continue
					}
					inputOnly = append(inputOnly, arg)
				}
			}
			added, err := c.emitOmittedInputs(node.Function.String(), inputs, inputOnly)
			if err != nil {
				return err
			}
			numArgs += added
		}

		c.emit(code.OpCall, numArgs)

		// --- Part 2: Compile output assignments ---
		// The result of the call (FB instance or return value hash) is now on the stack.
		for _, out := range outputArgs {
			// Keep the call's result for the next output and the return value.
			c.emit(code.OpDup)

			// Compile member access: <func_result>.<source_name>. Outputs are
			// keyed by their declared name, since identifiers are case-insensitive.
			outputName := out.Source.Value
			if calleeFn != nil {
				for _, o := range calleeFn.VarOutputs {
					if strings.EqualFold(o.Name.Value, outputName) {
						outputName = o.Name.Value
					}
				}
			}
			c.emitConstant(c.addConstant(&object.String{Value: outputName}))
			c.emit(code.OpIndex)

			// Compile assignment to the target. Any other target, such as an
			// array element or a structure member, is assigned from a hidden
			// temporary with an ordinary assignment.
			targetIdent, ok := out.Target.(*ast.Identifier)
			var symbol Symbol
			if ok {
				symbol, ok = c.symbolTable.Resolve(targetIdent.Value)
			}
			if !ok {
				temp, defined := c.symbolTable.Resolve(outputTemp)
				if !defined || (temp.Scope != LocalScope && c.symbolTable.Outer != nil) {
					temp = c.symbolTable.Define(outputTemp, false)
				}
				if err := c.setSymbol(temp); err != nil {
					return err
				}
				assign := &ast.AssignmentStatement{Token: node.Token, Left: out.Target, Value: &ast.Identifier{Token: node.Token, Value: outputTemp}}
				if err := c.Compile(assign); err != nil {
					return err
				}
				continue
			}
			err := c.setSymbol(symbol)
			if err != nil {
				return err
			}
		}

		// Leave the function's own result as the call's value.
		if calleeFn != nil && len(calleeFn.VarOutputs)+len(calleeFn.VarInOuts) > 0 {
			c.emitConstant(c.addConstant(&object.String{Value: "__return__"}))
			c.emit(code.OpIndex)
		}

	case *ast.IlInstructionStatement:
		return fmt.Errorf("IL instruction '%s' found in a Structured Text context", node.Operator)

	// A structure initializer such as `(PT := T#1s)` only has meaning as the
	// initial value of a structure or function block variable, where
	// compileStartingValue handles it.
	case *ast.StructLiteral:
		return fmt.Errorf("a structure initializer %s is only allowed as the initial value of a structure or function block variable", node.String())

	}

	return nil
}

// validateMethodOverride checks if a derived method has a signature compatible with its parent.
func (c *Compiler) validateMethodOverride(derived, parent *ast.MethodImplementation) error {
	// 1. Check return type
	derivedHasReturn := derived.ReturnType != nil
	parentHasReturn := parent.ReturnType != nil

	if derivedHasReturn != parentHasReturn {
		var derivedStr, parentStr string
		if derivedHasReturn {
			derivedStr = derived.ReturnType.String()
		} else {
			derivedStr = "no return type"
		}
		if parentHasReturn {
			parentStr = parent.ReturnType.String()
		} else {
			parentStr = "no return type"
		}
		return fmt.Errorf("return type mismatch for method '%s': derived has %s, parent has %s", derived.Name.Value, derivedStr, parentStr)
	}

	// If both have return types, check if they are compatible.
	if derivedHasReturn { // and parentHasReturn is implied
		derivedReturnStr := derived.ReturnType.String()
		parentReturnStr := parent.ReturnType.String()

		if !strings.EqualFold(derivedReturnStr, parentReturnStr) {
			// If types are different, check for covariance (only for function blocks).
			parentTypeNode, parentTypeFound := c.resolveTypeNode(parent.ReturnType)
			derivedTypeNode, derivedTypeFound := c.resolveTypeNode(derived.ReturnType)

			// If either type is not a defined type (i.e., it's a primitive like INT, BOOL), then they must be identical.
			// The initial `derivedReturn != parentReturn` check already covers this.
			if !parentTypeFound || !derivedTypeFound {
				return fmt.Errorf("return type mismatch for method '%s': derived is '%s', parent is '%s'", derived.Name.Value, derivedReturnStr, parentReturnStr)
			}

			parentFBDef, isParentFB := parentTypeNode.(*ast.FunctionBlockDeclaration)
			derivedFBDef, isDerivedFB := derivedTypeNode.(*ast.FunctionBlockDeclaration)

			// Covariance is only allowed if both return types are function blocks.
			if isParentFB && isDerivedFB {
				// Check if the derived return type is a subclass of the parent return type.
				if !c.isSubclassOf(derivedFBDef, parentFBDef) {
					return fmt.Errorf("incompatible return types for method '%s': '%s' is not a subclass of '%s'", derived.Name.Value, derivedReturnStr, parentReturnStr)
				}
				// If it is a subclass, this is a valid covariant return.
			} else {
				// If one or both are not function blocks (e.g., INTERFACE, STRUCT, or primitive), types must be identical.
				return fmt.Errorf("return type mismatch for method '%s': derived is '%s', parent is '%s'", derived.Name.Value, derivedReturnStr, parentReturnStr)
			}
		}
	}

	// 2. Check parameter count
	if len(derived.VarInputs) != len(parent.VarInputs) {
		return fmt.Errorf("parameter count mismatch for method '%s': derived has %d, parent has %d", derived.Name.Value, len(derived.VarInputs), len(parent.VarInputs))
	}

	// 3. Check parameter types
	for i, derivedParam := range derived.VarInputs {
		parentParam := parent.VarInputs[i]
		derivedParamTypeStr := derivedParam.DataType.String()
		parentParamTypeStr := parentParam.DataType.String()

		if !strings.EqualFold(derivedParamTypeStr, parentParamTypeStr) {
			// If types are different, check for contravariance (only for function blocks).
			parentTypeNode, parentTypeFound := c.resolveTypeNode(parentParam.DataType)
			derivedTypeNode, derivedTypeFound := c.resolveTypeNode(derivedParam.DataType)

			// If either type is not a defined type (i.e., it's a primitive), they must be identical.
			if !parentTypeFound || !derivedTypeFound {
				return fmt.Errorf("parameter type mismatch for method '%s' at index %d: derived is '%s', parent is '%s'", derived.Name.Value, i, derivedParamTypeStr, parentParamTypeStr)
			}

			parentFBDef, isParentFB := parentTypeNode.(*ast.FunctionBlockDeclaration)
			derivedFBDef, isDerivedFB := derivedTypeNode.(*ast.FunctionBlockDeclaration)

			// Contravariance is only allowed if both parameter types are function blocks.
			if isParentFB && isDerivedFB {
				// Check if the parent's parameter type is a subclass of the derived's parameter type.
				if !c.isSubclassOf(parentFBDef, derivedFBDef) {
					return fmt.Errorf("incompatible parameter type for method '%s' at index %d: '%s' is not a superclass of '%s'", derived.Name.Value, i, derivedParamTypeStr, parentParamTypeStr)
				}
				// If it is a superclass, this is a valid contravariant parameter.
			} else {
				// If one or both are not function blocks, types must be identical.
				return fmt.Errorf("parameter type mismatch for method '%s' at index %d: derived is '%s', parent is '%s'", derived.Name.Value, i, derivedParamTypeStr, parentParamTypeStr)
			}
		}
	}
	return nil
}

// findVarDeclOnFBChain recursively searches for a variable declaration starting from a given
// function block and traversing up its inheritance chain. It returns the declaration and its owner.
func (c *Compiler) findVarDeclOnFBChain(fbDef *ast.FunctionBlockDeclaration, varName string) (*ast.VarDeclStatement, *ast.FunctionBlockDeclaration) {
	if fbDef == nil {
		return nil, nil
	}

	// Search all var blocks in the current FB's definition.
	allVarBlocks := [][]*ast.VarDeclStatement{
		fbDef.VarInputs, fbDef.VarOutputs, fbDef.VarInOuts, fbDef.Vars,
	}
	for _, varBlock := range allVarBlocks {
		for _, decl := range varBlock {
			if strings.EqualFold(decl.Name.Value, varName) { // Case-insensitive comparison
				return decl, fbDef // Found it.
			}
		}
	}

	// Also search in the body, as the parser may place VAR blocks there,
	// and the parent FB's AST node is not pre-processed like the current one.
	if body, ok := fbDef.Body.(*ast.BlockStatement); ok {
		for _, stmt := range body.Statements {
			// Explicitly skip methods to avoid any ambiguity. This function
			// should only ever find variable declarations.
			if _, isMethod := stmt.(*ast.MethodImplementation); isMethod {
				continue
			}
			if varDecl, isVarDecl := stmt.(*ast.VarDeclStatement); isVarDecl {
				if strings.EqualFold(varDecl.Name.Value, varName) {
					return varDecl, fbDef
				}
			} else if varBlock, isVarBlock := stmt.(*ast.VarBlockDeclaration); isVarBlock {
				for _, decl := range varBlock.Declarations {
					if strings.EqualFold(decl.Name.Value, varName) {
						return decl, fbDef
					}
				}
			}
		}
	}

	// If not found, recurse to the parent.
	if fbDef.Extends != nil {
		parentObj, ok := c.resolveTypeNode(fbDef.Extends)
		if !ok {
			return nil, nil
		}
		if parentDef, ok := parentObj.(*ast.FunctionBlockDeclaration); ok {
			return c.findVarDeclOnFBChain(parentDef, varName)
		}
	}

	return nil, nil // Reached the top of the chain without finding the var.
}

// findMemberType searches within a STRUCT or FB definition for a member and returns its type.
func (c *Compiler) findMemberType(typeNode ast.Node, memberName string) (object.ObjectType, error) {
	switch def := typeNode.(type) {
	case *ast.FunctionBlockDeclaration:
		// It's a function block. Look for the member.
		// Check properties first.
		if prop, _ := c.findPropertyOnFBChain(def, memberName); prop != nil {
			return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(prop.DataType))), nil
		}
		// Check variables.
		if varDecl, _ := c.findVarDeclOnFBChain(def, memberName); varDecl != nil {
			return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(varDecl.DataType))), nil
		}
		// Check for methods. If found, it means user is trying to get value of method.
		if method, _ := c.findMethodOnFBChain(def, memberName); method != nil {
			return "", fmt.Errorf("cannot take value of method '%s'", memberName)
		}
		return "", fmt.Errorf("member '%s' not found on function block '%s'", memberName, def.Name.Value)

	case *ast.TypeDeclaration:
		if structDef, ok := def.DataType.(*ast.StructDefinition); ok {
			for _, m := range structDef.Members {
				if strings.EqualFold(m.Name.Value, memberName) {
					if _, isArray := m.DataType.(*ast.ArrayDefinition); isArray {
						return anyType, nil
					}
					return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(m.DataType))), nil
				}
			}
			return "", fmt.Errorf("member '%s' not found on structure '%s'", memberName, def.Name.Value)
		}
	}
	return "", fmt.Errorf("member access on non-composite type '%T'", typeNode)
}

// getExpressionType recursively determines the data type of an AST expression node.
func (c *Compiler) getExpressionType(expr ast.Expression) (object.ObjectType, error) {
	switch e := expr.(type) {
	case *copiedValue:
		return c.getExpressionType(e.Expression)
	case *ast.BitAccessExpression:
		return object.BOOLEAN_OBJ, nil
	case *bitSetValue:
		return c.getExpressionType(e.Target)
	case *ast.IntegerLiteral:
		return object.LINT_OBJ, nil // Default to largest integer type for literals
	case *ast.RealLiteral:
		return object.LREAL_OBJ, nil // Default to largest real type
	case *ast.Boolean:
		return object.BOOLEAN_OBJ, nil
	case *ast.StringLiteral:
		return object.STRING_OBJ, nil
	case *ast.WStringLiteral:
		return object.WSTRING_OBJ, nil
	case *ast.ThisExpression:
		if c.currentFB == nil {
			return "", fmt.Errorf("cannot use THIS outside of a function block context")
		}
		return object.ObjectType(c.currentFB.Name.Value), nil
	case *ast.SuperExpression:
		if c.currentFB == nil || c.currentFB.Extends == nil {
			return "", fmt.Errorf("SUPER used outside of a derived function block")
		}
		return object.ObjectType(c.flattenExpressionToString(c.currentFB.Extends)), nil
	case *ast.DereferenceExpression:
		return c.getExpressionType(e.Pointer)
	case *ast.Identifier:
		if c.currentFB != nil {
			// Check only instance variables (VAR), not I/O vars.
			for _, varDecl := range c.currentFB.Vars {
				if varDecl.Name.Value == e.Value {
					thisExpr := &ast.ThisExpression{Token: e.Token}
					memberAccess := &ast.MemberAccessExpression{Struct: thisExpr, Member: e}
					return c.getExpressionType(memberAccess)
				}
			}
			// Any other variable (e.g. an FB input read inside a method) or a
			// property of the enclosing FB, read by name.
			if symbol, isSymbol := c.symbolTable.Resolve(e.Value); !isSymbol || c.hidesSymbol(symbol, e.Value) {
				if varDecl, _ := c.findVarDeclOnFBChain(c.currentFB, e.Value); varDecl != nil && varDecl.DataType != nil {
					return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(varDecl.DataType))), nil
				}
				if prop, _ := c.findPropertyOnFBChain(c.currentFB, e.Value); prop != nil {
					return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(prop.DataType))), nil
				}
			}
		}
		symbol, ok := c.symbolTable.Resolve(e.Value)
		if !ok {
			return "", fmt.Errorf("undefined identifier: %s", e.Value)
		}
		if symbol.TypeName == "" {
			// This is a fallback for untyped identifiers, which can occur with
			// non-standard language extensions like 'fn' literals where parameter
			// types are not captured in the AST. Assume LINT to allow arithmetic.
			return object.LINT_OBJ, nil
		}
		return object.ObjectType(strings.ToUpper(symbol.TypeName)), nil
	case *ast.MemberAccessExpression:
		structType, err := c.getExpressionType(e.Struct)
		if err != nil {
			return "", err
		}
		if structType == anyType {
			return anyType, nil // Its members are checked when it runs.
		}
		typeNode, ok := c.resolveTypeNode(&ast.Identifier{Value: string(structType)})
		if !ok {
			return "", fmt.Errorf("type definition not found for '%s'", structType)
		}
		return c.findMemberType(typeNode, e.Member.Value)
	case *ast.InfixExpression:
		leftType, err := c.getExpressionType(e.Left)
		if err != nil {
			return "", err
		}
		rightType, err := c.getExpressionType(e.Right)
		if err != nil {
			return "", err
		}
		leftType, rightType = literalOperandTypes(e, leftType, rightType)
		return c.getResultingType(e.Operator, leftType, rightType)
	case *ast.PrefixExpression:
		// For prefix expressions, the type is usually the same as the operand's type.
		return c.getExpressionType(e.Right)
	case *ast.CallExpression:
		// This requires looking up the function's return type.
		if ident, ok := e.Function.(*ast.Identifier); ok {
			if funcDefNode, ok := c.resolveTypeName(ident.Value); ok {
				if funcDef, isFunc := funcDefNode.(*ast.FunctionDeclaration); isFunc {
					if funcDef.ReturnType != nil {
						return object.ObjectType(strings.ToUpper(funcDef.ReturnType.String())), nil
					}
					return object.NULL_OBJ, nil
				}
			}
		}
		if memberAccess, ok := e.Function.(*ast.MemberAccessExpression); ok {
			baseType, err := c.getExpressionType(memberAccess.Struct)
			if err != nil {
				return "", err
			}
			typeNode, ok := c.resolveTypeNode(&ast.Identifier{Value: string(baseType)})
			if !ok {
				return "", fmt.Errorf("type definition not found for '%s'", baseType)
			}
			if fbDef, isFB := typeNode.(*ast.FunctionBlockDeclaration); isFB {
				methodName := memberAccess.Member.Value
				if method, _ := c.findMethodOnFBChain(fbDef, methodName); method != nil {
					if method.ReturnType != nil {
						return object.ObjectType(strings.ToUpper(c.flattenExpressionToString(method.ReturnType))), nil
					}
					return object.NULL_OBJ, nil // Method with no return value
				}
				return "", fmt.Errorf("method '%s' not found on type '%s'", methodName, baseType)
			}
		}
		if ident, ok := e.Function.(*ast.Identifier); ok {
			if t, ok := c.builtinResultType(ident.Value, e.Arguments); ok {
				return t, nil
			}
		}
		// Other built-in functions have no declared return type here; their
		// result is checked by the VM when the program runs.
		return anyType, nil
	case *ast.UnsignedIntegerLiteral:
		return object.ULINT_OBJ, nil
	case *ast.LRealLiteral:
		return object.LREAL_OBJ, nil
	case *ast.BitStringLiteral:
		return bitStringTypeForWidth(e.Width), nil
	case *ast.TimeLiteral:
		return object.TIME_OBJ, nil
	case *ast.DateLiteral:
		return object.DATE_OBJ, nil
	case *ast.TimeOfDayLiteral:
		return object.TIME_OF_DAY_OBJ, nil
	case *ast.DateAndTimeLiteral:
		return object.DATE_AND_TIME_OBJ, nil
	case *ast.EnumeratedValueLiteral:
		return object.ObjectType(strings.ToUpper(e.TypeName.Value)), nil
	case *ast.TypedLiteral:
		return typedLiteralType(e.TypeName), nil
	case *ast.IndexExpression:
		return c.indexElementType(e), nil
	default:
		return "", fmt.Errorf("cannot determine type of expression: %T", expr)
	}
}

// anyType is the type of an operand the compiler cannot determine statically,
// such as a built-in function's result. Operations on it are not rejected at
// compile time; the VM checks the actual values when the program runs.
const anyType = object.ObjectType("ANY")

// typedLiteralType returns the type of a typed literal such as INT#5, T#1s or
// Color#Red, normalizing the short forms of the time and date types.
func typedLiteralType(typeName string) object.ObjectType {
	switch upper := strings.ToUpper(typeName); upper {
	case "T":
		return object.TIME_OBJ
	case "D":
		return object.DATE_OBJ
	case "TOD":
		return object.TIME_OF_DAY_OBJ
	case "DT":
		return object.DATE_AND_TIME_OBJ
	default:
		return object.ObjectType(upper)
	}
}

// bitStringTypeForWidth returns the bit-string type with the given width.
func bitStringTypeForWidth(width int) object.ObjectType {
	switch width {
	case 8:
		return object.BYTE_OBJ
	case 16:
		return object.WORD_OBJ
	case 32:
		return object.DWORD_OBJ
	default:
		return object.LWORD_OBJ
	}
}

// indexElementType returns the element type of an indexed array variable, as
// declared, or anyType when it cannot be determined statically.
func (c *Compiler) indexElementType(e *ast.IndexExpression) object.ObjectType {
	// The array may be a local, a function block's variable, a member, or
	// of a named array type.
	def := c.arrayTypeOf(c.declaredVarType(e.Left))
	if def == nil || def.DataType == nil {
		return anyType
	}
	if elementType := c.flattenExpressionToString(def.DataType); elementType != "" {
		return object.ObjectType(elementType)
	}
	return anyType
}

// isTemporalType reports whether t is a time or date type.
func isTemporalType(t object.ObjectType) bool {
	switch t {
	case object.TIME_OBJ, object.DATE_OBJ, object.TIME_OF_DAY_OBJ, object.DATE_AND_TIME_OBJ:
		return true
	}
	return false
}

// temporalResultType returns the type of an arithmetic operation involving a
// time or date operand, following IEC 61131-3, and false if it is not defined.
func temporalResultType(op string, left, right object.ObjectType) (object.ObjectType, bool) {
	isNumeric := func(t object.ObjectType) bool {
		return object.IsIntegerType(string(t)) || object.IsRealType(string(t))
	}
	switch {
	case left == object.TIME_OBJ && right == object.TIME_OBJ && (op == "+" || op == "-"):
		return object.TIME_OBJ, true
	case left == object.TIME_OBJ && isNumeric(right) && (op == "*" || op == "/"):
		return object.TIME_OBJ, true
	case isNumeric(left) && right == object.TIME_OBJ && op == "*":
		return object.TIME_OBJ, true
	case left == object.DATE_OBJ && right == object.DATE_OBJ && op == "-":
		return object.TIME_OBJ, true
	case (left == object.TIME_OF_DAY_OBJ || left == object.DATE_AND_TIME_OBJ) && right == object.TIME_OBJ && (op == "+" || op == "-"):
		return left, true
	case (left == object.TIME_OF_DAY_OBJ || left == object.DATE_AND_TIME_OBJ) && right == left && op == "-":
		return object.TIME_OBJ, true
	}
	return "", false
}

// isLogicalOperator reports whether op is one of the IEC 61131-3 logical/bitwise operators.
func isLogicalOperator(op string) bool {
	switch strings.ToUpper(op) {
	case "AND", "OR", "XOR", "NAND", "NOR", "&":
		return true
	}
	return false
}

// literalAsBitString returns the type an operand should have in a logical
// operation. An untyped integer literal adopts the other operand's bit-string
// type if it has one, and LWORD otherwise. Any other operand keeps its type.
func literalAsBitString(operand ast.Expression, operandType, otherType object.ObjectType) object.ObjectType {
	if _, isLiteral := operand.(*ast.IntegerLiteral); !isLiteral {
		return operandType
	}
	if object.IsBitStringType(string(otherType)) && otherType != object.BOOLEAN_OBJ {
		return otherType
	}
	return object.LWORD_OBJ
}

// literalOperandTypes returns the types an infix expression's operands have
// once an untyped integer literal takes its type from context. In a logical
// operation it is a bit string (e.g. `10 AND 12` or `myWord AND 16#FF`), and
// compared with a bit string it has the bit string's type (`myByte = 0`).
// Typed integer variables are still rejected.
func literalOperandTypes(e *ast.InfixExpression, left, right object.ObjectType) (object.ObjectType, object.ObjectType) {
	left, right = elementaryTypeName(left), elementaryTypeName(right)
	switch {
	case isLogicalOperator(e.Operator):
		return literalAsBitString(e.Left, left, right), literalAsBitString(e.Right, right, left)
	case object.IsComparisonOperator(e.Operator):
		if _, ok := e.Left.(*ast.IntegerLiteral); ok && object.IsBitStringType(string(right)) {
			left = right
		}
		if _, ok := e.Right.(*ast.IntegerLiteral); ok && object.IsBitStringType(string(left)) {
			right = left
		}
	}
	return left, right
}

// elementaryTypeName returns the name the type checker uses for an
// elementary type written with another of its names, such as BOOL or TOD.
func elementaryTypeName(t object.ObjectType) object.ObjectType {
	switch strings.ToUpper(string(t)) {
	case "BOOL":
		return object.BOOLEAN_OBJ
	case "TOD", "LTOD", "LTIME_OF_DAY":
		return object.TIME_OF_DAY_OBJ
	case "DT", "LDT", "LDATE_AND_TIME":
		return object.DATE_AND_TIME_OBJ
	case "LTIME":
		return object.TIME_OBJ
	case "LDATE":
		return object.DATE_OBJ
	}
	return t
}

// bitStringWidth returns a bit-string type's width, or 0.
func bitStringWidth(t object.ObjectType) int {
	w, _ := object.GetBitStringWidth(string(t))
	return w
}

// getResultingType checks if an operator is valid for the given operand types
// and returns the resulting type according to IEC 61131-3 type promotion rules.
func (c *Compiler) getResultingType(op string, left, right object.ObjectType) (object.ObjectType, error) {
	// Helper to check if a type is numeric
	isNumeric := func(t object.ObjectType) bool {
		return object.IsIntegerType(string(t)) || object.IsRealType(string(t))
	}

	left, right = elementaryTypeName(left), elementaryTypeName(right)
	// An operand whose type is only known at runtime is not rejected here.
	if left == anyType || right == anyType {
		switch op {
		case ">", "<", ">=", "<=", "=", "<>", "==", "!=":
			return object.BOOLEAN_OBJ, nil
		}
		if left == anyType {
			return right, nil
		}
		return left, nil
	}

	switch op {
	case "+", "-", "*", "/", "**":
		if resultType, ok := temporalResultType(op, left, right); ok {
			return resultType, nil
		}
		if isNumeric(left) && isNumeric(right) {
			// Simplified type promotion: if either is REAL, the result is REAL.
			if object.IsRealType(string(left)) || object.IsRealType(string(right)) {
				return object.LREAL_OBJ, nil
			}
			// Otherwise, the result is the largest integer type.
			return object.LINT_OBJ, nil
		}
		if (left == object.STRING_OBJ || left == object.WSTRING_OBJ) && (right == object.STRING_OBJ || right == object.WSTRING_OBJ) && op == "+" {
			if left == object.WSTRING_OBJ || right == object.WSTRING_OBJ {
				return object.WSTRING_OBJ, nil
			}
			return object.STRING_OBJ, nil
		}
		return "", fmt.Errorf("operator '%s' not defined for types %s and %s", op, left, right)

	case "MOD":
		if object.IsIntegerType(string(left)) && object.IsIntegerType(string(right)) {
			return object.LINT_OBJ, nil
		}
		return "", fmt.Errorf("operator '%s' not defined for types %s and %s", op, left, right)

	case ">", "<", ">=", "<=", "=", "<>", "==", "!=":
		// Comparisons are generally valid between any two numeric types.
		if isNumeric(left) && isNumeric(right) {
			return object.BOOLEAN_OBJ, nil
		}
		// Also allow string comparison
		if (left == object.STRING_OBJ || left == object.WSTRING_OBJ) && (right == object.STRING_OBJ || right == object.WSTRING_OBJ) {
			return object.BOOLEAN_OBJ, nil
		}
		// Also allow boolean comparison
		if left == object.BOOLEAN_OBJ && right == object.BOOLEAN_OBJ {
			return object.BOOLEAN_OBJ, nil
		}
		// Times and dates compare with values of the same type, and bit
		// strings with bit strings, the narrower widened to the wider.
		if left == right && isTemporalType(left) {
			return object.BOOLEAN_OBJ, nil
		}
		if object.IsBitStringType(string(left)) && object.IsBitStringType(string(right)) {
			return object.BOOLEAN_OBJ, nil
		}
		// Any two values of the same type (e.g. an enumeration) can be tested for equality.
		if left == right && (op == "=" || op == "<>" || op == "==" || op == "!=") {
			return object.BOOLEAN_OBJ, nil
		}
		return "", fmt.Errorf("comparison operator '%s' not defined for types %s and %s", op, left, right)

	case "AND", "OR", "XOR", "NAND", "NOR":
		if left == object.BOOLEAN_OBJ && right == object.BOOLEAN_OBJ {
			return left, nil
		}
		if object.IsBitStringType(string(left)) && object.IsBitStringType(string(right)) {
			// The narrower bit string is widened to the wider.
			if bitStringWidth(right) > bitStringWidth(left) {
				return right, nil
			}
			return left, nil
		}
		return "", fmt.Errorf("logical operator '%s' not defined for types %s and %s", op, left, right)
	}

	return "", fmt.Errorf("unknown operator '%s'", op)
}

// checkAbstractImplementation verifies that a concrete function block provides
// implementations for all abstract methods and properties from its inheritance chain.
func (c *Compiler) checkAbstractImplementation(fbDef *ast.FunctionBlockDeclaration) error {
	if fbDef.IsAbstract {
		return nil // Abstract classes don't need to implement anything.
	}

	// 1. Gather all abstract methods and properties from all ancestors.
	abstractMethodsToImplement := make(map[string]*ast.MethodImplementation)
	abstractPropertiesToImplement := make(map[string]*ast.PropertyDeclaration)

	var collectAbstracts func(currentFB *ast.FunctionBlockDeclaration)
	collectAbstracts = func(currentFB *ast.FunctionBlockDeclaration) {
		if currentFB == nil {
			return
		}
		// Recurse to parent first to gather from the top down.
		if currentFB.Extends != nil {
			parentObj, ok := c.resolveTypeNode(currentFB.Extends)
			if !ok {
				return
			}
			if parentDef, ok := parentObj.(*ast.FunctionBlockDeclaration); ok {
				collectAbstracts(parentDef)
			}
		}

		// Add abstract members from the current FB to the list of requirements.
		if body, ok := currentFB.Body.(*ast.BlockStatement); ok {
			for _, stmt := range body.Statements {
				if method, isMethod := stmt.(*ast.MethodImplementation); isMethod && method.IsAbstract {
					abstractMethodsToImplement[method.Name.Value] = method
				}
			}
		}
		for _, prop := range currentFB.Properties {
			if prop.IsAbstract {
				abstractPropertiesToImplement[prop.Name.Value] = prop
			}
		}
	}

	// Start collecting abstract members from the parent chain.
	if fbDef.Extends != nil {
		parentObj, ok := c.resolveTypeNode(fbDef.Extends)
		if !ok {
			return nil // Or an error
		}
		if parentDef, ok := parentObj.(*ast.FunctionBlockDeclaration); ok {
			collectAbstracts(parentDef)
		}
	}

	// 2. Check if the current concrete class provides non-abstract implementations for them.
	for methodName := range abstractMethodsToImplement {
		foundMethod, _ := c.findMethodOnFBChain(fbDef, methodName)
		if foundMethod == nil || foundMethod.IsAbstract {
			return fmt.Errorf("function block '%s' must implement abstract method '%s'", fbDef.Name.Value, methodName)
		}
	}

	for propName := range abstractPropertiesToImplement {
		foundProp, _ := c.findPropertyOnFBChain(fbDef, propName)
		if foundProp == nil || foundProp.IsAbstract {
			return fmt.Errorf("function block '%s' must implement abstract property '%s'", fbDef.Name.Value, propName)
		}
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
	// Check if an equal constant already exists to avoid duplicates.
	// This is important for efficiency and for stable test results.
	for i, constant := range c.constants {
		// Perform a deep comparison for CompiledFunction objects. This is necessary
		// because the generic object.IsEqual may not handle this type, leading to
		// duplicate constants for identical functions (e.g., empty FB main bodies).
		// This ensures that identical functions are stored only once in the constant pool.
		if fn1, ok1 := constant.(*object.CompiledFunction); ok1 {
			if fn2, ok2 := obj.(*object.CompiledFunction); ok2 {
				// Parameter and output names matter too: named arguments and
				// VAR_OUTPUT results are resolved through them.
				if fn1.NumLocals == fn2.NumLocals &&
					fn1.NumParameters == fn2.NumParameters &&
					bytes.Equal(fn1.Instructions, fn2.Instructions) &&
					equalStrings(fn1.ParameterNames, fn2.ParameterNames) &&
					equalStrings(fn1.OutputNames, fn2.OutputNames) &&
					equalInts(fn1.OutputIndices, fn2.OutputIndices) {
					return i
				}
			}
			// If the existing constant is a function, but the new object is not
			// (or they are different functions), they cannot be equal. We skip
			// the generic `object.IsEqual` check below and move to the next
			// constant in the pool. This prevents incorrect fall-through.
			continue
		}

		// Only constants of the same type are shared: object.IsEqual compares
		// numbers by value, so it would merge LREAL 0.0 into LINT 0.
		if constant.Type() == obj.Type() && object.IsEqual(constant, obj) {
			return i
		}
	}
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// equalStrings reports whether two string slices are identical.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// equalInts reports whether two int slices are identical.
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// patchLoopBreaks pops the current loop context and patches all accumulated
// EXIT jumps to point to the instruction immediately following the loop.
func (c *Compiler) patchLoopBreaks(afterLoopPos int) {
	loop := c.leaveLoop()
	if loop != nil {
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, afterLoopPos)
		}
	}
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
		varDecls:            make(map[string]*ast.VarDeclStatement),
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
	if s.IsConstant {
		return fmt.Errorf("cannot assign to a constant variable '%s'", s.Name)
	}
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
	c.emitConstant(addStringConst("initial_step"))
	c.emitConstant(addStringConst(initialStepName))

	// Key: "actions"
	c.emitConstant(addStringConst("actions"))
	for name, fnIndex := range actionClosures {
		c.emitConstant(addStringConst(name))
		c.emit(code.OpClosure, fnIndex, 0)
	}
	c.emit(code.OpHash, len(actionClosures)*2)

	// Key: "transitions"
	c.emitConstant(addStringConst("transitions"))
	numTransitions := 0
	for _, stmt := range node.Elements {
		if trans, ok := stmt.(*ast.TransitionStatement); ok {
			numTransitions++
			fnIndex := transitionClosures[trans]

			c.emitConstant(addStringConst("condition"))
			c.emit(code.OpClosure, fnIndex, 0)
			c.emitConstant(addStringConst("from"))
			for _, from := range trans.From {
				c.emitConstant(addStringConst(from.Value))
			}
			c.emit(code.OpArray, len(trans.From))
			c.emitConstant(addStringConst("to"))
			for _, to := range trans.To {
				c.emitConstant(addStringConst(to.Value))
			}
			c.emit(code.OpArray, len(trans.To))
			c.emit(code.OpHash, 6)
		}
	}
	c.emit(code.OpArray, numTransitions)

	// Key: "steps"
	c.emitConstant(addStringConst("steps"))
	numSteps := 0
	for _, stmt := range node.Elements {
		if step, ok := stmt.(*ast.StepStatement); ok {
			numSteps++
			c.emitConstant(addStringConst(step.Name.Value))
			c.emitConstant(addStringConst("actions"))
			for _, actionAssoc := range step.Actions {
				c.emitConstant(addStringConst("name"))
				c.emitConstant(addStringConst(actionAssoc.ActionName.Value))

				qualifier := "N"
				if actionAssoc.Qualifier != nil {
					qualifier = actionAssoc.Qualifier.Value
				}
				c.emitConstant(addStringConst("qualifier"))
				c.emitConstant(addStringConst(qualifier))
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
	symbol := c.symbolTable.Define(config.Name.Value, false)

	// Compile any global vars defined directly in the configuration.
	for _, gv := range config.GlobalVars {
		if err := c.Compile(gv); err != nil {
			return err
		}
	}

	// Build the final configuration hash object.
	// Key: "name"
	c.emitConstant(c.addConstant(&object.String{Value: "name"}))
	c.emitConstant(c.addConstant(&object.String{Value: config.Name.Value}))

	// Key: "resources"
	c.emitConstant(c.addConstant(&object.String{Value: "resources"}))
	// Match each VAR_CONFIG entry to the program instance it configures.
	configEntries, unmatched := config.ResolveConfigVars()
	if len(unmatched) > 0 {
		return fmt.Errorf("VAR_CONFIG path '%s' does not name a variable of a program instance in configuration '%s'", unmatched[0].AccessPath.String(), config.Name.Value)
	}

	// Compile each resource, leaving a resource hash object on the stack.
	for _, res := range config.Resources {
		if err := c.compileResource(res, configEntries); err != nil {
			return err
		}
	}
	// Create an array of resource hashes, which becomes the value for the "resources" key.
	c.emit(code.OpArray, len(config.Resources))

	c.emit(code.OpHash, 2*2) // 2 key-value pairs

	// Store the final configuration hash in its global variable.
	return c.setSymbol(symbol)
}

// compileResource compiles a RESOURCE block into a hash object containing its
// tasks and program instances.
func (c *Compiler) compileResource(res *ast.ResourceDeclaration, configEntries []*ast.ConfigVarEntry) error {
	// Build the resource hash by compiling its key-value pairs in order.
	// Key: "name"
	c.emitConstant(c.addConstant(&object.String{Value: "name"}))
	c.emitConstant(c.addConstant(&object.String{Value: res.Name.Value}))

	// Key: "type"
	c.emitConstant(c.addConstant(&object.String{Value: "type"}))
	// The implicit resource of the single-resource form has no type.
	resourceType := ""
	if res.ResourceType != nil {
		resourceType = res.ResourceType.Value
	}
	c.emitConstant(c.addConstant(&object.String{Value: resourceType}))

	// Key: "tasks"
	c.emitConstant(c.addConstant(&object.String{Value: "tasks"}))
	// Value: Compile tasks and create an array of task hashes.
	for _, task := range res.Tasks {
		if err := c.compileTask(task); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(res.Tasks))

	// Key: "programs"
	c.emitConstant(c.addConstant(&object.String{Value: "programs"}))
	// Value: Compile program instances and create an array of program hashes.
	for _, prog := range res.Programs {
		var progEntries []*ast.ConfigVarEntry
		for _, entry := range configEntries {
			if entry.Resource == res && entry.Program == prog {
				progEntries = append(progEntries, entry)
			}
		}
		if err := c.compileProgramConfig(prog, progEntries); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(res.Programs))

	c.emit(code.OpHash, 4*2) // 4 key-value pairs
	return nil
}

// compileTask compiles a TASK declaration into a hash object.
func (c *Compiler) compileTask(task *ast.TaskDeclaration) error { // cspell:disable-line
	// Build the task hash object.
	c.emitConstant(c.addConstant(&object.String{Value: "name"}))
	c.emitConstant(c.addConstant(&object.String{Value: task.Name.Value}))

	c.emitConstant(c.addConstant(&object.String{Value: "interval"}))
	if err := c.Compile(task.Interval); err != nil {
		return err
	}
	if task.Interval == nil {
		c.emit(code.OpNull)
	}

	c.emitConstant(c.addConstant(&object.String{Value: "priority"}))
	if err := c.Compile(task.Priority); err != nil {
		return err
	}
	if task.Priority == nil {
		c.emit(code.OpNull)
	}

	c.emit(code.OpHash, 3*2) // 3 key-value pairs
	return nil
}

// compileProgramConfig compiles a PROGRAM configuration instance into a hash
// object. Its "params" hash maps each VAR_CONFIG path, relative to the
// instance, to the configured initial value. Entries that only assign a
// location (`AT %...`) have no value and are not included.
func (c *Compiler) compileProgramConfig(prog *ast.ProgramConfiguration, configEntries []*ast.ConfigVarEntry) error {
	// Build the program configuration hash object.
	c.emitConstant(c.addConstant(&object.String{Value: "instance"}))
	c.emitConstant(c.addConstant(&object.String{Value: prog.InstanceName.Value}))

	c.emitConstant(c.addConstant(&object.String{Value: "task"}))
	taskName := ""
	if prog.TaskName != nil {
		taskName = prog.TaskName.Value
	}
	c.emitConstant(c.addConstant(&object.String{Value: taskName}))

	c.emitConstant(c.addConstant(&object.String{Value: "type"}))
	c.emitConstant(c.addConstant(&object.String{Value: prog.TypeName.Value}))

	// Add the parameters from VAR_CONFIG as a nested hash.
	c.emitConstant(c.addConstant(&object.String{Value: "params"}))
	numParams := 0
	for _, entry := range configEntries {
		if entry.Decl.Value == nil {
			continue // Location-only entry; see the doc comment above.
		}
		c.emitConstant(c.addConstant(&object.String{Value: entry.RelativePath()}))
		if err := c.Compile(entry.Decl.Value); err != nil {
			return err
		}
		numParams++
	}
	c.emit(code.OpHash, numParams*2)

	c.emit(code.OpHash, 4*2) // 4 key-value pairs
	return nil
}

// compileTypedLiteral compiles a typed literal (e.g., `INT#10`, `T#5s`) by
// parsing its value and creating the corresponding object.Object.
func (c *Compiler) parseTypedLiteralValue(node *ast.TypedLiteral) (object.Object, error) {
	typeName := strings.ToUpper(node.TypeName)
	valueStr := node.Value.String() // This is an ast.Identifier with the value part

	switch typeName {
	case "TIME", "T":
		// An IEC duration may have days and underscores, as in 1d_12h.
		d, err := object.ParseDuration(valueStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TIME literal '%s': %w", valueStr, err)
		}
		return &object.Time{Value: d}, nil
	case "DATE", "D":
		t, err := time.Parse("2006-01-02", valueStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DATE literal '%s': %w", valueStr, err)
		}
		return &object.Date{Value: t}, nil
	case "TIME_OF_DAY", "TOD":
		t, err := time.Parse("15:04:05.999", valueStr)
		if err != nil {
			// try without milliseconds
			t, err = time.Parse("15:04:05", valueStr)
			if err != nil {
				return nil, fmt.Errorf("invalid TIME_OF_DAY literal '%s': %w", valueStr, err)
			}
		}
		return &object.TimeOfDay{Value: t}, nil
	case "DATE_AND_TIME", "DT":
		t, err := time.Parse("2006-01-02-15:04:05.999", valueStr)
		if err != nil {
			// try without milliseconds
			t, err = time.Parse("2006-01-02-15:04:05", valueStr)
			if err != nil {
				return nil, fmt.Errorf("invalid DATE_AND_TIME literal '%s': %w", valueStr, err)
			}
		}
		return &object.DateAndTime{Value: t}, nil

	// Integer types
	case "SINT", "INT", "DINT", "LINT":
		val, err := c.parseBasedInteger(valueStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s literal '%s': %w", typeName, valueStr, err)
		}
		// The VM uses LINT for all integer operations for simplicity.
		return &object.LInt{Value: val}, nil
	case "USINT", "UINT", "UDINT", "ULINT":
		val, err := c.parseBasedUnsignedInteger(valueStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s literal '%s': %w", typeName, valueStr, err)
		}
		return &object.ULInt{Value: val}, nil

	// Real types
	case "REAL", "LREAL":
		val, err := strconv.ParseFloat(strings.ReplaceAll(valueStr, "_", ""), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid REAL/LREAL literal '%s': %w", valueStr, err)
		}
		return &object.LReal{Value: val}, nil

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
			return nil, fmt.Errorf("invalid %s literal '%s': %w", typeName, valueStr, err)
		}
		if width < 64 && val>>uint(width) != 0 {
			return nil, fmt.Errorf("%s literal '%s' does not fit in %d bits", typeName, valueStr, width)
		}
		return &object.BitString{Value: val, Width: width}, nil

	default:
		// Fallback for enum types like `COLOR#RED` or other user-defined types.
		return &object.EnumeratedValue{TypeName: node.TypeName, Value: valueStr}, nil
	}
}

// compileTypedLiteral compiles a typed literal (e.g., `INT#10`, `T#5s`) by
// parsing its value and creating the corresponding object.Object.
func (c *Compiler) compileTypedLiteral(node *ast.TypedLiteral) error {
	obj, err := c.parseTypedLiteralValue(node)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("internal compiler error: parseTypedLiteralValue returned nil object without error for %s", node.String())
	}

	c.emitConstant(c.addConstant(obj))
	return nil
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

	if count == 0 {
		return 0, nil
	}

	// Compile the elements once to generate the instructions for one repetition.
	startPos := len(c.currentInstructions())
	for _, el := range ar.Elements {
		if err := c.Compile(el); err != nil {
			return 0, err
		}
	}
	endPos := len(c.currentInstructions())
	oneRepetitionInstructions := c.currentInstructions()[startPos:endPos]

	// Now, append these instructions `count - 1` more times.
	for i := 0; i < count-1; i++ {
		c.addInstruction(oneRepetitionInstructions)
	}

	return totalElements, nil
}

// emitConstant emits an OpConstant instruction for a given constant pool index.
func (c *Compiler) emitConstant(index int) {
	c.emit(code.OpConstant, index)
}

// Bytecode holds the compiled instructions and the constant pool for a program or function.
type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

// CompilationScope represents a single level of scope during compilation,
// holding the instructions and instruction history for that scope.
type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
	varDecls            map[string]*ast.VarDeclStatement // Maps var name to its declaration AST node
}

// evaluateConstantInteger attempts to evaluate an expression at compile time.
// For now, it only supports simple integer literals.
func (c *Compiler) evaluateConstantInteger(expr ast.Expression) (int64, error) {
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return e.Value, nil
	case *ast.InfixExpression:
		left, err := c.evaluateConstantInteger(e.Left)
		if err != nil {
			return 0, err
		}
		right, err := c.evaluateConstantInteger(e.Right)
		if err != nil {
			return 0, err
		}
		switch e.Operator {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero in constant expression")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator in constant integer expression: %s", e.Operator)
		}
	case *ast.PrefixExpression:
		if e.Operator == "-" {
			right, err := c.evaluateConstantInteger(e.Right)
			if err != nil {
				return 0, err
			}
			return -right, nil
		}
		return 0, fmt.Errorf("unsupported prefix operator in constant integer expression: %s", e.Operator)
	}
	return 0, fmt.Errorf("expression is not a constant integer")
}

// checkArrayBounds performs a compile-time check to see if an array access
// with a constant integer index is within the declared bounds of the array.
func (c *Compiler) checkArrayBounds(arrayExpr, indexExpr ast.Expression) error {
	// 1. We can only check if the index is a compile-time constant integer.
	indexValue, err := c.evaluateConstantInteger(indexExpr)
	if err != nil {
		// If evaluation fails because it's not a constant, we can't check at compile time, which is fine.
		// However, if evaluation fails due to a semantic error like division by zero, we must report it.
		if err.Error() == "division by zero in constant expression" {
			return err
		}
		return nil // For other errors (e.g., not a constant), we can't check, so we proceed.
	}

	// 2. Find the declared type of the array: a variable in scope, a field of
	// the function block (or program) being compiled, or a member.
	arrayDef := c.declaredArrayType(arrayExpr)
	if arrayDef == nil {
		return nil // Not a declared array (e.g., func()[i]), cannot check.
	}

	// 5. For now, assume a 1D array. Get the bounds expression.
	if len(arrayDef.Ranges) == 0 {
		return nil // No ranges defined.
	}
	rangeExpr, ok := arrayDef.Ranges[0].(*ast.InfixExpression)
	if !ok || rangeExpr.Operator != ".." {
		return nil // Not a valid range expression.
	}

	// 6. Evaluate the bounds at compile time.
	lowerBound, err := c.evaluateConstantInteger(rangeExpr.Left)
	if err != nil {
		return nil // Lower bound is not a constant, cannot check.
	}
	upperBound, err := c.evaluateConstantInteger(rangeExpr.Right)
	if err != nil {
		return nil // Upper bound is not a constant, cannot check.
	}

	// 7. Perform the check.
	if indexValue < lowerBound || indexValue > upperBound {
		return fmt.Errorf("array index out of bounds: index is %d but bounds are %d..%d", indexValue, lowerBound, upperBound)
	}

	return nil
}

// EmittedInstruction represents an instruction that has been emitted by the
// compiler, tracking its opcode and position in the bytecode stream.
type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

// compileMethod compiles a method implementation into a CompiledFunction.
// It's a helper for compiling FUNCTION_BLOCKs.
func (c *Compiler) compileMethod(method *ast.MethodImplementation) (*object.CompiledFunction, error) {
	c.enterScope()
	// A method is a function within the context of a function block.
	// We need to push a function context so that return value assignments
	// (e.g., `MyMethod := ...`) are compiled correctly as return statements.
	// We create a temporary FunctionDeclaration for this context.
	c.pushFunction(&ast.FunctionDeclaration{Name: method.Name})
	defer c.popFunction()

	// Define params, return var, and local vars.
	// The order is important: THIS, then input params, then the return var, then other locals.
	c.symbolTable.Define("THIS", false) // 'THIS' is implicitly local 0
	for _, p := range method.VarInputs {
		typeName := c.flattenExpressionToString(p.DataType)
		c.symbolTable.DefineVarInput(p.Name.Value, typeName)
	}
	if err := c.prepareParameters(method.VarInputs); err != nil {
		return nil, err
	}

	// Define and initialize the implicit return variable.
	returnSymbol := c.symbolTable.Define(method.Name.Value, false)
	// The result starts at the return type's default (NULL for a method without one).
	if err := c.compileStartingValue(method.Name.Value, method.ReturnType, nil, map[ast.Node]bool{}); err != nil {
		return nil, err
	}
	c.emit(code.OpSetLocal, returnSymbol.Index)
	c.setFunctionResult(returnSymbol, false)

	for _, v := range method.Vars {
		if err := c.Compile(v); err != nil {
			return nil, err
		}
	}

	// Abstract methods have no body; they compile to a stub that just returns.
	if method.Body != nil {
		if err := c.Compile(method.Body); err != nil {
			return nil, err
		}
	}

	// Return the result variable (getters and methods) or nothing (setters).
	c.emitFunctionReturn()

	numLocals := c.symbolTable.numDefinitions
	instructions := c.leaveScope()

	// Parameter names let callers pass named arguments, e.g. `fb.M(b := 1, a := 2)`.
	// THIS is parameter 0, so the inputs follow it.
	paramNames := []string{"THIS"}
	for _, p := range method.VarInputs {
		paramNames = append(paramNames, p.Name.Value)
	}

	return &object.CompiledFunction{
		Instructions:   instructions,
		NumLocals:      numLocals,
		NumParameters:  len(method.VarInputs) + 1, // THIS is passed as the first argument
		ParameterNames: paramNames,
	}, nil
}

// compilePropertyAccessor compiles a property's GET or SET block into a CompiledFunction.
func (c *Compiler) compilePropertyAccessor(prop *ast.PropertyDeclaration, body *ast.BlockStatement, isGetter bool) (*object.CompiledFunction, error) {
	c.enterScope()

	// A property accessor is like a method. We push a function context.
	// For GET, the property name is the return variable.
	// For SET, we create a dummy function context.
	funcName := prop.Name.Value
	if !isGetter {
		funcName = "set_" + funcName
	}
	c.pushFunction(&ast.FunctionDeclaration{Name: &ast.Identifier{Value: funcName}})
	defer c.popFunction()

	c.symbolTable.Define("THIS", false) // 'THIS' is implicitly local 0

	var numParams int
	if isGetter {
		// Define and initialize the implicit return variable for the getter.
		returnSymbol := c.symbolTable.Define(prop.Name.Value, false)
		if err := c.compileStartingValue(prop.Name.Value, prop.DataType, nil, map[ast.Node]bool{}); err != nil {
			return nil, err
		}
		c.emit(code.OpSetLocal, returnSymbol.Index)
		c.setFunctionResult(returnSymbol, false)
		numParams = 1 // THIS
	} else {
		c.symbolTable.Define("value", false) // Implicit 'value' parameter for setters
		numParams = 2                        // THIS and value
	}

	if err := c.Compile(body); err != nil {
		return nil, err
	}

	// Return the result variable (getters and methods) or nothing (setters).
	c.emitFunctionReturn()

	numLocals := c.symbolTable.numDefinitions
	instructions := c.leaveScope()

	return &object.CompiledFunction{
		Instructions:  instructions,
		NumLocals:     numLocals,
		NumParameters: numParams,
	}, nil
}

func (c *Compiler) compileMemberAccess(node *ast.MemberAccessExpression) error {
	// Compile the struct/FB instance on the left
	if err := c.Compile(node.Struct); err != nil {
		return err
	}
	// Then, treat the member name as a string constant to be used as an index.
	c.emitConstant(c.addConstant(&object.String{Value: node.Member.Value}))
	c.emit(code.OpIndex)
	return nil
}

func (c *Compiler) isPropertyAccess(expr ast.Expression) (bool, string) {
	memberAccess, ok := expr.(*ast.MemberAccessExpression)
	if !ok {
		return false, ""
	}

	// Get the type name of the struct/FB instance being accessed.
	structTypeName, ok := c.getExpressionTypeName(memberAccess.Struct)
	if !ok {
		return false, ""
	}

	// Find the AST node that defines this type.
	structTypeNode, ok := c.resolveTypeNode(&ast.Identifier{Value: structTypeName})
	if !ok {
		return false, ""
	}

	// Check if the type is a function block.
	fbDef, isFB := structTypeNode.(*ast.FunctionBlockDeclaration)
	if !isFB {
		return false, ""
	}

	// Search the FB and its parents for a property with the matching name.
	prop, _ := c.findPropertyOnFBChain(fbDef, memberAccess.Member.Value)
	if prop != nil {
		return true, prop.Name.Value
	}

	return false, ""
}

func (c *Compiler) compilePropertySet(structExpr ast.Expression, propName string, valueExpr ast.Expression) error {
	// It's a property set. Compile as a call to the setter method.
	// e.g., `p.MyProp := 5` becomes a call to `p.set_MyProp(5)`
	setterCall := &ast.CallExpression{
		Function: &ast.MemberAccessExpression{
			Struct: structExpr,
			Member: &ast.Identifier{Value: "set_" + propName},
		},
		Arguments: []ast.Expression{valueExpr},
	}
	return c.Compile(setterCall)
}

// getExpressionTypeName attempts to statically determine the type name of an expression.
// This is a simplified type analysis for access control.
func (c *Compiler) getExpressionTypeName(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.Identifier:
		symbol, resolved := c.symbolTable.Resolve(e.Value)
		if resolved && symbol.TypeName != "" {
			return symbol.TypeName, true
		}
		// Inside a method, a variable of the enclosing function block is used
		// by name without being a symbol.
		if c.currentFB != nil {
			if varDecl, _ := c.findVarDeclOnFBChain(c.currentFB, e.Value); varDecl != nil && varDecl.DataType != nil {
				return c.flattenExpressionToString(varDecl.DataType), true
			}
		}
		return "", false
	case *ast.ThisExpression:
		if c.currentFB != nil {
			return strings.ToUpper(c.currentFB.Name.Value), true
		}
	case *ast.DereferenceExpression:
		// This handles cases like `SUPER^`
		return c.getExpressionTypeName(e.Pointer)
	case *ast.SuperExpression:
		// This resolves the type of `SUPER` to the parent FB's name.
		if c.currentFB != nil && c.currentFB.Extends != nil {
			return c.flattenExpressionToString(c.currentFB.Extends), true
		}
		// Other complex cases like `getMotor().Speed` are hard to analyze statically
		// without a full type system and are not handled here.
	}
	return "", false
}

// resolveTypeNode finds the AST definition for a type, handling qualified names.
func (c *Compiler) resolveTypeNode(typeExpr ast.Expression) (ast.Node, bool) {
	return c.resolveTypeName(c.flattenExpressionToString(typeExpr))
}

// resolveTypeName finds the AST definition for a type given its (possibly
// qualified) name. An unqualified name also matches a type declared inside a
// namespace, as long as the match is unambiguous.
func (c *Compiler) resolveTypeName(fqn string) (ast.Node, bool) {
	// IEC 61131-3 identifiers are case-insensitive. Always lookup in uppercase.
	upperFqn := strings.ToUpper(fqn)
	node, ok := c.typeInfo[upperFqn]
	if ok {
		return node, true
	}

	// 2. If not found, and it's an unqualified name, search for it.
	// This is a simplified resolution strategy for when `USING` is not present.
	if !strings.Contains(upperFqn, ".") {
		var foundNode ast.Node
		for key, val := range c.typeInfo {
			if strings.HasSuffix(key, "."+upperFqn) {
				if foundNode != nil {
					// Ambiguous reference. For now, we can't resolve it.
					return nil, false
				}
				foundNode = val
			}
		}
		return foundNode, foundNode != nil
	}

	return nil, false
}

// isSubclassOf checks if 'child' is a subclass of 'target' by traversing the
// inheritance chain using the compiler's type information.
func (c *Compiler) isSubclassOf(child *ast.FunctionBlockDeclaration, target *ast.FunctionBlockDeclaration) bool {
	current := child
	for current != nil {
		if current == target {
			return true
		}
		if current.Extends == nil {
			return false
		}
		parentObj, ok := c.resolveTypeNode(current.Extends)
		if !ok {
			return false // Should not happen if types are resolved
		}
		parentFB, ok := parentObj.(*ast.FunctionBlockDeclaration)
		if !ok {
			return false
		}
		current = parentFB
	}
	return false
}

// checkAccessPermission verifies if a member can be accessed from the current
// compilation context (c.currentFB).
func (c *Compiler) checkAccessPermission(accessSpecifier string, ownerDef *ast.FunctionBlockDeclaration) error {
	// PUBLIC members are always accessible. Default is PUBLIC.
	if accessSpecifier == "" || accessSpecifier == "PUBLIC" {
		return nil
	}

	// INTERNAL members: accessible only from within the same namespace. This
	// depends on the caller's namespace, not on whether the caller is an FB.
	if accessSpecifier == "INTERNAL" {
		var ownerFQN string
		for fqn, node := range c.typeInfo {
			if node == ownerDef {
				ownerFQN = fqn
				break
			}
		}
		if ownerFQN == "" {
			// Cannot determine namespace, so we cannot enforce INTERNAL.
			return nil
		}
		if c.pouNamespaces[ownerFQN] != c.currentNS {
			return fmt.Errorf("member is internal")
		}
		return nil
	}

	callerFB := c.currentFB
	if callerFB == nil {
		// Call is from outside any FB (e.g., a PROGRAM). Only PUBLIC is allowed.
		return fmt.Errorf("member is %s", strings.ToLower(accessSpecifier))
	}

	// PRIVATE members: accessible only from within the FB that defines them.
	if accessSpecifier == "PRIVATE" {
		if callerFB == ownerDef {
			return nil
		}
		return fmt.Errorf("member is private")
	}

	// PROTECTED members: accessible from the defining FB or derived FBs.
	if accessSpecifier == "PROTECTED" {
		if c.isSubclassOf(callerFB, ownerDef) {
			return nil
		}
		return fmt.Errorf("member is protected")
	}

	return nil // Should not be reached
}

// findMethodOnFBChain recursively searches for a method definition starting from a given
// function block and traversing up its inheritance chain. It returns the declaration and its owner.
func (c *Compiler) findMethodOnFBChain(fbDef *ast.FunctionBlockDeclaration, methodName string) (*ast.MethodImplementation, *ast.FunctionBlockDeclaration) {
	if fbDef == nil {
		return nil, nil
	}

	// Check if the body itself is the method we are looking for. This can happen
	// if the FB contains only a single method declaration and the parser sets
	// the Body field directly to that node instead of a BlockStatement.
	if method, isMethod := fbDef.Body.(*ast.MethodImplementation); isMethod {
		if method.Name != nil && strings.EqualFold(method.Name.Value, methodName) {
			return method, fbDef
		}
	}

	// Search for the method in the current FB's body.
	if body, ok := fbDef.Body.(*ast.BlockStatement); ok {
		for _, stmt := range body.Statements {
			if method, isMethod := stmt.(*ast.MethodImplementation); isMethod {
				if method.Name != nil && strings.EqualFold(method.Name.Value, methodName) { // Case-insensitive comparison
					return method, fbDef // Found it.
				}
			}
		}
	}

	// If not found, recurse to the parent.
	if fbDef.Extends != nil {
		parentObj, ok := c.resolveTypeNode(fbDef.Extends)
		if !ok {
			return nil, nil
		}
		if parentDef, ok := parentObj.(*ast.FunctionBlockDeclaration); ok {
			return c.findMethodOnFBChain(parentDef, methodName)
		}
	}

	return nil, nil // Reached the top of the chain without finding the method.
}

// findPropertyOnFBChain recursively searches for a property declaration starting from a given
// function block and traversing up its inheritance chain. It returns the declaration and its owner.
func (c *Compiler) findPropertyOnFBChain(fbDef *ast.FunctionBlockDeclaration, propName string) (*ast.PropertyDeclaration, *ast.FunctionBlockDeclaration) {
	if fbDef == nil {
		return nil, nil
	}

	// Search for the property in the current FB's definition.
	for _, prop := range fbDef.Properties {
		if strings.EqualFold(prop.Name.Value, propName) { // Case-insensitive comparison
			return prop, fbDef // Found it.
		}
	}

	// If not found, recurse to the parent.
	if fbDef.Extends != nil {
		parentObj, ok := c.resolveTypeNode(fbDef.Extends)
		if !ok {
			return nil, nil
		}
		if parentDef, isParentFB := parentObj.(*ast.FunctionBlockDeclaration); isParentFB {
			return c.findPropertyOnFBChain(parentDef, propName)
		}
	}

	return nil, nil // Reached the top of the chain without finding the property.
}

// checkInterfaceImplementation verifies that a concrete function block provides
// implementations for all methods and properties required by the interfaces it implements.
func (c *Compiler) checkInterfaceImplementation(fbDef *ast.FunctionBlockDeclaration) error {
	// Collect all interfaces implemented by this FB and its parents.
	allInterfaces := []ast.Expression{}
	currentFB := fbDef
	for currentFB != nil {
		allInterfaces = append(allInterfaces, currentFB.Implements...)
		if currentFB.Extends == nil {
			break
		}
		// The parent FB definition must be resolved from the compiler's type info.
		parentObj, ok := c.resolveTypeNode(currentFB.Extends)
		if !ok {
			return fmt.Errorf("parent function block '%s' not found during interface check", c.flattenExpressionToString(currentFB.Extends))
		}
		parentFB, ok := parentObj.(*ast.FunctionBlockDeclaration)
		if !ok {
			return fmt.Errorf("parent '%s' is not a function block", c.flattenExpressionToString(currentFB.Extends))
		}
		currentFB = parentFB
	}

	uniqueInterfaces := make(map[string]ast.Expression)
	for _, iface := range allInterfaces {
		uniqueInterfaces[c.flattenExpressionToString(iface)] = iface
	}

	for _, ifaceIdent := range uniqueInterfaces {
		// The interface must be found in the compiler's type info.
		ifaceObj, ok := c.resolveTypeNode(ifaceIdent)
		if !ok {
			return fmt.Errorf("interface '%s' not found", c.flattenExpressionToString(ifaceIdent))
		}
		ifaceDef, ok := ifaceObj.(*ast.InterfaceDeclaration)
		if !ok {
			return fmt.Errorf("'%s' is not an interface", c.flattenExpressionToString(ifaceIdent))
		}

		// Use helper to get all members from the interface and its parents.
		requiredMethods, requiredProperties, err := c.getAllInterfaceMembers(ifaceDef, make(map[string]bool))
		if err != nil {
			return err
		}

		for _, requiredMethod := range requiredMethods {
			foundMethod, _ := c.findMethodOnFBChain(fbDef, requiredMethod.Name.Value)
			if foundMethod == nil {
				return fmt.Errorf("function block '%s' does not implement method '%s' from interface '%s'", fbDef.Name.Value, requiredMethod.Name.Value, ifaceDef.Name.Value)
			}
			if foundMethod.IsAbstract {
				return fmt.Errorf("function block '%s' implements method '%s' from interface '%s' with an abstract method", fbDef.Name.Value, requiredMethod.Name.Value, ifaceDef.Name.Value)
			}
			if err := c.validateMethodOverride(foundMethod, requiredMethod); err != nil {
				return fmt.Errorf("signature mismatch for method '%s' required by interface '%s': %w", requiredMethod.Name.Value, ifaceDef.Name.Value, err)
			}
		}

		for _, requiredProp := range requiredProperties {
			foundProp, _ := c.findPropertyOnFBChain(fbDef, requiredProp.Name.Value)
			if foundProp == nil {
				return fmt.Errorf("function block '%s' does not implement property '%s' from interface '%s'", fbDef.Name.Value, requiredProp.Name.Value, ifaceDef.Name.Value)
			}
			if foundProp.IsAbstract {
				return fmt.Errorf("function block '%s' implements property '%s' from interface '%s' with an abstract property", fbDef.Name.Value, requiredProp.Name.Value, ifaceDef.Name.Value)
			}
		}
	}

	return nil
}

// getAllInterfaceMembers recursively collects all required methods and properties from an interface and its parents.
func (c *Compiler) getAllInterfaceMembers(ifaceDef *ast.InterfaceDeclaration, visited map[string]bool) (map[string]*ast.MethodImplementation, map[string]*ast.PropertyDeclaration, error) {
	requiredMethods := make(map[string]*ast.MethodImplementation)
	requiredProperties := make(map[string]*ast.PropertyDeclaration)

	if visited[ifaceDef.Name.Value] {
		return nil, nil, fmt.Errorf("circular interface inheritance detected: '%s' is already in the inheritance path", ifaceDef.Name.Value)
	}
	visited[ifaceDef.Name.Value] = true
	defer delete(visited, ifaceDef.Name.Value) // Clean up for other inheritance branches

	// Add members from the current interface
	for _, method := range ifaceDef.Methods {
		// An interface method is a prototype (*ast.MethodDeclaration), but for validation
		// against a concrete implementation, we need to treat it as an *ast.MethodImplementation
		// for signature checking. We create a temporary one here.
		requiredMethods[method.Name.Value] = &ast.MethodImplementation{
			Token:      method.Token,
			Name:       method.Name,
			ReturnType: method.ReturnType,
			VarInputs:  method.VarInputs,
			VarOutputs: method.VarOutputs,
			VarInOuts:  method.VarInOuts,
		}
	}
	for _, prop := range ifaceDef.Properties {
		requiredProperties[prop.Name.Value] = prop
	}

	// Recursively add members from parent interfaces
	// This assumes the parser has been updated to add an `Extends` field to ast.InterfaceDeclaration
	if ifaceDef.Extends != nil {
		for _, parentIdent := range ifaceDef.Extends {
			parentObj, ok := c.resolveTypeNode(parentIdent)
			if !ok {
				return nil, nil, fmt.Errorf("parent interface '%s' not found", c.flattenExpressionToString(parentIdent))
			}
			parentIface, ok := parentObj.(*ast.InterfaceDeclaration)
			if !ok {
				return nil, nil, fmt.Errorf("parent '%s' is not an interface", c.flattenExpressionToString(parentIdent))
			}
			if parentIface.IsFinal {
				return nil, nil, fmt.Errorf("cannot extend from FINAL interface '%s'", c.flattenExpressionToString(parentIdent))
			}

			parentMethods, parentProps, err := c.getAllInterfaceMembers(parentIface, visited)
			if err != nil {
				return nil, nil, err
			}

			for name, method := range parentMethods {
				requiredMethods[name] = method
			}
			for name, prop := range parentProps {
				requiredProperties[name] = prop
			}
		}
	}

	return requiredMethods, requiredProperties, nil
}

// buildPouInfo recursively traverses the AST to populate the compiler's typeInfo and
// pouNamespaces maps. It uses fully qualified names (e.g., MyLib.MyType) as keys
// to handle nested namespaces correctly.
func (c *Compiler) buildPouInfo(program *ast.Program) {
	var recursiveBuild func(stmts []ast.Statement, ns *ast.NamespaceDeclaration, prefix string)
	recursiveBuild = func(stmts []ast.Statement, ns *ast.NamespaceDeclaration, prefix string) {
		for _, stmt := range stmts {
			var pouName, fqn string
			var pouNode ast.Node

			switch node := stmt.(type) {
			case *ast.FunctionBlockDeclaration:
				pouName = node.Name.Value
				pouNode = node
			case *ast.ProgramDeclaration:
				pouName = node.Name.Value
				pouNode = node
			case *ast.FunctionDeclaration:
				pouName = node.Name.Value
				pouNode = node
			case *ast.InterfaceDeclaration:
				pouName = node.Name.Value
				pouNode = node
			case *ast.TypeBlockDeclaration:
				for _, decl := range node.Declarations {
					fqn = decl.Name.Value
					if prefix != "" {
						fqn = prefix + "." + decl.Name.Value
					}
					// Stored in upper case, like POUs: identifiers are case-insensitive.
					c.typeInfo[strings.ToUpper(fqn)] = decl
					c.pouNamespaces[strings.ToUpper(fqn)] = ns
				}
				continue // Skip the common POU registration logic
			case *ast.NamespaceDeclaration:
				nsName := c.flattenExpressionToString(node.Name)
				newPrefix := prefix
				if newPrefix != "" {
					newPrefix += "."
				}
				newPrefix += nsName
				recursiveBuild(node.Statements, node, newPrefix)
				continue // Skip the common POU registration logic
			default:
				continue // Not a POU or namespace
			}

			fqn = pouName
			if prefix != "" {
				fqn = prefix + "." + pouName
			}
			c.typeInfo[strings.ToUpper(fqn)] = pouNode // Store FQN in uppercase
			c.pouNamespaces[strings.ToUpper(fqn)] = ns
		}
	}
	recursiveBuild(program.Statements, nil, "")
}

// flattenExpressionToString converts a potentially nested MemberAccessExpression into a single qualified string.
func (c *Compiler) flattenExpressionToString(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return strings.ToUpper(ident.Value) // Return uppercase for consistency
	}
	if member, ok := expr.(*ast.MemberAccessExpression); ok {
		// Recursively flatten the struct part and append the member.
		return c.flattenExpressionToString(member.Struct) + "." + strings.ToUpper(member.Member.Value) // Uppercase member
	}
	if ts, ok := expr.(*ast.TypeSpecifier); ok {
		return strings.ToUpper(ts.Token.Literal) // Uppercase type literal
	}
	return "" // Should not happen for valid type names, but return empty string for safety.
}

// orderByInheritance returns the statements with function block declarations
// moved ahead of all other statements, and ordered so that each parent FB
// defined in the same statement list precedes the FBs that extend it. IEC
// 61131-3 declarations are order-independent, but at runtime a derived FB's
// hash loads its parent's hash, and an FB instance loads its class hash, so
// those must already exist. Other statements keep their relative order.
// Inheritance cycles are left in source order; the FB compile step reports them.
func (c *Compiler) orderByInheritance(stmts []ast.Statement) []ast.Statement {
	inList := make(map[*ast.FunctionBlockDeclaration]bool)
	for _, s := range stmts {
		if fb, ok := s.(*ast.FunctionBlockDeclaration); ok {
			inList[fb] = true
		}
	}
	if len(inList) == 0 {
		return stmts
	}

	fbs := make([]ast.Statement, 0, len(inList))
	others := make([]ast.Statement, 0, len(stmts)-len(inList))
	done := make(map[*ast.FunctionBlockDeclaration]bool)
	inProgress := make(map[*ast.FunctionBlockDeclaration]bool)
	hasCycle := false
	var visit func(fb *ast.FunctionBlockDeclaration)
	visit = func(fb *ast.FunctionBlockDeclaration) {
		if inProgress[fb] {
			hasCycle = true
			return
		}
		if done[fb] {
			return
		}
		inProgress[fb] = true
		if fb.Extends != nil {
			if parentNode, ok := c.resolveTypeNode(fb.Extends); ok {
				if parentFB, ok := parentNode.(*ast.FunctionBlockDeclaration); ok && inList[parentFB] {
					visit(parentFB)
				}
			}
		}
		delete(inProgress, fb)
		done[fb] = true
		fbs = append(fbs, fb)
	}

	// Functions come first, so that function block bodies and methods can
	// call them; function blocks only need each other's (predefined) classes.
	functions := []ast.Statement{}
	for _, s := range stmts {
		if fb, ok := s.(*ast.FunctionBlockDeclaration); ok {
			visit(fb)
			continue
		}
		if _, isFunction := s.(*ast.FunctionDeclaration); isFunction {
			functions = append(functions, s)
			continue
		}
		others = append(others, s)
	}
	if hasCycle {
		// Keep source order so the cycle is reported from the first FB written.
		return stmts
	}
	return append(append(functions, fbs...), others...)
}

// predefineFunctionBlocks defines a global symbol for every function block in
// stmts, including those inside namespaces, before any code is compiled. This
// lets an FB be instantiated (e.g. in a method's VAR block) by code compiled
// before the FB's own declaration, since IEC 61131-3 declarations are
// order-independent. The FB compile step later fills the symbol in.
func (c *Compiler) predefineFunctionBlocks(stmts []ast.Statement) {
	if c.scopeIndex != 0 {
		return
	}
	for _, s := range stmts {
		switch node := s.(type) {
		case *ast.FunctionBlockDeclaration:
			if _, ok := c.symbolTable.ResolveClass(node); !ok {
				c.symbolTable.DefineClass(node, c.symbolTable.Define(node.Name.Value, false))
			}
		case *ast.NamespaceDeclaration:
			c.predefineFunctionBlocks(node.Statements)
		}
	}
}

// predefineGlobals defines a global symbol for every function and VAR_GLOBAL
// variable declared at the top level of stmts, before any code is compiled,
// so a function can call a function or use a global declared after it. The
// declaration's own compile step later takes the symbol (see definePredefined).
func (c *Compiler) predefineGlobals(stmts []ast.Statement) {
	if c.scopeIndex != 0 {
		return
	}
	if c.predefined == nil {
		c.predefined = make(map[ast.Node]Symbol)
	}
	for _, s := range stmts {
		switch node := s.(type) {
		case *ast.FunctionDeclaration:
			c.predefined[node] = c.symbolTable.Define(node.Name.Value, false)
		case *ast.GlobalVarDeclaration:
			for _, decl := range node.Vars {
				if decl.Location != nil {
					continue
				}
				if _, isMacro := decl.Value.(*ast.MacroLiteral); isMacro {
					continue
				}
				typeName := ""
				if decl.DataType != nil {
					typeName = c.flattenExpressionToString(decl.DataType)
				}
				c.predefined[decl] = c.symbolTable.Define(decl.Name.Value, decl.IsConstant, typeName)
				c.scopes[0].varDecls[decl.Name.Value] = decl
			}
		}
	}
}

// definePredefined returns the symbol predefineGlobals made for a
// declaration, or defines a new one.
func (c *Compiler) definePredefined(node ast.Node, name string, isConstant bool, typeName ...string) Symbol {
	if symbol, ok := c.predefined[node]; ok && c.scopeIndex == 0 {
		delete(c.predefined, node)
		if current, found := c.symbolTable.Resolve(name); found && current == symbol {
			return symbol
		}
	}
	return c.symbolTable.Define(name, isConstant, typeName...)
}

// emitFBInstanceWith emits code that builds a new instance of fbDef and leaves it on
// the stack. An instance is a hash with a "__class__" entry holding the FB's
// class hash (so the VM can find methods and SUPER targets), plus one entry per
// variable declared along the inheritance chain, set to its initial value.
// A variable redeclared in a derived FB takes the derived declaration.
// Each variable starts from its initial value or its type's default, so a
// variable of FB type without an initializer becomes a nested instance.
// An initializer such as `(PT := T#1s)` overrides the named variables' starting values.
func (c *Compiler) emitFBInstanceWith(fbDef *ast.FunctionBlockDeclaration, init *ast.StructLiteral, visiting map[ast.Node]bool) error {
	if visiting[fbDef] {
		return fmt.Errorf("function block '%s' contains an instance of itself", fbDef.Name.Value)
	}
	visiting[fbDef] = true
	defer delete(visiting, fbDef)

	classSymbol, ok := c.symbolTable.ResolveClass(fbDef)
	if !ok {
		return fmt.Errorf("function block '%s' must be declared before it is instantiated", fbDef.Name.Value)
	}

	// Build the inheritance chain root-first, so derived declarations win.
	chain := []*ast.FunctionBlockDeclaration{}
	for current := fbDef; current != nil; {
		chain = append([]*ast.FunctionBlockDeclaration{current}, chain...)
		if current.Extends == nil {
			break
		}
		parentNode, ok := c.resolveTypeNode(current.Extends)
		if !ok {
			break
		}
		parentFB, ok := parentNode.(*ast.FunctionBlockDeclaration)
		if !ok || visiting[ast.Node(parentFB)] {
			break
		}
		current = parentFB
	}

	var order []string
	fields := make(map[string]*ast.VarDeclStatement)
	for _, fb := range chain {
		for _, group := range [][]*ast.VarDeclStatement{fb.VarInputs, fb.VarOutputs, fb.VarInOuts, fb.Vars} {
			for _, v := range group {
				key := strings.ToUpper(v.Name.Value)
				if _, seen := fields[key]; !seen {
					order = append(order, key)
				}
				fields[key] = v
			}
		}
	}

	overrides, err := structInitializers(init)
	if err != nil {
		return err
	}
	for member := range overrides {
		if _, ok := fields[strings.ToUpper(member)]; !ok {
			return fmt.Errorf("function block '%s' has no variable '%s'", fbDef.Name.Value, member)
		}
	}

	c.emitConstant(c.addConstant(&object.String{Value: "__class__"}))
	c.loadSymbol(classSymbol)
	for _, key := range order {
		v := fields[key]
		c.emitConstant(c.addConstant(&object.String{Value: v.Name.Value}))
		value := v.Value
		if override, ok := lookupFold(overrides, v.Name.Value); ok {
			value = override
		}
		if err := c.compileStartingValue(v.Name.Value, v.DataType, value, visiting); err != nil {
			return err
		}
	}
	c.emit(code.OpHash, (len(order)+1)*2)
	return nil
}

// isIlBlock checks if a given block statement contains IL instructions.
func isIlBlock(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if _, isIL := stmt.(*ast.IlInstructionStatement); isIL {
			return true
		}
	}
	return false
}
