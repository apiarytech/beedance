/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package stdlib

import "beedance/object"

func init() {
	object.RegisterBuiltin(BuiltinMux, "MUX", muxBuiltin)
	object.RegisterBuiltin(BuiltinLimit, "LIMIT", limitWrapperBuiltin)
	object.RegisterBuiltin(BuiltinSel, "SEL", selWrapperBuiltin)
	object.RegisterBuiltin(BuiltinMove, "MOVE", moveBuiltin)
}

func limitBuiltin(min, in, max object.Object) object.Object {
	args := []object.Object{min, in, max}
	hasReal := false
	for _, arg := range args {
		if !object.IsNumeric(arg) {
			return object.NewBuiltinError("all arguments to `LIMIT` must be INTEGER or REAL, got %s", arg.Type())
		}
		if arg.Type() == object.REAL_OBJ || arg.Type() == object.LREAL_OBJ {
			hasReal = true
		}
	}

	if hasReal {
		minVal, _ := object.GetFloat64Value(min)
		inVal, _ := object.GetFloat64Value(in)
		maxVal, _ := object.GetFloat64Value(max)
		if inVal < minVal {
			return &object.Real{Value: minVal}
		}
		if inVal > maxVal {
			return &object.Real{Value: maxVal}
		}
		return &object.Real{Value: inVal}
	}

	// All are integers
	minVal, _, _ := object.GetIntegerObjectValue(min)
	inVal, _, _ := object.GetIntegerObjectValue(in)
	maxVal, _, _ := object.GetIntegerObjectValue(max)
	if inVal < minVal {
		return &object.LInt{Value: minVal}
	}
	if inVal > maxVal {
		return &object.LInt{Value: maxVal}
	}
	return &object.LInt{Value: inVal}
}

func limitWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for LIMIT. got=%d, want=3", len(args))
	}
	mn := args[0]
	in := args[1]
	mx := args[2]
	return limitBuiltin(mn, in, mx)
}

func selWrapperBuiltin(args ...object.Object) object.Object {
	if len(args) != 3 {
		return object.NewBuiltinError("wrong number of arguments for SEL. got=%d, want=3", len(args))
	}
	return selBuiltin(args[0], args[1], args[2])
}

func selBuiltin(g, in0, in1 object.Object) object.Object {
	gBool, ok := g.(*object.Boolean)
	if !ok {
		return object.NewBuiltinError("argument 1 to `SEL` must be BOOLEAN, got %s", g.Type())
	}
	if in0.Type() != in1.Type() {
		return object.NewBuiltinError("arguments 2 and 3 to `SEL` must be of the same type, got %s and %s", in0.Type(), in1.Type())
	}
	if gBool.Value {
		return in1
	}
	return in0
}

func moveBuiltin(args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewBuiltinError("wrong number of arguments for MOVE. got=%d, want=1", len(args))
	}
	return args[0]
}

func muxBuiltin(args ...object.Object) object.Object {
	if len(args) < 2 {
		return object.NewBuiltinError("wrong number of arguments for MUX. got=%d, want>=2", len(args))
	}

	kVal, _, ok := object.GetIntegerObjectValue(args[0])
	if !ok {
		return object.NewBuiltinError("argument 1 to `MUX` must be INTEGER, got %s", args[0].Type())
	}

	valueArgs := args[1:]
	numInputs := len(valueArgs)

	// Check that all value arguments are of the same type.
	if numInputs > 1 {
		firstType := valueArgs[0].Type()
		for i := 1; i < numInputs; i++ {
			if valueArgs[i].Type() != firstType {
				return object.NewBuiltinError("all value arguments to `MUX` must be of the same type, got %s but expected %s", valueArgs[i].Type(), firstType)
			}
		}
	}

	if kVal < 0 || kVal >= int64(numInputs) {
		return object.NewBuiltinError("index %d out of bounds for MUX with %d inputs", kVal, numInputs)
	}

	return valueArgs[kVal]
}
