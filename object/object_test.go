package object

import (
	"math"
	"strings"
	"testing"

	"beedance/ast"
	"beedance/token"
)

func TestStringHashKey(t *testing.T) {
	hello1 := &String{Value: "Hello World"}
	hello2 := &String{Value: "Hello World"}
	diff1 := &String{Value: "My name is johnny"}
	diff2 := &String{Value: "My name is johnny"}

	if hello1.HashKey() != hello2.HashKey() {
		t.Errorf("strings with same content have different hash keys")
	}

	if diff1.HashKey() != diff2.HashKey() {
		t.Errorf("strings with same content have different hash keys")
	}

	if hello1.HashKey() == diff1.HashKey() {
		t.Errorf("strings with different content have same hash keys")
	}
}

func TestBooleanHashKey(t *testing.T) {
	true1 := &Boolean{Value: true}
	true2 := &Boolean{Value: true}
	false1 := &Boolean{Value: false}
	false2 := &Boolean{Value: false}

	if true1.HashKey() != true2.HashKey() {
		t.Errorf("trues do not have same hash key")
	}

	if false1.HashKey() != false2.HashKey() {
		t.Errorf("falses do not have same hash key")
	}

	if true1.HashKey() == false1.HashKey() {
		t.Errorf("true has same hash key as false")
	}
}

func TestIntegerHashKey(t *testing.T) {
	one1 := &Integer{Value: 1}
	one2 := &Integer{Value: 1}
	two1 := &Integer{Value: 2}
	two2 := &Integer{Value: 2}

	if one1.HashKey() != one2.HashKey() {
		t.Errorf("integers with same content have twoerent hash keys")
	}

	if two1.HashKey() != two2.HashKey() {
		t.Errorf("integers with same content have twoerent hash keys")
	}

	if one1.HashKey() == two1.HashKey() {
		t.Errorf("integers with twoerent content have same hash keys")
	}
}

// TestRealHashKey tests the Real object's HashKey method.
func TestRealHashKey(t *testing.T) {
	onePointOne1 := &Real{Value: 1.1}
	onePointOne2 := &Real{Value: 1.1}
	twoPointTwo1 := &Real{Value: 2.2}
	twoPointTwo2 := &Real{Value: 2.2}

	if onePointOne1.HashKey() != onePointOne2.HashKey() {
		t.Errorf("reals with same content have different hash keys")
	}

	if twoPointTwo1.HashKey() != twoPointTwo2.HashKey() {
		t.Errorf("reals with same content have different hash keys")
	}

	if onePointOne1.HashKey() == twoPointTwo1.HashKey() {
		t.Errorf("reals with different content have same hash keys")
	}

	// Test with negative values
	negOnePointOne1 := &Real{Value: -1.1}
	negOnePointOne2 := &Real{Value: -1.1}
	if negOnePointOne1.HashKey() != negOnePointOne2.HashKey() {
		t.Errorf("negative reals with same content have different hash keys")
	}
	if negOnePointOne1.HashKey() == onePointOne1.HashKey() {
		t.Errorf("negative real has same hash key as positive real")
	}

	// Test with zero
	zero1 := &Real{Value: 0.0}
	zero2 := &Real{Value: 0.0}
	if zero1.HashKey() != zero2.HashKey() {
		t.Errorf("zeros have different hash keys")
	}

	// Test with NaN (Not a Number) - should ideally have consistent hash for same NaN representation
	// Note: math.NaN() always returns the same bit pattern for NaN, so its hash should be consistent.
	nan1 := &Real{Value: math.NaN()}
	nan2 := &Real{Value: math.NaN()}
	if nan1.HashKey() != nan2.HashKey() {
		t.Errorf("NaNs have different hash keys")
	}
}

// TestNullObject tests the Null object's Type and Inspect methods.
func TestNullObject(t *testing.T) {
	null := &Null{}

	if null.Type() != NULL_OBJ {
		t.Errorf("null.Type() wrong. expected=%s, got=%s", NULL_OBJ, null.Type())
	}

	if null.Inspect() != "null" {
		t.Errorf("null.Inspect() wrong. expected=%q, got=%q", "null", null.Inspect())
	}
}

// TestRealObject tests the Real object's Type and Inspect methods.
func TestRealObject(t *testing.T) {
	realVal := &Real{Value: 3.14}

	if realVal.Type() != REAL_OBJ {
		t.Errorf("realVal.Type() wrong. expected=%s, got=%s", REAL_OBJ, realVal.Type())
	}

	// fmt.Sprintf("%f", 3.14) produces "3.140000" by default
	expectedInspect := "3.140000"
	if realVal.Inspect() != expectedInspect {
		t.Errorf("realVal.Inspect() wrong. expected=%q, got=%q", expectedInspect, realVal.Inspect())
	}

	// Test with a different value
	realVal2 := &Real{Value: -123.45}
	if realVal2.Inspect() != "-123.450000" {
		t.Errorf("realVal2.Inspect() wrong. expected=%q, got=%q", "-123.450000", realVal2.Inspect())
	}
}

// TestReturnValueObject tests the ReturnValue object's Type and Inspect methods.
func TestReturnValueObject(t *testing.T) {
	rv := &ReturnValue{Value: &Integer{Value: 10}}

	if rv.Type() != RETURN_VALUE_OBJ {
		t.Errorf("rv.Type() wrong. expected=%s, got=%s", RETURN_VALUE_OBJ, rv.Type())
	}

	if rv.Inspect() != "10" {
		t.Errorf("rv.Inspect() wrong. expected=%q, got=%q", "10", rv.Inspect())
	}
}

// TestErrorObject tests the Error object's Type and Inspect methods.
func TestErrorObject(t *testing.T) {
	err := &Error{Message: "test error"}

	if err.Type() != ERROR_OBJ {
		t.Errorf("err.Type() wrong. expected=%s, got=%s", ERROR_OBJ, err.Type())
	}

	if err.Inspect() != "ERROR: test error" {
		t.Errorf("err.Inspect() wrong. expected=%q, got=%q", "ERROR: test error", err.Inspect())
	}
}

// TestFunctionObject tests the Function object's Type and Inspect methods.
func TestFunctionObject(t *testing.T) {
	// Create dummy AST nodes for testing
	params := []*ast.Identifier{
		{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"},
		{Token: token.Token{Type: token.IDENT, Literal: "y"}, Value: "y"},
	}
	body := &ast.BlockStatement{
		Token:      token.Token{Type: token.LBRACE, Literal: "{"},
		Statements: []ast.Statement{}, // Empty for simplicity
	}
	env := NewEnvironment() // Assuming NewEnvironment is accessible

	fn := &Function{Parameters: params, Body: body, Env: env}

	if fn.Type() != FUNCTION_OBJ {
		t.Errorf("fn.Type() wrong. expected=%s, got=%s", FUNCTION_OBJ, fn.Type())
	}

	expectedInspect := "fn(x, y) {\n\n}" // Based on ast.BlockStatement.String() for empty body
	if fn.Inspect() != expectedInspect {
		t.Errorf("fn.Inspect() wrong. expected=%q, got=%q", expectedInspect, fn.Inspect())
	}
}

// TestBuiltinObject tests the Builtin object's Type and Inspect methods.
func TestBuiltinObject(t *testing.T) {
	builtinFn := func(args ...Object) Object { return &Null{} }
	builtin := &Builtin{Fn: builtinFn}

	if builtin.Type() != BUILTIN_OBJ {
		t.Errorf("builtin.Type() wrong. expected=%s, got=%s", BUILTIN_OBJ, builtin.Type())
	}

	if builtin.Inspect() != "builtin function" {
		t.Errorf("builtin.Inspect() wrong. expected=%q, got=%q", "builtin function", builtin.Inspect())
	}
}

// TestArrayObject tests the Array object's Type and Inspect methods.
func TestArrayObject(t *testing.T) {
	arr := &Array{
		Elements: []Object{
			&Integer{Value: 1},
			&Boolean{Value: true},
			&String{Value: "hello"},
		},
	}

	if arr.Type() != ARRAY_OBJ {
		t.Errorf("arr.Type() wrong. expected=%s, got=%s", ARRAY_OBJ, arr.Type())
	}

	expectedInspect := "[1, true, hello]"
	if arr.Inspect() != expectedInspect {
		t.Errorf("arr.Inspect() wrong. expected=%q, got=%q", expectedInspect, arr.Inspect())
	}

	// Test empty array
	emptyArr := &Array{Elements: []Object{}}
	if emptyArr.Inspect() != "[]" {
		t.Errorf("emptyArr.Inspect() wrong. expected=%q, got=%q", "[]", emptyArr.Inspect())
	}
}

// TestHashObject tests the Hash object's Type and Inspect methods.
func TestHashObject(t *testing.T) {
	hash := &Hash{
		Pairs: map[HashKey]HashPair{
			(&String{Value: "key1"}).HashKey(): {Key: &String{Value: "key1"}, Value: &Integer{Value: 1}},
			(&String{Value: "key2"}).HashKey(): {Key: &String{Value: "key2"}, Value: &Boolean{Value: false}},
		},
	}

	if hash.Type() != HASH_OBJ {
		t.Errorf("hash.Type() wrong. expected=%s, got=%s", HASH_OBJ, hash.Type())
	}

	// Inspect output for Hash is not strictly ordered, so we check for substrings.
	inspectStr := hash.Inspect()
	if !strings.HasPrefix(inspectStr, "{") || !strings.HasSuffix(inspectStr, "}") {
		t.Errorf("hash.Inspect() missing braces. got=%q", inspectStr)
	}
	if !strings.Contains(inspectStr, "key1: 1") {
		t.Errorf("hash.Inspect() missing 'key1: 1'. got=%q", inspectStr)
	}
	if !strings.Contains(inspectStr, "key2: false") {
		t.Errorf("hash.Inspect() missing 'key2: false'. got=%q", inspectStr)
	}

	// Test empty hash
	emptyHash := &Hash{Pairs: map[HashKey]HashPair{}}
	if emptyHash.Inspect() != "{}" {
		t.Errorf("emptyHash.Inspect() wrong. expected=%q, got=%q", "{}", emptyHash.Inspect())
	}
}

// TestQuoteObject tests the Quote object's Type and Inspect methods.
func TestQuoteObject(t *testing.T) {
	// Use an ast.Identifier as a dummy ast.Node
	node := &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Value: "myVar"}
	quote := &Quote{Node: node}

	if quote.Type() != QUOTE_OBJ {
		t.Errorf("quote.Type() wrong. expected=%s, got=%s", QUOTE_OBJ, quote.Type())
	}

	expectedInspect := "QUOTE(myVar)"
	if quote.Inspect() != expectedInspect {
		t.Errorf("quote.Inspect() wrong. expected=%q, got=%q", expectedInspect, quote.Inspect())
	}
}

// TestMacroObject tests the Macro object's Type and Inspect methods.
func TestMacroObject(t *testing.T) {
	// Create dummy AST nodes for testing
	params := []*ast.Identifier{
		{Token: token.Token{Type: token.IDENT, Literal: "a"}, Value: "a"},
		{Token: token.Token{Type: token.IDENT, Literal: "b"}, Value: "b"},
	}
	body := &ast.BlockStatement{
		Token:      token.Token{Type: token.LBRACE, Literal: "{"},
		Statements: []ast.Statement{}, // Empty for simplicity
	}
	env := NewEnvironment()

	macro := &Macro{Parameters: params, Body: body, Env: env}

	if macro.Type() != MACRO_OBJ {
		t.Errorf("macro.Type() wrong. expected=%s, got=%s", MACRO_OBJ, macro.Type())
	}

	expectedInspect := "macro(a, b) {\n\n}"
	if macro.Inspect() != expectedInspect {
		t.Errorf("macro.Inspect() wrong. expected=%q, got=%q", expectedInspect, macro.Inspect())
	}
}
