# esp32sim: an emulated ESP32, ESP32-C3, ESP32-S3 or ESP32-C6 in Docker

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
`esp32c3`, `esp32s3` or `esp32c6`):

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
Starting embedded beedance VM...
VM execution finished.
Result: 30
```

## The chips

| `CHIP` | Core | How it boots | QEMU |
|---|---|---|---|
| `esp32` | Xtensa | TinyGo's `esp32-qemu` target, from the ROM | `qemu-system-xtensa -machine esp32` |
| `esp32c3` | RISC-V | `tinygo/esp32c3-sim.json`, from the ROM | `qemu-system-riscv32 -machine esp32c3 -icount 3` |
| `esp32s3` | Xtensa | `tinygo/esp32s3-sim.json`, from the ROM | `qemu-system-xtensa -machine esp32s3` |
| `esp32c6` | RISC-V | ESP-IDF's bootloader, then the app built with `tinygo/esp32c6-idf.json` | `qemu-system-riscv32 -machine esp32c6 -icount 3`, built from source |

- **ESP32**: TinyGo writes a `.img`, a whole 4 MB flash image with the
  program at `0x1000`, where the ESP32's ROM loads it.
- **ESP32-C3 and ESP32-S3**: the ROM loads TinyGo's image from flash offset
  0, with code and read-only data mapped by TinyGo's startup code (since
  TinyGo 0.44). [tinygo](tinygo/) holds a target for each that sends output
  to UART0 rather than USB.
- **ESP32-C6**: the ROM cannot load a TinyGo image: it has segments mapped
  from flash, which only a bootloader sets up. So the flash image is
  ESP-IDF's: its second-stage bootloader at `0x0`, the partition table at
  `0x8000` (both in [boot](boot/README.md), built by ESP-IDF v5.4) and the
  app at `0x10000`. Two changes to TinyGo's target make the app bootable
  there ([tinygo](tinygo/), copies of TinyGo 0.44.0's files):
  - the linker script starts segment 0 (`.data`) with an `esp_app_desc_t`,
    the app description ESP-IDF's bootloader requires (without it the
    bootloader reads TinyGo's data as one and refuses the app:
    `Image requires efuse blk rev >= v596.79`);
  - the startup code maps code (`0x42000000`) and read-only data
    (`0x42800000`) separately from the app partition at `0x10000`. TinyGo
    maps one 16 MB window from 0, which puts the data 8 MB into flash and
    relies on the flash chip wrapping addresses, which QEMU does not.

  The C6 needs two more things. Its QEMU machine is in Espressif's source
  (`esp-develop`) but in no release yet, so the image builds QEMU from a
  pinned commit (`QEMU_COMMIT`, a few minutes). And its bootloader is built
  for 4 MB of flash: the C6's MMU page size follows the flash size, and
  the app is built for 64 KB pages.

  The same target builds firmware for real boards booted by ESP-IDF's
  bootloader (`esptool.py write_flash 0x0 bootloader.bin 0x8000
  partition-table.bin 0x10000 app.bin`).

## Known issues

- **ESP32-C3 and ESP32-C6: sleeping.** After the example's result, its idle
  loop's first `time.Sleep` raises an exception (`*** Exception: code: 0`).
  A three-line TinyGo program that sleeps does the same on the C3, so it is
  TinyGo's timer on these emulated RISC-V chips, not beedance; it has not
  been tried on a board. The test passes because the VM has finished by
  then.
- **Not emulated**: the ESP32-S2, H2, C2 and P4. Espressif's QEMU has no
  machine for them. TinyGo 0.44 builds firmware for the H2
  (`esp32h2-devkitm-1`), to be tried on a board, but has no target for the
  S2, C2 and P4 (only their register definitions). On every chip, Wi-Fi,
  Bluetooth and most peripherals are not emulated. TinyGo builds firmware
  for many more ESP32, C3, S3 and C6 boards (`tinygo targets | grep esp`).

## Other firmware

Build arguments pick what is built:

| Argument | Default | |
|---|---|---|
| `CHIP` | `esp32` | `esp32`, `esp32c3`, `esp32s3` or `esp32c6` |
| `PKG` | `./embedded/example/raspberrypi` | Go package to build |
| `TINYGO_VERSION` | `0.44.0` | The `tinygo/` files are TinyGo 0.44.0's: check them against a new version |
| `QEMU_TAG`, `QEMU_VERSION` | `esp-develop-9.2.2-20260417` | Espressif QEMU release (ESP32, C3, S3) |
| `QEMU_COMMIT` | `febae182e132e4055529be423a818225ebddaa3a` | Espressif QEMU source the C6's is built from |

```bash
docker build -f embedded/esp32sim/Dockerfile --build-arg CHIP=esp32s3 --build-arg PKG=./path/to/firmware -t my-esp32sim .
```
