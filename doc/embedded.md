# Microcontrollers

beedance's core uses only the standard library, so parts of it build with
[TinyGo](https://tinygo.org) for microcontrollers.

## What runs under TinyGo

| Package | TinyGo (0.42) |
|---|---|
| `token`, `lexer`, `ast`, `code`, `object`, `parser`, `stdlib` | Tests pass (checked in CI) |
| `compiler`, `vm`, `evaluator` | Tests pass with `-stack-size=1MB` (checked in CI) |

TinyGo gives each goroutine a fixed stack and does not grow it, and every
test runs in a goroutine of its own. The parser and the compiler recurse
deeply: compiling the OSCAT library needs more than 256 KB, and a stack
that is too small corrupts memory instead of failing cleanly, which shows
up as random nil pointer dereferences. Hence the flag:

```bash
tinygo test -stack-size=1MB ./compiler ./vm ./evaluator
```

On a microcontroller, compile on the host and run only the VM (see below).
A program with nested function blocks, timers and function calls runs in
8 KB of stack, the Pico's default.

## Raspberry Pi Pico

`embedded/example/raspberrypi` runs a small bytecode program on the VM.
CI builds it as firmware for `pico`, `pico-w`, `pico2` and `pico2-w` and
uploads the `.uf2` files as artifacts.

```bash
tinygo build -target=pico -size short -o beedance-pico.uf2 ./embedded/example/raspberrypi/
tinygo flash -target=pico ./embedded/example/raspberrypi/
```

To run the firmware without a board, `embedded/picosim` builds it into a
Docker image that emulates the RP2040 and serves UART0 over TCP. See
[embedded/picosim/README.md](../embedded/picosim/README.md).

```bash
docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
docker run -d --rm -p 4000:4000 beedance-picosim
go test -tags picosim ./embedded/picosim/
```

## ESP32

TinyGo builds the same VM firmware for the ESP32 family: `esp32-coreboard-v2`
(ESP32), `esp32c3-generic`, `esp32s3-generic`, `xiao-esp32c6` and others
(`tinygo targets | grep esp`). The example takes about 180 KB of flash and
16 to 32 KB of RAM, of the chips' 400 to 520 KB.

`embedded/esp32sim` runs it on an emulated ESP32 (Xtensa) in Docker, under
Espressif's QEMU, with UART0 on TCP; CI runs it. The ESP32-C3 and S3 are
not emulated yet. See
[embedded/esp32sim/README.md](../embedded/esp32sim/README.md).

```bash
docker build -f embedded/esp32sim/Dockerfile -t beedance-esp32sim .
docker run -d --rm -p 4000:4000 beedance-esp32sim
go test -tags esp32sim ./embedded/esp32sim/
```

On a Raspberry Pi running Linux, the regular Go toolchain builds the whole
of beedance (ARMv6, ARMv7 and arm64 are tested in CI under QEMU).

## Generating a standalone VM program

`embedded.GenerateEmbeddedGo(w, bytecode)` writes a Go `main` package that
holds the bytecode and its constants as Go literals and runs them on the VM:

```go
comp := compiler.New()
err := comp.Compile(program)
var out bytes.Buffer
err = embedded.GenerateEmbeddedGo(&out, comp.Bytecode())
os.WriteFile("main.go", out.Bytes(), 0o644) // then: tinygo build -target=pico .
```

Constants of these kinds are supported: every integer, real and bit
string type, strings, booleans, `TIME`, `DATE`, `TIME_OF_DAY`,
`DATE_AND_TIME`, arrays of these, and compiled functions. Reals are
written so they read back exactly. Other constants fail generation. The
generated program runs the bytecode once.

## Generating a PLC program

`embedded.GenerateEmbeddedProgram(w, prog, opts)` writes Go that runs a
`PROGRAM` the way a PLC does. It takes the program compiled by
`compiler.CompileProgramUnit`, runs the initialization once, then runs the
body scan by scan, `Interval` apart, with the standard timers on the
device's clock:

```go
prog, err := compiler.NewCompilerWithBuiltins(object.Builtins).CompileProgramUnit(source, "Main")
var out bytes.Buffer
err = embedded.GenerateEmbeddedProgram(&out, prog, embedded.ProgramOptions{Interval: 10 * time.Millisecond})
os.WriteFile("main.go", out.Bytes(), 0o644) // then: tinygo build -target=pico -stack-size=16KB .
```

The file has `Run(scans, onScan)`, and a `main` that calls it. Give
`ProgramOptions.Package` another name to leave out `main` and call `Run`
from firmware of your own. `onScan` gets the VM after each scan. Its
`IO()` holds the located variables by address, so it is where to read the
pins into `%I` and drive them from `%Q`.

A program with nested function blocks, a TON, a function call, an array
and a located output runs on the emulated RP2040 (`embedded/picosim`).
The firmware takes 234 KB of flash and 39 KB of static RAM.
`-stack-size=16KB` leaves room above the 8 KB such a program needs.

Nothing on the device's path compiles a regular expression. The
`regexp` package and the Unicode tables it pulls in would take 72 KB of
the RP2040's RAM. The parser and the diagram drawing compile theirs on
first use, which firmware never reaches.

The generated program makes its VM with `vm.WithGlobalsSize(n)`, where `n`
is `vm.GlobalsNeeded` of the bytecode. The default of `vm.GlobalsSize`
slots takes 512 KB on a 32-bit target, more than the RP2040's 264 KB of RAM.
Code that makes its own VM for a microcontroller should do the same.

## Not yet

- No binary bytecode format: programs are compiled from source, or
  embedded as Go literals as above.
- No board I/O mapping: located variables live in the VM's I/O image and
  the application moves them to and from pins.
