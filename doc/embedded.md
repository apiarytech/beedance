# Microcontrollers

beedance's core uses only the standard library, so parts of it build with
[TinyGo](https://tinygo.org) for microcontrollers.

## What runs under TinyGo

| Package | TinyGo (0.44) |
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
8 KB of stack, the Pico's default. The evaluator, which parses and runs the
source on the device, needs more: a TON program overflows 8 KB on an RP2040.
Build it with `-stack-size=16KB` or more.

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

`embedded/esp32sim` runs it on an emulated ESP32, ESP32-C3, ESP32-S3 or
ESP32-C6 in Docker, under Espressif's QEMU, with UART0 on TCP; CI runs all
four. The ESP32, C3 and S3 boot it from the ROM. The C6 boots it through
ESP-IDF's second-stage bootloader, with a TinyGo target that adds the app
description the bootloader requires and maps flash from the app partition;
the C6's QEMU is built from
Espressif's source, which has its machine before any release. The S2, H2,
C2 and P4 are not emulated.
See [embedded/esp32sim/README.md](../embedded/esp32sim/README.md).

```bash
docker build -f embedded/esp32sim/Dockerfile --build-arg CHIP=esp32c3 -t beedance-esp32sim .
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

The command line does the same for a file's first `PROGRAM`, with
`-interval` as `Interval` (100 ms when it is not given):

```bash
beedance -iec blink.st -embedded main.go -interval 20ms
tinygo build -target=pico -stack-size=16KB .
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

## Surviving memory faults

A microcontroller in space, or near a reactor or an accelerator, takes
bit flips from radiation, and neither the RP2040 nor the classic ESP32 has
ECC RAM. The generated program checks its memory with package
`embedded/harden` and restarts the program when it finds a fault, instead
of stopping.

| Check | When | Covers |
|---|---|---|
| `harden.RepairSingletons` | after every scan | `vm.True` and `vm.False`, which every `BOOL` points to |
| `harden.Scrubber` | `ScrubBytes` bytes after every scan | the instructions and constants, against the checksum written into the file |
| `harden.Checksum` | after every fault | the whole image, restored from the constants in flash when it does not match |
| `harden.Retain` | at each start, and saved every `RetainEvery` scans | `RETAIN` variables, in two copies with a CRC each |

The checks run before `onScan`, so outputs from a corrupt scan never
leave. A fault is a scan that fails or a check that fails. It calls
`SafeState(err)`, restores the image if it is corrupt, runs the
initialization again and loads the `RETAIN` variables. After `Restarts`
faults in a row, `Run` returns the last one.

The firmware sets these before `Run`:

| Variable | What |
|---|---|
| `Restarts` | faults in a row before `Run` gives up; less than 0 for no bound; 0 (default) returns the first fault, as before |
| `ScrubBytes` | bytes of the image checked per scan; 0 checks only after a fault |
| `SafeState func(error)` | drives the outputs to a safe state |
| `Watchdog func()` | called after each scan that passed its checks: feed the hardware watchdog in it |
| `RetainStore harden.Store` | two copies in flash, FRAM or battery-backed RAM; `harden.MemStore` keeps them in RAM |
| `RetainEvery` | scans between saves; a save writes only when the values changed |

`ProgramOptions.Restarts`, `ScrubBytes` and `RetainEvery` set their
starting values. `Faults` counts restarts; `RetainErr` is the last
retain error, `harden.ErrNoRetain` after a cold start.

```go
program.Restarts = -1
program.ScrubBytes = 256
program.SafeState = func(error) { relay.Low() }
program.Watchdog = func() { machine.Watchdog.Update() }
program.RetainStore = flashStore{}
err := program.Run(0, readAndDrivePins)
```

A copy of the `RETAIN` variables holds their names' and types' fingerprint.
A program whose `RETAIN` variables changed starts cold rather than load
another program's values. Elementary types, strings, dates and times,
arrays and structs are kept. A variable holding anything else, such as a
TON with its internal state, starts at its initial value.

Not covered: the VM's stack and the program's other variables in RAM
(only restart clears them), and the CPU itself. A scan run twice and
compared, or variables stored three times, would cover them at a cost in
scan time and RAM.

## Not yet

- No binary bytecode format: programs are compiled from source, or
  embedded as Go literals as above.
- No board I/O mapping: located variables live in the VM's I/O image and
  the application moves them to and from pins.
