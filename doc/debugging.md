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

## Not yet

- One source file per session.
- Debugging a program running on a beehive edge node (attach) is
  beehive's to add, on this package.
- The firmware `-embedded` generates carries no line table, so it cannot
  be debugged at source level on the board.
