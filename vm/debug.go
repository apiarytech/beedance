/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package vm

import (
	"errors"

	"github.com/apiarytech/beedance/object"
)

// ErrPaused is returned by Run when the VM's hook stopped it (SetHook). The
// run is not over: its stack, frames and instruction pointers are kept, and
// the next Run carries on from the instruction it stopped before. Reset
// abandons it instead.
var ErrPaused = errors.New("vm: paused")

// Hook is called before each instruction while it is set: fn is the code
// the instruction is in, ip its offset there, and depth the number of
// frames (1 in the program's body). Returning true pauses the run there
// (ErrPaused); the instruction then runs on the next Run, without the hook
// being asked about it again.
type Hook func(fn *object.CompiledFunction, ip, depth int) bool

// SetHook sets the hook a debugger watches the run through (package
// vmdebug); nil removes it. Without a hook the VM runs as fast as before.
func (vm *VM) SetHook(h Hook) {
	vm.hook = h
}

// FrameInfo is one call of the run the VM is in, for a debugger.
type FrameInfo struct {
	// Fn is the code the frame runs.
	Fn *object.CompiledFunction
	// IP is the offset in Fn the frame is at: in the innermost frame, the
	// instruction that runs next; in the others, the call they are in.
	// object.LineAt(Fn.Lines, IP) is its statement.
	IP int
	// Locals are the frame's local variables by slot (Fn.LocalNames), in
	// place: a debugger reads them, it does not keep them.
	Locals []object.Object
}

// Frames are the calls the VM is in, the program's body first and the
// innermost last; while it is paused, the innermost is where it stopped.
func (vm *VM) Frames() []FrameInfo {
	out := make([]FrameInfo, 0, vm.framesIndex)
	for i := 0; i < vm.framesIndex; i++ {
		f := vm.frames[i]
		info := FrameInfo{Fn: f.cl.Fn, IP: f.ip}
		if i == vm.framesIndex-1 {
			info.IP = f.ip + 1
		}
		if i > 0 && f.basePointer+f.cl.Fn.NumLocals <= len(vm.stack) {
			info.Locals = vm.stack[f.basePointer : f.basePointer+f.cl.Fn.NumLocals]
		}
		out = append(out, info)
	}
	return out
}
