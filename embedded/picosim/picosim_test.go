//go:build picosim

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package picosim_test checks firmware running on an emulated Raspberry Pi
// Pico. It talks to the picosim container's UART0 over TCP:
//
//	docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
//	docker run -d --rm -p 4000:4000 beedance-picosim
//	go test -tags picosim ./embedded/picosim/
//
// PICOSIM_ADDR overrides the UART address (default localhost:4000).
package picosim_test

import (
	"bufio"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// dialUART connects to the emulator's UART0. The emulator replays its
// output backlog on connect, so lines printed at boot are not missed.
func dialUART(t *testing.T) net.Conn {
	t.Helper()
	addr := os.Getenv("PICOSIM_ADDR")
	if addr == "" {
		addr = "localhost:4000"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("connect to picosim UART at %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// expectLines reads UART lines until each of want has appeared in order,
// failing on a TinyGo runtime crash or when timeout passes.
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
		if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
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
