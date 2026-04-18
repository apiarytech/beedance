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
	"fmt"
	"io"

	"beedance/lexer"
	"beedance/parser"

	"beedance/evaluator"
	"beedance/object"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()
	macroEnv := object.NewEnvironment()

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

		evaluator.DefineMacros(program, macroEnv)
		expanded := evaluator.ExpandMacros(program, macroEnv)

		evaluated := evaluator.Eval(expanded, env)
		if evaluated != nil {
			io.WriteString(out, evaluated.Inspect())
			io.WriteString(out, "\n")
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
	io.WriteString(out, " parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
