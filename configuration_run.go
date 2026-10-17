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

// beedance -vm -iec FILE runs a file's CONFIGURATION on the VM (package
// vmconfig): its tasks on the wall clock, 10 ms a cycle, for -cycles
// cycles or until interrupted, then prints its globals.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/vmconfig"
)

// configCycles is -cycles: the cycles a configuration runs, 0 for until
// interrupted.
var configCycles int

// allowInputWrites is -allow-input-writes (IEC-69), for the VM runs and
// configurations; the evaluator takes it from evaluator.SetAllowInputWrites.
var allowInputWrites bool

// configurationPeriod is the time between two cycles of a configuration.
const configurationPeriod = 10 * time.Millisecond

// hasConfiguration reports whether program declares a CONFIGURATION.
func hasConfiguration(program *ast.Program) bool {
	for _, s := range program.Statements {
		if _, ok := s.(*ast.ConfigurationDeclaration); ok {
			return true
		}
	}
	return false
}

// runConfiguration loads source's CONFIGURATION and runs it.
func runConfiguration(source string, out io.Writer) {
	c, err := vmconfig.LoadSourceWith(source, vmconfig.LoadOptions{AllowInputWrites: allowInputWrites})
	if err != nil {
		fmt.Fprintf(out, "Woops! Loading the configuration failed:\n %s\n", err)
		return
	}
	fmt.Fprintf(out, "--- Running configuration %s on the VM ---\n", c.Name)
	if configCycles > 0 {
		for i := 0; i < configCycles; i++ {
			if err := c.Cycle(time.Duration(i) * configurationPeriod); err != nil {
				fmt.Fprintf(out, "Woops! Cycle %d failed:\n %s\n", i+1, err)
				return
			}
		}
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := c.Run(ctx, configurationPeriod); err != nil {
			fmt.Fprintf(out, "Woops! %s\n", err)
			return
		}
	}
	names := c.GlobalNames()
	sort.Strings(names)
	for _, name := range names {
		v, _ := c.Global(name)
		if v != nil {
			fmt.Fprintf(out, "%s = %s\n", name, v.Inspect())
		}
	}
}
