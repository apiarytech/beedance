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
    *   `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT`, `VAR`, and `VAR_TEMP` blocks.
    *   `EN`/`ENO` (Enable/Enable Out) mechanism for function blocks.

*   **Data Types:**
    *   **Elementary Types:** Full range of `SINT`, `INT`, `DINT`, `LINT`, `USINT`, `UINT`, `UDINT`, `ULINT`, `REAL`, `LREAL`, `BOOL`, `STRING`.
    *   **Time & Date Types:** `TIME`, `DATE`, `TIME_OF_DAY` (TOD), and `DATE_AND_TIME` (DT).
    *   **Bit-String Types:** `BYTE`, `WORD`, `DWORD`, `LWORD` with bitwise operators.
    *   **Derived Types:** `STRUCT`, enumerated types, and subrange types.
    *   **Complex Types:** `ARRAY` literals, indexing, and manipulation functions.

*   **Standard Library:**
    *   **Function Blocks:** `TON`, `TOF`, `TP`, `CTU`, `CTD`, `CTUD`, `R_TRIG`, `F_TRIG`, `SR`, `RS`.
    *   **Built-in Functions:** A rich library including type conversions (`INT_TO_REAL`), arithmetic (`ADD`, `SQRT`, `ABS`), string manipulation (`LEFT`, `MID`, `FIND`, `CONCAT`), array functions (`INSERT`, `DELETE`), and more.

## Core Features & Design Philosophy

The design of `beedance` is centered around creating a high-quality, maintainable, and strictly compliant implementation of the IEC 61131-3 standard.

### Advanced Parser Design
The project is built upon a **Pratt (Top-Down Operator Precedence) parser**. This modern parsing technique is exceptionally well-suited for handling the complexities of computer language grammars. It allows for a clean, efficient, and easily extensible implementation that can gracefully handle operator precedence, prefix operators, and infix expressions.

### Clean & Maintainable Architecture

A core principle of the `beedance` parser is a clean separation of concerns:

*   **Simple Lexer:** The lexical analyzer (lexer) is intentionally kept "dumb." Its only job is to scan the source text and convert it into a stream of tokens based on simple patterns. It has no knowledge of the language's context or semantics.

*   **Intelligent Parser:** The parser consumes the token stream and is responsible for understanding the context. It determines whether an identifier is a variable name, a data type, or a function name based on its position in the grammar.

*   **Stateful Tree-Walking Evaluator:** The evaluator directly traverses the Abstract Syntax Tree (AST) produced by the parser. It manages state through a system of enclosed environments, which provides a clean and robust way to handle variable scopes, function calls, and instances of function blocks. Each function block instance maintains its own separate environment, naturally encapsulating its state.

This three-tiered separation makes the codebase significantly easier to read, debug, and extend compared to more traditional approaches where these components are tightly coupled.
### Standards Compliance

A primary goal of `beedance` is to adhere as closely as possible to the IEC 61131-3 standard. The flexible parser architecture is key to achieving this.

For example, the parser is designed to be context-aware. This allows it to correctly differentiate between language elements like an Instruction List (IL) operator and a variable with the same name. By understanding the context in which a token appears, the parser avoids the need to reserve operators as keywords, which would be a deviation from the standard and would limit identifier naming for programmers. This commitment ensures that code written for `beedance` is portable and predictable for engineers familiar with the standard.

## Getting Started

To start the `beedance` Read-Eval-Print-Loop (REPL), you can run the following command from the project root:

```sh
go run main.go
```

## Licensing

This project is dual-licensed under the terms of the **GNU General Public License version 2 (GPLv2)** and a **commercial license**.

*   **Open Source Usage:** If you are developing open-source software, you are free to use `beedance` under the terms of the GPLv2.
*   **Commercial Usage:** If you wish to use this software in a proprietary, closed-source application, you must purchase a commercial license.

Please see the `LICENSE.md` and `gpl-2.0.md` files for more details.