# esp32sim: an emulated ESP32 in Docker

esp32sim builds a TinyGo package as ESP32 firmware and runs it on an
emulated ESP32 (Xtensa), so tests can check firmware without a board. It
uses [Espressif's QEMU](https://github.com/espressif/qemu): upstream QEMU
has no ESP32 machine. UART0 is served over TCP.

| Port | What |
|---|---|
| 4000 | UART0, both directions. The emulated CPU starts when the first client connects, so no boot output is missed. |

It is the ESP32 counterpart of [picosim](../picosim/README.md).

## Run

From the beedance module root:

```bash
docker build -f embedded/esp32sim/Dockerfile -t beedance-esp32sim .
docker run -d --rm --name esp32sim -p 4000:4000 beedance-esp32sim
go test -tags esp32sim ./embedded/esp32sim/
```

`nc localhost 4000` gives an interactive serial console instead (the
firmware starts on connect). The example prints:

```text
ets Jul 29 2019 12:21:46

rst:0x1 (POWERON_RESET),boot:0x12 (SPI_FAST_FLASH_BOOT)
...
Starting embedded beedance VM...
VM execution finished.
Result: 30
```

## Other firmware

Build arguments pick what is built:

| Argument | Default | |
|---|---|---|
| `PKG` | `./embedded/example/raspberrypi` | Go package to build |
| `TINYGO_VERSION` | `0.42.0` | |
| `QEMU_TAG`, `QEMU_VERSION` | `esp-develop-9.2.2-20260417` | Espressif QEMU release |

```bash
docker build -f embedded/esp32sim/Dockerfile --build-arg PKG=./path/to/firmware -t my-esp32sim .
```

The firmware is TinyGo's `esp32-qemu` target, built as a `.img`: a whole
4 MB flash image with the program at `0x1000`, where the ESP32's ROM loads
it. A `.bin` would start at 0 and not boot.

## What is emulated

- **ESP32 (Xtensa)**: runs. The CPU, RAM, flash and UART0 are what this
  firmware needs.
- **ESP32-C3 and ESP32-S3**: Espressif's QEMU has these machines too, but
  TinyGo's C3 image does not boot on it as is: the ROM faults loading it
  (it expects ESP-IDF's second-stage bootloader). TinyGo has no QEMU target
  for them.
- **Wi-Fi, Bluetooth and most peripherals** are not emulated.

TinyGo builds the same firmware for real boards: `esp32-coreboard-v2`,
`esp32c3-generic`, `esp32s3-generic`, `xiao-esp32c6` and others
(`tinygo targets | grep esp`).
