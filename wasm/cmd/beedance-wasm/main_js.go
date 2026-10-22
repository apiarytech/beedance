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
//	                       {yield: "message"} (see yielder)
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
		if len(args) < 1 || args[0].Type() != js.TypeFunction {
			panic("beedance.dap takes one function, called with each message")
		}
		onMessage := args[0]
		mode := ""
		if len(args) > 1 && args[1].Type() == js.TypeObject {
			if y := args[1].Get("yield"); y.Type() == js.TypeString {
				mode = y.String()
			}
		}
		yield, releaseYield := yielder(mode)
		d := wasm.NewDAP(func(msg []byte) { onMessage.Invoke(string(msg)) }, wasm.DAPYield, yield)
		var send, closeFn js.Func
		send = js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) == 1 && args[0].Type() == js.TypeString {
				d.Send([]byte(args[0].String()))
			}
			return nil
		})
		// close ends the session and releases its functions: a page that
		// restarts the debugger often does not leak them. The page must
		// not call send or close again after it.
		closeFn = js.FuncOf(func(this js.Value, args []js.Value) any {
			d.Close()
			send.Release()
			closeFn.Release()
			releaseYield()
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

// yielder is how a free-running program lets the host in between scans.
// "sleep" (and ""), nil: the debugger sleeps a millisecond, a timer that
// lets the host's other timers and messages in fairly. "message": it
// posts itself a MessageChannel message and waits for it, a task a
// browser schedules among its others without a timer's clamping; under
// Node.js it starves the timers (see doc/web-debugger.md, The yield).
// release frees what the yield holds.
func yielder(mode string) (yield func(), release func()) {
	if mode != "message" {
		return nil, func() {}
	}
	ch := js.Global().Get("MessageChannel").New()
	wake := make(chan struct{}, 1)
	onMessage := js.FuncOf(func(this js.Value, args []js.Value) any {
		select {
		case wake <- struct{}{}:
		default:
		}
		return nil
	})
	port1, port2 := ch.Get("port1"), ch.Get("port2")
	port1.Set("onmessage", onMessage)
	done := make(chan struct{})
	yield = func() {
		select {
		case <-done:
			return
		default:
		}
		port2.Call("postMessage", nil)
		select {
		case <-wake:
		case <-done: // closed while it waited: the message never comes
		}
	}
	release = func() {
		close(done)
		port1.Set("onmessage", js.Null())
		port1.Call("close")
		onMessage.Release()
	}
	return yield, release
}
