# beedance

`beedance` is a toolkit for the IEC 61131-3 programming languages, providing a robust parser and foundation for building compilers, interpreters, and analysis tools for programmable logic controllers (PLCs).

## Core Features & Design Philosophy

The design of `beedance` is centered around creating a high-quality, maintainable, and strictly compliant implementation of the IEC 61131-3 standard.

### Advanced Parser Design

The project is built upon a **Pratt (Top-Down Operator Precedence) parser**. This modern parsing technique is exceptionally well-suited for handling the complexities of computer language grammars. It allows for a clean, efficient, and easily extensible implementation that can gracefully handle operator precedence, prefix operators, and infix expressions.

### Clean & Maintainable Architecture

A core principle of the `beedance` parser is a clean separation of concerns:

*   **Simple Lexer:** The lexical analyzer (lexer) is intentionally kept "dumb." Its only job is to scan the source text and convert it into a stream of tokens based on simple patterns. It has no knowledge of the language's context or semantics.
*   **Intelligent Parser:** The parser consumes the token stream and is responsible for understanding the context. It determines whether an identifier is a variable name, a data type, or a function name based on its position in the grammar.

This separation makes the codebase significantly easier to read, debug, and extend compared to more traditional approaches where the lexer and parser are tightly coupled.

### Strict Standards Compliance

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