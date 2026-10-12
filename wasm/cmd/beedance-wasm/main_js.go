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

// main gives the page globalThis.beedance.handle(request), which takes a
// JSON request string and returns the JSON reply string (see package wasm),
// then calls globalThis.onBeedanceReady, if the page set it, and stays
// running to answer calls.
func main() {
	h := wasm.New()
	handle := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"handle takes one JSON request string"}`
		}
		return string(h.Handle([]byte(args[0].String())))
	})
	js.Global().Set("beedance", js.ValueOf(map[string]any{"handle": handle}))
	if ready := js.Global().Get("onBeedanceReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
