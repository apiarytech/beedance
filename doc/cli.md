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
| `beedance -iec FILE -vm [-cycles N]` with a `CONFIGURATION` | Runs the configuration (package [vmconfig](embedding.md#running-a-configuration-vm)): its tasks, 10 ms a cycle, for N cycles or until Ctrl+C, then prints its globals |
| `beedance -e "TEXT"` | Evaluates a string (with `-vm`, on the VM) |
| `beedance -iec FILE -go OUT.go` | Transpiles to Go |
| `beedance -iec FILE -go OUT.go -go-compact-image` | Transpiles to Go with a process image of only the entries the located variables use, for microcontroller firmware |
| `beedance -iec FILE -embedded OUT.go [-interval D]` | Writes Go that runs the file's first PROGRAM on the VM scan by scan, D apart (default 100 ms), for a microcontroller; build it with TinyGo (see [Microcontrollers](embedded.md#generating-a-plc-program)) |
| `beedance -iec FILE -to-xml OUT.xml` | Exports to PLCopen TC6 XML |
| `beedance -from-xml FILE.xml [-iec OUT.st]` | Imports PLCopen XML to source text (printed, or written to the `-iec` file) |
| `beedance -trace ...` | Prints the parser's trace, with any other flag |
| `beedance -test FILE.st...` | Runs the `PROGRAM TEST_...` unit tests on a simulated clock, on both engines (see [Unit tests](#unit-tests)) |
| `beedance -check-builtins` | Checks the built-in function tables (run from the repository root) |
| `beedance -version` | Prints the version |

## Unit tests

A worked example, with every flag and its output, is in
[examples/unit_tests](../examples/unit_tests/README.md).

```bash
beedance -test [-interval 10ms] [-max-scans 50000] [-engines eval,vm,go,wasm|all] [-tol 1e-3] [-run NAME] [-members outputs|all] [-depth 2] [-csv DIR] [-go-timeout 1m] [-go-replace MOD=DIR] [-wasm-runtime CMD] [-io tcp://HOST:PORT] [-realtime] FILE.st...
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
b(run := TRUE, period := T#2s);
IF b.q THEN done := TRUE; END_IF;
END_PROGRAM
```

A test without `done` runs one scan. With `done` it runs scan by scan,
`-interval` of simulated time apart, until `done` is TRUE; one not done
after `-max-scans` fails. Timers run on the simulated clock, so the test
above takes 201 scans of 10 ms and well under a second. A longer timer
needs a longer `-interval`: at 10 ms, a 2 h period would need 720,001
scans, past `-max-scans`.

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

`-engines all` (or `eval,vm,go`) adds the transpiler: the source is
transpiled to Go on royaljelly, a harness runs each test on the same
simulated clock, and it is built with the Go toolchain in a temporary
module (royaljelly v0.3.1, beebread v0.1.0 for OSCAT; downloaded the first
time). Every engine is compared with the first one listed.
`-go-replace github.com/apiarytech/royaljelly=../royaljelly` builds with a
local checkout instead.

`-engines eval,vm,wasm` runs the transpiled tests as WebAssembly: the same
Go, built for WASI (`GOOS=wasip1`) and run by a WASI runtime, as the
program would run as a WebAssembly target. The runtime is `-wasm-runtime`
(for example `"wasmtime run"`), or else the first of wasmtime, wazero and
Node.js in `PATH`. Node.js must be 24 or later: 20 and 22 at times crash
running a Go WASI module. `all` leaves `wasm` out, as it needs a runtime.

Each scan has a budget of 10 million statements and loop iterations, so a
runaway loop fails its test with "execution budget exceeded". Native Go
has no such count: on the go and wasm engines each test runs in its own
process, stopped after `-go-timeout`.

| Flag | Default | |
|---|---|---|
| `-interval` | `10ms` | simulated time between scans |
| `-max-scans` | `50000` | scans a test with `done` may take |
| `-engines` | `eval,vm` | engines to run on: `eval`, `vm`, `go` (the transpiler), `wasm` (the transpiler to WebAssembly), or `all` (`eval,vm,go`) |
| `-tol` | `1e-3` | relative tolerance for REAL values between engines |
| `-run` | | run only tests whose name contains this text (any case) |
| `-members` | `outputs` | members of function block instances to watch: `outputs`, or `all` (inputs and internal variables too) |
| `-depth` | `2` | levels of instance and structure members watched below a variable |
| `-csv` | | write `TEST_NAME.ENGINE.csv` per test and engine: the watched values after each scan |
| `-go-timeout` | `1m` | time one test may take on the go and wasm engines |
| `-go-replace` | | `MODULE=DIR,...`: build the go and wasm engines with local modules |
| `-wasm-runtime` | wasmtime, wazero or Node.js 24+ in `PATH` | the command that runs a WASI module on the wasm engine, e.g. `"wazero run"` |

A test without `done` gets one scan, 10 ms of simulated time, so its
checks are of that first scan: here, that a 2 s timer has not run out yet.
To check `q` after the timer runs out, give the test `done`, as
`TEST_Blink` does.

```iecst
PROGRAM TEST_BlinkOneScan
VAR_OUTPUT failures : INT; message : STRING; END_VAR
VAR b : Blink; END_VAR
b(run := TRUE, period := T#2s);
IF b.q THEN failures := failures + 1; message := 'q set before 2 s'; END_IF;
END_PROGRAM
```

Run on `TEST_Blink`, a `TEST_Add` that checks `2 + 3 = 5`, and
`TEST_BlinkOneScan`:

```text
beedance: 3 tests

TEST                               EVAL                   VM                     ENGINES
TEST_Blink                         PASS (201 scans)       PASS (201 scans)       agree
TEST_Add                           PASS (1 scans)         PASS (1 scans)         agree
TEST_BlinkOneScan                  PASS (1 scans)         PASS (1 scans)         agree

PASSED: all 3 tests
```

A failing test shows `FAIL (n)`, n being its `failures`, with a line per
engine under it giving the reason: its last `message`, or `not done after
50000 scans` for a test that did not finish. Had `TEST_BlinkOneScan`
expected `q` set (`IF NOT b.q THEN ...`), its one scan would have failed:

```text
TEST_BlinkOneScan                  FAIL (1)               FAIL (1)               agree
    eval: 1 failed checks, last: q not set after 2 s
    vm: 1 failed checks, last: q not set after 2 s
```

Exit status: 0 when all tests pass, 1 when a test fails or the engines
disagree, 2 when a file cannot be read or parsed or holds no tests. The
same runner is a Go API, package `sil` (see
[Embedding](embedding.md#unit-tests-software-in-the-loop)).

`-io tcp://HOST:PORT` connects the tests' located variables to a rig: their
`%I` inputs are read from it before each scan, and their `%Q` outputs are
written to it after. `-realtime` runs scans on the wall clock, which `-io`
implies. See [Hardware in the loop](hil.md).

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
