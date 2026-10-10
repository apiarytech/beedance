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

// This file runs the go engine with Options.IO. The transpiled test runs in
// its own process, as without IO, but scan by scan on request: sil reads
// the inputs from the IO, sends them with the scan's time, and the harness
// writes them into the transpiled file's process image, runs the scan, and
// replies with the watched values and the outputs from the image.
//
//	← {"points":[{"address":"%IW0","type":"INT"}, ...]}
//	→ {"t_ms":10,"inputs":{"%IW0":512}}
//	← {"row":["0","512",...],"outputs":{"%QX0.0":true},"done":false}

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/transpiler"
)

// ioPoint is a located variable of a test as the harness drives it.
type ioPoint struct {
	Point
	slot, slotType string // its process image entry, e.g. img.I.W[0], iec.WORD
}

// testPoints lists the located %I and %Q variables a test program declares.
func testPoints(decl *ast.ProgramDeclaration) ([]ioPoint, error) {
	var out []ioPoint
	for _, block := range [][]*ast.VarDeclStatement{decl.VarInputs, decl.VarOutputs, decl.Vars} {
		for _, d := range block {
			if d == nil || d.Location == nil || d.Location.Location == nil || d.DataType == nil {
				continue
			}
			addr := d.Location.Location.FullAddress()
			typ := strings.ToUpper(d.DataType.String())
			area, slot, slotType, err := transpiler.ProcessImageSlot(addr, typ)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", d.Name.Value, err)
			}
			if area == "M" {
				continue // memory, not I/O
			}
			if goConv(typ) == "" {
				return nil, fmt.Errorf("%s: a %s is not supported for I/O", d.Name.Value, typ)
			}
			out = append(out, ioPoint{Point: Point{Address: addr, Type: typ}, slot: slot, slotType: slotType})
		}
	}
	return out, nil
}

// goConv names the harness function that converts a JSON value for typ.
func goConv(typ string) string {
	switch typ {
	case "BOOL":
		return "silBool"
	case "SINT", "INT", "DINT", "LINT":
		return "silInt"
	case "USINT", "UINT", "UDINT", "ULINT", "BYTE", "WORD", "DWORD", "LWORD":
		return "silUint"
	case "REAL", "LREAL":
		return "silFloat"
	case "STRING", "WSTRING":
		return "silString"
	}
	return ""
}

// harnessIOSource is the Go of RunIO, which runs one test on request, scan
// by scan, its I/O through the process image.
func harnessIOSource(tests []*testInfo, hasClock bool) (string, error) {
	var b strings.Builder
	b.WriteString(`
type silPoint struct {
	Address string ` + "`json:\"address\"`" + `
	Type    string ` + "`json:\"type\"`" + `
}

type silRequest struct {
	TimeMS float64        ` + "`json:\"t_ms\"`" + `
	Inputs map[string]any ` + "`json:\"inputs\"`" + `
}

type silReply struct {
	Points  []silPoint     ` + "`json:\"points,omitempty\"`" + `
	Row     []string       ` + "`json:\"row,omitempty\"`" + `
	Outputs map[string]any ` + "`json:\"outputs,omitempty\"`" + `
	Done    bool           ` + "`json:\"done\"`" + `
	Err     string         ` + "`json:\"err,omitempty\"`" + `
}

func silBool(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%v is not a BOOL", v)
	}
	return b, nil
}

func silInt(v any) (int64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%v is not an integer", v)
	}
	return n.Int64()
}

func silUint(v any) (uint64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%v is not an unsigned integer", v)
	}
	return strconv.ParseUint(n.String(), 10, 64)
}

func silFloat(v any) (float64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%v is not a number", v)
	}
	return n.Float64()
}

func silString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%v is not a string", v)
	}
	return s, nil
}

// silJSON is an IEC value as JSON takes it.
func silJSON(v any) any {
	switch x := v.(type) {
	case iec.BOOL:
		return bool(x)
	case iec.SINT:
		return int64(x)
	case iec.INT:
		return int64(x)
	case iec.DINT:
		return int64(x)
	case iec.LINT:
		return int64(x)
	case iec.USINT:
		return uint64(x)
	case iec.UINT:
		return uint64(x)
	case iec.UDINT:
		return uint64(x)
	case iec.ULINT:
		return uint64(x)
	case iec.BYTE:
		return uint64(x)
	case iec.WORD:
		return uint64(x)
	case iec.DWORD:
		return uint64(x)
	case iec.LWORD:
		return uint64(x)
	case iec.REAL:
		return float64(x)
	case iec.LREAL:
		return float64(x)
	case iec.STRING:
		return string(x)
	case iec.WSTRING:
		return string(x)
	}
	return fmt.Sprint(v)
}

// RunIO runs the test called name, a scan for each request read from r,
// replying on w. It first replies with the test's I/O points.
func RunIO(name string, r io.Reader, w io.Writer) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	enc := json.NewEncoder(w)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var elapsed time.Duration
	_ = elapsed
`)
	if hasClock {
		b.WriteString("\tplcClock = func() time.Duration { return elapsed }\n")
	}
	b.WriteString("\tswitch name {\n")
	usesImage := false
	for _, t := range tests {
		if t.out.failures == "" {
			continue
		}
		points, err := testPoints(t.decl)
		if err != nil {
			return "", fmt.Errorf("%s: %v", t.decl.Name.Value, err)
		}
		name := t.decl.Name.Value
		fmt.Fprintf(&b, "\tcase %q:\n\t\tp := New%s()\n", name, name)
		b.WriteString("\t\tenc.Encode(silReply{Points: []silPoint{\n")
		for _, pt := range points {
			fmt.Fprintf(&b, "\t\t\t{%q, %q},\n", pt.Address, pt.Type)
		}
		b.WriteString("\t\t}})\n")
		b.WriteString("\t\tfor {\n\t\t\tvar req silRequest\n\t\t\tif dec.Decode(&req) != nil {\n\t\t\t\treturn\n\t\t\t}\n")
		b.WriteString("\t\t\tvar rep silReply\n\t\t\tfunc() {\n")
		b.WriteString("\t\t\t\tdefer func() {\n\t\t\t\t\tif x := recover(); x != nil {\n\t\t\t\t\t\trep.Err = fmt.Sprintf(\"panic: %v\", x)\n\t\t\t\t\t}\n\t\t\t\t}()\n")
		for _, pt := range points {
			if !pt.Input() {
				continue
			}
			usesImage = true
			fmt.Fprintf(&b, "\t\t\t\tif v, ok := req.Inputs[%q]; ok {\n", pt.Address)
			fmt.Fprintf(&b, "\t\t\t\t\tx, err := %s(v)\n", goConv(pt.Type))
			fmt.Fprintf(&b, "\t\t\t\t\tif err != nil {\n\t\t\t\t\t\trep.Err = %q + err.Error()\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n", pt.Address+": ")
			fmt.Fprintf(&b, "\t\t\t\t\tprocessImage.Write(func(img *vars.Image) { %s = %s(iec.%s(x)) })\n", pt.slot, pt.slotType, pt.Type)
			b.WriteString("\t\t\t\t}\n")
		}
		b.WriteString("\t\t\t\telapsed = time.Duration(req.TimeMS * float64(time.Millisecond))\n")
		b.WriteString("\t\t\t\tp.Logic(start.Add(elapsed))\n")
		b.WriteString("\t\t\t\trep.Row = []string{\n")
		for _, v := range t.vars {
			fmt.Fprintf(&b, "\t\t\t\t\t%s,\n", goFormat(goSelector(v.Path), v.Type))
		}
		b.WriteString("\t\t\t\t}\n\t\t\t\trep.Outputs = map[string]any{}\n")
		for _, pt := range points {
			if !pt.Output() {
				continue
			}
			usesImage = true
			fmt.Fprintf(&b, "\t\t\t\tprocessImage.Read(func(img *vars.Image) { rep.Outputs[%q] = silJSON(iec.%s(%s)) })\n", pt.Address, pt.Type, pt.slot)
		}
		if t.out.done != "" {
			fmt.Fprintf(&b, "\t\t\t\trep.Done = bool(p.%s)\n", transpiler.GoName(t.out.done))
		}
		b.WriteString("\t\t\t}()\n\t\t\tenc.Encode(rep)\n\t\t\tif rep.Err != \"\" || rep.Done {\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n")
	}
	b.WriteString("\tdefault:\n\t\tenc.Encode(silReply{Err: \"no such test\"})\n\t}\n}\n")
	src := b.String()
	if !usesImage {
		src += "\nvar _ vars.Image\n"
	}
	return src, nil
}

// harnessIOImports are what harnessIOSource needs beyond the harness's own.
const harnessIOImports = `	"strconv"

	"github.com/apiarytech/royaljelly/iec"
	"github.com/apiarytech/royaljelly/vars"
`

// nativeReply is a reply of RunIO.
type nativeReply struct {
	Points  []Point        `json:"points"`
	Row     []string       `json:"row"`
	Outputs map[string]any `json:"outputs"`
	Done    bool           `json:"done"`
	Err     string         `json:"err"`
}

// runIO runs one test with opts.IO, scan by scan. GoTimeout bounds each
// scan, so a runaway loop fails the test while a long real-time run does
// not.
func (n *native) runIO(ctx context.Context, t *testInfo, opts Options) (r EngineResult) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, n.bin, t.decl.Name.Value, strconv.Itoa(t.limit), opts.Interval.String(), "io")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.Err = err.Error()
		return r
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.Err = err.Error()
		return r
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		r.Err = "go: " + err.Error()
		return r
	}
	defer func() {
		stdin.Close()
		cancel()
		cmd.Wait()
	}()
	replies := make(chan nativeReply)
	go func() {
		defer close(replies)
		dec := json.NewDecoder(bufio.NewReader(stdout))
		dec.UseNumber()
		for {
			var rep nativeReply
			if dec.Decode(&rep) != nil {
				return
			}
			select {
			case replies <- rep:
			case <-ctx.Done():
				return
			}
		}
	}()
	next := func() (nativeReply, error) {
		timer := time.NewTimer(opts.GoTimeout)
		defer timer.Stop()
		select {
		case rep, ok := <-replies:
			if !ok {
				return rep, fmt.Errorf("go: the test stopped: %s", strings.TrimSpace(stderr.String()))
			}
			if rep.Err != "" {
				return rep, fmt.Errorf("%s", rep.Err)
			}
			return rep, nil
		case <-timer.C:
			return nativeReply{}, fmt.Errorf("no reply after %v: a runaway loop? (see -go-timeout)", opts.GoTimeout)
		case <-ctx.Done():
			return nativeReply{}, ctx.Err()
		}
	}

	first, err := next()
	if err != nil {
		r.Err = err.Error()
		return r
	}
	types := map[string]string{}
	for _, p := range first.Points {
		types[p.Address] = p.Type
	}
	if err := opts.IO.Begin(ctx, t.decl.Name.Value, Go, first.Points); err != nil {
		r.Err = "io begin: " + err.Error()
		return r
	}
	defer func() {
		if err := opts.IO.End(context.WithoutCancel(ctx)); err != nil && r.Err == "" {
			r.Err = "io end: " + err.Error()
		}
	}()
	enc := json.NewEncoder(stdin)
	clk := newClock(opts)
	for n := 0; n < t.limit; n++ {
		at, err := clk.at(ctx, n)
		if err != nil {
			r.Err = err.Error()
			break
		}
		in, err := opts.IO.Read(ctx, at)
		if err != nil {
			r.Err = fmt.Sprintf("scan %d: io read: %v", n+1, err)
			break
		}
		inputs := map[string]any{}
		for addr, o := range in {
			if !ast.IsInputAddress(addr) {
				r.Err = fmt.Sprintf("scan %d: io read: %s is not an input (%%I) address", n+1, addr)
				return r
			}
			for a := range types { // the test's spelling of the address
				if strings.EqualFold(a, addr) {
					addr = a
				}
			}
			v, err := jsonValue(o)
			if err != nil {
				r.Err = fmt.Sprintf("scan %d: io read: %s: %v", n+1, addr, err)
				return r
			}
			inputs[addr] = v
		}
		if err := enc.Encode(map[string]any{"t_ms": float64(at) / float64(time.Millisecond), "inputs": inputs}); err != nil {
			r.Err = fmt.Sprintf("scan %d: go: %v", n+1, err)
			break
		}
		rep, err := next()
		if err != nil {
			r.Err = fmt.Sprintf("scan %d: %v", n+1, err)
			break
		}
		values := Scan{}
		for i, v := range t.vars {
			if i < len(rep.Row) {
				values[v.Name] = rep.Row[i]
			}
		}
		for addr, o := range in {
			if o != nil {
				values[addr] = o.Inspect()
			}
		}
		outputs := map[string]object.Object{}
		for addr, v := range rep.Outputs {
			o, err := ToObject(types[addr], v)
			if err != nil {
				r.Err = fmt.Sprintf("scan %d: output %s: %v", n+1, addr, err)
				return r
			}
			outputs[addr] = o
			values[addr] = o.Inspect()
		}
		r.Scans = append(r.Scans, values)
		if err := opts.IO.Write(ctx, at, outputs); err != nil {
			r.Err = fmt.Sprintf("scan %d: io write: %v", n+1, err)
			break
		}
		if rep.Done {
			break
		}
	}
	return r
}

// jsonValue is one object as the protocols' JSON value.
func jsonValue(o object.Object) (any, error) {
	m, err := jsonValues(map[string]object.Object{"": o})
	return m[""], err
}
