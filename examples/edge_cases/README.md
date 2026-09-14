# `beedance` - IEC 61131-3 Language Edge Cases

This directory contains a practical demonstration and test suite that validates how the `beedance` project successfully handles common and complex edge cases associated with implementing the IEC 61131-3 standard.

## Overview

The development of a standards-compliant compiler or interpreter for IEC 61131-3 is fraught with challenges, particularly due to ambiguous grammar and legacy language design choices. The `Language_Edge_Cases.txt` document provides a detailed analysis of issues encountered by other compilers (like `matiec`) and explains how `beedance`'s modern architecture was designed to prevent them from the outset.

The files in this directory serve as a live validation of these architectural advantages.

## The Edge Cases Demonstrated

The `edge_cases.st` file contains Structured Text code specifically written to exercise these known scenarios, while `edge_cases_test.go` executes this code and asserts that `beedance` produces the correct results.

The key edge cases tested are:

### 1. Symbol Disambiguation

A significant challenge for many parsers is distinguishing between the same identifier used for different purposes within the same scope.

*   **The Scenario:** An identifier like `Ambiguous` is used as a `TYPE` name, a `FUNCTION` name, and a `VARIABLE` name. A less sophisticated parser might struggle to resolve these correctly.
*   **`beedance`'s Solution:** The `Test_Symbol_Disambiguation` sub-test verifies that `beedance`'s Pratt parser and evaluator correctly resolve each identifier based on its context, demonstrating a clean separation between syntax and semantics.

### 2. Instruction List (IL) Operators as Variable Names

The IEC 61131-3 standard does not reserve Instruction List (IL) mnemonics as keywords in Structured Text (ST).

*   **The Pitfall:** A compiler might incorrectly treat IL operators like `ST`, `LD`, or `ADD` as reserved keywords, making it impossible to declare a variable with that name (e.g., `VAR ST : INT; END_VAR`).
*   **`beedance`'s Solution:** The `Test_IL_Operators_As_Vars` sub-test confirms that `beedance`'s context-aware parser correctly allows these names to be used as variables in ST, adhering more closely to the standard.
 
### 3. Complex Grammar in `TYPE` Declarations

The grammar for `TYPE` declarations can be ambiguous for older LALR(1) parsers, especially when initial values are included.

*   **The Scenario:** A declaration like `TYPE MyString : STRING := 'Default'; END_TYPE` could be parsed in multiple ways by older parser types, leading to conflicts.
*   **`beedance`'s Solution:** The `Test_Type_Declaration_Ambiguity` sub-test shows that `beedance`'s top-down parser handles these declarations without issue, correctly parsing the type and its default initial value.

## How to Run This Example

To run the validation test, navigate to this directory in your terminal and use the standard `go test` command. The `-v` flag is recommended for verbose output.

```sh
# Navigate to the edge cases example directory
cd c:\go\github.com\apiarytech\beedance\examples\edge_cases

# Run the test
go test -v
```

A successful run indicates that the core `beedance` parser and evaluator are robust against these known language challenges.
