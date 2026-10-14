# embedded/bench: the engines on microcontrollers

This module measures beedance's three engines on microcontrollers:

- the bytecode **VM** (`compiler` + `vm`), as embedded firmware runs it;
- **transpiled Go** (`transpiler`, on royaljelly), compiled by TinyGo;
- the tree-walking **evaluator**, parsing the source on the device.

It reports their speed (on a board or the Pico emulator), their size, and a
sampling profile of the firmware. It is a module of its own because the
transpiled engine needs royaljelly, which beedance does not.

## Workloads

| File | What it exercises |
|---|---|
| `st/plc.st` | A TON that restarts itself: a function block call, TIME arithmetic and an output (`%QX0.0`) each scan. The same program as `embedded/esp32hw/plc`. |
| `st/calc.st` | A 100-iteration FOR loop of DINT and REAL arithmetic: instruction dispatch and arithmetic only. `acc` ends at 303. |

`go generate ./...` (or `go run ./gen`) writes each engine's form of them:
`vmplc`, `vmcalc` (bytecode, scans back to back), `goplc`, `gocalc`
(transpiled with `CompactImage`) and `evalsrc` (the source). `go run ./gen
-check` fails when they are out of date.

## Firmware

| Package | Runs |
|---|---|
| `fw/all` | Both workloads on all three engines: one flash measures everything |
| `fw/vm`, `fw/go`, `fw/eval` | One engine, so its size can be measured alone |
| `fw/none` | No engine: the baseline the engines' sizes are measured against |
| `fw/profvmcalc`, `fw/profvmplc` | One workload on the VM for ever, for `gdbprof` |

Each prints, per engine and workload:

```
bench engine=vm workload=calc scans=500 avg_us=... min_us=... max_us=... wall_ms=... gcs=... mallocs=... alloc_bytes=... heap_inuse=... result=...
```

`gcs`, `mallocs` and `alloc_bytes` are what the run added. `result` is what
the engine computed, so engines can be compared.

**The evaluator needs a 16 KB goroutine stack** (`-stack-size=16KB`) for
`fw/eval` and `fw/all`. It overflows the 8 KB default, and an overflow
corrupts memory rather than failing cleanly.

## Running

On the host, the engines must agree (`acc` = 303 on all three, two pulses in
2.5 s), and Go benchmarks compare them:

```bash
go test ./...                      # from embedded/bench
go test -run XXX -bench . ./host
```

Host timings say little about a microcontroller. TinyGo compiles
differently (for size, `-opt=z`, and interface method calls as a search over
every implementing type), so measure on a board or the emulator.

Sizes, per engine (subtract `fw/none`):

```bash
tinygo build -target=pico2 -size short -o x.elf ./fw/vm
tinygo build -target=pico2 -stack-size=16KB -size short -o x.elf ./fw/eval
```

On the emulated Pico (`embedded/picosim`), whose timing counts cycles, so
runs are repeatable, though it does not model the flash cache:

```bash
docker build -f embedded/picosim/Dockerfile -t beedance-picosim .   # from the beedance root, once
./picosim.sh vm 20                                                  # fw/vm, 20 scans per workload
STACK=16KB ./picosim.sh all 20
```

A profile of the VM on the emulated Pico, by function, caller and source
line:

```bash
./profile.sh profvmcalc 60
```

On an ESP32, flash `fw/all` and read its UART:

```bash
tinygo flash -target=esp32-coreboard-v2 -stack-size=16KB -port COM6 ./fw/all
```

A board whose BOOT button must be held to flash starts in download mode if
it is still held when the flash resets it. Release it, then reset the board
and read its output with `esp32.ps1 -Port COM6` (Windows; it resets through
RTS). Elsewhere, add `-monitor` to the flash command.

## Results

Measured on 2026-10-13 with TinyGo 0.43.0. Before is beedance 8431e20; after
is the commit that added this module.

Speed on an ESP32 (`esp32-coreboard-v2`, `fw/all`, 500 scans, average per
scan):

| Engine | PLC before | PLC after | Calc before | Calc after |
|---|---|---|---|---|
| Transpiled Go | 24 µs | 22 µs | 8 µs | 8 µs |
| VM | 2,705 µs | 867 µs | 76,967 µs | 15,722 µs |
| Evaluator | 1,075 µs | 863 µs | 60,390 µs | 46,983 µs |

The evaluator also got faster without code changes. The firmware's static
RAM dropped from 134 KB to 30 KB, so the heap is larger and garbage
collection runs less often.

Speed on the emulated Pico (RP2040, 20 scans):

| Engine | PLC before | PLC after | Calc before | Calc after |
|---|---|---|---|---|
| Transpiled Go | 11 µs | 11 µs | 237 µs | 237 µs |
| VM | 632 µs | 408 µs | 16,363 µs | 6,111 µs |
| Evaluator | 599 µs | 598 µs | 50,090 µs | 50,119 µs |

Firmware size, flash / static RAM (data + bss; the heap is the rest):

| Firmware | ESP32 (4 MB / 520 KB) | Pico (2 MB / 264 KB) | Pico 2 (4 MB / 520 KB) |
|---|---|---|---|
| `fw/none` | 69.9 / 6.0 KB | 61.8 / 7.5 KB | 59.0 / 7.5 KB |
| `fw/vm` | 259.5 / 15.4 KB | 217.7 / 16.9 KB | 204.3 / 16.9 KB |
| `fw/go` | 77.9 / 6.3 KB | 69.8 / 7.8 KB | 66.7 / 7.8 KB |
| `fw/eval` | 578.7 / 24.8 KB | 477.1 / 26.4 KB | 435.0 / 26.4 KB |

## Limits

- The Pico 2 (RP2350) is built but not run: picosim emulates the RP2040 only.
- The emulator does not model the flash cache, through which both chips run
  their code. A large hot path costs more on a board than on the emulator.
- `gdbprof` reads `lr` as the caller, which holds only until the sampled
  function calls another: treat the caller table as a hint.
