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

func TestBits(t *testing.T) {
	values := []Object{&SInt{Value: -1}, &Int{Value: -1}, &DInt{Value: -1}, &LInt{Value: -1}, &USInt{Value: 255}, &UInt{Value: 65535},
		&UDInt{Value: 1<<32 - 1}, &ULInt{Value: 1<<64 - 1}, &Byte{Value: 255}, &Word{Value: 65535}, &DWord{Value: 1<<32 - 1},
		&LWord{Value: 1<<64 - 1}, &BitString{Value: 255, Width: 8}}
	for _, v := range values {
		on, err := GetBit(v, 0)
		if err != nil || !on {
			t.Errorf("%T bit 0: %v %v", v, on, err)
		}
		off, err := SetBit(v, 0, false)
		if err != nil || off.Type() != v.Type() {
			t.Fatalf("%T: %v %v", v, off, err)
		}
		if bit, _ := GetBit(off, 0); bit {
			t.Errorf("%T: bit 0 still set", v)
		}
		if _, err := GetBit(v, 64); err == nil {
			t.Errorf("%T: bit 64 accepted", v)
		}
		if _, err := SetBit(v, -1, true); err == nil {
			t.Errorf("%T: bit -1 accepted", v)
		}
	}
	for _, bad := range []Object{&Real{Value: 1}, &String{Value: "x"}} {
		if _, err := GetBit(bad, 0); err == nil {
			t.Errorf("%T: bit access accepted", bad)
		}
		if _, err := SetBit(bad, 0, true); err == nil {
			t.Errorf("%T: bit access accepted", bad)
		}
	}
	// A value held wider than its declared type keeps the type's width and sign.
	if v, _ := SetBitAs(&LInt{Value: -1}, 15, false, "INT"); v.(*LInt).Value != 32767 {
		t.Errorf("INT -1 without bit 15 = %v", v)
	}
	if v, _ := SetBitAs(&LInt{Value: 5}, 15, true, "INT"); v.(*LInt).Value != -32763 {
		t.Errorf("INT 5 with bit 15 = %v", v)
	}
	if v, _ := SetBitAs(&ULInt{Value: 1}, 7, true, "BYTE"); v.(*ULInt).Value != 129 {
		t.Errorf("BYTE 1 with bit 7 = %v", v)
	}
	if v, _ := SetBitAs(&Int{Value: 1}, 3, true, "INT"); v.(*Int).Value != 9 {
		t.Errorf("INT 1 with bit 3 = %v", v)
	}
	if v, _ := SetBitAs(&LInt{Value: 1}, 63, true, "LINT"); v.(*LInt).Value >= 0 {
		t.Errorf("LINT with bit 63 = %v", v)
	}
	if _, err := SetBitAs(&LInt{}, 16, true, "INT"); err == nil {
		t.Error("bit 16 of INT accepted")
	}
	if _, err := GetBitAs(&LInt{}, 8, "sint"); err == nil {
		t.Error("bit 8 of SINT accepted")
	}
	if on, err := GetBitAs(&LInt{Value: 4}, 2, ""); err != nil || !on {
		t.Errorf("untyped bit 2: %v %v", on, err)
	}
	if got := bitTypeName(&BitString{Value: 0, Width: 16}); got != "WORD" {
		t.Errorf("bit string name = %s", got)
	}
}
