/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package evaluator

// The assignment attempt (IEC 61131-3 3rd edition), target ?= value: the
// instance the value refers to (an interface variable's, or a reference's)
// is assigned if it is of the target's type, an interface or a REF_TO of a
// function block (or one extending it); otherwise the target is NULL.

import (
	"strings"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// evalAssignmentAttempt evaluates target ?= value.
func evalAssignmentAttempt(node *ast.AssignmentStatement, env *object.Environment) object.Object {
	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}
	// The instance the value refers to, if any.
	var inst *object.FunctionBlockInstance
	switch v := val.(type) {
	case *object.FunctionBlockInstance:
		inst = v
	case *object.Reference:
		if !v.IsNull() {
			if got, err := v.Get(); err == nil {
				inst, _ = got.(*object.FunctionBlockInstance)
			}
		}
	}
	declared := declaredTypeOf(node.Left, env)
	if rt, isRef := isRefToType(declared); isRef {
		if inst != nil && instanceIsA(inst, dataTypeName(rt.BaseType), env) {
			var held object.Object = inst
			return assignValue(node, node.Left, &object.Reference{Slot: &held, DataType: rt.BaseType}, env)
		}
		return assignValue(node, node.Left, nullReference(rt, env), env)
	}
	if declared != nil && inst != nil && instanceIsA(inst, dataTypeName(declared), env) {
		return assignValue(node, node.Left, inst, env)
	}
	return assignValue(node, node.Left, NULL, env)
}

// instanceIsA reports whether a function block instance is of the type
// typeName: its function block or one it extends, or an interface one of
// those implements (or one such an interface extends).
func instanceIsA(inst *object.FunctionBlockInstance, typeName string, env *object.Environment) bool {
	if inst.Definition == nil {
		return false
	}
	var interfaceIs func(name string, depth int) bool
	interfaceIs = func(name string, depth int) bool {
		if strings.EqualFold(name, typeName) {
			return true
		}
		if depth > 16 {
			return false
		}
		if obj, ok := env.Get(name); ok {
			if iface, isIface := obj.(*object.InterfaceDefinition); isIface {
				for _, parent := range iface.Extends {
					if interfaceIs(dataTypeName(parent), depth+1) {
						return true
					}
				}
			}
		}
		return false
	}
	def := inst.Definition
	for depth := 0; def != nil && depth < 16; depth++ {
		if def.Name != nil && strings.EqualFold(def.Name.Value, typeName) {
			return true
		}
		if def.Definition == nil {
			return false
		}
		for _, impl := range def.Definition.Implements {
			if interfaceIs(dataTypeName(impl), 0) {
				return true
			}
		}
		if def.Definition.Extends == nil {
			return false
		}
		parent, ok := env.Get(dataTypeName(def.Definition.Extends))
		if !ok {
			return false
		}
		def, _ = parent.(*object.FunctionBlock)
	}
	return false
}
