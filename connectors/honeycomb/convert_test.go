/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package honeycomb

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/royaljelly/iec"
)

// An input read from a tag becomes the value of the variable's declared
// type, keeping the bits when the widths match.
func TestToObject(t *testing.T) {
	day := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		v    any
		typ  string
		want object.Object
	}{
		{iec.WORD(7), "INT", &object.Int{Value: 7}},
		{iec.WORD(0xFFFF), "INT", &object.Int{Value: -1}},
		{iec.WORD(0xFFFF), "UINT", &object.UInt{Value: 0xFFFF}},
		{iec.WORD(7), "", &object.Word{Value: 7}},
		{iec.WORD(7), "dint", &object.DInt{Value: 7}},
		{iec.BYTE(0x80), "SINT", &object.SInt{Value: -128}},
		{iec.DWORD(0x3FC00000), "REAL", &object.Real{Value: 1.5}},
		{iec.LWORD(0x3FF8000000000000), "LREAL", &object.LReal{Value: 1.5}},
		{iec.BOOL(true), "BOOL", &object.Boolean{Value: true}},
		{iec.INT(-3), "LINT", &object.LInt{Value: -3}},
		{iec.REAL(2.5), "LREAL", &object.LReal{Value: 2.5}},
		{iec.STRING("hi"), "STRING(20)", &object.String{Value: "hi"}},
		{iec.STRING("hi"), "WSTRING", &object.WString{Value: "hi"}},
		{iec.TIME(2 * time.Second), "TIME", &object.Time{Value: 2 * time.Second}},
		{iec.DATE(day), "DATE", &object.Date{Value: day}},
		{iec.DT(day), "DT", &object.DateAndTime{Value: day}},
		{iec.TOD(day), "TOD", &object.TimeOfDay{Value: day}},
		{[]iec.BOOL{true, false}, "ARRAY[1..2] OF BOOL", &object.Array{
			Elements: []object.Object{&object.Boolean{Value: true}, &object.Boolean{Value: false}}, LowerBound: 1}},
	}
	for _, tt := range tests {
		got, err := ToObject(tt.v, tt.typ)
		if err != nil {
			t.Errorf("ToObject(%T(%v), %q): %v", tt.v, tt.v, tt.typ, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ToObject(%T(%v), %q) = %#v, want %#v", tt.v, tt.v, tt.typ, got, tt.want)
		}
	}
}

// An output becomes a value of its tag's type: first its declared type by
// value (the VM holds an INT as an LINT), then the tag's by its bits.
func TestFromObject(t *testing.T) {
	tests := []struct {
		obj  object.Object
		typ  string
		like any
		want any
	}{
		{&object.LInt{Value: 8}, "INT", iec.WORD(0), iec.WORD(8)},
		{&object.LInt{Value: -1}, "INT", iec.WORD(0), iec.WORD(0xFFFF)},
		{&object.LInt{Value: 5}, "BYTE", iec.BYTE(0), iec.BYTE(5)},
		{&object.LReal{Value: 1.5}, "REAL", iec.DWORD(0), iec.DWORD(0x3FC00000)},
		{&object.LReal{Value: 1.5}, "REAL", nil, iec.REAL(1.5)},
		{&object.LInt{Value: 8}, "", nil, iec.LINT(8)},
		{&object.LInt{Value: 8}, "", iec.DINT(0), iec.DINT(8)},
		{&object.Boolean{Value: true}, "BOOL", iec.BOOL(false), iec.BOOL(true)},
		{&object.BitString{Value: 0x1FF, Width: 8}, "", nil, iec.BYTE(0xFF)},
		{&object.LReal{Value: 2.6}, "DINT", nil, iec.DINT(3)},
		{&object.WString{Value: "hé"}, "WSTRING", iec.WSTRING(""), iec.WSTRING("hé")},
		{&object.Time{Value: time.Second}, "TIME", iec.TIME(0), iec.TIME(time.Second)},
		{&object.Array{Elements: []object.Object{&object.LInt{Value: 1}, &object.LInt{Value: 2}}},
			"ARRAY[0..1] OF INT", []iec.INT{}, []iec.INT{1, 2}},
		{&object.Array{Elements: []object.Object{&object.LInt{Value: 1}}}, "", nil, []any{iec.LINT(1)}},
	}
	for _, tt := range tests {
		got, err := FromObject(tt.obj, tt.typ, tt.like)
		if err != nil {
			t.Errorf("FromObject(%s, %q, %T): %v", tt.obj.Inspect(), tt.typ, tt.like, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("FromObject(%s, %q, %T) = %T(%v), want %T(%v)", tt.obj.Inspect(), tt.typ, tt.like, got, got, tt.want, tt.want)
		}
	}
}

// Values that do not fit their type, and types that do not convert, are
// errors rather than wrapped or truncated values.
func TestConversionErrors(t *testing.T) {
	fromObject := []struct {
		obj  object.Object
		typ  string
		like any
		want string
	}{
		{&object.LInt{Value: 40000}, "INT", iec.WORD(0), "out of the range of INT"},
		{&object.LInt{Value: -1}, "UINT", nil, "out of the range of UINT"},
		{&object.LInt{Value: 256}, "BYTE", nil, "out of the range of BYTE"},
		{&object.LInt{Value: 2}, "BOOL", nil, "out of the range of BOOL"},
		{&object.LInt{Value: 70000}, "", iec.WORD(0), "out of the range of WORD"},
		{&object.LReal{Value: 1e40}, "REAL", nil, "out of the range of REAL"},
		{&object.String{Value: "x"}, "INT", nil, "cannot convert STRING to INT"},
		{&object.Time{Value: 1}, "DATE", nil, "cannot convert TIME to DATE"},
		{&object.LInt{Value: 1}, "Motor", nil, "not an elementary type"},
		{&object.Null{}, "INT", nil, "no elementary IEC type"},
		{&object.Array{}, "INT", nil, "not a one-dimensional array"},
		{&object.LInt{Value: 1}, "INT", iec.CHAR('a'), "not an elementary iec type"},
	}
	for _, tt := range fromObject {
		_, err := FromObject(tt.obj, tt.typ, tt.like)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("FromObject(%s, %q, %T): %v, want an error containing %q", tt.obj.Inspect(), tt.typ, tt.like, err, tt.want)
		}
	}

	toObject := []struct {
		v    any
		typ  string
		want string
	}{
		{iec.WORD(0x8000), "SINT", "out of the range of SINT"},
		{iec.STRING("x"), "TIME", "cannot convert STRING to TIME"},
		{iec.WCHAR('a'), "", "not an elementary iec type"},
		{[]iec.BOOL{true}, "BOOL", "not a one-dimensional array"},
		{[]iec.INT{700}, "ARRAY[0..0] OF SINT", "element 0: INT value 700"},
	}
	for _, tt := range toObject {
		_, err := ToObject(tt.v, tt.typ)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ToObject(%T(%v), %q): %v, want an error containing %q", tt.v, tt.v, tt.typ, err, tt.want)
		}
	}
}

// Every elementary type survives a round trip through beedance.
func TestRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 3, 8, 30, 0, 0, time.UTC)
	for _, v := range []any{
		iec.BOOL(true), iec.SINT(-8), iec.INT(-16), iec.DINT(-32), iec.LINT(-64),
		iec.USINT(8), iec.UINT(16), iec.UDINT(32), iec.ULINT(1 << 63),
		iec.BYTE(0xAB), iec.WORD(0xABCD), iec.DWORD(0xDEADBEEF), iec.LWORD(1<<64 - 1),
		iec.REAL(-0.25), iec.LREAL(3.141592653589793),
		iec.STRING("abc"), iec.WSTRING("ωx"), iec.TIME(1500 * time.Millisecond),
		iec.DATE(now), iec.TOD(now), iec.DT(now),
	} {
		obj, err := ToObject(v, "")
		if err != nil {
			t.Errorf("ToObject(%T): %v", v, err)
			continue
		}
		back, err := FromObject(obj, "", nil)
		if err != nil {
			t.Errorf("FromObject(%s): %v", obj.Inspect(), err)
			continue
		}
		if !reflect.DeepEqual(back, v) {
			t.Errorf("%T(%v) came back as %T(%v)", v, v, back, back)
		}
	}
}
