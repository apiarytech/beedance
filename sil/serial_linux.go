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
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// cbaud masks the speed bits of c_cflag (asm-generic/termbits.h);
// package syscall does not define it.
const cbaud = 0o10017

var linuxBauds = map[int]uint32{
	1200: syscall.B1200, 2400: syscall.B2400, 4800: syscall.B4800, 9600: syscall.B9600,
	19200: syscall.B19200, 38400: syscall.B38400, 57600: syscall.B57600, 115200: syscall.B115200,
	230400: syscall.B230400, 460800: syscall.B460800, 921600: syscall.B921600,
}

// openSerial opens a serial port raw, 8N1 at baud, reads blocking until a
// byte arrives.
func openSerial(path string, baud int) (io.ReadWriteCloser, error) {
	speed, ok := linuxBauds[baud]
	if !ok {
		return nil, fmt.Errorf("baud %d is not supported", baud)
	}
	f, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	var t syscall.Termios
	if err := termios(f, syscall.TCGETS, &t); err != nil {
		f.Close()
		return nil, fmt.Errorf("%s is not a serial port: %w", path, err)
	}
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR |
		syscall.IGNCR | syscall.ICRNL | syscall.IXON | syscall.IXOFF
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB | syscall.CSTOPB | cbaud
	t.Cflag |= syscall.CS8 | syscall.CREAD | syscall.CLOCAL | speed
	t.Ispeed, t.Ospeed = speed, speed
	t.Cc[syscall.VMIN], t.Cc[syscall.VTIME] = 1, 0
	if err := termios(f, syscall.TCSETS, &t); err != nil {
		f.Close()
		return nil, fmt.Errorf("configure %s: %w", path, err)
	}
	return f, nil
}

func termios(f *os.File, req uintptr, t *syscall.Termios) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(t))); errno != 0 {
		return errno
	}
	return nil
}
