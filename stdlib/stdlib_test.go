package stdlib

import (
	"beedance/ast"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"testing"
)

func TestBuiltinFunctions(t *testing.T) {
	// This test now lives in the stdlib package and tests the registered functions.
	// We need a way to evaluate expressions. We can create a minimal evaluator here
	// for testing purposes, or just test the functions directly.
	// For simplicity, we'll test the functions directly.

	tests := []struct {
		input    string
		expected interface{}
	}{
		{`LEN("")`, int64(0)},
		{`LEN("four")`, int64(4)},
		{`LEN([1, 2, 3])`, int64(3)},
		{`FIRST([1, 2, 3])`, int64(1)},
		{`LAST([1, 2, 3])`, int64(3)},
		{`REST([1, 2, 3])`, []int64{2, 3}},
		{`PUSH([1, 2], 3)`, []int64{1, 2, 3}},
		{`ADD(10, 20)`, int64(30)},
		{`SUB(20, 5)`, int64(15)},
		{`MUL(10, 5)`, int64(50)},
		{`DIV(10, 2)`, int64(5)},
		{`GT(10, 5)`, true},
		{`LT(10, 5)`, false},
	}

	// Since we can't use the full evaluator, we'll manually parse and call the functions.
	// This is more of a unit test for the built-in functions themselves.
	object.FinalizeBuiltins() // Ensure built-ins are registered for the test

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			stmt := program.Statements[0].(*ast.ExpressionStatement)
			call := stmt.Expression.(*ast.CallExpression)
			funcName := call.Function.String()

			builtin, ok := object.GetBuiltinByName(funcName)
			if !ok {
				t.Fatalf("builtin not found: %s", funcName)
			}

			// This is a simplified test evaluator for arguments
			args := []object.Object{}
			for _, argNode := range call.Arguments {
				switch node := argNode.(type) {
				case *ast.IntegerLiteral:
					args = append(args, &object.LInt{Value: node.Value})
				case *ast.StringLiteral:
					args = append(args, &object.String{Value: node.Value})
				case *ast.ArrayLiteral:
					elements := []object.Object{}
					for _, elNode := range node.Elements {
						if intLit, ok := elNode.(*ast.IntegerLiteral); ok {
							elements = append(elements, &object.LInt{Value: intLit.Value})
						}
					}
					args = append(args, &object.Array{Elements: elements})
				}
			}

			result := builtin.Fn(args...)

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, result, expected)
			case bool:
				testBooleanObject(t, result, expected)
			case []int64:
				arr, ok := result.(*object.Array)
				if !ok {
					t.Fatalf("result is not Array. got=%T", result)
				}
				if len(arr.Elements) != len(expected) {
					t.Fatalf("wrong array length. want=%d, got=%d", len(expected), len(arr.Elements))
				}
				for i, v := range expected {
					testIntegerObject(t, arr.Elements[i], v)
				}
			}
		})
	}
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) {
	t.Helper()
	result, ok := obj.(*object.LInt)
	if !ok {
		t.Fatalf("object is not Integer. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
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
