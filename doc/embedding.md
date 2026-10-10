# Embedding beedance

A host (a soft-PLC, a SCADA node, a test tool) runs programs on its own
schedule, binds their variables to its own data, and must not be stalled by
a faulty program. These APIs serve that.

## Running a program in scans (VM)

```go
object.FinalizeBuiltins()
prog, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(source, "Mixer")

globals := make([]object.Object, vm.GlobalsSize)
init := vm.NewWithGlobalsStore(prog.InitBytecode, globals)
scan := vm.NewWithGlobalsStore(prog.CyclicBytecode, globals)

// The task's clock: timers measure time since the task started.
origin, now := time.Now(), time.Now()
for idx, b := range stdlib.ClockBuiltins(func() time.Duration { return now.Sub(origin) }) {
    init.SetBuiltin(idx, b)
    scan.SetBuiltin(idx, b)
}
scan.SetBudget(10_000_000) // instructions per scan

if err := init.Run(); err != nil { /* initialization failed */ }
for addr, v := range init.IO() {
    scan.IO()[addr] = v // located variables' initial values
}

for t := range ticker.C {
    now = t
    // inputs: write variables (see below)
    scan.Reset()
    if err := scan.Run(); err != nil {
        // a runtime error or vm.ErrBudget: stop the program, as a PLC faults
    }
    // outputs: read variables
}
```

| API | Purpose |
|---|---|
| `VM.Reset()` | Prepares the VM to run its bytecode again: one VM per program, reused every scan |
| `VM.SetBuiltin(index, b)` | Replaces a built-in for this VM only |
| `VM.SetBudget(n)` / `vm.ErrBudget` | Bounds the instructions of each run (from one `Reset` to the next); a scan past it fails with `ErrBudget` instead of looping for ever |
| `VM.Globals()`, `VM.IO()` | The global slots and the I/O image of located variables |
| `vm.New(b, vm.WithGlobalsSize(n))` | A VM with `n` global slots instead of `vm.GlobalsSize` (65536, 512 KB on a 32-bit target), for a microcontroller's RAM; a global past `n` fails the run |
| `vm.GlobalsNeeded(b)` | The global slots bytecode `b` uses. For a store shared by init and scan, size it to the larger of the two, and to the `Global` slot of any variable the host binds |
| `stdlib.ClockBuiltins(clock)` | `__CLOCK` (read by the standard timers) and `TIME()` reading the host's clock, by built-in index |

## Binding variables

`CompiledProgram.Variables` lists the program's variables a host may bind,
in declaration order:

```go
type Variable struct {
    Name     string
    Block    string // VAR_GLOBAL, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT or VAR
    Type     string // as declared: "INT", "TON", "Motor", ...
    Global   int    // slot in VM.Globals(); -1 for a located variable
    Address  string // "%IX0.0" for a located variable
    Retain   bool
    Constant bool
}
```

- A non-located variable lives in `globals[v.Global]`; a located one in
  `scan.IO()[v.Address]`.
- A direct variable a program uses in statements (`x := %IW0;`) is listed
  too, named by its address (`Name` and `Address` both `"%IW0"`, `Block`
  `VAR`, `Type` from its size: `WORD`), unless a located variable is
  declared at that address; a host binds it like a located variable.
- Values are `object` values. Convert them to and from the host's types
  by the **declared** `Type`: the VM may hold a `REAL` member as an
  `LREAL`, so the runtime value's type is not the declared one.
- A STRUCT or function-block instance is an `*object.Hash` keyed by member
  name; an ARRAY is an `*object.Array` with its `LowerBound`.
- A typical host reads inputs into variables before each scan and writes
  outputs after it, and treats `Retain` variables as persisted.
- Macro variables are left out: they have no runtime value.

## Native programs (transpiler)

With `Options{HostBinding: true}`, the transpiler also generates, for each
PROGRAM:

```go
func NewMixer() *Mixer                      // the program with its initial values
func (p *Mixer) Variables() []core.Variable // its elementary variables
func (p *Mixer) Logic(now time.Time)        // one scan (always generated)
```

`core.Variable` (royaljelly) has the same name, block, type, address,
`Retain` and `Constant` as above, and `Get() any` / `Set(v any) bool`
closures that read and write the field without reflection; `Set` refuses a
value of the wrong Go type. Function-block instances are not listed. Call
`Get` and `Set` only between scans.

## Engineering sessions

The evaluator can simulate scans on a clock of the host's, record each
statement, and stop runaway code:

```go
end := evaluator.Session(func() time.Time { return simulated }) // the evaluator's clock
defer end()
evaluator.SetBudget(1_000_000) // statements and loop iterations; reset when the session ends
evaluator.Trace(func(stmt ast.Statement, env *object.Environment) {
    // called before each statement, also inside called blocks and functions
})
result := evaluator.Eval(program, env)
io := evaluator.IO() // located variables by address
```

- One session at a time: the clock and the I/O image belong to the
  package, so `Session` waits for the previous one to end.
- `Trace` gives what an engineering tool needs for stepping: what changes
  between two calls is the effect of the first call's statement (for an IF
  or a loop, of its condition). Breakpoints are a condition checked in the
  trace function.
- Past the budget, evaluation returns an "execution budget exceeded" error.

## Unit tests (software-in-the-loop)

Package `sil` runs the `PROGRAM TEST_...` unit tests of a source on a
simulated clock, on the evaluator, the VM or both, and returns the results
to the caller. `beedance -test` (see [Command line](cli.md#unit-tests)) is a
thin report over it; a Go test or a CI tool can call it directly:

```go
results, err := sil.Run(ctx, source, sil.Options{
    Interval: 10 * time.Millisecond, // simulated time between scans
    Engines:  []sil.Engine{sil.Evaluator, sil.VM}, // the default: both, compared
})
// err: a *sil.ParseError, sil.ErrNoTests, or ctx's error
for _, r := range results {
    if !r.Passed {
        // r.Mismatch: the first scan and variable where the engines differ
        for _, e := range r.Engines {
            fmt.Println(r.Name, e.Engine, e.Problems()) // e.Scans: values after each scan
        }
    }
}
```

Each engine gets its own simulated clock (an evaluator `Session`; for the
VM, `stdlib.ClockBuiltins`) and a per-scan budget, so a runaway loop fails
its test. `sil.WriteCSV` writes an engine's scans for plotting or diffing.

## Watching function block outputs

Package `watch` names what a host can watch of a program, the same way on
both engines: its variables of elementary types and, under dotted names,
the outputs of its function block instances (inherited ones too) and the
members of its structures, e.g. `pid.MV` or `pt.x`. `sil` records these;
a simulator or trace view can do the same:

```go
vars := watch.Variables(program, decl, watch.Options{
    Members: watch.Outputs, // or watch.All: inputs and internal variables too
    Depth:   2,             // pid.MV is 1 level, pid.tSample.ET 2
})
for _, v := range vars {
    val, ok := watch.Evaluator(prog.Env, v)   // tree-walking evaluator
    val, ok = watch.VM(globals, slots, v)     // VM: slots by upper-case name
    _, _ = val, ok
}
```

`v.Type` is the declared elementary type, to convert the value by (see
Binding variables). Arrays are not listed. A standard function block lists
its inputs and outputs only, not the internal state the engines keep
differently; `compiler.StandardFunctionBlock(name)` returns its declaration.

## A complete host

beehive's `logic` service is a full example: it compiles downloaded ST on
the node, runs each program on royaljelly's scheduler with the VM (or a
native build), binds variables (including members of structures, arrays
and function blocks) to honeycomb tags, keeps `RETAIN` values across
restarts and downloads, and faults a program that exceeds its budget. Its
`engineering` service uses evaluator sessions for simulation, traces,
breakpoints and unit tests written in ST.
