//go:build picosimrig

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package picosim_test

// The Pico rig (embedded/rig/pico) on the emulator, with GP2 wired to GP10
// and ADC0 at 2048:
//
//	docker build -f embedded/picosim/Dockerfile --build-arg PKG=./embedded/rig/pico -t beedance-picosim-rig .
//	docker run -d --rm -p 4100:4000 -e UART_BACKLOG=0 -e WIRES=2:10 -e ADC=0:2048 beedance-picosim-rig
//	go test -tags picosimrig ./embedded/picosim/
//
// UART_BACKLOG=0: a replayed backlog would hand a new client the replies
// to an earlier one.
//
// PICOSIM_RIG_ADDR overrides the rig's address (default localhost:4100).

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/sil"
)

func dialRig(t *testing.T) *sil.Rig {
	t.Helper()
	addr := os.Getenv("PICOSIM_RIG_ADDR")
	if addr == "" {
		addr = "localhost:4100"
	}
	rig, err := sil.OpenRig(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rig.Close() })
	return rig
}

var bench = []sil.Point{
	{Address: "%QX0.0", Type: "BOOL"},
	{Address: "%IX0.0", Type: "BOOL"},
	{Address: "%IW0", Type: "INT"},
}

func read(t *testing.T, rig *sil.Rig) map[string]object.Object {
	t.Helper()
	in, err := rig.Read(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// The output drives the input through the wire; the analog input reads.
func TestRigLoopback(t *testing.T) {
	ctx := context.Background()
	rig := dialRig(t)
	if err := rig.Begin(ctx, "TEST_Loopback", sil.VM, bench); err != nil {
		t.Fatal(err)
	}
	defer rig.End(ctx)
	if in := read(t, rig); in["%IX0.0"].Inspect() != "false" || in["%IW0"].Inspect() != "2048" {
		t.Fatalf("before: %v", in)
	}
	if err := rig.Write(ctx, 0, map[string]object.Object{"%QX0.0": &object.Boolean{Value: true}}); err != nil {
		t.Fatal(err)
	}
	if in := read(t, rig); in["%IX0.0"].Inspect() != "true" {
		t.Fatalf("after driving GP2: %v", in)
	}
	if err := rig.End(ctx); err != nil {
		t.Fatal(err)
	}
	// End made the output safe: the wire carries it back low.
	rig.Begin(ctx, "TEST_Loopback", sil.VM, bench)
	if in := read(t, rig); in["%IX0.0"].Inspect() != "false" {
		t.Fatalf("after end: %v", in)
	}
}

// A point the rig does not have, or of the wrong type, is refused.
func TestRigRefusesPoints(t *testing.T) {
	ctx := context.Background()
	rig := dialRig(t)
	for _, p := range []sil.Point{{Address: "%QX1.0", Type: "BOOL"}, {Address: "%QW0", Type: "WORD"},
		{Address: "%IX0.0", Type: "INT"}, {Address: "%IW0", Type: "REAL"}} {
		err := rig.Begin(ctx, "T", sil.VM, []sil.Point{p})
		if err == nil {
			t.Errorf("%v accepted", p)
		}
	}
	if _, err := rig.Read(ctx, 0); err == nil || !strings.Contains(err.Error(), "begin first") {
		t.Errorf("read without a run: %v", err)
	}
}

// A host that goes quiet within a run gets its outputs made safe.
func TestRigHostTimeout(t *testing.T) {
	ctx := context.Background()
	rig := dialRig(t)
	if err := rig.Begin(ctx, "T", sil.VM, bench); err != nil {
		t.Fatal(err)
	}
	rig.Write(ctx, 0, map[string]object.Object{"%QX0.0": &object.Boolean{Value: true}})
	time.Sleep(3 * time.Second)
	_, err := rig.Read(ctx, 0)
	if err == nil || !strings.Contains(err.Error(), "made safe") {
		t.Fatalf("after 3 s of silence: %v", err)
	}
	rig.Begin(ctx, "T", sil.VM, bench)
	defer rig.End(ctx)
	if in := read(t, rig); in["%IX0.0"].Inspect() != "false" {
		t.Errorf("output still on: %v", in)
	}
}
