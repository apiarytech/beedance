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

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// Point is an I/O point of a run: a located variable's address, e.g.
// "%IX0.0", and its declared type, e.g. "BOOL" or "INT" for
// `x AT %IW1 : INT`; a direct variable used without a declaration has the
// type its size gives (BOOL, BYTE, WORD, DWORD, LWORD).
type Point struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// Input reports whether p is in the input area (%I), which the IO gives the
// program before each scan.
func (p Point) Input() bool { return ast.IsInputAddress(p.Address) }

// Output reports whether p is in the output area (%Q), which the program
// gives the IO after each scan.
func (p Point) Output() bool { return isOutputAddress(p.Address) }

func isOutputAddress(addr string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(addr)), "%Q")
}

// IO connects the located variables of a run to the world outside the
// program: a plant model (software in the loop with a plant) or real I/O
// (hardware in the loop). With Options.IO each scan of a test is
//
//	inputs, _ := io.Read(ctx, t)  // into the I/O image: %I addresses only
//	the program's scan
//	io.Write(ctx, t, outputs)     // the I/O image's %Q addresses
//
// where t is the run's time since its first scan, simulated or, with
// Options.RealTime, measured. Each run of a test on an engine is bracketed
// by Begin and End, so a rig can put the plant into a known state first.
// Values are beedance objects (object.Boolean, object.Int, object.Word,
// ...); ToObject and FromObject convert them from and to plain Go values.
//
// An error from any method fails that engine's run of the test.
type IO interface {
	Begin(ctx context.Context, test string, engine Engine, points []Point) error
	Read(ctx context.Context, t time.Duration) (map[string]object.Object, error)
	Write(ctx context.Context, t time.Duration, outputs map[string]object.Object) error
	End(ctx context.Context) error
}

// ioRun is an engine's run connected to Options.IO.
type ioRun struct {
	io     IO
	image  map[string]object.Object
	points []Point
	key    map[string]string // the image's spelling of an address, by upper case
}

// beginIO starts a run on image, the engine's I/O image after
// initialization; types are the declared types of located variables by
// address. A nil IO returns a nil ioRun, whose methods do nothing.
func beginIO(ctx context.Context, io IO, test string, e Engine, image map[string]object.Object, types map[string]string) (*ioRun, error) {
	if io == nil {
		return nil, nil
	}
	r := &ioRun{io: io, image: image, key: map[string]string{}}
	add := func(addr, typ string) {
		up := strings.ToUpper(addr)
		if _, seen := r.key[up]; seen {
			return
		}
		if typ == "" {
			typ, _ = ast.DirectType(addr)
		}
		r.key[up] = addr
		r.points = append(r.points, Point{Address: addr, Type: strings.ToUpper(typ)})
	}
	for addr, typ := range types {
		add(addr, typ)
	}
	for addr := range image {
		if _, ok := ast.DirectType(addr); ok {
			add(addr, "")
		}
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i].Address < r.points[j].Address })
	if err := io.Begin(ctx, test, e, r.points); err != nil {
		return nil, fmt.Errorf("io begin: %w", err)
	}
	return r, nil
}

// read sets the inputs for the next scan.
func (r *ioRun) read(ctx context.Context, t time.Duration) error {
	if r == nil {
		return nil
	}
	in, err := r.io.Read(ctx, t)
	if err != nil {
		return fmt.Errorf("io read: %w", err)
	}
	for addr, v := range in {
		if !ast.IsInputAddress(addr) {
			return fmt.Errorf("io read: %s is not an input (%%I) address", addr)
		}
		if v == nil {
			continue
		}
		k, ok := r.key[strings.ToUpper(addr)]
		if !ok {
			k = strings.ToUpper(addr) // a direct variable no declaration names
		}
		r.image[k] = v
	}
	return nil
}

// write sends the outputs after a scan.
func (r *ioRun) write(ctx context.Context, t time.Duration) error {
	if r == nil {
		return nil
	}
	out := map[string]object.Object{}
	for addr, v := range r.image {
		if isOutputAddress(addr) && v != nil {
			out[addr] = v
		}
	}
	if err := r.io.Write(ctx, t, out); err != nil {
		return fmt.Errorf("io write: %w", err)
	}
	return nil
}

// record adds the I/O points' values to a scan, by address.
func (r *ioRun) record(values Scan) {
	if r == nil {
		return
	}
	for _, p := range r.points {
		if v, ok := r.image[p.Address]; ok && v != nil {
			values[p.Address] = v.Inspect()
		}
	}
}

// end ends the run; err is the run's error so far, kept over End's.
func (r *ioRun) end(ctx context.Context, res *EngineResult) {
	if r == nil {
		return
	}
	// End runs even when ctx is done, so a rig can make the plant safe.
	if err := r.io.End(context.WithoutCancel(ctx)); err != nil && res.Err == "" {
		res.Err = "io end: " + err.Error()
	}
}

// clock paces a run's scans: on a simulated clock scan n is at
// n × Interval at once; with RealTime it waits until then and reads the
// time since the first scan.
type clock struct {
	real     bool
	interval time.Duration
	start    time.Time
}

func newClock(opts Options) *clock {
	return &clock{real: opts.RealTime, interval: opts.Interval, start: time.Now()}
}

// at returns scan n's time since the first scan.
func (c *clock) at(ctx context.Context, n int) (time.Duration, error) {
	due := time.Duration(n) * c.interval
	if !c.real {
		return due, nil
	}
	if wait := time.Until(c.start.Add(due)); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
	return time.Since(c.start), nil
}

// ToObject converts v to the beedance object of the elementary type typ:
// a bool for BOOL; an integer or float64 (as encoding/json decodes a
// number) for the integer, REAL and bit string types; a string for STRING
// and WSTRING; a time.Duration, or a string such as "T#1.5s" or "1500ms",
// for TIME.
func ToObject(typ string, v any) (object.Object, error) {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	bad := func() (object.Object, error) { return nil, fmt.Errorf("%v (%T) is not a %s", v, v, typ) }
	switch typ {
	case "BOOL":
		if b, ok := v.(bool); ok {
			return &object.Boolean{Value: b}, nil
		}
		return bad()
	case "STRING", "WSTRING":
		s, ok := v.(string)
		if !ok {
			return bad()
		}
		if typ == "WSTRING" {
			return &object.WString{Value: s}, nil
		}
		return &object.String{Value: s}, nil
	case "TIME":
		switch x := v.(type) {
		case time.Duration:
			return &object.Time{Value: x}, nil
		case string:
			s := strings.ToLower(x)
			for _, p := range []string{"time#", "t#"} {
				s = strings.TrimPrefix(s, p)
			}
			d, err := object.ParseDuration(s)
			if err != nil {
				return bad()
			}
			return &object.Time{Value: d}, nil
		}
		return bad()
	case "REAL", "LREAL":
		f, ok := number(v)
		if !ok {
			return bad()
		}
		if typ == "REAL" {
			return &object.Real{Value: f}, nil
		}
		return &object.LReal{Value: f}, nil
	}
	if n, ok := v.(json.Number); ok { // exactly, beyond float64's 53 bits
		if i, err := n.Int64(); err == nil {
			v = i
		} else if u, err := strconv.ParseUint(n.String(), 10, 64); err == nil {
			v = u
		}
	}
	f, ok := number(v)
	if !ok || f != math.Trunc(f) {
		return bad()
	}
	fits := func(lo, hi float64) bool { return f >= lo && f <= hi }
	switch typ {
	case "SINT":
		if fits(math.MinInt8, math.MaxInt8) {
			return &object.SInt{Value: int8(f)}, nil
		}
	case "INT":
		if fits(math.MinInt16, math.MaxInt16) {
			return &object.Int{Value: int16(f)}, nil
		}
	case "DINT":
		if fits(math.MinInt32, math.MaxInt32) {
			return &object.DInt{Value: int32(f)}, nil
		}
	case "LINT":
		if n, ok := v.(int64); ok {
			return &object.LInt{Value: n}, nil
		}
		if fits(math.MinInt64, math.MaxInt64) {
			return &object.LInt{Value: int64(f)}, nil
		}
	case "USINT", "BYTE":
		if fits(0, math.MaxUint8) {
			if typ == "BYTE" {
				return &object.Byte{Value: uint8(f)}, nil
			}
			return &object.USInt{Value: uint8(f)}, nil
		}
	case "UINT", "WORD":
		if fits(0, math.MaxUint16) {
			if typ == "WORD" {
				return &object.Word{Value: uint16(f)}, nil
			}
			return &object.UInt{Value: uint16(f)}, nil
		}
	case "UDINT", "DWORD":
		if fits(0, math.MaxUint32) {
			if typ == "DWORD" {
				return &object.DWord{Value: uint32(f)}, nil
			}
			return &object.UDInt{Value: uint32(f)}, nil
		}
	case "ULINT", "LWORD":
		n, ok := v.(uint64)
		if i, signed := v.(int64); signed && i >= 0 {
			n, ok = uint64(i), true
		}
		if !ok && fits(0, math.MaxUint64) {
			n, ok = uint64(f), true
		}
		if ok {
			if typ == "LWORD" {
				return &object.LWord{Value: n}, nil
			}
			return &object.ULInt{Value: n}, nil
		}
	default:
		return nil, fmt.Errorf("type %s is not supported for I/O", typ)
	}
	return nil, fmt.Errorf("%v does not fit a %s", v, typ)
}

// number returns v as a float64: a Go integer or float, or a json.Number.
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case interface{ Float64() (float64, error) }: // json.Number
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

// FromObject converts a beedance object to a plain Go value, as ToObject
// takes it: bool, int64, uint64 (unsigned integers and bit strings),
// float64, string, or time.Duration. An engine may hold a value in a wider
// object than its declared type (a literal is an LINT), so the value, not
// the object's type, is converted.
func FromObject(o object.Object) (any, error) {
	switch x := o.(type) {
	case *object.Boolean:
		return x.Value, nil
	case *object.Real:
		return x.Value, nil
	case *object.LReal:
		return x.Value, nil
	case *object.String:
		return x.Value, nil
	case *object.WString:
		return x.Value, nil
	case *object.Time:
		return x.Value, nil
	case *object.Byte:
		return uint64(x.Value), nil
	case *object.Word:
		return uint64(x.Value), nil
	case *object.DWord:
		return uint64(x.Value), nil
	case *object.LWord:
		return x.Value, nil
	case *object.BitString:
		return x.Value, nil
	case *object.ULInt:
		return x.Value, nil
	}
	if n, unsigned, ok := object.GetIntegerObjectValue(o); ok {
		if unsigned {
			return uint64(n), nil
		}
		return n, nil
	}
	if o == nil {
		return nil, fmt.Errorf("no value")
	}
	return nil, fmt.Errorf("%s (%T) is not an elementary value", o.Inspect(), o)
}
