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
| `beedance -test FILE.st...` | Runs the `PROGRAM TEST_...` unit tests on a simulated clock, on both engines (see [Unit tests](#unit-tests)) |
| `beedance -check-builtins` | Checks the built-in function tables (run from the repository root) |
| `beedance -version` | Prints the version |

`-uvm` is reserved for a future microcontroller VM and does nothing yet.

## Unit tests

```bash
beedance -test [-interval 10ms] [-max-scans 50000] [-engines eval,vm] [-tol 1e-3] [-run NAME] [-members outputs|all] [-depth 2] [-csv DIR] FILE.st...
```

The files are joined into one source, so tests can sit in their own files
beside the function blocks they test. Every `PROGRAM` whose name starts
with `TEST_` (any case) is a test, by the convention beehive's engineering
service uses too:

```iecst
PROGRAM TEST_Blink
VAR_OUTPUT
    failures : INT;    (* checks that failed: 0 to pass *)
    message  : STRING; (* optional: what failed *)
    done     : BOOL;   (* optional: run scan by scan until TRUE *)
END_VAR
VAR b : Blink; END_VAR
b(run := TRUE, period := T#2h);
IF b.q THEN done := TRUE; END_IF;
END_PROGRAM
```

A test without `done` runs one scan. With `done` it runs scan by scan,
`-interval` of simulated time apart, until `done` is TRUE; one not done
after `-max-scans` fails. Timers run on the simulated clock, so the test
above takes 7,201 scans and well under a second.

By default each test runs on the evaluator and the VM, and the two must
agree on every watched value of the test program after every scan (REAL
and LREAL within the relative `-tol`). The watched values are its
variables of elementary types and, under dotted names, the outputs of its
function block instances and the members of its structures: `b.Q`,
`pid.MV`, `pt.x`, two levels deep. With `-members all` an instance's
inputs and internal variables are watched too, e.g. `pid.tSample.ET`
(but not a standard block's internal state). A difference is a beedance
bug: please report it (see [Backends](backends.md)). `-engines eval` or
`-engines vm` runs on one engine only.

Each scan has a budget of 10 million statements and loop iterations, so a
runaway loop fails its test with "execution budget exceeded".

| Flag | Default | |
|---|---|---|
| `-interval` | `10ms` | simulated time between scans |
| `-max-scans` | `50000` | scans a test with `done` may take |
| `-engines` | `eval,vm` | engines to run on |
| `-tol` | `1e-3` | relative tolerance for REAL values between engines |
| `-run` | | run only tests whose name contains this text (any case) |
| `-members` | `outputs` | members of function block instances to watch: `outputs`, or `all` (inputs and internal variables too) |
| `-depth` | `2` | levels of instance and structure members watched below a variable |
| `-csv` | | write `TEST_NAME.ENGINE.csv` per test and engine: the watched values after each scan |

```text
beedance: 2 tests

TEST                               EVAL                   VM                     ENGINES
TEST_Add                           PASS (1 scans)         PASS (1 scans)         agree
TEST_Blink                         FAIL (1)               FAIL (1)               agree
    eval: 1 failed checks, last: q not set after 2 h
    vm: 1 failed checks, last: q not set after 2 h

FAILED: 1 of 2 tests
```

Exit status: 0 when all tests pass, 1 when a test fails or the engines
disagree, 2 when a file cannot be read or parsed or holds no tests. The
same runner is a Go API, package `sil` (see
[Embedding](embedding.md#unit-tests-software-in-the-loop)).

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
