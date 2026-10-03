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

- The graphical languages: Ladder Diagram and Function Block Diagram, and
  SFC in its graphical PLCopen form. The PLCopen importer refuses POUs with
  graphical bodies (see [PLCopen XML](plcopen.md)).
- A binary file format for compiled bytecode.
