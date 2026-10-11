# esp32sim: an emulated ESP32, ESP32-C3 or ESP32-S3 in Docker

esp32sim builds a TinyGo package as firmware and runs it on an emulated
ESP32 chip, so tests can check firmware without a board. It uses
[Espressif's QEMU](https://github.com/espressif/qemu): upstream QEMU has no
ESP32 machines. UART0 is served over TCP.

| Port | What |
|---|---|
| 4000 | UART0, both directions. The emulated CPU starts when the first client connects, so no output is missed. |

It is the ESP32 counterpart of [picosim](../picosim/README.md).

## Run

From the beedance module root, for one chip (`CHIP`: `esp32`, the default,
`esp32c3` or `esp32s3`):

```bash
docker build -f embedded/esp32sim/Dockerfile --build-arg CHIP=esp32c3 -t beedance-esp32sim .
docker run -d --rm --name esp32sim -p 4000:4000 beedance-esp32sim
go test -tags esp32sim ./embedded/esp32sim/
```

`nc localhost 4000` gives an interactive serial console instead (the
firmware starts on connect). The example prints, on the ESP32-C3:

```text
ESP-ROM:esp32c3-api1-20210207
...
I (0) boot: ESP-IDF v5.4.4-1438-g962f1ee3888 2nd stage bootloader
...
I (18) boot: Loaded app from partition at offset 0x10000
...
Starting embedded beedance VM...
VM execution finished.
Result: 30
```

## The chips

| `CHIP` | Core | How it boots | QEMU |
|---|---|---|---|
| `esp32` | Xtensa | TinyGo's `esp32-qemu` target, from the ROM | `qemu-system-xtensa -machine esp32` |
| `esp32c3` | RISC-V | ESP-IDF's bootloader, then the app built with `tinygo/esp32c3-idf.json` | `qemu-system-riscv32 -machine esp32c3 -icount 3` |
| `esp32s3` | Xtensa | ESP-IDF's bootloader, then the app built with `tinygo/esp32s3-idf.json` | `qemu-system-xtensa -machine esp32s3` |

- **ESP32**: TinyGo writes a `.img`, a whole 4 MB flash image with the
  program at `0x1000`, where the ESP32's ROM loads it.
- **ESP32-C3 and ESP32-S3**: the ROM cannot load a TinyGo image: its first
  segment is mapped from flash, which only a bootloader sets up (the ROM
  faults at `0x3c000020`). So the flash image is ESP-IDF's: its
  second-stage bootloader at `0x0`, the partition table at `0x8000`
  (both in [boot](boot/README.md), built by ESP-IDF v5.4) and the app at
  `0x10000`. Two changes to TinyGo's target make the app bootable there
  ([tinygo](tinygo/), copies of TinyGo 0.42.0's files):
  - the linker script starts the first segment with an `esp_app_desc_t`,
    the app description ESP-IDF's bootloader requires (without it the
    bootloader reads TinyGo's data as one and refuses the app:
    `Image requires efuse blk rev >= v596.79`);
  - the startup code maps flash from the app partition at `0x10000`,
    instead of from 0 as when the ROM starts the program.

  The same targets build firmware for real boards booted by ESP-IDF's
  bootloader (`esptool.py write_flash 0x0 bootloader.bin 0x8000
  partition-table.bin 0x10000 app.bin`).

## Known issues

- **ESP32-C3: sleeping.** After the example's result, its idle loop's first
  `time.Sleep` raises an exception (`*** Exception: code: 0`). A three-line
  TinyGo program that sleeps does the same, so it is TinyGo's timer on the
  emulated C3, not beedance; it has not been tried on a board. The test
  passes because the VM has finished by then.
- **Not emulated**: ESP32-C6, S2, H2, C2 and P4 (Espressif's QEMU has no
  machine for them), and on every chip Wi-Fi, Bluetooth and most
  peripherals. TinyGo still builds the firmware for the C6
  (`xiao-esp32c6`) and for many ESP32, C3 and S3 boards
  (`tinygo targets | grep esp`).

## Other firmware

Build arguments pick what is built:

| Argument | Default | |
|---|---|---|
| `CHIP` | `esp32` | `esp32`, `esp32c3` or `esp32s3` |
| `PKG` | `./embedded/example/raspberrypi` | Go package to build |
| `TINYGO_VERSION` | `0.42.0` | The `tinygo/` files are TinyGo 0.42.0's: check them against a new version |
| `QEMU_TAG`, `QEMU_VERSION` | `esp-develop-9.2.2-20260417` | Espressif QEMU release |

```bash
docker build -f embedded/esp32sim/Dockerfile --build-arg CHIP=esp32s3 --build-arg PKG=./path/to/firmware -t my-esp32sim .
```
