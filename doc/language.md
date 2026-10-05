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
  `TRANSITION`, action blocks with the qualifiers `N`, `S`, `R`, `P`, and
  the timed `D`, `L`, `SD`, `DS`, `SL`; simultaneous divergence and
  convergence.
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

A rung ends in a coil or a block. Rungs run top to bottom.

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
(`X := value`), a wire named (`w = value`) or a function block called
(`inst : TYPE(...)`, which declares `inst` if the POU does not, or
`inst(...)`). A value is a variable, a literal, a wire, `NOT value`, a
function with positional inputs (`ADD(a, b, c)`; `EN := x` may come
first), or an instance's output (`inst.Q`). Statements run in their order,
except that a block runs before the statements reading its outputs.

Package `diagram` also draws a body, from either form, as SVG
(`diagram.SVG`): a layout computed from its wires, with rungs, contacts,
coils and blocks as IEC 61131-3 draws them. Editors use it for previews.

## Program organization units

- `PROGRAM`, `FUNCTION`, `FUNCTION_BLOCK`, with `EN`/`ENO`.
- Variable blocks: `VAR`, `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT`,
  `VAR_GLOBAL`, `VAR_EXTERNAL`, `VAR_ACCESS`, `VAR_TEMP`, `VAR_CONFIG`;
  qualifiers `CONSTANT`, `RETAIN`, `NON_RETAIN`, `R_EDGE`, `F_EDGE`.
- Located variables (`AT %IX0.0`, `%QW4`, `%MD2`) and bit access on bit
  strings and integers (`w.3`).
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

## The CODESYS dialect used by OSCAT

beedance parses the dialect of the OSCAT BASIC library, so the converted
library runs on every engine (tested in the parser, compiler, evaluator
and VM):

- nested comments `(* (* *) *)`;
- identifiers that ignore case;
- `SR`, `RS`, `S` and `R` as the names of blocks and variables;
- bit access on any bit string or integer;
- integers widened to bit strings where CODESYS allows it.

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

## Not supported yet

- SFC in its graphical PLCopen form; jumps, labels and returns in LD and
  FBD (see [PLCopen XML](plcopen.md)).
- `T`, `D`, `DT` and `TOD` are keywords (the short names of the time
  types), so a variable may not have one of these names.
- A binary file format for compiled bytecode.
