//go:build esp32hw

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// These tests check firmware on a real ESP32 on a serial port: they reset
// the board through RTS (its EN pin, on the usual USB bridge) and read its
// UART. Flash the firmware a test checks first, holding BOOT if the board
// does not enter its bootloader by itself:
//
//	tinygo flash -target=esp32-coreboard-v2 -port COM6 ./embedded/example/raspberrypi
//	ESP32_PORT=COM6 go test -tags esp32hw -run EmbeddedVM ./embedded/esp32hw/
//
//	tinygo flash -target=esp32-coreboard-v2 -port COM6 ./embedded/esp32hw/plc
//	ESP32_PORT=COM6 go test -tags esp32hw -run PLC ./embedded/esp32hw/
//
// A test whose firmware is not on the board is skipped.
package esp32hw_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// line is a line from the board and when the host received it.
type line struct {
	text string
	at   time.Time
}

// boot resets the board on ESP32_PORT and returns its lines, as they
// arrive, until stop says so or timeout passes.
func boot(t *testing.T, timeout time.Duration, stop func(string) bool) []line {
	t.Helper()
	name := os.Getenv("ESP32_PORT")
	if name == "" {
		t.Skip("ESP32_PORT is not set (e.g. COM6 or /dev/ttyUSB0)")
	}
	p, err := openPort(name, 115200)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer p.Close()
	if err := p.reset(); err != nil {
		t.Fatalf("reset through RTS: %v", err)
	}
	var lines []line
	var pending []byte
	buf := make([]byte, 512)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := p.Read(buf)
		now := time.Now()
		if err != nil && !errors.Is(err, io.EOF) { // EOF: nothing within the read timeout
			t.Fatalf("read %s: %v", name, err)
		}
		pending = append(pending, buf[:n]...)
		for {
			i := bytes.IndexByte(pending, '\n')
			if i < 0 {
				break
			}
			text := strings.TrimRight(string(pending[:i]), "\r")
			pending = pending[i+1:]
			lines = append(lines, line{text, now})
			if strings.HasPrefix(text, "panic:") || strings.HasPrefix(text, "fatal error:") {
				t.Fatalf("firmware crashed: %s\n%s", text, dump(lines))
			}
			if stop(text) {
				return lines
			}
		}
	}
	return lines
}

func dump(lines []line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text + "\n")
	}
	return b.String()
}

// skipUnless skips the test when the board runs other firmware than the one
// marker identifies.
func skipUnless(t *testing.T, lines []line, marker, flash string) {
	t.Helper()
	for _, l := range lines {
		if strings.HasPrefix(l.text, marker) {
			return
		}
	}
	t.Skipf("the board does not run this test's firmware (no %q after reset): flash it with\n\ttinygo flash -target=esp32-coreboard-v2 -port %s %s\nboard output:\n%s",
		marker, os.Getenv("ESP32_PORT"), flash, dump(lines))
}

// The VM example, as esp32sim checks it under QEMU, on silicon.
func TestEmbeddedVM(t *testing.T) {
	lines := boot(t, 10*time.Second, func(s string) bool { return strings.HasPrefix(s, "Result:") || strings.HasPrefix(s, "plc: start") })
	skipUnless(t, lines, "--- Embedded Bytecode Disassembly ---", "./embedded/example/raspberrypi")
	want := []string{"Starting embedded beedance VM...", "VM execution finished.", "Result: 30"}
	for _, l := range lines {
		if len(want) > 0 && l.text == want[0] {
			want = want[1:]
		}
	}
	if len(want) > 0 {
		t.Fatalf("did not see %q\nboard output:\n%s", want[0], dump(lines))
	}
}

// A PLC program on the device's clock: a TON restarting itself pulses
// %QX0.0 every 1020 ms (1 s and two 10 ms scans). The pulses must keep that
// period by the device's clock and by the host's, and the scans must keep
// their schedule.
func TestPLC(t *testing.T) {
	lines := boot(t, 20*time.Second, func(s string) bool {
		return s == "plc: done" || strings.HasPrefix(s, "plc: error") || strings.HasPrefix(s, "Result:") // the VM example
	})
	skipUnless(t, lines, "plc: start", "./embedded/esp32hw/plc")
	t.Logf("board output:\n%s", dump(lines))

	type pulse struct {
		device time.Duration
		host   time.Time
	}
	var pulses []pulse
	var scans, periodAvg, periodMax, scanAvg, scanMax, overruns int
	done := false
	for _, l := range lines {
		var n, ms int
		if _, err := fmt.Sscanf(l.text, "pulse %d at %d ms", &n, &ms); err == nil {
			pulses = append(pulses, pulse{time.Duration(ms) * time.Millisecond, l.at})
		}
		if strings.HasPrefix(l.text, "stats ") {
			if _, err := fmt.Sscanf(l.text, "stats scans=%d period_avg_us=%d period_max_us=%d scan_avg_us=%d scan_max_us=%d overruns=%d",
				&scans, &periodAvg, &periodMax, &scanAvg, &scanMax, &overruns); err != nil {
				t.Fatalf("stats line %q: %v", l.text, err)
			}
		}
		if strings.HasPrefix(l.text, "plc: error") {
			t.Fatal(l.text)
		}
		done = done || l.text == "plc: done"
	}
	if !done {
		t.Fatal("no \"plc: done\"")
	}
	if len(pulses) < 4 {
		t.Fatalf("%d pulses in six seconds, want at least 4", len(pulses))
	}

	// Each period by the device's clock: 1020 ms, give or take a scan.
	const period = 1020 * time.Millisecond
	for i := 1; i < len(pulses); i++ {
		d := pulses[i].device - pulses[i-1].device
		if d < period-10*time.Millisecond || d > period+20*time.Millisecond {
			t.Errorf("pulse %d came %v after pulse %d by the device's clock, want %v", i+1, d, i, period)
		}
	}
	// The device's clock against the host's, over all the pulses: serial
	// latency aside, they must agree. A wrongly set CPU or timer clock would
	// be off by a factor.
	first, last := pulses[0], pulses[len(pulses)-1]
	deviceSpan, hostSpan := last.device-first.device, last.host.Sub(first.host)
	if diff := math.Abs(float64(deviceSpan - hostSpan)); diff > float64(30*time.Millisecond) {
		t.Errorf("over %d pulses the device's clock measured %v, the host's %v", len(pulses)-1, deviceSpan, hostSpan)
	}

	if scans != 600 || overruns != 0 {
		t.Errorf("%d scans, %d overruns: want 600, 0", scans, overruns)
	}
	if periodAvg < 9900 || periodAvg > 10100 {
		t.Errorf("average scan period %d µs, want 10000", periodAvg)
	}
	if scanMax >= 10000 {
		t.Errorf("longest scan %d µs: not within the 10 ms interval", scanMax)
	}
	t.Logf("device %v vs host %v over %d pulses; scan period avg %d µs, max %d µs; scan time avg %d µs, max %d µs",
		deviceSpan, hostSpan.Round(time.Millisecond), len(pulses)-1, periodAvg, periodMax, scanAvg, scanMax)
}
