# beedance

`beedance` is an interpreter and toolkit for the IEC 61131-3 industrial automation programming languages. It provides a robust, standards-compliant parser and an evaluator capable of executing Structured Text (ST), Instruction List (IL), and Sequential Function Chart (SFC). It serves as a powerful foundation for building compilers, analysis tools, and virtual controllers for PLCs.

## Implemented Features

`beedance` supports a comprehensive set of features from the IEC 61131-3 standard:

*   **Programming Languages:**
    *   **Structured Text (ST):** Full support for expressions, assignments, and control flow.
    *   **Instruction List (IL):** A functional IL interpreter that supports operators like `LD`, `ST`, `JMP`, `CAL`, `RET`, and arithmetic/logic instructions.
    *   **Sequential Function Chart (SFC):** A comprehensive implementation including:
        *   Steps (`INITIAL_STEP`, `STEP`) and `TRANSITION`s.
        *   Action blocks with standard qualifiers (`N`, `S`, `R`, `P`).
        *   Timed action qualifiers (`D`, `L`, `SD`, `DS`, `SL`).
        *   Simultaneous divergence (fork) and convergence (join).

*   **Program Organization Units (POUs):**
    *   `PROGRAM`, `FUNCTION`, and `FUNCTION_BLOCK` declarations.
    *   `EN`/`ENO` (Enable/Enable Out) mechanism for function blocks.
    *   `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT`, `VAR`, `VAR_GLOBAL`, `VAR_EXTERNAL`, `VAR_ACCESS`, and `VAR_TEMP` blocks.

*   **Namespaces:** Full support for `NAMESPACE` blocks to organize POUs and data types into hierarchical scopes, preventing naming conflicts when using libraries. Nested namespaces (e.g., `MyCompany.MyLibrary`) are also supported.

*   **Object-Oriented Programming (OOP):** Full support for object-oriented principles, enabling more modular and reusable code. This includes `INTERFACE` definitions, `FUNCTION_BLOCK` inheritance via `EXTENDS`, interface adherence with `IMPLEMENTS`, and `ABSTRACT` function blocks, methods, and properties. Methods can access their own instance via the `THIS` pointer and call parent implementations using `SUPER`.

*   **Data Types:**
    *   **Elementary Types:** Full range of `SINT`, `INT`, `DINT`, `LINT`, `USINT`, `UINT`, `UDINT`, `ULINT`, `REAL`, `LREAL`, `BOOL`, `STRING`.
    *   **Time & Date Types:** `TIME`, `DATE`, `TIME_OF_DAY` (TOD), and `DATE_AND_TIME` (DT).
    *   **Bit-String Types:** `BYTE`, `WORD`, `DWORD`, `LWORD` with bitwise operators.
    *   **Derived Types:** `STRUCT`, enumerated types, and subrange types.
    *   **Complex Types:** `ARRAY` literals, indexing, and manipulation functions.

*   **Standard Library:**
    *   **Function Blocks:** `TON`, `TOF`, `TP`, `CTU`, `CTD`, `CTUD`, `R_TRIG`, `F_TRIG`, `SR`, `RS`.
    *   **Built-in Functions:** A rich library including type conversions (`INT_TO_REAL`), arithmetic (`ADD`, `SQRT`, `ABS`), string manipulation (`LEFT`, `MID`, `FIND`, `CONCAT`), array functions (`INSERT`, `DELETE`), and more.
*   **System Configuration:**
    *   Full support for `CONFIGURATION`, `RESOURCE`, and `TASK` blocks.
    *   Program instantiation with task assignment (`WITH`).
    *   Instance-specific parameterization via `VAR_CONFIG`.

## Core Features & Design Philosophy

The design of `beedance` is centered around creating a high-quality, maintainable, and strictly compliant implementation of the IEC 61131-3 standard.

### Advanced Parser Design
The project is built upon a **Pratt (Top-Down Operator Precedence) parser**. This modern parsing technique is exceptionally well-suited for handling the complexities of computer language grammars. It allows for a clean, efficient, and easily extensible implementation that can gracefully handle operator precedence, prefix operators, and infix expressions.
The parser also features a robust **panic-and-recover** error handling strategy. This allows it to log detailed syntax errors without halting, synchronize to the next valid statement, and continue parsing the rest of the file. This makes it highly resilient to errors in source code and provides better feedback to the developer.

### Clean & Modular Architecture

A core principle of `beedance` is a clean, modular architecture with a clear separation of concerns. This makes the codebase easy to understand, maintain, and extend. The key packages include:


*   **`stdlib`:** The standard library, containing the Go implementations for all stateless built-in functions (e.g., `SIN`, `LEN`, `CONCAT`). It acts as a "plugin" that registers its functions with the `object` package at startup.

*   **`parser`:** A powerful and efficient **Pratt (Top-Down Operator Precedence) parser** that consumes tokens from the lexer and produces a clean Abstract Syntax Tree (AST).

*   **`evaluator`:** A tree-walking interpreter that directly executes the AST. It is responsible for managing stateful logic, such as variable environments and the execution of standard function blocks (`TON`, `CTU`, etc.).

*   **`compiler` & `vm`:** A bytecode compiler and virtual machine that provide a faster alternative to the evaluator. These components are fully decoupled from the evaluator and can be used independently.

*   **`transpiler`:** A source-to-source compiler that translates the iec61131 AST into human-readable and efficient Go code. This enables integration with Go-native runtimes and compilation to native binaries or WebAssembly.

This modular design allows components like the compiler and VM to operate without any dependency on the evaluator, enabling the creation of lightweight, high-performance tools.
### Powerful Macro Engine

`beedance` includes a powerful, Lisp-inspired macro engine that operates as a pure compile-time pre-processing step. Macros are defined using `MACRO` syntax and allow developers to perform advanced code generation before the program is compiled, transpiled, or evaluated.

Key features of the macro system include:
*   **AST-to-AST Transformation:** Macros take code (as AST nodes) for their arguments and produce new AST nodes.
*   **Unquoting with `EVAL`:** The `EVAL` function allows arguments to be evaluated within the macro's expansion context.
*   **Code Generation with `EXPR`:** The `EXPR` function "quotes" a block of code, turning it into an AST fragment that can be injected back into the program.
*   **Seamless Integration:** The macro expansion pass is integrated into all backends (evaluator, compiler/VM, and transpiler), ensuring that macros can be used to generate code for any target.

```iecst
(* Example of a macro expands to `(2 + 4)` at compile time.*)
VAR
	my_macro : MACRO := macro(a, b) { EXPR(EVAL(a) + EVAL(b)); };
END_VAR
my_macro(1 + 1, 2 + 2);
```
### Standards Compliance
### Extensible Backend Architecture

Beyond its tree-walking evaluator, `beedance` is designed with a flexible backend architecture. The AST produced by the parser can be consumed by different components, allowing for multiple execution or compilation strategies. This makes it possible to add new backends, such as:

*   **Go Compiler:** A compiler that translates IEC 61131-3 source code directly into Go source code. This enables further compilation to highly-optimized native binaries or WebAssembly (WASM) modules using tools like TinyGo.
*   **VM Compiler:** A compiler that generates bytecode for a custom, high-performance virtual machine, offering a significant speed advantage over direct evaluation.

A primary goal of `beedance` is to adhere as closely as possible to the IEC 61131-3 standard. The flexible parser architecture is key to achieving this.

For example, the parser is designed to be context-aware. This allows it to correctly differentiate between language elements like an Instruction List (IL) operator and a variable with the same name. By understanding the context in which a token appears, the parser avoids the need to reserve operators as keywords, which would be a deviation from the standard and would limit identifier naming for programmers. This commitment ensures that code written for `beedance` is portable and predictable for engineers familiar with the standard.

## Getting Started

The primary way to use `beedance` is through its command-line interface, which supports evaluation (REPL, file execution) and compilation.

### Command-Line Flags

You can run `beedance` with the following flags. By default (with no flags), it runs the interactive REPL using the tree-walking evaluator.

*   **No flags:** Starts the interactive REPL (evaluator mode).
    ```sh
    go run main.go
    ```
*   **`-vm`:** Use the bytecode virtual machine instead of the default tree-walking evaluator. This flag can be combined with the REPL, `-iec`, or `-e`.
    ```sh
    go run main.go -vm
    ```
*   **`-iec <path>`:** Executes a program from a source file (e.g., `.st`).
    ```sh
    go run main.go -iec /path/to/your/program.st
    ```
*   **`-e "<string>"`:** Evaluates a single string of code.
    ```sh
    go run main.go -e "MyVar := 10 + 20;"
    ```
*   **`-trace`:** Enables detailed parser tracing, which is useful for debugging the parsing process. This can be combined with other flags.
    ```sh
    go run main.go -trace -iec /path/to/your/program.st
    ```
*   **`-go <path>`:** Transpiles an IEC 61131-3 source file into a Go source file. This flag must be used in conjunction with `-iec`.
    ```sh
    go run main.go -iec /path/to/your/program.st -go /path/to/output.go
    ```
*   **`-test FILE.st...`:** Runs the `PROGRAM TEST_...` unit tests in the files on a simulated clock, on both the evaluator and the VM, and compares the engines scan by scan. Exits 0 if all pass, 1 if a test fails or the engines disagree, 2 if parsing fails. See [doc/cli.md](doc/cli.md#unit-tests).
    ```sh
    go run . -test -interval 10ms lib.st tests.st
    ```
*   **`-version`:** Prints the application version.
    ```sh
    go run main.go -version
    ```

### Examples

#### Running the REPL
You can start the REPL with either the evaluator (default) or the VM.
```sh
# Start REPL with the evaluator
go run .

# Start REPL with the virtual machine
go run . -vm
```

#### Executing a File
The `-iec` flag can be combined with `-vm` to choose the execution engine.
```sh
# Execute a file with the evaluator
go run . -iec /path/to/your/program.st

# Execute a file with the virtual machine
go run . -vm -iec /path/to/your/program.st
```

#### Transpiling to Go
The `-go` flag allows you to convert an IEC 61131-3 Structured Text file into a Go source file. This is useful for integrating with Go-native runtimes like `royaljelly`.

```sh
# Transpile a .st file to a .go file
go run . -iec /path/to/your/program.st -go /path/to/transpiled.go
```


#### Simple State Machine in Structured Text

The following example demonstrates a simple timed-state machine using a `TON` (Timer On-Delay) function block and a `CASE` statement. This code can be saved in a file (e.g., `traffic_light.st`) and executed with `go run main.go -iec traffic_light.st`.

```iecst
PROGRAM Traffic_Light
	VAR
		State : INT := 0;
		StateTimer : TON;
		EnableTimer : BOOL;
		Green_Light : BOOL;
		Yellow_Light : BOOL;
		Red_Light : BOOL;
	END_VAR

	(* A single, clear call to the timer instance on every scan. *)
	(* The IN parameter is controlled by the state machine logic. *)
	StateTimer(IN := EnableTimer, PT := T#5s);

	CASE State OF
		0: (* Green State *)
			Green_Light  := TRUE;
			Yellow_Light := FALSE;
			Red_Light    := FALSE;
			EnableTimer  := TRUE; (* Timer runs during this state *)

			IF StateTimer.Q THEN
				State := 1;
				EnableTimer := FALSE; (* Reset timer for the next state *)
			END_IF

		1: (* Yellow State *)
			Green_Light := FALSE;
			Yellow_Light := TRUE;
			Red_Light    := FALSE;
			EnableTimer  := TRUE;

			IF StateTimer.Q THEN
				State := 2;
				EnableTimer := FALSE;
			END_IF

		2: (* Red State *)
			Green_Light  := FALSE;
			Yellow_Light := FALSE;
			Red_Light := TRUE;
			EnableTimer  := TRUE;

			IF StateTimer.Q THEN
				State := 0;
				EnableTimer := FALSE;
			END_IF
	END_CASE
END_PROGRAM
```

## Licensing

This project is dual-licensed under the terms of the **GNU General Public License version 2 (GPLv2)** and a **commercial license**.

*   **Open Source Usage:** If you are developing open-source software, you are free to use `beedance` under the terms of the GPLv2.
*   **Commercial Usage:** If you wish to use this software in a proprietary, closed-source application, you must purchase a commercial license.

Please see the `LICENSE.md` and `gpl-2.0.md` files for more details.

## Code of Conduct

This project and everyone participating in it is governed by the
`beedance` Code of Conduct. By participating, you are
expected to uphold this code.

## Contributing

Contributions are welcome and greatly appreciated! `beedance` aims to be a high-quality, standards-compliant toolkit, and community contributions are vital to achieving that goal.

If you'd like to contribute, please follow these general steps:

1.  **Open an Issue:** Before starting significant work, please open an issue on GitHub to discuss your proposed changes. This could be a bug report, a feature request, or a suggestion for improvement. This helps ensure your contribution aligns with the project's direction and avoids duplicate effort.
2.  **Fork the Repository:** Create your own fork of the `beedance` repository.
3.  **Create a Branch:** Work on a separate feature branch for your changes (`git checkout -b feature/my-new-feature`).
4.  **Commit Your Changes:** Make your changes and commit them with clear, descriptive messages.
5.  **Submit a Pull Request:** Push your branch to your fork and open a pull request against the main `beedance` repository. Please link the pull request to the issue you opened.

We appreciate your help in making `beedance` a better tool for the industrial automation community!