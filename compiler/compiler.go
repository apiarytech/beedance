package compiler

import (
	"beedance/ast"
	"beedance/code"
	"beedance/object"
	"fmt"
	"sort"
)

// CompiledProgram holds the separated bytecode for a program's one-time
// initialization and its cyclic execution logic.
type CompiledProgram struct {
	InitBytecode   *Bytecode
	CyclicBytecode *Bytecode
}

type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int
}

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
		constants:   []object.Object{},
		symbolTable: symbolTable,
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
	}
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

// CompileProgram orchestrates the compilation of a program declaration into
// separate initialization and cyclic execution bytecode.
func CompileProgram(node *ast.ProgramDeclaration) (*CompiledProgram, error) {
	// Create two separate compilers to handle the two passes.
	// They share the same symbol table to ensure variable indices are consistent.
	symbolTable := NewSymbolTable()
	constants := []object.Object{}

	initCompiler := NewWithState(symbolTable, constants)
	cyclicCompiler := NewWithState(symbolTable, constants)

	// --- Initialization Pass ---
	// The init compiler processes ALL var blocks to set up the initial state.
	for _, block := range node.VarGlobal {
		initCompiler.Compile(block)
	}
	for _, block := range node.VarExternal {
		initCompiler.Compile(block)
	}
	for _, block := range node.VarAccess {
		initCompiler.Compile(block)
	}
	for _, block := range node.VarTemp {
		initCompiler.Compile(block)
	}
	for _, decl := range node.Vars {
		initCompiler.Compile(decl)
	}

	// --- Cyclic Pass ---
	// The cyclic compiler only re-initializes VAR_TEMP blocks and then runs the body.
	for _, block := range node.VarTemp {
		cyclicCompiler.Compile(block)
	}

	if err := cyclicCompiler.Compile(node.Body); err != nil {
		return nil, err
	}

	return &CompiledProgram{
		InitBytecode:   initCompiler.Bytecode(),
		CyclicBytecode: cyclicCompiler.Bytecode(),
	}, nil
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ProgramDeclaration:
		// When compiling a program, we process all its variable blocks first
		// to populate the symbol table, then compile the body.
		for _, block := range node.VarGlobal {
			if err := c.Compile(block); err != nil {
				return err
			}
		}
		for _, block := range node.VarExternal {
			if err := c.Compile(block); err != nil {
				return err
			}
		}
		for _, block := range node.VarAccess {
			if err := c.Compile(block); err != nil {
				return err
			}
		}
		for _, block := range node.VarTemp {
			if err := c.Compile(block); err != nil {
				return err
			}
		}
		return c.Compile(node.Body)

	case *ast.FunctionDeclaration:
		// This is a statement that defines a function in the current scope.
		// First, define the function name in the current scope so it can be captured in a closure.
		symbol := c.symbolTable.Define(node.Name.Value)

		// Then, compile the function body itself.
		c.enterScope()
		c.symbolTable.DefineFunctionName(node.Name.Value) // For recursion

		// Define the function name as a local variable to hold the return value.
		c.symbolTable.Define(node.Name.Value)

		for _, p := range node.VarInputs {
			c.symbolTable.DefineVarInput(p.Name.Value)
		}

		for _, p := range node.VarOutputs {
			c.symbolTable.Define(p.Name.Value)
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

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
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

	case *ast.VarBlockDeclaration:
		for _, decl := range node.Declarations {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	case *ast.GlobalVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

	case *ast.ExternalVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

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

	case *ast.TempVarDeclaration:
		for _, decl := range node.Vars {
			err := c.Compile(decl)
			if err != nil {
				return err
			}
		}

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

	case *ast.IntegerLiteral:
		lint := &object.LInt{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(lint))

	case *ast.RealLiteral:
		lreal := &object.LReal{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(lreal))

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.PrefixExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

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

		// This is a workaround. The parser should wrap an if-expression used as a
		// statement in an ExpressionStatement, which would emit this OpPop.
		// It seems to do so for if-else, but not for if-without-else.
		if node.Alternative == nil {
			c.emit(code.OpPop)
		}

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.loadSymbol(symbol)

	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpArray, len(node.Elements))

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
			c.emit(code.OpReturn)
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

	case *ast.ReturnStatement:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	case *ast.CallExpression:
		err := c.Compile(node.Function)
		if err != nil {
			return err
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

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	new := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = new
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

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

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

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

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}
