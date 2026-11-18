/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package harden

import (
	"errors"
	"testing"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/vm"
)

// image is a program image with what a compiled program's constants hold:
// values of several kinds and a compiled function.
func image() []*compiler.Bytecode {
	fn := &object.CompiledFunction{Instructions: []byte{1, 2, 3, 4, 5}, NumLocals: 2, NumParameters: 1,
		ParameterNames: []string{"x"}, OutputNames: []string{"y"}, OutputIndices: []int{1}, Name: "F"}
	return []*compiler.Bytecode{
		{Instructions: []byte{9, 8, 7, 6}, Constants: []object.Object{&object.Int{Value: 7}, fn}},
		{Instructions: []byte{0, 1, 0, 2, 0, 3}, Constants: []object.Object{
			&object.LReal{Value: 0.5}, &object.String{Value: "abc"}, vm.True,
			&object.Array{LowerBound: 1, Elements: []object.Object{&object.DInt{Value: 1}, &object.DInt{Value: 2}}},
			&object.Date{Value: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)},
		}},
	}
}

// A flip of any one bit of the image changes its checksum: in the
// instructions, in a function's instructions, in a constant's value.
func TestChecksumSeesFlips(t *testing.T) {
	want := Checksum(image()...)
	if Checksum(image()...) != want {
		t.Fatal("checksum not stable")
	}
	for _, c := range []struct {
		name string
		flip func([]*compiler.Bytecode)
	}{
		{"instruction", func(b []*compiler.Bytecode) { b[1].Instructions[3] ^= 0x10 }},
		{"function instruction", func(b []*compiler.Bytecode) {
			b[0].Constants[1].(*object.CompiledFunction).Instructions[0] ^= 1
		}},
		{"function locals", func(b []*compiler.Bytecode) { b[0].Constants[1].(*object.CompiledFunction).NumLocals ^= 4 }},
		{"INT constant", func(b []*compiler.Bytecode) { b[0].Constants[0].(*object.Int).Value ^= 1 }},
		{"LREAL constant", func(b []*compiler.Bytecode) { b[1].Constants[0].(*object.LReal).Value = 0.5000001 }},
		{"array element", func(b []*compiler.Bytecode) {
			b[1].Constants[3].(*object.Array).Elements[1].(*object.DInt).Value ^= 2
		}},
	} {
		img := image()
		c.flip(img)
		if Checksum(img...) == want {
			t.Errorf("%s: flip not seen", c.name)
		}
	}
	// What the generated code drops of a function does not count.
	img := image()
	img[0].Constants[1].(*object.CompiledFunction).Name = "G"
	if Checksum(img...) != want {
		t.Error("a function's name counted")
	}
}

// A Scrubber's passes, however small its steps, agree with Checksum, and a
// flip made between passes fails the next.
func TestScrubber(t *testing.T) {
	img := image()
	want := Checksum(img...)
	for _, step := range []int{1, 3, 7, 1000} {
		s := NewScrubber(want, img...)
		passes := 0
		for i := 0; i < 1000 && passes < 2; i++ {
			done, ok := s.Step(step)
			if done {
				if !ok {
					t.Fatalf("step %d: pass %d failed on a sound image", step, passes)
				}
				passes++
			} else if !ok {
				t.Fatalf("step %d: ok false before a pass ended", step)
			}
		}
		if passes != 2 {
			t.Fatalf("step %d: %d passes", step, passes)
		}
	}
	s := NewScrubber(want, img...)
	img[1].Instructions[0] ^= 0x80
	for {
		if done, ok := s.Step(5); done {
			if ok {
				t.Error("flip not seen by the scrubber")
			}
			break
		}
	}
	img[1].Instructions[0] ^= 0x80
	for {
		if done, ok := s.Step(5); done {
			if !ok {
				t.Error("repaired image still fails")
			}
			break
		}
	}
}

func TestRepairSingletons(t *testing.T) {
	if RepairSingletons() {
		t.Fatal("sound singletons reported")
	}
	vm.True.Value = false
	if !RepairSingletons() || !vm.True.Value {
		t.Fatal("TRUE not repaired")
	}
	vm.False.Value = true
	if !RepairSingletons() || vm.False.Value {
		t.Fatal("FALSE not repaired")
	}
}

// globals are a program's variables as its initialization left them.
func globals() []object.Object {
	return []object.Object{
		&object.Int{Value: 0},
		vm.False,
		&object.LReal{Value: 0},
		&object.String{Value: ""},
		&object.Array{LowerBound: 0, Elements: []object.Object{&object.UDInt{Value: 0}, &object.UDInt{Value: 0}}},
		&object.Hash{Pairs: map[object.HashKey]object.HashPair{
			(&object.Int{Value: 1}).HashKey(): {Key: &object.Int{Value: 1}, Value: &object.Time{Value: 0}},
		}},
		&object.Int{Value: 5}, // not retained
	}
}

var slots = []int{0, 1, 2, 3, 4, 5}

func set(g []object.Object, n int) {
	g[0] = &object.Int{Value: int16(n)}
	g[1] = vm.True
	g[2] = &object.LReal{Value: float64(n) / 4}
	g[3] = &object.String{Value: "run"}
	g[4].(*object.Array).Elements[1] = &object.UDInt{Value: uint32(n * 10)}
	k := (&object.Int{Value: 1}).HashKey()
	h := g[5].(*object.Hash)
	h.Pairs[k] = object.HashPair{Key: h.Pairs[k].Key, Value: &object.Time{Value: time.Duration(n) * time.Second}}
	g[6] = &object.Int{Value: 99}
}

func check(t *testing.T, g []object.Object, n int) {
	t.Helper()
	if g[0].(*object.Int).Value != int16(n) || g[1] != vm.True || g[2].(*object.LReal).Value != float64(n)/4 ||
		g[3].(*object.String).Value != "run" || g[4].(*object.Array).Elements[1].(*object.UDInt).Value != uint32(n*10) {
		t.Fatalf("loaded %v %v %v %v %v", g[0].Inspect(), g[1].Inspect(), g[2].Inspect(), g[3].Inspect(), g[4].Inspect())
	}
	tm := g[5].(*object.Hash).Pairs[(&object.Int{Value: 1}).HashKey()].Value.(*object.Time).Value
	if tm != time.Duration(n)*time.Second {
		t.Fatalf("struct member %v", tm)
	}
	if g[6].(*object.Int).Value != 5 {
		t.Fatal("a variable not retained was loaded")
	}
}

// Values saved are loaded into a fresh start; saves alternate copies.
func TestRetainRoundTrip(t *testing.T) {
	store := &MemStore{}
	layout := Layout("n", "INT")
	if err := NewRetain(store, slots, layout).Load(globals()); !errors.Is(err, ErrNoRetain) {
		t.Fatalf("empty store: %v", err)
	}
	r := NewRetain(store, slots, layout)
	g := globals()
	set(g, 3)
	if err := r.Save(g); err != nil {
		t.Fatal(err)
	}
	set(g, 4)
	if err := r.Save(g); err != nil {
		t.Fatal(err)
	}
	if store.Copies[0] == nil || store.Copies[1] == nil {
		t.Fatal("saves did not alternate copies")
	}
	g = globals()
	if err := NewRetain(store, slots, layout).Load(g); err != nil {
		t.Fatal(err)
	}
	check(t, g, 4)
}

// An unchanged save writes nothing.
func TestRetainSkipsUnchanged(t *testing.T) {
	writes := 0
	store := &countingStore{n: &writes}
	r := NewRetain(store, slots, 1)
	g := globals()
	set(g, 1)
	for range 3 {
		if err := r.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 {
		t.Fatalf("%d writes", writes)
	}
}

type countingStore struct {
	MemStore
	n *int
}

func (c *countingStore) WriteCopy(i int, data []byte) error {
	*c.n++
	return c.MemStore.WriteCopy(i, data)
}

// A spoilt newer copy, from a bit flip or a reset mid-write, is passed
// over for the older.
func TestRetainFallsBack(t *testing.T) {
	store := &MemStore{}
	r := NewRetain(store, slots, 7)
	g := globals()
	set(g, 1)
	r.Save(g)
	set(g, 2)
	r.Save(g)
	newest := 1
	store.Copies[newest][headerSize+3] ^= 0x04
	g = globals()
	if err := NewRetain(store, slots, 7).Load(g); err != nil {
		t.Fatal(err)
	}
	check(t, g, 1)
	// The next save goes over the spoilt copy, not the good one.
	r = NewRetain(store, slots, 7)
	g = globals()
	r.Load(g)
	set(g, 5)
	r.Save(g)
	g = globals()
	NewRetain(store, slots, 7).Load(g)
	check(t, g, 5)
	if _, _, ok := r.parse(store.Copies[0]); !ok {
		t.Error("the good copy was overwritten")
	}
}

// Data of another layout, or that does not fit the variables, is not
// loaded.
func TestRetainRefusesOtherPrograms(t *testing.T) {
	store := &MemStore{}
	g := globals()
	set(g, 2)
	NewRetain(store, slots, 1).Save(g)
	if err := NewRetain(store, slots, 2).Load(globals()); !errors.Is(err, ErrNoRetain) {
		t.Fatalf("other layout: %v", err)
	}
	// Same layout, but variable 0 is now a REAL.
	g = globals()
	g[0] = &object.Real{Value: 1}
	if err := NewRetain(store, slots, 1).Load(g); !errors.Is(err, ErrNoRetain) {
		t.Fatalf("other type: %v", err)
	}
	if g[0].(*object.Real).Value != 1 {
		t.Fatal("variables changed by a refused load")
	}
}

// The sequence number may wrap.
func TestRetainSequenceWraps(t *testing.T) {
	store := &MemStore{}
	r := NewRetain(store, slots, 1)
	r.seq = ^uint32(0) - 1
	g := globals()
	set(g, 1)
	r.Save(g) // seq max
	set(g, 2)
	r.Save(g) // seq 0
	g = globals()
	NewRetain(store, slots, 1).Load(g)
	check(t, g, 2)
}

// A variable with a value the encoding does not wholly cover keeps its
// initial value, whole, while the others load.
func TestRetainSkipsUncovered(t *testing.T) {
	fb := func(et time.Duration) *object.Hash {
		return &object.Hash{Pairs: map[object.HashKey]object.HashPair{
			(&object.Int{Value: 1}).HashKey(): {Key: &object.Int{Value: 1}, Value: &object.Time{Value: et}},
			(&object.Int{Value: 2}).HashKey(): {Key: &object.Int{Value: 2}, Value: &object.Builtin{}},
		}}
	}
	store := &MemStore{}
	g := []object.Object{&object.Int{Value: 4}, fb(3 * time.Second)}
	NewRetain(store, []int{0, 1}, 1).Save(g)
	init := fb(0)
	g = []object.Object{&object.Int{Value: 0}, init}
	if err := NewRetain(store, []int{0, 1}, 1).Load(g); err != nil {
		t.Fatal(err)
	}
	if g[0].(*object.Int).Value != 4 || g[1] != init {
		t.Fatalf("loaded %s %s", g[0].Inspect(), g[1].Inspect())
	}
}
