/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package repl

import (
	"bufio"
	"flag"
	"fmt"
	"io"

	"beedance/compiler"
	"beedance/lexer"
	"beedance/parser"

	"beedance/evaluator"
	"beedance/object"
	"beedance/vm"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer, engine string) {
	if flag.Lookup("go").Value.String() != "" {
		io.WriteString(out, "Transpilation to Go (-go) is not supported in REPL mode.\n")
		io.WriteString(out, "Please use it with an input file: beedance -iec <input.st> -go <output.go>\n")
	}

	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()
	macroEnv := object.NewEnvironment()

	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)

	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	for {
		fmt.Fprintf(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		if engine == "vm" {
			comp := compiler.NewWithState(symbolTable, constants)
			err := comp.Compile(program)
			if err != nil {
				fmt.Fprintf(out, "Woops! Compilation failed:\n %s\n", err)
				continue
			}

			code := comp.Bytecode()
			constants = code.Constants

			machine := vm.NewWithGlobalsStore(code, globals)
			err = machine.Run()
			if err != nil {
				fmt.Fprintf(out, "Woops! Executing bytecode failed:\n %s\n", err)
				continue
			}

			lastPopped := machine.LastPoppedStackElem()
			io.WriteString(out, lastPopped.Inspect())
			io.WriteString(out, "\n")
		} else {
			evaluator.DefineMacros(program, macroEnv)
			expanded := evaluator.ExpandMacros(program, macroEnv)
			evaluated := evaluator.Eval(expanded, env)
			if evaluated != nil {
				io.WriteString(out, evaluated.Inspect())
				io.WriteString(out, "\n")
			}
		}
	}
}

const beedance_FACE = `
              \   /
               [ ]
             __/_\__
            / _   _ \   *
     .     | [o] [o] |       +
    _______ \   =   / _______
   /|--o-- \ \_____/ / --o--|\
  ( |  |    \/[###]\/    |  | )
   \|--+----/[#####]\----+--|/
    \______/[#######]\______/
     /_ / _/[#######]\_ \ _\
    /  /  /  \[###]/  \  \  \
    +         \___/          .
               [|]
`

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, beedance_FACE)
	io.WriteString(out, " Beedance! parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
