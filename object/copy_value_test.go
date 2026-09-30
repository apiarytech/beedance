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

import "testing"

func TestCopyValue(t *testing.T) {
	x := &String{Value: "x"}
	inner := &Array{Elements: []Object{&LInt{Value: 1}}, LowerBound: 1}
	point := &Hash{Pairs: map[HashKey]HashPair{x.HashKey(): {Key: x, Value: inner}}}
	outer := &Array{Elements: []Object{point}, LowerBound: -1}

	copied := CopyValue(outer).(*Array)
	if copied == outer || copied.LowerBound != -1 {
		t.Fatalf("array not copied with its bound: %+v", copied)
	}
	copiedPoint := copied.Elements[0].(*Hash)
	if copiedPoint == point {
		t.Fatal("structure inside the array was not copied")
	}
	copiedInner := copiedPoint.Pairs[x.HashKey()].Value.(*Array)
	if copiedInner == inner || copiedInner.LowerBound != 1 {
		t.Fatalf("array inside the structure not copied with its bound: %+v", copiedInner)
	}
	copiedInner.Elements[0] = &LInt{Value: 9}
	if inner.Elements[0].(*LInt).Value != 1 {
		t.Error("changing the copy changed the original")
	}

	// A function block instance and other values are not copied.
	class := &String{Value: "__class__"}
	instance := &Hash{Pairs: map[HashKey]HashPair{class.HashKey(): {Key: class, Value: &Hash{}}}}
	if CopyValue(instance) != Object(instance) {
		t.Error("a function block instance was copied")
	}
	number := &LInt{Value: 5}
	if CopyValue(number) != Object(number) {
		t.Error("a number was copied")
	}
}
