/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

// This file runs references: REF(x) and ADR(x) make an object.Reference to a
// variable's slot, an array element or a member, r^ reads the variable it
// refers to, and r^ := value writes it.

import (
	"fmt"

	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
)

// executeReference executes OpRef, OpRefIndex, OpDeref or OpSetDeref, the
// instruction at ip.
func (vm *VM) executeReference(op code.Opcode, ins code.Instructions, ip int) error {
	frame := vm.currentFrame()
	switch op {
	case code.OpRef:
		scope := int(code.ReadUint8(ins[ip+1:]))
		index := int(code.ReadUint16(ins[ip+2:]))
		frame.ip += 3
		var ref *object.Reference
		switch scope {
		case code.RefGlobal:
			ref = &object.Reference{Slot: &vm.globals[index]}
		case code.RefLocal:
			// A local variable lives on the stack until its call returns;
			// after that, the slot belongs to other calls.
			ref = &object.Reference{Slot: &vm.stack[frame.basePointer+index], Live: func() bool { return !frame.returned }}
		case code.RefFree:
			ref = &object.Reference{Slot: &frame.cl.Free[index]}
		case code.RefExternal:
			address, ok := vm.constants[index].(*object.String)
			if !ok {
				return fmt.Errorf("a located variable's operand must be an address string constant, got %T", vm.constants[index])
			}
			ref = &object.Reference{IO: vm.io, Name: address.Value}
		default:
			return fmt.Errorf("OpRef: unknown scope %d", scope)
		}
		return vm.push(ref)

	case code.OpRefIndex:
		index := vm.pop()
		container := vm.pop()
		ref, err := object.NewElementReference(container, index)
		if err != nil {
			return err
		}
		return vm.push(ref)

	case code.OpDeref:
		bounds := int(code.ReadUint16(ins[ip+1:]))
		frame.ip += 2
		ref, err := referenceOperand(vm.pop())
		if err != nil {
			return err
		}
		val, err := ref.Get()
		if err != nil {
			return err
		}
		if val == nil {
			val = Null
		}
		if bounds > 0 {
			lower, err := lowerBounds(vm.constants[bounds-1])
			if err != nil {
				return err
			}
			val = object.ArrayView(val, lower)
		}
		return vm.push(val)

	case code.OpSetDeref:
		val := vm.pop()
		ref, err := referenceOperand(vm.pop())
		if err != nil {
			return err
		}
		if current, err := ref.Get(); err == nil {
			if a, isArray := current.(*object.Array); isArray {
				if v, isArray := val.(*object.Array); isArray {
					v.LowerBound = a.LowerBound // an array keeps its declared bounds
				}
			}
		}
		return ref.Set(val)
	}
	return fmt.Errorf("not a reference instruction: %d", op)
}

// referenceOperand returns v as a reference that refers to a variable.
func referenceOperand(v object.Object) (*object.Reference, error) {
	switch r := v.(type) {
	case *object.Reference:
		if r.IsNull() {
			return nil, fmt.Errorf("dereferencing a NULL reference")
		}
		return r, nil
	case *object.Null:
		return nil, fmt.Errorf("dereferencing a NULL reference")
	}
	return nil, fmt.Errorf("dereference operator (^) not applicable to type %s", v.Type())
}

// lowerBounds returns the lower bounds an OpDeref constant holds.
func lowerBounds(c object.Object) ([]int64, error) {
	arr, ok := c.(*object.Array)
	if !ok {
		return nil, fmt.Errorf("OpDeref: the bounds constant is a %s", c.Type())
	}
	lower := make([]int64, len(arr.Elements))
	for i, e := range arr.Elements {
		n, _, ok := object.GetIntegerObjectValue(e)
		if !ok {
			return nil, fmt.Errorf("OpDeref: bound %d is a %s", i, e.Type())
		}
		lower[i] = n
	}
	return lower, nil
}
