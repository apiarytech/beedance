package object

import (
	"fmt"
	"testing"
)

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		obj      Object
		expected bool
	}{
		{&SInt{}, true},
		{&Int{}, true},
		{&DInt{}, true},
		{&LInt{}, true},
		{&USInt{}, true},
		{&UInt{}, true},
		{&UDInt{}, true},
		{&ULInt{}, true},
		{&Real{}, true},
		{&LReal{}, true},
		{&Boolean{}, false},
		{&String{}, false},
		{&Null{}, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("IsNumeric(%T)", tt.obj), func(t *testing.T) {
			if got := IsNumeric(tt.obj); got != tt.expected {
				t.Errorf("IsNumeric() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetIntegerObjectValue(t *testing.T) {
	tests := []struct {
		name       string
		obj        Object
		wantVal    int64
		wantUnsign bool
		wantSucc   bool
	}{
		{"SInt", &SInt{Value: -8}, -8, false, true},
		{"Int", &Int{Value: -16}, -16, false, true},
		{"DInt", &DInt{Value: -32}, -32, false, true},
		{"LInt", &LInt{Value: -64}, -64, false, true},
		{"USInt", &USInt{Value: 8}, 8, true, true},
		{"UInt", &UInt{Value: 16}, 16, true, true},
		{"UDInt", &UDInt{Value: 32}, 32, true, true},
		{"ULInt", &ULInt{Value: 64}, 64, true, true},
		{"Real", &Real{Value: 1.23}, 0, false, false},
		{"String", &String{Value: "123"}, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotUnsign, gotSucc := GetIntegerObjectValue(tt.obj)
			if gotVal != tt.wantVal {
				t.Errorf("GetIntegerObjectValue() gotVal = %v, want %v", gotVal, tt.wantVal)
			}
			if gotUnsign != tt.wantUnsign {
				t.Errorf("GetIntegerObjectValue() gotUnsign = %v, want %v", gotUnsign, tt.wantUnsign)
			}
			if gotSucc != tt.wantSucc {
				t.Errorf("GetIntegerObjectValue() gotSucc = %v, want %v", gotSucc, tt.wantSucc)
			}
		})
	}
}

func TestGetFloat64Value(t *testing.T) {
	tests := []struct {
		name     string
		obj      Object
		wantVal  float64
		wantSucc bool
	}{
		{"SInt", &SInt{Value: -8}, -8.0, true},
		{"Int", &Int{Value: 1600}, 1600.0, true},
		{"LInt", &LInt{Value: -123456}, -123456.0, true},
		{"USInt", &USInt{Value: 255}, 255.0, true},
		{"ULInt", &ULInt{Value: 12345}, 12345.0, true},
		{"Real", &Real{Value: 3.14}, 3.14, true},
		{"LReal", &LReal{Value: -1.23e4}, -12300.0, true},
		{"Boolean", &Boolean{Value: true}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotSucc := GetFloat64Value(tt.obj)
			if gotVal != tt.wantVal {
				t.Errorf("GetFloat64Value() gotVal = %v, want %v", gotVal, tt.wantVal)
			}
			if gotSucc != tt.wantSucc {
				t.Errorf("GetFloat64Value() gotSucc = %v, want %v", gotSucc, tt.wantSucc)
			}
		})
	}
}

func TestIsTypeFunctions(t *testing.T) {
	// IsIntegerType
	if !IsIntegerType("DINT") {
		t.Error("IsIntegerType('DINT') should be true")
	}
	if IsIntegerType("REAL") {
		t.Error("IsIntegerType('REAL') should be false")
	}

	// IsBooleanType
	if !IsBooleanType("BOOL") {
		t.Error("IsBooleanType('BOOL') should be true")
	}
	if IsBooleanType("INT") {
		t.Error("IsBooleanType('INT') should be false")
	}

	// IsRealType
	if !IsRealType("LREAL") {
		t.Error("IsRealType('LREAL') should be true")
	}
	if IsRealType("STRING") {
		t.Error("IsRealType('STRING') should be false")
	}

	// IsStringType
	if !IsStringType("STRING") {
		t.Error("IsStringType('STRING') should be true")
	}
	if !IsStringType("WSTRING") {
		t.Error("IsStringType('WSTRING') should be true")
	}
	if IsStringType("BYTE") {
		t.Error("IsStringType('BYTE') should be false")
	}

	// IsBitStringType
	if !IsBitStringType("WORD") {
		t.Error("IsBitStringType('WORD') should be true")
	}
	if IsBitStringType("INT") {
		t.Error("IsBitStringType('INT') should be false")
	}

	// IsTimeDateKeyword
	if !IsTimeDateKeyword("DATE_AND_TIME") {
		t.Error("IsTimeDateKeyword('DATE_AND_TIME') should be true")
	}
	if !IsTimeDateKeyword("T") {
		t.Error("IsTimeDateKeyword('T') should be true")
	}
	if IsTimeDateKeyword("TIME_AND_DATE") {
		t.Error("IsTimeDateKeyword('TIME_AND_DATE') should be false")
	}
}

func TestGetBitStringWidth(t *testing.T) {
	tests := []struct {
		name     string
		wantW    int
		wantSucc bool
	}{
		{"BYTE", 8, true},
		{"WORD", 16, true},
		{"DWORD", 32, true},
		{"LWORD", 64, true},
		{"INT", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotSucc := GetBitStringWidth(tt.name)
			if gotW != tt.wantW {
				t.Errorf("GetBitStringWidth() gotW = %v, want %v", gotW, tt.wantW)
			}
			if gotSucc != tt.wantSucc {
				t.Errorf("GetBitStringWidth() gotSucc = %v, want %v", gotSucc, tt.wantSucc)
			}
		})
	}
}

func TestNewBuiltinError(t *testing.T) {
	err := NewBuiltinError("test error %d", 123)
	expected := "BUILTIN ERROR: test error 123"
	if err.Message != expected {
		t.Errorf("NewBuiltinError() message wrong. want=%q, got=%q", expected, err.Message)
	}
}

func TestNativeBoolToBooleanObject(t *testing.T) {
	if nativeBoolToBooleanObject(true) != TRUE {
		t.Error("nativeBoolToBooleanObject(true) should return TRUE singleton")
	}
	if nativeBoolToBooleanObject(false) != FALSE {
		t.Error("nativeBoolToBooleanObject(false) should return FALSE singleton")
	}
}

func TestApplyConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    Object
		fromType string
		toType   string
		expected Object
	}{
		{"INT_TO_DINT", &Int{Value: 100}, "INT", "DINT", &DInt{Value: 100}},
		{"REAL_TO_INT", &Real{Value: 12.6}, "REAL", "INT", &Int{Value: 13}},
		{"STRING_TO_INT", &String{Value: "-42"}, "STRING", "INT", &LInt{Value: -42}},
		{"BOOL_TO_STRING", TRUE, "BOOL", "STRING", &String{Value: "TRUE"}},
		{"INT_TO_SINT_ok", &Int{Value: 127}, "INT", "SINT", &SInt{Value: 127}},
		{"INT_TO_SINT_fail", &Int{Value: 128}, "INT", "SINT", NewBuiltinError("value 128 is out of range for type SINT (-128 to 127)")},
		{"INT_TO_USINT_fail", &Int{Value: -1}, "INT", "USINT", NewBuiltinError("value -1 is out of range for type USINT (0 to 255)")},
		{"INT_TO_BYTE", &Int{Value: 255}, "INT", "BYTE", &BitString{Value: 255, Width: 8}},
		{"INT_TO_BCD", &Int{Value: 123}, "INT", "BCD", &BitString{Value: 0x123, Width: 16}},
		{"BCD_TO_INT", &BitString{Value: 0x123, Width: 16}, "BCD", "INT", &LInt{Value: 123}},
		{"BCD_TO_INT_fail", &BitString{Value: 0x1A3, Width: 16}, "BCD", "INT", NewBuiltinError("invalid BCD format: nibble 1 has value 10 > 9")},
		{"ANY_REAL_TO_INT", &Int{Value: 12}, "ANY_REAL", "INT", &Int{Value: 12}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyConversion(tt.input, tt.fromType, tt.toType)
			if got.Type() != tt.expected.Type() {
				t.Fatalf("ApplyConversion() type wrong. got=%s, want=%s", got.Type(), tt.expected.Type())
			}
			// For error comparison, just check the message
			if err, ok := got.(*Error); ok {
				expErr := tt.expected.(*Error)
				if err.Message != expErr.Message {
					t.Errorf("ApplyConversion() error message wrong. got=%q, want=%q", err.Message, expErr.Message)
				}
				return
			}
			if got.Inspect() != tt.expected.Inspect() {
				t.Errorf("ApplyConversion() inspect wrong. got=%q, want=%q", got.Inspect(), tt.expected.Inspect())
			}
		})
	}
}

func TestBcdConversions(t *testing.T) {
	t.Run("intToBcd", func(t *testing.T) {
		tests := []struct {
			in   int64
			want uint16
			fail bool
		}{
			{1234, 0x1234, false},
			{9999, 0x9999, false},
			{0, 0x0000, false},
			{8, 0x0008, false},
			{10000, 0, true},
			{-1, 0, true},
		}
		for _, tt := range tests {
			got, err := intToBcd(tt.in)
			if (err != nil) != tt.fail {
				t.Errorf("intToBcd(%d) error status wrong. got err=%v, want fail=%v", tt.in, err, tt.fail)
			}
			if !tt.fail && got != tt.want {
				t.Errorf("intToBcd(%d) = 0x%X, want 0x%X", tt.in, got, tt.want)
			}
		}
	})

	t.Run("bcdToInt", func(t *testing.T) {
		tests := []struct {
			in   Object
			want int64
			fail bool
		}{
			{&BitString{Value: 0x1234, Width: 16}, 1234, false},
			{&BitString{Value: 0x9999, Width: 16}, 9999, false},
			{&BitString{Value: 0x0008, Width: 16}, 8, false},
			{&BitString{Value: 0x1A2B, Width: 16}, 0, true}, // Invalid BCD
			{&BitString{Value: 0x1234, Width: 8}, 0, true},  // Wrong width
			{&Int{Value: 1234}, 0, true},                    // Wrong type
		}
		for _, tt := range tests {
			gotObj := bcdToInt(tt.in)
			if err, ok := gotObj.(*Error); ok {
				if !tt.fail {
					t.Errorf("bcdToInt(%v) failed unexpectedly: %s", tt.in, err.Message)
				}
			} else {
				if tt.fail {
					t.Errorf("bcdToInt(%v) should have failed but didn't", tt.in)
				}
				if got := gotObj.(*LInt).Value; got != tt.want {
					t.Errorf("bcdToInt(%v) = %d, want %d", tt.in, got, tt.want)
				}
			}
		}
	})
}

func TestIsEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b Object
		want bool
	}{
		{"equal ints", &LInt{Value: 5}, &Int{Value: 5}, true},
		{"unequal ints", &LInt{Value: 5}, &LInt{Value: 6}, false},
		{"int and real", &LInt{Value: 5}, &Real{Value: 5.0}, true},
		{"int and real unequal", &LInt{Value: 5}, &Real{Value: 5.1}, false},
		{"equal strings", &String{Value: "hi"}, &String{Value: "hi"}, true},
		{"unequal strings", &String{Value: "hi"}, &String{Value: "ho"}, false},
		{"string and int", &String{Value: "5"}, &LInt{Value: 5}, false},
		{"equal bools", TRUE, &Boolean{Value: true}, true},
		{"unequal bools", TRUE, FALSE, false},
		{"nulls", &Null{}, &Null{}, true},
		{"null and int", &Null{}, &LInt{Value: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("IsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
