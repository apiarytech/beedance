/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"beedance/code"
	"beedance/compiler"
	"beedance/object"
	"fmt"
	"math"
)

// StackSize defines the maximum number of objects that can be on the stack.
const StackSize = 2048

// GlobalsSize defines the maximum number of global variables.
const GlobalsSize = 65536

// MaxFrames defines the maximum number of frames (call stack depth).
const MaxFrames = 1024

// True is a singleton object representing the boolean true value.
var True = &object.Boolean{Value: true}

// False is a singleton object representing the boolean false value.
var False = &object.Boolean{Value: false}

// Null is a singleton object representing the null value.
var Null = &object.Null{}

// VM represents the virtual machine that executes beedance bytecode.
type VM struct {
	constants []object.Object

	stack []object.Object
	// sp always points to the next available slot on the stack. The top of the stack is stack[sp-1].
	sp int

	globals []object.Object

	frames      []*Frame
	framesIndex int

	builtins []*object.Builtin
}

// New creates a new VM instance with the given bytecode.
func New(bytecode *compiler.Bytecode) *VM {
	// The object.Builtins slice is guaranteed to be sorted by index
	// thanks to the FinalizeBuiltins function called in main.

	// Create a slice for the VM's built-ins that is explicitly sized
	// to the highest registered index. This is more robust than relying
	// on the length of the `object.Builtins` slice.
	maxIndex := -1
	for _, entry := range object.Builtins {
		if entry.Index > maxIndex {
			maxIndex = entry.Index
		}
	}

	builtins := make([]*object.Builtin, maxIndex+1)
	for _, entry := range object.Builtins {
		builtins[entry.Index] = entry.Builtin
	}

	return NewWithBuiltins(bytecode, builtins)
}

// NewWithBuiltins creates a new VM with a specific set of built-in functions.
// This is useful for testing to avoid dependency on global state.
func NewWithBuiltins(bytecode *compiler.Bytecode, builtins []*object.Builtin) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		globals:     make([]object.Object, GlobalsSize),
		frames:      frames,
		framesIndex: 1,
		builtins:    builtins,
	}
}

// NewWithGlobalsStore creates a new VM with a pre-populated global variable store.
func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

// Globals returns the global variable store of the VM.
func (vm *VM) Globals() []object.Object {
	return vm.globals
}

// LastPoppedStackElem returns the object at the top of the stack without removing it.
func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

// Run is the main execution loop of the VM. It fetches, decodes, and executes instructions.
func (vm *VM) Run() error {
	var ins code.Instructions
	var err error

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++ // Advance instruction pointer to the next opcode
		ip := vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op := code.Opcode(ins[ip])

		switch op {
		// OpConstant pushes a constant from the constant pool onto the stack.
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // Advance past operand
			err = vm.push(vm.constants[constIndex])

		// OpPop removes the top element from the stack.
		case code.OpPop:
			vm.pop()

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod, code.OpExponent,
			code.OpAnd, code.OpOr, code.OpXor, code.OpNand, code.OpNor:
			err = vm.executeBinaryOperation(op)

		case code.OpTrue:
			// OpTrue pushes the singleton True object onto the stack.
			err = vm.push(True)

		case code.OpFalse:
			// OpFalse pushes the singleton False object onto the stack.
			err = vm.push(False)

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpLessThan,
			// Comparison operators pop two values, compare them, and push a boolean result.
			code.OpGreaterThanOrEqual, code.OpLessThanOrEqual:
			err = vm.executeComparison(op)

		case code.OpBang:
			err = vm.executeBangOperator()

		case code.OpMinus:
			err = vm.executeMinusOperator()

		case code.OpJump:
			// OpJump unconditionally sets the instruction pointer to a new location.
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1 // Set IP to the instruction *before* the target, so next loop iteration increments to target

		case code.OpJumpNotTruthy:
			// OpJumpNotTruthy pops a value from the stack and jumps if it's not truthy.
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2 // Advance past operand
			condition := vm.pop()
			if !object.IsTruthy(condition) {
				vm.currentFrame().ip = pos - 1 // Set IP to the instruction *before* the target, so next loop iteration increments to target
			}

		case code.OpNull:
			// OpNull pushes the singleton Null object onto the stack.
			err = vm.push(Null)

		case code.OpSetGlobal:
			// OpSetGlobal pops a value from the stack and stores it in the globals slice.
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // Advance past operand
			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			// OpGetGlobal retrieves a value from the globals slice and pushes it onto the stack.
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // Advance past operand
			err = vm.push(vm.globals[globalIndex])

		// OpArray creates an array object from a number of elements on the stack.
		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements
			err = vm.push(array)

		case code.OpHash:
			// OpHash creates a hash object from key-value pairs on the stack.
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			var hash object.Object
			hash, err = vm.buildHash(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements
			if err == nil {
				err = vm.push(hash)
			}

		case code.OpIndex:
			// OpIndex retrieves an element from an array or hash.
			index := vm.pop()
			left := vm.pop()
			err = vm.executeIndexExpression(left, index)

		case code.OpCall:
			// OpCall executes a function or closure call.
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1 // Advance past operand
			err = vm.executeCall(int(numArgs))

		case code.OpReturnValue:
			// OpReturnValue returns a value from a function.
			returnValue := vm.pop()

			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err = vm.push(returnValue)

		case code.OpReturn:
			// OpReturn returns Null from a function.
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err = vm.push(Null)

		case code.OpSetLocal:
			// OpSetLocal pops a value and sets it as a local variable in the current frame.
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			// OpGetLocal retrieves a local variable from the current frame and pushes it onto the stack.
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			err = vm.push(vm.stack[frame.basePointer+int(localIndex)])

		case code.OpGetBuiltin:
			// OpGetBuiltin retrieves a built-in function and pushes it onto the stack.
			// The operand is a 16-bit index to support more than 256 built-ins.
			builtinIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // Advance past the 2-byte operand

			if int(builtinIndex) >= len(vm.builtins) {
				return fmt.Errorf("invalid builtin index: %d", builtinIndex)
			}
			definition := vm.builtins[builtinIndex]
			err = vm.push(definition)

		case code.OpClosure:
			// OpClosure creates a closure from a compiled function and free variables.
			constIndex := code.ReadUint16(ins[ip+1:])
			numFree := code.ReadUint8(ins[ip+3:])
			vm.currentFrame().ip += 3 // Advance past operands
			err = vm.pushClosure(int(constIndex), int(numFree))

		case code.OpGetFree:
			// OpGetFree retrieves a free variable from the current closure.
			freeIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			currentClosure := vm.currentFrame().cl
			err = vm.push(currentClosure.Free[freeIndex])

		case code.OpCurrentClosure:
			// OpCurrentClosure pushes the current closure onto the stack, for recursion.
			currentClosure := vm.currentFrame().cl
			err = vm.push(currentClosure)

		case code.OpMakeNamedArg:
			nameIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			nameObj, ok := vm.constants[nameIndex].(*object.String)
			if !ok {
				return fmt.Errorf("operand to OpMakeNamedArg must be a string constant")
			}

			value := vm.pop()
			namedArg := &object.NamedArgument{Name: nameObj.Value, Value: value}
			err = vm.push(namedArg)

		case code.OpReturnValueMulti:
			// This opcode is used for functions with VAR_OUTPUTs.
			// It constructs a hash of all outputs and returns that.
			primaryReturnValue := vm.pop()
			frame := vm.popFrame()

			hash, err := vm.buildOutputHash(primaryReturnValue, frame)
			if err != nil {
				return err
			}
			vm.sp = frame.basePointer - 1
			err = vm.push(hash)
		case code.OpDup:
			// OpDup duplicates the top element of the stack.
			err = vm.push(vm.stack[vm.sp-1])
		case code.OpSwap:
			// OpSwap swaps the top two elements of the stack.
			vm.stack[vm.sp-1], vm.stack[vm.sp-2] = vm.stack[vm.sp-2], vm.stack[vm.sp-1]
		case code.OpSuperIndex:
			// OpSuperIndex finds a method on a parent class. It expects the instance ('this')
			// and the method name on the stack. It uses the currently executing closure
			// to determine the correct parent class for the lookup.
			methodName := vm.pop()
			instance := vm.pop()
			err = vm.executeSuperIndex(instance, methodName)
			if err == nil {
				err = vm.push(instance)
			}
		}
		if err != nil {
			return err
		}
	}

	return err
}

// push adds an object to the top of the stack.
func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

// pop removes and returns the object from the top of the stack.
func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

// executeBinaryBooleanOperation performs a binary operation on two boolean objects.
func (vm *VM) executeBinaryBooleanOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue := left.(*object.Boolean).Value
	rightValue := right.(*object.Boolean).Value

	var result bool
	switch op {
	case code.OpAnd:
		result = leftValue && rightValue
	case code.OpOr:
		result = leftValue || rightValue
	case code.OpXor:
		result = leftValue != rightValue
	case code.OpNand:
		result = !(leftValue && rightValue)
	case code.OpNor:
		result = !(leftValue || rightValue)
	default:
		return fmt.Errorf("unknown boolean operator: %d", op)
	}
	return vm.push(nativeBoolToBooleanObject(result))
}

// executeBinaryOperation dispatches to the correct binary operation based on operand types.
func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.BOOLEAN_OBJ && rightType == object.BOOLEAN_OBJ:
		return vm.executeBinaryBooleanOperation(op, left, right)
	case isInteger(left) && isInteger(right):
		return vm.executeBinaryIntegerOperation(op, left, right)
	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	default:
		return fmt.Errorf("unsupported types for binary operation: %s %s",
			leftType, rightType)
	}
}

// executeBinaryIntegerOperation performs a binary operation on two integer objects.
func (vm *VM) executeBinaryIntegerOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue, _, ok := object.GetIntegerObjectValue(left)
	if !ok {
		return fmt.Errorf("left operand is not an integer: %s", left.Type())
	}
	rightValue, _, ok := object.GetIntegerObjectValue(right)
	if !ok {
		return fmt.Errorf("right operand is not an integer: %s", right.Type())
	}

	var result int64 // Keep as int64 for now, or change to int32 if object.Integer.Value is changed

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		if rightValue == 0 {
			return fmt.Errorf("division by zero")
		}
		result = leftValue / rightValue
	case code.OpMod:
		if rightValue == 0 {
			return fmt.Errorf("division by zero")
		}
		result = leftValue % rightValue
	case code.OpExponent:
		result = int64(math.Pow(float64(leftValue), float64(rightValue)))
	case code.OpAnd:
		result = leftValue & rightValue
	case code.OpOr:
		result = leftValue | rightValue
	case code.OpXor:
		result = leftValue ^ rightValue
	case code.OpNand:
		result = ^(leftValue & rightValue)
	case code.OpNor:
		result = ^(leftValue | rightValue)
	default:
		return fmt.Errorf("unknown integer operator: %d", op)
	}

	return vm.push(&object.LInt{Value: result})
}

// executeComparison dispatches to the correct comparison operation based on operand types.
func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if isInteger(left) && isInteger(right) {
		return vm.executeIntegerComparison(op, left, right)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))
	default:
		return fmt.Errorf("unknown operator: %d (%s %s)",
			op, left.Type(), right.Type())
	}
}

// executeIntegerComparison performs a comparison operation on two integer objects.
func (vm *VM) executeIntegerComparison(
	op code.Opcode,
	left, right object.Object,
) error {
	leftValue, _, ok := object.GetIntegerObjectValue(left)
	if !ok {
		return fmt.Errorf("left operand is not an integer: %s", left.Type())
	}
	rightValue, _, ok := object.GetIntegerObjectValue(right)
	if !ok {
		return fmt.Errorf("right operand is not an integer: %s", right.Type())
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue == rightValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue != rightValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(leftValue < rightValue))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue >= rightValue))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(leftValue <= rightValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

// executeBangOperator performs a logical NOT operation on the top of the stack.
func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	// Use a type switch to correctly handle different object types.
	switch operand := operand.(type) {
	case *object.Boolean:
		if operand.Value {
			return vm.push(False)
		}
		return vm.push(True)
	case *object.Null:
		return vm.push(True)
	case *object.BitString:
		// Handle bitwise NOT for bitstrings
		var mask uint64 = math.MaxUint64
		if operand.Width < 64 {
			mask = (1 << operand.Width) - 1
		}
		invertedValue := ^operand.Value & mask
		return vm.push(&object.BitString{Value: invertedValue, Width: operand.Width})
	default:
		// For any other type (like integers), the IEC standard implies that
		// the result of a logical NOT is boolean. For non-boolean inputs,
		// this behavior can be considered as resulting in FALSE.
		return vm.push(False)
	}
}

// executeMinusOperator performs a negation operation on the top of the stack.
func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()

	// The `isInteger` check allows multiple integer types, so we must handle them.
	switch o := operand.(type) {
	case *object.LInt:
		return vm.push(&object.LInt{Value: -o.Value})
	case *object.Real:
		return vm.push(&object.Real{Value: -o.Value})
	case *object.LReal:
		return vm.push(&object.LReal{Value: -o.Value})
		// In a full implementation, other integer types (SINT, INT, DINT) would be handled here.
	default:
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}
}

// executeBinaryStringOperation performs a binary operation on two string objects.
func (vm *VM) executeBinaryStringOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

// buildArray creates an array object from elements on the stack.
func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)

	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

// buildHash creates a hash object from key-value pairs on the stack.
func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		pair := object.HashPair{Key: key, Value: value}

		hashKey, ok := key.(object.Hashable) // cspell:disable-line
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

// executeIndexExpression executes an index operation on an array or hash.
func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && isInteger(index):
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

// executeArrayIndex executes an index operation on an array.
func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i, _, ok := object.GetIntegerObjectValue(index)
	if !ok {
		return fmt.Errorf("array index must be an integer, got %s", index.Type())
	}
	max := int64(len(arrayObject.Elements) - 1) // Keep as int64 for comparison with int64 `i`

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

// executeHashIndex executes an index operation on a hash.
func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable) // cspell:disable-line
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

// currentFrame returns the currently executing frame.
func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

// pushFrame pushes a new frame onto the frame stack.
func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

// popFrame pops the current frame from the frame stack.
func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

// executeCall executes a function call.
func (vm *VM) executeCall(numArgs int) error {
	callee := vm.stack[vm.sp-1-numArgs]
	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	default:
		return fmt.Errorf("calling non-closure and non-builtin")
	}
}

// callClosure handles the logic for calling a closure.
func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	// The callee (closure object) is at vm.stack[vm.sp - 1 - numArgs].
	// The arguments are at vm.stack[vm.sp - numArgs] to vm.stack[vm.sp - 1].

	// Check for argument count mismatch.
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d",
			cl.Fn.NumParameters, numArgs)
	}

	// The basePointer for the new frame will be where the arguments start on the stack.
	basePointer := vm.sp - numArgs

	// Pop the callee (closure object) from the stack.
	// This effectively removes the closure object itself, leaving only the arguments.
	vm.sp = basePointer

	frame := NewFrame(cl, basePointer)
	vm.pushFrame(frame)

	// Initialize local variables *after* the parameters to Null.
	// The parameters themselves are already on the stack (at basePointer to basePointer + numArgs - 1).
	// These slots correspond to the first `cl.Fn.NumParameters` locals.
	// So, we start initializing from `basePointer + cl.Fn.NumParameters`.
	for i := cl.Fn.NumParameters; i < cl.Fn.NumLocals; i++ {
		vm.stack[frame.basePointer+i] = Null
	}

	// Handle named arguments.
	// Create a map of parameter names to their index for quick lookup.
	paramIndexMap := make(map[string]int)
	for i, name := range cl.Fn.ParameterNames {
		paramIndexMap[name] = i
	}

	// Iterate through the arguments that were passed on the stack.
	for i := 0; i < numArgs; i++ {
		arg := vm.stack[basePointer+i] // Get the argument from its current position
		if namedArg, ok := arg.(*object.NamedArgument); ok {
			idx, exists := paramIndexMap[namedArg.Name]
			if !exists {
				return fmt.Errorf("unknown named argument: %s", namedArg.Name)
			}
			vm.stack[frame.basePointer+idx] = namedArg.Value
		}
	}

	// Set the new stack pointer past all locals for the new frame.
	vm.sp = frame.basePointer + cl.Fn.NumLocals

	return nil
}

// callBuiltin handles the logic for calling a built-in function.
func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]

	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	return nil
}

// pushClosure creates a new closure and pushes it onto the stack.
func (vm *VM) pushClosure(constIndex, numFree int) error {
	constant := vm.constants[constIndex]
	function, ok := constant.(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("not a function: %+v", constant)
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp = vm.sp - numFree

	closure := &object.Closure{Fn: function, Free: free}
	return vm.push(closure)
}

// buildOutputHash creates a hash object from a function's primary return value
// and its VAR_OUTPUT parameters stored in a frame's local variables.
func (vm *VM) buildOutputHash(primaryReturn object.Object, frame *Frame) (*object.Hash, error) {
	cl := frame.cl
	numOutputs := len(cl.Fn.OutputNames)
	pairs := make(map[object.HashKey]object.HashPair, numOutputs+1)

	// Add primary return value
	key := &object.String{Value: "__return__"}
	pairs[key.HashKey()] = object.HashPair{Key: key, Value: primaryReturn}

	// Ensure the compiler provided consistent data.
	if len(cl.Fn.OutputNames) != len(cl.Fn.OutputIndices) {
		return nil, fmt.Errorf("internal vm error: mismatch between output names and indices")
	}

	// Add VAR_OUTPUT values by looking them up in the stack frame using the indices
	// provided by the compiler.
	for i, name := range cl.Fn.OutputNames {
		index := cl.Fn.OutputIndices[i]
		val := vm.stack[frame.basePointer+index]
		key := &object.String{Value: name}
		pairs[key.HashKey()] = object.HashPair{Key: key, Value: val}
	}
	return &object.Hash{Pairs: pairs}, nil
}

// nativeBoolToBooleanObject returns a singleton boolean object for a given native boolean value.
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

// isInteger checks if an object is one of the integer types.
func isInteger(obj object.Object) bool {
	switch obj.Type() {
	case object.SINT_OBJ, object.INT_OBJ, object.DINT_OBJ, object.LINT_OBJ,
		object.USINT_OBJ, object.UINT_OBJ, object.UDINT_OBJ, object.ULINT_OBJ:
		return true
	default:
		return false
	}
}

// executeSuperIndex finds a method on a parent class and pushes the corresponding closure onto the stack.
// It is used to implement `SUPER^.Method()`. It consumes the instance and method name from the stack
// and pushes the parent's method closure. The instance is pushed back on by the caller (`Run` loop).
func (vm *VM) executeSuperIndex(instance, methodName object.Object) error {
	methodNameStr, ok := methodName.(*object.String)
	if !ok {
		return fmt.Errorf("super index method name must be a string, got %T", methodName)
	}

	instanceHash, ok := instance.(*object.Hash)
	if !ok {
		return fmt.Errorf("base of super call must be a function block instance (hash), got %T", instance)
	}

	// The currently executing method's closure is in the current frame.
	currentClosure := vm.currentFrame().cl

	var classHash *object.Hash
	// An instance must have a `__class__` field pointing to its class hash.
	// This is a design assumption for the VM's OOP model.
	classHashKey := (&object.String{Value: "__class__"}).HashKey()
	classPair, ok := instanceHash.Pairs[classHashKey]
	if !ok {
		// Fallback for tests where an instance might be the class hash itself.
		classHash = instanceHash
	} else {
		classHash, ok = classPair.Value.(*object.Hash)
		if !ok {
			return fmt.Errorf("internal VM error: __class__ is not a hash")
		}
	}

	// Find which class in the hierarchy owns the currently executing method.
	ownerClass, err := vm.findClosureOwner(classHash, currentClosure)
	if err != nil {
		return err
	}

	// Get the parent of that owner class.
	parentHashKey := (&object.String{Value: "__parent__"}).HashKey()
	parentPair, ok := ownerClass.Pairs[parentHashKey]
	if !ok {
		return fmt.Errorf("super call on a class with no parent")
	}
	parentHash, ok := parentPair.Value.(*object.Hash)
	if !ok {
		return fmt.Errorf("internal VM error: parent class is not a hash")
	}

	// Find the method in the parent's hierarchy.
	method, err := vm.findMethodInHierarchy(parentHash, methodNameStr)
	if err != nil {
		return err
	}

	// Push the found parent method closure onto the stack.
	return vm.push(method)
}

// findClosureOwner recursively searches the class hierarchy starting from `class`
// to find which class hash contains the given closure `cl`.
func (vm *VM) findClosureOwner(class *object.Hash, cl *object.Closure) (*object.Hash, error) {
	for _, pair := range class.Pairs {
		if methodClosure, ok := pair.Value.(*object.Closure); ok && methodClosure == cl {
			return class, nil
		}
	}

	parentHashKey := (&object.String{Value: "__parent__"}).HashKey()
	if parentPair, ok := class.Pairs[parentHashKey]; ok {
		if parentHash, ok := parentPair.Value.(*object.Hash); ok {
			return vm.findClosureOwner(parentHash, cl)
		}
	}

	return nil, fmt.Errorf("internal VM error: closure owner not found in class hierarchy")
}

// findMethodInHierarchy recursively searches the class hierarchy starting from `class`
// to find a method by name.
func (vm *VM) findMethodInHierarchy(class *object.Hash, methodName *object.String) (object.Object, error) {
	if methodPair, ok := class.Pairs[methodName.HashKey()]; ok {
		return methodPair.Value, nil
	}

	parentHashKey := (&object.String{Value: "__parent__"}).HashKey()
	if parentPair, ok := class.Pairs[parentHashKey]; ok {
		if parentHash, ok := parentPair.Value.(*object.Hash); ok {
			return vm.findMethodInHierarchy(parentHash, methodName)
		}
	}

	return nil, fmt.Errorf("method '%s' not found in class hierarchy", methodName.Value)
}
