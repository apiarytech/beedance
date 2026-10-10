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
variables and their bodies: ST and IL as text, and bodies written in the LD
or FBD text form (doc/language.md) as **graphical diagrams** other IEC
tools draw. A diagram is laid out as beedance draws it (package `diagram`,
`diagram.XML`), with left and right power rails for each rung and named
connectors for FBD wires. A function contact in a rung (`EQ(Mode, 2)`) is
written as an AND of the rung's power and the function's result. The
POU's interface declares the instances the body declares, not the helpers
beedance's lowering adds (edge detectors). Data types become `<dataType>`
entries. Rung names (`RUNG sealin`) are not kept: TC6 has no place
for them.

**SFC bodies** are exported as graphical charts (`diagram.SFCXML`), laid
out as beedance draws them: steps, transitions with their conditions as
inline ST (as written in the source), selection and simultaneous
divergences and convergences, a loop back as a jump, and action blocks.
`ACTION`s become the POU's `<actions>`, in ST. A selection's transitions
get priorities in the order the text lists them, so an import (below)
gives the first precedence, as IEC 61131-3 does; the text form fires every
transition whose condition holds. A chart the graphical form cannot hold (a
step with statements of its own, more than one initial step) is exported
in the text form, as ST.

## Import

```go
text, err := plcopen.ImportToIECText(xmlData)   // source text
program, err := plcopen.ImportToAST(xmlData)    // parsed
project, err := plcopen.Import(xmlData)         // the XML structure
text, err := plcopen.ConvertXMLToIECText("project.xml")
```

POUs with **FBD or LD bodies** can stay diagrams:

```go
text, err := plcopen.ImportToIECTextOptions(xmlData, plcopen.ImportOptions{KeepDiagrams: true})
```

writes them in the text form (`LD ... END_LD`, `FBD ... END_FBD`; package
`diagram`, `diagram.Format`), so they can be drawn, edited and exported
again. FBD becomes a netlist, which holds any FBD. LD becomes rungs of
contacts in series and `[ | ]` branches; crossing branches of plain
contacts are written by repeating those contacts, which have no state. A
body the text form cannot hold (a crossing through an edge contact or a
function block, a function powered through EN in a rung, an edge on a block
input) is lowered to ST instead, with a comment giving the reason.

Without `KeepDiagrams` (and with `ImportToIECText`), FBD and LD bodies are
lowered to ST statements (`LowerGraphical`), as other IEC tools compile them:

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

**SFC bodies** are written in the text form of SFC (`INITIAL_STEP`,
`STEP`, `ACTION`, `TRANSITION FROM ... TO ...`), which every engine runs
(`ReadSFC` returns the chart, a `diagram.Chart`):

- a transition's source steps are found through selection divergences and
  simultaneous convergences, its target steps through selection
  convergences, simultaneous divergences and jumps;
- a condition may be inline ST, a named transition of the POU (ST, as
  `expr`, `:= expr;` or `Name := expr;`, or an LD or FBD body assigning the
  transition's name), or a network of the chart wired into the
  transition; `negated` conditions become `NOT (...)`;
- transitions leaving the same steps are a selection: the first by
  `priority`, then left to right, wins, so a later one's condition is
  ANDed with `NOT` of those before it;
- an action block's actions become associations (`Name(Q);`,
  `Name(L, T#2s);`); the POU's named actions (ST, or LD and FBD lowered to
  ST) become `ACTION`s, and an inline action an `ACTION` named after its
  step (`_Fill_1`).

Not converted yet, and the import fails naming the POU: jumps, labels and
returns in LD and FBD; a function with `EN` whose result feeds a block
under a different enable; macro steps; IL actions and conditions; a chart
with more than one initial step, or a condition that needs statements
(an edge contact, say) rather than one expression.

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
