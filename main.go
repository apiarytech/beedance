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
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"

	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"beedance/repl"
)

const version = "0.1.0"

func main() {
	trace := flag.Bool("trace", false, "Enable parser tracing")
	versionFlag := flag.Bool("version", false, "Print the application version")
	iecFile := flag.String("iec", "", "Path to an IEC 61131-3 source file to execute")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("beedance version %s\n", version)
		os.Exit(0)
	}

	if *trace {
		parser.SetTracing(true)
		fmt.Println("Parser tracing enabled.")
	}

	if *iecFile != "" {
		executeFile(*iecFile, os.Stdout)
		os.Exit(0)
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

func executeFile(filepath string, out io.Writer) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Fprintf(out, "Error opening file: %s\n", err)
		return
	}
	defer file.Close()

	// Read the entire file content
	scanner := bufio.NewScanner(file)
	var input string
	for scanner.Scan() {
		input += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(out, "Error reading file: %s\n", err)
		return
	}

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return
	}

	env := object.NewEnvironment()
	evaluated := evaluator.Eval(program, env)

	io.WriteString(out, evaluated.Inspect()+"\n")
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "Parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
