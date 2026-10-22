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
	"context"
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/code"
	"github.com/apiarytech/beedance/object"
)

// A disassembly view lists the program's code with its source: the body
// first, then each of the source's POUs, one after another at addresses of
// their own, so that an address names one instruction of one of them.

// Instruction is an instruction of the program's code.
type Instruction struct {
	// Address is the instruction's: its code's base and its offset there.
	Address int
	// POU is the code it is in: the program's body, a function block's,
	// "FB.Method".
	POU string
	// Text is the instruction, e.g. "OpGetGlobal 3 (n)" or
	// "OpConstant 7 (T#2s)".
	Text string
	// Line and Column are its statement's; 0 if it has none.
	Line, Column int
}

// block is a piece of code at an address.
type block struct {
	fn   *object.CompiledFunction
	name string
	base int
}

// blocks are the program's code at their addresses, made on first use.
func (e *vmEngine) codeBlocks() []block {
	if e.blocks != nil {
		return e.blocks
	}
	main := e.vm.Frames()[0].Fn
	fns := []*object.CompiledFunction{main}
	for _, c := range e.compiled.CyclicBytecode.Constants {
		if fn, ok := c.(*object.CompiledFunction); ok && fn.Name != "" && e.isUser(fn) {
			fns = append(fns, fn)
		}
	}
	base := 0x1000
	e.blockBase = map[*object.CompiledFunction]int{}
	for _, fn := range fns {
		name := fn.Name
		if fn == main {
			name = e.name
		}
		e.blocks = append(e.blocks, block{fn: fn, name: name, base: base})
		e.blockBase[fn] = base
		base += (len(fn.Instructions)/0x100 + 1) * 0x100
	}
	return e.blocks
}

// address is the address of offset ip of fn; 0 if fn is not the source's.
func (e *vmEngine) address(fn *object.CompiledFunction, ip int) int {
	e.codeBlocks()
	if base, ok := e.blockBase[fn]; ok {
		return base + ip
	}
	return 0
}

// Disassembly is the program's code, in address order; nil on an engine
// other than the VM, which has no code to show.
func (s *Session) Disassembly() []Instruction {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.img.eng.(*vmEngine)
	if !ok {
		return nil
	}
	names := map[int]string{}
	for _, v := range e.compiled.Variables {
		if v.Global >= 0 {
			names[v.Global] = v.Name
		}
	}
	consts := e.compiled.CyclicBytecode.Constants
	var out []Instruction
	for _, b := range e.codeBlocks() {
		ins := b.fn.Instructions
		for i := 0; i < len(ins); {
			in := Instruction{Address: b.base + i, POU: b.name}
			if l, ok := object.LineAt(b.fn.Lines, i); ok {
				in.Line, in.Column = s.lineOut(l.Row), l.Column
			}
			def, err := code.Lookup(ins[i])
			if err != nil {
				in.Text = fmt.Sprintf("?? %d", ins[i])
				out = append(out, in)
				i++
				continue
			}
			operands, read := code.ReadOperands(def, ins[i+1:])
			parts := []string{def.Name}
			for _, o := range operands {
				parts = append(parts, fmt.Sprint(o))
			}
			in.Text = strings.Join(parts, " ")
			if len(operands) > 0 {
				switch def.Name {
				case "OpGetGlobal", "OpSetGlobal":
					if n, ok := names[operands[0]]; ok {
						in.Text += " (" + n + ")"
					}
				case "OpGetLocal", "OpSetLocal":
					if o := operands[0]; o < len(b.fn.LocalNames) && b.fn.LocalNames[o] != "" {
						in.Text += " (" + b.fn.LocalNames[o] + ")"
					}
				case "OpConstant":
					if o := operands[0]; o < len(consts) {
						switch c := consts[o].(type) {
						case *object.CompiledFunction:
							in.Text += " (code " + c.Name + ")"
						default:
							if t := Format(c); len(t) <= 40 {
								in.Text += " (" + t + ")"
							}
						}
					}
				}
			}
			out = append(out, in)
			i += 1 + read
		}
	}
	return out
}

// StepInstruction runs one instruction of the source's code: the next,
// into a call or out of one; standard blocks' code is run through. On an
// engine with no instructions it steps to the next statement, into a
// call if there is one.
func (s *Session) StepInstruction(ctx context.Context) (Stop, error) {
	return s.run(ctx, modeInstruction)
}
