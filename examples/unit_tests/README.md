# Unit tests for Structured Text: `beedance -test`

`beedance -test` runs unit tests written in Structured Text, software in
the loop: each test program runs scan by scan on a **simulated clock**, on
one or more of beedance's engines, and the engines are compared with each
other after every scan. It needs no PLC, no runtime and no other tool, and
its exit status suits CI.

| Engine | `-engines` | What runs the test |
|---|---|---|
| Evaluator | `eval` | the tree-walking interpreter |
| Compiler + VM | `vm` | the bytecode compiler and virtual machine |
| Transpiler | `go` | the source transpiled to Go on royaljelly, built with the Go toolchain |

The default is `-engines eval,vm`. `-engines all` runs all three.

## The files

| File | What it is |
|---|---|
| [motor.st](motor.st) | The code under test: `MotorStarter`, a function block with a start delay, and `Scale`, a function |
| [motor_tests.st](motor_tests.st) | Its tests, which pass on every engine |
| [failing.st](failing.st) | Tests that fail on purpose, one of each kind of failure |
| [precision.st](precision.st) | REAL precision across engines, and `-tol` |
| [unit_tests_test.go](unit_tests_test.go) | The same tests run from Go with package `sil`, the API behind `-test` |

Run the commands below from this directory. Install `beedance` with
`go install github.com/apiarytech/beedance@latest`, or replace `beedance`
with `go run ../..`.

## Writing a test

A test is a `PROGRAM` whose name starts with `TEST_` (any case). It may use
everything the other files declare. Its outputs say how it went:

```iecst
PROGRAM TEST_MotorStartsAfterDelay
VAR_OUTPUT
    failures : INT;     (* checks that failed: 0 to pass. Required *)
    message  : STRING;  (* optional: what failed *)
    done     : BOOL;    (* optional: run scan by scan until TRUE *)
END_VAR
VAR m : MotorStarter; scans : INT; END_VAR
scans := scans + 1;
m(start := TRUE, delay := T#5s);
IF scans = 400 AND m.run THEN    (* 3.99 s at 10 ms a scan *)
    failures := failures + 1; message := 'ran before the delay';
END_IF;
done := m.run;
END_PROGRAM
```

- Without `done`, a test runs **one scan**: right for functions and
  calculations (`TEST_Scale`).
- With `done`, it runs **scan by scan** until `done` is TRUE. Each scan is
  `-interval` of simulated time after the last (10 ms by default), so
  timers run as on a PLC, only faster: `TEST_LongDelay` waits 5 minutes of
  simulated time in 30,001 scans, well under a second.
- A test passes when, on every engine, it runs without error, is done,
  and `failures` is 0, and the engines agree.

## Examples

### 1. Run the tests (evaluator and VM)

```text
$ beedance -test motor.st motor_tests.st
beedance: 4 tests

TEST                               EVAL                   VM                     ENGINES
TEST_Scale                         PASS (1 scans)         PASS (1 scans)         agree
TEST_MotorStartsAfterDelay         PASS (501 scans)       PASS (501 scans)       agree
TEST_MotorStopsOnFault             PASS (20 scans)        PASS (20 scans)        agree
TEST_LongDelay                     PASS (30001 scans)     PASS (30001 scans)     agree

PASSED: all 4 tests
```

The files are joined into one source, so tests can sit in their own files
next to the code they test. List them in any order.

### 2. Run on all three engines

```text
$ beedance -test -engines all motor.st motor_tests.st
beedance: 4 tests

TEST                               EVAL                   VM                     GO                     ENGINES
TEST_Scale                         PASS (1 scans)         PASS (1 scans)         PASS (1 scans)         agree
TEST_MotorStartsAfterDelay         PASS (501 scans)       PASS (501 scans)       PASS (501 scans)       agree
TEST_MotorStopsOnFault             PASS (20 scans)        PASS (20 scans)        PASS (20 scans)        agree
TEST_LongDelay                     PASS (30001 scans)     PASS (30001 scans)     PASS (30001 scans)     agree

PASSED: all 4 tests
```

`go` transpiles the source to Go, adds a harness that runs each test on
the same simulated clock, and builds it in a temporary module with
royaljelly v0.3.1 (and beebread v0.1.0 when OSCAT functions are used). It
needs the Go toolchain, and downloads those modules the first time; after
that a run takes a few seconds. `all` is the same as `eval,vm,go`.

### 3. Run on one engine

```text
$ beedance -test -engines vm motor.st motor_tests.st
beedance: 4 tests

TEST                               VM
TEST_Scale                         PASS (1 scans)
TEST_MotorStartsAfterDelay         PASS (501 scans)
TEST_MotorStopsOnFault             PASS (20 scans)
TEST_LongDelay                     PASS (30001 scans)

PASSED: all 4 tests
```

`-engines eval` and `-engines go` work the same way. With one engine
there is nothing to compare, so no ENGINES column: use it to test the
engine your target runs, e.g. `vm` for a soft-PLC, `go` for a native build.

### 4. Choose and order engines

```text
$ beedance -test -engines vm,go motor.st motor_tests.st
beedance: 4 tests

TEST                               VM                     GO                     ENGINES
TEST_Scale                         PASS (1 scans)         PASS (1 scans)         agree
...
```

Every engine is compared with the **first** one listed, so the first is
the reference a difference is reported against. `evaluator` and
`transpiler` are accepted as names for `eval` and `go`.

### 5. Run some tests: `-run`

```text
$ beedance -test -run motor motor.st motor_tests.st
beedance: 2 tests

TEST                               EVAL                   VM                     ENGINES
TEST_MotorStartsAfterDelay         PASS (501 scans)       PASS (501 scans)       agree
TEST_MotorStopsOnFault             PASS (20 scans)        PASS (20 scans)        agree

PASSED: all 2 tests
```

`-run` keeps the tests whose name contains the text, in any case.

### 6. Simulated time: `-interval` and `-max-scans`

A coarser scan reaches long delays in fewer scans:

```text
$ beedance -test -run longdelay -interval 1s motor.st motor_tests.st
TEST                               EVAL                   VM                     ENGINES
TEST_LongDelay                     PASS (301 scans)       PASS (301 scans)       agree
```

A test with `done` that is not done after `-max-scans` (50,000 by
default) fails:

```text
$ beedance -test -run longdelay -max-scans 1000 motor.st motor_tests.st
TEST                               EVAL                   VM                     ENGINES
TEST_LongDelay                     FAIL (0)               FAIL (0)               agree
    eval: not done after 1000 scans
    vm: not done after 1000 scans

FAILED: 1 of 1 tests
```

A test that counts scans to time its checks (as `TEST_MotorStartsAfterDelay`
does with scan 400 = 3.99 s) assumes an interval, so run it with the
interval it was written for. To test a delay of hours, use
`-interval 1s`: a 2-hour delay is then 7,201 scans.

### 7. How failures are reported

[failing.st](failing.st) fails in each possible way:

```text
$ beedance -test -engines all -go-timeout 5s -max-scans 2000 motor.st failing.st
beedance: 4 tests

TEST                               EVAL                   VM                     GO                     ENGINES
TEST_WrongExpectation              FAIL (1)               FAIL (1)               FAIL (1)               agree
    eval: 1 failed checks, last: expected 99 %
    vm: 1 failed checks, last: expected 99 %
    go: 1 failed checks, last: expected 99 %
TEST_NeverDone                     FAIL (0)               FAIL (0)               FAIL (0)               agree
    eval: not done after 2000 scans
    vm: not done after 2000 scans
    go: not done after 2000 scans
TEST_RunawayLoop                   ERROR                  ERROR                  ERROR                  -
    eval: scan 1: ERROR: ERROR (71:7): execution budget exceeded
    vm: scan 1: vm: execution budget exceeded
    go: no result after 5s: a runaway loop? (see -go-timeout)
TEST_NoFailuresOutput              ERROR                  ERROR                  ERROR                  -
    eval: a test program needs VAR_OUTPUT failures : INT
    vm: a test program needs VAR_OUTPUT failures : INT
    go: a test program needs VAR_OUTPUT failures : INT

FAILED: 4 of 4 tests
```

- `FAIL (n)`: the test ran and `failures` was `n`, or it was not done.
- `ERROR`: it could not run to the end: a runtime error, a runaway loop,
  or a program that is not a test by the convention.
- The evaluator and the VM stop a runaway loop with a budget of 10 million
  statements and loop iterations per scan. Native Go has no such count:
  on `go` each test runs in its own process, stopped after `-go-timeout`
  (one minute by default).
- `-` in ENGINES: an engine did not finish, so not every engine could be
  compared.

### 8. When engines disagree: `DIFFER` and `-tol`

After every scan, each engine's watched values are compared with the first
engine's. A difference fails the test, even when every engine passes its
own checks: it is a beedance bug, or a real difference between targets.

REAL is one such difference: the evaluator and the VM hold a REAL in 64
bits, transpiled Go in 32. [precision.st](precision.st) computes 100/3:

```text
$ beedance -test -engines all motor.st precision.st
TEST                               EVAL                   VM                     GO                     ENGINES
TEST_RealPrecision                 PASS (1 scans)         PASS (1 scans)         PASS (1 scans)         agree

$ beedance -test -engines all -tol 1e-9 motor.st precision.st
TEST                               EVAL                   VM                     GO                     ENGINES
TEST_RealPrecision                 PASS (1 scans)         PASS (1 scans)         PASS (1 scans)         DIFFER: scan 1 third: eval 33.333333, go 33.333332

FAILED: 1 of 1 tests
```

`-tol` is the relative tolerance for REAL and LREAL values: `1e-3` by
default, so 33.333333 and 33.333332 agree. Every other type must match
exactly; bit strings compare by value (`WORD#16#FF` = `255`).

### 9. Record every scan: `-csv`

```text
$ beedance -test -run stopsonfault -csv out motor.st motor_tests.st
$ ls out
TEST_MotorStopsOnFault.eval.csv  TEST_MotorStopsOnFault.vm.csv
$ head -3 out/TEST_MotorStopsOnFault.eval.csv
scan,done,failures,fault,m.run,m.status.running,m.status.starts,message,scans
1,false,0,false,false,false,0,,1
2,false,0,false,false,false,0,,2
```

One file per test and engine, one row per scan: open it in a spreadsheet
to plot a value against time, or diff two engines' files.

### 10. What is watched: `-members` and `-depth`

The engines record, compare and write to CSV the test program's variables
of elementary types and, under dotted names, the **outputs** of its
function block instances and the members of its structures: `m.run`,
`m.status.running`. No copy into a probe variable is needed.

`-members all` adds each instance's inputs and internal variables:

```text
$ beedance -test -run stopsonfault -members all -csv out motor.st motor_tests.st
$ head -1 out/TEST_MotorStopsOnFault.vm.csv
scan,done,failures,fault,m.delay,m.fault,m.run,m.start,m.startDelay.ET,m.startDelay.IN,m.startDelay.PT,m.startDelay.Q,m.status.running,m.status.starts,m.stop,message,scans
```

`-depth` is how many levels below a variable are watched, 2 by default
(`m.status.running` is 2). `-depth 1` stops at `m.run`:

```text
$ beedance -test -run stopsonfault -depth 1 -csv out motor.st motor_tests.st
$ head -1 out/TEST_MotorStopsOnFault.vm.csv
scan,done,failures,fault,m.run,message,scans
```

A standard block (TON, CTU, ...) lists its inputs and outputs only, never
its internal state, which each engine keeps its own way. Arrays are not
watched.

### 11. Build the go engine with local modules: `-go-replace`

```text
$ beedance -test -engines all -go-replace github.com/apiarytech/royaljelly=../../../royaljelly motor.st motor_tests.st
```

`MODULE=DIR`, comma-separated for several, builds with a local checkout
instead of the published version: to test a royaljelly change against
your ST code, or to work offline.

### 12. Exit status, for CI

| Status | Meaning |
|---|---|
| 0 | every test passed, and the engines agree |
| 1 | a test failed, or the engines disagree |
| 2 | a file could not be read or parsed, or there are no tests |

```text
$ beedance -test motor.st does_not_exist.st
open does_not_exist.st: The system cannot find the file specified.     (exit 2)

$ beedance -test motor.st
no test programs (PROGRAM TEST_...)                                    (exit 2)

$ beedance -test bad.st
Parser errors:
	expected next token to be ;, got END_VAR instead at row 2, column 27  (exit 2)
```

A GitHub Actions step:

```yaml
- name: ST unit tests
  run: |
    go install github.com/apiarytech/beedance@latest
    beedance -test -engines all examples/unit_tests/motor.st examples/unit_tests/motor_tests.st
```

In a shell script, `beedance -test ... || exit 1`; in PowerShell,
`beedance -test ...; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }`.

### 13. From Go: package `sil`

`-test` is a report over package `sil`, which returns every result and
scan to the caller. [unit_tests_test.go](unit_tests_test.go) runs this
example's tests that way, as part of `go test ./...`:

```go
results, err := sil.Run(ctx, source, sil.Options{
    Engines:  []sil.Engine{sil.Evaluator, sil.VM, sil.Go},
    Interval: 10 * time.Millisecond,
})
// err: a *sil.ParseError, sil.ErrNoTests, or ctx's error
for _, r := range results {
    fmt.Println(r.Name, r.Passed, r.Mismatch)
    for _, e := range r.Engines {
        fmt.Println("  ", e.Engine, e.Problems(), len(e.Scans), "scans")
    }
}
```

```text
$ go test -v ./examples/unit_tests
--- PASS: TestMotor
--- PASS: TestFailing
--- PASS: TestPrecision
```

`go test -short` leaves out the `go` engine, which builds Go code.

## All the flags

| Flag | Default | |
|---|---|---|
| `-test` | | run the `PROGRAM TEST_...` tests of the files |
| `-engines` | `eval,vm` | `eval`, `vm`, `go`, comma-separated, or `all`; compared with the first |
| `-interval` | `10ms` | simulated time between scans |
| `-max-scans` | `50000` | scans a test with `done` may take |
| `-run` | | only tests whose name contains this text |
| `-tol` | `1e-3` | relative tolerance for REAL values between engines |
| `-members` | `outputs` | members of function block instances watched: `outputs` or `all` |
| `-depth` | `2` | levels of members watched below a variable |
| `-csv` | | write each test's scans per engine as CSV into this directory |
| `-go-timeout` | `1m` | time one test may take on the `go` engine |
| `-go-replace` | | `MODULE=DIR,...`: build the `go` engine with local modules |

See also [doc/cli.md](../../doc/cli.md#unit-tests) and, for the Go API,
[doc/embedding.md](../../doc/embedding.md#unit-tests-software-in-the-loop).
