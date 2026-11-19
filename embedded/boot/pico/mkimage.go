//go:build ignore

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// mkimage finishes the bootloader's image: it writes the CRC the bootrom
// checks into boot2's last 4 bytes, and writes the image as UF2 for the
// Pico's USB mass storage mode.
//
//	go run mkimage.go bootloader.bin bootloader.uf2
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

// boot2CRC is CRC-32/MPEG-2, the bootrom's check of boot2's first 252 bytes.
func boot2CRC(b []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, v := range b {
		crc ^= uint32(v) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// uf2 is the image at base as UF2 blocks of 256 bytes for the RP2040.
func uf2(img []byte, base uint32) []byte {
	const familyRP2040 = 0xE48BFF56
	n := (len(img) + 255) / 256
	var out []byte
	for i := 0; i < n; i++ {
		block := make([]byte, 512)
		le := binary.LittleEndian
		le.PutUint32(block[0:], 0x0A324655)
		le.PutUint32(block[4:], 0x9E5D5157)
		le.PutUint32(block[8:], 0x00002000) // family ID present
		le.PutUint32(block[12:], base+uint32(i*256))
		le.PutUint32(block[16:], 256)
		le.PutUint32(block[20:], uint32(i))
		le.PutUint32(block[24:], uint32(n))
		le.PutUint32(block[28:], familyRP2040)
		copy(block[32:], img[i*256:min(len(img), (i+1)*256)])
		le.PutUint32(block[508:], 0x0AB16F30)
		out = append(out, block...)
	}
	return out
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run mkimage.go bootloader.bin bootloader.uf2")
		os.Exit(2)
	}
	img, err := os.ReadFile(os.Args[1])
	if err != nil || len(img) < 256 {
		fmt.Fprintln(os.Stderr, "mkimage: no image of at least 256 bytes:", err)
		os.Exit(1)
	}
	binary.LittleEndian.PutUint32(img[252:], boot2CRC(img[:252]))
	if err := os.WriteFile(os.Args[1], img, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], uf2(img, 0x10000000), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
