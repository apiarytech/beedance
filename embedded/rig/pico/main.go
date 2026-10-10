/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command pico is a hardware-in-the-loop rig on a Raspberry Pi Pico: it
// serves beedance's rig protocol (doc/hil.md) on its serial port, so
// `beedance -test -io serial:/dev/ttyACM0 tests.st` runs Structured Text
// tests against the pins wired to it.
//
//	%IX0.0 .. %IX0.7   digital inputs  GP10 .. GP17 (pulled down)
//	%QX0.0 .. %QX0.7   digital outputs GP2 .. GP9
//	%IW0 .. %IW2       analog inputs   ADC0 .. ADC2 (GP26 .. GP28), 0 .. 4095
//
// A test may use only these points: begin refuses any other. The outputs
// are low (the safe state) at power-up, at begin, at end, and when the host
// says nothing for HostTimeout within a run, e.g. because it crashed.
//
//	tinygo flash -target=pico ./embedded/rig/pico              # USB serial
//	tinygo build -target=pico -serial=uart ./embedded/rig/pico # UART0, GP0/GP1
package main

import (
	"encoding/json"
	"fmt"
	"machine"
	"strings"
	"time"
)

// HostTimeout is how long the host may say nothing within a run before the
// outputs are made safe.
const HostTimeout = 2 * time.Second

// The rig's points, by address.
var (
	digitalIn  = map[string]machine.Pin{}
	digitalOut = map[string]machine.Pin{}
	analogIn   = map[string]machine.ADC{}
)

func init() {
	for i := 0; i < 8; i++ {
		digitalIn[fmt.Sprintf("%%IX0.%d", i)] = machine.GP10 + machine.Pin(i)
		digitalOut[fmt.Sprintf("%%QX0.%d", i)] = machine.GP2 + machine.Pin(i)
	}
	for i, pin := range []machine.Pin{machine.ADC0, machine.ADC1, machine.ADC2} {
		analogIn[fmt.Sprintf("%%IW%d", i)] = machine.ADC{Pin: pin}
	}
}

// message is a request or a reply of the rig protocol.
type message struct {
	Op      string         `json:"op,omitempty"`
	Test    string         `json:"test,omitempty"`
	Points  []point        `json:"points,omitempty"`
	Outputs map[string]any `json:"outputs,omitempty"`
	Inputs  map[string]any `json:"inputs,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type point struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// rig is the state of the current run.
type rig struct {
	running  bool
	timedOut bool
	points   []point
}

func main() {
	for _, pin := range digitalIn {
		pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}
	for _, pin := range digitalOut {
		pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	}
	safe()
	machine.InitADC()
	for _, a := range analogIn {
		a.Configure(machine.ADCConfig{})
	}

	var r rig
	var line []byte
	last := time.Now()
	serial := machine.Serial
	for {
		if serial.Buffered() == 0 {
			if r.running && time.Since(last) > HostTimeout {
				safe()
				r.running, r.timedOut = false, true
			}
			time.Sleep(time.Millisecond)
			continue
		}
		b, err := serial.ReadByte()
		if err != nil {
			continue
		}
		if b != '\n' {
			if len(line) < 4096 {
				line = append(line, b)
			}
			continue
		}
		last = time.Now()
		reply := r.handle(line)
		line = line[:0]
		out, err := json.Marshal(reply)
		if err != nil {
			out = []byte(`{"error":"cannot encode the reply"}`)
		}
		serial.Write(append(out, '\n'))
	}
}

// handle answers one request.
func (r *rig) handle(line []byte) message {
	var req message
	if err := json.Unmarshal(line, &req); err != nil {
		return message{Error: "not a rig protocol request: " + err.Error()}
	}
	switch req.Op {
	case "begin":
		for _, p := range req.Points {
			if err := supported(p); err != nil {
				safe()
				r.running = false
				return message{Error: err.Error()}
			}
		}
		safe()
		r.running, r.timedOut, r.points = true, false, req.Points
		return message{}
	case "end":
		safe()
		r.running = false
		return message{}
	}
	if !r.running {
		if r.timedOut {
			return message{Error: fmt.Sprintf("the host said nothing for %v: the outputs were made safe; begin again", HostTimeout)}
		}
		return message{Error: "no run: begin first"}
	}
	switch req.Op {
	case "read":
		in := map[string]any{}
		for _, p := range r.points {
			addr := strings.ToUpper(p.Address)
			if pin, ok := digitalIn[addr]; ok {
				in[p.Address] = pin.Get()
			} else if a, ok := analogIn[addr]; ok {
				in[p.Address] = a.Get() >> 4 // 12 bits
			}
		}
		return message{Inputs: in}
	case "write":
		for addr, v := range req.Outputs {
			pin, ok := digitalOut[strings.ToUpper(addr)]
			if !ok {
				continue // not one of this rig's: begin checked the points
			}
			on, ok := v.(bool)
			if !ok {
				return message{Error: fmt.Sprintf("%s: %v is not a BOOL", addr, v)}
			}
			pin.Set(on)
		}
		return message{}
	}
	return message{Error: fmt.Sprintf("unknown op %q", req.Op)}
}

// supported reports whether this rig has the point p, with a type it fits.
func supported(p point) error {
	addr, typ := strings.ToUpper(p.Address), strings.ToUpper(p.Type)
	_, din := digitalIn[addr]
	_, dout := digitalOut[addr]
	_, ain := analogIn[addr]
	switch {
	case din || dout:
		if typ != "BOOL" {
			return fmt.Errorf("%s is a digital point: BOOL, not %s", p.Address, p.Type)
		}
	case ain:
		switch typ {
		case "INT", "UINT", "DINT", "UDINT", "WORD", "DWORD":
		default:
			return fmt.Errorf("%s is an analog input (0..4095): not a %s", p.Address, p.Type)
		}
	default:
		return fmt.Errorf("%s is not wired on this rig", p.Address)
	}
	return nil
}

// safe sets every output low.
func safe() {
	for _, pin := range digitalOut {
		pin.Low()
	}
}
