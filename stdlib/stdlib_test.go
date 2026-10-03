package stdlib

import (
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"strings"
	"testing"
	"time"
)

// wstringExpectation is a helper struct for testing WString results.
type wstringExpectation struct {
	value string
}

// testEval is a helper that parses and evaluates a given input string using the
// main evaluator, ensuring that tests run through the same code path as the application.
func testEval(t *testing.T, input string) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Errorf("parser has %d errors", len(p.Errors()))
		for _, msg := range p.Errors() {
			t.Errorf("parser error: %q", msg)
		}
		t.FailNow()
	}
	env := object.NewEnvironment()
	return evaluator.Eval(program, env)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) {
	t.Helper()
	val, _, ok := object.GetIntegerObjectValue(obj) // cspell:disable-line
	if !ok {
		t.Fatalf("object is not an integer type. got=%T (%+v)", obj, obj)
	}
	if val != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", val, expected)
	}
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) {
	t.Helper()
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Fatalf("object is not Boolean. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
	}
}

func testStringObject(t *testing.T, obj object.Object, expected string) {
	t.Helper()
	result, ok := obj.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%q, want=%q", result.Value, expected)
	}
}

func testRealObject(t *testing.T, obj object.Object, expected float64) {
	t.Helper()
	var val float64 // cspell:disable-line
	switch o := obj.(type) {
	case *object.Real:
		val = o.Value
	case *object.LReal:
		val = o.Value
	default:
		t.Fatalf("object is not a REAL or LREAL. got=%T (%+v)", obj, obj)
	}

	if diff := val - expected; diff < -0.000001 || diff > 0.000001 {
		t.Errorf("wrong real value. want=%f, got=%f", expected, val)
	}
}

func testTimeObject(t *testing.T, obj object.Object, expected time.Duration) {
	t.Helper()
	result, ok := obj.(*object.Time)
	if !ok {
		t.Fatalf("object is not Time. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%v, want=%v", result.Value, expected)
	}
}

func testStringOrError(t *testing.T, obj object.Object, expected string) {
	t.Helper()
	errObj, isErr := obj.(*object.Error)
	if isErr {
		if !strings.Contains(errObj.Message, expected) {
			t.Errorf("error message %q does not contain %q", errObj.Message, expected)
		}
		return
	}
	testStringObject(t, obj, expected)
}

func testNullObject(t *testing.T, obj object.Object) bool {
	t.Helper()
	if _, ok := obj.(*object.Null); !ok {
		t.Errorf("object is not NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}
