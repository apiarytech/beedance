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
	"strconv"
	"strings"
)

// DefaultBaud is a serial rig's speed unless its address gives one. A USB
// CDC port (a Raspberry Pi Pico's) ignores it.
const DefaultBaud = 115200

// OpenRig connects to a rig at addr:
//
//	tcp://host:port or host:port     over TCP
//	serial:/dev/ttyACM0              a serial port, 8N1 at DefaultBaud
//	serial:COM3?baud=9600            on Windows, at 9600 baud
func OpenRig(ctx context.Context, addr string) (*Rig, error) {
	path, ok := strings.CutPrefix(addr, "serial:")
	if !ok {
		return DialRig(ctx, addr)
	}
	path = strings.TrimPrefix(path, "//")
	baud := DefaultBaud
	if p, q, found := strings.Cut(path, "?"); found {
		path = p
		v, ok := strings.CutPrefix(q, "baud=")
		n, err := strconv.Atoi(v)
		if !ok || err != nil || n <= 0 {
			return nil, fmt.Errorf("rig %s: want serial:PATH?baud=N", addr)
		}
		baud = n
	}
	if path == "" {
		return nil, fmt.Errorf("rig %s: no serial port", addr)
	}
	port, err := openSerial(path, baud)
	if err != nil {
		return nil, fmt.Errorf("rig %s: %w", addr, err)
	}
	return NewRig(port), nil
}
