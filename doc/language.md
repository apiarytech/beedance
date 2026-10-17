# Language support

## Languages

- **Structured Text (ST)**: expressions, assignments, IF, CASE, FOR,
  WHILE, REPEAT, EXIT, RETURN, function and function-block calls with
  formal and positional arguments, output assignment (`o => x`).
- **Instruction List (IL)**: the accumulator (current result) and its
  operators: `LD`, `ST`, `S`, `R`, arithmetic and logic, comparisons,
  `JMP`/`JMPC`/`JMPCN`, `CAL`/`CALC`/`CALCN`, `RET`, parenthesized
  expressions.
- **Sequential Function Chart (SFC)**: `INITIAL_STEP`, `STEP`,
  `TRANSITION`, action blocks with the qualifiers `N`, `S`, `R`, `P`, the
  pulses `P1` (as `P`, on the step becoming active) and `P0` (on the step
  becoming inactive) of the 3rd edition, and the timed `D`, `L`, `SD`, `DS`,
  `SL` (`DS` stored only if the step is still active when its time is up,
  `SD` whether it is or not); simultaneous divergence and convergence. The
  evaluator, the VM and the transpiled Go agree on them scan by scan
  (`sfc/qualifiers_test.go`): the VM and the transpiler run a chart as the
  Structured Text package `sfc` lowers it to. A graphical PLCopen SFC body is imported into this text
  form ([PLCopen XML](plcopen.md)), and package `diagram` draws a chart as
  SVG (`diagram.ChartOf`, `diagram.SFCSVG`): loops back are drawn as
  jumps, selections with single bars, simultaneous branches with double
  ones. The bytecode compiler runs a chart as the Structured Text package
  `sfc` lowers it to (a flag per step, each action's control with its
  timers), as LD and FBD bodies are lowered; the evaluator runs the chart
  itself, and the two agree scan by scan (package `sfc`'s tests, software in
  the loop).
- **Ladder Diagram (LD)** and **Function Block Diagram (FBD)**, in
  beedance's text form below or as graphical PLCopen XML
  ([PLCopen XML](plcopen.md)). Both are lowered to ST statements by
  package `diagram`, so they run alike in the evaluator, the VM and the
  transpiler.

## Ladder Diagram and Function Block Diagram as text

A POU's body may be written between `LD` and `END_LD`, or `FBD` and
`END_FBD`, after its declarations. The text keeps no layout: an editor
computes it. Comments (`(* *)` and `//`) may stand anywhere.

```
FUNCTION_BLOCK Motor
VAR_INPUT Start, Stop : BOOL; END_VAR
VAR_OUTPUT Run, Done : BOOL; Starts : INT; END_VAR
LD
  RUNG sealin (* start, hold, stop *)
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S) ( Done )
  RUNG count
    +Run cu:CTU(PV := 100, CV => Starts)
END_LD
END_FUNCTION_BLOCK
```

A rung is `RUNG` and an optional name, then its elements left to right,
coils last:

| Element | Text | Meaning |
|---|---|---|
| Contact | `A`, `/A` | `A`, `NOT A` in series (AND) |
| Edge contact | `+A`, `-A` | a rising, falling edge of `A` (an `R_TRIG`, `F_TRIG` declared for it) |
| Branch | `[ a b \| c ]` | the OR of its legs; legs are series, and branches nest |
| Function contact | `EQ(Mode, 2)`, `/GT(Temp, 80.0)` | the function's BOOL result; `(` directly after the name |
| Block | `t1:TON(PT := T#5S, ET => Elapsed)` | a function block call: the rung drives its power input and continues from its power output (`IN`/`Q` for timers, `CU`/`Q`, `CD`/`Q`, `CU`/`QU` for counters, `CLK`/`Q`, `S1`/`Q1`, `S`/`Q1`; `EN`/`ENO` for any other block); other inputs by name, outputs bound with `=>` |
| Coil | `( X )`, `( /X )` | `X :=` the rung, or its negation |
| Set, reset coil | `( S X )`, `( R X )` | latch, unlatch |
| Edge coil | `( P X )`, `( N X )` | `X` TRUE for one call on a rising, falling rung |
| Jump | `>>LABEL` | after the coils: jump to `LABEL` when the rung is on (IEC-71); a rung of only `>>LABEL` always jumps |
| Return | `RETURN` | after the coils: the POU returns when the rung is on |
| Label | `LABEL:` | on a line of its own, before the rung it names |

A rung ends in a coil, a block, a jump or a return. Rungs run top to
bottom; a jump goes on from its label's rung, as an ST jump (Jumps in
Structured Text, below) with the same rules.

```
FUNCTION_BLOCK Level
VAR_INPUT L, SP : REAL; END_VAR
VAR_OUTPUT High : BOOL; Speed : REAL; END_VAR
FBD
  above = GE(L, SP)                     // a wire, named
  hold : TON(IN := above, PT := T#3S)   // an instance, declared and called
  High := OR(hold.Q, AND(High, NOT(LT(L, SUB(SP, 5.0)))))
  Speed := SEL(High, MUL(L, 0.5), 0.0)
END_FBD
END_FUNCTION_BLOCK
```

An FBD statement, one per line (or ended by `;`), is a variable written
(`X := value`), a wire named (`w = value`), a function block called
(`inst : TYPE(...)`, which declares `inst` if the POU does not, or
`inst(...)`), or a jump, a return or a label (IEC-71): `>>LABEL(value)`
jumps when the value is TRUE and `>>LABEL` always; `RETURN(value)` and
`RETURN` likewise; `LABEL:` on a line of its own. A value is a variable, a literal, a wire, `NOT value`, a
function with positional inputs (`ADD(a, b, c)`; `EN := x` may come
first), or an instance's output (`inst.Q`). Statements run in their order,
except that a block runs before the statements reading its outputs.

Package `diagram` also draws a body, from either form, as SVG
(`diagram.SVG`): a layout computed from its wires, with rungs, contacts,
coils and blocks as IEC 61131-3 draws them. Editors use it for previews.

## Program organization units

- `PROGRAM`, `FUNCTION`, `FUNCTION_BLOCK`, with `EN`/`ENO`. A POU may
  declare them itself (`VAR_INPUT EN : BOOL := TRUE; END_VAR`,
  `VAR_OUTPUT ENO : BOOL; END_VAR`): EN FALSE, its body is not run and ENO
  is FALSE, a block's outputs keeping their values; otherwise ENO is TRUE
  before the body runs, which may set it FALSE. A call that does not pass
  EN leaves it as it was.
- Variable blocks: `VAR`, `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT`,
  `VAR_GLOBAL`, `VAR_EXTERNAL`, `VAR_ACCESS`, `VAR_TEMP`, `VAR_CONFIG`;
  qualifiers `CONSTANT`, `RETAIN`, `NON_RETAIN`, `R_EDGE`, `F_EDGE`.
- Located variables (`AT %IX0.0`, `%QW4`, `%MD2`) and bit access on bit
  strings and integers (`w.3`).
- Directly represented variables in statements: `level := %IW0;`,
  `%QX0.1 := run;`. Their type is their size: `X` (or none, `%I0.1`) BOOL,
  `B` BYTE, `W` WORD, `D` DWORD, `L` LWORD. An input (`%I`) is read-only.
  One nothing has set reads as its type's default. Both engines handle them.
- A bit string a host puts in the I/O image as `object.Byte`, `Word`, `DWord`
  or `LWord` (as the honeycomb connector does) computes as a bit string of
  its width: `%IW0 OR 16#0100`, `lvl AND WORD#16#00FF`, `%IB2 = 0`, `NOT %IW0`.
  A located variable's expressions are typed by its declaration.
- `NAMESPACE`, nested (`MyCompany.MyLibrary`), to keep library names apart.

## Object-oriented extensions

`INTERFACE`, `EXTENDS`, `IMPLEMENTS`, `ABSTRACT` and `FINAL` blocks,
methods and properties, access specifiers, `THIS` and `SUPER`.

## Data types

| Kind | Types |
|---|---|
| Elementary | `BOOL`, `SINT`, `INT`, `DINT`, `LINT`, `USINT`, `UINT`, `UDINT`, `ULINT`, `REAL`, `LREAL`, `STRING`, `WSTRING` |
| Bit strings | `BYTE`, `WORD`, `DWORD`, `LWORD`, with bitwise operators and bit access |
| Time and date | `TIME`, `DATE`, `TIME_OF_DAY` (`TOD`), `DATE_AND_TIME` (`DT`), with typed literals |
| Derived | `STRUCT`, enumerations, subranges, aliases (`TYPE Speed : REAL; END_TYPE`) |
| Arrays | one or more dimensions, any lower bound, array literals; multi-dimensional arrays are filled in row order |

## Standard library

- **Function blocks**: `TON`, `TOF`, `TP`, `CTU`, `CTD`, `CTUD`, `R_TRIG`,
  `F_TRIG`, `SR`, `RS`.
- **Functions**: type conversions (`INT_TO_REAL`, `TIME_TO_DINT`, between
  times, dates and numbers), arithmetic (`ADD`, `ABS`, `SQRT`, `EXPT`,
  trigonometry), selection (`SEL`, `MAX`, `MIN`, `LIMIT`, `MUX`), bit
  strings (`SHL`, `SHR`, `ROL`, `ROR`), strings (`LEN`, `LEFT`, `RIGHT`,
  `MID`, `CONCAT`, `INSERT`, `DELETE`, `REPLACE`, `FIND`), comparison, and
  `TIME()` (the time since start, used by timers).

`beedance -check-builtins` (run from the repository root) checks that
`stdlib`'s index constants, its name-to-index map and the functions it
registers agree.

## Configurations

`CONFIGURATION`, `RESOURCE` and `TASK` (cyclic and event), program
instances assigned to tasks with `WITH`, and instance-specific values with
`VAR_CONFIG`. The evaluator runs program instances without a task in a
background task.

## Jumps in Structured Text

`JMP label;` and `label:` (IEC-68), a CODESYS/TwinCAT extension that
beedance has by the owner's decision that a jump is allowed in every
language; IEC 61131-3 Ed.3 lists no jump among ST's statements. On every
engine:

```
IF NOT IN_ENABLE THEN
  JMP CLEAR_COMMANDS;
END_IF;
(* ... *)
CLEAR_COMMANDS:
CMD_ACK := FALSE;
```

- A label names the statement after it; a body may start with one. Labels
  ignore case, as identifiers do.
- A jump goes forward or back within its POU body (a program, function,
  function block, method or action), never to another.
- A jump may leave an `IF`, `CASE`, `FOR`, `WHILE` or `REPEAT` but not
  enter one: its label is in the jump's own statement list or one around
  it.
- A label is used once in its body. A jump to no label, a label used twice
  and a jump into a block are parse errors.
- In a `CASE` branch, `name:` is the next branch, as always; a label goes
  inside an `IF`, a loop or the body around the `CASE`.
- An ST jump ends with `;`; IL's `JMP label` has none, which is how a body
  starting with `JMP` is told apart.

## The CODESYS dialect used by OSCAT

beedance parses the dialect of the OSCAT BASIC library, so the converted
library runs on every engine (tested in the parser, compiler, evaluator
and VM):

- nested comments `(* (* *) *)`;
- identifiers that ignore case;
- `SR`, `RS`, `S` and `R` as the names of blocks and variables;
- bit access on any bit string or integer;
- integers widened to bit strings where CODESYS allows it;
- a POU writing its own inputs, with input writes allowed (below).

### Writing inputs

IEC 61131-3 makes a `VAR_INPUT` read-only inside its POU. CODESYS allows
the write, and libraries written for it use it: OSCAT, and beeguard's
alarm blocks, which clear an operator's one-shot request once handled
(`CMD_OP_ACK := FALSE;`). beedance follows IEC by default on every engine,
the same way (IEC-69): `cannot assign to read-only variable 'X'`. It allows
the write with:

- `-allow-input-writes` on the command line (`-test`, `-vm`, `-go`, the
  evaluator and configurations);
- `sil.Options.AllowInputWrites`, `compiler.Compiler.AllowInputWrites`,
  `evaluator.SetAllowInputWrites`, `transpiler.Options.AllowInputWrites`
  and `vmconfig.LoadOptions.AllowInputWrites` in Go;
- beehive, for a project with a library (logic, Check, Simulate, Test and
  native builds alike).

What is refused, on every engine as on the VM: an assignment to an input
by name in a function, a function block (its own inputs and those of the
blocks it extends) or a method (its own and its block's). A member or an
element of an input, a `FOR` counting with one, IL's `ST` into one, and
the edge inputs' own lowering are not.

Allowed, an input is the instance's own copy, as in CODESYS: a write inside
the block stays until a call passes the input again, and a call that does
not pass it keeps the written value. A caller sets a request between calls
(`alarm.CMD_OP_ACK := TRUE;`) and the block clears it.

The transpiler maps OSCAT functions and blocks to the
[beebread](https://github.com/apiarytech/beebread) Go packages. Where both
OSCAT and the standard define a name (`ROUND`, `CEIL`), the standard's wins
unless `PreferOSCAT` is set.

## Macros

A compile-time macro takes AST fragments and returns new ones, before any
backend runs:

```iecst
VAR
    add2 : MACRO := macro(a, b) { EXPR(EVAL(a) + EVAL(b)); };
END_VAR
add2(1 + 1, 2 + 2);   (* expands to (2 + 4) *)
```

`EVAL` evaluates an argument in the expansion; `EXPR` quotes code to insert.
Macros are expanded for the evaluator, the compiler and the transpiler alike.
A macro variable has no runtime value: hosts do not see it as a variable.

## Keywords as names

A reserved keyword such as `AT` or `THEN` cannot name a variable. The
declaration is one error that names the keyword, e.g. `AT is a reserved
keyword and cannot be used as a variable name at row 3, column 5`, and its
uses are not reported again. Keywords that read as names where they are
used may name variables: the IL operators (`LD`, the input of `CTD`;
`CAL`, `JMP`, `RET`), `EN` and `ENO`, the task settings (`INTERVAL`,
`PRIORITY`, `SINGLE`, `TASK`, `RESOURCE`), `INTERNAL`, `METHOD`,
`PROPERTY`, `READ_ONLY`, `READ_WRITE`, `RANGE`, `FROM`, `WITH`, and `STEP`,
`SET`, `GET`, `ON`, `S` and `R`. The operator words `ASSIGN`, `PLUS`,
`ASTERISK`, `SLASH` and `NIL` are accepted in a declaration, as before, but
not read as variables.

## Not supported yet

The gaps against IEC 61131-3, numbered IEC-1 onwards and in the order they
are to be fixed, are in [conformance.md](conformance.md). Also:

- Macro steps and IL actions or conditions in graphical SFC (see
  [PLCopen XML](plcopen.md)).
- A function block with methods and an IL body (IEC-76): give it an ST
  body, or put the IL in its methods.
- `JMP` is to be allowed in every language (owner's decision, 2026-10-15;
  [conformance.md](conformance.md), Phase 6).
- `T`, `D`, `DT` and `TOD` are keywords (the short names of the time
  types), so a variable may not have one of these names.
- A binary file format for compiled bytecode.
