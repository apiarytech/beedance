# IEC 61131-3 conformance

What beedance does not do yet of IEC 61131-3, one item per
gap, in the order they are to be fixed. Found on 2026-10-14 by running a
small test program per feature on the evaluator, the VM and the transpiled
Go (`beedance -test -engines all`); 32 of 86 such probes passed on every
engine.

The references, in the apiarytech folder: IEC 61131-3:2003 (2nd edition), `archive/Reference/iec61131_standards.pdf`, and for the 3rd edition the Siemens STEP 7 manual "Standards compliance according to IEC 61131-3 (3rd Edition)" (04/2015), `archive/Reference/IEC_61131_compliance_en_US.pdf`, which lists the 3rd edition's feature tables. beedance targets the 3rd edition.

## How to use this file

- Each item has an ID, **IEC-1** onwards. IDs are never reused or
  renumbered; cite them in commits ("IEC-3: string escapes").
- Items are fixed one at a time, in ID order unless a row says otherwise.
- A fix comes with a test that runs on every engine (`sil` package, as
  `sil/fb_arrays_test.go`), and the beedance and beehive suites pass.
- When an item is done, set its status to **Done** with the date, and remove
  it from [Not supported yet](language.md#not-supported-yet) if it is listed
  there.

Engines: **E** evaluator, **V** VM, **G** transpiled Go. Size: **S** a
function or two, **M** one engine in depth or several engines lightly,
**L** the parser and every engine.

## Phase 1: wrong results

The program runs but gives a wrong value, or the engines disagree.

| ID | Item | Engines | Size | Status |
|---|---|---|---|---|
| IEC-1 | `SR` and `RS` on the evaluator: set and reset dominance were swapped, and a variable named like the type (`sr`) hid it (`sil/bistables_test.go`) | E | S | Done 2026-10-14 |
| IEC-2 | `LOWER_BOUND` and `UPPER_BOUND` ignore the declared bounds (`ARRAY[2..5]` gives 0 and 3); missing in Go | E V G | S | Done 2026-10-14 |
| IEC-3 | String escapes are not decoded: `$'`, `$$`, `$N`, `$L`, `$R`, `$T`, `$P`, `$hh` (and `$"`, `$hhhh` in `WSTRING`) | E V G | M | Done 2026-10-14 |
| IEC-4 | `STRING(n)` does not cut a longer value to `n` characters (also function block inputs, and WSTRING(n) in Go) | E V | S | Done 2026-10-14 |
| IEC-5 | `LEN` of a `WSTRING` counts bytes, not characters (`"héllo"` is 6) | E V G | S | Done 2026-10-14 |
| IEC-6 | Integer overflow: `SINT` 127 + 1 wraps in Go, not on the evaluator and the VM; one behaviour on every engine | E V G | M | Done 2026-10-14 |
| IEC-7 | `REAL_TO_STRING` and `TIME_TO_STRING` differ by engine (`1.500000` / `1.5`, `1s` / `T#1s`); IEC's form on every engine | E V G | S | Done 2026-10-14 |

## Phase 2: works on some engines only

| ID | Item | Missing on | Size | Status |
|---|---|---|---|---|
| IEC-8 | Interface variables: assigning an instance to an interface, calling its methods, arrays of interfaces, interfaces that extend interfaces (methods have pointer receivers) | G | M | Done 2026-10-14 |
| IEC-9 | A function block instance passed as `VAR_IN_OUT` to a function (`now` undefined) | G | S | Done 2026-10-14 |
| IEC-10 | `DT + TIME`, `TOD + TIME` and the other time operators | G | S | Done 2026-10-14 |
| IEC-11 | Array bounds given by a constant, `ARRAY[0..NMAX]` (length 0, panics) | G | S | Done 2026-10-14 |
| IEC-12 | SFC step flags `step.X` and `step.T` | V G | M | Done 2026-10-14 |
| IEC-13 | SFC action flags `action.Q` and `action.A` (`A` is `Q`: no final scan after an action ends) | V G | S | Done 2026-10-14 |
| IEC-14 | An alias type compared with a literal (`TYPE VOLT : REAL`; `v = 230.0`) | V | S | Done 2026-10-14 |
| IEC-15 | `BOOL#1` and `BOOL#0` | V | S | Done 2026-10-14 |
| IEC-16 | A call qualified by its namespace, `LIB.Twice(3)` | V | S | Done 2026-10-14 |
| IEC-17 | A method called by name, or with `THIS^`, from its function block's own body | E V | M | Done 2026-10-14 |
| IEC-18 | Extensible comparisons with more than two inputs, `GT(a, b, c)`, `EQ(a, b, c)` | E V | S | Done 2026-10-14 |
| IEC-19 | `CONCAT_DATE_TOD` | E V | S | Done 2026-10-14 |
| IEC-20 | The power operator `**` | E G | S | Done 2026-10-14 |
| IEC-21 | `AND`, `OR`, `XOR` on integer literals (`16#F0 AND 16#3C`) | E G | S | Done 2026-10-14 |
| IEC-22 | `INT_TO_BCD`, `BCD_TO_INT` and the other BCD conversions (wrong on the VM, missing elsewhere) | E V G | S | Done 2026-10-14 |
| IEC-23 | `CHAR` and `WCHAR` with their conversions (`BYTE_TO_CHAR`, `CHAR_TO_BYTE`, `CHAR_TO_STRING`, ...) | V G | M | Done 2026-10-14 |

## Phase 3: language features not supported

The source does not parse, or a construct is refused, on every engine.

| ID | Item | Size | Status |
|---|---|---|---|
| IEC-24 | `CONTINUE` in `FOR`, `WHILE` and `REPEAT` | M | Done 2026-10-14 |
| IEC-25 | `R_EDGE` and `F_EDGE` after the type, as IEC writes them (`c : BOOL R_EDGE`); only `R_EDGE c : BOOL` parses; neither form had any effect: now an edge input is TRUE only on its edge (package edges) | S | Done 2026-10-14 |
| IEC-26 | `STRING[n]` and `WSTRING[n]` (only `STRING(n)` parses) | S | Done 2026-10-14 |
| IEC-27 | `R` and `S` as structure member names | S | Done 2026-10-14 |
| IEC-28 | Pragmas `{ ... }`, kept or ignored | S | Done 2026-10-14 |
| IEC-29 | The `OVERRIDE` method modifier (overriding works without it) | S | Done 2026-10-14 |
| IEC-30 | A `FUNCTION` without a result type | M | Done 2026-10-14 |
| IEC-31 | Enumerations with values, `(RED := 1, GREEN := 5)`, and their conversions to integers | M | Done 2026-10-14 |
| IEC-32 | Typed enumerations, `INT (A := 1, B := 2) := A` (after IEC-31) | M | Done 2026-10-14 |
| IEC-33 | Enumeration values without the type name where it is clear (`CASE c OF X: ...`) | M | Done 2026-10-14 |
| IEC-34 | Named SFC transitions, `TRANSITION T1 FROM S0 TO S1 := ...` (and `PRIORITY`) | S | Done 2026-10-14 |
| IEC-35 | Assignment attempt `?=` on interfaces and references (Ed. 3, compliance manual: assignment attempt with interfaces / references) | M | Done 2026-10-14 |
| IEC-36 | Arrays of any length as `VAR_IN_OUT`, `ARRAY[*] OF INT` (after IEC-2) (Go: one dimension; the evaluator and the VM any number) | L | Done 2026-10-14 |
| IEC-37 | Arrays of arrays: an array of a named array type, as the standard's grammar has it, works on every engine (`ARRAY[1..2] OF ROW`); `ARRAY[..] OF ARRAY[..]` written in place is a CODESYS extension, in neither reference, and will not be supported (decided 2026-10-14) | L | Done 2026-10-14 |
| IEC-38 | The long time types `LTIME`, `LDATE`, `LTOD`, `LDT` and their literals (`LT#1s`) | L | Done 2026-10-14 |

## Phase 4: standard library

| ID | Item | Size | Status |
|---|---|---|---|
| IEC-39 | Generic conversions `TO_INT`, `TO_REAL`, ... and `TRUNC_INT`, `TRUNC_DINT`, ... | M | Done 2026-10-14 |
| IEC-40 | Time functions `ADD_TIME`, `SUB_TIME`, `MUL_TIME`, `DIV_TIME`, `ADD_DT_TIME`, `SUB_DT_TIME`, `SUB_DT_DT`, `ADD_TOD_TIME`, `SUB_TOD_TOD`, `SUB_DATE_DATE`, `DT_TO_DATE`, ... | M | Done 2026-10-14: every function of Table 36 and 37 with its long form, and MULTIME, DIVTIME and CONCAT_DATE, CONCAT_TOD, CONCAT_DT; SPLIT_* and DAY_OF_WEEK left out (outputs; OSCAT has DAY_OF_WEEK) |
| IEC-41 | Byte order `TO_BIG_ENDIAN`, `TO_LITTLE_ENDIAN`, `FROM_BIG_ENDIAN`, `FROM_LITTLE_ENDIAN` | S | Done 2026-10-14: a value is little-endian, so the big-endian functions reverse the bytes of an integer, a bit string or a real |
| IEC-42 | `IS_VALID` and `IS_VALID_BCD` | S | Done 2026-10-14 |
| IEC-43 | Typed counters and timers: `CTU_INT`, `CTU_DINT`, `CTU_LINT`, `CTU_UDINT`, `CTU_ULINT` (and `CTD_`, `CTUD_`), `TON_LTIME`, `TOF_LTIME`, `TP_LTIME` (after IEC-38) | M | Done 2026-10-14: package typedfbs declares them in ST for every engine when a program uses them |
| IEC-44 | `CLASS` ... `END_CLASS` | L | Done 2026-10-14: parsed as a function block with no body (ast.FunctionBlockDeclaration.IsClass), which every engine runs; VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT and a body are errors. Not checked: calling an instance of a class. PLCopen XML has no class, so a class exports as a function block |

## Still to review

| ID | Item | Status |
|---|---|---|
| IEC-45 | Probe what the first review left out: IL, LD and FBD beyond [Not supported yet](language.md#not-supported-yet), configurations, tasks and `VAR_ACCESS`; add the gaps found here | Done 2026-10-14: 25 probes of the 3rd edition's Tables 48, 49, 62 and 67 to 70; the gaps are Phase 5 |

## Phase 5: found by the second review

Instruction List from the 3rd edition's Tables 67 to 70, and configurations
from Table 62. Classes (Table 48, 49), `ABSTRACT`/`OVERRIDE` classes,
instance initialization, `VAR_ACCESS`, `RETC` and configuration-level
`VAR_GLOBAL` passed on every engine.

| ID | Item | Engines | Size | Status |
|---|---|---|---|---|
| IEC-46 | IL in Go: `LD` of a BOOL member of an instance (`LD t.Q`) does not compile (the accumulator is a LINT), nor does a parenthesized expression (`ADD( LD b ... )`) | G | M | Done 2026-10-14: the accumulator of an operand from its Go type, standard FB members included; a parenthesized operand is a closure. The evaluator gained IL NOT and MOD |
| IEC-47 | IL parenthesized expression in short form, `ADD( b` (Table 67, 2) | E V G | S | Done 2026-10-14 |
| IEC-48 | IL `&` and `&N` for `AND` and `ANDN` (Table 68, 5) | E V G | S | Done 2026-10-14; Go also gained the N modifier of LD, ST, AND, OR and XOR |
| IEC-49 | IL `ST?`, the assignment attempt (Table 68, 24) | E V G | S | Done 2026-10-14: each engine runs ST? x as x ?= the current result; Go, which has no accumulator for an instance, attempts the operand of the last LD. Go's ?= to an interface is now nominal (a type switch over the blocks that implement it), and an INTERFACE may name a method GET or SET |
| IEC-50 | IL calls of functions and methods: non-formal, `LIMIT 0, 10` with the accumulator first, and formal, `LIMIT(MN := 0, IN := a, MX := 10)`, `f.Add(k := 5)` (Table 69, 3a to 4b) | E V G | M | Done 2026-10-14: the parser reads a call instruction as LD of the call, the current result (ast.IlCurrentResult) its first input in the non-formal form |
| IEC-51 | IL standard function block operators: `S1 f`, `R f`, `CLK t`, `CU c`, `CD c`, `PV c`, `IN t`, `PT t`, ... (Table 70) | E V G | M | Done 2026-10-14: the parser reads CU c as CAL c(CU := the current result); S and R of an RS or SR instance do the same in each engine. Not done: RESET of Table 70 |
| IEC-52 | A non-formal call of a standard function block, `CAL t(TRUE, T#0s)` or `t(TRUE, T#0s);`, is "too many arguments" in the evaluator (Table 69, 1a) | E | S | Done 2026-10-14: the evaluator names the inputs of a standard block, and counts inherited inputs; Go, which dropped every positional input of a function block, names them as the VM orders them |
| IEC-53 | A contextual keyword as a member name in a structure's initial value, `(step := 3)` | E V G | S | Done 2026-10-14 |
| IEC-54 | `VAR_GLOBAL` of a `RESOURCE` is not seen by its programs (Table 62, 4); check program connections `P1 WITH T1 : MAIN(k := g)` (8a to 9b) on the runtime, which `sil` does not run | E V G | M | Done 2026-10-14: the VM and Go compile a resource's VAR_GLOBAL with the configuration's (one scope: two resources may not share a name); sil runs a configured program as its instance; the evaluator makes an instance's connections on every call, an input read each cycle |
| IEC-55 | Go: a partly located variable, `x AT %I* : INT`, with its address in `VAR_CONFIG` (Table 62, 11a) | G | M | Done 2026-10-14: Go takes the address VAR_CONFIG gives (one per program type: Go has one struct for it), and an unconfigured %I* is an ordinary variable; the evaluator kept a located variable's value no longer when VAR_CONFIG moved it, and now does (outputs and memory) |
| IEC-56 | A formal call of a standard function, `LIMIT(MN := 0, IN := v, MX := 20)`: the evaluator panics, the VM is wrong, Go is "takes its arguments in order". Found while fixing IEC-50 | E V G | S | Done 2026-10-14: package stdcalls puts the inputs in order for every engine |
| IEC-57 | Program connections in a configuration, `PROGRAM P1 WITH T1 : MAIN(k := g, out => cg)`, on the VM and in Go, which ignore them; beehive runs its own task list, not a textual configuration (Table 62, 8a to 9b). Found while fixing IEC-54 | V G | M | Done 2026-10-14 for Go: a connected program gets new<Program>, and main wraps its Logic to read the connected inputs before each run and write the outputs after. On the VM, package vmconfig (added 2026-10-14) runs a configuration: tasks by INTERVAL, SINGLE and PRIORITY, shared globals and process image, connections and VAR_CONFIG; `beedance -vm -iec FILE` uses it |
| IEC-58 | Calling an instance of a CLASS, `c();`, is accepted; a class has no body, only methods (Table 48) | E V G | S | Done 2026-10-14: each engine reports the call; sil/class_call_test.go |
| IEC-59 | IL `RESET c`, the RESET operator of CTU and CTUD (Table 70) | E V G | S | Done 2026-10-14: RESET c is CAL c(R := the current result) |
| IEC-60 | `SPLIT_DATE`, `SPLIT_TOD`, `SPLIT_DT` and the long forms, functions with outputs (Table 37) | E V G | M | Done 2026-10-14: package stdcalls makes a call the assignment of each output, y := $DATEPART(dt, 'YEAR'), a built-in of the evaluator and VM and inline Go |
| IEC-61 | `DAY_OF_WEEK` of IEC 61131-3, which must not replace OSCAT's (another convention) | E V G | S | Done 2026-10-14: DAY_OF_WEEK is IEC's, 0 for Sunday, on every engine, unless the program declares it (OSCAT's library) or Go has PreferOSCAT. Go's default changed: it gave OSCAT's for an undeclared DAY_OF_WEEK |
| IEC-62 | `r ?= a`: an assignment attempt to a variable named like a keyword (R, S) does not parse | E V G | S | Done 2026-10-15: a mnemonic before ?= is a name, as before := |
| IEC-63 | An SFC transition condition in IL, `TRANSITION FROM S0 TO S1 :` then `LD n` `GE 2` then `END_TRANSITION` (2003 Table 41, 6b and 7c; Ed.3 Table 55). An ST body, `TRANSITION := ...`, is not in the standard's grammar | E V G | M | Done 2026-10-15: the parser folds the simple instruction list into the condition, LD n GE 2 into n >= 2, so every engine runs it. Not covered: a condition as an LD or FBD network (7a, 7b) |
| IEC-64 | Go: a TOD from DT_TO_TOD keeps its date, so `DT_TO_TOD(d) = TOD#01:00:00` is FALSE | G | S | Done 2026-10-15: Go compares two TODs by their clock |
| IEC-65 | Go: DAY_OF_WEEK's result is not converted to its target, `n : USINT := DAY_OF_WEEK(d)` | G | S | Done 2026-10-15: Go's IEC DAY_OF_WEEK takes the type its assignment expects |
| IEC-66 | PROPERTY with GET and SET (a CODESYS extension beedance accepts): the evaluator does not find it, the VM gives a wrong value | E V | M | Done 2026-10-15: a setter reads the new value as value or by the property's name on every engine (the evaluator and the VM knew only value) |
| IEC-67 | An integer assigned to a REAL or LREAL variable stays an integer on the evaluator and the VM, so `r := i; r := r / 2` divides as integers (and the VM compared 3 with 3.0 as unequal). Found while fixing IEC-65 | E V | S | Done 2026-10-15: the evaluator converts an integer given to a REAL or LREAL (variable, member, element, input); the compiler assigns TO_REAL(value); the VM compares an integer with a real as reals |

## Phase 6: found with beeguard's alarm blocks

Found on 2026-10-15 while running beeguard's reference alarm function
blocks, `beeguard/reference/digital_alarm.st` (`BG_ALM_DIG`, ISA-18.2
digital alarm) and `analog_alarm.st` (`BG_ALM_ANA`, six `BG_ALM_DIG`
inside), for drone-buzz. **They do not compile as written.** With exactly
these two items worked round in a copy, both blocks pass a unit test on
the evaluator, the VM and the transpiled Go, and the engines agree (high
alarm, acknowledgement, on-delay; `beedance -test -engines all`). Both items
are dialect extensions (CODESYS, TwinCAT), not features of the 3rd edition,
like `PROPERTY` (IEC-66) and OSCAT's input writes. beedance accepts such
extensions when common libraries need them.

**Decision (owner, 2026-10-15): `JMP` is allowed in every language.**
Where each language stands:

| Language | Jumps today | Item |
|---|---|---|
| IL | `JMP`, `JMPC`, `JMPCN` and labels on every engine in a `PROGRAM`; the Go engine cannot transpile IL in a `FUNCTION` or `FUNCTION_BLOCK` at all | IEC-70 |
| ST | `JMP label;` and `label:` on every engine (a CODESYS/TwinCAT extension; Ed.3 Table 72 has none) | IEC-68, done |
| LD, FBD | PLCopen `jump`, `label` and `return` elements and the text form's `>>LABEL`, `RETURN` and `LABEL:`, lowered to ST jumps, on every engine | IEC-71, done |
| SFC | A jump to a step is imported from PLCopen and drawn; within an action or transition body, as the body's own language | - |

| ID | Item | Engines | Size | Status |
|---|---|---|---|---|
| IEC-68 | `JMP label;` and `label:` in Structured Text. Both blocks end with `IF NOT IN_ENABLE THEN JMP CLEAR_COMMANDS; END_IF` ... `CLEAR_COMMANDS:` followed by statements. The parser stops: `expected next token to be END_FUNCTION_BLOCK, got IDENT ... parser stuck on token CLEAR_COMMANDS`. Ed.3 Table 72 lists no jump among the ST statements; jumps are IL (2003 Table 52 item 18, Ed.3 Table 68) and LD/FBD (Ed.3 Table 73) | E V G | L | Done 2026-10-16: `ast.JumpStatement`, `ast.LabelStatement`; the parser checks every POU body (`parser/jumps.go`: a label once, the target in the same body, no jump into a block) so every engine refuses the same; evaluator (a jump passes up to the statement list holding its label), VM (`compiler/jumps.go`, OpJump patched), Go (`transpiler/jumps.go`, goto, each statement of a labelled list in its own block). `sil/st_jump_test.go` on all three; `parser/jumps_test.go`. beeguard's two alarm blocks now compile as written with input writes on (IEC-69 left) |
| IEC-69 | A function block assigning its own `VAR_INPUT` (`CMD_OP_ACK := FALSE;`, the blocks clear their one-shot requests). **The engines disagree:** the evaluator and the Go engine accept it, the VM refuses it (`vm: compile: cannot assign to read-only variable 'CMD_OP_ACK'`), because `compiler.Compiler.AllowInputWrites` is off by default and nothing can turn it on outside Go code | E V G | M | Done 2026-10-16: one rule on every engine, IEC's by default: package `inputwrites` refuses what the VM's compiler refuses (`cannot assign to read-only variable`) for the evaluator and the transpiler, `inputwrites_test.go` holding them to the VM case by case; `AllowInputWrites` in sil, the evaluator (session), the transpiler, vmconfig and `-allow-input-writes`; beehive allows it for a project with a library (logic already, now Check, Simulate, Test, HIL tests, beehive-build). CODESYS's semantics agree on every engine (`sil/input_writes_test.go`); beeguard's two alarm blocks, unchanged, pass on all three (`sil/alarm_blocks_test.go`) |
| IEC-70 | Go: an IL body in a `FUNCTION` or `FUNCTION_BLOCK` does not transpile, with or without jumps (`go: transpile: unhandled AST node type: *ast.IlInstructionStatement`); the evaluator and the VM run it. IL in a `PROGRAM`, `JMP`/`JMPC` loops included, runs and agrees on all three engines. Found 2026-10-15 while checking IL jumps for IEC-68. Likely fix: the POU writers call the IL path that `PROGRAM` bodies use (`isIlBlock`, `transpileIlInstruction` in transpiler.go) | G | M | Done 2026-10-16: `transpileBlockStatement` writes an IL list through the PROGRAM's IL path, so FUNCTION, FUNCTION_BLOCK and METHOD bodies transpile; `ST` into a function's or method's own name (its result) finds its type. Found on the way and fixed: `RETCN` and `CALCN` acted when the result was TRUE (the N was ignored); a label nothing jumps to did not build in Go, and a label named like a Go keyword could not be one (IL labels are now `L_NAME`, used by a goto that never runs, as ST's). `sil/il_pou_test.go` on all three engines |
| IEC-71 | Jumps, labels and returns in LD and FBD (Ed.3 Table 73; 2003 4.1.4, Table 58: `>>LABELA` and `LABELA:`), by the owner's decision that `JMP` is allowed in every language. Today a PLCopen `jump`, `label` or `return` element fails the import (`diagram/lower.go`, "not supported yet in FBD and LD"), and beedance's LD/FBD text form has no jump. Needs IEC-68 first: LD and FBD are lowered to ST | E V G | L | Done 2026-10-16: `diagram` lowers `jump` to `JMP` (under its rung or signal unless wired to the left rail), `return` to `RETURN`, `label` to `label:` before its network, so the engines run them as IEC-68's ST jumps; the text form has `>>LABEL`, `RETURN` and `LABEL:` in LD (ending a rung) and FBD (`>>LABEL(value)`); `diagram.XML`, `Format` and the SVG write and draw them; PLCopen import and export keep them. `sil/diagram_jump_test.go` (all three engines, and through PLCopen as diagrams and as ST), `diagram/jumps_test.go` (lowering, round trip, drawing) |
| IEC-72 | A structured type declared after the POU that uses it: a member read on its own works (`IF s.b THEN`), but a member in an expression does not (`(s.t - T#1s) >= T#30s` fails with "type definition not found for 'ST_X'", `compiler.go`). IEC types are global to the project, so order must not matter. Found 2026-10-16 with beehive D-60, which puts `ST_SystemDiagnostics` before line 1 to work around it Holds in the working tree since 2026-10-16 (no longer reproduces; the uncommitted compiler work of the same day). Kept by `TestStructTypeDeclaredAfterUse` (compiler/compiler_struct_members_test.go): before, after and in another case, a member read, typed in an expression and written | V | S | Done |
| IEC-73 | A write to a member a structured type does not have (`s.nope := TRUE;` with `ST_X` having no `nope`) compiles without an error; it should be refused, as a read of an unknown variable is. Found 2026-10-16 with beehive D-60 Built 2026-10-16: a read or a write of a member a structure does not have is refused (`member 'nope' not found on structure 'ST_X'`, at the member), in `compiler.go`'s member access and assignment; function blocks are left as they were. `TestStructUnknownMemberWrite` | V | S | Done |
| IEC-74 | A `RETURN` in a function block ended the calling program's scan on the evaluator: the block's body result, a return, went back to the caller as one. Found 2026-10-16 with IEC-71's LD `RETURN` | E | S | Done 2026-10-16: an FB call unwraps it (`evaluator.go`); `sil/fb_return_test.go` on all three engines |
| IEC-75 | A function block that declares EN and ENO itself, as IEC 61131-3 says they are provided (2003 2.5.1.2 and 2.5.2.1a, Ed.3 Table 18: `VAR_INPUT EN : BOOL := 1; END_VAR`, `VAR_OUTPUT ENO : BOOL; END_VAR`, names in any case), runs differently on each engine. The rules: EN FALSE, the body is not executed and ENO is reset to FALSE; otherwise ENO is set TRUE, then the body runs (it may set ENO FALSE); with ENO FALSE a block's outputs keep their previous values. The evaluator follows them. The VM skips the body on EN FALSE but leaves ENO FALSE when EN is TRUE (rule 2), and ran the body on EN FALSE where the body itself reads EN (IEC-71's first test). The Go engine does not build such a block. Found 2026-10-16 with IEC-71's test; checked against the 2003 text the same day | V G | S | Done 2026-10-16: package `enables` writes the rules into the body of a FUNCTION or FUNCTION_BLOCK that declares EN or ENO (`ENO := TRUE; IF EN THEN <body> ELSE ENO := FALSE; END_IF`), for every engine; the VM and Go leave their own EN/ENO out for it, and Go passes a declared EN and ENO to a function as any input and output. A call that does not pass EN leaves it as it was, as any input. `sil/en_eno_declared_test.go` on all three engines |
| IEC-76 | A function block with methods and an IL body does not parse: methods come before the body, and a body after them is read as ST only (`fb_parser.go`); the same block with an ST body, or an IL body and no methods, does. Found 2026-10-16 with IEC-70's test | E V G | S | Open |

### IEC-68: how to fix

Everything below already exists for IL, so ST reuses it:

1. **Parser.** Inside an ST statement list, `IDENT ':'` where the colon is
   not part of `:=` is a label (a new `ast.LabelStatement`), and
   `JMP IDENT ';'` is a jump (a new `ast.JumpStatement`). Care: `JMP` is
   today an IL token, and the parser chooses IL or ST by a body's first
   tokens (parser.go, around line 384). An ST jump ends with `;`, an IL
   jump does not, so a body like `JMP L1;` must still be taken as ST.
   `String()` prints both, so text round-trips (PLCopen export of ST
   bodies keeps the text).
2. **Checks, the same on every engine.** A label is unique in its POU
   body. A jump's target exists in the same POU body (not in another
   POU, method or action). A jump may leave an `IF`, `CASE`, `FOR`,
   `WHILE` or `REPEAT`, but not enter one: the label must be in the
   jump's own statement list or an enclosing one. Both alarm blocks only
   jump forward to the top level of the body. This rule is also what Go's
   `goto` allows.
3. **Evaluator.** A jump returns `object.Jump{TargetLabel}` (as IL does,
   evaluator.go around line 1422). It passes up through nested blocks
   like a `RETURN`. The POU body's statement loop finds the label with the
   label map IL already uses (evaluator.go around line 1160) and goes on
   from there.
4. **Compiler and VM.** Emit `OpJump` with the back-patching of
   `compiler_il.go` (`labelPositions`, `jumpsToPatch`). Leaving a `FOR`
   must clean the stack as `EXIT` does, so reuse `EXIT`'s exit path.
5. **Transpiler.** Write `label:` and `goto label` as for IL
   (transpiler.go around lines 737 and 828). Go refuses a `goto` that
   jumps over a variable declaration in the same block. The transpiler
   must therefore declare a body's temporaries before its first label,
   or put the statements between jump and label in their own block.
6. **Tests.** A `sil` case on every engine (pattern `sil/fb_arrays_test.go`,
   keyed "IEC-68"): a forward jump, a backward jump (a loop), a jump out of
   `IF` and out of `FOR`, an undefined label, a duplicate label, a jump into
   an `IF` refused. Plus the alarm-block pattern: `IF c THEN JMP L; END_IF`
   ... `L:` and statements after it.
7. **Docs.** Under "Not IEC 61131-3" below, and in language.md.

### IEC-69: how to fix

1. **One rule on every engine.** Today the evaluator and the Go engine
   allow writes and the VM does not, so the same source runs on two
   engines and fails on the third. Make `AllowInputWrites` an option of
   all three (evaluator and transpiler options beside the compiler's),
   with the same default. Keep the default IEC's: refused, with the
   VM's message. Then the evaluator and the Go engine start refusing what
   they accept today, so check beebread, OSCAT and beehive projects first
   (`compiler_oscat_libraries_test.go` already turns it on for OSCAT).
2. **A way to turn it on without Go code.** The CLI (`-allow-input-writes`
   for `-test`, `-transpile`, `-vm` and the REPL), `vmconfig`/configuration
   runs, and beehive's engineering service (a project setting, or per
   library). Optionally a pragma in the source, in CODESYS's style, so a
   library that needs it says so itself.
3. **Semantics, as CODESYS.** An input is the instance's own copy. A write
   inside the block stays until the next call that passes that input. A
   call that omits it keeps the written value. That is what makes the
   alarm blocks' pattern work: the caller sets `inst.CMD_OP_ACK := TRUE`
   between calls, and the block clears it once handled. Test that the
   engines agree on exactly this: a write, a call without the input, a
   call with it.
4. **Tests.** A `sil` case on every engine, keyed "IEC-69": refused by
   default with the same message on all three engines; accepted with the
   option; the semantics of point 3.
5. **Docs.** Under "Not IEC 61131-3", and in language.md.

**Done when** `beeguard/reference/digital_alarm.st` and `analog_alarm.st`,
unchanged, compile and pass the alarm test on `-engines all` (with the
IEC-69 option on), and the beedance and beehive suites pass. The test used
on 2026-10-15 (a high alarm with acknowledgement on `BG_ALM_ANA`, a 2 s
on-delay on `BG_ALM_DIG`, 300 scans 10 ms apart) can become that case.

Alternatively, beeguard could rewrite the two blocks in conformant ST (an
`IF` around the body instead of the jump; requests as `VAR_IN_OUT` or
cleared by the caller). That makes them load anywhere, but it changes
their interface. It is beeguard's decision, and beedance should still
handle IEC-68 and IEC-69, since CODESYS libraries use both.

### IEC-71: how to fix

LD and FBD bodies reach every engine as the Structured Text that package
`diagram` lowers them to. So once ST has `JMP` and labels (IEC-68), LD and
FBD jumps become ST jumps, and no engine needs a change of its own:

1. **Lowering** (`diagram/lower.go`). Keep the networks in execution
   order (PLCopen `executionOrderId`, then position):
   - a `label` element becomes `LABELA:` before its network's statements;
   - a `jump` becomes `IF <its condition> THEN JMP LABELA; END_IF;`, or
     `JMP LABELA;` when it is wired to the left rail or a constant `TRUE`;
   - a `return` becomes `IF <condition> THEN RETURN; END_IF;`.
   A jump's condition is the rung or signal wired into it, lowered as
   coils and outputs are today.
2. **Text form.** Add the three to beedance's LD/FBD text form, so a
   body written in text can jump. In `diagram.Format` (XML to text) and
   `diagram.XML` (text to XML), write and read them, so a body
   round-trips. The syntax is to be decided with the text form's grammar
   (in language.md).
3. **Checks**, as IEC-68's: labels unique in the POU, a jump's label in
   the same POU. 2003 4.1.4 also says the target of a jump inside an
   `ACTION` must be in that action.
4. **Drawing** (`diagram` SVG and the Logic pages of beehive and drone):
   a jump as a line ending in `>>LABELA`, a label at the start of its
   network, and power flow shown into the jump as into a coil (L-4).
5. **Import and export.** Remove the "not supported yet in FBD and LD"
   refusal. Export writes the three elements; import reads them.
6. **Tests.** A `sil` case keyed "IEC-71", on every engine: a forward
   jump past a network, a conditional jump, a loop through a backward
   jump, a return. Plus a PLCopen round trip (export, import, same
   behaviour) in package `plcopen`.

## Not IEC 61131-3

Recorded so they are not taken for gaps of the standard:

- **A POU writing its own inputs** (IEC-69) is CODESYS's: IEC 61131-3
  makes a `VAR_INPUT` read-only in its POU. beedance refuses it by default
  on every engine and allows it with `-allow-input-writes` (beehive: a
  project with a library), for OSCAT and beeguard's alarm blocks
  ([language.md](language.md), Writing inputs).

- **Jumps in Structured Text**, `JMP label;` and `label:` (IEC-68), are
  CODESYS's and TwinCAT's: Ed.3 Table 72 lists no jump among ST's
  statements. beedance has them by the owner's decision (2026-10-15) that
  a jump is allowed in every language ([language.md](language.md), Jumps
  in Structured Text).

- **Properties** come from CODESYS. beedance's `SET` takes the new value as
  `value`; CODESYS assigns the property's name (`SET lvl := Level;`), which
  gives an error on the evaluator and 0 on the VM.
- `DAY_OF_WEEK` is OSCAT's, not the standard's: the transpiler maps it to
  beebread, the evaluator and the VM do not have it.
