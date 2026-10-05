# PLCopen XML

The `plcopen` package converts between beedance's AST and PLCopen TC6 XML
v2.01, the exchange format of most IEC 61131-3 tools.

## Export

```go
xmlData, err := plcopen.ExportSourceToXML(source, "MyProject")
// or from an AST: plcopen.ExportToXML(program, "MyProject")
// or the project structure: plcopen.Export(program, "MyProject")
// or file to file: plcopen.ConvertIECToXMLFile("in.st", "out.xml")
```

POUs (programs, function blocks, functions) are exported with their
variables and their bodies as text (ST or IL); data types become
`<dataType>` entries.

## Import

```go
text, err := plcopen.ImportToIECText(xmlData)   // source text
program, err := plcopen.ImportToAST(xmlData)    // parsed
project, err := plcopen.Import(xmlData)         // the XML structure
text, err := plcopen.ConvertXMLToIECText("project.xml")
```

POUs with **FBD or LD bodies** are lowered to ST statements
(`LowerGraphical`), as Beremiz does:

- a function block becomes a call, its outputs read as `inst.OUT`; an
  unnamed standard block (`R_TRIG`, `TON`, ...) gets a generated instance;
- a function becomes an operator (`ADD` → `+`, `GT` → `>`) or a call;
- an output variable, in/out variable or coil becomes an assignment;
- contacts in series are `AND`, wires joining at one input are `OR`,
  negation is `NOT(...)`, set/reset coils become `IF`, rising and falling
  edges use generated `R_TRIG`/`F_TRIG` instances;
- a block with `EN` wired runs only when it is TRUE; its outputs (and a
  function's result) are assigned under `IF`, and `ENO` can be wired on.

Statements run in `executionOrderId` order when every element has one;
otherwise networks run top to bottom, each in dataflow order (a loop is
cut at its topmost statement, with a warning).

Not converted yet, and the import fails naming the POU: jumps, labels and
returns; graphical SFC; a function with `EN` whose result feeds a block
under a different enable.

## Validation

| Call | Checks |
|---|---|
| `ValidateProject(project)` | Structural checks of a project (names, types, bodies) |
| `ValidateWithXSD(xmlData, xsdPath)` | Against the XSD with `xmllint` when it is on the PATH (`XSDValidatorAvailable()`); otherwise only the structural checks |
| `SchemaXSD()` | The embedded `tc6_xml_v201.xsd` |

## Command line

```bash
beedance -iec program.st -to-xml program.xml
beedance -from-xml program.xml -iec program.st
```
