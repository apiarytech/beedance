package evaluator

import (
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

func TestEXPR(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`EXPR(5);`,
			`5`,
		},
		{
			`EXPR(5 + 8);`,
			`(5 + 8)`,
		},
		{
			`EXPR(foobar);`,
			`foobar`,
		},
		{
			`EXPR(foobar + barfoo);`,
			`(foobar + barfoo)`,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input) // This helper checks for parser errors
		quote, ok := evaluated.(*object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)",
				evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		if quote.Node.String() != tt.expected {
			t.Errorf("not equal. got=%q, want=%q",
				quote.Node.String(), tt.expected)
		}
	}
}

func TestEXPREVAL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`EXPR(EVAL(4));`, `4`},
		{`EXPR(EVAL(4 + 4));`, `8`},
		{`EXPR(8 + EVAL(4 + 4));`, `(8 + 8)`},
		{`EXPR(EVAL(4 + 4) + 8);`, `(8 + 8)`},
		{`VAR foobar : INT := 8; END_VAR EXPR(EVAL(foobar));`, `8`},
		{`EXPR(EVAL(TRUE));`, `TRUE`},
		{`EXPR(EVAL(TRUE = FALSE));`, `FALSE`},
		{`EXPR(EVAL(EXPR(4 + 4)));`, `(4 + 4)`},
		{`VAR quotedInfixExpression : MACRO := EXPR(4 + 4); END_VAR EXPR(EVAL(4 + 4) + EVAL(quotedInfixExpression));`, `(8 + (4 + 4))`},
		// Tests for wrong number of arguments to EVAL, which should not be evaluated.
		{`EXPR(EVAL());`, `EVAL()`},
		{`EXPR(EVAL(1, 2));`, `EVAL(1, 2)`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)
			quote, ok := evaluated.(*object.Quote)
			if !ok {
				t.Fatalf("expected *object.Quote. got=%T (%+v)",
					evaluated, evaluated)
			}

			if quote.Node == nil {
				t.Fatalf("quote.Node is nil")
			}

			if quote.Node.String() != tt.expected {
				t.Errorf("not equal. got=%q, want=%q",
					quote.Node.String(), tt.expected)
			}
		})
	}
}

func TestConvertObjectToASTNode(t *testing.T) {
	tests := []struct {
		name     string
		obj      object.Object
		expected func(t *testing.T, node ast.Node)
	}{
		{
			name: "SInt to IntegerLiteral",
			obj:  &object.SInt{Value: 123},
			expected: func(t *testing.T, node ast.Node) {
				il, ok := node.(*ast.IntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.IntegerLiteral, got %T", node)
				}
				if il.Value != 123 {
					t.Errorf("expected value 123, got %d", il.Value)
				}
			},
		},
		{
			name: "Int to IntegerLiteral",
			obj:  &object.Int{Value: 12345},
			expected: func(t *testing.T, node ast.Node) {
				il, ok := node.(*ast.IntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.IntegerLiteral, got %T", node)
				}
				if il.Value != 12345 {
					t.Errorf("expected value 12345, got %d", il.Value)
				}
			},
		},
		{
			name: "DInt to IntegerLiteral",
			obj:  &object.DInt{Value: 123456789},
			expected: func(t *testing.T, node ast.Node) {
				il, ok := node.(*ast.IntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.IntegerLiteral, got %T", node)
				}
				if il.Value != 123456789 {
					t.Errorf("expected value 123456789, got %d", il.Value)
				}
			},
		},
		{
			name: "LInt to IntegerLiteral",
			obj:  &object.LInt{Value: 9876543210},
			expected: func(t *testing.T, node ast.Node) {
				il, ok := node.(*ast.IntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.IntegerLiteral, got %T", node)
				}
				if il.Value != 9876543210 {
					t.Errorf("expected value 9876543210, got %d", il.Value)
				}
			},
		},
		{
			name: "USInt to UnsignedIntegerLiteral",
			obj:  &object.USInt{Value: 255},
			expected: func(t *testing.T, node ast.Node) {
				uil, ok := node.(*ast.UnsignedIntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.UnsignedIntegerLiteral, got %T", node)
				}
				if uil.Value != 255 {
					t.Errorf("expected value 255, got %d", uil.Value)
				}
			},
		},
		{
			name: "UInt to UnsignedIntegerLiteral",
			obj:  &object.UInt{Value: 65535},
			expected: func(t *testing.T, node ast.Node) {
				uil, ok := node.(*ast.UnsignedIntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.UnsignedIntegerLiteral, got %T", node)
				}
				if uil.Value != 65535 {
					t.Errorf("expected value 65535, got %d", uil.Value)
				}
			},
		},
		{
			name: "UDInt to UnsignedIntegerLiteral",
			obj:  &object.UDInt{Value: 4294967295},
			expected: func(t *testing.T, node ast.Node) {
				uil, ok := node.(*ast.UnsignedIntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.UnsignedIntegerLiteral, got %T", node)
				}
				if uil.Value != 4294967295 {
					t.Errorf("expected value 4294967295, got %d", uil.Value)
				}
			},
		},
		{
			name: "ULInt to UnsignedIntegerLiteral",
			obj:  &object.ULInt{Value: 18446744073709551615},
			expected: func(t *testing.T, node ast.Node) {
				uil, ok := node.(*ast.UnsignedIntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.UnsignedIntegerLiteral, got %T", node)
				}
				if uil.Value != 18446744073709551615 {
					t.Errorf("expected value 18446744073709551615, got %d", uil.Value)
				}
			},
		},
		{
			name: "Boolean true to BooleanLiteral",
			obj:  TRUE,
			expected: func(t *testing.T, node ast.Node) {
				b, ok := node.(*ast.Boolean)
				if !ok {
					t.Fatalf("expected *ast.Boolean, got %T", node)
				}
				if !b.Value {
					t.Errorf("expected value true, got %t", b.Value)
				}
			},
		},
		{
			name: "Boolean false to BooleanLiteral",
			obj:  FALSE,
			expected: func(t *testing.T, node ast.Node) {
				b, ok := node.(*ast.Boolean)
				if !ok {
					t.Fatalf("expected *ast.Boolean, got %T", node)
				}
				if b.Value {
					t.Errorf("expected value false, got %t", b.Value)
				}
			},
		},
		{
			name: "Real to RealLiteral",
			obj:  &object.Real{Value: 3.14},
			expected: func(t *testing.T, node ast.Node) {
				rl, ok := node.(*ast.RealLiteral)
				if !ok {
					t.Fatalf("expected *ast.RealLiteral, got %T", node)
				}
				if rl.Value != 3.14 {
					t.Errorf("expected value 3.14, got %f", rl.Value)
				}
			},
		},
		{
			name: "BitString (BYTE) to BitStringLiteral",
			obj:  &object.BitString{Value: 0xA5, Width: 8},
			expected: func(t *testing.T, node ast.Node) {
				bsl, ok := node.(*ast.BitStringLiteral)
				if !ok {
					t.Fatalf("expected *ast.BitStringLiteral, got %T", node)
				}
				if bsl.Value != 0xA5 {
					t.Errorf("expected value 0xA5, got %X", bsl.Value)
				}
				if bsl.Width != 8 {
					t.Errorf("expected width 8, got %d", bsl.Width)
				}
			},
		},
		{
			name: "BitString (WORD) to BitStringLiteral",
			obj:  &object.BitString{Value: 0xABCD, Width: 16},
			expected: func(t *testing.T, node ast.Node) {
				bsl, ok := node.(*ast.BitStringLiteral)
				if !ok {
					t.Fatalf("expected *ast.BitStringLiteral, got %T", node)
				}
				if bsl.Value != 0xABCD {
					t.Errorf("expected value 0xABCD, got %X", bsl.Value)
				}
				if bsl.Width != 16 {
					t.Errorf("expected width 16, got %d", bsl.Width)
				}
			},
		},
		{
			name: "BitString (DWORD) to BitStringLiteral",
			obj:  &object.BitString{Value: 0x1234ABCD, Width: 32},
			expected: func(t *testing.T, node ast.Node) {
				bsl, ok := node.(*ast.BitStringLiteral)
				if !ok {
					t.Fatalf("expected *ast.BitStringLiteral, got %T", node)
				}
				if bsl.Value != 0x1234ABCD {
					t.Errorf("expected value 0x1234ABCD, got %X", bsl.Value)
				}
				if bsl.Width != 32 {
					t.Errorf("expected width 32, got %d", bsl.Width)
				}
			},
		},
		{
			name: "BitString (unsupported width) returns nil",
			obj:  &object.BitString{Value: 0x1, Width: 24}, // 24 is not a standard width
			expected: func(t *testing.T, node ast.Node) {
				if node != nil {
					t.Errorf("expected nil for unsupported bitstring width, got %T", node)
				}
			},
		},
		{
			name: "BitString (LWORD) to BitStringLiteral",
			obj:  &object.BitString{Value: 0x1234567890ABCDEF, Width: 64},
			expected: func(t *testing.T, node ast.Node) {
				bsl, ok := node.(*ast.BitStringLiteral)
				if !ok {
					t.Fatalf("expected *ast.BitStringLiteral, got %T", node)
				}
				if bsl.Value != 0x1234567890ABCDEF {
					t.Errorf("expected value 0x1234567890ABCDEF, got %X", bsl.Value)
				}
				if bsl.Width != 64 {
					t.Errorf("expected width 64, got %d", bsl.Width)
				}
			},
		},
		{
			name: "String to StringLiteral",
			obj:  &object.String{Value: "hello"},
			expected: func(t *testing.T, node ast.Node) {
				sl, ok := node.(*ast.StringLiteral)
				if !ok {
					t.Fatalf("expected *ast.StringLiteral, got %T", node)
				}
				if sl.Value != "hello" {
					t.Errorf("expected value 'hello', got %q", sl.Value)
				}
			},
		},
		{
			name: "WString to WStringLiteral",
			obj:  &object.WString{Value: "world"},
			expected: func(t *testing.T, node ast.Node) {
				wsl, ok := node.(*ast.WStringLiteral)
				if !ok {
					t.Fatalf("expected *ast.WStringLiteral, got %T", node)
				}
				if wsl.Value != "world" {
					t.Errorf("expected value 'world', got %q", wsl.Value)
				}
			},
		},
		{
			name: "Time to Identifier",
			obj:  &object.Time{Value: 5 * time.Second},
			expected: func(t *testing.T, node ast.Node) {
				ident, ok := node.(*ast.Identifier)
				if !ok {
					t.Fatalf("expected *ast.Identifier, got %T", node)
				}
				if ident.Value != "T#5s" {
					t.Errorf("expected value 'T#5s', got %q", ident.Value)
				}
			},
		},
		{
			name: "Date to Identifier",
			obj:  &object.Date{Value: time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)},
			expected: func(t *testing.T, node ast.Node) {
				ident, ok := node.(*ast.Identifier)
				if !ok {
					t.Fatalf("expected *ast.Identifier, got %T", node)
				}
				if ident.Value != "D#2026-05-21" {
					t.Errorf("expected value 'D#2026-05-21', got %q", ident.Value)
				}
			},
		},
		{
			name: "TimeOfDay to Identifier",
			obj:  &object.TimeOfDay{Value: time.Date(0, 1, 1, 14, 30, 0, 0, time.UTC)},
			expected: func(t *testing.T, node ast.Node) {
				ident, ok := node.(*ast.Identifier)
				if !ok {
					t.Fatalf("expected *ast.Identifier, got %T", node)
				}
				if ident.Value != "TOD#14:30:00" {
					t.Errorf("expected value 'TOD#14:30:00', got %q", ident.Value)
				}
			},
		},
		{
			name: "Quote to its inner Node",
			obj:  &object.Quote{Node: &ast.IntegerLiteral{Value: 42}},
			expected: func(t *testing.T, node ast.Node) {
				il, ok := node.(*ast.IntegerLiteral)
				if !ok {
					t.Fatalf("expected *ast.IntegerLiteral, got %T", node)
				}
				if il.Value != 42 {
					t.Errorf("expected value 42, got %d", il.Value)
				}
			},
		},
		{
			name: "Unknown object type returns nil",
			obj:  &object.Function{}, // An object type not handled by convertObjectToASTNode
			expected: func(t *testing.T, node ast.Node) {
				if node != nil {
					t.Errorf("expected nil for unhandled object type, got %T", node)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := convertObjectToASTNode(tt.obj)
			tt.expected(t, node)
		})
	}
}
