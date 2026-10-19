# Debugging

beedance debugs a `PROGRAM` on the VM as a PLC runs it, scan by scan:
breakpoints on its lines, stepping through statements, calls and scans,
its variables, forcing inputs and variables, and changing the program
while it runs (online change). `beedance -dap` serves this over the Debug
Adapter Protocol, so it works in VS Code and in any editor that speaks it;
package `vmdebug` is the same as a Go API.

## In VS Code

The beehive Studio extension (`beehive/editors/vscode`) registers the
debugger. Put `beedance` on the PATH, or set `beehive.beedance` to its
path, open a `.st` file and press F5: it runs the file's first `PROGRAM`
that is not a test. A `launch.json` chooses more:

```json
{
  "type": "beedance",
  "request": "launch",
  "name": "Debug the pump",
  "program": "${workspaceFolder}/pump.st",
  "programName": "Pump",
  "interval": "100ms",
  "realtime": true,
  "stopOnEntry": false
}
```

| Attribute | Default | |
|---|---|---|
| `program` | the file in the editor | the `.st` file |
| `programName` | the first `PROGRAM` that is not a `TEST_` | the `PROGRAM` to run |
| `interval` | `10ms` | the scan time: the clock moves this much each scan |
| `realtime` | `true` | run the scans an interval apart on the wall clock; `false` runs them as fast as they go |
| `stopOnEntry` | `false` | stop before the first scan |
| `allowInputWrites` | `false` | let a POU assign its own inputs, as CODESYS does (OSCAT) |
| `source` | | the program's text, for a host with no files (a web page); `program` then only names it |

The clock is simulated: it moves `interval` at the start of each scan, and
stands still while the program is stopped, so a timer stopped at a
breakpoint has run exactly as long as the scans before it.

What VS Code's debugger shows:

- **Breakpoints** on statement lines, with a condition in Structured Text
  (`n > 3 AND NOT pump.running`) on the program's variables and the locals
  of the call it stops in. A breakpoint on a line with no statement moves
  to the next statement of the same POU.
- **Stepping**: Step Over, Step Into a function block or function, Step
  Out of it; stepping past the body's last statement goes on into the next
  scan. Standard blocks (TON, CTU) are stepped over, as their code is not
  in the source.
- **Call stack**: the program's body and the function blocks, functions
  and methods it is in.
- **Variables**: the program's (instances and arrays opened member by
  member, located ones with their address, forced ones marked), the
  locals of the selected call (in a function block, its instance's
  variables), and the forces. Each can be written: double-click it.
- **Hover and Watch**: any Structured Text expression.
- **Disassembly**: Open Disassembly View shows the bytecode beside the
  source, each instruction with its statement; stepping there goes one
  instruction at a time.
- **Pause** stops the program at its next statement.

The PLC's own operations are commands (`beedance:` in the command palette,
two of them on the debug toolbar) and debug console commands:

| Command | Debug console | |
|---|---|---|
| Force a Variable or Address | `force NAME VALUE` | keep `VALUE` on a variable, a member (`b.PT`) or an address (`%IX0.0`) |
| Unforce | `unforce NAME` | end a force; the value stays until the program changes it |
| | `forces` | list the forces |
| | `set NAME VALUE` | write once |
| Run to the End of the Scan | `scan` | stop between scans |
| Online Change | `change` | save the file and load it into the running program |
| | `live` | the watched values as the last scan left them, while it runs |

Values are written as the source writes them: `TRUE`, `42`, `16#FF`,
`1.5`, `T#2s`, `'text'`.

## Forcing

A force holds a value until it is removed: it is written before every scan,
so the program reads it as it would a forced input, and after every scan,
so the outputs and anyone watching see it, as a forced output. In the scan,
the program may still change the variable; its change does not last past
the scan's end. Forces, and `set`, given while the program runs take
effect when the next scan starts.

## Online change

`change` (or the Online Change command) replaces the program with the file
as it is now, between two scans, without stopping it. The variables carry
over by name and declared type; a function block instance member by
member, so a timer keeps its elapsed time. What does not carry over is
reported:

- **reset**: its type changed, it is a constant, or it holds a reference;
  it starts at its initial value;
- **added**, **removed**: variables of only the new, or only the old,
  program;
- **unforced**: forces whose variable is gone or no longer takes their
  value.

The I/O image, the clock, the forces and the breakpoints carry over too.
While the program runs, the change waits for the end of the scan in
progress; stopped inside a scan, run to its end first (`scan`).

## Other editors

`beedance -dap` speaks the protocol on stdin and stdout (`-dap-log FILE`
records every message). Launch arguments are those of `launch.json` above;
the PLC commands are custom requests: `beedance/force` and
`beedance/unforce` (`{"name", "value"}`), `beedance/forces`,
`beedance/stepScan`, `beedance/onlineChange` and `beedance/live`.

For a simulator's virtual field devices, three more:

| Request or event | |
|---|---|
| `beedance/io` | the I/O image, `{"scan", "elapsedMs", "io": {"%IX0.0": true, "%QW0": 70}}`: as it is when stopped, as the last scan left it while running |
| `beedance/setInputs` `{"values": {"%IX0.0": true}}` | inputs for one scan, queued: each call is taken by the next scan to start, so a press and release faster than a scan is still seen; a force wins over them |
| `beedance/scan` (event) | with `"scanEvents": MS` in `launch`, the watched values and the I/O image (as `beedance/live`) at most every MS of wall time while the program runs, and once at launch |

`beedance/live` replies `{"scan", "elapsedMs", "values": [{"name", "type",
"address", "text", "forced"}], "io"}`.

And for simulation:

| Request | |
|---|---|
| `beedance/speed` `{"factor": 2}` | how fast simulated time runs against the wall clock while the program runs: 1 real time, 0 as fast as it goes; also `"speed"` in `launch` |
| `beedance/trace` `{"signals": [...], "depth": 4000, "scans": 500}` | record the signals (variables, addresses, `PLANT_Name.var`) at the end of every scan, keeping the last `depth`; replies the last `scans`: `{"signals", "scans", "timesMs", "values"}` |
| `beedance/diagrams` `{"prefixes": {"Motor": "m"}}` | the LD, FBD and SFC bodies' live views (package ide): `{"program", "diagrams": [{"pou", "lang", "prefix", "live": {"values", "exprs", "steps"}}]}`; a function block's body is seen through `prefixes`' instance, else the program's first of its type |

## Plant models

A source's `PROGRAM`s named `PLANT_...` are plant models: they simulate
what the program controls (a tank, a conveyor) in Structured Text. The
debugger runs each after every scan of the program, on the same simulated
clock and I/O image: a plant reads the outputs and writes the inputs
through its located variables (`level AT %IW0 : INT`). It is not stopped
by breakpoints; while stopped, its variables are a scope of their own
(`PLANT PLANT_Tank`), read-only. Online change carries its variables as
the program's, so a running plant keeps its state while the program is
tuned. The PROGRAM debugged by default is the first that is neither a
test nor a plant model.

## In a web page

The WebAssembly build of beedance serves the same adapter to a page, one
message at a time (`globalThis.beedance.dap`, see
[wasm](../wasm/README.md#debugging)): `launch` takes the program's text as
`source`, and `beedance/onlineChange` the new text. What a page needs to
debug in a browser, and to simulate a plant with virtual field devices, is
in [Web debugger](web-debugger.md).

## From Go

```go
s, err := vmdebug.Load(source, vmdebug.Options{Interval: 10 * time.Millisecond})
s.SetBreakpoints([]vmdebug.BreakpointSpec{{Line: 12, Condition: "n > 3"}})
stop, err := s.Continue(ctx)       // stop.Reason == vmdebug.OnBreakpoint
for _, f := range s.Frames() {     // innermost first
	fmt.Println(f.Name, f.Line)
}
s.Force("%IX0.0", "TRUE")
stop, err = s.StepScan(ctx)        // to the end of the scan
report, err := s.Change(edited)    // online change
```

The debugger runs on what the compiler keeps for it: each statement's
place in the bytecode (`Bytecode.Lines`, `object.CompiledFunction.Lines`)
and the names of functions and their locals. The VM stops through a hook
(`vm.SetHook`): `Run` returns `vm.ErrPaused`, and the next `Run` carries on
where it stopped. Without a hook the VM runs as before.

## Attached to a host's program

A host that runs programs itself, on VMs of its own (beehive's logic:
each program on royaljelly's scheduler, bound to a node's tags), can put a
program under the debugger as it runs, without stopping it: `vmdebug.Attach`
takes the host's compiled program, its globals store, its cyclic VM and
its clock (`vmdebug.Host`) and returns a session with no VM or clock of
its own. The host keeps its cycle; before each one it asks the session
what to do (`State`), and in place of resetting and running its VM it
calls `Scan`:

| `State()` | The host's cycle |
|---|---|
| `HostScan` | as usual: the inputs, `Scan()`, the outputs |
| `HostResume` | the scan the program stopped in goes on: `Scan()`, then the outputs; the inputs stay as the scan read them |
| `HostHold` | nothing: the program is stopped, and its clock stands still, as under a hold |

A breakpoint stops the program where it is, in the middle of a scan, with
its outputs unwritten until the scan completes; the cycles meanwhile are
skipped, as a held program's are. `Continue` and the steps do not run the
VM: they let the program go on and wait for the host's cycles to stop it,
so each step takes one cycle of the host's task. A step that reaches the
end of the scan stops between scans (`OnScanEnd`), where the outputs have
been written. `Pause` takes effect at the program's next statement when
the hook is in (a breakpoint, a step), else before its next scan. Forces
and writes are applied by `Scan` as they are in a session of `Load`'s.
Without a breakpoint, a step or a pause, `Scan` runs the VM with no hook,
at its full speed; a breakpoint set while it runs is seen from the next
scan. `Detach` lets the program go: a run waiting for a stop gets
`OnDetach`, a scan it was stopped in completes in the host's next cycle,
and `Scan` returns `ErrDetached` after it, on which the host drops the
session. A run's error (`OnException`) is also returned to the host, which
faults as it would without the debugger.

What the host does is refused on an attached session: an online change
(`ErrAttached`: beehive's `Apply` makes those, and restarts the program,
which ends the session) and a simulator's inputs; plant models do not run
(the plant is real). A library the host compiles before the file the
editor shows offsets its lines: `Host.LineOffset` makes the lines given
and reported the file's.

Over the protocol, `attach` takes `{"program", "file", "source",
"stopOnEntry"}`: `dap.Server.Attach` is the host's hook that returns the
session, `file` names the editor's source for the stack trace and the
breakpoints, and `source`, if given, lets the host refuse a file that is
not what the program runs from. `disconnect` and `terminate` detach; the
program runs on. `beedance/onlineChange`, `beedance/speed` and
`beedance/setInputs` are refused. beehive serves this through its gateway
(`LogicService.Debug`, decision D-61 there), for VS Code's attach
configuration.

## Not yet

- One source file per session.
- The firmware `-embedded` generates carries no line table, so it cannot
  be debugged at source level on the board.
