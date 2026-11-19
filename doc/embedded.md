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
ECC RAM. The generated file's `PLC`, a `harden.PLC`, checks its memory
and restarts the program when it finds a fault, instead of stopping.

| Check | When | Covers |
|---|---|---|
| `harden.RepairSingletons` | after every scan | `vm.True` and `vm.False`, which every `BOOL` points to |
| `harden.Scrubber` | `ScrubBytes` bytes after every scan | the instructions and constants, against the image's checksum |
| `Image.Checksum` | after every fault | the whole image, made afresh from flash when it does not match |
| `harden.Retain` | at each start, and saved every `RetainEvery` scans | `RETAIN` variables, in two copies with a CRC each |

The checks run before `onScan`, so outputs from a corrupt scan never
leave. A fault is a scan that fails or a check that fails. It calls
`SafeState(err)`, restores the image if it is corrupt, runs the
initialization again and loads the `RETAIN` variables. After `Restarts`
faults in a row, `Run` returns the last one.

The firmware sets `PLC`'s fields before `Run`:

| Field | What |
|---|---|
| `Restarts` | faults in a row before `Run` gives up; less than 0 for no bound; 0 (default) returns the first fault |
| `ScrubBytes` | bytes of the image checked per scan; 0 checks only after a fault |
| `SafeState func(error)` | drives the outputs to a safe state |
| `Watchdog func()` | called after each scan that passed its checks: feed the hardware watchdog in it |
| `RetainStore harden.Store` | two copies in flash, FRAM or battery-backed RAM; `harden.MemStore` keeps them in RAM |
| `RetainEvery` | scans between saves; a save writes only when the values changed |
| `Images harden.ImageStore` | two slots of flash for programs sent while it runs (below); nil refuses updates |
| `TrialScans` | scans a new program runs on trial (default 100) |
| `HoldDuringUpdate` | stop the scans, outputs safe, while an update is written (default true; holds only with `OnHold` set) |
| `OnHold func()` | called each `Interval` while held: write the update's next part in it |

`ProgramOptions` sets the starting values of `Restarts`, `ScrubBytes`,
`RetainEvery` and `TrialScans`, and `ScanDuringUpdate` turns
`HoldDuringUpdate` off. `PLC` reports `LastScan`, `Faults`, `Rollbacks`,
`RetainErr` and `SlotErr`.

```go
program.PLC.Restarts = -1
program.PLC.ScrubBytes = 256
program.PLC.SafeState = func(error) { relay.Low() }
program.PLC.Watchdog = func() { machine.Watchdog.Update() }
program.PLC.RetainStore = retainFlash{}
program.PLC.Images = imageFlash{}
program.PLC.OnHold = receiveUpdate
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

## Changing the program while it runs

A program goes to the PLC as a program image: `beedance -iec FILE -image
OUT` writes one (`harden.EncodeImage`). It holds the bytecode, the
`RETAIN` variables, the built-in functions it calls by name, and a CRC.

| Step | Call | What happens |
|---|---|---|
| begin | `PLC.BeginUpdate(size)` | picks the slot the running program is not in; with `HoldDuringUpdate`, calls `SafeState(harden.ErrUpdating)` and stops the scans |
| write | `PLC.WriteUpdate(part)`, part by part | writes to flash, erasing a sector as the writes reach it |
| commit | `PLC.CommitUpdate()` | reads the slot back, checks its CRC and that this firmware has its built-ins, writes a trailer |
| switch | at the end of the scan | the new program starts: initialization, `RETAIN` variables, a new scrub pass |
| trial | `TrialScans` scans | a fault rolls it back; after them it is kept |

The calls are made from `onScan` or `OnHold`, between scans. While an
update is written, the image the scans run and scrub does not change:
the update goes to the other slot. A spoilt or incomplete update is
refused at commit, and the program runs on.

What runs after a start, a fault or a rollback:

| Slots hold | Runs |
|---|---|
| nothing committed | the golden image, built into the firmware and never overwritten |
| a kept program | the newest kept program |
| a program started, not kept | it was on trial when the chip reset: it is cleared, and the one before runs |
| programs committed under another golden image | the golden image: firmware flashed with a new program runs it |

A rollback clears the slot, so the program before it runs: the other
slot's, or the golden image. `RETAIN` variables are not saved during a
trial, so a program rolled back to finds its own.

A write to flash stops the chip for the length of an erase, tens of
milliseconds, as code runs from the same flash. So:

- write in parts of a sector or less, each part one call;
- set the hardware watchdog's timeout above the longest erase;
- leave `HoldDuringUpdate` on where late scans could do harm; the
  scans stop and the outputs stay safe until the switch.

## Transpiled firmware

A program transpiled to Go runs as native code, faster than the VM, from
flash. `transpiler.Options.Firmware` (`-go-firmware`) gives each PROGRAM
`New<Program>Firmware()`, a `Firmware` that runs it scan by scan with the
same protections as `harden.PLC`, written into the file: it imports
nothing of beedance.

| `Firmware` field | What |
|---|---|
| `Interval` | time between the starts of scans (default 100 ms) |
| `Restarts` | faults in a row before `Run` gives up; less than 0 for no bound; 0 returns the first |
| `SafeState func(error)` | called with each fault: drive the outputs to a safe state |
| `Watchdog func()` | called after each scan that passed its checks |
| `RetainStore FirmwareStore` | the two copies of the `RETAIN` variables; the methods of `harden.Store`, so one driver serves both |
| `RetainEvery` | scans between saves (default 1); a save writes only when the values changed |
| `Code func() []byte` | the firmware's code in flash, as a slice over it |
| `CodeChecksum` | its CRC-32 from the build; 0 takes it at the start of `Run` |
| `ScrubBytes` | bytes of the code checked per scan |

```go
fw := NewMainFirmware()
fw.Interval = 10 * time.Millisecond
fw.Restarts = -1
fw.SafeState = func(error) { relay.Low() }
fw.Watchdog = func() { machine.Watchdog.Update() }
fw.RetainStore = retainFlash{}
fw.Code, fw.ScrubBytes = codeInFlash, 256
err := fw.Run(0, func(p *Main) { readAndDrivePins() })
```

A fault is a scan that panics (`FirmwareFault`: an index out of range, a
division by zero), a panic in `onScan`, or a check of the code that fails.
Run calls `SafeState` and makes the program afresh, its `RETAIN`
variables loaded. `onScan` runs only after a scan that passed, so a
panicking scan's outputs stay in the process image and never reach the
pins. Where TinyGo cannot recover a panic, the chip halts and the
watchdog resets it, which loads the `RETAIN` variables the same way.

The code runs from flash, so it cannot be restored from itself: a check
that fails ends `Run` with `ErrFirmwareCode`, for the firmware to reset the
chip or a bootloader to start another image. `CodeChecksum` 0 takes the
checksum when `Run` starts, which finds only flips made after.

Differences from the VM:

| | VM (`harden.PLC`) | Transpiled (`Firmware`) |
|---|---|---|
| `RETAIN` kept | elementary types, strings, dates, arrays, structs | elementary types, strings, dates, arrays of them |
| what is checked | the bytecode in RAM, restored from flash | the code in flash, not restorable |
| shared `TRUE`/`FALSE` | checked and repaired | none: a `BOOL` is a value |
| a new program while it runs | two slots, trial, rollback | a new firmware at a reset, through the bootloader (Pico): trial, rollback |
| copy format | magic `BDR1` | magic `BDRG`, as values are encoded otherwise |

## Bootloader (Pico)

Updating native firmware in flight takes a bootloader:
[`embedded/boot/pico`](../embedded/boot/pico/README.md), in C, built with
clang. It keeps four slots of 448 KB in the Pico's flash: run (the app),
staging (an update), backup (the app before) and golden (the first app,
never overwritten). An update is installed at a reset, on trial, and
rolled back at the next reset unless the app confirms it. A run slot whose
CRC fails at boot is restored from the backup, or else the golden slot.

Package `embedded/boot` is the app's side. Built with TinyGo's
`pico-boot` target (`pico-boot.json`), it gives `boot.Pico`, over
`machine.Flash`:

| Call | What |
|---|---|
| `BeginUpdate(size)`, `WriteUpdate(part)`, `CommitUpdate()` | write an image to the staging slot, sector by sector, and mark it to install |
| `boot.Reset()` | reset into the bootloader, to install it |
| `OnTrial()`, `Confirm()` | keep an update on trial |
| `Status()` | the bootloader's record: state, flags (`RolledBack`, `Restored`, `Rejected`), slot sizes and CRCs |
| `boot.Code()` | the run slot and its CRC, for `Firmware.Code` and `CodeChecksum` |

With transpiled firmware:

```go
code, crc := boot.Code()
fw.Code, fw.CodeChecksum, fw.ScrubBytes = func() []byte { return code }, crc, 256
scans := 0
err := fw.Run(0, func(p *Main) {
	if scans++; scans == 1000 && boot.Pico.OnTrial() {
		boot.Pico.Confirm() // a thousand good scans: keep the update
	}
})
// a fault Run cannot recover from, ErrFirmwareCode above all: the
// bootloader checks the run slot and restores it if it must.
boot.Reset()
```

The data area after the golden slot (`boot.DataOff`, 216 KB) is the app's:
a `harden.Store` for the `RETAIN` variables and a `harden.ImageStore` for
VM programs go there, at `machine.Flash` offsets from `boot.StatusOff`.

## Not yet

- No loader for a program image but `harden.PLC`: a standalone VM
  program embeds its bytecode as Go literals.
- No board I/O mapping: located variables live in the VM's I/O image and
  the application moves them to and from pins.
