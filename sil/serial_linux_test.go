/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// openPTY returns a pseudo-terminal's master and the path of its slave, a
// serial port as far as termios is concerned.
func openPTY(t *testing.T) (*os.File, string) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminals: %v", err)
	}
	var unlock int32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		t.Fatal(e)
	}
	var n uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); e != 0 {
		t.Fatal(e)
	}
	return m, fmt.Sprintf("/dev/pts/%d", n)
}

// The rig protocol over a serial line, the plant model at the far end.
func TestSerialRig(t *testing.T) {
	master, slave := openPTY(t)
	defer master.Close()
	k := &tank{}
	go ServeRig(context.Background(), master, k)
	rig, err := OpenRig(context.Background(), "serial:"+slave+"?baud=9600")
	if err != nil {
		t.Fatal(err)
	}
	defer rig.Close()
	results, err := Run(context.Background(), tankSource, Options{IO: rig, Deterministic: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := results[0]; !r.Passed {
		t.Fatalf("%+v", r)
	}
	if k.ends != 2 {
		t.Errorf("ends %d", k.ends)
	}
	if _, err := OpenRig(context.Background(), "serial:"+slave+"?baud=12345"); err == nil {
		t.Error("an unsupported baud rate accepted")
	}
	if _, err := OpenRig(context.Background(), "serial:/dev/null"); err == nil {
		t.Error("/dev/null accepted as a serial port")
	}
}
