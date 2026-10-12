//go:build esp32hw && windows

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
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetCommState       = kernel32.NewProc("GetCommState")
	procSetCommState       = kernel32.NewProc("SetCommState")
	procSetCommTimeouts    = kernel32.NewProc("SetCommTimeouts")
	procEscapeCommFunction = kernel32.NewProc("EscapeCommFunction")
	procPurgeComm          = kernel32.NewProc("PurgeComm")
)

// dcb is the Win32 DCB structure.
type dcb struct {
	DCBlength  uint32
	BaudRate   uint32
	Flags      uint32
	wReserved  uint16
	XonLim     uint16
	XoffLim    uint16
	ByteSize   byte
	Parity     byte
	StopBits   byte
	XonChar    byte
	XoffChar   byte
	ErrorChar  byte
	EofChar    byte
	EvtChar    byte
	wReserved1 uint16
}

type commTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

const (
	setRTS = 3
	clrRTS = 4
	clrDTR = 6
)

type port struct{ f *os.File }

// openPort opens a COM port 8N1 at baud with DTR and RTS off, so a board
// whose USB bridge drives EN and IO0 from them runs. A read returns what
// has arrived within 100 ms.
func openPort(name string, baud int) (*port, error) {
	f, err := os.OpenFile(`\\.\`+strings.TrimPrefix(name, `\\.\`), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	h := f.Fd()
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if ok, _, err := procGetCommState.Call(h, uintptr(unsafe.Pointer(&d))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("%s is not a serial port: %v", name, err)
	}
	d.BaudRate = uint32(baud)
	d.Flags = 1 // fBinary; DTR and RTS control disabled, no flow control
	d.ByteSize, d.Parity, d.StopBits = 8, 0, 0
	if ok, _, err := procSetCommState.Call(h, uintptr(unsafe.Pointer(&d))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("%s: SetCommState: %v", name, err)
	}
	ct := commTimeouts{ReadIntervalTimeout: 20, ReadTotalTimeoutConstant: 100}
	if ok, _, err := procSetCommTimeouts.Call(h, uintptr(unsafe.Pointer(&ct))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("%s: SetCommTimeouts: %v", name, err)
	}
	return &port{f: f}, nil
}

func (p *port) escape(fn uintptr) error {
	if ok, _, err := procEscapeCommFunction.Call(p.f.Fd(), fn); ok == 0 {
		return err
	}
	return nil
}

// reset pulses EN low through RTS, with IO0 high (DTR off): a normal boot.
// What arrived before it is discarded.
func (p *port) reset() error {
	if err := p.escape(clrDTR); err != nil {
		return err
	}
	if err := p.escape(setRTS); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	const purgeRxClear = 0x0008
	procPurgeComm.Call(p.f.Fd(), purgeRxClear)
	return p.escape(clrRTS)
}

func (p *port) Read(b []byte) (int, error) { return p.f.Read(b) }
func (p *port) Close() error               { return p.f.Close() }
