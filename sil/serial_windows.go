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
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetCommState   = kernel32.NewProc("GetCommState")
	procSetCommState   = kernel32.NewProc("SetCommState")
	procSetCommTimeout = kernel32.NewProc("SetCommTimeouts")
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

// openSerial opens a COM port raw, 8N1 at baud, DTR and RTS on (a USB CDC
// device such as a Pico sends only with DTR on); reads return as soon as a
// byte arrives.
func openSerial(path string, baud int) (io.ReadWriteCloser, error) {
	if !strings.HasPrefix(path, `\.\`) {
		path = `\.\` + path // COM10 and above need it
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	h := f.Fd()
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if ok, _, err := procGetCommState.Call(h, uintptr(unsafe.Pointer(&d))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("%s is not a serial port: %v", path, err)
	}
	const (
		fBinary          = 1 << 0
		fDtrControlOn    = 1 << 4
		fRtsControlOn    = 1 << 12
		dtrRtsAndControl = 0x3<<4 | 0x3<<12 | 1<<1 | 1<<2 | 1<<3 | 1<<6 | 1<<8 | 1<<9 // fParity..fInX, flow
	)
	d.BaudRate = uint32(baud)
	d.Flags = d.Flags&^dtrRtsAndControl | fBinary | fDtrControlOn | fRtsControlOn
	d.ByteSize, d.Parity, d.StopBits = 8, 0, 0 // 8N1
	if ok, _, err := procSetCommState.Call(h, uintptr(unsafe.Pointer(&d))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("configure %s: %v", path, err)
	}
	// A read returns when a byte arrives, waiting up to ~49 days for one.
	t := commTimeouts{ReadIntervalTimeout: 0xFFFFFFFF, ReadTotalTimeoutMultiplier: 0xFFFFFFFF, ReadTotalTimeoutConstant: 0xFFFFFFFE}
	if ok, _, err := procSetCommTimeout.Call(h, uintptr(unsafe.Pointer(&t))); ok == 0 {
		f.Close()
		return nil, fmt.Errorf("configure %s: %v", path, err)
	}
	return f, nil
}
