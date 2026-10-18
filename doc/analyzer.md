# Static analysis

Package `analyzer` checks IEC 61131-3 sources without running them: what
is likely a bug, and what goes against the
[PLCopen Coding Guidelines](https://www.plcopen.org/) (v1.0, 2016). It reads
what the parser reads: ST, IL, SFC, and LD and FBD in their text form,
which the parser lowers to ST with their source positions.

```bash
beedance -lint [-lint-level info|warning|error] [-lint-disable RULE,...] [-lint-max-iterations N] FILE.st...
beedance -lint-rules
```

The files are checked as one source, as `-test` joins them, so a POU may
use another file's; each finding is reported at its own file's row:

```text
motor.st:14:7: warning: assignment: DINT to INT may overflow; write DINT_TO_INT [narrowing, PLCopen CP25]
motor.st:31:1: error: WHILE never ends once it starts: nothing in it changes its condition [unbounded-loop]
beedance: 1 errors, 1 warnings, 0 infos
```

`-lint-level` is the least severity shown (`warning` by default: the
style rules are `info`). `-lint-disable` takes rule IDs or PLCopen rules:
`CP19` turns off every rule that checks CP19. Exit status: 0 when no
warning or error is shown, 1 when one is, 2 when a file cannot be read or
parsed (each file is parsed on its own first, so its errors carry its
rows).

## Rules

| Rule | PLCopen | Severity | Reports |
|---|---|---|---|
| `unused-variable` | CP24 | warning | A local, temporary, external or global variable never read or written |
| `unread-variable` | CP24 | info | A temporary, or a function's or method's local, written but never read (a PROGRAM's or block's variable keeps its value, so a host may read it) |
| `unused-input` | CP24 | info | An input never read, an in-out never used |
| `unset-output` | CP24 | info | An output never written; a function or method that never assigns its result |
| `unreachable-code` | CP2 | warning | The statement after `RETURN`, `EXIT`, `CONTINUE` or `JMP` (up to the next label); a branch a constant condition never takes (`IF FALSE`, the `ELSE` of `IF TRUE`, `WHILE FALSE`); a `FOR` over an empty constant range |
| `narrowing` | CP25 | warning | An assignment, a parameter, an output (`=>`), a function's result or a `FOR` bound whose value converts implicitly to a type that cannot hold all of it: a wider integer, a signed into an unsigned, a real into an integer, `LREAL` into `REAL`, a bit string into an integer and back |
| `literal-range` | CP25 | error | An integer literal out of the range of the type it goes to (`i := 70000` with `i : INT`) |
| `multiple-output-writes` | CP12 | warning | A physical output (`AT %Q...`, or `%Q...` in code) written in more than one place in the program |
| `global-writers` | CP26 | warning | A global variable written by more than one `PROGRAM` |
| `unbounded-loop` | | error, warning | Error: a loop that never ends: `WHILE TRUE` (`REPEAT ... UNTIL FALSE`) that nothing leaves, a loop whose condition nothing in it can change, a `FOR` with a step of 0, a `FOR ... TO` the largest value of its counter's type (the counter wraps). Warning: every other `WHILE` and `REPEAT`, whose iterations a scan cannot bound |
| `loop-iterations` | | warning | A loop whose worst case in one scan, times the loops around it, passes `-lint-max-iterations` (100,000): constant bounds give the count, other bounds the range of the counter's type (`FOR i := 1 TO n` with a `DINT` counter: 2³²) |
| `for-counter-modified` | L12 | warning | A `FOR` counter assigned inside its loop |
| `float-equality` | CP8 | warning | `=` or `<>` on `REAL` or `LREAL` |
| `time-equality` | CP28 | warning | `=` or `<>` on `TIME` (a timer's `ET` steps past a value) |
| `recursion` | CP13 | warning | A POU that calls itself, directly or through others |
| `input-write` | CP17 | warning | A POU assigning its own input (CODESYS allows it, and OSCAT does it: `-lint-disable input-write` for such a library) |
| `instance-calls` | CP20 | warning | A function block instance called more than once in a POU |
| `external-in-fb` | CP6 | info | `VAR_EXTERNAL` in a function block |
| `jump-return` | CP19 | info | `JMP`, `RETURN`, and IL's jumps and returns |
| `continue-exit` | L10 | info | `CONTINUE` and `EXIT` |
| `if-without-else` | L17 | info | An `IF` (with its `ELSIF`s) without `ELSE` |
| `direct-address` | N1 | info | A physical address (`%IX0.0`) written in code instead of a variable declared `AT` it |
| `shadowed-global` | N5 | warning | A local variable named like a global |

## Types

beedance's engines take a value's type at run time, so the analyzer works
types out itself. It reads the declared types (aliases followed to the
type they name), the types of literals (an integer or a real literal
without a type takes the type it goes to, and must fit it), the result types
of operators, of the `_TO_` conversions and of the standard functions, the
members of structures, of the program's function blocks and of the
standard ones (`TON.Q`, `CTU.CV`), and the results of functions and
methods. A conversion is narrowing when IEC 61131-3 Ed.3 does not make it
implicitly (its Figure 11: `SINT` to `INT` to `DINT` to `LINT`, `USINT` to
`UINT` and up and to the wider signed types, integers to the reals that
hold them, `REAL` to `LREAL`, `BYTE` to `WORD` to `DWORD` to `LWORD`). An
expression whose type cannot be worked out is not checked: the analyzer
errs on the side of saying nothing.

This catches what the engines let through: `i := d` with `d : DINT` runs on
all three, and the evaluator, which keeps a `REAL` assigned to an `INT` as a
`REAL`, then disagrees with the transpiled Go, which truncates it.

## What is not checked

- PLCopen rules that are a team's choice rather than a check (N2 to N4, N6
  to N8, N10, C1, C6, L1, L3, L5, L8, L9, L11, L16), and comments (C2 to C5).
- Initialization before use (CP3), overlapping addresses (CP4), complexity
  (CP9), tasks (CP10, CP11, CP15, CP16), the number of parameters (CP23),
  deprecated features (CP27), pointers and dynamic memory (E1 to E3).
- A function block's `VAR_OUTPUT` written in several places: only physical
  outputs (CP12) are.

## API

```go
findings, parseErrors := analyzer.CheckSource(src, analyzer.Options{
	Disable:       map[string]bool{"if-without-else": true},
	MaxIterations: 50_000,
})
for _, f := range findings {
	fmt.Println(f) // "row:column: severity: message [rule, PLCopen ID]"
}
```

`analyzer.Check(program, opts)` takes a parsed `*ast.Program`;
`analyzer.Rules()` lists the rules. Findings come sorted by position, each
with its rule, PLCopen rule, severity, row, column and POU.
