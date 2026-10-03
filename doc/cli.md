# Command line

```bash
go install github.com/apiarytech/beedance@latest
```

| Command | Does |
|---|---|
| `beedance` | Starts the REPL with the evaluator |
| `beedance -vm` | Starts the REPL with the bytecode VM |
| `beedance -iec FILE` | Runs a source file with the evaluator |
| `beedance -iec FILE -vm` | Compiles the file's first PROGRAM (with the file's function blocks, functions and types) and runs its initialization, then 5 scans 100 ms apart, printing each |
| `beedance -e "TEXT"` | Evaluates a string (with `-vm`, on the VM) |
| `beedance -iec FILE -go OUT.go` | Transpiles to Go |
| `beedance -iec FILE -to-xml OUT.xml` | Exports to PLCopen TC6 XML |
| `beedance -from-xml FILE.xml [-iec OUT.st]` | Imports PLCopen XML to source text (printed, or written to the `-iec` file) |
| `beedance -trace ...` | Prints the parser's trace, with any other flag |
| `beedance -check-builtins` | Checks the built-in function tables (run from the repository root) |
| `beedance -version` | Prints the version |

`-uvm` is reserved for a future microcontroller VM and does nothing yet.

## REPL

The REPL reads a line, parses it, runs it on the chosen engine and prints
the result. Variables persist between lines. Parser errors are printed with
their position and the line is skipped.

```text
>> x := 10 + 20;
30
>> x * 2;
60
```
