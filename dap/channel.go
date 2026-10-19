/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// Channel is a debug adapter session that takes and gives whole
// messages: each a Debug Adapter Protocol request, response or event as
// JSON, without the Content-Length header. It is for a host that passes
// messages rather than a byte stream: a web page posting them to a Web
// Worker, a service relaying them over its RPCs. Behind it is the same
// Server `beedance -dap` serves, framed and unframed here. Send never
// blocks, so a host may call it from a callback it must return from.
type Channel struct {
	in   *io.PipeWriter
	mu   sync.Mutex
	q    [][]byte
	wake chan struct{}
	done chan struct{}
	once sync.Once
}

// NewChannel starts a debug adapter. onMessage is given each response and
// event, in order, from a goroutine of the adapter's, until the session
// ends (Close, or a disconnect request), and then once with nil. setup,
// if not nil, configures the Server before it serves: its YieldEvery and
// Yield for a single-threaded host, its Attach for a host with programs
// of its own.
func NewChannel(onMessage func(msg []byte), setup func(*Server)) *Channel {
	sr, cw := io.Pipe()
	cr, sw := io.Pipe()
	c := &Channel{in: cw, wake: make(chan struct{}, 1), done: make(chan struct{})}
	srv := NewServer(sr, sw)
	if setup != nil {
		setup(srv)
	}
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
				c.Close()
				onMessage(nil)
				return
			}
			onMessage(body)
		}
	}()
	go c.pump()
	return c
}

// Send queues a request for the adapter.
func (c *Channel) Send(msg []byte) {
	c.mu.Lock()
	c.q = append(c.q, append([]byte(nil), msg...))
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Close ends the session: the adapter sees its input end. It may be
// called more than once.
func (c *Channel) Close() {
	c.once.Do(func() { close(c.done) })
}

// Done is closed when the session has ended.
func (c *Channel) Done() <-chan struct{} { return c.done }

// pump writes the queued requests to the adapter, framed, one at a time:
// the pipe blocks until the adapter has read each.
func (c *Channel) pump() {
	defer c.in.Close()
	for {
		c.mu.Lock()
		q := c.q
		c.q = nil
		c.mu.Unlock()
		for _, m := range q {
			if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(m), m); err != nil {
				return
			}
		}
		select {
		case <-c.wake:
		case <-c.done:
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
