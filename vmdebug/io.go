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

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/sil"
)

// The process image, as a simulator's virtual field devices see it: the
// outputs to show and the inputs to write, a scan at a time.

// IO is the I/O image: every located variable's and direct address's
// value the program has, by address ("%IX0.0", "%QW2"): a BOOL, an
// integer, a REAL, or, for TIME and the like, the value as the source
// writes it. An address the program has neither declared nor written is
// not in it.
func (s *Session) IO() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.io()
}

func (s *Session) io() map[string]any {
	out := map[string]any{}
	for addr, o := range s.img.vm.IO() {
		if o != nil {
			out[addr] = ioValue(o)
		}
	}
	return out
}

// ioValue is o as JSON writes it: a bool, a number, else its text.
func ioValue(o object.Object) any {
	v, err := sil.FromObject(o)
	if err != nil {
		return Format(o)
	}
	if _, isTime := v.(time.Duration); isTime {
		return Format(o)
	}
	return v
}

// maxInputs bounds the scans of inputs SetInputs queues; past it, inputs
// join the last scan's.
const maxInputs = 1024

// SetInputs queues values (by address or variable name, as Set takes
// them) for one scan: the first scan to start takes the first call's
// values, the next scan the next call's, so a button pressed and released
// between two scans is still pressed for one. Values that are not strings
// (a bool, a number) are written as the source would write them. It may
// be called while a run is in progress, or stopped: the values wait for
// the next scan. Forces are applied after them, and win.
func (s *Session) SetInputs(values map[string]any) error {
	if s.host != nil {
		return ErrAttached // the inputs are the host's plant's
	}
	if len(values) == 0 {
		return nil
	}
	names := make([]string, 0, len(values))
	for n := range values {
		names = append(names, n)
	}
	sort.Strings(names)
	s.cmu.Lock()
	defer s.cmu.Unlock()
	scan := make([]write, 0, len(values))
	for _, n := range names {
		if _, err := s.resolveWith(n, false); err != nil {
			return err
		}
		text, err := literal(values[n])
		if err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		scan = append(scan, write{n, text})
	}
	if len(s.inputs) >= maxInputs {
		last := len(s.inputs) - 1
		s.inputs[last] = append(s.inputs[last], scan...)
		return nil
	}
	s.inputs = append(s.inputs, scan)
	return nil
}

// QueuedInputs is the number of scans of inputs SetInputs has queued and
// no scan has taken yet.
func (s *Session) QueuedInputs() int {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	return len(s.inputs)
}

// literal is v, a JSON value, as the source would write it.
func literal(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		if x {
			return "TRUE", nil
		}
		return "FALSE", nil
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10), nil
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	}
	return "", fmt.Errorf("%v is not a value", v)
}

// applyInputs writes the first queued scan's inputs.
func (s *Session) applyInputs() {
	s.cmu.Lock()
	if len(s.inputs) == 0 {
		s.cmu.Unlock()
		return
	}
	scan := s.inputs[0]
	s.inputs = s.inputs[1:]
	var failed []string
	for _, w := range scan {
		if err := s.set(w.name, w.value); err != nil {
			failed = append(failed, "input "+strings.TrimPrefix(err.Error(), "vmdebug: "))
		}
	}
	s.cmu.Unlock()
	for _, msg := range failed {
		s.notify(msg)
	}
}

// SetSampled names more values the Sample of each scan's end keeps, as
// text (Sample.Extra): variables, members ("t.Q", "m.t.ET") or addresses,
// for a view that shows them while the program runs (a diagram's live
// values). It may be called while a run is in progress.
func (s *Session) SetSampled(names []string) {
	cp := append([]string(nil), names...)
	s.sampled.Store(&cp)
}

// Texts reads names now (variables, members or addresses), as the source
// would write their values; a name with no value is left out. It waits for
// a run in progress to stop.
func (s *Session) Texts(names []string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.texts(names)
}

// texts is Texts with mu held.
func (s *Session) texts(names []string) map[string]string {
	out := make(map[string]string, len(names))
	for _, n := range names {
		var o object.Object
		var ok bool
		if strings.HasPrefix(n, "%") {
			o, ok = s.img.vm.IO()[strings.ToUpper(n)]
		} else {
			o, ok = s.lookup(strings.Split(n, "."))
		}
		if ok && o != nil {
			out[n] = Format(o)
		}
	}
	return out
}
