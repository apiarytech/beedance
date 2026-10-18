//go:build js

/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package main

import (
	"syscall/js"

	"github.com/apiarytech/beedance/wasm"
)

// main gives the page globalThis.beedance:
//
//	handle(request)  takes a JSON request string and returns the JSON
//	                 reply string (see package wasm)
//	dap(onMessage)   starts a debug adapter (wasm.DAP) and returns
//	                 {send(json), close()}; onMessage(json) is called with
//	                 each response and event
//
// then calls globalThis.onBeedanceReady, if the page set it, and stays
// running to answer calls. The debugger lets the page in between scans
// (wasm.DAPYield), so the module may run on the page's own thread, though
// a Web Worker keeps the page smooth.
func main() {
	h := wasm.New()
	handle := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"handle takes one JSON request string"}`
		}
		return string(h.Handle([]byte(args[0].String())))
	})
	debug := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeFunction {
			panic("beedance.dap takes one function, called with each message")
		}
		onMessage := args[0]
		// The debugger yields by sleeping a millisecond: a timer lets the
		// host's other timers and messages in fairly. A self-posted
		// MessageChannel message does not, under Node.js, which drains
		// the port before its timers (see doc/web-debugger.md).
		d := wasm.NewDAP(func(msg []byte) { onMessage.Invoke(string(msg)) }, wasm.DAPYield, nil)
		send := js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) == 1 && args[0].Type() == js.TypeString {
				d.Send([]byte(args[0].String()))
			}
			return nil
		})
		closeFn := js.FuncOf(func(this js.Value, args []js.Value) any {
			d.Close()
			return nil
		})
		return js.ValueOf(map[string]any{"send": send, "close": closeFn})
	})
	js.Global().Set("beedance", js.ValueOf(map[string]any{"handle": handle, "dap": debug}))
	if ready := js.Global().Get("onBeedanceReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
