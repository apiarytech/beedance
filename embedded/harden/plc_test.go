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
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

// counter is a program that adds step to a RETAIN count each scan, runs a
// TON (a built-in, the clock) and fails a scan while %IX0.0 is set.
func counter(step string) string {
	return strings.ReplaceAll(`
PROGRAM Main
VAR RETAIN
	count : DINT;
END_VAR
VAR
	idx : INT := 1;
	arr : ARRAY[1..2] OF INT;
	bad AT %IX0.0 : BOOL;
	t : TON;
END_VAR
count := count + STEP;
t(IN := TRUE, PT := T#1s);
IF bad THEN
	arr[idx + 5] := 1;
END_IF;
END_PROGRAM
`, "STEP", step)
}

func compile(t *testing.T, src string) Image {
	t.Helper()
	object.FinalizeBuiltins()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	cp, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(prog, "Main")
	if err != nil {
		t.Fatal(err)
	}
	return ImageOf(cp)
}

func encode(t *testing.T, src string) []byte {
	t.Helper()
	b, err := EncodeImage(compile(t, src))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newPLC is a PLC whose golden program counts by 1, with two slots.
func newPLC(t *testing.T, store *MemImageStore) *PLC {
	src := counter("1")
	return &PLC{
		Golden:         func() Image { return compile(t, src) },
		GoldenChecksum: compile(t, src).Checksum(),
		Interval:       time.Microsecond,
		Images:         store,
		TrialScans:     3,
		Restarts:       -1,
	}
}

func count(p *PLC, m *vm.VM) int64 {
	switch v := m.Globals()[p.Image().RetainSlots[0]].(type) {
	case *object.DInt:
		return int64(v.Value)
	case *object.LInt:
		return v.Value
	}
	return -1
}

// send writes data to p as an update, in parts of k bytes, and commits it.
func send(t *testing.T, p *PLC, data []byte, k int) {
	t.Helper()
	if err := p.BeginUpdate(len(data)); err != nil {
		t.Fatal(err)
	}
	for len(data) > 0 {
		n := min(k, len(data))
		if err := p.WriteUpdate(data[:n]); err != nil {
			t.Fatal(err)
		}
		data = data[n:]
	}
	if err := p.CommitUpdate(); err != nil {
		t.Fatal(err)
	}
}

func TestImageRoundTrip(t *testing.T) {
	im := compile(t, counter("7"))
	data, err := EncodeImage(im)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeImage(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Checksum() != im.Checksum() || got.Globals != im.Globals || got.RetainLayout != im.RetainLayout ||
		len(got.RetainSlots) != 1 || got.RetainSlots[0] != im.RetainSlots[0] {
		t.Fatalf("decoded image differs")
	}
	for i := range data {
		bad := append([]byte(nil), data...)
		bad[i] ^= 0x01
		if _, err := DecodeImage(bad); !errors.Is(err, ErrImageFormat) {
			t.Fatalf("flip at %d: %v", i, err)
		}
	}
	if _, err := DecodeImage(data[:len(data)-1]); !errors.Is(err, ErrImageFormat) {
		t.Fatalf("cut short: %v", err)
	}
}

// An image whose built-ins this firmware has under other indexes is
// refused.
func TestImageIncompatible(t *testing.T) {
	data := encode(t, counter("1"))
	refs, _ := builtinRefs(compile(t, counter("1")).Scan)
	if len(refs) == 0 {
		t.Fatal("the program calls no built-in")
	}
	name := object.Builtins[refs[0]].Name
	object.Builtins[refs[0]].Name = "OTHER"
	defer func() { object.Builtins[refs[0]].Name = name }()
	if _, err := DecodeImage(data); !errors.Is(err, ErrIncompatible) {
		t.Fatalf("got %v", err)
	}
}

// A scrub pass allocates nothing once its buffers are made.
func TestScrubberAllocates(t *testing.T) {
	im := compile(t, counter("1"))
	s := NewScrubber(im.Checksum(), im.Init, im.Scan)
	s.Step(1 << 20)
	if a := testing.AllocsPerRun(20, func() {
		if done, ok := s.Step(1 << 20); !done || !ok {
			t.Fatal("pass failed")
		}
	}); a != 0 {
		t.Fatalf("%v allocations a pass", a)
	}
}

// An update written in parts while the program scans runs from the next
// scan, on trial, then is kept; a reset then starts it, not the golden
// program, without a trial.
func TestUpdateRunsAndIsKept(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	update := encode(t, counter("100"))
	var counts []int64
	n, parts := 0, 0
	scans := len(update)/100 + 10
	err := p.Run(scans, func(m *vm.VM) {
		counts = append(counts, count(p, m))
		n++
		switch {
		case n == 2:
			if err := p.BeginUpdate(len(update)); err != nil {
				t.Fatal(err)
			}
		case n > 2 && parts*100 < len(update):
			// One part a scan: the program scans on.
			k := min(100, len(update)-parts*100)
			if err := p.WriteUpdate(update[parts*100 : parts*100+k]); err != nil {
				t.Fatal(err)
			}
			if parts++; parts*100 >= len(update) {
				if err := p.CommitUpdate(); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// The golden program counts 1, 2, ... until the commit, then the
	// update counts 100, 200, ...
	last := counts[len(counts)-1]
	if counts[0] != 1 || last%100 != 0 || last == 0 {
		t.Fatalf("counts %v", counts)
	}
	if p.Active() != 0 || p.OnTrial() {
		t.Fatalf("active %d, trial %v", p.Active(), p.OnTrial())
	}
	s, _ := p.readSlot(0)
	if !s.started || !s.kept {
		t.Fatalf("slot marks: started %v kept %v", s.started, s.kept)
	}
	q := newPLC(t, store)
	q.Run(1, nil)
	if q.Active() != 0 || q.OnTrial() {
		t.Fatalf("after reset: active %d, trial %v", q.Active(), q.OnTrial())
	}
}

// A new program that faults on trial is rolled back to the program before
// it: the golden one, then a kept update.
func TestUpdateRollsBack(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	good := encode(t, counter("100"))
	bad := encode(t, counter("10000"))
	var safe []error
	p.SafeState = func(err error) { safe = append(safe, err) }
	n := 0
	var last int64
	err := p.Run(20, func(m *vm.VM) {
		n++
		last = count(p, m)
		switch n {
		case 1:
			send(t, p, bad, 64)
		case 2:
			m.IO()["%IX0.0"] = vm.True // the update faults on its first scan
		case 4:
			if p.Active() != -1 || p.Rollbacks != 1 {
				t.Errorf("after the first rollback: active %d, rollbacks %d", p.Active(), p.Rollbacks)
			}
			send(t, p, good, 64)
		case 10:
			send(t, p, bad, 64)
		case 11:
			m.IO()["%IX0.0"] = vm.True
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rollbacks != 2 || p.Active() < 0 || last%100 != 0 || last%10000 == 0 {
		t.Fatalf("rollbacks %d, active %d, count %d", p.Rollbacks, p.Active(), last)
	}
	if len(safe) != 2 || p.RollbackErr == nil {
		t.Fatalf("safe %v, rollback error %v", safe, p.RollbackErr)
	}
	// The kept update stays; the faulty one is cleared.
	if s := p.slots(); len(s) != 1 || s[0].i != p.Active() {
		t.Fatalf("slots %+v", s)
	}
}

// A reset while an update is on trial rolls it back at the next start.
func TestUpdateResetOnTrial(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	p.TrialScans = 100
	update := encode(t, counter("100"))
	n := 0
	p.Run(3, func(m *vm.VM) {
		if n++; n == 1 {
			send(t, p, update, 1000)
		}
	})
	if !p.OnTrial() {
		t.Fatal("update not on trial")
	}
	q := newPLC(t, store)
	q.Run(1, nil)
	if q.Active() != -1 || len(q.slots()) != 0 {
		t.Fatalf("active %d, slots %d", q.Active(), len(q.slots()))
	}
}

// While an update holds the program, no scan runs: OnHold writes it, the
// outputs are in their safe state, and the scans resume on the update.
func TestUpdateHolds(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	p.HoldDuringUpdate = true
	update := encode(t, counter("100"))
	var events []string
	p.SafeState = func(err error) { events = append(events, "safe") }
	sent := 0
	p.OnHold = func() {
		events = append(events, "hold")
		k := min(500, len(update)-sent)
		p.WriteUpdate(update[sent : sent+k])
		if sent += k; sent == len(update) {
			if err := p.CommitUpdate(); err != nil {
				t.Fatal(err)
			}
		}
	}
	n := 0
	p.Run(4, func(m *vm.VM) {
		events = append(events, "scan")
		if n++; n == 2 {
			p.BeginUpdate(len(update))
		}
	})
	got := strings.Join(events, " ")
	want := "scan scan safe " + strings.Repeat("hold ", (len(update)+499)/500) + "scan scan"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

// An update spoilt in transit, or too big, is refused, and the program
// runs on; so is one before Images are given.
func TestUpdateRefused(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	update := encode(t, counter("100"))
	spoilt := append([]byte(nil), update...)
	spoilt[len(spoilt)/2] ^= 0x40
	n := 0
	p.Run(3, func(m *vm.VM) {
		if n++; n != 1 {
			return
		}
		p.BeginUpdate(len(spoilt))
		p.WriteUpdate(spoilt)
		if err := p.CommitUpdate(); !errors.Is(err, ErrImageFormat) {
			t.Errorf("spoilt: %v", err)
		}
		if err := p.BeginUpdate(store.Size); !errors.Is(err, ErrUpdateSize) {
			t.Errorf("too big: %v", err)
		}
		p.BeginUpdate(len(update))
		if err := p.BeginUpdate(len(update)); !errors.Is(err, ErrUpdateBusy) {
			t.Errorf("twice: %v", err)
		}
		if err := p.WriteUpdate(append(update, 0)); !errors.Is(err, ErrUpdateSize) {
			t.Errorf("past its size: %v", err)
		}
		if err := p.CommitUpdate(); !errors.Is(err, ErrNoUpdate) {
			t.Errorf("after an error: %v", err)
		}
	})
	if p.Active() != -1 {
		t.Fatalf("active %d", p.Active())
	}
	p = newPLC(t, store)
	p.Images = nil
	if err := p.BeginUpdate(len(update)); !errors.Is(err, ErrNoImages) {
		t.Fatalf("no images: %v", err)
	}
}

// Firmware flashed with another golden program runs it, not an update
// sent to the firmware before.
func TestUpdateNewFirmware(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	p.TrialScans = 1
	update := encode(t, counter("100"))
	n := 0
	p.Run(3, func(m *vm.VM) {
		if n++; n == 1 {
			send(t, p, update, 1000)
		}
	})
	if p.Active() != 0 {
		t.Fatal("update not running")
	}
	q := newPLC(t, store)
	q.GoldenChecksum ^= 1 // as another firmware's
	q.Golden = func() Image { im := compile(t, counter("1")); return im }
	q.boot()
	if q.Active() != -1 {
		t.Fatalf("active %d", q.Active())
	}
}

// A slot's program found corrupt in RAM after a fault is made afresh
// from flash.
func TestUpdateReloadsFromSlot(t *testing.T) {
	store := &MemImageStore{Size: 1 << 16}
	p := newPLC(t, store)
	p.TrialScans = 1
	p.ScrubBytes = 1 << 20
	update := encode(t, counter("100"))
	n := 0
	var last int64
	err := p.Run(8, func(m *vm.VM) {
		n++
		last = count(p, m)
		switch n {
		case 1:
			send(t, p, update, 1000)
		case 4:
			p.Image().Scan.Constants[0] = &object.DInt{Value: 3}
		}
	})
	if err != nil || p.Active() != 0 || p.Faults != 1 || last%100 != 0 {
		t.Fatalf("err %v, active %d, faults %d, count %d", err, p.Active(), p.Faults, last)
	}
}
