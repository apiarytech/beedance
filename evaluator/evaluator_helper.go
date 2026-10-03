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

import (
	"fmt"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
	"strings"
)

// checkAccessPermission verifies if a member can be accessed based on its access specifier
// (PUBLIC, PRIVATE, PROTECTED) from the current evaluation environment.
func checkAccessPermission(accessSpecifier string, ownerDef *ast.FunctionBlockDeclaration, callerEnv *object.Environment) *object.Error {
	// PUBLIC members are always accessible. Default is PUBLIC.
	if accessSpecifier == "" || accessSpecifier == "PUBLIC" {
		return nil
	}

	// Get the calling instance from the 'THIS' variable in the caller's environment.
	callerThisObj, ok := callerEnv.Get("THIS")
	if !ok {
		// If 'THIS' is not in the environment, the call is from outside any FB instance (e.g., from a PROGRAM).
		// In this case, only PUBLIC members are allowed.
		if accessSpecifier == "PRIVATE" || accessSpecifier == "PROTECTED" { // cspell:disable-line
			return &object.Error{Message: fmt.Sprintf("member is %s", strings.ToLower(accessSpecifier))}
		}
		return nil
	}

	callerInstance, ok := callerThisObj.(*object.FunctionBlockInstance)
	if !ok {
		// This would be an internal error.
		return &object.Error{Message: "internal error: THIS is not a FunctionBlockInstance"}
	}

	// Add a nil check for the caller's definition to prevent panics.
	if callerInstance.Definition == nil || callerInstance.Definition.Definition == nil {
		// A caller without a definition (like a built-in FB) cannot access non-public members.
		return &object.Error{Message: fmt.Sprintf("cannot access %s member from an undefined context", strings.ToLower(accessSpecifier))}
	}

	// PRIVATE members: accessible only from within the FB that defines them.
	if accessSpecifier == "PRIVATE" {
		// The caller's definition must be the same as the owner's definition.
		if callerInstance.Definition.Definition == ownerDef {
			return nil
		}
		// Check if the call is from a derived class.
		if isSubclassOf(callerInstance.Definition, ownerDef, callerEnv) {
			return &object.Error{Message: "member is private and cannot be accessed from derived function block"}
		}
		return &object.Error{Message: "member is private"}
	}

	// PROTECTED members: accessible from the same instance or derived instances.
	if accessSpecifier == "PROTECTED" {
		// Caller must be same class or a subclass of owner.
		if isSubclassOf(callerInstance.Definition, ownerDef, callerEnv) {
			return nil
		}

		return &object.Error{Message: "member is protected"}
	}

	return nil // Default case, allow access.
}

func isSubclassOf(d *object.FunctionBlock, target *ast.FunctionBlockDeclaration, env *object.Environment) bool {
	current := d
	for current != nil {
		if current.Definition == target {
			return true
		}
		if current.Definition.Extends == nil {
			return false
		}
		// The parent FB must be found in the environment where the current FB was defined.
		parentName := current.Definition.Extends.String()
		parentObj, ok := current.Env.Get(parentName)
		if !ok {
			return false
		}
		parentFB, ok := parentObj.(*object.FunctionBlock)
		if !ok {
			return false
		}
		current = parentFB
	}
	return false
}

// TimeDateLiteral parses the value of a TIME, DATE, TIME_OF_DAY or
// DATE_AND_TIME literal (such as "5s" for T#5s), returning an *object.Time,
// *object.Date, *object.TimeOfDay, *object.DateAndTime or *object.Error. The
// transpiler uses it so that every engine reads literals the same way.
func TimeDateLiteral(value, typeName string) object.Object {
	return applyTimeDateConversion(value, typeName)
}
