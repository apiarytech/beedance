//go:build !js

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package main

import (
	"errors"
	"io"
	"syscall"
	"time"
)

// retryWriter writes all of p to w, waiting while w is not ready. A WASI
// runtime may give the module a non-blocking pipe as stdout (Node.js does),
// whose write fails with EAGAIN once the pipe is full: past 64 KB, a reply
// would otherwise be cut short.
type retryWriter struct{ w io.Writer }

func (r retryWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n, err := r.w.Write(p)
		written += n
		p = p[n:]
		if errors.Is(err, syscall.EAGAIN) {
			time.Sleep(time.Millisecond)
			continue
		}
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
