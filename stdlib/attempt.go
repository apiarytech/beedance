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

// $ATTEMPT is the assignment attempt target ?= value on the VM. The compiler
// gives it the value, whether the target is a reference, and the classes
// (function block hashes) an instance may be of to be assigned:
// $ATTEMPT(value, isRef, class...). '$' cannot start an identifier, so no
// program can name it.

import (
	"github.com/apiarytech/beedance/object"
)

func init() {
	object.RegisterBuiltin(BuiltinAttempt, "$ATTEMPT", attemptFn)
}

var (
	classKey  = (&object.String{Value: "__class__"}).HashKey()
	parentKey = (&object.String{Value: "__parent__"}).HashKey()
)

// attemptFn returns the instance the value refers to (or a reference to it,
// for a reference target) if its class, or one its class extends, is one
// given; NULL otherwise.
var attemptFn = func(args ...object.Object) object.Object {
	if len(args) < 2 {
		return object.NewBuiltinError("$ATTEMPT: got %d arguments, want at least 2", len(args))
	}
	isRef := false
	if b, ok := args[1].(*object.Boolean); ok {
		isRef = b.Value
	}
	null := func() object.Object {
		if isRef {
			return &object.Reference{}
		}
		return object.NULL
	}
	var inst *object.Hash
	switch v := args[0].(type) {
	case *object.Hash:
		inst = v
	case *object.Reference:
		if !v.IsNull() {
			if got, err := v.Get(); err == nil {
				inst, _ = got.(*object.Hash)
			}
		}
	}
	if inst == nil {
		return null()
	}
	pair, ok := inst.Pairs[classKey]
	if !ok {
		return null()
	}
	class, _ := pair.Value.(*object.Hash)
	for depth := 0; class != nil && depth < 32; depth++ {
		for _, allowed := range args[2:] {
			if allowed == object.Object(class) {
				if isRef {
					var held object.Object = inst
					return &object.Reference{Slot: &held}
				}
				return inst
			}
		}
		parent, ok := class.Pairs[parentKey]
		if !ok {
			break
		}
		class, _ = parent.Value.(*object.Hash)
	}
	return null()
}
