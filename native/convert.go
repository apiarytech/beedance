/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package native

// Conversions between beedance's values and the fields of a Go block. The
// Go side is read by kind and, for IEC types, by type name (royaljelly's
// iec.DWORD is a uint32 named DWORD), so beedance needs no IEC package.

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/apiarytech/beedance/object"
)

var durationType = reflect.TypeOf(time.Duration(0))

// isTime reports whether a Go type is a duration: time.Duration or an IEC
// TIME (an int64 named TIME).
func isTime(t reflect.Type) bool {
	return t == durationType || (t.Kind() == reflect.Int64 && strings.EqualFold(t.Name(), "TIME"))
}

// toGo stores a beedance value in a Go field.
func toGo(o object.Object, f reflect.Value) error {
	if c, ok := o.(*object.Constant); ok {
		o = c.Value
	}
	t := f.Type()
	switch {
	case isTime(t):
		tm, ok := o.(*object.Time)
		if !ok {
			return fmt.Errorf("want a TIME, got %s", o.Type())
		}
		f.SetInt(int64(tm.Value))
		return nil
	}
	switch t.Kind() {
	case reflect.Bool:
		b, ok := o.(*object.Boolean)
		if !ok {
			return fmt.Errorf("want a BOOL, got %s", o.Type())
		}
		f.SetBool(b.Value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := integer(o)
		if !ok {
			return fmt.Errorf("want an integer, got %s", o.Type())
		}
		f.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := integer(o)
		if !ok {
			return fmt.Errorf("want an integer, got %s", o.Type())
		}
		f.SetUint(uint64(n))
	case reflect.Float32, reflect.Float64:
		x, ok := object.GetFloat64Value(o)
		if !ok {
			return fmt.Errorf("want a number, got %s", o.Type())
		}
		f.SetFloat(x)
	case reflect.String:
		switch s := o.(type) {
		case *object.String:
			f.SetString(s.Value)
		case *object.WString:
			f.SetString(s.Value)
		default:
			return fmt.Errorf("want a STRING, got %s", o.Type())
		}
	case reflect.Struct:
		h, ok := o.(*object.Hash)
		if !ok {
			return fmt.Errorf("want a structure, got %s", o.Type())
		}
		for _, p := range h.Pairs {
			k, ok := p.Key.(*object.String)
			if !ok || strings.HasPrefix(k.Value, "__") {
				continue
			}
			sub := f.FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, k.Value) })
			if !sub.IsValid() || !sub.CanSet() {
				continue
			}
			if err := toGo(p.Value, sub); err != nil {
				return fmt.Errorf("%s: %w", k.Value, err)
			}
		}
	case reflect.Array, reflect.Slice:
		a, ok := o.(*object.Array)
		if !ok {
			return fmt.Errorf("want an array, got %s", o.Type())
		}
		if t.Kind() == reflect.Slice {
			f.Set(reflect.MakeSlice(t, len(a.Elements), len(a.Elements)))
		}
		for i := 0; i < len(a.Elements) && i < f.Len(); i++ {
			if err := toGo(a.Elements[i], f.Index(i)); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}
	default:
		return fmt.Errorf("cannot pass a %s", o.Type())
	}
	return nil
}

func integer(o object.Object) (int64, bool) {
	if b, ok := o.(*object.BitString); ok {
		return int64(b.Value), true
	}
	n, _, ok := object.GetIntegerObjectValue(o)
	return n, ok
}

// bitWidths are the IEC bit string types and their widths.
var bitWidths = map[string]int{"BYTE": 8, "WORD": 16, "DWORD": 32, "LWORD": 64}

// fromGo returns a Go field as a beedance value of its IEC type.
func fromGo(f reflect.Value) object.Object {
	t := f.Type()
	if isTime(t) {
		return &object.Time{Value: time.Duration(f.Int())}
	}
	name := strings.ToUpper(t.Name())
	if w, ok := bitWidths[name]; ok {
		return &object.BitString{Value: f.Uint(), Width: w}
	}
	switch t.Kind() {
	case reflect.Bool:
		if f.Bool() {
			return object.TRUE
		}
		return object.FALSE
	case reflect.Int8:
		return &object.SInt{Value: int8(f.Int())}
	case reflect.Int16:
		return &object.Int{Value: int16(f.Int())}
	case reflect.Int32:
		return &object.DInt{Value: int32(f.Int())}
	case reflect.Int, reflect.Int64:
		return &object.LInt{Value: f.Int()}
	case reflect.Uint8:
		return &object.USInt{Value: uint8(f.Uint())}
	case reflect.Uint16:
		return &object.UInt{Value: uint16(f.Uint())}
	case reflect.Uint32:
		return &object.UDInt{Value: uint32(f.Uint())}
	case reflect.Uint, reflect.Uint64:
		return &object.ULInt{Value: f.Uint()}
	case reflect.Float32:
		return &object.Real{Value: f.Float()}
	case reflect.Float64:
		return &object.LReal{Value: f.Float()}
	case reflect.String:
		return &object.String{Value: f.String()}
	case reflect.Struct:
		h := &object.Hash{Pairs: map[object.HashKey]object.HashPair{}}
		for i := range t.NumField() {
			if !t.Field(i).IsExported() {
				continue
			}
			k := &object.String{Value: t.Field(i).Name}
			h.Pairs[k.HashKey()] = object.HashPair{Key: k, Value: fromGo(f.Field(i))}
		}
		return h
	case reflect.Array, reflect.Slice:
		els := make([]object.Object, f.Len())
		for i := range els {
			els[i] = fromGo(f.Index(i))
		}
		return &object.Array{Elements: els}
	}
	return object.NewBuiltinError("cannot read a Go %s", t)
}
