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
	"flag"
	"fmt"
	"os"

	"beedance/parser"
	"beedance/repl"
	"beedance/tui"
)

const version = "0.1.0"

func main() {
	trace := flag.Bool("trace", false, "Enable parser tracing")
	versionFlag := flag.Bool("version", false, "Print the application version")
	simpleREPL := flag.Bool("simple", false, "Run a simple command-line REPL instead of the TUI")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("beedance version %s\n", version)
		os.Exit(0)
	}

	if *simpleREPL {
		fmt.Printf("Beedance IEC 61131-3 Interpreter v%s (Simple REPL)\n", version)
		fmt.Println("Type 'exit' to quit.")
		repl.Start(os.Stdin, os.Stdout)
		os.Exit(0)
	}

	if *trace {
		// Redirect trace output to a file to keep the TUI clean.
		logFile, _ := os.OpenFile("trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		os.Stderr = logFile
		defer logFile.Close()
		parser.SetTracing(true)
	}

	// Launch the main TUI application
	tui.StartTUI()
}
