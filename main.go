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
	"os/user"

	"beedance/parser"
	"beedance/repl"
)

const version = "0.1.0"

func main() {
	trace := flag.Bool("trace", false, "Enable parser tracing")
	versionFlag := flag.Bool("version", false, "Print the application version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("beedance version %s\n", version)
		os.Exit(0)
	}

	if *trace {
		parser.SetTracing(true)
		fmt.Println("Parser tracing enabled.")
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! This is the Beedance (IEC 61131) programming language!\n",
		user.Username)
	fmt.Printf("Feel free to type in commands\n")
	repl.Start(os.Stdin, os.Stdout)
}
