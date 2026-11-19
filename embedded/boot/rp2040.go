//go:build rp2040 && beedance_boot

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package boot

import (
	"device/arm"
	"machine"
	"unsafe"
)

// Pico is the bootloader's app side on this chip. machine.Flash starts at
// the status sectors under pico/pico-boot.json's linker script.
var Pico = &Boot{Flash: machine.Flash}

// Code returns the image in the run slot as flash holds it, and the CRC-32
// the bootloader checked it against: Firmware.Code and CodeChecksum of a
// transpiled program, so that each scan checks a slice of the firmware.
func Code() ([]byte, uint32) {
	r, err := Pico.Status()
	if err != nil || r.RunSize == 0 || r.RunSize > RegionSize {
		return nil, 0
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(XIPBase+RunOff))), r.RunSize), r.RunCRC
}

// Reset resets the chip, into the bootloader: to install a committed
// update, or after a fault the program cannot recover from in place.
func Reset() { arm.SystemReset() }
