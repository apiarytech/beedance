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

// The go engine: the program transpiled to Go with the debugger's hooks
// (transpiler.Options.Debug), built with the Go toolchain in a temporary
// module on royaljelly, as package sil builds the tests and the firmware
// is built, and run in a process of its own. The engine speaks to it on
// its pipes, a JSON object a line, one reply a request
// (engine_go_runtime.go is the process's side):
//
//	→ {"cmd":"scan","t_ms":10,"io":{"%IX0.0":"TRUE"},"stop":[12],"all":false}
//	← {"at":{"pou":"Main","row":12,"col":3,"depth":1}}   stopped before a statement
//	→ {"cmd":"go","stop":[12],"all":false}
//	← {"end":true,"io":{"%QX0.0":{"type":"BOOL","text":"TRUE"}}}
//	← {"err":"...","frames":[...]}                       the scan failed
//	→ {"cmd":"frames"} {"cmd":"vars","frame":-1} {"cmd":"read","frame":-1,"path":"c.total"}
//	→ {"cmd":"members","path":"c"} {"cmd":"write","path":"n","value":"5"} {"cmd":"io"}
//
// A statement is reported when it is on a stop line (a breakpoint's) or
// when every statement is asked for (a step, a pause): the session says
// which before each run (hints), so a free run costs no round trips but
// its breakpoints'. Values cross the pipe as the source writes them.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/transpiler"
)

// GoBuild configures the go engine (EngineGo): how the transpiled program
// is built and run.
type GoBuild struct {
	// Replace points a module the transpiled program imports at a local
	// directory, as a go.mod replace directive: {"github.com/apiarytech/
	// royaljelly": "../royaljelly"}; for developing the libraries.
	Replace map[string]string
	// Timeout bounds a scan's wall-clock time, so a runaway loop fails
	// the scan; default 10 s.
	Timeout time.Duration
	// Keep leaves the module's directory in place after Close, for a look
	// at the generated code; its path is Session.GoModule.
	Keep bool
}

// ErrNoGoToolchain is returned by Load on the go engine when go is not in
// PATH.
var ErrNoGoToolchain = errors.New("vmdebug: the go engine needs the Go toolchain: go not found in PATH")

// hinter is an engine that is told, before a run, which statements the
// hook wants: the breakpoints' rows, or every statement.
type hinter interface {
	hints(rows []int, every bool)
}

// goEngine is a PROGRAM transpiled, built and running in its own process.
type goEngine struct {
	name    string
	dir     string
	keep    bool
	decls   []Variable
	rows    map[int]bool
	first   int
	timeout time.Duration

	cmd        *exec.Cmd
	stdin      *bufio.Writer
	closeStdin func() error
	out        *json.Decoder
	stderr     bytes.Buffer
	dead       error // why the process is gone
	clock      func() time.Duration
	elapsed    time.Duration

	io      map[string]object.Object // the I/O image's mirror
	ioTypes map[string]string
	midScan bool
	stops   []int
	every   bool
	failed  []frame // where the last scan failed
}

// newGoEngine transpiles img's source with the hooks, builds it and starts
// it.
func newGoEngine(img *image, o Options, clock func() time.Duration) (eng *goEngine, err error) {
	goTool, lookErr := exec.LookPath("go")
	if lookErr != nil {
		return nil, ErrNoGoToolchain
	}
	program, errs := parseSource(img.source)
	if len(errs) > 0 {
		return nil, fmt.Errorf("parse: %s", strings.Join(errs, "; "))
	}
	code, err := transpiler.GoFileWith(program, transpiler.Options{Package: "dbgprog", HostBinding: true, Debug: true, AllowInputWrites: o.AllowInputWrites})
	if err != nil {
		return nil, fmt.Errorf("transpile: %w", err)
	}
	dir, err := os.MkdirTemp("", "beedance-debug-")
	if err != nil {
		return nil, err
	}
	e := &goEngine{name: img.name, dir: dir, keep: o.GoBuild.Keep, timeout: o.GoBuild.Timeout,
		io: map[string]object.Object{}, ioTypes: map[string]string{}}
	if e.timeout <= 0 {
		e.timeout = 10 * time.Second
	}
	defer func() {
		if err != nil {
			e.Close()
		}
	}()
	hasClock := bytes.Contains(code, []byte("var plcClock"))
	files := map[string]string{
		"go.mod":           goModule(string(code), o.GoBuild.Replace),
		"main.go":          goMainSource,
		"dbgprog/prog.go":  string(code),
		"dbgprog/serve.go": goRuntime(img.name, hasClock),
	}
	if err := os.MkdirAll(filepath.Join(dir, "dbgprog"), 0o755); err != nil {
		return nil, err
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(src), 0o644); err != nil {
			return nil, err
		}
	}
	bin := filepath.Join(dir, "dbgrun")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	goCmd := func(args ...string) error {
		cmd := exec.Command(goTool, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go %s: %v\n%s", args[0], err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := goCmd("mod", "tidy"); err != nil {
		return nil, err
	}
	if err := goCmd("build", "-o", bin, "."); err != nil {
		return nil, err
	}
	e.cmd = exec.Command(bin)
	stdin, err := e.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	e.cmd.Stderr = &e.stderr
	if err := e.cmd.Start(); err != nil {
		return nil, err
	}
	e.stdin = bufio.NewWriter(stdin)
	e.closeStdin = stdin.Close
	e.clock = clock
	e.out = json.NewDecoder(bufio.NewReader(stdout))
	e.decls, _ = declVariables(img.decl)
	e.rows, e.first = sourceRows(img)
	if err := e.refreshIO(); err != nil {
		return nil, err
	}
	return e, nil
}

// goModule is the module's go.mod: royaljelly, and beebread when the code
// uses OSCAT, at package sil's versions, or replaced.
func goModule(code string, replace map[string]string) string {
	var b strings.Builder
	b.WriteString("module beedance.debug/run\n\ngo " + sil.GoDirective + "\n\n")
	b.WriteString("require github.com/apiarytech/royaljelly " + sil.RoyaljellyVersion + "\n")
	if strings.Contains(code, "github.com/apiarytech/beebread") {
		b.WriteString("require github.com/apiarytech/beebread " + sil.BeebreadVersion + "\n")
	}
	mods := make([]string, 0, len(replace))
	for m := range replace {
		mods = append(mods, m)
	}
	sort.Strings(mods)
	for _, m := range mods {
		dir, err := filepath.Abs(replace[m])
		if err != nil {
			dir = replace[m]
		}
		fmt.Fprintf(&b, "replace %s => %s\n", m, filepath.ToSlash(dir))
	}
	return b.String()
}

// GoModule is the directory of the go engine's module (GoBuild.Keep), or
// empty on another engine.
func (s *Session) GoModule() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.img.eng.(*goEngine); ok {
		return e.dir
	}
	return ""
}

// request sends req and reads the reply, within timeout.
func (e *goEngine) request(req map[string]any, timeout time.Duration) (map[string]any, error) {
	if e.dead != nil {
		return nil, e.dead
	}
	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := e.stdin.Write(append(line, '\n')); err != nil {
		return nil, e.died(err)
	}
	if err := e.stdin.Flush(); err != nil {
		return nil, e.died(err)
	}
	type result struct {
		reply map[string]any
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		var reply map[string]any
		err := e.out.Decode(&reply)
		ch <- result{reply, err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, e.died(r.err)
		}
		if msg, ok := r.reply["err"].(string); ok && msg != "" {
			return r.reply, errors.New(msg)
		}
		return r.reply, nil
	case <-ctx.Done():
		e.kill()
		return nil, e.died(fmt.Errorf("no reply after %v: a runaway loop?", timeout))
	}
}

// died notes that the process is gone, with what stderr said.
func (e *goEngine) died(err error) error {
	if e.dead == nil {
		msg := strings.TrimSpace(e.stderr.String())
		if msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		e.dead = fmt.Errorf("the program's process ended: %w", err)
	}
	return e.dead
}

func (e *goEngine) kill() {
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
	}
}

// Close ends the process and removes the module.
func (e *goEngine) Close() {
	if e.cmd != nil {
		if e.closeStdin != nil {
			e.closeStdin()
		}
		done := make(chan struct{})
		go func() {
			e.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			e.kill()
			<-done
		}
		e.cmd = nil
	}
	if e.dir != "" && !e.keep {
		os.RemoveAll(e.dir)
	}
}

func (e *goEngine) Engine() Engine               { return EngineGo }
func (e *goEngine) Variables() []Variable        { return e.decls }
func (e *goEngine) IO() map[string]object.Object { return e.io }
func (e *goEngine) Rows() map[int]bool           { return e.rows }
func (e *goEngine) FirstRow() int                { return e.first }
func (e *goEngine) Reset()                       {}

func (e *goEngine) hints(rows []int, every bool) { e.stops, e.every = rows, every }

// refreshIO reads the process image into the mirror.
func (e *goEngine) refreshIO() error {
	reply, err := e.request(map[string]any{"cmd": "io"}, e.timeout)
	if err != nil {
		return err
	}
	e.takeIO(reply["io"])
	return nil
}

// takeIO puts an io reply's values in the mirror.
func (e *goEngine) takeIO(img any) {
	m, _ := img.(map[string]any)
	for addr, v := range m {
		entry, _ := v.(map[string]any)
		typ, _ := entry["type"].(string)
		text, _ := entry["text"].(string)
		addr = strings.ToUpper(addr)
		e.ioTypes[addr] = typ
		if o, err := parseValue(typ, text); err == nil {
			e.io[addr] = o
		}
	}
}

// Run starts a scan with the mirror's image, or lets the stopped one go
// on, and reports to hook at each statement the process stops at.
func (e *goEngine) Run(hook func(site) bool, _ bool) error {
	var req map[string]any
	stop := make([]int, 0, len(e.stops))
	stop = append(stop, e.stops...)
	every := e.every && hook != nil
	if hook == nil {
		stop = nil
	}
	if !e.midScan {
		e.elapsed = e.clock()
		img := map[string]any{}
		for addr, o := range e.io {
			img[addr] = Format(o)
		}
		req = map[string]any{"cmd": "scan", "t_ms": float64(e.elapsed) / float64(time.Millisecond), "io": img, "stop": stop, "all": every}
	} else {
		req = map[string]any{"cmd": "go", "stop": stop, "all": every}
	}
	e.failed = nil
	for {
		reply, err := e.request(req, e.timeout)
		if err != nil {
			e.midScan = false
			if reply != nil {
				e.failed = goFrames(reply["frames"])
			}
			return err
		}
		if at, ok := reply["at"].(map[string]any); ok {
			e.midScan = true
			st := site{row: goNum(at["row"]), column: goNum(at["col"]), depth: goNum(at["depth"]), statement: true}
			if hook != nil && hook(st) {
				return errPaused
			}
			req = map[string]any{"cmd": "go", "stop": stop, "all": every}
			continue
		}
		e.midScan = false
		e.takeIO(reply["io"])
		return nil
	}
}

func goNum(v any) int {
	f, _ := v.(float64)
	return int(f)
}

// goFrames reads a frames reply.
func goFrames(v any) []frame {
	list, _ := v.([]any)
	out := make([]frame, 0, len(list))
	for _, f := range list {
		m, _ := f.(map[string]any)
		pou, _ := m["pou"].(string)
		out = append(out, frame{name: pou, row: goNum(m["row"]), column: goNum(m["col"]), depth: goNum(m["depth"])})
	}
	return out
}

func (e *goEngine) Frames() []frame {
	var frames []frame
	if !e.midScan && e.failed != nil {
		frames = e.failed
	} else {
		reply, err := e.request(map[string]any{"cmd": "frames"}, e.timeout)
		if err != nil {
			return nil
		}
		frames = goFrames(reply["frames"])
	}
	for i := range frames {
		if frames[i].depth <= 1 {
			continue
		}
		idx := i
		frames[i].locals = func() []Value { return e.values(idx, "") }
		frames[i].setLocal = func(name, value string) (object.Object, error) { return e.write(idx, name, value) }
	}
	return frames
}

// values are the variables of frame (-1: the program's), or the members
// of path in it.
func (e *goEngine) values(frame int, path string) []Value {
	req := map[string]any{"cmd": "vars", "frame": frame}
	if path != "" {
		req = map[string]any{"cmd": "members", "frame": frame, "path": path}
	}
	reply, err := e.request(req, e.timeout)
	if err != nil {
		return nil
	}
	list, _ := reply["vars"].([]any)
	out := make([]Value, 0, len(list))
	for _, v := range list {
		m, _ := v.(map[string]any)
		name, _ := m["name"].(string)
		typ, _ := m["type"].(string)
		addr, _ := m["address"].(string)
		full := name
		if path != "" && !strings.HasPrefix(name, "[") {
			full = path + "." + name
		} else if path != "" {
			full = path + name
		}
		out = append(out, Value{Name: name, Type: typ, Address: addr, Object: e.object(frame, full, m)})
	}
	return out
}

// object is a variable's value as the debugger holds it: an elementary
// value parsed from its text, else a remoteValue whose members are read
// on demand.
func (e *goEngine) object(frame int, path string, m map[string]any) object.Object {
	typ, _ := m["type"].(string)
	text, _ := m["text"].(string)
	members, _ := m["members"].(bool)
	if !members && text != "" {
		if o, err := parseValue(typ, text); err == nil {
			return o
		}
	}
	return &remoteValue{eng: e, frame: frame, path: path, typ: typ, text: text, structured: members}
}

// write writes value to path in frame.
func (e *goEngine) write(frame int, path, value string) (object.Object, error) {
	reply, err := e.request(map[string]any{"cmd": "write", "frame": frame, "path": path, "value": value}, e.timeout)
	if err != nil {
		return nil, err
	}
	m, _ := reply["var"].(map[string]any)
	return e.object(frame, path, m), nil
}

func (e *goEngine) Read(path []string) (object.Object, bool) {
	full := strings.Join(path, ".")
	reply, err := e.request(map[string]any{"cmd": "read", "frame": -1, "path": full}, e.timeout)
	if err != nil {
		return nil, false
	}
	m, _ := reply["var"].(map[string]any)
	return e.object(-1, full, m), true
}

func (e *goEngine) Write(path []string, o object.Object) error {
	_, err := e.write(-1, strings.Join(path, "."), Format(o))
	return err
}

// remoteValue is a value of a program in another process (the go engine):
// what it was when read, its members read on demand.
type remoteValue struct {
	eng        *goEngine
	frame      int
	path       string
	typ        string
	text       string
	structured bool
}

// Type is the object's type.
func (r *remoteValue) Type() object.ObjectType { return object.ObjectType("REMOTE") }

// Inspect is the value's text, or its type for a structured one.
func (r *remoteValue) Inspect() string {
	if r.structured {
		return r.typ
	}
	return r.text
}

func (r *remoteValue) members() []Value {
	if !r.structured {
		return nil
	}
	return r.eng.values(r.frame, r.path)
}

func (r *remoteValue) set(name, value string) (object.Object, error) {
	path := r.path + "." + name
	if strings.HasPrefix(name, "[") {
		path = r.path + name
	}
	return r.eng.write(r.frame, path, value)
}
