# Development

## Requirements

Go 1.27.1 or later. No C compiler is needed except for the race detector.
TinyGo 0.42 for the microcontroller builds.

## Build and test

```bash
go build ./...
go vet ./...
go test ./...
go test -race -count=1 ./...
tinygo test ./token ./lexer ./ast ./code ./object ./parser ./stdlib
```

## Layout

| Path | Contents |
|---|---|
| `token/` ... `transpiler/` | The packages; see [Architecture](architecture.md) |
| `main.go`, `check_builtins.go` | The `beedance` command |
| `examples/` | `simple`, `traffic_lights`, `oop_example`, `edge_cases` (the last two with tests) |
| `embedded/` | Standalone VM program generator and the Pico example |
| `reference/` | Grammar notes, sample configurations, the OSCAT BASIC source used in tests |
| `doc/` | These pages |

## Tests

- Each package has unit tests; the backends share programs so the
  evaluator, the VM and the transpiled Go are held to the same results.
- The converted OSCAT BASIC library (`reference/beedance_oscat_basic.st`)
  is parsed, compiled, evaluated and run on the VM in tests.
- Host APIs have their own tests: `vm/vm_host_test.go`,
  `evaluator/session_test.go`, `transpiler/host_binding_test.go`.

## CI

`.github/workflows/go.yml`:

| Job | Checks |
|---|---|
| test | Build, vet and test on Linux, macOS and Windows, with Go 1.27 and stable |
| lint-race | gofmt, and the tests with the race detector |
| raspberry-pi | ARMv6, ARMv7 and arm64 under QEMU; uploads a `beedance` binary per target |
| tinygo-test | The core packages, compiler, vm and evaluator under TinyGo, with 1 MB goroutine stacks |
| raspberry-pi-pico | Firmware for `pico`, `pico-w`, `pico2`, `pico2-w`, uploaded as artifacts |
| pico-emulator | The Pico firmware and the rig on an emulated RP2040 (`embedded/picosim`) |
| esp32-emulator | The VM firmware on an emulated ESP32 (`embedded/esp32sim`) |

## Releases

Tags are `vX.Y.Z` (`-betaN` for pre-releases). The `version` constant in
`main.go` must match the tag. Tag only a commit whose CI is green.

## Conventions

- Every exported identifier has a doc comment.
- A language feature lands in the parser and in every backend, with tests
  on each, or the backends drift apart.
- Errors carry the source position where there is one.
- No third-party runtime dependencies.

## Known issues

- **`-uvm` does nothing.** The flag selects an engine that is not
  implemented.
- **TinyGo stacks**: under TinyGo the `compiler`, `vm` and `evaluator`
  tests need `-stack-size=1MB` (512 KB is enough today). TinyGo does not
  grow goroutine stacks, and the parser and compiler recurse deeply (see
  [Microcontrollers](embedded.md)).
- **VM speed**: values are boxed; typed fast paths would close part of the
  gap to native code.
- **README gaps**: the README does not list `-to-xml`, `-from-xml`,
  `-check-builtins` or the host APIs; these pages do.
- **Global evaluator state**: the evaluator's clock, I/O image, trace hook
  and budget are package-wide, so engineering sessions run one at a time.
