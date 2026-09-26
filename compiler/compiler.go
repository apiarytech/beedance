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
type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int

	functionStack []*ast.FunctionDeclaration
	loopCtxStack  []*loopContext
	currentFB     *ast.FunctionBlockDeclaration
	typeInfo      map[string]ast.Node
	pouNamespaces map[string]*ast.NamespaceDeclaration // Maps POU name to its namespace
	currentNS     *ast.NamespaceDeclaration            // The namespace currently being compiled
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
// constant pool, which is useful for testing or for a REPL environment.
func NewWithState(s *SymbolTable, constants []object.Object, typeInfo map[string]ast.Node) *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
		varDecls:            make(map[string]*ast.VarDeclStatement),
	}
	if typeInfo == nil {
		typeInfo = make(map[string]ast.Node)
	}
	pouNamespaces := make(map[string]*ast.NamespaceDeclaration)

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
func (c *Compiler) CompileProgram(node *ast.ProgramDeclaration) (*CompiledProgram, error) {
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

	for _, decl := range varDecls {
		if err := c.Compile(decl); err != nil {
			return nil, err
		}
	}
	initBytecode := c.Bytecode()

	// --- Cyclic Phase ---
	// Create a new, clean compiler for the cyclic part to ensure it doesn't
	// re-declare variables. It shares the same symbol table and constants.
	cyclicCompiler := NewWithState(c.symbolTable, c.constants, c.typeInfo)

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

		// Second pass: compile all statements.
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
			if _, ok := s.(*ast.IfStatement); ok {
				c.emit(code.OpPop)
			}
		}

	// A Configuration block is compiled into a data structure representing the system setup.
	case *ast.ConfigurationDeclaration:
		return c.compileConfiguration(node)

	case *ast.NamespaceDeclaration:
		originalNS := c.currentNS
		c.currentNS = node
		for _, s := range node.Statements {
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
		c.typeInfo[node.Name.Value] = node

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

	case *ast.InterfaceDeclaration:
		// Define the interface name in the symbol table and store its AST node for type checking.
		symbol := c.symbolTable.Define(node.Name.Value, false)
		c.typeInfo[node.Name.Value] = node
		// Interfaces don't generate executable code, they are for compile-time checks.
		// We emit a null and set it to a global var to have a placeholder.
		c.emit(code.OpNull)
		c.emit(code.OpSetGlobal, symbol.Index)
		return nil

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
		// the main logic body, and each method.
		symbol := c.symbolTable.Define(node.Name.Value, false)
		c.typeInfo[node.Name.Value] = node

		// If the FB extends another, we need to add a "__parent__" key to the hash.
		// The VM will use this to walk the inheritance chain.
		var parentSymbol *Symbol
		if node.Extends != nil {
			// --- FINAL Function Block Check ---
			parentName := c.flattenExpressionToString(node.Extends)
			parentDefNode, ok := c.resolveTypeNode(node.Extends)
			if !ok {
				return fmt.Errorf("parent function block '%s' definition not found", parentName)
			}
			parentFB, ok := parentDefNode.(*ast.FunctionBlockDeclaration)
			if !ok {
				return fmt.Errorf("parent '%s' is not a function block", parentName)
			}
			if parentFB.IsFinal {
				return fmt.Errorf("cannot extend from FINAL function block '%s'", parentName)
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
		if node.Extends != nil {
			_ = c.flattenExpressionToString(node.Extends)
			parentDefNode, ok := c.resolveTypeNode(node.Extends)
			if ok {
				if parentFB, ok := parentDefNode.(*ast.FunctionBlockDeclaration); ok {
					for _, derivedMethod := range methods {
						if parentMethod, ownerFB := c.findMethodOnFBChain(parentFB, derivedMethod.Name.Value); parentMethod != nil {
							if parentMethod.IsFinal {
								return fmt.Errorf("cannot override FINAL method '%s' from function block '%s'", parentMethod.Name.Value, ownerFB.Name.Value)
							}
							// --- Method Signature Validation ---
							if err := c.validateMethodOverride(derivedMethod, parentMethod); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		// --- FINAL Property Override Check ---
		if node.Extends != nil {
			_ = c.flattenExpressionToString(node.Extends)
			parentDefNode, ok := c.resolveTypeNode(node.Extends)
			if ok {
				if parentFB, ok := parentDefNode.(*ast.FunctionBlockDeclaration); ok {
					for _, derivedProp := range node.Properties {
						parentProp, ownerFB := c.findPropertyOnFBChain(parentFB, derivedProp.Name.Value)
						if parentProp != nil && parentProp.IsFinal {
							return fmt.Errorf("cannot override FINAL property '%s' from function block '%s'", parentProp.Name.Value, ownerFB.Name.Value)
						}
					}
				}
			}
		}

		// --- FINAL Variable Override Check ---
		if node.Extends != nil {
			_ = c.flattenExpressionToString(node.Extends)
			parentDefNode, ok := c.resolveTypeNode(node.Extends)
			if ok {
				if parentFB, ok := parentDefNode.(*ast.FunctionBlockDeclaration); ok {
					// We need to check all variable declarations in the current FB
					// This includes VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR
					allVars := [][]*ast.VarDeclStatement{
						node.VarInputs, node.VarOutputs, node.VarInOuts, node.Vars,
					}
					for _, varBlock := range allVars {
						for _, derivedVar := range varBlock {
							parentVar, ownerFB := c.findVarDeclOnFBChain(parentFB, derivedVar.Name.Value)
							if parentVar != nil && parentVar.IsFinal {
								return fmt.Errorf("cannot override FINAL variable '%s' from function block '%s'", parentVar.Name.Value, ownerFB.Name.Value)
							}
						}
					}
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

		// Compile main body
		c.enterScope()
		c.symbolTable.Define("THIS", false) // 'THIS' is implicitly local 0
		for _, p := range node.VarInputs {
			c.symbolTable.Define(p.Name.Value, false)
		}
		for _, p := range node.VarOutputs {
			c.symbolTable.Define(p.Name.Value, false)
		}
		for _, p := range node.VarInOuts {
			c.symbolTable.Define(p.Name.Value, false)
		}
		// Instance variables (VAR blocks) are not compiled into the main body's
		// instructions. They represent the state of the FB instance and are
		// handled by the VM during instantiation.
		if err := c.Compile(mainBody); err != nil {
			return err
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		numLocals := c.symbolTable.numDefinitions
		mainLogicInstructions := c.leaveScope()
		mainFn := &object.CompiledFunction{
			Instructions:  mainLogicInstructions,
			NumLocals:     numLocals,
			NumParameters: len(node.VarInputs),
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
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: "__parent__"}))
			c.loadSymbol(*parentSymbol)
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
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: pair.name})) // cspell:disable-line
			c.emit(code.OpClosure, pair.index, 0)                                    // cspell:disable-line
		}
		c.emit(code.OpHash, numHashPairs*2)
		c.emit(code.OpSetGlobal, symbol.Index)
		return nil

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
		symbol := c.symbolTable.Define(node.Name.Value, false)
		c.typeInfo[node.Name.Value] = node

		// Then, compile the function body itself.
		c.enterScope()
		c.pushFunction(node)

		c.symbolTable.DefineFunctionName(node.Name.Value) // For recursion

		paramNames := make([]string, len(node.VarInputs))
		// Define input parameters first, as they are the first locals in the stack frame.
		for i, p := range node.VarInputs {
			c.symbolTable.DefineVarInput(p.Name.Value)
			paramNames[i] = p.Name.Value
		}

		// Define the function name as a local variable to hold the return value.
		returnSymbol := c.symbolTable.Define(node.Name.Value, false)

		outputNames := make([]string, len(node.VarOutputs))
		outputIndices := make([]int, len(node.VarOutputs))
		for i, p := range node.VarOutputs {
			outputNames[i] = p.Name.Value
			// The index will be populated when the symbol is defined below.
		}

		// Initialize the return variable to Null. This ensures that if no explicit
		// return value is assigned, the function implicitly returns Null.
		c.emit(code.OpNull)
		c.emit(code.OpSetLocal, returnSymbol.Index)

		for i, p := range node.VarOutputs {
			symbol := c.symbolTable.Define(p.Name.Value, false)
			outputIndices[i] = symbol.Index
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

		// If the last statement in a function body is an expression, its result
		// should be the return value. We replace the OpPop with OpReturnValue.
		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}

		// If the function body did not already emit a return value (e.g., via
		// an assignment to the function name, which is compiled as a return),
		// we add an implicit return of the function's return variable.
		if !c.lastInstructionIs(code.OpReturnValue) {
			// If the function "falls off the end" without an explicit return,
			// implicitly return NULL.
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		c.popFunction()
		compiledFn := &object.CompiledFunction{
			Instructions:   instructions,
			NumLocals:      numLocals,
			NumParameters:  len(node.VarInputs),
			ParameterNames: paramNames,
			OutputNames:    outputNames,
			OutputIndices:  outputIndices,
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
		// The result of an expression statement is not used, so we must
		// pop it from the stack to prevent stack corruption.
		c.emit(code.OpPop)

	case *ast.AssignmentStatement:
		// Use a type switch to safely handle different kinds of assignment targets
		// and prevent panics if the parser produces an unexpected AST node.
		switch target := node.Left.(type) {
		case *ast.Identifier:
			// Check if this is an assignment to the current function's name,
			// which is the IEC 61131-3 way of setting a return value.
			if currentFn := c.currentFunction(); currentFn != nil && target.Value == currentFn.Name.Value {
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				c.emit(code.OpReturnValue)
				return nil
			}

			// Check if this is an assignment to an instance variable, which needs
			// to be implicitly treated as an assignment to `THIS.variable`.
			isInstanceVar := false
			if c.currentFB != nil {
				for _, varDecl := range c.currentFB.Vars {
					if varDecl.Name.Value == target.Value {
						isInstanceVar = true
						break
					}
				}
			}

			if isInstanceVar {
				// Compile as `THIS.variable := value`
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				// The 'THIS' symbol is always local 0 in a method/FB body.
				thisSymbol, _ := c.symbolTable.Resolve("THIS")
				c.loadSymbol(thisSymbol)
				// The member name becomes a constant index for the set operation.
				c.emit(code.OpConstant, c.addConstant(&object.String{Value: target.Value}))
				c.emit(code.OpSetIndex)
				return nil
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
			// Compile the value to be assigned (RHS)
			if err := c.Compile(node.Value); err != nil {
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
			c.emit(code.OpSetIndex)
			return nil
		case *ast.MemberAccessExpression:
			// Check if this is an assignment to a property.
			// --- Start of new access control logic for assignment ---
			if structTypeName, ok := c.getExpressionTypeName(target.Struct); ok {
				if structTypeNode, ok := c.typeInfo[structTypeName]; ok {
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
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				if err := c.Compile(target.Struct); err != nil {
					return err
				}
				c.emit(code.OpConstant, c.addConstant(&object.String{Value: target.Member.Value}))
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

		symbol := c.symbolTable.Define(node.Name.Value, node.IsConstant, typeName)
		c.scopes[c.scopeIndex].varDecls[node.Name.Value] = node

		if node.Value != nil {
			err := c.Compile(node.Value)
			if err != nil {
				return err
			}
		} else {
			// If no initial value is provided, check if we are trying to instantiate an abstract FB.
			if typeName != "" { // This check will now work correctly.
				if typeDef, ok := c.resolveTypeNode(node.DataType); ok {
					if fbDef, isFB := typeDef.(*ast.FunctionBlockDeclaration); isFB {
						if fbDef.IsAbstract {
							return fmt.Errorf("cannot instantiate abstract function block '%s'", typeName)
						}
						// Check for INTERNAL access
						if fbDef.AccessSpecifier == "INTERNAL" {
							defNS := c.pouNamespaces[fbDef.Name.Value]
							callerNS := c.currentNS
							if defNS != callerNS {
								return fmt.Errorf("cannot access INTERNAL function block '%s' from a different namespace", typeName)
							}
						}
					}
				}
			}
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
		c.enterScope() // Scope for the control variable
		c.enterLoop()

		// 1. Initialization
		controlVarName := node.ControlVar.Left.(*ast.Identifier).Value
		if err := c.Compile(node.ControlVar.Value); err != nil {
			return err
		}
		symbol := c.symbolTable.Define(controlVarName, false)
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
		// Check if this is an IL program body by inspecting the first statement.
		if len(node.Statements) > 0 {
			if _, ok := node.Statements[0].(*ast.IlInstructionStatement); ok {
				return c.compileIlProgram(node)
			}
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
		// Check if this identifier refers to an instance variable of the current function block.
		// If so, implicitly transform it into a `THIS.Identifier` access. This only
		// applies to `VAR` instance variables, not I/O vars which are locals/params.
		if c.currentFB != nil {
			// We only check the `Vars` list, which contains the instance variables
			// (from VAR...END_VAR blocks). Input, output, and in-out variables are
			// handled as local symbols within the scope of the FB's main body or methods,
			// so they should not be checked here.
			for _, varDecl := range c.currentFB.Vars {
				if varDecl.Name.Value == node.Value {
					thisExpr := &ast.ThisExpression{Token: node.Token}
					memberAccess := &ast.MemberAccessExpression{Struct: thisExpr, Member: node}
					return c.Compile(memberAccess)
				}
			}
		}
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
			if structTypeNode, ok := c.typeInfo[structTypeName]; ok {
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

		// Handle SUPER calls first, as they are a special form of member access.
		if deref, ok := node.Struct.(*ast.DereferenceExpression); ok {
			if _, ok := deref.Pointer.(*ast.SuperExpression); ok {
				// This is a SUPER^.Method access.
				// We push the current instance ('THIS') and the method name,
				// then use a special opcode to tell the VM to do a lookup on the parent.
				c.emit(code.OpGetLocal, 0) // Get THIS instance
				c.emit(code.OpConstant, c.addConstant(&object.String{Value: node.Member.Value}))
				c.emit(code.OpSuperIndex) // New opcode for super-method lookup
				return nil
			}
		}

		// Check if this is a property access (read).
		isProperty := false
		// A more robust implementation would inspect the type of `node.Struct`.
		// For now, we check if we are inside an FB and the access is on `THIS`.
		if c.currentFB != nil {
			if _, ok := node.Struct.(*ast.ThisExpression); ok {
				for _, prop := range c.currentFB.Properties {
					if prop.Name.Value == node.Member.Value {
						isProperty = true
						break
					}
				}
			}
		}

		if isProperty {
			// It's a property get. Compile as a call to the getter method.
			// e.g., `x := p.MyProp` is compiled as `x := p.get_MyProp()`
			getterCall := &ast.CallExpression{
				Function: &ast.MemberAccessExpression{
					Struct: node.Struct,
					Member: &ast.Identifier{Value: "get_" + node.Member.Value},
				},
				Arguments: []ast.Expression{}, // Getter has no arguments
			}
			return c.Compile(getterCall)
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
		constIndex := c.addConstant(macro)
		c.emit(code.OpConstant, constIndex)

	// A ReturnStatement compiles the return value and emits OpReturnValue.
	case *ast.ReturnStatement:
		if node.ReturnValue == nil {
			c.emit(code.OpReturn)
			return nil
		}
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		c.emit(code.OpReturnValue)

	// A CallExpression compiles the function/callable and all arguments, then
	// emits an OpCall instruction.
	case *ast.CallExpression:
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
				nameIndex := c.addConstant(&object.String{Value: named.Name.Value})
				c.emit(code.OpMakeNamedArg, nameIndex)
			} else {
				// It's a positional argument.
				if err := c.Compile(arg); err != nil {
					return err
				}
			}
		}

		c.emit(code.OpCall, len(inputArgs))

		// --- Part 2: Compile output assignments ---
		// The result of the call (FB instance or return value hash) is now on the stack.
		for i, out := range outputArgs {
			// If we have more assignments to make, duplicate the function result on the stack.
			if i < len(outputArgs) {
				c.emit(code.OpDup)
			}

			// Compile member access: <func_result>.<source_name>
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: out.Source.Value}))
			c.emit(code.OpIndex)

			// Compile assignment to the target variable.
			targetIdent, ok := out.Target.(*ast.Identifier)
			if !ok {
				return fmt.Errorf("output argument target must be an identifier, got %T", out.Target)
			}
			symbol, ok := c.symbolTable.Resolve(targetIdent.Value)
			if !ok {
				return fmt.Errorf("undefined variable %s", targetIdent.Value)
			}
			err := c.setSymbol(symbol)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// validateMethodOverride checks if a derived method has a signature compatible with its parent.
func (c *Compiler) validateMethodOverride(derived, parent *ast.MethodImplementation) error {
	// 1. Check return type
	derivedReturn := "VOID"
	if derived.ReturnType != nil {
		derivedReturn = derived.ReturnType.String()
	}
	parentReturn := "VOID"
	if parent.ReturnType != nil {
		parentReturn = parent.ReturnType.String()
	}

	if derivedReturn != parentReturn {
		// If types are different, check for covariance (only for function blocks).
		parentTypeNode, parentTypeFound := c.typeInfo[parentReturn]
		derivedTypeNode, derivedTypeFound := c.typeInfo[derivedReturn]

		// If either type is not a defined type (i.e., it's a primitive like INT, BOOL), then they must be identical.
		// The initial `derivedReturn != parentReturn` check already covers this.
		if !parentTypeFound || !derivedTypeFound {
			return fmt.Errorf("return type mismatch for method '%s': derived is '%s', parent is '%s'", derived.Name.Value, derivedReturn, parentReturn)
		}

		parentFBDef, isParentFB := parentTypeNode.(*ast.FunctionBlockDeclaration)
		derivedFBDef, isDerivedFB := derivedTypeNode.(*ast.FunctionBlockDeclaration)

		// Covariance is only allowed if both return types are function blocks.
		if isParentFB && isDerivedFB {
			// Check if the derived return type is a subclass of the parent return type.
			if !c.isSubclassOf(derivedFBDef, parentFBDef) {
				return fmt.Errorf("incompatible return types for method '%s': '%s' is not a subclass of '%s'", derived.Name.Value, derivedReturn, parentReturn)
			}
			// If it is a subclass, this is a valid covariant return.
		} else {
			// If one or both are not function blocks (e.g., INTERFACE, STRUCT, or primitive), types must be identical.
			return fmt.Errorf("return type mismatch for method '%s': derived is '%s', parent is '%s'", derived.Name.Value, derivedReturn, parentReturn)
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

		if derivedParamTypeStr != parentParamTypeStr {
			// If types are different, check for contravariance (only for function blocks).
			parentTypeNode, parentTypeFound := c.typeInfo[parentParamTypeStr]
			derivedTypeNode, derivedTypeFound := c.typeInfo[derivedParamTypeStr]

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
			if decl.Name.Value == varName {
				return decl, fbDef // Found it.
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

// getExpressionType recursively determines the data type of an AST expression node.
func (c *Compiler) getExpressionType(expr ast.Expression) (object.ObjectType, error) {
	switch e := expr.(type) {
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
	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(e.Value)
		if !ok {
			return "", fmt.Errorf("undefined identifier: %s", e.Value)
		}
		if symbol.TypeName == "" {
			return "", fmt.Errorf("cannot determine type of identifier: %s", e.Value)
		}
		return object.ObjectType(strings.ToUpper(symbol.TypeName)), nil
	case *ast.InfixExpression:
		leftType, err := c.getExpressionType(e.Left)
		if err != nil {
			return "", err
		}
		rightType, err := c.getExpressionType(e.Right)
		if err != nil {
			return "", err
		}
		return c.getResultingType(e.Operator, leftType, rightType)
	case *ast.PrefixExpression:
		// For prefix expressions, the type is usually the same as the operand's type.
		return c.getExpressionType(e.Right)
	case *ast.CallExpression:
		// This requires looking up the function's return type.
		if ident, ok := e.Function.(*ast.Identifier); ok {
			if funcDefNode, ok := c.typeInfo[ident.Value]; ok {
				if funcDef, isFunc := funcDefNode.(*ast.FunctionDeclaration); isFunc {
					if funcDef.ReturnType != nil {
						return object.ObjectType(strings.ToUpper(funcDef.ReturnType.String())), nil
					}
					return object.NULL_OBJ, nil // VOID function
				}
			}
		}
		// Fallback for built-ins or complex expressions. This would need to be expanded.
		return "", fmt.Errorf("type inference for function call '%s' not yet implemented", e.Function.String())
	default:
		return "", fmt.Errorf("cannot determine type of expression: %T", expr)
	}
}

// getResultingType checks if an operator is valid for the given operand types
// and returns the resulting type according to IEC 61131-3 type promotion rules.
func (c *Compiler) getResultingType(op string, left, right object.ObjectType) (object.ObjectType, error) {
	// Helper to check if a type is numeric
	isNumeric := func(t object.ObjectType) bool {
		return object.IsIntegerType(string(t)) || object.IsRealType(string(t))
	}

	switch op {
	case "+", "-", "*", "/":
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

	case ">", "<", ">=", "<=", "=", "<>":
		// Comparisons are generally valid between any two numeric types.
		if isNumeric(left) && isNumeric(right) {
			return object.BOOLEAN_OBJ, nil
		}
		// Also allow string comparison
		if (left == object.STRING_OBJ || left == object.WSTRING_OBJ) && (right == object.STRING_OBJ || right == object.WSTRING_OBJ) {
			return object.BOOLEAN_OBJ, nil
		}
		return "", fmt.Errorf("comparison operator '%s' not defined for types %s and %s", op, left, right)

	case "AND", "OR", "XOR":
		if (left == object.BOOLEAN_OBJ && right == object.BOOLEAN_OBJ) || (object.IsBitStringType(string(left)) && object.IsBitStringType(string(right))) {
			return left, nil // Result type is the same as operand type
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
				if fn1.NumLocals == fn2.NumLocals &&
					fn1.NumParameters == fn2.NumParameters &&
					bytes.Equal(fn1.Instructions, fn2.Instructions) {
					return i
				}
			}
			// If the existing constant is a function, but the new object is not
			// (or they are different functions), they cannot be equal. We skip
			// the generic `object.IsEqual` check below and move to the next
			// constant in the pool. This prevents incorrect fall-through.
			continue
		}

		if object.IsEqual(constant, obj) {
			return i
		}
	}
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
	symbol := c.symbolTable.Define(config.Name.Value, false)

	// Compile any global vars defined directly in the configuration.
	for _, gv := range config.GlobalVars {
		if err := c.Compile(gv); err != nil {
			return err
		}
	}

	// Build the final configuration hash object.
	// Key: "name"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "name"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: config.Name.Value}))

	// Key: "resources"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "resources"}))
	// Compile each resource, leaving a resource hash object on the stack.
	for _, res := range config.Resources {
		if err := c.compileResource(res, config.VarConfigs); err != nil {
			return err
		}
	}
	// Create an array of resource hashes, which becomes the value for the "resources" key.
	c.emit(code.OpArray, len(config.Resources))

	c.emit(code.OpHash, 2*2) // 2 key-value pairs

	// Store the final configuration hash in its global variable.
	c.emit(code.OpSetGlobal, symbol.Index)
	return nil
}

// compileResource compiles a RESOURCE block into a hash object containing its
// tasks and program instances.
func (c *Compiler) compileResource(res *ast.ResourceDeclaration, varConfigs []*ast.ConfigVarDeclaration) error {
	// Build the resource hash by compiling its key-value pairs in order.
	// Key: "name"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "name"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: res.Name.Value}))

	// Key: "type"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "type"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: res.ResourceType.Value}))

	// Key: "tasks"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "tasks"}))
	// Value: Compile tasks and create an array of task hashes.
	for _, task := range res.Tasks {
		if err := c.compileTask(task); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(res.Tasks))

	// Key: "programs"
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "programs"}))
	// Value: Compile program instances and create an array of program hashes.
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
	c.emit(code.OpArray, len(res.Programs))

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
	if task.Interval == nil {
		c.emit(code.OpNull)
	}

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "priority"}))
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
// object, including its parameters from any associated VAR_CONFIG block.
func (c *Compiler) compileProgramConfig(prog *ast.ProgramConfiguration, varConfig *ast.ConfigVarDeclaration) error {
	// Build the program configuration hash object.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "instance"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: prog.InstanceName.Value}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "task"}))
	taskName := ""
	if prog.TaskName != nil {
		taskName = prog.TaskName.Value
	}
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: taskName}))

	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "type"}))
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: prog.TypeName.Value}))

	// Add the parameters from VAR_CONFIG as a nested hash.
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: "params"}))
	if varConfig != nil {
		for _, decl := range varConfig.Declarations {
			// The parser for VAR_CONFIG puts the variable path into AccessPath.
			// We use its string representation as the key.
			paramName := decl.AccessPath.String()
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: paramName}))
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
func (c *Compiler) parseTypedLiteralValue(node *ast.TypedLiteral) (object.Object, error) {
	typeName := strings.ToUpper(node.TypeName)
	valueStr := node.Value.String() // This is an ast.Identifier with the value part

	switch typeName {
	case "TIME", "T":
		// IEC duration can have underscores, Go's time.ParseDuration does not support them.
		durationStr := strings.ReplaceAll(valueStr, "_", "")
		d, err := time.ParseDuration(durationStr)
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
			return nil, err
		}
		// The VM uses LINT for all integer operations for simplicity.
		return &object.LInt{Value: val}, nil
	case "USINT", "UINT", "UDINT", "ULINT":
		val, err := c.parseBasedUnsignedInteger(valueStr)
		if err != nil {
			return nil, err
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
			return nil, err
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

	c.emit(code.OpConstant, c.addConstant(obj))
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
		return nil // Index is not a constant, cannot check at compile time.
	}

	// 2. Resolve the array expression to an identifier to find its declaration.
	arrayIdent, ok := arrayExpr.(*ast.Identifier)
	if !ok {
		return nil // Array expression is complex (e.g., func()[i]), cannot check.
	}

	// 3. Find the variable's declaration by searching scopes.
	var varDecl *ast.VarDeclStatement
	for i := c.scopeIndex; i >= 0; i-- {
		if decl, found := c.scopes[i].varDecls[arrayIdent.Value]; found {
			varDecl = decl
			break
		}
	}
	if varDecl == nil {
		return nil // Declaration not found in any scope.
	}

	// 4. Get the ArrayDefinition from the variable's declaration.
	arrayDef, ok := varDecl.DataType.(*ast.ArrayDefinition)
	if !ok {
		return nil // The variable is not an array.
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
		c.symbolTable.DefineVarInput(p.Name.Value)
	}

	// Define and initialize the implicit return variable.
	returnSymbol := c.symbolTable.Define(method.Name.Value, false)
	c.emit(code.OpNull)
	c.emit(code.OpSetLocal, returnSymbol.Index)

	for _, v := range method.Vars {
		if err := c.Compile(v); err != nil {
			return nil, err
		}
	}

	if err := c.Compile(method.Body); err != nil {
		return nil, err
	}

	if c.lastInstructionIs(code.OpPop) {
		c.replaceLastPopWithReturn()
	}
	if !c.lastInstructionIs(code.OpReturnValue) {
		c.emit(code.OpReturn)
	}

	numLocals := c.symbolTable.numDefinitions
	instructions := c.leaveScope()

	return &object.CompiledFunction{
		Instructions:  instructions,
		NumLocals:     numLocals,
		NumParameters: len(method.VarInputs),
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
		c.emit(code.OpNull)
		c.emit(code.OpSetLocal, returnSymbol.Index)
		numParams = 0
	} else {
		c.symbolTable.Define("value", false) // Implicit 'value' parameter for setters
		numParams = 1
	}

	if err := c.Compile(body); err != nil {
		return nil, err
	}

	if c.lastInstructionIs(code.OpPop) {
		c.replaceLastPopWithReturn()
	}
	if !c.lastInstructionIs(code.OpReturnValue) {
		c.emit(code.OpReturn)
	}

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
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: node.Member.Value}))
	c.emit(code.OpIndex)
	return nil
}

func (c *Compiler) isPropertyAccess(expr ast.Expression) (bool, string) {
	if memberAccess, ok := expr.(*ast.MemberAccessExpression); ok {
		if c.currentFB != nil {
			// This is a simplification. A real implementation would need to know the type
			// of `target.Struct` to check its properties. For now, we assume access on `THIS`.
			if _, ok := memberAccess.Struct.(*ast.ThisExpression); ok {
				for _, prop := range c.currentFB.Properties {
					if prop.Name.Value == memberAccess.Member.Value {
						return true, prop.Name.Value
					}
				}
			}
		}
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
		if !resolved {
			return "", false
		}
		return symbol.TypeName, symbol.TypeName != ""
	case *ast.ThisExpression:
		if c.currentFB != nil {
			return c.currentFB.Name.Value, true
		}
		// Other complex cases like `getMotor().Speed` are hard to analyze statically
		// without a full type system and are not handled here.
	}
	return "", false
}

// resolveTypeNode finds the AST definition for a type, handling qualified names.
func (c *Compiler) resolveTypeNode(typeExpr ast.Expression) (ast.Node, bool) {
	fqn := c.flattenExpressionToString(typeExpr)
	// For now, we assume all lookups use the fully qualified name,
	// which `buildPouInfo` now registers. A more advanced resolver
	// would also check relative to the current namespace.
	node, ok := c.typeInfo[fqn]
	return node, ok
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

	// INTERNAL members: accessible only from within the same namespace.
	if accessSpecifier == "INTERNAL" {
		ownerNS := c.pouNamespaces[ownerDef.Name.Value]
		callerNS := c.currentNS
		if ownerNS != callerNS {
			return fmt.Errorf("member is internal")
		}
		return nil
	}

	return nil // Should not be reached
}

// findMethodOnFBChain recursively searches for a method definition starting from a given
// function block and traversing up its inheritance chain. It returns the declaration and its owner.
func (c *Compiler) findMethodOnFBChain(fbDef *ast.FunctionBlockDeclaration, methodName string) (*ast.MethodImplementation, *ast.FunctionBlockDeclaration) {
	if fbDef == nil {
		return nil, nil
	}

	// Search for the method in the current FB's body.
	if body, ok := fbDef.Body.(*ast.BlockStatement); ok {
		for _, stmt := range body.Statements {
			if method, isMethod := stmt.(*ast.MethodImplementation); isMethod {
				if method.Name != nil && method.Name.Value == methodName {
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
		if prop.Name.Value == propName {
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
					c.typeInfo[fqn] = decl
					c.pouNamespaces[fqn] = ns
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
			c.typeInfo[fqn] = pouNode
			c.pouNamespaces[fqn] = ns
		}
	}
	recursiveBuild(program.Statements, nil, "")
}

// flattenExpressionToString converts a potentially nested MemberAccessExpression into a single qualified string.
func (c *Compiler) flattenExpressionToString(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Value
	}
	if member, ok := expr.(*ast.MemberAccessExpression); ok {
		// Recursively flatten the struct part and append the member.
		return c.flattenExpressionToString(member.Struct) + "." + member.Member.Value
	}
	if ts, ok := expr.(*ast.TypeSpecifier); ok {
		return ts.Token.Literal
	}
	return "" // Should not happen for valid type names
}
