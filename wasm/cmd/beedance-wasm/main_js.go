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
//	handle(request)        takes a JSON request string and returns the
//	                       JSON reply string (see package wasm)
//	dap(onMessage, opts)   starts a debug adapter (wasm.DAP) and returns
//	                       {send(json), close()}; onMessage(json) is
//	                       called with each response and event. opts is
//	                       optional: {yield: "sleep"} (the default) or
//	                       {yield: "message"} (see yielder), and
//	                       goStart, a function that builds and runs the
//	                       go engine's program (wasm.GoStart). A module
//	                       built with -tags nodebug has no dap.
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
	api := map[string]any{"handle": handle}
	if debug := debugFunc(); debug != nil {
		api["dap"] = debug
	}
	js.Global().Set("beedance", js.ValueOf(api))
	if ready := js.Global().Get("onBeedanceReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
