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

POUs with **graphical bodies** (FBD, LD, or SFC in its graphical form)
cannot be converted to text, and the import fails naming the POU.

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
