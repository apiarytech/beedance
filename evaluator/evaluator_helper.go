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
	"beedance/ast"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
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

func getParentFB(fb *object.FunctionBlock) *object.FunctionBlock {
	if fb == nil || fb.Definition == nil || fb.Definition.Extends == nil {
		return nil
	}
	// Look for the parent's definition in the environment where the current FB was defined.
	parentName := fb.Definition.Extends.String()
	parentObj, ok := fb.Env.Get(parentName)
	if !ok {
		return nil
	}
	parentFB, ok := parentObj.(*object.FunctionBlock)
	if !ok {
		return nil
	}
	return parentFB
}

func testEval(t *testing.T, input string) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, t.Name(), input)
	env := object.NewEnvironment()

	return Eval(program, env)
}

func testEvalWithEnv(t *testing.T, input string, env *object.Environment) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p, t.Name(), input)
	return Eval(program, env)
}

func testIntegerObject(t *testing.T, obj object.Object, name string, expected int64) bool {
	t.Helper()
	val, _, ok := object.GetIntegerObjectValue(obj)

	if !ok {
		t.Errorf("object is not a supported Integer type. got=%T (%+v)", obj, obj)
		return false
	}
	if val != expected {
		t.Errorf("variable '%s' has wrong value. got=%d, want=%d", name, val, expected)
		return false
	}
	return true
}

func testIntegerObjectInEnv(t *testing.T, env *object.Environment, name string, expected int64) bool {
	t.Helper()
	obj, err := mustGet(env, name)
	if err != nil {
		t.Errorf("%v", err)
		return false
	}
	return testIntegerObject(t, obj, name, expected)
}

func testBooleanObjectInEnv(t *testing.T, env *object.Environment, name string, expected bool) bool {
	t.Helper()
	obj, err := mustGet(env, name)
	if err != nil {
		t.Errorf("object is not Boolean. got=%T (%+v)", err, err)
		return false
	}
	return testBooleanObject(t, obj, name, expected)
}

func testBooleanObject(t *testing.T, obj object.Object, name string, expected bool) bool {
	t.Helper()
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("variable '%s' has wrong value. got=%t, want=%t", name, result.Value, expected)
		return false
	}
	return true
}

func testTimeObject(t *testing.T, obj object.Object, name string, expected time.Duration) bool {
	t.Helper()
	result, ok := obj.(*object.Time)
	if !ok {
		t.Errorf("object is not Time. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("variable '%s' has wrong value. got=%s, want=%s", name, result.Value, expected)
		return false
	}
	return true
}

func testTimeObjectInEnv(t *testing.T, env *object.Environment, name string, expected time.Duration) bool {
	t.Helper()
	obj, err := mustGet(env, name)
	if err != nil {
		t.Errorf("object is not Time. got=%T (%+v)", err, err)
		return false
	}
	return testTimeObject(t, obj, name, expected)
}

func testNullObject(t *testing.T, obj object.Object) bool {
	t.Helper()
	if obj != NULL {
		t.Errorf("object is not NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}

func testErrorObjectContains(t *testing.T, obj object.Object, expectedMessage string) bool {
	t.Helper()
	if obj == nil {
		t.Errorf("object is nil, expected Error containing %q", expectedMessage)
		return false
	}
	errObj, ok := obj.(*object.Error)
	if !ok {
		t.Errorf("object is not Error. got=%T (%+v)", obj, obj)
		return false
	}
	if !strings.Contains(errObj.Message, expectedMessage) {
		t.Errorf("wrong error message. expected to contain %q, got %q", expectedMessage, errObj.Message)
		return false
	}
	return true
}

func checkParserErrors(t *testing.T, p *parser.Parser, testName string, input string) {
	t.Helper()
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("FAIL: %s - parser has %d errors for input:\n%s", testName, len(errors), input)
	for _, msg := range errors {
		t.Errorf("Parser error: %q", msg)
	}
	t.FailNow()
}

// mustGet is a test helper to get a value from the environment and fail if not found.
func mustGet(env *object.Environment, name string) (object.Object, error) {
	obj, ok := env.Get(name)
	if !ok {
		return nil, fmt.Errorf("variable '%s' not found in environment", name)
	}
	return obj, nil
}

func testRealObject(t *testing.T, obj object.Object, name string, expected float64) bool {
	t.Helper()
	val, ok := object.GetFloat64Value(obj)
	if !ok {
		t.Errorf("object is not a REAL type. got=%T (%+v)", obj, obj)
		return false
	}
	const epsilon = 1e-9
	if diff := val - expected; diff < -epsilon || diff > epsilon {
		t.Errorf("variable '%s' has wrong value. got=%f, want=%f", name, val, expected)
		return false
	}
	return true
}

func testRealObjectInEnv(t *testing.T, env *object.Environment, name string, expected float64) bool {
	t.Helper()
	obj, err := mustGet(env, name)
	if err != nil {
		t.Errorf("object is not a REAL type. got=%T (%+v)", err, err)
		return false
	}
	return testRealObject(t, obj, name, expected)
}

func testBitStringObject(t *testing.T, obj object.Object, expected uint64) bool {
	t.Helper()
	bs, ok := obj.(*object.BitString)
	if !ok {
		t.Errorf("object is not BitString. got=%T (%+v)", obj, obj)
		return false
	}
	if bs.Value != expected {
		t.Errorf("wrong bitstring value. want=%d (0x%X), got=%d (0x%X)", expected, expected, bs.Value, bs.Value)
		return false
	}
	return true
}

func testStringObject(t *testing.T, obj object.Object, name string, expected string) bool {
	t.Helper()
	result, ok := obj.(*object.String)
	if !ok {
		t.Errorf("object is not String. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("variable '%s' has wrong value. got=%q, want=%q", name, result.Value, expected)
		return false
	}
	return true
}

func testStringObjectInEnv(t *testing.T, env *object.Environment, name string, expected string) bool {
	t.Helper()
	obj, err := mustGet(env, name)
	if err != nil {
		t.Errorf("object is not String. got=%T (%+v)", err, err)
		return false
	}
	return testStringObject(t, obj, name, expected)
}

func testParserErrorContains(t *testing.T, errors []string, expectedMessage string) bool {
	t.Helper()
	found := false
	for _, msg := range errors {
		if strings.Contains(msg, expectedMessage) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected parser error containing %q, but none found in %v", expectedMessage, errors)
	}
	return found
}

func testErrorObject(t *testing.T, obj object.Object, expectedMessage string) bool {
	errObj, ok := obj.(*object.Error)
	if !ok {
		t.Errorf("object is not Error. got=%T (%+v)", obj, obj)
		return false
	}
	if errObj.Message != expectedMessage {
		t.Errorf("wrong error message. expected=%q, got=%q", expectedMessage, errObj.Message)
		return false
	}
	return true
}

func testEvalWithPi(t *testing.T, input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	env.Set("PI", &object.Real{Value: math.Pi})
	checkParserErrors(t, p, t.Name(), input)
	return Eval(program, env)
}

func testEvalWithBuiltinVars(t *testing.T, input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	env.Set("PI", &object.Real{Value: math.Pi})
	env.Set("E", &object.Real{Value: math.E})
	checkParserErrors(t, p, t.Name(), input)
	return Eval(program, env)
}

func testEvalWithParserErrors(t *testing.T, input string) (object.Object, []string) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	return Eval(program, env), p.Errors()
}
