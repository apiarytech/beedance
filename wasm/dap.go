/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

// This file is the debugger for a host that passes whole messages rather
// than a byte stream: a web page, where the module runs in a Web Worker
// and the page posts it Debug Adapter Protocol requests. It is
// dap.Channel, with the yield a single-threaded host needs.

import (
	"time"

	"github.com/apiarytech/beedance/dap"
)

// DAPYield is how long a program left to run as fast as it goes keeps the
// processor before it lets a single-threaded host in: about a frame.
const DAPYield = 16 * time.Millisecond

// DAP is a debug adapter session that takes and gives whole messages: each
// a Debug Adapter Protocol request, response or event as JSON, without the
// Content-Length header. Send never blocks, so a host may call it from a
// callback it must return from (GOOS=js).
type DAP struct {
	*dap.Channel
}

// NewDAP starts a debug adapter. onMessage is given each response and
// event, in order, from a goroutine of the adapter's, until the session
// ends (Close, or a disconnect request). yieldEvery and yield are the
// adapter's dap.Server.YieldEvery and Yield: for a web page, DAPYield and
// a function that waits for a macrotask; 0 and nil for a host with
// threads.
func NewDAP(onMessage func(msg []byte), yieldEvery time.Duration, yield func()) *DAP {
	ch := dap.NewChannel(func(msg []byte) {
		if msg != nil {
			onMessage(msg)
		}
	}, func(srv *dap.Server) { srv.YieldEvery, srv.Yield = yieldEvery, yield })
	return &DAP{Channel: ch}
}
