//go:build !lite

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

// This file is the full build's ops: the tests (package sil, which brings
// the evaluator) and the transpiler. A build with -tags lite leaves them
// out (lite.go): a page that only edits, checks and debugs loads a smaller
// module.

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/transpiler"
)

// buildOps are the ops of this build besides the common ones.
var buildOps = []string{"test", "transpile"}

// parseErrors are the parse errors behind err, if it is sil's.
func parseErrors(err error) ([]string, bool) {
	var pe *sil.ParseError
	if errors.As(err, &pe) {
		return pe.Errors, true
	}
	return nil, false
}

type transpileReply struct {
	Go string `json:"go"`
}

// transpileOp is the transpile op: the source as a Go file.
func transpileOp(req Request) (any, error) {
	program, err := parse(req.Source)
	if err != nil {
		return nil, err
	}
	src, err := transpiler.GoFile(program)
	if err != nil {
		return nil, err
	}
	return transpileReply{Go: string(src)}, nil
}

type testReply struct {
	Passed  bool         `json:"passed"`
	Failed  int          `json:"failed"`
	Results []testResult `json:"results"`
}

type testResult struct {
	Name      string         `json:"name"`
	Passed    bool           `json:"passed"`
	Mismatch  string         `json:"mismatch,omitempty"`
	Variables []variable     `json:"variables"`
	Engines   []engineResult `json:"engines"`
}

type engineResult struct {
	Engine    string     `json:"engine"`
	Passed    bool       `json:"passed"`
	Done      bool       `json:"done"`
	Failures  int        `json:"failures"`
	Message   string     `json:"message,omitempty"`
	Error     string     `json:"error,omitempty"`
	Problems  []string   `json:"problems,omitempty"`
	ScanCount int        `json:"scanCount"`
	Scans     []sil.Scan `json:"scans,omitempty"`
}

// test runs the TEST_ programs of the source with package sil. The go and
// wasm engines build the tests with the Go toolchain, which a WebAssembly
// module has not got.
func testOp(req Request) (any, error) {
	engines := []sil.Engine{sil.Evaluator, sil.VM}
	if len(req.Engines) > 0 {
		var err error
		if engines, err = sil.ParseEngines(strings.Join(req.Engines, ",")); err != nil {
			return nil, err
		}
	}
	for _, e := range engines {
		if (e == sil.Go || e == sil.Wasm) && runtime.GOARCH == "wasm" {
			return nil, fmt.Errorf("the %s engine needs the Go toolchain, which WebAssembly has not got: use eval and vm", e)
		}
	}
	results, err := sil.Run(context.Background(), req.Source, sil.Options{
		Interval:  interval(req.IntervalMs),
		MaxScans:  req.MaxScans,
		Tolerance: req.Tolerance,
		Engines:   engines,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, err
	}
	reply := testReply{Passed: true, Results: []testResult{}}
	for _, res := range results {
		tr := testResult{Name: res.Name, Passed: res.Passed, Mismatch: res.Mismatch, Variables: []variable{}}
		for _, v := range res.Variables {
			tr.Variables = append(tr.Variables, variable{Name: v.Name, Type: v.Type})
		}
		for _, r := range res.Engines {
			er := engineResult{Engine: string(r.Engine), Passed: r.Passed, Done: r.Done, Failures: r.Failures,
				Message: r.Message, Error: r.Err, Problems: r.Problems(), ScanCount: len(r.Scans)}
			if req.Scans {
				er.Scans = r.Scans
			}
			tr.Engines = append(tr.Engines, er)
		}
		if !res.Passed {
			reply.Passed = false
			reply.Failed++
		}
		reply.Results = append(reply.Results, tr)
	}
	return reply, nil
}
