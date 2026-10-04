# beedance honeycomb connector

Connects beedance programs to a [honeycomb](https://github.com/apiarytech/honeycomb)
tag database. It is a Go module of its own, so that beedance itself, and the
TinyGo builds for the Raspberry Pi Pico, do not depend on honeycomb or
royaljelly.

## Converting values

honeycomb's tags hold royaljelly `iec` values; beedance holds `object.Object`
values. A located variable has two types: the one it is declared with
(`raw AT %IW1 : INT`) and the one of the I/O image entry its address names
(`%IW1` is a `WORD`). The converter goes through the declared type:

```go
import bdhc "github.com/apiarytech/beedance/connectors/honeycomb"

// Input: a tag's value to the variable's declared type.
tagValue, _ := db.GetTagValue(v.Address)       // iec.WORD(65515)
obj, err := bdhc.ToObject(tagValue, v.Type)    // object.Int -21

// Output: the variable's value to its tag's type.
val, err := bdhc.FromObject(vm.IO()[v.Address], v.Type, tagValue) // iec.WORD
db.SetTagValueQualityAt(v.Address, val, honeycomb.QualityGood, scanTime)
```

`v` is a `compiler.Variable` from `CompiledProgram.Variables` (VM), or an
address and type from `evaluator.IO()` and `evaluator.IOTypes()`.

- Between types of the same width the bits are kept, as in a PLC's memory:
  `WORD 16#FFFF` is `INT -1`, and `DWORD 16#3FC00000` is `REAL 1.5`.
- Otherwise the value is kept and must fit: `LINT 40000` is not an `INT`, and
  `REAL 2.6` becomes `DINT 3`. The VM holds an `INT` variable's value as an
  `LINT`, so `FromObject` converts it to the declared type by value first.
- All elementary types are supported, and one-dimensional arrays of them
  (`ARRAY[1..4] OF BOOL` and `[]iec.BOOL`). Structures, enumerations, `CHAR`
  and `WCHAR` are not.

## Building

Inside this repository `go.mod` replaces `github.com/apiarytech/beedance` with
`../..`. CI tests the module in its own step, as `go test ./...` at the
repository root does not reach it.
