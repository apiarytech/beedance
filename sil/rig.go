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

// This file is the rig protocol: an IO over a byte stream (TCP, or a
// serial line through a TCP bridge), so a test rig in any language, on a
// Raspberry Pi, a microcontroller or a PC with an I/O card, puts real I/O
// in the loop. It is JSON, one object per line, a request and its reply:
//
//	→ {"op":"begin","test":"TEST_Fill","engine":"vm","points":[{"address":"%IX0.0","type":"BOOL"}]}
//	← {}
//	→ {"op":"read","t_ms":10}
//	← {"inputs":{"%IX0.0":true,"%IW1":512}}
//	→ {"op":"write","t_ms":10,"outputs":{"%QX0.0":true}}
//	← {}
//	→ {"op":"end"}
//	← {}
//
// A reply with "error" fails the run. Values are JSON: true and false for
// BOOL, numbers for the integer, REAL and bit string types, strings for
// STRING and for TIME ("T#1.5s"). t_ms is the run's time in milliseconds.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// rigMessage is a request or a reply of the rig protocol.
type rigMessage struct {
	Op      string         `json:"op,omitempty"`
	Test    string         `json:"test,omitempty"`
	Engine  Engine         `json:"engine,omitempty"`
	Points  []Point        `json:"points,omitempty"`
	TimeMS  *float64       `json:"t_ms,omitempty"`
	Inputs  map[string]any `json:"inputs,omitempty"`
	Outputs map[string]any `json:"outputs,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// MaxRigLine bounds one line of the rig protocol.
const MaxRigLine = 1 << 20

// Rig is an IO served by a rig over the rig protocol.
type Rig struct {
	mu    sync.Mutex
	conn  io.ReadWriteCloser
	in    *bufio.Reader
	types map[string]string // the points' types, by upper-case address
}

var _ IO = (*Rig)(nil)

// DialRig connects to a rig at addr, host:port, over TCP; "tcp://" may
// prefix it.
func DialRig(ctx context.Context, addr string) (*Rig, error) {
	addr = strings.TrimPrefix(addr, "tcp://")
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("rig: %w", err)
	}
	return NewRig(conn), nil
}

// NewRig returns an IO that speaks the rig protocol over conn, e.g. a
// serial port.
func NewRig(conn io.ReadWriteCloser) *Rig {
	return &Rig{conn: conn, in: bufio.NewReaderSize(conn, 64<<10), types: map[string]string{}}
}

// Close closes the connection.
func (r *Rig) Close() error { return r.conn.Close() }

// Begin implements IO.
func (r *Rig) Begin(ctx context.Context, test string, engine Engine, points []Point) error {
	r.mu.Lock()
	r.types = map[string]string{}
	for _, p := range points {
		r.types[strings.ToUpper(p.Address)] = p.Type
	}
	r.mu.Unlock()
	_, err := r.call(ctx, rigMessage{Op: "begin", Test: test, Engine: engine, Points: points})
	return err
}

// Read implements IO.
func (r *Rig) Read(ctx context.Context, t time.Duration) (map[string]object.Object, error) {
	rep, err := r.call(ctx, rigMessage{Op: "read", TimeMS: ms(t)})
	if err != nil {
		return nil, err
	}
	out := map[string]object.Object{}
	for addr, v := range rep.Inputs {
		o, err := ToObject(r.typeOf(addr), v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", addr, err)
		}
		out[addr] = o
	}
	return out, nil
}

// Write implements IO.
func (r *Rig) Write(ctx context.Context, t time.Duration, outputs map[string]object.Object) error {
	vals, err := jsonValues(outputs)
	if err != nil {
		return err
	}
	_, err = r.call(ctx, rigMessage{Op: "write", TimeMS: ms(t), Outputs: vals})
	return err
}

// End implements IO.
func (r *Rig) End(ctx context.Context) error {
	_, err := r.call(ctx, rigMessage{Op: "end"})
	return err
}

func (r *Rig) typeOf(addr string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.types[strings.ToUpper(addr)]; ok {
		return t
	}
	t, _ := ast.DirectType(addr)
	return t
}

// call sends req and reads its reply, within ctx's deadline when the
// connection has deadlines.
func (r *Rig) call(ctx context.Context, req rigMessage) (rigMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dl, ok := r.conn.(interface{ SetDeadline(time.Time) error }); ok {
		deadline, _ := ctx.Deadline()
		dl.SetDeadline(deadline)
		stop := context.AfterFunc(ctx, func() { dl.SetDeadline(time.Unix(1, 0)) })
		defer stop()
	}
	if err := writeLine(r.conn, req); err != nil {
		return rigMessage{}, fmt.Errorf("rig %s: %w", req.Op, err)
	}
	var rep rigMessage
	if err := readLine(r.in, &rep); err != nil {
		return rigMessage{}, fmt.Errorf("rig %s: %w", req.Op, err)
	}
	if rep.Error != "" {
		return rep, fmt.Errorf("rig %s: %s", req.Op, rep.Error)
	}
	return rep, nil
}

// ServeRig serves the rig protocol on conn with h, the rig's own I/O, until
// the client closes conn or ctx is done. A test rig is a program around it:
//
//	ln, _ := net.Listen("tcp", ":5000")
//	for {
//		conn, _ := ln.Accept()
//		sil.ServeRig(ctx, conn, myIO) // myIO reads and drives the pins
//	}
//
// If the client goes away within a run, h.End is called, so the rig can
// make the plant safe.
func ServeRig(ctx context.Context, conn io.ReadWriter, h IO) error {
	in := bufio.NewReaderSize(conn, 64<<10)
	types := map[string]string{}
	running := false
	defer func() {
		if running {
			h.End(context.WithoutCancel(ctx))
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var req rigMessage
		if err := readLine(in, &req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var rep rigMessage
		var err error
		t := time.Duration(0)
		if req.TimeMS != nil {
			t = time.Duration(*req.TimeMS * float64(time.Millisecond))
		}
		switch req.Op {
		case "begin":
			types = map[string]string{}
			for _, p := range req.Points {
				types[strings.ToUpper(p.Address)] = p.Type
			}
			if err = h.Begin(ctx, req.Test, req.Engine, req.Points); err == nil {
				running = true
			}
		case "read":
			var in map[string]object.Object
			if in, err = h.Read(ctx, t); err == nil {
				rep.Inputs, err = jsonValues(in)
			}
		case "write":
			out := map[string]object.Object{}
			for addr, v := range req.Outputs {
				typ, ok := types[strings.ToUpper(addr)]
				if !ok {
					typ, _ = ast.DirectType(addr)
				}
				var o object.Object
				if o, err = ToObject(typ, v); err != nil {
					err = fmt.Errorf("%s: %w", addr, err)
					break
				}
				out[addr] = o
			}
			if err == nil {
				err = h.Write(ctx, t, out)
			}
		case "end":
			running = false
			err = h.End(ctx)
		default:
			err = fmt.Errorf("unknown op %q", req.Op)
		}
		if err != nil {
			rep = rigMessage{Error: err.Error()}
		}
		if err := writeLine(conn, rep); err != nil {
			return err
		}
	}
}

// jsonValues converts objects to the protocol's JSON values.
func jsonValues(objs map[string]object.Object) (map[string]any, error) {
	out := make(map[string]any, len(objs))
	for addr, o := range objs {
		v, err := FromObject(o)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", addr, err)
		}
		if d, ok := v.(time.Duration); ok {
			v = (&object.Time{Value: d}).Inspect()
		}
		out[addr] = v
	}
	return out, nil
}

func ms(t time.Duration) *float64 {
	f := float64(t) / float64(time.Millisecond)
	return &f
}

func writeLine(w io.Writer, m rigMessage) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func readLine(r *bufio.Reader, m *rigMessage) error {
	var line []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		if err != nil {
			return err
		}
		line = append(line, chunk...)
		if len(line) > MaxRigLine {
			return fmt.Errorf("line longer than %d bytes", MaxRigLine)
		}
		if !isPrefix {
			break
		}
	}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	dec.UseNumber()
	return dec.Decode(m)
}
