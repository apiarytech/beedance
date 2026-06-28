package evaluator

import (
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"math"
	"strings"
	"testing"
	"time"
)

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

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	t.Helper()
	var val int64
	var ok bool

	switch o := obj.(type) {
	case *object.SInt:
		val, ok = int64(o.Value), true
	case *object.Int:
		val, ok = int64(o.Value), true
	case *object.DInt:
		val, ok = int64(o.Value), true
	case *object.LInt:
		val, ok = o.Value, true
	case *object.USInt:
		val, ok = int64(o.Value), true
	case *object.UInt:
		val, ok = int64(o.Value), true
	case *object.UDInt:
		val, ok = int64(o.Value), true
	case *object.ULInt:
		val, ok = int64(o.Value), true
	default:
		ok = false
	}

	if !ok {
		t.Errorf("object is not a supported Integer type. got=%T (%+v)", obj, obj)
		return false
	}
	if val != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", val, expected)
		return false
	}
	return true
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) bool {
	t.Helper()
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t",
			result.Value, expected)
		return false
	}
	return true
}

func testTimeObject(t *testing.T, obj object.Object, expected time.Duration) bool {
	t.Helper()
	result, ok := obj.(*object.Time)
	if !ok {
		t.Errorf("object is not Time. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
		return false
	}
	return true
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
func mustGet(env *object.Environment, name string) object.Object {
	obj, ok := env.Get(name)
	if !ok {
		panic("variable " + name + " not found in environment")
	}
	return obj
}

func testRealObject(t *testing.T, obj object.Object, expected float64) bool {
	result, ok := obj.(*object.Real)
	if !ok {
		t.Errorf("object is not Real. got=%T (%+v)", obj, obj)
		return false
	}
	const epsilon = 1e-9
	if diff := result.Value - expected; diff < -epsilon || diff > epsilon {
		t.Errorf("object has wrong value. got=%f, want=%f", result.Value, expected)
		return false
	}
	return true
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

func testStringObject(t *testing.T, obj object.Object, expected string) bool {
	result, ok := obj.(*object.String)
	if !ok {
		t.Errorf("object is not String. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%q, want=%q",
			result.Value, expected)
		return false
	}
	return true
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
