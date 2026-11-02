//go:build js

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

import (
	"bytes"
	"errors"
	"io"
	"syscall/js"

	"github.com/apiarytech/beedance/vmdebug"
)

// GoStart is the go engine's vmdebug.GoBuild.Start in a web page, on the
// page's function start:
//
//	start({source, program, allowInputWrites}, onLine)
//	    → Promise<{send(line), close()}>
//
// The page builds the program (a build service, for GOOS=js) and runs it
// (a Web Worker); send gives it a request line, onLine(line) is called
// with each reply, and close ends it. A rejected promise fails the launch
// with its message.
func GoStart(start js.Value) func(vmdebug.GoStart) (io.ReadWriteCloser, error) {
	return func(g vmdebug.GoStart) (io.ReadWriteCloser, error) {
		r, w := io.Pipe()
		lines := make(chan string, 64)
		onLine := js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) == 1 {
				line := args[0].String()
				go func() { lines <- line }()
			}
			return nil
		})
		go func() {
			for line := range lines {
				if _, err := w.Write([]byte(line + "\n")); err != nil {
					return
				}
			}
		}()
		req := js.ValueOf(map[string]any{"source": g.Source, "program": g.Program,
			"allowInputWrites": g.AllowInputWrites})
		handle, err := await(start.Invoke(req, onLine))
		if err != nil {
			onLine.Release()
			w.Close()
			return nil, err
		}
		return &jsConn{handle: handle, r: r, w: w, onLine: onLine}, nil
	}
}

// await waits for promise p (or a plain value), from a goroutine that is
// not the JS event loop's.
func await(p js.Value) (js.Value, error) {
	if p.Type() != js.TypeObject || p.Get("then").Type() != js.TypeFunction {
		return p, nil
	}
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	ok := js.FuncOf(func(this js.Value, args []js.Value) any {
		v := js.Undefined()
		if len(args) > 0 {
			v = args[0]
		}
		ch <- result{v: v}
		return nil
	})
	fail := js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := "the go engine's program did not start"
		if len(args) > 0 {
			if m := args[0].Get("message"); m.Type() == js.TypeString {
				msg = m.String()
			} else {
				msg = args[0].String()
			}
		}
		ch <- result{err: errors.New(msg)}
		return nil
	})
	defer ok.Release()
	defer fail.Release()
	p.Call("then", ok, fail)
	res := <-ch
	return res.v, res.err
}

// jsConn is the page's running program as a connection: lines written are
// sent, lines it replies are read.
type jsConn struct {
	handle js.Value
	r      *io.PipeReader
	w      *io.PipeWriter
	onLine js.Func
	buf    []byte
	closed bool
}

func (c *jsConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func (c *jsConn) Write(p []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	c.buf = append(c.buf, p...)
	for {
		i := bytes.IndexByte(c.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		c.handle.Call("send", string(c.buf[:i]))
		c.buf = c.buf[i+1:]
	}
}

func (c *jsConn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	c.handle.Call("close")
	c.w.Close()
	c.onLine.Release()
	return nil
}
