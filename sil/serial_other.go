/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

//go:build !linux && !darwin && !windows

package sil

import (
	"errors"
	"io"
)

func openSerial(path string, baud int) (io.ReadWriteCloser, error) {
	return nil, errors.New("serial rigs are supported on Linux, macOS and Windows; use a serial-to-TCP bridge")
}
