//go:build js && !nodebug

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

	"github.com/apiarytech/beedance/dap"
	"github.com/apiarytech/beedance/wasm"
)

// debugFunc is globalThis.beedance.dap (see main).
func debugFunc() any {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 1 || args[0].Type() != js.TypeFunction {
			panic("beedance.dap takes one function, called with each message")
		}
		onMessage := args[0]
		mode := ""
		var setup []func(*dap.Server)
		if len(args) > 1 && args[1].Type() == js.TypeObject {
			if y := args[1].Get("yield"); y.Type() == js.TypeString {
				mode = y.String()
			}
			if g := args[1].Get("goStart"); g.Type() == js.TypeFunction {
				setup = append(setup, func(srv *dap.Server) { srv.GoStart = wasm.GoStart(g) })
			}
		}
		yield, releaseYield := yielder(mode)
		d := wasm.NewDAP(func(msg []byte) { onMessage.Invoke(string(msg)) }, wasm.DAPYield, yield, setup...)
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
