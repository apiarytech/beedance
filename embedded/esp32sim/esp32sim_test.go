//go:build esp32sim

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package esp32sim_test checks firmware running on an emulated ESP32. It
// talks to the esp32sim container's UART0 over TCP:
//
//	docker build -f embedded/esp32sim/Dockerfile -t beedance-esp32sim .
//	docker run -d --rm -p 4000:4000 beedance-esp32sim
//	go test -tags esp32sim ./embedded/esp32sim/
//
// ESP32SIM_ADDR overrides the UART address (default localhost:4000).
package esp32sim_test

import (
	"bufio"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// dialUART connects to the emulator's UART0. The emulated CPU starts when
// the first client connects, so no boot output is missed.
func dialUART(t *testing.T) net.Conn {
	t.Helper()
	addr := os.Getenv("ESP32SIM_ADDR")
	if addr == "" {
		addr = "localhost:4000"
	}
	var conn net.Conn
	var err error
	// The container may still be starting QEMU.
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		if conn, err = net.DialTimeout("tcp", addr, 5*time.Second); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("connect to esp32sim UART at %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// expectLines reads UART lines until each of want has appeared in order,
// failing on a TinyGo runtime crash, an ESP32 exception or when timeout
// passes.
func expectLines(t *testing.T, conn net.Conn, timeout time.Duration, want ...string) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatal(err)
	}
	var seen []string
	sc := bufio.NewScanner(conn)
	for len(want) > 0 && sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		seen = append(seen, line)
		if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") ||
			strings.HasPrefix(line, "Guru Meditation Error") || strings.HasPrefix(line, "invalid header") {
			t.Fatalf("firmware crashed: %s\nUART output:\n%s", line, strings.Join(seen, "\n"))
		}
		if line == want[0] {
			want = want[1:]
		}
	}
	if len(want) > 0 {
		t.Fatalf("did not see %q (%v)\nUART output:\n%s", want[0], sc.Err(), strings.Join(seen, "\n"))
	}
}

func TestEmbeddedVM(t *testing.T) {
	conn := dialUART(t)
	expectLines(t, conn, 60*time.Second,
		"Starting embedded beedance VM...",
		"VM execution finished.",
		"Result: 30",
	)
}
