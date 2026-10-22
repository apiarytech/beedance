# wasm: beedance as WebAssembly

Package `wasm` puts beedance's toolchain behind a single JSON call: the
parser, the evaluator, the bytecode compiler and VM, the transpiler and the
`TEST_` runner (package `sil`). `cmd/beedance-wasm` builds it as a
WebAssembly module in two forms:

| Build | Host | What it gives |
|---|---|---|
| `GOOS=js` | a web page, or Node.js | `globalThis.beedance.handle(request)`: the [playground](web/) |
| `GOOS=wasip1` | a WASI runtime | JSON requests over stdin and stdout, and `test`: software-in-the-loop tests |

The module needs no Go toolchain to run, so the `go` and `wasm` engines of
`beedance -test` are not available in it. Tests run on `eval` and `vm`, and
the two engines are compared.

This is the toolchain running as WebAssembly. The other way round, the
`wasm` engine of `beedance -test` builds a *program* for WebAssembly: its
transpiled Go, for WASI, run by a WASI runtime and compared with the other
engines scan by scan (see [doc/cli.md](../doc/cli.md#unit-tests)).

## Requests

Each request is one JSON object, and so is each reply. A reply has `"error"`
set when the op failed, and `"errors"` lists any parse errors.

| Request | Reply |
|---|---|
| `{"op":"version"}` | Go version, `goos`/`goarch`, the ops |
| `{"op":"check","source":S}` | `ok`, `errors`, `programs`, `tests`; `pous` (kind, name, line: an outline) and `located` (each PROGRAM's variables at an address) |
| `{"op":"test","source":S,"engines":["eval","vm"],"filter":"","intervalMs":10,"maxScans":50000,"tolerance":1e-3,"scans":false}` | `passed`, `failed`, and per test the result on each engine; with `scans`, the values after every scan |
| `{"op":"transpile","source":S}` | `go`: the Go source on royaljelly |
| `{"op":"exportXML","source":S,"name":N,"sim":JSON,"from":X}` | `xml`: the source as a PLCopen project (IEC 61131-10, TC6 XML), the simulator's settings `sim` as additional data under `https://apiarytech.io/beedance/sim`; `from`, the XML it was imported from, keeps other tools' additional data |
| `{"op":"lint","source":S}` | `findings`: line, column, severity (info, warning, error), rule, PLCopen, message (package analyzer) |
| `{"op":"hover","source":S,"line":L,"column":C}`; `complete`, `definition`, `references`, `rename` (`"newName"`) | hover's `text` and `symbol`; completion's `items`; a `span`; the `spans` to change (package ide; columns in bytes, from 1) |
| `{"op":"diagrams","source":S}` | `diagrams`: each LD, FBD and SFC body: its POU, language, model, SVG and the names its live view reads |
| `{"op":"importXML","xml":X}` | `source`, `name`, and `sim` if the project has it; if `sim` holds the source and the POUs are still what it makes, that text, comments and all |
| `{"op":"load","source":S,"program":"Main","intervalMs":10}` | `session`, the `variables` (name, type, block, address), their `values` and the `io` image |
| `{"op":"scan","session":N,"count":1,"values":{...}}` | sets `values`, runs `count` scans; `scans`, `timeMs`, `values`, `io` |
| `{"op":"set","session":N,"values":{"start":true,"%IX0.1":true,"limit":10}}` | as scan, without scanning |
| `{"op":"read","session":N}` | as scan, without scanning |
| `{"op":"unload","session":N}` | ends the session |

`load` compiles a PROGRAM to bytecode and runs its initialization. When
`program` is not given, it takes the first PROGRAM that is not a test. Each
scan advances a simulated clock by the interval, so `TON` with `PT := T#1h`
runs in milliseconds. Values are set by variable name, or by address for
located and direct variables, and are converted to the declared type. TIME
values are strings such as `T#1s500ms`.

## Debugging

The web build also gives `globalThis.beedance.dap(onMessage, options)`:
the debugger `beedance -dap` serves ([doc/debugging.md](../doc/debugging.md);
`launch` takes `engine`, `vm` or `eval` in a module),
by whole Debug Adapter Protocol messages, without the `Content-Length`
framing. It returns `{send(json), close()}`, and calls `onMessage(json)`
with each response and event; `close()` ends the session and frees it.
`options` is optional: `{yield: "message"}` lets the page in between scans
by a MessageChannel message, which browsers schedule more evenly than the
default 1 ms sleep (see [doc/web-debugger.md](../doc/web-debugger.md#the-yield));
under Node.js keep the default.

```js
const dbg = beedance.dap((msg) => console.log(JSON.parse(msg)));
dbg.send(JSON.stringify({ seq: 1, type: 'request', command: 'initialize', arguments: {} }));
dbg.send(JSON.stringify({ seq: 2, type: 'request', command: 'launch',
  arguments: { program: 'main.st', source: text, realtime: false } }));
```

`launch` takes the program's text as `source` (a page has no files);
`program` then only names it, and breakpoints are set on that name.
`beedance/onlineChange` takes the new text as `source`. While the program
runs, the debugger lets the page's event loop in between scans (every
16 ms, `DAPYield`), so requests are answered and timers fire. Package
`wasm`'s `DAP` is the same, natively.

```bash
node wasm/node/dap.mjs wasm/web/wasm_exec.js wasm/web/beedance.wasm   # under a timeout
```

debugs a program in the web module under Node.js, and fails if the
running program starves the event loop. What a page gets from the
module, and the yield measured in browsers, is
[doc/web-debugger.md](../doc/web-debugger.md).

## Build

From the beedance module root:

```bash
GOOS=js GOARCH=wasm go build -o wasm/web/beedance.wasm ./wasm/cmd/beedance-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/
GOOS=wasip1 GOARCH=wasm go build -o beedance-wasi.wasm ./wasm/cmd/beedance-wasm
```

`wasm_exec.js` must come from the same Go that built the module. The
module is about 17 MB, or 4 MB gzipped (Go 1.27).

A page that edits, checks and debugs, and never runs the tests or the
transpiler, can load a smaller module: `-tags lite` leaves out `test`
(package sil) and `transpile`, and the `version` reply's `ops` lists what
the build has; its `engines` lists the debugger's engines a `launch` may
name (`vm` and `eval`, in either build). With `-ldflags="-s -w"`
too, it is about 12.5 MB, or 3.2 MB gzipped:

```bash
GOOS=js GOARCH=wasm go build -trimpath -tags lite -ldflags="-s -w" -o beedance.wasm ./wasm/cmd/beedance-wasm
```

Built natively (`go run ./wasm/cmd/beedance-wasm`), the command speaks the
same stdin/stdout protocol as the WASI build, which helps when debugging.

## Docker

```bash
docker build -f wasm/Dockerfile -t beedance-wasm .                     # SIL (default)
docker build -f wasm/Dockerfile --target web -t beedance-wasm-web .    # playground

# software-in-the-loop: the TEST_ programs of the files, or of stdin
docker run --rm -v "$PWD/examples/unit_tests:/work" beedance-wasm test motor.st motor_tests.st
cat lib.st tests.st | docker run --rm -i beedance-wasm test -engines vm -run Motor

# the JSON protocol, one request per line
docker run --rm -i beedance-wasm serve < requests.jsonl

# the playground on http://localhost:8080
docker run --rm -p 8080:8080 beedance-wasm-web
```

`test` exits with 0 when every test passes, 1 when one fails and 2 when the
source cannot run, so it can gate a CI job. Its flags are `-engines`,
`-run`, `-interval-ms`, `-max-scans` and `-tol`.

The `sil` image runs the modules on Node.js 24, through
[node/wasi.mjs](node/wasi.mjs) and [node/js.mjs](node/js.mjs). Node 20 and
22 at times crash (SIGSEGV) in their WASI support after a long run.

## Tests

```bash
go test ./wasm/...                                          # the API and the command, natively
docker build -f wasm/Dockerfile -t beedance-wasm .
go test -tags wasmsil ./wasm/                               # the modules in the image
```

The `wasmsil` tests send the same requests to the native package, the WASI
module and the web module, and require identical replies.

## License

This folder and the modules built from it are dual-licensed, like the rest
of beedance: the GNU General Public License version 2 (or, at your option,
any later version) or a commercial license. See [LICENSE.md](LICENSE.md),
which also covers the Go runtime inside the modules and `wasm_exec.js`,
which keep Go's own BSD-style license.
