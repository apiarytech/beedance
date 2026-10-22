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

// The trace, a logic analyzer's view: chosen signals recorded at the end
// of every scan, the last scans kept. And the speed: how fast simulated
// time runs against the wall clock.

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/watch"
)

// DefaultTraceDepth is how many scans a trace keeps when not told.
const DefaultTraceDepth = 4000

// traceTarget is where a traced signal's value is read: an address, a
// variable's path, or a plant model's variable.
type traceTarget struct {
	addr  string
	path  []string
	plant string // a plant model's name, for "PLANT_Tank.level"
}

// tracer records the signals.
type tracer struct {
	mu      sync.Mutex
	signals []string
	targets []traceTarget
	depth   int
	head    int // the next slot
	n       int // slots filled
	scans   []int
	times   []float64
	values  [][]any // by signal, by slot
}

// Trace is a trace's recording: the scans recorded, oldest first, their
// simulated times, and each signal's values over them (a bool, a number,
// or a value's text).
type Trace struct {
	Signals []string  `json:"signals"`
	Scans   []int     `json:"scans"`
	TimesMs []float64 `json:"timesMs"`
	Values  [][]any   `json:"values"`
}

// SetTrace records signals (variables, members, addresses, or a plant
// model's variables as "PLANT_Name.var") at the end of every scan from now
// on, keeping the last depth scans (DefaultTraceDepth if 0). An empty list
// stops the trace. It may be called while a run is in progress; the
// recording starts over.
func (s *Session) SetTrace(signals []string, depth int) error {
	if depth <= 0 {
		depth = DefaultTraceDepth
	}
	s.cmu.Lock()
	targets := make([]traceTarget, len(signals))
	for i, name := range signals {
		t, err := s.traceTarget(name)
		if err != nil {
			s.cmu.Unlock()
			return err
		}
		targets[i] = t
	}
	s.cmu.Unlock()
	tr := &tracer{signals: append([]string(nil), signals...), targets: targets, depth: depth,
		scans: make([]int, depth), times: make([]float64, depth), values: make([][]any, len(signals))}
	for i := range tr.values {
		tr.values[i] = make([]any, depth)
	}
	if len(signals) == 0 {
		tr = nil
	}
	s.trace.Store(tr)
	return nil
}

// traceTarget resolves a traced name; cmu is held.
func (s *Session) traceTarget(name string) (traceTarget, error) {
	name = strings.TrimSpace(name)
	first, rest, dotted := strings.Cut(name, ".")
	if dotted && IsPlant(first) {
		for _, p := range s.img.plants {
			if strings.EqualFold(p.name, first) {
				path := strings.Split(rest, ".")
				if _, ok := p.decls[strings.ToUpper(path[0])]; !ok {
					return traceTarget{}, fmt.Errorf("%s has no variable %s", p.name, path[0])
				}
				return traceTarget{plant: p.name, path: path}, nil
			}
		}
		return traceTarget{}, fmt.Errorf("no plant model %s", first)
	}
	t, err := s.resolveWith(name, false)
	if err != nil {
		return traceTarget{}, err
	}
	return traceTarget{addr: t.addr, path: t.path}, nil
}

// retrace resolves the traced names again, on a changed program; a name
// that is gone reads as nothing. cmu is held.
func (s *Session) retrace() {
	tr := s.trace.Load()
	if tr == nil {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for i, name := range tr.signals {
		t, err := s.traceTarget(name)
		if err != nil {
			t = traceTarget{path: []string{"\x00gone"}}
		}
		tr.targets[i] = t
	}
}

// record keeps the traced signals' values at the end of a scan; mu is held.
func (s *Session) record() {
	tr := s.trace.Load()
	if tr == nil {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	at := tr.head
	tr.scans[at] = s.scans
	tr.times[at] = float64(s.elapsed) / float64(time.Millisecond)
	for i, t := range tr.targets {
		var v any
		if o, ok := s.traceRead(t); ok && o != nil {
			v = ioValue(o)
		}
		tr.values[i][at] = v
	}
	tr.head = (tr.head + 1) % tr.depth
	tr.n = min(tr.n+1, tr.depth)
}

// traceRead reads a traced signal; mu is held.
func (s *Session) traceRead(t traceTarget) (object.Object, bool) {
	if t.addr != "" {
		o, ok := s.img.eng.IO()[t.addr]
		return o, ok
	}
	if t.plant != "" {
		for _, p := range s.img.plants {
			if !strings.EqualFold(p.name, t.plant) {
				continue
			}
			d, ok := p.decls[strings.ToUpper(t.path[0])]
			if !ok {
				return nil, false
			}
			var top object.Object
			if d.Address != "" {
				top = s.img.eng.IO()[d.Address]
			} else if d.Global >= 0 && d.Global < len(p.globals) {
				top = p.globals[d.Global]
			}
			if len(t.path) == 1 || top == nil {
				return top, top != nil
			}
			return watch.Member(top, t.path[1:])
		}
		return nil, false
	}
	return s.lookup(t.path)
}

// TraceData is the recording of the last n scans (all it keeps if n is 0
// or more), oldest first; nil values are scans the signal had none.
func (s *Session) TraceData(n int) Trace {
	tr := s.trace.Load()
	if tr == nil {
		return Trace{Signals: []string{}, Scans: []int{}, TimesMs: []float64{}, Values: [][]any{}}
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if n <= 0 || n > tr.n {
		n = tr.n
	}
	out := Trace{Signals: append([]string(nil), tr.signals...), Scans: make([]int, n), TimesMs: make([]float64, n), Values: make([][]any, len(tr.signals))}
	start := (tr.head - n + tr.depth) % tr.depth
	for j := range out.Values {
		out.Values[j] = make([]any, n)
	}
	for i := 0; i < n; i++ {
		at := (start + i) % tr.depth
		out.Scans[i] = tr.scans[at]
		out.TimesMs[i] = tr.times[at]
		for j := range tr.signals {
			out.Values[j][i] = tr.values[j][at]
		}
	}
	return out
}

// SetSpeed sets how fast simulated time runs against the wall clock while
// the program runs on (Continue): 1 is real time, 2 twice as fast, 0.5
// half; 0 runs scans as fast as they go. It may be called while a run is
// in progress.
func (s *Session) SetSpeed(factor float64) {
	if factor < 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		factor = 0
	}
	s.speed.Store(math.Float64bits(factor))
}

// Speed is the speed SetSpeed set (or Options.Realtime: 1, else 0).
func (s *Session) Speed() float64 { return math.Float64frombits(s.speed.Load()) }
