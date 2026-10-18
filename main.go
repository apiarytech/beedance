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
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/apiarytech/beedance/analyzer"
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/compiler"
	"github.com/apiarytech/beedance/dap"
	"github.com/apiarytech/beedance/embedded"
	"github.com/apiarytech/beedance/evaluator"
	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/plcopen"
	"github.com/apiarytech/beedance/repl"
	"github.com/apiarytech/beedance/sil"
	_ "github.com/apiarytech/beedance/stdlib" // Import for side-effect of registering built-ins
	"github.com/apiarytech/beedance/transpiler"
	"github.com/apiarytech/beedance/vm"
	"github.com/apiarytech/beedance/watch"
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
	allowInputWritesFlag := flag.Bool("allow-input-writes", false, "Let a POU assign its own VAR_INPUT variables, as CODESYS does and libraries written for it need (OSCAT; IEC-69), on every engine: -test, -vm, -go, the evaluator and configurations. Off, IEC 61131-3's rule, such a source is refused")
	embeddedFile := flag.String("embedded", "", "Path to the output Go file that runs the first PROGRAM of an -iec file on the VM, scan by scan, for a microcontroller (TinyGo); -interval sets its scan time")
	goFile := flag.String("go", "", "Path to the output Go file for transpilation from an -iec file")
	goCompactImage := flag.Bool("go-compact-image", false, "With -go: give located variables a process image of only the entries they use, instead of royaljelly's vars.ProcessImage (about 33 KB on a 32-bit microcontroller)")
	toXMLFile := flag.String("to-xml", "", "Path to output PLCopen TC6 XML file converted from an -iec file")
	fromXMLFile := flag.String("from-xml", "", "Path to input PLCopen TC6 XML file to convert to IEC 61131-3 text")
	checkBuiltinsFlag := flag.Bool("check-builtins", false, "Run the built-in function consistency checker")
	testFlag := flag.Bool("test", false, "Run the PROGRAM TEST_... unit tests of the FILE.st arguments on a simulated clock")
	interval := flag.Duration("interval", 10*time.Millisecond, "With -test: simulated time between scans")
	lintFlag := flag.Bool("lint", false, "Check the FILE.st arguments with the static analyzer: unused variables, unreachable code, narrowing, multiple writes, unbounded loops, the PLCopen coding guidelines")
	lintLevel := flag.String("lint-level", "warning", "With -lint: the least severity to report: info, warning or error")
	lintDisable := flag.String("lint-disable", "", "With -lint: rules not to report, by ID or PLCopen rule, e.g. if-without-else,CP19")
	lintMaxIterations := flag.Int64("lint-max-iterations", analyzer.DefaultMaxIterations, "With -lint: the iterations a scan may make of a loop and those around it before loop-iterations reports it")
	lintRules := flag.Bool("lint-rules", false, "List the static analyzer's rules")
	flag.IntVar(&configCycles, "cycles", 0, "With -vm on a file with a CONFIGURATION: the cycles to run, 10 ms apart; 0 runs until interrupted")
	maxScans := flag.Int("max-scans", 50000, "With -test: a test with done that is not done by then fails")
	enginesFlag := flag.String("engines", "eval,vm", "With -test: engines to run the tests on: eval, vm, go (the transpiler), wasm (the transpiler to WebAssembly, on a WASI runtime) or all (eval,vm,go); compared with the first")
	tol := flag.Float64("tol", 1e-3, "With -test: relative tolerance when comparing REAL values between engines")
	runFilter := flag.String("run", "", "With -test: run only tests whose name contains this text")
	csvDir := flag.String("csv", "", "With -test: write each test's per-scan values per engine as CSV into this directory")
	membersFlag := flag.String("members", "outputs", "With -test: members of function block instances to record and compare: outputs or all")
	depth := flag.Int("depth", 2, "With -test: levels of instance and structure members to record below a variable")
	goTimeout := flag.Duration("go-timeout", time.Minute, "With -test on the go and wasm engines: time one test may take")
	goReplace := flag.String("go-replace", "", "With -test on the go and wasm engines: MODULE=DIR,... builds with local modules, e.g. github.com/apiarytech/royaljelly=../royaljelly")
	wasmRuntime := flag.String("wasm-runtime", "", "With -test on the wasm engine: the command that runs a WASI module, e.g. \"wasmtime run\" (default: wasmtime, wazero or Node.js 24+, the first in PATH)")
	ioFlag := flag.String("io", "", "With -test: connect located variables (%I, %Q) to a rig over the rig protocol at tcp://HOST:PORT or serial:PORT[?baud=N] (/dev/ttyACM0, COM3): hardware in the loop; implies -realtime")
	realTime := flag.Bool("realtime", false, "With -test: run scans -interval apart on the wall clock instead of a simulated clock")
	dapFlag := flag.Bool("dap", false, "Serve the Debug Adapter Protocol on stdin and stdout, for VS Code and other editors: debug a PROGRAM on the VM, scan by scan (see doc/debugging.md)")
	dapLog := flag.String("dap-log", "", "With -dap: write every message of the protocol to this file")
	flag.Parse()
	// IEC-69: input writes, CODESYS's dialect, on every engine.
	allowInputWrites = *allowInputWritesFlag
	evaluator.SetAllowInputWrites(allowInputWrites)

	if *dapFlag {
		// stdout is the protocol's: nothing else may write to it.
		server := dap.NewServer(os.Stdin, os.Stdout)
		if *dapLog != "" {
			f, err := os.Create(*dapLog)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			defer f.Close()
			server.Log = f
		}
		if err := server.Serve(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if *testFlag {
		engines, err := sil.ParseEngines(*enginesFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		opts := sil.Options{Interval: *interval, MaxScans: *maxScans, Tolerance: *tol, Engines: engines, Filter: *runFilter, AllowInputWrites: *allowInputWritesFlag,
			Watch: watch.Options{Depth: *depth}, GoTimeout: *goTimeout, GoReplace: map[string]string{}, WasmRuntime: strings.Fields(*wasmRuntime)}
		// Real I/O runs on the wall clock unless -realtime=false says not.
		opts.RealTime = *realTime || (*ioFlag != "" && !flagSet("realtime"))
		for _, r := range strings.Split(*goReplace, ",") {
			if r = strings.TrimSpace(r); r == "" {
				continue
			}
			mod, dir, ok := strings.Cut(r, "=")
			if !ok {
				fmt.Fprintf(os.Stderr, "-go-replace %q: want MODULE=DIR\n", r)
				os.Exit(2)
			}
			opts.GoReplace[strings.TrimSpace(mod)] = strings.TrimSpace(dir)
		}
		switch strings.ToLower(*membersFlag) {
		case "outputs":
		case "all":
			opts.Watch.Members = watch.All
		default:
			fmt.Fprintf(os.Stderr, "-members %q: want outputs or all\n", *membersFlag)
			os.Exit(2)
		}
		if *ioFlag != "" {
			rig, err := sil.OpenRig(context.Background(), *ioFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			opts.IO = rig
			status := runTests(flag.Args(), opts, *csvDir, os.Stdout, os.Stderr)
			rig.Close()
			os.Exit(status)
		}
		os.Exit(runTests(flag.Args(), opts, *csvDir, os.Stdout, os.Stderr))
	}

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
	} else {
		engine = "eval"
	}

	if *trace {
		parser.SetTracing(true)
		fmt.Println("Parser tracing enabled.")
	}

	if *lintRules {
		listLintRules(os.Stdout)
		os.Exit(0)
	}

	if *lintFlag {
		level, err := analyzer.ParseSeverity(*lintLevel)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		opts := analyzer.Options{Disable: lintDisabled(*lintDisable), MaxIterations: *lintMaxIterations}
		os.Exit(runLint(flag.Args(), opts, level, os.Stdout, os.Stderr))
	}

	if *goFile != "" {
		if *iecFile == "" {
			fmt.Fprintln(os.Stderr, "The -iec flag must be provided with the -go flag to specify the input file.")
			os.Exit(1)
		}
		transpileFile(*iecFile, *goFile, transpiler.Options{CompactImage: *goCompactImage, AllowInputWrites: *allowInputWritesFlag}, os.Stdout)
		os.Exit(0)
	}

	if *embeddedFile != "" {
		if *iecFile == "" {
			fmt.Fprintln(os.Stderr, "The -iec flag must be provided with the -embedded flag to specify the input file.")
			os.Exit(1)
		}
		// The generator's own scan time, unless -interval is given.
		var scan time.Duration
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "interval" {
				scan = *interval
			}
		})
		if err := generateEmbedded(*iecFile, *embeddedFile, embedded.ProgramOptions{Interval: scan}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
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

	// A CONFIGURATION runs its tasks and programs; see configuration_run.go.
	if engine == "vm" && hasConfiguration(program) {
		runConfiguration(input, out)
		return
	}
	if engine == "vm" {
		symbolTable := compiler.NewSymbolTable()
		for _, v := range object.Builtins {
			symbolTable.DefineBuiltin(v.Index, v.Name)
		}
		globals := make([]object.Object, vm.GlobalsSize)

		comp := compiler.NewWithState(symbolTable, nil, nil, nil)
		comp.AllowInputWrites = allowInputWrites // IEC-69

		// Since we are executing a file, which is likely a full PROGRAM,
		// we use the new CompileProgram function to get separated bytecode.
		// The first PROGRAM in the file, compiled with the function blocks,
		// functions and types the file declares.
		var programDecl *ast.ProgramDeclaration
		for _, s := range program.Statements {
			if pd, ok := s.(*ast.ProgramDeclaration); ok {
				programDecl = pd
				break
			}
		}
		if programDecl == nil {
			fmt.Fprintln(out, "Error: IEC file does not contain a valid PROGRAM declaration.")
			return
		}

		compiledProg, err := comp.CompileProgramUnit(program, programDecl.Name.Value)
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
		comp.AllowInputWrites = allowInputWrites // IEC-69
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

// generateEmbedded writes outputFile, Go that runs the first PROGRAM of
// inputFile on the VM as a PLC does (embedded.GenerateEmbeddedProgram), for
// a microcontroller: build it with TinyGo (see doc/embedded.md).
func generateEmbedded(inputFile, outputFile string, opts embedded.ProgramOptions) error {
	src, err := os.ReadFile(inputFile)
	if err != nil {
		return err
	}
	p := parser.New(lexer.New(string(src)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return fmt.Errorf("parser errors:\n\t%s", strings.Join(p.Errors(), "\n\t"))
	}
	var name string
	for _, s := range program.Statements {
		if pd, ok := s.(*ast.ProgramDeclaration); ok {
			name = pd.Name.Value
			break
		}
	}
	if name == "" {
		return fmt.Errorf("%s has no PROGRAM", inputFile)
	}
	comp := compiler.NewCompilerWithBuiltins(object.Builtins)
	comp.AllowInputWrites = allowInputWrites // IEC-69
	prog, err := comp.CompileProgramUnit(program, name)
	if err != nil {
		return err
	}
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	if err := embedded.GenerateEmbeddedProgram(f, prog, opts); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func transpileFile(inputFile, outputFile string, opts transpiler.Options, out io.Writer) {
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
	formatted, err := transpiler.GoFileWith(program, opts)
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
