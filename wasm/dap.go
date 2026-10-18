/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package wasm

// This file is the debugger for a host that passes whole messages rather
// than a byte stream: a web page, where the module runs in a Web Worker
// and the page posts it Debug Adapter Protocol requests. Behind it is the
// same adapter `beedance -dap` serves (package dap), framed and unframed
// here, so the page and VS Code debug alike.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
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
	in   *io.PipeWriter
	mu   sync.Mutex
	q    [][]byte
	wake chan struct{}
	done chan struct{}
	once sync.Once
}

// NewDAP starts a debug adapter. onMessage is given each response and
// event, in order, from a goroutine of the adapter's, until the session
// ends (Close, or a disconnect request). yieldEvery and yield are the
// adapter's dap.Server.YieldEvery and Yield: for a web page, DAPYield and
// a function that waits for a macrotask; 0 and nil for a host with
// threads.
func NewDAP(onMessage func(msg []byte), yieldEvery time.Duration, yield func()) *DAP {
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	d := &DAP{in: cw, wake: make(chan struct{}, 1), done: make(chan struct{})}
	srv := dap.NewServer(sr, sw)
	srv.YieldEvery, srv.Yield = yieldEvery, yield
	go func() {
		_ = srv.Serve(context.Background())
		sw.Close()
	}()
	go func() { // the adapter's output, unframed
		r := bufio.NewReader(cr)
		for {
			body, err := readFrame(r)
			if err != nil {
				cr.Close()
				d.Close()
				return
			}
			onMessage(body)
		}
	}()
	go d.pump()
	return d
}

// Send queues a request for the adapter.
func (d *DAP) Send(msg []byte) {
	d.mu.Lock()
	d.q = append(d.q, append([]byte(nil), msg...))
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Close ends the session: the adapter sees its input end.
func (d *DAP) Close() {
	d.once.Do(func() { close(d.done) })
}

// pump writes the queued requests to the adapter, framed, one at a time:
// the pipe blocks until the adapter has read each.
func (d *DAP) pump() {
	defer d.in.Close()
	for {
		d.mu.Lock()
		q := d.q
		d.q = nil
		d.mu.Unlock()
		for _, m := range q {
			if _, err := fmt.Fprintf(d.in, "Content-Length: %d\r\n\r\n%s", len(m), m); err != nil {
				return
			}
		}
		select {
		case <-d.wake:
		case <-d.done:
			return
		}
	}
}

// readFrame reads one framed message's body.
func readFrame(r *bufio.Reader) ([]byte, error) {
	h, err := textproto.NewReader(r).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(h.Get("Content-Length")))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("dap: bad Content-Length %q", h.Get("Content-Length"))
	}
	body := make([]byte, n)
	_, err = io.ReadFull(r, body)
	return body, err
}
