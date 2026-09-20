package object

import (
	"math"
	"strings"
	"testing"
	"time"

	"beedance/ast"
	"beedance/token"
)

func TestStringHashKey(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same value", &String{Value: "Hello World"}, &String{Value: "Hello World"}, true},
		{"different value", &String{Value: "Hello World"}, &String{Value: "My name is johnny"}, false},
		{"different types", &String{Value: "1"}, &LInt{Value: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestBooleanHashKey(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same true", &Boolean{Value: true}, &Boolean{Value: true}, true},
		{"same false", &Boolean{Value: false}, &Boolean{Value: false}, true},
		{"true and false", &Boolean{Value: true}, &Boolean{Value: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestIntegerHashKey(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same value", &LInt{Value: 1}, &LInt{Value: 1}, true},
		{"different value", &LInt{Value: 1}, &LInt{Value: 2}, false},
		{"different types", &LInt{Value: 1}, &String{Value: "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

// TestRealHashKey tests the Real object's HashKey method.
func TestRealHashKey(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same positive value", &Real{Value: 1.1}, &Real{Value: 1.1}, true},
		{"different positive value", &Real{Value: 1.1}, &Real{Value: 2.2}, false},
		{"same negative value", &Real{Value: -1.1}, &Real{Value: -1.1}, true},
		{"positive and negative", &Real{Value: 1.1}, &Real{Value: -1.1}, false},
		{"zero", &Real{Value: 0.0}, &Real{Value: 0.0}, true},
		{"NaN", &Real{Value: math.NaN()}, &Real{Value: math.NaN()}, true},
		{"different types", &Real{Value: 1.1}, &String{Value: "1.1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

// TestFunctionObject tests the Function object's Type and Inspect methods.
func TestFunctionObject(t *testing.T) {
	tests := []struct {
		name        string
		fnName      *ast.Identifier
		varInputs   []*ast.VarDeclStatement
		varOutputs  []*ast.VarDeclStatement
		varInOuts   []*ast.VarDeclStatement
		body        *ast.BlockStatement
		wantType    ObjectType
		wantInspect string
	}{
		{
			name:        "function with no vars",
			fnName:      &ast.Identifier{Value: "myFunc"},
			body:        &ast.BlockStatement{Statements: []ast.Statement{}},
			wantType:    FUNCTION_OBJ,
			wantInspect: "FUNCTION myFunc ()",
		},
		{
			name:   "function with VarInputs",
			fnName: &ast.Identifier{Value: "funcWithInputs"},
			varInputs: []*ast.VarDeclStatement{
				{
					Token:    token.Token{Type: token.VAR_INPUT, Literal: "VAR_INPUT"},
					Name:     &ast.Identifier{Value: "in1"},
					DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
				},
			},
			body:        &ast.BlockStatement{Statements: []ast.Statement{}},
			wantType:    FUNCTION_OBJ,
			wantInspect: "FUNCTION funcWithInputs (VAR_INPUT in1 : INT;)",
		},
		{
			name:   "function with VarOutputs",
			fnName: &ast.Identifier{Value: "funcWithOutputs"},
			varOutputs: []*ast.VarDeclStatement{
				{
					Token:    token.Token{Type: token.VAR_OUTPUT, Literal: "VAR_OUTPUT"},
					Name:     &ast.Identifier{Value: "out1"},
					DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}},
				},
			},
			body:        &ast.BlockStatement{Statements: []ast.Statement{}},
			wantType:    FUNCTION_OBJ,
			wantInspect: "FUNCTION funcWithOutputs (VAR_OUTPUT out1 : BOOL;)",
		},
		{
			name:   "function with VarInOuts",
			fnName: &ast.Identifier{Value: "funcWithInOuts"},
			varInOuts: []*ast.VarDeclStatement{
				{
					Token:    token.Token{Type: token.VAR_IN_OUT, Literal: "VAR_IN_OUT"},
					Name:     &ast.Identifier{Value: "inout1"},
					DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.REAL, Literal: "REAL"}},
				},
			},
			body:        &ast.BlockStatement{Statements: []ast.Statement{}},
			wantType:    FUNCTION_OBJ,
			wantInspect: "FUNCTION funcWithInOuts (VAR_IN_OUT inout1 : REAL;)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &Function{
				Name: tt.fnName, Body: tt.body, Env: NewEnvironment(),
				VarInputs: tt.varInputs, VarOutputs: tt.varOutputs, VarInOuts: tt.varInOuts,
			}

			if fn.Type() != tt.wantType {
				t.Errorf("fn.Type() wrong. got=%s, want=%s", fn.Type(), tt.wantType)
			}
			if fn.Inspect() != tt.wantInspect {
				t.Errorf("fn.Inspect() wrong. got=%q, want=%q", fn.Inspect(), tt.wantInspect)
			}
		})
	}
}

// TestSimpleObjectInspection consolidates tests for Type() and Inspect() on simple objects.
func TestSimpleObjectInspection(t *testing.T) {
	tests := []struct {
		name        string
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		{"Null", &Null{}, NULL_OBJ, "null"},
		{"Real positive", &Real{Value: 3.14}, REAL_OBJ, "3.140000"},
		{"Real negative", &Real{Value: -123.45}, REAL_OBJ, "-123.450000"},
		{"ReturnValue", &ReturnValue{Value: &LInt{Value: 10}}, RETURN_VALUE_OBJ, "10"},
		{"Error", &Error{Message: "test error"}, ERROR_OBJ, "ERROR: test error"},
		{"Builtin", &Builtin{Fn: func(args ...Object) Object { return nil }}, BUILTIN_OBJ, "builtin function"},
		{"Quote", &Quote{Node: &ast.Identifier{Value: "myVar"}}, QUOTE_OBJ, "QUOTE(myVar)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Type() != tt.wantType {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.obj.Type(), tt.wantType)
			}
			if tt.obj.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.obj.Inspect(), tt.wantInspect)
			}
		})
	}
}

// TestArrayObject tests the Array object's Type and Inspect methods.
func TestArrayObject(t *testing.T) {
	tests := []struct {
		name        string
		obj         Object
		wantInspect string
	}{
		{"populated array", &Array{Elements: []Object{&LInt{Value: 1}, &String{Value: "hello"}}}, "[1, hello]"},
		{"empty array", &Array{Elements: []Object{}}, "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Type() != ARRAY_OBJ {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.obj.Type(), ARRAY_OBJ)
			}
			if tt.obj.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.obj.Inspect(), tt.wantInspect)
			}
		})
	}
}

// TestHashObject tests the Hash object's Type and Inspect methods.
func TestHashObject(t *testing.T) {
	tests := []struct {
		name           string
		hash           *Hash
		expectedSubstr []string
		exactMatch     string
	}{
		{
			name: "populated hash",
			hash: &Hash{Pairs: map[HashKey]HashPair{
				(&String{Value: "key1"}).HashKey(): {Key: &String{Value: "key1"}, Value: &LInt{Value: 1}},
				(&String{Value: "key2"}).HashKey(): {Key: &String{Value: "key2"}, Value: &Boolean{Value: false}},
			}},
			expectedSubstr: []string{"key1: 1", "key2: false"},
		},
		{"empty hash", &Hash{Pairs: map[HashKey]HashPair{}}, nil, "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inspectStr := tt.hash.Inspect()
			for _, substr := range tt.expectedSubstr {
				if !strings.Contains(inspectStr, substr) {
					t.Errorf("Inspect() missing substring %q. got=%q", substr, inspectStr)
				}
			}
			if tt.exactMatch != "" && inspectStr != tt.exactMatch {
				t.Errorf("Inspect() wrong. got=%q, want=%q", inspectStr, tt.exactMatch)
			}
		})
	}
}

// TestMacroObject tests the Macro object's Type and Inspect methods.
func TestMacroObject(t *testing.T) {
	tests := []struct {
		name        string
		params      []*ast.Identifier
		wantInspect string
	}{
		{
			"two parameters",
			[]*ast.Identifier{{Value: "a"}, {Value: "b"}},
			"macro(a, b) {\n\n}",
		},
		{
			"no parameters",
			[]*ast.Identifier{},
			"macro() {\n\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &ast.BlockStatement{Statements: []ast.Statement{}}
			macro := &Macro{Parameters: tt.params, Body: body, Env: NewEnvironment()}

			if macro.Type() != MACRO_OBJ {
				t.Errorf("macro.Type() wrong. got=%s, want=%s", macro.Type(), MACRO_OBJ)
			}
			if macro.Inspect() != tt.wantInspect {
				t.Errorf("macro.Inspect() wrong. got=%q, want=%q", macro.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestTimeDateObjects(t *testing.T) {
	timeVal, _ := time.ParseDuration("5s300ms")
	dateVal, _ := time.Parse("2006-01-02", "2026-05-20")
	todVal, _ := time.Parse("15:04:05.999", "14:30:05.123")
	dtVal, _ := time.Parse("2006-01-02-15:04:05.999", "2026-05-20-14:30:05.123")

	tests := []struct {
		name        string
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		{"Time", &Time{Value: timeVal}, TIME_OBJ, "T#5.3s"},
		{"Date", &Date{Value: dateVal}, DATE_OBJ, "D#2026-05-20"},
		{"TimeOfDay", &TimeOfDay{Value: todVal}, TIME_OF_DAY_OBJ, "TOD#14:30:05.123"},
		{"DateAndTime", &DateAndTime{Value: dtVal}, DATE_AND_TIME_OBJ, "DT#2026-05-20-14:30:05.123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Type() != tt.wantType {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.obj.Type(), tt.wantType)
			}
			if tt.obj.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.obj.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestFunctionBlockObject(t *testing.T) {
	tests := []struct {
		name        string
		fb          *FunctionBlock
		wantInspect string
	}{
		{
			name: "FB with no vars",
			fb: &FunctionBlock{
				Name: &ast.Identifier{Value: "MyFB"},
			},
			wantInspect: "FUNCTION_BLOCK MyFB ()",
		},
		{
			name: "FB with VarInputs",
			fb: &FunctionBlock{
				Name: &ast.Identifier{Value: "FB_Inputs"},
				VarInputs: []*ast.VarDeclStatement{{
					Name:     &ast.Identifier{Value: "In1"},
					DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
				}},
			},
			wantInspect: "FUNCTION_BLOCK FB_Inputs (VAR_INPUT In1 : INT;)",
		},
		{
			name: "FB with VarOutputs",
			fb: &FunctionBlock{
				Name:       &ast.Identifier{Value: "FB_Outputs"},
				VarOutputs: []*ast.VarDeclStatement{{Name: &ast.Identifier{Value: "Out1"}, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}}},
			},
			wantInspect: "FUNCTION_BLOCK FB_Outputs (VAR_OUTPUT Out1 : BOOL;)",
		},
		{
			name: "FB with VarInOuts",
			fb: &FunctionBlock{
				Name:      &ast.Identifier{Value: "FB_InOuts"},
				VarInOuts: []*ast.VarDeclStatement{{Name: &ast.Identifier{Value: "InOut1"}, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.REAL, Literal: "REAL"}}}},
			},
			wantInspect: "FUNCTION_BLOCK FB_InOuts (VAR_IN_OUT InOut1 : REAL;)",
		},
		{
			name: "FB with all var types",
			fb: &FunctionBlock{
				Name: &ast.Identifier{Value: "FB_Complex"},
				VarInputs: []*ast.VarDeclStatement{
					{Name: &ast.Identifier{Value: "In1"}, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}},
				},
				VarOutputs: []*ast.VarDeclStatement{
					{Name: &ast.Identifier{Value: "Out1"}, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}},
				},
				VarInOuts: []*ast.VarDeclStatement{
					{Name: &ast.Identifier{Value: "InOut1"}, DataType: &ast.TypeSpecifier{Token: token.Token{Type: token.REAL, Literal: "REAL"}}},
				},
			},
			wantInspect: "FUNCTION_BLOCK FB_Complex (VAR_INPUT In1 : INT; VAR_OUTPUT Out1 : BOOL; VAR_IN_OUT InOut1 : REAL;)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set a dummy environment and body, as they are not used by Inspect()
			tt.fb.Env = NewEnvironment()
			tt.fb.Body = &ast.BlockStatement{}

			if tt.fb.Type() != FUNCTION_BLOCK_OBJ {
				t.Errorf("fb.Type() wrong. got=%s, want=%s", tt.fb.Type(), FUNCTION_BLOCK_OBJ)
			}
			if tt.fb.Inspect() != tt.wantInspect {
				t.Errorf("fb.Inspect() wrong.\ngot:  %q\nwant: %q", tt.fb.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestSfcObjects(t *testing.T) {
	tests := []struct {
		name        string
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		{"SFC", &SFC{}, SFC_OBJ, "SFC"},
		{"Step", &Step{Name: &ast.Identifier{Value: "S1"}}, STEP_OBJ, "STEP S1"},
		{"Transition", &Transition{}, TRANSITION_OBJ, "TRANSITION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Type() != tt.wantType {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.obj.Type(), tt.wantType)
			}
			if tt.obj.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.obj.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestFunctionBlockInstanceObject(t *testing.T) {
	fbDef := &FunctionBlock{
		Name: &ast.Identifier{Value: "MyFB"},
	}
	fbInstance := &FunctionBlockInstance{
		Definition: fbDef,
		Env:        NewEnvironment(),
	}

	if fbInstance.Type() != FUNCTION_BLOCK_INSTANCE_OBJ {
		t.Errorf("fbInstance.Type() wrong. got=%s, want=%s", fbInstance.Type(), FUNCTION_BLOCK_INSTANCE_OBJ)
	}

	expectedInspect := "FUNCTION_BLOCK_INSTANCE(MyFB)"
	if fbInstance.Inspect() != expectedInspect {
		t.Errorf("fbInstance.Inspect() wrong. got=%q, want=%q", fbInstance.Inspect(), expectedInspect)
	}
}

func TestBitStringObject(t *testing.T) {
	tests := []struct {
		name        string
		bitstring   *BitString
		wantInspect string
	}{
		{
			name:        "width 8",
			bitstring:   &BitString{Value: 0xAB, Width: 8},
			wantInspect: "BYTE#16#AB",
		},
		{
			name:        "width 16",
			bitstring:   &BitString{Value: 0xABCD, Width: 16},
			wantInspect: "WORD#16#ABCD",
		},
		{
			name:        "width 32",
			bitstring:   &BitString{Value: 0xABCDEF01, Width: 32},
			wantInspect: "DWORD#16#ABCDEF01",
		},
		{
			name:        "width 64",
			bitstring:   &BitString{Value: 0x1234567890ABCDEF, Width: 64},
			wantInspect: "LWORD#16#1234567890ABCDEF",
		},
		{
			name:        "default width",
			bitstring:   &BitString{Value: 0xFF, Width: 12}, // Custom width
			wantInspect: "BITSTRING#12#FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bitstring.Type() != BITSTRING_OBJ {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.bitstring.Type(), BITSTRING_OBJ)
			}
			if tt.bitstring.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.bitstring.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestLRealObject(t *testing.T) {
	lreal := &LReal{Value: 123.456}

	if lreal.Type() != LREAL_OBJ {
		t.Errorf("lreal.Type() wrong. expected=%s, got=%s", LREAL_OBJ, lreal.Type())
	}

	expectedInspect := "123.456000"
	if lreal.Inspect() != expectedInspect {
		t.Errorf("lreal.Inspect() wrong. expected=%q, got=%q", expectedInspect, lreal.Inspect())
	}

	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same value", &LReal{Value: 1.1}, &LReal{Value: 1.1}, true},
		{"different value", &LReal{Value: 1.1}, &LReal{Value: 2.2}, false},
		{"different types", &LReal{Value: 1.1}, &Real{Value: 1.1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name+" hashkey", func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestWStringObject(t *testing.T) {
	wstr := &WString{Value: "wide string"}

	if wstr.Type() != WSTRING_OBJ {
		t.Errorf("wstr.Type() wrong. expected=%s, got=%s", WSTRING_OBJ, wstr.Type())
	}

	if wstr.Inspect() != "wide string" {
		t.Errorf("wstr.Inspect() wrong. expected=%q, got=%q", "wide string", wstr.Inspect())
	}

	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		{"same value", &WString{Value: "hello"}, &WString{Value: "hello"}, true},
		{"different value", &WString{Value: "hello"}, &WString{Value: "world"}, false},
		{"different types", &WString{Value: "hello"}, &String{Value: "hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name+" hashkey", func(t *testing.T) {
			if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s and %s was %v, want %v",
					tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestBitStringObjects(t *testing.T) {
	testCases := []struct {
		name        string
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		{"Byte", &Byte{Value: 0x1A}, BYTE_OBJ, "BYTE#16#1A"},
		{"Word", &Word{Value: 0x1234}, WORD_OBJ, "WORD#16#1234"},
		{"DWord", &DWord{Value: 0x12345678}, DWORD_OBJ, "DWORD#16#12345678"},
		{"LWord", &LWord{Value: 0x123456789ABCDEF0}, LWORD_OBJ, "LWORD#16#123456789ABCDEF0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name+" type and inspect", func(t *testing.T) {
			if tc.obj.Type() != tc.wantType {
				t.Errorf("Type() wrong. got=%s, want=%s", tc.obj.Type(), tc.wantType)
			}
			if tc.obj.Inspect() != tc.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tc.obj.Inspect(), tc.wantInspect)
			}
		})
	}
}

func TestBitStringHashKeys(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		// Byte
		{"same byte", &Byte{Value: 1}, &Byte{Value: 1}, true},
		{"different byte", &Byte{Value: 1}, &Byte{Value: 2}, false},
		{"byte vs integer", &Byte{Value: 1}, &LInt{Value: 1}, false},

		// Word
		{"same word", &Word{Value: 100}, &Word{Value: 100}, true},
		{"different word", &Word{Value: 100}, &Word{Value: 200}, false},
		{"word vs integer", &Word{Value: 100}, &LInt{Value: 100}, false},

		// DWord
		{"same dword", &DWord{Value: 1000}, &DWord{Value: 1000}, true},
		{"different dword", &DWord{Value: 1000}, &DWord{Value: 2000}, false},
		{"dword vs integer", &DWord{Value: 1000}, &LInt{Value: 1000}, false},

		// LWord
		{"same lword", &LWord{Value: 10000}, &LWord{Value: 10000}, true},
		{"different lword", &LWord{Value: 10000}, &LWord{Value: 20000}, false},
		{"lword vs integer", &LWord{Value: 10000}, &LInt{Value: 10000}, false},

		// Cross bit-string types
		{"byte vs word", &Byte{Value: 1}, &Word{Value: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashA := tt.a.(Hashable).HashKey()
			hashB := tt.b.(Hashable).HashKey()
			if (hashA == hashB) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s (%v) and %s (%v) was %v, want %v",
					tt.a.Inspect(), hashA, tt.b.Inspect(), hashB, !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestExtendedObjectInspection(t *testing.T) {
	fbDef := &FunctionBlock{Name: &ast.Identifier{Value: "MyFB"}}
	progDef := &Program{Name: &ast.Identifier{Value: "MyProg"}}

	tests := []struct {
		name        string
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		// Specific Integer Types
		{"SInt", &SInt{Value: -8}, SINT_OBJ, "-8"},
		{"Int", &Int{Value: -16}, INT_OBJ, "-16"},
		{"DInt", &DInt{Value: -32}, DINT_OBJ, "-32"},
		{"LInt", &LInt{Value: -64}, LINT_OBJ, "-64"},
		{"USInt", &USInt{Value: 8}, USINT_OBJ, "8"},
		{"UInt", &UInt{Value: 16}, UINT_OBJ, "16"},
		{"UDInt", &UDInt{Value: 32}, UDINT_OBJ, "32"},
		{"ULInt", &ULInt{Value: 64}, ULINT_OBJ, "64"},

		// Internal/Flow Control Objects
		{"Exit", &Exit{}, EXIT_OBJ, "EXIT"},
		{"Jump", &Jump{TargetLabel: "LBL1"}, JUMP_OBJ, "JUMP to LBL1"},
		{"Return", &Return{}, RETURN_OBJ, "RETURN"},

		// POU and Instance Objects
		{"BuiltinFunctionBlock", &BuiltinFunctionBlock{}, BUILTIN_FUNCTION_BLOCK_OBJ, "builtin function block"},
		{"Pointer", &Pointer{Name: "targetVar"}, POINTER_OBJ, "POINTER(targetVar)"},
		{"SubrangeType", &SubrangeType{Name: "MyRange", BaseType: INT_OBJ, LowerBound: 0, UpperBound: 100}, SUBRANGE_TYPE_OBJ, "SUBRANGE INT (0..100)"},
		{"Action", &Action{Name: &ast.Identifier{Value: "MyAction"}}, ACTION_OBJ, "ACTION MyAction"},
		{"Task", &Task{Name: "MyTask", Priority: 1, Interval: time.Second}, "TASK", "TASK(MyTask, Priority: 1, Interval: 1s)"},
		{"Scheduler", &Scheduler{}, "SCHEDULER", "SCHEDULER()"},
		{"FunctionBlock", fbDef, FUNCTION_BLOCK_OBJ, "FUNCTION_BLOCK MyFB ()"},
		{"Program", progDef, PROGRAM_OBJ, "PROGRAM MyProg ()"},
		{"ProgramInstance", &ProgramInstance{Definition: progDef}, PROGRAM_INSTANCE_OBJ, "INSTANCE OF MyProg"},
		{"Namespace", &Namespace{Name: "MyLib"}, NAMESPACE_OBJ, "NAMESPACE(MyLib)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.obj.Type() != tt.wantType {
				t.Errorf("Type() wrong. got=%s, want=%s", tt.obj.Type(), tt.wantType)
			}
			if tt.obj.Inspect() != tt.wantInspect {
				t.Errorf("Inspect() wrong. got=%q, want=%q", tt.obj.Inspect(), tt.wantInspect)
			}
		})
	}
}

func TestIntegerTypesHashKeys(t *testing.T) {
	tests := []struct {
		name      string
		a, b      Object
		wantEqual bool
	}{
		// SInt
		{"same sint", &SInt{Value: 1}, &SInt{Value: 1}, true},
		{"different sint", &SInt{Value: 1}, &SInt{Value: 2}, false},
		// Int
		{"same int", &Int{Value: 1}, &Int{Value: 1}, true},
		{"different int", &Int{Value: 1}, &Int{Value: 2}, false},
		// DInt
		{"same dint", &DInt{Value: 1}, &DInt{Value: 1}, true},
		{"different dint", &DInt{Value: 1}, &DInt{Value: 2}, false},
		// LInt
		{"same lint", &LInt{Value: 1}, &LInt{Value: 1}, true},
		{"different lint", &LInt{Value: 1}, &LInt{Value: 2}, false},
		// USInt
		{"same usint", &USInt{Value: 1}, &USInt{Value: 1}, true},
		{"different usint", &USInt{Value: 1}, &USInt{Value: 2}, false},
		// UInt
		{"same uint", &UInt{Value: 1}, &UInt{Value: 1}, true},
		{"different uint", &UInt{Value: 1}, &UInt{Value: 2}, false},
		// UDInt
		{"same udint", &UDInt{Value: 1}, &UDInt{Value: 1}, true},
		{"different udint", &UDInt{Value: 1}, &UDInt{Value: 2}, false},
		// ULInt
		{"same ulint", &ULInt{Value: 1}, &ULInt{Value: 1}, true},
		{"different ulint", &ULInt{Value: 1}, &ULInt{Value: 2}, false},
		// Cross-type
		{"sint vs int", &SInt{Value: 1}, &Int{Value: 1}, false},
		{"usint vs uint", &USInt{Value: 1}, &UInt{Value: 1}, false},
		{"sint vs usint", &SInt{Value: 1}, &USInt{Value: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashA := tt.a.(Hashable).HashKey()
			hashB := tt.b.(Hashable).HashKey()
			if (hashA == hashB) != tt.wantEqual {
				t.Errorf("HashKey() equality for %s (%v) and %s (%v) was %v, want %v",
					tt.a.Inspect(), hashA, tt.b.Inspect(), hashB, !tt.wantEqual, tt.wantEqual)
			}
		})
	}
}

func TestEnumeratedObjects(t *testing.T) {
	// Setup for the test
	red := &EnumeratedValue{TypeName: "COLOR", Value: "RED"}
	green := &EnumeratedValue{TypeName: "COLOR", Value: "GREEN"}

	colorType := &EnumeratedType{
		Name: "COLOR",
		Values: map[string]*EnumeratedValue{
			"RED":   red,
			"GREEN": green,
		},
	}

	t.Run("EnumeratedType inspection", func(t *testing.T) {
		if colorType.Type() != ENUMERATED_TYPE_OBJ {
			t.Errorf("colorType.Type() wrong. got=%s, want=%s", colorType.Type(), ENUMERATED_TYPE_OBJ)
		}
		inspectStr := colorType.Inspect()
		if !strings.HasPrefix(inspectStr, "ENUM(COLOR: ") {
			t.Errorf("Inspect() should start with 'ENUM(COLOR: '. got=%q", inspectStr)
		}
		if !strings.Contains(inspectStr, "RED") || !strings.Contains(inspectStr, "GREEN") {
			t.Errorf("Inspect() should contain all enum values. got=%q", inspectStr)
		}
	})

	t.Run("EnumeratedValue inspection", func(t *testing.T) {
		if red.Type() != ENUMERATED_VALUE_OBJ {
			t.Errorf("red.Type() wrong. got=%s, want=%s", red.Type(), ENUMERATED_VALUE_OBJ)
		}
		if red.Inspect() != "COLOR#RED" {
			t.Errorf("red.Inspect() wrong. got=%q, want=%q", red.Inspect(), "COLOR#RED")
		}
	})

	t.Run("EnumeratedValue hashkey", func(t *testing.T) {
		tests := []struct {
			name      string
			a, b      Object
			wantEqual bool
		}{
			{"same enum value", &EnumeratedValue{TypeName: "COLOR", Value: "RED"}, &EnumeratedValue{TypeName: "COLOR", Value: "RED"}, true},
			{"different enum value", &EnumeratedValue{TypeName: "COLOR", Value: "RED"}, &EnumeratedValue{TypeName: "COLOR", Value: "BLUE"}, false},
			{"different enum type", &EnumeratedValue{TypeName: "COLOR", Value: "RED"}, &EnumeratedValue{TypeName: "SHADE", Value: "RED"}, false},
			{"enum vs string", &EnumeratedValue{TypeName: "COLOR", Value: "RED"}, &String{Value: "COLOR#RED"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if (tt.a.(Hashable).HashKey() == tt.b.(Hashable).HashKey()) != tt.wantEqual {
					t.Errorf("HashKey() equality for %s and %s was %v, want %v",
						tt.a.Inspect(), tt.b.Inspect(), !tt.wantEqual, tt.wantEqual)
				}
			})
		}
	})
}
