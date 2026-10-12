/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command gen compiles program.st to bytecode and writes it as the Go
// package program (embedded.GenerateEmbeddedProgram), scans 10 ms apart:
//
//	go generate ./embedded/esp32hw/plc/       # from the module root
//	go run ./gen [-dir DIR] [-check]           # from plc
//
// -check writes nothing and fails when program/program.go is not what
// program.st gives.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/embedded"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	_ "github.com/apiarytech/beedance/stdlib" // the built-in functions
)

func main() {
	dir := flag.String("dir", ".", "the plc directory")
	check := flag.Bool("check", false, "fail if program/program.go is out of date")
	flag.Parse()
	if err := run(*dir, *check); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(dir string, check bool) error {
	src, err := os.ReadFile(filepath.Join(dir, "program.st"))
	if err != nil {
		return err
	}
	object.FinalizeBuiltins()
	p := parser.New(lexer.New(string(src)))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return fmt.Errorf("program.st: %s", strings.Join(errs, "; "))
	}
	cp, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, "Main")
	if err != nil {
		return fmt.Errorf("program.st: %v", err)
	}
	var out bytes.Buffer
	if err := embedded.GenerateEmbeddedProgram(&out, cp, embedded.ProgramOptions{Package: "program", Interval: 10 * time.Millisecond, StackSize: 256, MaxFrames: 32}); err != nil {
		return err
	}
	code, err := format.Source(out.Bytes())
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "program", "program.go")
	if check {
		have, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n")), code) {
			return fmt.Errorf("%s is out of date: go generate ./embedded/esp32hw/plc/", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, code, 0o644)
}
