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

## A complete host

beehive's `logic` service is a full example: it compiles downloaded ST on
the node, runs each program on royaljelly's scheduler with the VM (or a
native build), binds variables (including members of structures, arrays
and function blocks) to honeycomb tags, keeps `RETAIN` values across
restarts and downloads, and faults a program that exceeds its budget. Its
`engineering` service uses evaluator sessions for simulation, traces,
breakpoints and unit tests written in ST.
