/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import (
	"strings"
	"testing"
)

func TestReferenceTargets(t *testing.T) {
	env := NewEnvironment()
	env.Set("x", &Int{Value: 1})
	inner := NewEnclosedEnvironment(env)
	scope, name, ok := inner.Owner("X")
	if !ok || scope != env || name != "x" {
		t.Fatalf("Owner(X) = %v %q %v", scope == env, name, ok)
	}

	var slot Object = &Int{Value: 2}
	io := map[string]Object{"%IW1": &Int{Value: 3}}
	arr := &Array{Elements: []Object{&Int{Value: 4}, &Int{Value: 5}}, LowerBound: 1}
	hash := &Hash{Pairs: map[HashKey]HashPair{}}
	key := &String{Value: "m"}
	hash.Pairs[key.HashKey()] = HashPair{Key: key, Value: &Int{Value: 6}}
	elem, err := NewElementReference(arr, &Int{Value: 2})
	if err != nil {
		t.Fatal(err)
	}
	member, err := NewElementReference(hash, key)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range []*Reference{
		{Env: env, Name: "x"}, {Slot: &slot}, {IO: io, Name: "%IW1"}, elem, member,
	} {
		if err := r.Set(&Int{Value: 9}); err != nil {
			t.Fatalf("%s: %v", r.Inspect(), err)
		}
		if v, err := r.Get(); err != nil || v.Inspect() != "9" {
			t.Errorf("%s: got %v %v", r.Inspect(), v, err)
		}
	}
	if arr.Elements[1].Inspect() != "9" {
		t.Errorf("the array element was not written: %s", arr.Inspect())
	}
}

func TestReferenceErrors(t *testing.T) {
	null := &Reference{}
	if _, err := null.Get(); err == nil || null.Inspect() != "NULL" {
		t.Errorf("NULL: %v %s", err, null.Inspect())
	}
	if err := null.Set(&Int{}); err == nil {
		t.Error("writing through NULL")
	}
	env := NewEnvironment()
	env.Set("k", &Constant{Value: &Int{Value: 1}})
	if err := (&Reference{Env: env, Name: "k"}).Set(&Int{}); err == nil || !strings.Contains(err.Error(), "constant") {
		t.Errorf("writing a constant: %v", err)
	}
	if v, err := (&Reference{Env: env, Name: "k"}).Get(); err != nil || v.Inspect() != "1" {
		t.Errorf("reading a constant: %v %v", v, err)
	}
	if _, err := (&Reference{Env: env, Name: "gone"}).Get(); err == nil {
		t.Error("reading a variable that does not exist")
	}
	if _, err := NewElementReference(&Array{Elements: []Object{&Int{}}}, &Int{Value: 3}); err == nil {
		t.Error("an element out of bounds")
	}
	if _, err := NewElementReference(&Array{}, &String{Value: "x"}); err == nil {
		t.Error("an array index that is not an integer")
	}
	if _, err := NewElementReference(&Int{}, &Int{}); err == nil {
		t.Error("an element of an INT")
	}
	if _, err := NewElementReference(&Hash{Pairs: map[HashKey]HashPair{}}, &Array{}); err == nil {
		t.Error("a member named by an array")
	}
	missing, _ := NewElementReference(&Hash{Pairs: map[HashKey]HashPair{}}, &String{Value: "m"})
	if _, err := missing.Get(); err == nil {
		t.Error("a member that does not exist")
	}
}

func TestCompareReferences(t *testing.T) {
	env := NewEnvironment()
	x, y := &Reference{Env: env, Name: "x"}, &Reference{Env: env, Name: "y"}
	var slot Object
	tests := []struct {
		left     Object
		op       string
		right    Object
		want, ok bool
		err      string
	}{
		{x, "=", &Reference{Env: env, Name: "X"}, true, true, ""},
		{x, "<>", y, true, true, ""},
		{x, "=", &Null{}, false, true, ""},
		{&Reference{}, "=", &Null{}, true, true, ""},
		{&Null{}, "NE", x, true, true, ""},
		{&Reference{Slot: &slot}, "EQ", &Reference{Slot: &slot}, true, true, ""},
		{&Reference{IO: map[string]Object{}, Name: "%IX0.0"}, "=", &Reference{IO: map[string]Object{}, Name: "%IX0.0"}, true, true, ""},
		{&Int{}, "=", &Int{}, false, false, ""},
		{x, "=", &Int{}, false, true, "compared only with a reference or NULL"},
		{x, "<", y, false, true, "no arithmetic"},
	}
	for _, tt := range tests {
		got, ok, err := CompareReferences(tt.left, tt.op, tt.right)
		if ok != tt.ok || (tt.err == "" && (err != nil || got != tt.want)) || (tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err))) {
			t.Errorf("%s %s %s: got %v %v %v", tt.left.Inspect(), tt.op, tt.right.Inspect(), got, ok, err)
		}
	}
}

func TestArrayViewAndRetarget(t *testing.T) {
	a := &Array{Elements: []Object{&Int{Value: 1}}, LowerBound: 1}
	view := ArrayView(a, []int64{0}).(*Array)
	if view.LowerBound != 0 || &view.Elements[0] != &a.Elements[0] {
		t.Error("a view shares the elements, indexed from its own bound")
	}
	if ArrayView(a, []int64{1}) != a || ArrayView(a, nil) != a || ArrayView(&Int{}, []int64{0}).Inspect() != "0" {
		t.Error("a view that changes nothing is the array itself")
	}
	typed := &Reference{Lower: []int64{0}}
	got := typed.Retarget(&Reference{Env: NewEnvironment(), Name: "a"})
	if got.Name != "a" || len(got.Lower) != 1 || !typed.Retarget(&Null{}).IsNull() {
		t.Errorf("Retarget: %+v", got)
	}
	if (&Reference{Slot: new(Object)}).Inspect() != "REF(variable)" || (&Reference{Container: a, Index: &Int{Value: 1}}).Inspect() != "REF(ARRAY[1])" {
		t.Error("Inspect")
	}
	if (&Reference{}).Type() != REFERENCE_OBJ || ReferenceSize != 4 {
		t.Error("type and size")
	}
}
