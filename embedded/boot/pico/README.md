# Pico bootloader

A bootloader for the Raspberry Pi Pico (RP2040) that installs firmware
updates on trial, rolls back an update the app does not confirm, and
restores a corrupt app from a backup or from the golden image. Package
[boot](../) is the app's side of it.

## Flash

| Offset | Size | Slot |
|---|---|---|
| `0x000000` | 32 KB | boot2, then the bootloader |
| `0x008000` | 448 KB | run: the app, linked here |
| `0x078000` | 8 KB | status, two copies |
| `0x07A000` | 448 KB | staging: an update, written by the app |
| `0x0EA000` | 448 KB | backup: the app before the last update |
| `0x15A000` | 448 KB | golden: the first app, copied at first boot, never overwritten |
| `0x1CA000` | 216 KB | the app's data: RETAIN variables, VM program slots |

The RP2040 cannot remap flash, so the app always runs from the run slot: an
update is copied into it, not run where it lies.

## What it does at each boot

| Status | Does |
|---|---|
| none (first boot) | copies the app to the golden slot, records its size and CRC |
| `Idle` | checks the run slot's CRC and vector table, then starts it; if they fail, restores it from the backup, else from the golden slot |
| `Pending` | checks the update's CRC (rejects it if wrong), copies the run slot to the backup, then to `Install` |
| `Install` | copies the update into the run slot, checks it, then starts it as `Trial` |
| `Trial` | the app did not confirm before this reset: restores the backup (or golden) |
| `RestoreBackup`, `RestoreGolden` | copies that slot into the run slot |

Each state is recorded before its copy starts and is safe to start again,
and the status is written over the older of its two copies. A power cut at
any erase or program leaves something sound to run, as
[the tests](../pico_test.go) check at every write. The bootloader runs from
RAM and feeds the watchdog between sectors; it stops the watchdog before it
starts the app.

## Build

With clang and lld (LLVM 15 or later) and Go:

```bash
./build.sh                  # out/bootloader.uf2, out/bootloader.bin
```

Flash `bootloader.uf2` once, then the app.

## The app

Build it with TinyGo for the run slot: copy `pico-boot.json` and
`pico-boot.ld` into TinyGo's `targets/` and build with
`-target=pico-boot`. The target has no boot2 (the bootloader's comes first)
and sets build tag `beedance_boot`, under which package boot has `Pico`,
`Code` and `Reset`.

```go
import "github.com/apiarytech/beedance/embedded/boot"

// An update, in parts, while the program runs:
boot.Pico.BeginUpdate(size)
boot.Pico.WriteUpdate(part) // as parts arrive
boot.Pico.CommitUpdate()
// outputs safe, then:
boot.Reset()

// Once the new app has shown it works:
if boot.Pico.OnTrial() {
	boot.Pico.Confirm()
}
```

## Tests

`go test ./embedded/boot/` runs the bootloader's core, compiled for the
computer (`host.c`), through first boot, updates kept and rolled back,
restores, a spoilt update, an empty flash, and a power cut at every
write. With `RP2040JS_ESM` (rp2040js's `dist/esm`) and `RP2040JS_BOOTROM`
(a module exporting `bootromB1`, as picosim's `bootrom.mjs`) set, the same
tests run the bootloader built for the RP2040 on rp2040js
(`emulate.mjs`), which stands in for the flash chip behind the bootrom's
flash routines.

Not tested here: a TinyGo app built with `pico-boot.json`, and a board.

## Licenses

`rp2040-boot-stage2.S` and `pico-boot-stage2.S` are TinyGo's boot2, from
the Pico SDK: BSD 3-Clause, copyright Raspberry Pi (Trading) Ltd. and the
TinyGo authors (`LICENSE.tinygo`).
