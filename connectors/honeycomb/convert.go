/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package honeycomb connects beedance programs to a honeycomb tag database.
//
// This file converts values between beedance and royaljelly, whose iec types
// honeycomb's tags hold. A located variable has two types: the type it is
// declared with (`w AT %IW1 : INT`) and the type of the I/O image entry its
// address names (%IW1 is a WORD). Going from one to the other of the same
// width keeps the bits, as a PLC's memory does: WORD 16#FFFF is INT -1, and
// DWORD 16#3FC00000 is REAL 1.5. A value is converted to its declared type by
// value instead, checking the range, because the VM holds an INT variable's
// value as an LINT.
package honeycomb

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/royaljelly/iec"
)

// ToObject converts v, a royaljelly iec value such as iec.WORD(7) or a slice
// of them, to the beedance value of a variable declared typ, such as "INT"
// or "ARRAY[1..4] OF BOOL", keeping the bits when the widths match. An empty
// typ keeps v's own type.
func ToObject(v any, typ string) (object.Object, error) {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice {
		elem, lower, err := arrayType(typ)
		if err != nil {
			return nil, err
		}
		out := &object.Array{Elements: make([]object.Object, rv.Len()), LowerBound: lower}
		for i := range out.Elements {
			if out.Elements[i], err = ToObject(rv.Index(i).Interface(), elem); err != nil {
				return nil, fmt.Errorf("element %d: %w", i, err)
			}
		}
		return out, nil
	}
	s, err := iecScalar(v)
	if err != nil {
		return nil, err
	}
	if typ != "" {
		to, err := elementaryType(typ)
		if err != nil {
			return nil, err
		}
		if s, err = convert(s, to, true); err != nil {
			return nil, err
		}
	}
	return s.object(), nil
}

// FromObject converts obj, the value of a variable declared typ, to a value
// of the Go type of like, such as iec.WORD(0) or a tag's current value,
// keeping the bits when the widths match. obj is first converted to typ by
// value, so an INT the VM holds as an LINT must fit an INT. An empty typ
// takes obj as it is, and a nil like gives obj's own iec type.
func FromObject(obj object.Object, typ string, like any) (any, error) {
	if arr, ok := obj.(*object.Array); ok {
		elem := ""
		if typ != "" {
			var err error
			if elem, _, err = arrayType(typ); err != nil {
				return nil, err
			}
		}
		var likeElem any
		sliceType := reflect.TypeOf([]any{})
		if lv := reflect.ValueOf(like); lv.Kind() == reflect.Slice {
			sliceType = lv.Type()
			likeElem = reflect.Zero(lv.Type().Elem()).Interface()
		}
		out := reflect.MakeSlice(sliceType, len(arr.Elements), len(arr.Elements))
		for i, e := range arr.Elements {
			v, err := FromObject(e, elem, likeElem)
			if err != nil {
				return nil, fmt.Errorf("element %d: %w", i, err)
			}
			if likeElem == nil {
				out.Index(i).Set(reflect.ValueOf(&v).Elem())
			} else {
				out.Index(i).Set(reflect.ValueOf(v))
			}
		}
		return out.Interface(), nil
	}
	s, err := objectScalar(obj)
	if err != nil {
		return nil, err
	}
	if typ != "" {
		declared, err := elementaryType(typ)
		if err != nil {
			return nil, err
		}
		if s, err = convert(s, declared, false); err != nil {
			return nil, err
		}
	}
	if like != nil {
		likeScalar, err := iecScalar(like)
		if err != nil {
			return nil, err
		}
		if s, err = convert(s, likeScalar.t, true); err != nil {
			return nil, err
		}
	}
	return s.iec(), nil
}

// kind is how an elementary type's values are held.
type kind int

const (
	boolean  kind = iota // BOOL
	signed               // SINT, INT, DINT, LINT
	unsigned             // USINT, UINT, UDINT, ULINT
	bits                 // BYTE, WORD, DWORD, LWORD
	float                // REAL, LREAL
	text                 // STRING, WSTRING
	duration             // TIME
	instant              // DATE, TIME_OF_DAY, DATE_AND_TIME
)

// elementary is an elementary IEC 61131-3 type.
type elementary struct {
	name  string
	kind  kind
	width int // in bits, for BOOL, the integers, the bit strings and the reals
}

// numeric reports whether the type's values are held as bits: BOOL, the
// integers, the bit strings and the reals.
func (e elementary) numeric() bool { return e.kind <= float }

var elementaries = map[string]elementary{}

func init() {
	for _, e := range []elementary{
		{"BOOL", boolean, 1},
		{"SINT", signed, 8}, {"INT", signed, 16}, {"DINT", signed, 32}, {"LINT", signed, 64},
		{"USINT", unsigned, 8}, {"UINT", unsigned, 16}, {"UDINT", unsigned, 32}, {"ULINT", unsigned, 64},
		{"BYTE", bits, 8}, {"WORD", bits, 16}, {"DWORD", bits, 32}, {"LWORD", bits, 64},
		{"REAL", float, 32}, {"LREAL", float, 64},
		{"STRING", text, 0}, {"WSTRING", text, 0},
		{"TIME", duration, 0},
		{"DATE", instant, 0}, {"TIME_OF_DAY", instant, 0}, {"DATE_AND_TIME", instant, 0},
	} {
		elementaries[e.name] = e
	}
	elementaries["TOD"] = elementaries["TIME_OF_DAY"]
	elementaries["DT"] = elementaries["DATE_AND_TIME"]
}

// elementaryType returns the elementary type a declaration names, such as
// "INT", "int" or "STRING(20)".
func elementaryType(typ string) (elementary, error) {
	name := strings.ToUpper(strings.TrimSpace(typ))
	if i := strings.IndexAny(name, "(["); i > 0 && (strings.HasPrefix(name, "STRING") || strings.HasPrefix(name, "WSTRING")) {
		name = strings.TrimSpace(name[:i]) // a length: STRING(20), STRING[20]
	}
	e, ok := elementaries[name]
	if !ok {
		return elementary{}, fmt.Errorf("type %s is not an elementary type", typ)
	}
	return e, nil
}

// arrayDecl matches a one-dimensional array type, ARRAY[1..4] OF BOOL.
var arrayDecl = regexp.MustCompile(`(?i)^ARRAY\s*\[\s*(-?\d+)\s*\.\.\s*-?\d+\s*\]\s*OF\s+(.+)$`)

// arrayType returns the element type and the lower bound of a declared
// one-dimensional array type; an empty typ is an array of values as they are.
func arrayType(typ string) (elem string, lower int64, err error) {
	if typ == "" {
		return "", 0, nil
	}
	m := arrayDecl.FindStringSubmatch(strings.TrimSpace(typ))
	if m == nil {
		return "", 0, fmt.Errorf("type %s is not a one-dimensional array", typ)
	}
	lower, err = strconv.ParseInt(m[1], 10, 64)
	return strings.TrimSpace(m[2]), lower, err
}

// scalar is a value of an elementary type.
type scalar struct {
	t elementary
	// n holds the bits of a BOOL, an integer (two's complement), a bit
	// string or a real (IEEE 754).
	n  uint64
	s  string
	d  time.Duration
	tm time.Time
}

func boolScalar(b bool) scalar {
	if b {
		return scalar{t: elementaries["BOOL"], n: 1}
	}
	return scalar{t: elementaries["BOOL"]}
}

func signedScalar(name string, v int64) scalar {
	t := elementaries[name]
	return scalar{t: t, n: mask(uint64(v), t.width)}
}

func unsignedScalar(name string, v uint64) scalar { return scalar{t: elementaries[name], n: v} }

func realScalar(v float64) scalar {
	return scalar{t: elementaries["REAL"], n: uint64(math.Float32bits(float32(v)))}
}

func lrealScalar(v float64) scalar {
	return scalar{t: elementaries["LREAL"], n: math.Float64bits(v)}
}

// mask keeps the low width bits of n.
func mask(n uint64, width int) uint64 {
	if width >= 64 {
		return n
	}
	return n & (1<<width - 1)
}

// signedValue returns the value of a signed integer's bits.
func (s scalar) signedValue() int64 {
	shift := 64 - s.t.width
	return int64(s.n<<shift) >> shift
}

// floatValue returns the value of a real's bits.
func (s scalar) floatValue() float64 {
	if s.t.width == 32 {
		return float64(math.Float32frombits(uint32(s.n)))
	}
	return math.Float64frombits(s.n)
}

// convert returns s as a value of type to. With keepBits, a value of the
// same width keeps its bits (WORD 16#FFFF is INT -1); otherwise, and between
// widths, it keeps its value, which must fit.
func convert(s scalar, to elementary, keepBits bool) (scalar, error) {
	if s.t == to {
		return s, nil
	}
	switch {
	case s.t.numeric() && to.numeric():
		if keepBits && s.t.width == to.width {
			return scalar{t: to, n: s.n}, nil
		}
		return convertNumber(s, to)
	case s.t.kind == text && to.kind == text:
		return scalar{t: to, s: s.s}, nil
	}
	return scalar{}, fmt.Errorf("cannot convert %s to %s", s.t.name, to.name)
}

// convertNumber converts the value of a BOOL, an integer, a bit string or a
// real to another of those types, checking that it fits. A real becomes an
// integer rounded to the nearest, as REAL_TO_INT does.
func convertNumber(s scalar, to elementary) (scalar, error) {
	outOfRange := func() error {
		return fmt.Errorf("%s value %s is out of the range of %s", s.t.name, s.object().Inspect(), to.name)
	}
	if to.kind == float {
		var f float64
		switch s.t.kind {
		case float:
			f = s.floatValue()
		case signed:
			f = float64(s.signedValue())
		default:
			f = float64(s.n)
		}
		if to.width == 32 {
			if math.Abs(f) > math.MaxFloat32 && !math.IsInf(f, 0) {
				return scalar{}, outOfRange()
			}
			return realScalar(f), nil
		}
		return lrealScalar(f), nil
	}

	// An integer, a bit string or BOOL: the value as a sign and a magnitude.
	var negative bool
	var magnitude uint64
	switch s.t.kind {
	case float:
		f := math.Round(s.floatValue())
		if math.IsNaN(f) || f <= -(1<<63)-1 || f >= 1<<64 {
			return scalar{}, outOfRange()
		}
		if f < 0 {
			negative, magnitude = true, uint64(-f)
		} else {
			magnitude = uint64(f)
		}
	case signed:
		v := s.signedValue()
		if v < 0 {
			negative, magnitude = true, uint64(-v)
		} else {
			magnitude = uint64(v)
		}
	default:
		magnitude = s.n
	}

	switch to.kind {
	case signed:
		limit := uint64(1) << (to.width - 1) // -limit .. limit-1
		if (negative && magnitude > limit) || (!negative && magnitude >= limit) {
			return scalar{}, outOfRange()
		}
		if negative {
			return scalar{t: to, n: mask(-magnitude, to.width)}, nil
		}
		return scalar{t: to, n: magnitude}, nil
	default: // unsigned, bits, boolean
		if negative || (to.width < 64 && magnitude >= 1<<to.width) {
			return scalar{}, outOfRange()
		}
		return scalar{t: to, n: magnitude}, nil
	}
}

// objectScalar returns the elementary value a beedance object holds.
func objectScalar(obj object.Object) (scalar, error) {
	switch o := obj.(type) {
	case *object.Boolean:
		return boolScalar(o.Value), nil
	case *object.SInt:
		return signedScalar("SINT", int64(o.Value)), nil
	case *object.Int:
		return signedScalar("INT", int64(o.Value)), nil
	case *object.DInt:
		return signedScalar("DINT", int64(o.Value)), nil
	case *object.LInt:
		return signedScalar("LINT", o.Value), nil
	case *object.USInt:
		return unsignedScalar("USINT", uint64(o.Value)), nil
	case *object.UInt:
		return unsignedScalar("UINT", uint64(o.Value)), nil
	case *object.UDInt:
		return unsignedScalar("UDINT", uint64(o.Value)), nil
	case *object.ULInt:
		return unsignedScalar("ULINT", o.Value), nil
	case *object.Byte:
		return unsignedScalar("BYTE", uint64(o.Value)), nil
	case *object.Word:
		return unsignedScalar("WORD", uint64(o.Value)), nil
	case *object.DWord:
		return unsignedScalar("DWORD", uint64(o.Value)), nil
	case *object.LWord:
		return unsignedScalar("LWORD", o.Value), nil
	case *object.BitString:
		name := map[int]string{1: "BOOL", 8: "BYTE", 16: "WORD", 32: "DWORD", 64: "LWORD"}[o.Width]
		if name == "" {
			return scalar{}, fmt.Errorf("a bit string of %d bits has no IEC type", o.Width)
		}
		return unsignedScalar(name, mask(o.Value, o.Width)), nil
	case *object.Real:
		return realScalar(o.Value), nil
	case *object.LReal:
		return lrealScalar(o.Value), nil
	case *object.String:
		return scalar{t: elementaries["STRING"], s: o.Value}, nil
	case *object.WString:
		return scalar{t: elementaries["WSTRING"], s: o.Value}, nil
	case *object.Time:
		return scalar{t: elementaries["TIME"], d: o.Value}, nil
	case *object.Date:
		return scalar{t: elementaries["DATE"], tm: o.Value}, nil
	case *object.TimeOfDay:
		return scalar{t: elementaries["TIME_OF_DAY"], tm: o.Value}, nil
	case *object.DateAndTime:
		return scalar{t: elementaries["DATE_AND_TIME"], tm: o.Value}, nil
	case nil:
		return scalar{}, fmt.Errorf("no value")
	}
	return scalar{}, fmt.Errorf("a %s value has no elementary IEC type", obj.Type())
}

// object returns the beedance object holding s.
func (s scalar) object() object.Object {
	switch s.t.name {
	case "BOOL":
		return &object.Boolean{Value: s.n != 0}
	case "SINT":
		return &object.SInt{Value: int8(s.signedValue())}
	case "INT":
		return &object.Int{Value: int16(s.signedValue())}
	case "DINT":
		return &object.DInt{Value: int32(s.signedValue())}
	case "LINT":
		return &object.LInt{Value: s.signedValue()}
	case "USINT":
		return &object.USInt{Value: uint8(s.n)}
	case "UINT":
		return &object.UInt{Value: uint16(s.n)}
	case "UDINT":
		return &object.UDInt{Value: uint32(s.n)}
	case "ULINT":
		return &object.ULInt{Value: s.n}
	case "BYTE":
		return &object.Byte{Value: uint8(s.n)}
	case "WORD":
		return &object.Word{Value: uint16(s.n)}
	case "DWORD":
		return &object.DWord{Value: uint32(s.n)}
	case "LWORD":
		return &object.LWord{Value: s.n}
	case "REAL":
		return &object.Real{Value: s.floatValue()}
	case "LREAL":
		return &object.LReal{Value: s.floatValue()}
	case "STRING":
		return &object.String{Value: s.s}
	case "WSTRING":
		return &object.WString{Value: s.s}
	case "TIME":
		return &object.Time{Value: s.d}
	case "DATE":
		return &object.Date{Value: s.tm}
	case "TIME_OF_DAY":
		return &object.TimeOfDay{Value: s.tm}
	default: // DATE_AND_TIME
		return &object.DateAndTime{Value: s.tm}
	}
}

// iecScalar returns the elementary value a royaljelly iec value holds.
func iecScalar(v any) (scalar, error) {
	switch x := v.(type) {
	case iec.BOOL:
		return boolScalar(bool(x)), nil
	case iec.SINT:
		return signedScalar("SINT", int64(x)), nil
	case iec.INT:
		return signedScalar("INT", int64(x)), nil
	case iec.DINT:
		return signedScalar("DINT", int64(x)), nil
	case iec.LINT:
		return signedScalar("LINT", int64(x)), nil
	case iec.USINT:
		return unsignedScalar("USINT", uint64(x)), nil
	case iec.UINT:
		return unsignedScalar("UINT", uint64(x)), nil
	case iec.UDINT:
		return unsignedScalar("UDINT", uint64(x)), nil
	case iec.ULINT:
		return unsignedScalar("ULINT", uint64(x)), nil
	case iec.BYTE:
		return unsignedScalar("BYTE", uint64(x)), nil
	case iec.WORD:
		return unsignedScalar("WORD", uint64(x)), nil
	case iec.DWORD:
		return unsignedScalar("DWORD", uint64(x)), nil
	case iec.LWORD:
		return unsignedScalar("LWORD", uint64(x)), nil
	case iec.REAL:
		return realScalar(float64(x)), nil
	case iec.LREAL:
		return lrealScalar(float64(x)), nil
	case iec.STRING:
		return scalar{t: elementaries["STRING"], s: string(x)}, nil
	case iec.WSTRING:
		return scalar{t: elementaries["WSTRING"], s: string(x)}, nil
	case iec.TIME:
		return scalar{t: elementaries["TIME"], d: time.Duration(x)}, nil
	case iec.DATE:
		return scalar{t: elementaries["DATE"], tm: time.Time(x)}, nil
	case iec.TIME_OF_DAY:
		return scalar{t: elementaries["TIME_OF_DAY"], tm: time.Time(x)}, nil
	case iec.DT:
		return scalar{t: elementaries["DATE_AND_TIME"], tm: time.Time(x)}, nil
	}
	return scalar{}, fmt.Errorf("%T is not an elementary iec type", v)
}

// iec returns the royaljelly iec value holding s.
func (s scalar) iec() any {
	switch s.t.name {
	case "BOOL":
		return iec.BOOL(s.n != 0)
	case "SINT":
		return iec.SINT(s.signedValue())
	case "INT":
		return iec.INT(s.signedValue())
	case "DINT":
		return iec.DINT(s.signedValue())
	case "LINT":
		return iec.LINT(s.signedValue())
	case "USINT":
		return iec.USINT(s.n)
	case "UINT":
		return iec.UINT(s.n)
	case "UDINT":
		return iec.UDINT(s.n)
	case "ULINT":
		return iec.ULINT(s.n)
	case "BYTE":
		return iec.BYTE(s.n)
	case "WORD":
		return iec.WORD(s.n)
	case "DWORD":
		return iec.DWORD(s.n)
	case "LWORD":
		return iec.LWORD(s.n)
	case "REAL":
		return iec.REAL(s.floatValue())
	case "LREAL":
		return iec.LREAL(s.floatValue())
	case "STRING":
		return iec.STRING(s.s)
	case "WSTRING":
		return iec.WSTRING(s.s)
	case "TIME":
		return iec.TIME(s.d)
	case "DATE":
		return iec.DATE(s.tm)
	case "TIME_OF_DAY":
		return iec.TIME_OF_DAY(s.tm)
	default: // DATE_AND_TIME
		return iec.DT(s.tm)
	}
}
