/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vmdebug

// The runtime the go engine puts beside the transpiled program, in its
// package: it sets dbgHook and speaks to the engine on stdin and stdout,
// a JSON object a line, one reply a request (engine_go.go has the
// protocol). A scan runs in a goroutine of its own, which parks in the
// statement hook while the program is stopped, as the evaluator engine's
// does.

import "strings"

// goRuntimeSource is serve.go of the built module; {{PROGRAM}} is the
// PROGRAM's name and {{CLOCK}} the line that ties the file's clock to the
// debugger's, if the file has one.
const goRuntimeSource = `package dbgprog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// dbgFrame is a POU the program is in.
type dbgFrame struct {
	pou      string
	vars     func() []dbgVar
	row, col int
}

// dbgServer runs the program for the debugger.
type dbgServer struct {
	logic   func(time.Time)
	vars    func() []dbgVar
	points  []dbgPoint
	enc     *json.Encoder
	elapsed time.Duration

	stack  []dbgFrame
	failed []dbgFrame
	stops  map[int]bool
	all    bool

	running bool // a scan's goroutine is alive
	paused  chan struct{}
	resume  chan struct{}
	done    chan error
}

// Serve runs the program for the debugger on r and w until r ends.
func Serve(r io.Reader, w io.Writer) {
	p := New{{PROGRAM}}()
	s := &dbgServer{logic: p.Logic, vars: p.dbgVars, points: dbgPoints(), enc: json.NewEncoder(w),
		stops: map[int]bool{}, paused: make(chan struct{}), resume: make(chan struct{}), done: make(chan error, 1)}
	{{CLOCK}}
	dbgHook = s
	dec := json.NewDecoder(bufio.NewReader(r))
	for {
		var req map[string]any
		if err := dec.Decode(&req); err != nil {
			return
		}
		s.enc.Encode(s.handle(req))
	}
}

func (s *dbgServer) handle(req map[string]any) map[string]any {
	switch req["cmd"] {
	case "scan":
		if s.running {
			return map[string]any{"err": "a scan is in progress"}
		}
		if ms, ok := req["t_ms"].(float64); ok {
			s.elapsed = time.Duration(ms * float64(time.Millisecond))
		}
		if img, ok := req["io"].(map[string]any); ok {
			for addr, v := range img {
				text, _ := v.(string)
				for _, pt := range s.points {
					if strings.EqualFold(pt.Address, addr) {
						pt.Set(text)
					}
				}
			}
		}
		s.setStops(req)
		s.stack, s.failed = s.stack[:0], nil
		s.running = true
		go s.scan()
		return s.wait()
	case "go":
		if !s.running {
			return map[string]any{"err": "no scan is in progress"}
		}
		s.setStops(req)
		s.resume <- struct{}{}
		return s.wait()
	case "frames":
		return map[string]any{"frames": s.frames()}
	case "vars":
		vars, err := s.scope(req)
		if err != nil {
			return map[string]any{"err": err.Error()}
		}
		return map[string]any{"vars": dbgList(vars)}
	case "read", "members":
		v, err := s.find(req)
		if err != nil {
			return map[string]any{"err": err.Error()}
		}
		if req["cmd"] == "read" {
			return map[string]any{"var": dbgDescribe(*v)}
		}
		if v.Members == nil {
			return map[string]any{"vars": []any{}}
		}
		return map[string]any{"vars": dbgList(v.Members())}
	case "write":
		v, err := s.find(req)
		if err != nil {
			return map[string]any{"err": err.Error()}
		}
		if v.Set == nil {
			return map[string]any{"err": fmt.Sprintf("%s cannot be written", v.Name)}
		}
		text, _ := req["value"].(string)
		if err := v.Set(text); err != nil {
			return map[string]any{"err": err.Error()}
		}
		return map[string]any{"ok": true, "var": dbgDescribe(*v)}
	case "io":
		return map[string]any{"io": s.image()}
	}
	return map[string]any{"err": fmt.Sprintf("unknown command %v", req["cmd"])}
}

// setStops reads which statements the hook reports: all, or those on the
// stop lines.
func (s *dbgServer) setStops(req map[string]any) {
	s.all, _ = req["all"].(bool)
	s.stops = map[int]bool{}
	if rows, ok := req["stop"].([]any); ok {
		for _, r := range rows {
			if f, ok := r.(float64); ok {
				s.stops[int(f)] = true
			}
		}
	}
}

// scan runs one scan of the program in its goroutine.
func (s *dbgServer) scan() {
	var err error
	func() {
		defer func() {
			if x := recover(); x != nil {
				err = fmt.Errorf("%v", x)
				if s.failed == nil {
					s.failed = append([]dbgFrame(nil), s.stack...)
				}
			}
		}()
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		s.logic(start.Add(s.elapsed))
	}()
	s.done <- err
}

// wait is the reply to a scan or go request: where the program stopped,
// or the scan's end, or its error.
func (s *dbgServer) wait() map[string]any {
	select {
	case <-s.paused:
		f := s.stack[len(s.stack)-1]
		return map[string]any{"at": map[string]any{"pou": f.pou, "row": f.row, "col": f.col, "depth": len(s.stack)}}
	case err := <-s.done:
		s.running = false
		if err != nil {
			return map[string]any{"err": err.Error(), "frames": s.frames()}
		}
		return map[string]any{"end": true, "io": s.image()}
	}
}

// At is the statement hook: it reports the statement if asked, and parks
// the scan until the debugger lets it go on.
func (s *dbgServer) At(row, col int) {
	if len(s.stack) == 0 {
		return
	}
	f := &s.stack[len(s.stack)-1]
	f.row, f.col = row, col
	if s.all || s.stops[row] {
		s.paused <- struct{}{}
		<-s.resume
	}
}

func (s *dbgServer) Enter(pou string, vars func() []dbgVar) {
	s.stack = append(s.stack, dbgFrame{pou: pou, vars: vars})
}

func (s *dbgServer) Leave() {
	if len(s.stack) > 0 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

// frames are the POUs the program is in, innermost first; after a failed
// scan, where it failed.
func (s *dbgServer) frames() []any {
	stack := s.stack
	if !s.running && s.failed != nil {
		stack = s.failed
	}
	out := []any{}
	for i := len(stack) - 1; i >= 0; i-- {
		f := stack[i]
		out = append(out, map[string]any{"pou": f.pou, "row": f.row, "col": f.col, "depth": i + 1})
	}
	return out
}

// scope is the variables a request names: the program's (frame -1), or a
// frame's (an index of frames).
func (s *dbgServer) scope(req map[string]any) ([]dbgVar, error) {
	frame := -1
	if f, ok := req["frame"].(float64); ok {
		frame = int(f)
	}
	if frame < 0 {
		return s.vars(), nil
	}
	stack := s.stack
	if !s.running && s.failed != nil {
		stack = s.failed
	}
	i := len(stack) - 1 - frame
	if i < 0 || i >= len(stack) {
		return nil, fmt.Errorf("no frame %d", frame)
	}
	if stack[i].vars == nil {
		return nil, nil
	}
	return stack[i].vars(), nil
}

// find is the variable a request's path names in its scope.
func (s *dbgServer) find(req map[string]any) (*dbgVar, error) {
	vars, err := s.scope(req)
	if err != nil {
		return nil, err
	}
	path, _ := req["path"].(string)
	parts := strings.Split(path, ".")
	var found *dbgVar
	for i, name := range parts {
		found = nil
		for j := range vars {
			if strings.EqualFold(vars[j].Name, name) {
				found = &vars[j]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("no %s", strings.Join(parts[:i+1], "."))
		}
		if i < len(parts)-1 {
			if found.Members == nil {
				return nil, fmt.Errorf("%s has no members", strings.Join(parts[:i+1], "."))
			}
			vars = found.Members()
		}
	}
	return found, nil
}

// dbgDescribe is a variable as the debugger reads it.
func dbgDescribe(v dbgVar) map[string]any {
	out := map[string]any{"name": v.Name, "type": v.Type, "address": v.Address, "members": v.Members != nil}
	if v.Text != nil {
		out["text"] = v.Text()
	}
	return out
}

func dbgList(vars []dbgVar) []any {
	out := make([]any, 0, len(vars))
	for _, v := range vars {
		out = append(out, dbgDescribe(v))
	}
	return out
}

// image is the located variables' values, as text, by address.
func (s *dbgServer) image() map[string]any {
	out := map[string]any{}
	for _, pt := range s.points {
		out[pt.Address] = map[string]any{"type": pt.Type, "text": pt.Text()}
	}
	return out
}

var _ = os.Stdin
`

// goMainSource is main.go of the built module.
const goMainSource = `package main

import (
	"os"

	"beedance.debug/run/dbgprog"
)

func main() {
	dbgprog.Serve(os.Stdin, os.Stdout)
}
`

// goRuntime is the runtime for the PROGRAM name, with the clock tied to
// the debugger's if the transpiled code has one.
func goRuntime(name string, hasClock bool) string {
	clock := ""
	if hasClock {
		clock = "plcClock = func() time.Duration { return s.elapsed }"
	}
	return strings.NewReplacer("{{PROGRAM}}", name, "{{CLOCK}}", clock).Replace(goRuntimeSource)
}
