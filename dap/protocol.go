/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
)

// The Debug Adapter Protocol's messages: a header, "Content-Length: N",
// a blank line, and N bytes of JSON.

// message is any message, as read.
type message struct {
	Seq     int    `json:"seq"`
	Type    string `json:"type"` // request, response or event
	Command string `json:"command,omitempty"`
	// Arguments are a request's.
	Arguments json.RawMessage `json:"arguments,omitempty"`

	// A response's.
	RequestSeq int             `json:"request_seq,omitempty"`
	Success    bool            `json:"success,omitempty"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
	// An event's.
	Event string `json:"event,omitempty"`
}

// response is a response sent.
type response struct {
	Seq        int    `json:"seq"`
	Type       string `json:"type"`
	RequestSeq int    `json:"request_seq"`
	Success    bool   `json:"success"`
	Command    string `json:"command"`
	Message    string `json:"message,omitempty"`
	Body       any    `json:"body,omitempty"`
}

// event is an event sent.
type event struct {
	Seq   int    `json:"seq"`
	Type  string `json:"type"`
	Event string `json:"event"`
	Body  any    `json:"body,omitempty"`
}

// readMessage reads the next message from r.
func readMessage(r *bufio.Reader) (*message, error) {
	tp := textproto.NewReader(r)
	header, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF && len(header) == 0 {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("dap: header: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(header.Get("Content-Length")))
	if err != nil || n < 0 || n > 64<<20 {
		return nil, fmt.Errorf("dap: bad Content-Length %q", header.Get("Content-Length"))
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("dap: body: %w", err)
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("dap: %w", err)
	}
	return &m, nil
}

// writeMessage writes v as a message to w.
func writeMessage(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// The bodies and arguments used, as the protocol names them.

type capabilities struct {
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest"`
	SupportsConditionalBreakpoints   bool `json:"supportsConditionalBreakpoints"`
	SupportsEvaluateForHovers        bool `json:"supportsEvaluateForHovers"`
	SupportsSetVariable              bool `json:"supportsSetVariable"`
	SupportsTerminateRequest         bool `json:"supportsTerminateRequest"`
	SupportsDisassembleRequest       bool `json:"supportsDisassembleRequest"`
	SupportsSteppingGranularity      bool `json:"supportsSteppingGranularity"`
	SupportsInstructionBreakpoints   bool `json:"supportsInstructionBreakpoints"`
}

type launchArgs struct {
	// Program is the path of the .st file. With Source, it only names the
	// program (in messages, and the breakpoints' source path).
	Program string `json:"program"`
	// Source is the program's text, for a host with no files (a web
	// page); when set, Program is not read.
	Source string `json:"source"`
	// ProgramName is the PROGRAM to run; empty: the first that is not a
	// test.
	ProgramName string `json:"programName"`
	StopOnEntry bool   `json:"stopOnEntry"`
	NoDebug     bool   `json:"noDebug"`
	// Interval is the scan time, e.g. "10ms"; default 10 ms.
	Interval string `json:"interval"`
	// Realtime runs scans Interval apart on the wall clock; default true.
	Realtime         *bool `json:"realtime"`
	AllowInputWrites bool  `json:"allowInputWrites"`
	// ScanEvents, if positive, emits a beedance/scan event (the watched
	// values and the I/O image, as beedance/live) at most this many
	// milliseconds apart while the program runs.
	ScanEvents int `json:"scanEvents"`
	// Speed, if set, is how fast simulated time runs against the wall
	// clock while the program runs: 1 real time, 0 as fast as it goes. It
	// overrides Realtime.
	Speed *float64 `json:"speed"`
}

type source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

type sourceBreakpoint struct {
	Line      int    `json:"line"`
	Condition string `json:"condition,omitempty"`
}

type setBreakpointsArgs struct {
	Source      source             `json:"source"`
	Breakpoints []sourceBreakpoint `json:"breakpoints"`
}

type breakpoint struct {
	ID       int    `json:"id"`
	Verified bool   `json:"verified"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
	Source   source `json:"source"`
}

type thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type stackFrame struct {
	ID                          int     `json:"id"`
	Name                        string  `json:"name"`
	Source                      *source `json:"source,omitempty"`
	Line                        int     `json:"line"`
	Column                      int     `json:"column"`
	InstructionPointerReference string  `json:"instructionPointerReference,omitempty"`
}

type scope struct {
	Name               string `json:"name"`
	PresentationHint   string `json:"presentationHint,omitempty"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive"`
}

type variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	EvaluateName       string `json:"evaluateName,omitempty"`
	VariablesReference int    `json:"variablesReference"`
}

type frameArgs struct {
	FrameID int `json:"frameId"`
}

type variablesArgs struct {
	VariablesReference int `json:"variablesReference"`
}

type setVariableArgs struct {
	VariablesReference int    `json:"variablesReference"`
	Name               string `json:"name"`
	Value              string `json:"value"`
}

type evaluateArgs struct {
	Expression string `json:"expression"`
	FrameID    int    `json:"frameId"`
	Context    string `json:"context"`
}

type stepArgs struct {
	Granularity string `json:"granularity"`
}

type disassembleArgs struct {
	MemoryReference   string `json:"memoryReference"`
	InstructionOffset int    `json:"instructionOffset"`
	InstructionCount  int    `json:"instructionCount"`
}

type disassembledInstruction struct {
	Address     string  `json:"address"`
	Instruction string  `json:"instruction"`
	Symbol      string  `json:"symbol,omitempty"`
	Location    *source `json:"location,omitempty"`
	Line        int     `json:"line,omitempty"`
}

// onlineChangeArgs are beedance/onlineChange's: Source is the new text of
// the program; empty re-reads its file.
type onlineChangeArgs struct {
	Source string `json:"source"`
}

// speedArgs are beedance/speed's: as launchArgs.Speed.
type speedArgs struct {
	Factor float64 `json:"factor"`
}

// traceArgs are beedance/trace's: Signals, when given, are what to record
// from now on (empty stops); Depth the scans kept; Scans how many of the
// last scans to return (0: all kept).
type traceArgs struct {
	Signals *[]string `json:"signals"`
	Depth   int       `json:"depth"`
	Scans   int       `json:"scans"`
}

// setInputsArgs are beedance/setInputs': the values of one scan's inputs,
// by address or name.
type setInputsArgs struct {
	Values map[string]any `json:"values"`
}

type forceArgs struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
