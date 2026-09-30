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
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"beedance/ast"
	"beedance/compiler"
	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"beedance/plcopen"
	"beedance/repl"
	_ "beedance/stdlib" // Import for side-effect of registering built-ins
	"beedance/transpiler"
	"beedance/vm"
)

const version = "0.1.0"

func main() {
	// Finalize the list of built-in functions after all packages have been initialized.
	object.FinalizeBuiltins()

	trace := flag.Bool("trace", false, "Enable parser tracing")
	versionFlag := flag.Bool("version", false, "Print the application version")
	iecFile := flag.String("iec", "", "Path to an IEC 61131-3 source file to execute")
	evalStr := flag.String("e", "", "A string of IEC 61131-3 text to evaluate")
	vmFlag := flag.Bool("vm", false, "Use the virtual machine instead of the evaluator")
	uvmFlag := flag.Bool("uvm", false, "Use the universal microcontroller virtual machine instead of the evaluator")
	goFile := flag.String("go", "", "Path to the output Go file for transpilation from an -iec file")
	toXMLFile := flag.String("to-xml", "", "Path to output PLCopen TC6 XML file converted from an -iec file")
	fromXMLFile := flag.String("from-xml", "", "Path to input PLCopen TC6 XML file to convert to IEC 61131-3 text")
	checkBuiltinsFlag := flag.Bool("check-builtins", false, "Run the built-in function consistency checker")
	flag.Parse()

	if *checkBuiltinsFlag {
		checkBuiltins()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("beedance version %s\n", version)
		os.Exit(0)
	}

	var engine string
	if *vmFlag {
		engine = "vm"
	} else if *uvmFlag {
		engine = "uvm"
	} else {
		engine = "eval"
	}

	if *trace {
		parser.SetTracing(true)
		fmt.Println("Parser tracing enabled.")
	}

	if *goFile != "" {
		if *iecFile == "" {
			fmt.Fprintln(os.Stderr, "The -iec flag must be provided with the -go flag to specify the input file.")
			os.Exit(1)
		}
		transpileFile(*iecFile, *goFile, os.Stdout)
		os.Exit(0)
	}

	if *toXMLFile != "" {
		if *iecFile == "" {
			fmt.Fprintln(os.Stderr, "The -iec flag must be provided with the -to-xml flag to specify the input file.")
			os.Exit(1)
		}
		convertIECToXML(*iecFile, *toXMLFile, os.Stdout)
		os.Exit(0)
	}

	if *fromXMLFile != "" {
		convertXMLToIEC(*fromXMLFile, *iecFile, os.Stdout)
		os.Exit(0)
	}

	if *iecFile != "" {
		executeFile(*iecFile, os.Stdout, engine)
		os.Exit(0)
	}

	if *evalStr != "" {
		executeString(*evalStr, os.Stdout, engine)
		os.Exit(0)
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! This is the Beedance (IEC 61131) programming language!\n",
		user.Username)
	fmt.Printf("Feel free to type in commands. Using %s engine.\n", engine)
	repl.Start(os.Stdin, os.Stdout, engine)
}

func executeFile(filepath string, out io.Writer, engine string) {
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

	if engine == "vm" {
		symbolTable := compiler.NewSymbolTable()
		for _, v := range object.Builtins {
			symbolTable.DefineBuiltin(v.Index, v.Name)
		}
		globals := make([]object.Object, vm.GlobalsSize)

		comp := compiler.NewWithState(symbolTable, nil, nil, nil)

		// Since we are executing a file, which is likely a full PROGRAM,
		// we use the new CompileProgram function to get separated bytecode.
		programDecl, ok := program.Statements[0].(*ast.ProgramDeclaration)
		if !ok {
			fmt.Fprintln(out, "Error: IEC file does not contain a valid PROGRAM declaration.")
			return
		}

		compiledProg, err := comp.CompileProgram(programDecl)
		if err != nil {
			fmt.Fprintf(out, "Woops! Compilation failed:\n %s\n", err)
			return
		}

		// --- Basic Scheduler Loop ---
		// 1. Run initialization code once.
		initVM := vm.NewWithGlobalsStore(compiledProg.InitBytecode, globals)
		if err := initVM.Run(); err != nil {
			fmt.Fprintf(out, "Woops! Executing initialization bytecode failed:\n %s\n", err)
			return
		}

		// 2. Run cyclic code in a loop (simulating a PLC scan).
		// For this example, we'll just run it a few times.
		fmt.Fprintln(out, "--- Starting simulated PLC scan (5 cycles) ---")
		for i := 0; i < 5; i++ {
			cyclicVM := vm.NewWithGlobalsStore(compiledProg.CyclicBytecode, globals)
			if err := cyclicVM.Run(); err != nil {
				fmt.Fprintf(out, "Woops! Executing cyclic bytecode failed on cycle %d:\n %s\n", i+1, err)
				return
			}
			globals = cyclicVM.Globals() // Update globals with the state from the completed cycle
			fmt.Fprintf(out, "Cycle %d complete. Last popped value: %s\n", i+1, cyclicVM.LastPoppedStackElem().Inspect())
			time.Sleep(100 * time.Millisecond) // Simulate scan time
		}
	} else if engine == "uvm" {
		// UVM execution logic goes here
	} else {
		env := object.NewEnvironment()
		evaluated := evaluator.Eval(program, env)
		io.WriteString(out, evaluated.Inspect()+"\n")
	}
}

func executeString(input string, out io.Writer, engine string) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return
	}

	if engine == "vm" {
		symbolTable := compiler.NewSymbolTable()
		for _, v := range object.Builtins {
			symbolTable.DefineBuiltin(v.Index, v.Name)
		}
		globals := make([]object.Object, vm.GlobalsSize)

		comp := compiler.NewWithState(symbolTable, nil, nil, nil)
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(out, "Woops! Compilation failed:\n %s\n", err)
			return
		}

		machine := vm.NewWithGlobalsStore(comp.Bytecode(), globals)
		err = machine.Run()
		if err != nil {
			fmt.Fprintf(out, "Woops! Executing bytecode failed:\n %s\n", err)
			return
		}

		lastPopped := machine.LastPoppedStackElem()
		io.WriteString(out, lastPopped.Inspect()+"\n")
	} else {
		env := object.NewEnvironment()
		evaluated := evaluator.Eval(program, env)
		io.WriteString(out, evaluated.Inspect()+"\n")
	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "Parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func transpileFile(inputFile, outputFile string, out io.Writer) {
	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Fprintf(out, "Error opening input file: %s\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var input string
	for scanner.Scan() {
		input += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(out, "Error reading input file: %s\n", err)
		return
	}

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return
	}

	// Transpile to a complete Go file that imports the royaljelly packages
	// the generated code uses.
	formatted, err := transpiler.GoFile(program)
	if err != nil {
		fmt.Fprintf(out, "Transpilation error: %s\n", err)
		return
	}

	// Write the code to the output file.
	outFile, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(out, "Error creating output file: %s\n", err)
		return
	}
	defer outFile.Close()

	_, err = outFile.Write(formatted)
	if err != nil {
		fmt.Fprintf(out, "Error writing to output file: %s\n", err)
		return
	}

	fmt.Fprintf(out, "Successfully transpiled %s to %s\n", inputFile, outputFile)

	// Automatically run go mod tidy on the output file's directory
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = filepath.Dir(outputFile)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "Warning: 'go mod tidy' failed: %s\n", err)
	}
}

func convertIECToXML(inputFile, outputFile string, out io.Writer) {
	if err := plcopen.ConvertIECToXMLFile(inputFile, outputFile); err != nil {
		fmt.Fprintf(out, "Error converting IEC to XML: %s\n", err)
		return
	}
	fmt.Fprintf(out, "Successfully converted %s to PLCopen XML %s\n", inputFile, outputFile)
}

func convertXMLToIEC(xmlFile, outputFile string, out io.Writer) {
	iecText, err := plcopen.ConvertXMLToIECText(xmlFile)
	if err != nil {
		fmt.Fprintf(out, "Error converting XML to IEC: %s\n", err)
		return
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(iecText), 0644); err != nil {
			fmt.Fprintf(out, "Error writing IEC output file: %s\n", err)
			return
		}
		fmt.Fprintf(out, "Successfully converted %s to IEC text %s\n", xmlFile, outputFile)
	} else {
		out.Write([]byte(iecText))
	}
}
