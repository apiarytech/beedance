# CLAUDE.md

Guidance for Claude (and other coding agents) working in this repository.

## What this is

beedance is an IEC 61131-3 toolchain in Go: one Pratt parser for ST, IL,
SFC, LD and FBD (text and PLCopen XML), lowerings shared by every engine,
and three engines held to the same results: the tree-walking `evaluator`,
the bytecode `compiler` and `vm`, and the `transpiler` to Go (royaljelly).
Around them: `vmdebug` and `dap` (the debugger), `sil` (unit tests, `TEST_`
programs, compared across engines), `analyzer` (lint), `diagram` (LD, FBD
and SFC as SVG), `ide` (language queries), `embedded` (VM firmware for
Pico and ESP32), and `wasm` (the toolchain as WebAssembly).

Start with `doc/architecture.md` (the pipeline and every package) and
`doc/development.md` (build, CI, conventions).

## Current initiative: the web debugger

A browser IDE in the style of Wokwi or Tinkercad Circuits: an editor, a
canvas of virtual field devices wired to the PLC's I/O, and the debugger,
all on the VM compiled to WebAssembly. **The plan is
`doc/web-debugger.md`: read it before working on this, and keep it current
(mark what lands as built, record decisions and measurements there).**

The rules it rests on:

- **The page speaks DAP.** The browser talks to the same `dap.Server`
  `beedance -dap` runs, through `wasm.DAP` (whole messages, no framing)
  and `globalThis.beedance.dap` in the GOOS=js module. Do not invent a
  second debug protocol: add a standard DAP request, or a `beedance/...`
  custom request in `dap`, with a test in `dap/server_test.go`.
- **A running program must yield.** GOOS=js is single-threaded and Go does
  not preempt there. `vmdebug.Options.YieldEvery`/`Yield` make a free run
  give up the processor between scans; the web module sets them
  (`wasm.DAPYield`). Never add a loop in the run path that can spin
  without reaching that yield. `runtime.Gosched` and sub-millisecond
  timers do not return to the JS event loop.
- **No files in the browser.** Anything the debugger reads from disk needs
  a text alternative (as `launch` `source` and `beedance/onlineChange`
  `source`). `filepath.Abs` and `os.Getwd` fail in a browser.
- **Simulated time.** Scans advance the session's clock by the interval;
  plant models (planned) run in the VM on that clock, not in JavaScript.
- **Front end**: `wasm/sim/` (planned), plain ES modules and Web
  Components, no build step, unless the open question in the plan is
  settled otherwise. The existing `wasm/web/` playground stays as it is.

## Commands

```bash
go build ./... && go vet ./... && go test ./...
go test -race -count=1 ./vmdebug/ ./dap/ ./wasm/
gofmt -l .                                   # must print nothing

# the web module, and the debugger in it under Node.js
GOOS=js GOARCH=wasm go build -o wasm/web/beedance.wasm ./wasm/cmd/beedance-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/
timeout 60 node wasm/node/dap.mjs wasm/web/wasm_exec.js wasm/web/beedance.wasm
GOOS=js GOARCH=wasm go vet ./wasm/...        # the js-only files

beedance -dap                                # the adapter on stdio (VS Code)
beedance -test FILE.st                       # TEST_ programs on every engine
```

Go 1.27.2 is required (`go.mod`). The `go` engine of the `sil` tests
(`sil`, `sfc`, `examples/unit_tests`, `transpiler`, `oscatdiff`) builds
transpiled programs with the Go toolchain and fetches royaljelly, so it
needs that Go and network access; elsewhere those tests fail for that
reason alone. `wasm_exec.js` must come from the Go that built the module.

## Conventions

- **Every package is in `doc/architecture.md`'s table and drawn in
  `doc/architecture.svg`.** `architecture_test.go` fails otherwise; a new
  package needs both.
- Every exported identifier has a doc comment. Every Go file starts with
  the license header the others have (copyright Franklin D. Amador,
  GPL v2.0 or commercial).
- A language feature lands in the parser and in every engine, with tests
  on each, or the engines drift apart.
- No third-party runtime dependencies in Go (go-cmp in tests only). In the
  front end, a dependency is vendored with its license, and development
  tools (Playwright) stay out of what is shipped.
- Errors carry the source position where there is one.
- Docs and comments are plain and terse: short declarative sentences,
  tables for reference, no marketing. Match the surrounding text.
- Tests: table-driven where it fits; the debugger's tests drive a real
  session (`vmdebug_test.go`) or a real adapter over pipes
  (`dap/server_test.go`, `wasm/dap_test.go`).
- Commits: one subject line in the imperative ("Add ...", "vm: ...",
  "doc/cli.md: ..."), the body saying why.

## Map of the debugger

| Where | What |
|---|---|
| `vm/debug.go` | `SetHook`, `ErrPaused`, `Frames`: the VM stops before an instruction and resumes |
| `compiler/lines.go`, `object.SourceLine` | the line table the debugger maps instructions to statements with |
| `vmdebug/session.go` | `Session`: run modes, breakpoints, the yield, scans and the clock |
| `vmdebug/values.go` | variables, locals, evaluate, set, force, `Live` samples |
| `vmdebug/change.go` | online change: carrying variables over |
| `vmdebug/code.go` | disassembly, stepping by instruction |
| `dap/` | the Debug Adapter Protocol over a `Session` |
| `wasm/dap.go` | the adapter by whole messages, for a page |
| `wasm/cmd/beedance-wasm/main_js.go` | `globalThis.beedance.handle` and `.dap` |
| `wasm/node/dap.mjs` | the debugger in the web module under Node.js |
| `ide/live.go`, `diagram/` | live LD, FBD and SFC drawings (for phase 4) |
