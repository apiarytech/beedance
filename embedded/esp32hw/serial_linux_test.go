//go:build esp32hw && linux

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package esp32hw_test

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

type port struct{ f *os.File }

var bauds = map[int]uint32{9600: syscall.B9600, 115200: syscall.B115200, 230400: syscall.B230400}

// openPort opens a tty raw, 8N1 at baud, with DTR and RTS off, so a board
// whose USB bridge drives EN and IO0 from them runs. A read returns what
// has arrived within 100 ms.
func openPort(name string, baud int) (*port, error) {
	speed, ok := bauds[baud]
	if !ok {
		return nil, fmt.Errorf("baud %d not supported", baud)
	}
	f, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	t := syscall.Termios{
		Cflag:  speed | syscall.CS8 | syscall.CREAD | syscall.CLOCAL,
		Ispeed: speed,
		Ospeed: speed,
	}
	t.Cc[syscall.VMIN], t.Cc[syscall.VTIME] = 0, 1 // 100 ms
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&t))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("%s: %v", name, e)
	}
	p := &port{f: f}
	p.modem(syscall.TIOCMBIC, syscall.TIOCM_DTR|syscall.TIOCM_RTS)
	return p, nil
}

func (p *port) modem(req uintptr, bits int) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, p.f.Fd(), req, uintptr(unsafe.Pointer(&bits))); e != 0 {
		return e
	}
	return nil
}

// reset pulses EN low through RTS, with IO0 high (DTR off): a normal boot.
// What arrived before it is discarded.
func (p *port) reset() error {
	if err := p.modem(syscall.TIOCMBIC, syscall.TIOCM_DTR); err != nil {
		return err
	}
	if err := p.modem(syscall.TIOCMBIS, syscall.TIOCM_RTS); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	const tcflsh, tciflush = 0x540B, 0
	syscall.Syscall(syscall.SYS_IOCTL, p.f.Fd(), tcflsh, tciflush)
	return p.modem(syscall.TIOCMBIC, syscall.TIOCM_RTS)
}

func (p *port) Read(b []byte) (int, error) { return p.f.Read(b) }
func (p *port) Close() error               { return p.f.Close() }
