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
	"errors"
	"fmt"
	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/token"
	"math"
	"strings"
)

// StackSize defines the maximum number of objects that can be on the stack.
const StackSize = 2048

// GlobalsSize is the default number of global variable slots of a VM made by
// New, the most a global's 16-bit index can address. It takes 512 KB on a
// 32-bit target and 1 MB on a 64-bit one; on a microcontroller, pass a
// smaller size with WithGlobalsSize.
const GlobalsSize = 65536

// MaxFrames defines the maximum number of frames (call stack depth).
const MaxFrames = 1024

// True is a singleton object representing the boolean true value.
var True = &object.Boolean{Value: true}

// False is a singleton object representing the boolean false value.
var False = &object.Boolean{Value: false}

// Null is a singleton object representing the null value.
var Null = object.NULL

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

	// io is the I/O image: the values of located variables (e.g. `x AT %IX0.0`)
	// and external access paths, keyed by address. A host reads outputs from
	// it and writes inputs to it between runs; see IO.
	io map[string]object.Object

	// budget bounds the instructions of one run (SetBudget); used counts
	// them since the last Reset.
	budget, used int64

	// hook is a debugger's (SetHook); resumed is set when it paused the
	// run, so that the instruction it paused before runs on the next Run.
	hook    Hook
	resumed bool
}

// ErrBudget is returned by Run when a run executes more instructions than
// its budget: a program caught in a loop is stopped instead of running for
// ever.
var ErrBudget = errors.New("vm: execution budget exceeded")

// SetBudget bounds the instructions of each run (from one Reset to the
// next) to n; 0 or less means no bound. A host running a program every scan
// sets one, so a scan that loops for ever fails instead of stalling it.
func (vm *VM) SetBudget(n int64) { vm.budget = n }

// Option configures a VM made by New, NewWithBuiltins or
// NewWithGlobalsStore.
type Option func(*options)

type options struct {
	globalsSize int
	stackSize   int
	maxFrames   int
}

// WithStackSize sets the VM's operand stack to n slots instead of
// StackSize; a run that needs more fails with "stack overflow". On a
// microcontroller a few hundred slots do for most programs, and save most
// of the 8 or 16 KB the default takes. n must be positive.
func WithStackSize(n int) Option {
	return func(o *options) { o.stackSize = n }
}

// WithMaxFrames sets the VM's call depth to n frames instead of MaxFrames;
// a run that calls deeper fails with "call stack overflow". n must be
// positive.
func WithMaxFrames(n int) Option {
	return func(o *options) { o.maxFrames = n }
}

// WithGlobalsSize sets the number of global variable slots to n instead of
// GlobalsSize. A program can use globals 0 to n-1; reading, writing or taking
// a reference to a higher one fails the run. On a microcontroller, n is the
// program's number of globals (see GlobalsNeeded), since the default does
// not fit in its RAM. n must not be negative.
func WithGlobalsSize(n int) Option {
	return func(o *options) { o.globalsSize = n }
}

// GlobalsNeeded returns the number of global slots bytecode uses: one more
// than the highest global its instructions, or those of its compiled
// functions, read, write or take a reference to. It is the smallest
// WithGlobalsSize that runs bytecode. For bytecode it cannot decode, it
// returns GlobalsSize.
func GlobalsNeeded(bytecode *compiler.Bytecode) int {
	n := globalsNeeded(bytecode.Instructions)
	for _, c := range bytecode.Constants {
		if fn, ok := c.(*object.CompiledFunction); ok {
			n = max(n, globalsNeeded(fn.Instructions))
		}
	}
	return n
}

// globalsNeeded returns one more than the highest global ins refers to.
func globalsNeeded(ins code.Instructions) int {
	n := 0
	for ip := 0; ip < len(ins); {
		def, err := code.Lookup(ins[ip])
		if err != nil {
			return GlobalsSize
		}
		width := 0
		for _, w := range def.OperandWidths {
			width += w
		}
		if ip+1+width > len(ins) {
			return GlobalsSize
		}
		operands, _ := code.ReadOperands(def, ins[ip+1:])
		switch code.Opcode(ins[ip]) {
		case code.OpGetGlobal, code.OpSetGlobal:
			n = max(n, operands[0]+1)
		case code.OpRef:
			if operands[0] == code.RefGlobal {
				n = max(n, operands[1]+1)
			}
		}
		ip += 1 + width
	}
	return n
}

// optionsFor applies opts to the defaults.
func optionsFor(opts []Option) options {
	o := options{globalsSize: GlobalsSize, stackSize: StackSize, maxFrames: MaxFrames}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// New creates a new VM instance with the given bytecode.
func New(bytecode *compiler.Bytecode, opts ...Option) *VM {
	o := optionsFor(opts)
	return newVM(bytecode, defaultBuiltins(), make([]object.Object, o.globalsSize), o)
}

// defaultBuiltins returns a VM's own copy of the registered built-ins, indexed
// by built-in index, so that SetBuiltin changes only that VM.
func defaultBuiltins() []*object.Builtin {
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
	return builtins
}

// NewWithBuiltins creates a new VM with a specific set of built-in functions.
// This is useful for testing to avoid dependency on global state.
func NewWithBuiltins(bytecode *compiler.Bytecode, builtins []*object.Builtin, opts ...Option) *VM {
	o := optionsFor(opts)
	return newVM(bytecode, builtins, make([]object.Object, o.globalsSize), o)
}

// NewWithGlobalsStore creates a new VM with a pre-populated global variable
// store, which the VM uses (and changes) in place. WithGlobalsSize does not
// apply: s is the store.
func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object, opts ...Option) *VM {
	return newVM(bytecode, defaultBuiltins(), s, optionsFor(opts))
}

func newVM(bytecode *compiler.Bytecode, builtins []*object.Builtin, globals []object.Object, o options) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions, Lines: bytecode.Lines}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, max(o.maxFrames, 1))
	frames[0] = mainFrame

	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]object.Object, max(o.stackSize, 1)),
		sp:          0,
		globals:     globals,
		frames:      frames,
		framesIndex: 1,
		builtins:    builtins,
		io:          make(map[string]object.Object),
	}
}

// Reset prepares the VM to run its bytecode again, as the next scan of a
// program: the stack and the call frames start over, while the globals and
// the I/O image keep their values. A host that runs a program every scan
// calls Run, Reset, Run, ... on one VM instead of building a VM per scan.
func (vm *VM) Reset() {
	vm.used = 0
	vm.resumed = false
	clear(vm.stack[:vm.sp])
	vm.sp = 0
	for _, f := range vm.frames[1:vm.framesIndex] {
		f.returned = true // a run that failed in a call ends the call
	}
	clear(vm.frames[1:vm.framesIndex])
	vm.frames[0].ip = -1
	vm.framesIndex = 1
}

// SetBuiltin replaces the built-in function at index for this VM only, e.g.
// to give a program a clock of its host's choosing (see stdlib.ClockBuiltins).
func (vm *VM) SetBuiltin(index int, b *object.Builtin) {
	if index >= len(vm.builtins) {
		grown := make([]*object.Builtin, index+1)
		copy(grown, vm.builtins)
		vm.builtins = grown
	}
	vm.builtins[index] = b
}

// IO returns the VM's I/O image: the values of located variables and external
// access paths, keyed by address (e.g. "%IX0.0"). A host sets inputs in it
// before Run and reads outputs from it afterwards.
func (vm *VM) IO() map[string]object.Object {
	return vm.io
}

// ShareIO makes image the VM's I/O image, which several VMs then share: the
// programs of a configuration have one process image (see package
// vmconfig). Values the VM's image held are kept in image unless it has
// its own.
func (vm *VM) ShareIO(image map[string]object.Object) {
	for addr, v := range vm.io {
		if _, ok := image[addr]; !ok {
			image[addr] = v
		}
	}
	vm.io = image
}

// externalAddress reads the 2-byte constant index operand of an external
// access instruction at ip and returns the address string it names.
func (vm *VM) externalAddress(ins code.Instructions, ip int) (string, error) {
	constIndex := code.ReadUint16(ins[ip+1:])
	vm.currentFrame().ip += 2
	address, ok := vm.constants[constIndex].(*object.String)
	if !ok {
		return "", fmt.Errorf("external variable operand must be an address string constant, got %T", vm.constants[constIndex])
	}
	return address.Value, nil
}

// Globals returns the global variable store of the VM.
func (vm *VM) Globals() []object.Object {
	return vm.globals
}

// checkGlobal reports an error if the VM has no global slot index, as when
// it was made with a WithGlobalsSize too small for its program.
func (vm *VM) checkGlobal(index int) error {
	if index >= len(vm.globals) {
		return fmt.Errorf("global %d out of range: the VM has %d global slots (see WithGlobalsSize)", index, len(vm.globals))
	}
	return nil
}

// LastPoppedStackElem returns the object at the top of the stack without removing it.
func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

// Run is the main execution loop of the VM. It fetches, decodes, and executes instructions.
// Malformed bytecode that would crash the host process, such as popping an
// empty stack, is reported as an error instead.
func (vm *VM) Run() (runErr error) {
	defer func() {
		if r := recover(); r != nil {
			runErr = fmt.Errorf("vm runtime error: %v", r)
		}
	}()

	var ins code.Instructions
	var err error

	// cur is the frame the instruction runs in, read once per instruction:
	// an instruction that pushes or pops a frame does so after its last use
	// of cur, and the next instruction reads the new frame.
	for {
		cur := vm.frames[vm.framesIndex-1]
		ins = cur.cl.Fn.Instructions
		if cur.ip >= len(ins)-1 {
			break
		}
		if vm.hook != nil {
			if vm.resumed {
				vm.resumed = false
			} else if vm.hook(cur.cl.Fn, cur.ip+1, vm.framesIndex) {
				vm.resumed = true
				return ErrPaused
			}
		}
		if vm.budget > 0 {
			if vm.used++; vm.used > vm.budget {
				return ErrBudget
			}
		}
		cur.ip++ // Advance instruction pointer to the next opcode
		ip := cur.ip
		op := code.Opcode(ins[ip])

		switch op {
		// OpConstant pushes a constant from the constant pool onto the stack.
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			cur.ip += 2 // Advance past operand
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
			cur.ip = pos - 1 // Set IP to the instruction *before* the target, so next loop iteration increments to target

		case code.OpJumpNotTruthy:
			// OpJumpNotTruthy pops a value from the stack and jumps if it's not truthy.
			pos := int(code.ReadUint16(ins[ip+1:]))
			cur.ip += 2 // Advance past operand
			condition := vm.pop()
			if !object.IsTruthy(condition) {
				cur.ip = pos - 1 // Set IP to the instruction *before* the target, so next loop iteration increments to target
			}

		case code.OpNull:
			// OpNull pushes the singleton Null object onto the stack.
			err = vm.push(Null)

		case code.OpSetGlobal:
			// OpSetGlobal pops a value from the stack and stores it in the globals slice.
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			cur.ip += 2 // Advance past operand
			if err = vm.checkGlobal(globalIndex); err == nil {
				vm.globals[globalIndex] = vm.pop()
			}

		case code.OpGetGlobal:
			// OpGetGlobal retrieves a value from the globals slice and pushes it onto the stack.
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			cur.ip += 2 // Advance past operand
			if err = vm.checkGlobal(globalIndex); err != nil {
				break
			}
			value := vm.globals[globalIndex]
			if value == nil {
				// A global that was defined but never assigned reads as NULL. This
				// happens when a declaration's code did not run, e.g. after a
				// failed REPL line, and would otherwise crash later instructions.
				value = Null
			}
			err = vm.push(value)

		// OpArray creates an array object from a number of elements on the stack.
		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			cur.ip += 2
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements
			err = vm.push(array)

		case code.OpArrayBounds:
			// Sets the declared lower bounds of the array on top of the stack.
			boundsIndex := code.ReadUint16(ins[ip+1:])
			cur.ip += 2
			bounds, ok := vm.constants[boundsIndex].(*object.Array)
			if !ok {
				return fmt.Errorf("internal VM error: array bounds constant is %T", vm.constants[boundsIndex])
			}
			setArrayBounds(vm.stack[vm.sp-1], bounds.Elements)

		case code.OpCopy:
			// Arrays and structures are assigned and passed by value.
			vm.stack[vm.sp-1] = object.CopyValue(vm.stack[vm.sp-1])

		case code.OpWrapInt:
			// An integer assigned to a narrow integer type wraps into its range.
			wrap := int(code.ReadUint8(ins[ip+1:]))
			cur.ip += 1
			if n, _, isInt := object.GetIntegerObjectValue(vm.stack[vm.sp-1]); isInt {
				if w := object.WrapIntegerCode(n, wrap); w != n {
					vm.stack[vm.sp-1] = &object.LInt{Value: w}
				}
			}

		case code.OpGetBit:
			// A bit of an integer or bit string, e.g. flags.3.
			n := int64(code.ReadUint8(ins[ip+1:]))
			typeName := vm.bitTypeName(code.ReadUint16(ins[ip+2:]))
			cur.ip += 3
			bit, bitErr := object.GetBitAs(vm.stack[vm.sp-1], n, typeName)
			if bitErr != nil {
				return bitErr
			}
			vm.stack[vm.sp-1] = nativeBoolToBooleanObject(bit)

		case code.OpSetBit:
			// The integer or bit string with one bit set: flags.3 := value.
			n := int64(code.ReadUint8(ins[ip+1:]))
			typeName := vm.bitTypeName(code.ReadUint16(ins[ip+2:]))
			cur.ip += 3
			value := vm.pop()
			updated, bitErr := object.SetBitAs(vm.stack[vm.sp-1], n, object.IsTruthy(value), typeName)
			if bitErr != nil {
				return bitErr
			}
			vm.stack[vm.sp-1] = updated

		case code.OpHash:
			// OpHash creates a hash object from key-value pairs on the stack.
			numElements := int(code.ReadUint16(ins[ip+1:]))
			cur.ip += 2
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

		case code.OpSetIndex:
			val := vm.pop()
			index := vm.pop()
			left := vm.pop()
			err = vm.executeSetIndex(left, index, val)

		case code.OpCall:
			// OpCall executes a function or closure call.
			numArgs := code.ReadUint8(ins[ip+1:])
			cur.ip += 1 // Advance past operand
			err = vm.executeCall(int(numArgs))

		case code.OpReturnValue:
			// OpReturnValue returns a value from a function. In the main
			// frame (a RETURN in a program's body) it ends the run, as the
			// end of the body does, with the value on the stack.
			if vm.framesIndex == 1 {
				return nil
			}
			returnValue := vm.pop()

			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err = vm.push(returnValue)

		case code.OpReturn:
			// OpReturn returns Null from a function. In the main frame (IL
			// RET in a program's body) it ends the run, as the end of the
			// body does.
			if vm.framesIndex == 1 {
				return vm.push(Null)
			}
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err = vm.push(Null)

		case code.OpSetLocal:
			// OpSetLocal pops a value and sets it as a local variable in the current frame.
			localIndex := code.ReadUint8(ins[ip+1:])
			cur.ip += 1
			frame := cur
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			// OpGetLocal retrieves a local variable from the current frame and pushes it onto the stack.
			localIndex := code.ReadUint8(ins[ip+1:])
			cur.ip += 1
			frame := cur
			err = vm.push(vm.stack[frame.basePointer+int(localIndex)])

		case code.OpGetBuiltin:
			// OpGetBuiltin retrieves a built-in function and pushes it onto the stack.
			// The operand is a 16-bit index to support more than 256 built-ins.
			builtinIndex := code.ReadUint16(ins[ip+1:])
			cur.ip += 2 // Advance past the 2-byte operand

			if int(builtinIndex) >= len(vm.builtins) {
				return fmt.Errorf("invalid builtin index: %d", builtinIndex)
			}
			definition := vm.builtins[builtinIndex]
			err = vm.push(definition)

		case code.OpClosure:
			// OpClosure creates a closure from a compiled function and free variables.
			constIndex := code.ReadUint16(ins[ip+1:])
			numFree := code.ReadUint8(ins[ip+3:])
			cur.ip += 3 // Advance past operands
			err = vm.pushClosure(int(constIndex), int(numFree))

		case code.OpGetFree:
			// OpGetFree retrieves a free variable from the current closure.
			freeIndex := code.ReadUint8(ins[ip+1:])
			cur.ip += 1
			currentClosure := cur.cl
			err = vm.push(currentClosure.Free[freeIndex])

		case code.OpCurrentClosure:
			// OpCurrentClosure pushes the current closure onto the stack, for recursion.
			currentClosure := cur.cl
			err = vm.push(currentClosure)

		case code.OpMakeNamedArg:
			nameIndex := code.ReadUint16(ins[ip+1:])
			cur.ip += 2

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

			var hash *object.Hash
			if hash, err = vm.buildOutputHash(primaryReturnValue, frame); err != nil {
				return err
			}
			vm.sp = frame.basePointer - 1
			err = vm.push(hash) // checked after the switch, like every opcode's
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
			// It leaves the parent's method bound to the instance, so the call
			// that follows passes the instance as THIS.
			methodName := vm.pop()
			instance := vm.pop()
			err = vm.executeSuperIndex(instance, methodName)

		case code.OpGetExternal:
			// Reads a located variable or access path from the I/O image. The
			// operand is the constant holding its address, e.g. "%IX0.0".
			address, addrErr := vm.externalAddress(ins, ip)
			if addrErr != nil {
				return addrErr
			}
			value, ok := vm.io[address]
			if !ok {
				// A direct variable the host has not set yet reads as its
				// type's default, as a located variable starts.
				value = directZero(address)
			}
			err = vm.push(value)

		case code.OpSetExternal:
			// Writes a located variable or access path to the I/O image.
			address, addrErr := vm.externalAddress(ins, ip)
			if addrErr != nil {
				return addrErr
			}
			vm.io[address] = vm.pop()

		case code.OpInitExternal:
			// Gives a located variable its starting value, unless the host
			// has set its address in the I/O image already.
			address, addrErr := vm.externalAddress(ins, ip)
			if addrErr != nil {
				return addrErr
			}
			value := vm.pop()
			if _, set := vm.io[address]; !set {
				vm.io[address] = value
			}
		case code.OpRef, code.OpRefIndex, code.OpDeref, code.OpSetDeref:
			// REF/ADR, r^ and r^ := value; see references.go.
			err = vm.executeReference(op, ins, ip)

		case code.OpSetFree:
			// Assigns to a variable captured by the current closure.
			freeIndex := code.ReadUint8(ins[ip+1:])
			cur.ip += 1
			cur.cl.Free[freeIndex] = vm.pop()

		default:
			// Skipping an unknown opcode would run its operand bytes as
			// instructions, so it is an error.
			return fmt.Errorf("unknown opcode %d at position %d", op, ip)
		}
		if err != nil {
			return err
		}
	}

	return err
}

// push adds an object to the top of the stack.
func (vm *VM) push(o object.Object) error {
	if vm.sp >= len(vm.stack) {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

// pop removes and returns the object from the top of the stack.
// errStackUnderflow is what malformed bytecode that pops an empty stack
// fails with, the same on every compiler (not a runtime index panic).
var errStackUnderflow = errors.New("stack underflow")

func (vm *VM) pop() object.Object {
	if vm.sp <= 0 {
		panic(errStackUnderflow)
	}
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
		return fmt.Errorf("unknown boolean operator: %s", opcodeName(op))
	}
	return vm.push(nativeBoolToBooleanObject(result))
}

// executeBinaryOperation dispatches to the correct binary operation based on operand types.
func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	if done, err := vm.fastBinary(op, left, right); done {
		return err
	}
	right, left = asBitString(right), asBitString(left)

	switch {
	case isBoolean(left) && isBoolean(right):
		return vm.executeBinaryBooleanOperation(op, left, right)
	case isInteger(left) && isInteger(right):
		return vm.executeBinaryIntegerOperation(op, left, right)
	case isReal(left) && isReal(right):
		return vm.executeBinaryRealOperation(op, left, right)
	case (isReal(left) && isInteger(right)) || (isInteger(left) && isReal(right)):
		// An integer with a real is widened to a real, as IEC 61131-3 does.
		l, _ := object.GetFloat64Value(left)
		r, _ := object.GetFloat64Value(right)
		return vm.executeBinaryRealOperation(op, &object.LReal{Value: l}, &object.LReal{Value: r})
	case isString(left) && isString(right):
		return vm.executeBinaryStringOperation(op, left, right)
	case usesSharedOperations(left) || usesSharedOperations(right):
		return vm.executeSharedOperation(op, left, right)
	default:
		return fmt.Errorf("unsupported types for binary operation: %s %s",
			left.Type(), right.Type())
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
	return vm.integerOperation(op, leftValue, rightValue)
}

// integerOperation performs a binary operation on two integer values and
// pushes the result as a LINT.
func (vm *VM) integerOperation(op code.Opcode, leftValue, rightValue int64) error {
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
		return fmt.Errorf("unknown integer operator: %s", opcodeName(op))
	}

	return vm.push(newLInt(result))
}

// executeBinaryRealOperation performs a binary operation on two real-number objects.
func (vm *VM) executeBinaryRealOperation(
	op code.Opcode,
	left, right object.Object,
) error {
	// Promote both to float64 for the operation
	var leftValue, rightValue float64
	if l, ok := left.(*object.Real); ok {
		leftValue = l.Value
	} else if l, ok := left.(*object.LReal); ok {
		leftValue = l.Value
	} else {
		return fmt.Errorf("left operand is not a real: %s", left.Type())
	}

	if r, ok := right.(*object.Real); ok {
		rightValue = r.Value
	} else if r, ok := right.(*object.LReal); ok {
		rightValue = r.Value
	} else {
		return fmt.Errorf("right operand is not a real: %s", right.Type())
	}
	return vm.realOperation(op, leftValue, rightValue)
}

// realOperation performs a binary operation on two real values and pushes
// the result as an LREAL.
func (vm *VM) realOperation(op code.Opcode, leftValue, rightValue float64) error {
	var result float64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		if rightValue == 0.0 {
			return fmt.Errorf("division by zero")
		}
		result = leftValue / rightValue
	case code.OpExponent:
		result = math.Pow(leftValue, rightValue)
	default:
		return fmt.Errorf("unknown real operator: %s", opcodeName(op))
	}

	return vm.push(&object.LReal{Value: result})
}

// executeComparison dispatches to the correct comparison operation based on operand types.
func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	if done, err := vm.fastComparison(op, left, right); done {
		return err
	}
	right, left = asBitString(right), asBitString(left)

	// r = NULL, r1 <> r2: references compare by what they refer to.
	if op == code.OpEqual || op == code.OpNotEqual {
		operator := "="
		if op == code.OpNotEqual {
			operator = "<>"
		}
		if equal, ok, err := object.CompareReferences(left, operator, right); ok {
			if err != nil {
				return err
			}
			return vm.push(nativeBoolToBooleanObject(equal))
		}
	}

	if isInteger(left) && isInteger(right) {
		return vm.executeIntegerComparison(op, left, right)
	}

	if isReal(left) && isReal(right) {
		return vm.executeRealComparison(op, left, right)
	}

	// An integer and a real compare as reals (INT converts to REAL).
	if (isInteger(left) && isReal(right)) || (isReal(left) && isInteger(right)) {
		return vm.executeRealComparison(op, asLReal(left), asLReal(right))
	}

	if isString(left) && isString(right) {
		return vm.executeStringComparison(op, stringValue(left), stringValue(right))
	}

	// A time, date or bit string compared with another type, such as a bit
	// string with an integer literal, is left to object.EvalInfix.
	if usesSharedOperations(left) || usesSharedOperations(right) {
		return vm.executeSharedOperation(op, left, right)
	}

	// Enumerated values are equal when they name the same value of the same
	// type, even if they are separate objects (e.g. two Color#Red constants).
	leftEnum, leftIsEnum := left.(*object.EnumeratedValue)
	rightEnum, rightIsEnum := right.(*object.EnumeratedValue)
	if leftIsEnum && rightIsEnum && (op == code.OpEqual || op == code.OpNotEqual) {
		same := token.SameIdent(leftEnum.TypeName, rightEnum.TypeName) && token.SameIdent(leftEnum.Value, rightEnum.Value)
		return vm.push(nativeBoolToBooleanObject(same == (op == code.OpEqual)))
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))
	default:
		return fmt.Errorf("unsupported operator %s for types %s and %s",
			opcodeName(op), left.Type(), right.Type())
	}
}

// executeStringComparison compares two strings, character by character.
func (vm *VM) executeStringComparison(op code.Opcode, left, right string) error {
	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(left == right))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(left != right))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(left > right))
	case code.OpLessThan:
		return vm.push(nativeBoolToBooleanObject(left < right))
	case code.OpGreaterThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(left >= right))
	case code.OpLessThanOrEqual:
		return vm.push(nativeBoolToBooleanObject(left <= right))
	default:
		return fmt.Errorf("unknown string comparison operator: %s", opcodeName(op))
	}
}

// sharedOperators maps opcodes to the operator names used by object.EvalInfix.
var sharedOperators = map[code.Opcode]string{
	code.OpAdd: "+", code.OpSub: "-", code.OpMul: "*", code.OpDiv: "/", code.OpMod: "MOD",
	code.OpAnd: "AND", code.OpOr: "OR", code.OpXor: "XOR",
	code.OpEqual: "=", code.OpNotEqual: "<>",
	code.OpGreaterThan: ">", code.OpLessThan: "<",
	code.OpGreaterThanOrEqual: ">=", code.OpLessThanOrEqual: "<=",
}

// usesSharedOperations reports whether obj is a time, date or bit-string value.
// Operations on these use the object package's implementation, which the
// evaluator also uses, so both engines follow the same IEC 61131-3 rules.
func usesSharedOperations(obj object.Object) bool {
	switch obj.(type) {
	case *object.Time, *object.Date, *object.TimeOfDay, *object.DateAndTime, *object.BitString:
		return true
	}
	return false
}

// executeSharedOperation performs an arithmetic, logical or comparison
// operation on time, date or bit-string operands with object.EvalInfix.
func (vm *VM) executeSharedOperation(op code.Opcode, left, right object.Object) error {
	operator, ok := sharedOperators[op]
	if !ok {
		return fmt.Errorf("unsupported operator %s for types %s and %s", opcodeName(op), left.Type(), right.Type())
	}
	result := object.EvalInfix(left, operator, right)
	if errObj, isErr := result.(*object.Error); isErr {
		return fmt.Errorf("%s", strings.TrimPrefix(errObj.Message, "BUILTIN ERROR: "))
	}
	return vm.push(result)
}

// opcodeName returns the name of an opcode for error messages, e.g. "OpAdd".
func opcodeName(op code.Opcode) string {
	if def, err := code.Lookup(byte(op)); err == nil {
		return def.Name
	}
	return fmt.Sprintf("opcode %d", op)
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
	return vm.integerComparison(op, leftValue, rightValue)
}

// integerComparison compares two integer values and pushes the BOOL result.
func (vm *VM) integerComparison(op code.Opcode, leftValue, rightValue int64) error {
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
		return fmt.Errorf("unknown integer comparison operator: %s", opcodeName(op))
	}
}

// executeRealComparison performs a comparison operation on two real-number objects.
func (vm *VM) executeRealComparison(
	op code.Opcode,
	left, right object.Object,
) error {
	var leftValue, rightValue float64
	if l, ok := left.(*object.Real); ok {
		leftValue = l.Value
	} else if l, ok := left.(*object.LReal); ok {
		leftValue = l.Value
	} else {
		return fmt.Errorf("left operand is not a real: %s", left.Type())
	}

	if r, ok := right.(*object.Real); ok {
		rightValue = r.Value
	} else if r, ok := right.(*object.LReal); ok {
		rightValue = r.Value
	} else {
		return fmt.Errorf("right operand is not a real: %s", right.Type())
	}
	return vm.realComparison(op, leftValue, rightValue)
}

// realComparison compares two real values and pushes the BOOL result.
func (vm *VM) realComparison(op code.Opcode, leftValue, rightValue float64) error {
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
		return fmt.Errorf("unknown real comparison operator: %s", opcodeName(op))
	}
}

// executeBangOperator performs a logical NOT operation on the top of the stack.
func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	operand = asBitString(operand)
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
		return fmt.Errorf("unknown string operator: %s", opcodeName(op))
	}

	joined := stringValue(left) + stringValue(right)
	// Joining with a WSTRING gives a WSTRING, matching the compiler's typing.
	if isWString(left) || isWString(right) {
		return vm.push(&object.WString{Value: joined})
	}
	return vm.push(&object.String{Value: joined})
}

// isString reports whether obj is a STRING or WSTRING.
func isString(obj object.Object) bool {
	switch obj.(type) {
	case *object.String, *object.WString:
		return true
	}
	return false
}

// stringValue returns the text of a STRING or WSTRING object.
func stringValue(obj object.Object) string {
	switch s := obj.(type) {
	case *object.String:
		return s.Value
	case *object.WString:
		return s.Value
	}
	return ""
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
	case isArray(left) && isInteger(index):
		return vm.executeArrayIndex(left, index)
	case isHash(left):
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
	i -= arrayObject.LowerBound                 // Arrays are indexed from their declared lower bound.
	max := int64(len(arrayObject.Elements) - 1) // Keep as int64 for comparison with int64 `i`

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

// executeHashIndex executes an index operation on a hash.
func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := hashKeyOf(index)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key]
	if !ok {
		if name, isString := index.(*object.String); isString {
			if k, found := foldKey(hashObject, name); found {
				return vm.push(hashObject.Pairs[k].Value)
			}
		}
		// A name missing from an FB instance may be a method or property
		// accessor, which live on the instance's class hierarchy.
		if name, isString := index.(*object.String); isString {
			if class, isInstance := instanceClass(hashObject); isInstance {
				if method, err := vm.findMethodInHierarchy(class, name); err == nil {
					if cl, isClosure := method.(*object.Closure); isClosure {
						return vm.push(&BoundMethod{Fn: cl, Receiver: hashObject})
					}
					return vm.push(method)
				}
			}
		}
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

// BoundMethod is a method closure paired with the function block instance it
// was looked up on. Calling it passes the instance as the first argument,
// which is the method's THIS local.
type BoundMethod struct {
	Fn       *object.Closure
	Receiver object.Object
}

// Type returns the object type of a bound method.
func (b *BoundMethod) Type() object.ObjectType { return "BOUND_METHOD" }

// Inspect returns a string representation of a bound method.
func (b *BoundMethod) Inspect() string { return "bound method" }

// instanceClass returns the class hash of an FB instance, which is stored
// under the "__class__" key. It reports false if hash is not an instance.
func instanceClass(hash *object.Hash) (*object.Hash, bool) {
	pair, ok := hash.Pairs[classKey]
	if !ok {
		return nil, false
	}
	class, ok := pair.Value.(*object.Hash)
	return class, ok
}

// executeSetIndex executes a set index operation on an array or hash.
func (vm *VM) executeSetIndex(left, index, val object.Object) error {
	switch {
	case isArray(left) && isInteger(index):
		return vm.executeArraySetIndex(left, index, val)
	case isHash(left):
		return vm.executeHashSetIndex(left, index, val)
	default:
		return fmt.Errorf("index operator not supported for setting on: %s", left.Type())
	}
}

// executeArraySetIndex executes a set index operation on an array.
func (vm *VM) executeArraySetIndex(array, index, val object.Object) error {
	arrayObject := array.(*object.Array)
	i, _, ok := object.GetIntegerObjectValue(index)
	if !ok {
		return fmt.Errorf("array index must be an integer, got %s", index.Type())
	}
	declared := i
	i -= arrayObject.LowerBound // Arrays are indexed from their declared lower bound.
	max := int64(len(arrayObject.Elements) - 1)

	if i < 0 || i > max {
		return fmt.Errorf("array index out of bounds: %d", declared)
	}

	arrayObject.Elements[i] = val
	return nil
}

// executeHashSetIndex executes a set index operation on a hash.
func (vm *VM) executeHashSetIndex(hash, index, val object.Object) error {
	hashObject := hash.(*object.Hash)
	key, ok := hashKeyOf(index)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}
	if _, exact := hashObject.Pairs[key]; !exact {
		if name, isString := index.(*object.String); isString {
			if k, found := foldKey(hashObject, name); found {
				hashObject.Pairs[k] = object.HashPair{Key: hashObject.Pairs[k].Key, Value: val}
				return nil
			}
		}
	}
	hashObject.Pairs[key] = object.HashPair{Key: index, Value: val}
	return nil
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
	frame := vm.frames[vm.framesIndex]
	frame.returned = true
	return frame
}

// executeCall executes a function call.
func (vm *VM) executeCall(numArgs int) error {
	callee := vm.stack[vm.sp-1-numArgs]
	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	case *BoundMethod:
		// Insert the receiver as the first argument (THIS): shift the
		// arguments up one slot and replace the callee with the closure.
		calleePos := vm.sp - 1 - numArgs
		if vm.sp >= len(vm.stack) {
			return fmt.Errorf("stack overflow")
		}
		copy(vm.stack[calleePos+2:vm.sp+1], vm.stack[calleePos+1:vm.sp])
		vm.stack[calleePos] = callee.Fn
		vm.stack[calleePos+1] = callee.Receiver
		vm.sp++
		return vm.callClosure(callee.Fn, numArgs+1)
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

	if vm.framesIndex >= len(vm.frames) {
		return fmt.Errorf("call stack overflow")
	}
	if basePointer+cl.Fn.NumLocals > len(vm.stack) {
		return fmt.Errorf("stack overflow")
	}
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
	// Names are matched case-insensitively, as IEC 61131-3 identifiers are.
	paramIndexMap := make(map[string]int)
	for i, name := range cl.Fn.ParameterNames {
		paramIndexMap[strings.ToUpper(name)] = i
	}

	// Read every argument before placing any of them: a named argument may
	// belong in a slot that still holds another argument that has not been
	// read yet, e.g. `f(b := 1, a := 10)`.
	args := make([]object.Object, numArgs)
	copy(args, vm.stack[basePointer:basePointer+numArgs])
	for i, arg := range args {
		if namedArg, ok := arg.(*object.NamedArgument); ok {
			idx, exists := paramIndexMap[strings.ToUpper(namedArg.Name)]
			if !exists {
				return fmt.Errorf("unknown named argument: %s", namedArg.Name)
			}
			vm.stack[frame.basePointer+idx] = namedArg.Value
		} else {
			vm.stack[frame.basePointer+i] = arg
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

// The type tests on the hot path switch on the concrete type rather than
// calling obj.Type(): TinyGo compiles an interface method call to a search
// over every Object type, which made each instruction many times slower on
// a microcontroller. Each ObjectType belongs to exactly one concrete type.

// isInteger checks if an object is one of the integer types.
func isInteger(obj object.Object) bool {
	switch obj.(type) {
	case *object.SInt, *object.Int, *object.DInt, *object.LInt,
		*object.USInt, *object.UInt, *object.UDInt, *object.ULInt:
		return true
	default:
		return false
	}
}

// isReal checks if an object is one of the real-number types.
// asLReal returns an integer as an LREAL, and any other value as it is.
func asLReal(obj object.Object) object.Object {
	if n, _, ok := object.GetIntegerObjectValue(obj); ok && isInteger(obj) {
		if u, isU := obj.(*object.ULInt); isU {
			return &object.LReal{Value: float64(u.Value)}
		}
		return &object.LReal{Value: float64(n)}
	}
	return obj
}

func isReal(obj object.Object) bool {
	switch obj.(type) {
	case *object.Real, *object.LReal:
		return true
	default:
		return false
	}
}

// isBoolean checks if an object is a BOOL.
func isBoolean(obj object.Object) bool {
	_, ok := obj.(*object.Boolean)
	return ok
}

// isWString checks if an object is a WSTRING.
func isWString(obj object.Object) bool {
	_, ok := obj.(*object.WString)
	return ok
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
	classHashKey := classKey
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
	parentHashKey := parentKey
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

	// Push the parent method, bound to the instance so it receives THIS.
	if cl, ok := method.(*object.Closure); ok {
		return vm.push(&BoundMethod{Fn: cl, Receiver: instance})
	}
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

	parentHashKey := parentKey
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

	parentHashKey := parentKey
	if parentPair, ok := class.Pairs[parentHashKey]; ok {
		if parentHash, ok := parentPair.Value.(*object.Hash); ok {
			return vm.findMethodInHierarchy(parentHash, methodName)
		}
	}

	return nil, fmt.Errorf("method '%s' not found in class hierarchy", methodName.Value)
}

// setArrayBounds sets the lower bound of each dimension of an array: the
// first bound on the array itself and the rest on the arrays it contains.
// A value that is not an array is left alone.
func setArrayBounds(value object.Object, bounds []object.Object) {
	array, ok := value.(*object.Array)
	if !ok || len(bounds) == 0 {
		return
	}
	if low, _, ok := object.GetIntegerObjectValue(bounds[0]); ok {
		array.LowerBound = low
	}
	for _, element := range array.Elements {
		setArrayBounds(element, bounds[1:])
	}
}

// bitTypeName returns the declared type a bit instruction names: its operand
// is 1 + the constant index of the type's name, or 0 for none.
func (vm *VM) bitTypeName(operand uint16) string {
	if operand == 0 {
		return ""
	}
	if name, ok := vm.constants[operand-1].(*object.String); ok {
		return name.Value
	}
	return ""
}

// foldKey finds the key of a hash's string-keyed pair whose name matches
// name in any case: IEC 61131-3 names are not case-sensitive, so `b.q`
// reaches an instance's `Q`. It is used only when the exact key misses.
func foldKey(hash *object.Hash, name *object.String) (object.HashKey, bool) {
	for k, pair := range hash.Pairs {
		if s, ok := pair.Key.(*object.String); ok && token.SameIdent(s.Value, name.Value) {
			return k, true
		}
	}
	return object.HashKey{}, false
}

// isArray checks if an object is an ARRAY.
func isArray(obj object.Object) bool {
	_, ok := obj.(*object.Array)
	return ok
}

// isHash checks if an object is a structure or other hash.
func isHash(obj object.Object) bool {
	_, ok := obj.(*object.Hash)
	return ok
}

// fastBinary computes an arithmetic operation on two LINTs or two LREALs,
// the types the VM computes in, without the general dispatch, and reports
// whether it did. The result is the one executeBinaryOperation gives: these
// types are neither bit strings nor BOOLs, so its first matching case is the
// integer or real one.
func (vm *VM) fastBinary(op code.Opcode, left, right object.Object) (bool, error) {
	switch l := left.(type) {
	case *object.LInt:
		if r, ok := right.(*object.LInt); ok {
			return true, vm.integerOperation(op, l.Value, r.Value)
		}
	case *object.LReal:
		if r, ok := right.(*object.LReal); ok {
			return true, vm.realOperation(op, l.Value, r.Value)
		}
	}
	return false, nil
}

// fastComparison is fastBinary for comparisons. Neither type is a reference,
// so executeComparison's reference rule does not apply.
func (vm *VM) fastComparison(op code.Opcode, left, right object.Object) (bool, error) {
	switch l := left.(type) {
	case *object.LInt:
		if r, ok := right.(*object.LInt); ok {
			return true, vm.integerComparison(op, l.Value, r.Value)
		}
	case *object.LReal:
		if r, ok := right.(*object.LReal); ok {
			return true, vm.realComparison(op, l.Value, r.Value)
		}
	}
	return false, nil
}

// smallInts are the LINTs 0 to 255. Most integer results in a PLC program
// (counters, indexes, small sums) are small, and sharing these saves an
// allocation for each, which on a microcontroller is much of an
// instruction's cost. Objects are never changed in place, so sharing is safe.
var smallInts = func() (a [256]object.LInt) {
	for i := range a {
		a[i].Value = int64(i)
	}
	return a
}()

// newLInt returns a LINT of value v, shared if it is small.
func newLInt(v int64) *object.LInt {
	if uint64(v) < uint64(len(smallInts)) {
		return &smallInts[v]
	}
	return &object.LInt{Value: v}
}

// hashKeyOf returns index's hash key, and false if it cannot be one. A
// member name, a STRING, is the usual key, and is hashed without an
// interface call (see isInteger).
func hashKeyOf(index object.Object) (object.HashKey, bool) {
	if s, ok := index.(*object.String); ok {
		return s.HashKey(), true
	}
	h, ok := index.(object.Hashable)
	if !ok {
		return object.HashKey{}, false
	}
	return h.HashKey(), true
}

// The hash keys of an instance's class and a class's parent, hashed once.
var (
	classKey  = (&object.String{Value: "__class__"}).HashKey()
	parentKey = (&object.String{Value: "__parent__"}).HashKey()
)
