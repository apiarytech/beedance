# Backends

All three backends take the same AST and behave the same on the same
program; the test suites run shared programs on each.

## Evaluator

`evaluator.Eval(node, env)` walks the AST and executes it directly.

```go
p := parser.New(lexer.New(source))
program := p.ParseProgram()          // check p.Errors()
env := object.NewEnvironment()
result := evaluator.Eval(program, env)
```

- Runs configurations, resources and tasks, and the standard function
  blocks, with no compile step.
- Statement by statement, with the source positions of the AST, so it is
  the backend for engineering: simulation, tracing, breakpoints and unit
  tests (see [Embedding: engineering sessions](embedding.md#engineering-sessions)).
- The slowest backend: every statement is a tree walk.

## Compiler and VM

The compiler turns a program into bytecode for a stack machine; the VM runs
it.

```go
object.FinalizeBuiltins()            // once, after all packages registered built-ins
prog, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(program, "Pump")

globals := make([]object.Object, vm.GlobalsSize)
init := vm.NewWithGlobalsStore(prog.InitBytecode, globals)
scan := vm.NewWithGlobalsStore(prog.CyclicBytecode, globals)
err = init.Run()                     // once
for range ticker.C {                 // every scan
    scan.Reset()
    err = scan.Run()
}
```

- `CompileProgramUnit(program, name)` compiles one PROGRAM with the
  function blocks, functions and types of its source; other programs in the
  file are left out.
- The **initialization bytecode** declares and initializes; the **cyclic
  bytecode** is the body. Both share the globals slice.
- Located variables live in the VM's I/O image (`VM.IO()`), by address.
- A runtime error (index out of range, division by zero, malformed
  bytecode) is returned by `Run`; the VM never panics into the host.
- No Go toolchain is needed where programs are compiled or run: a program
  is data, so it can be changed online, between two scans, without
  restarting the process.

Limits: stack 2,048 values, 65,536 globals, call depth 1,024.

## Transpiler

The transpiler writes Go source that runs on the
[royaljelly](https://github.com/apiarytech/royaljelly) runtime: each
PROGRAM and FUNCTION_BLOCK becomes a struct with a `Logic(now time.Time)`
method, using royaljelly's IEC types (`iec.DINT`, ...) and standard blocks.

```go
src, err := transpiler.GoFileWith(program, transpiler.Options{
    Package:     "programs", // default "main"
    HostBinding: true,       // New<Program>() and Variables(); see embedding.md
    PreferOSCAT: false,      // OSCAT's version of names the standard also defines
})
```

or from the command line: `beedance -iec program.st -go program.go`.

- Generated code is formatted and imports only what it uses.
- IEC names that Go cannot use are renamed; case-insensitive IEC names map
  to one Go spelling.
- OSCAT BASIC functions and blocks map to the beebread packages.
- The result is compiled with the Go toolchain (or TinyGo) into a native
  binary: the fastest backend, at the cost of a build step and a restart for
  every change.

## Choosing a backend

| | Evaluator | VM | Transpiler |
|---|---|---|---|
| Speed | slowest | about 200× slower than native on arithmetic loops (296 µs against 1.5 µs for 1,000 iterations) | native |
| Build step | none | compile on the target, in process | Go toolchain |
| Change a running program | re-evaluate | swap bytecode between scans | rebuild and restart |
| Debugging | statement by statement, source positions | scan level | Go tools |
| Best for | engineering, tests, REPL | a soft-PLC runtime with 10–100 ms scans | hot paths, devices without a VM |

The VM's speed fits process control scans; the transpiler is the option
where a program needs native speed. A host can offer both and keep the
evaluator for engineering, as beehive does.
