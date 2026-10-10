# picosim: an emulated Raspberry Pi Pico in Docker

picosim builds a TinyGo package as Pico firmware and runs it on an emulated
RP2040 ([rp2040js](https://github.com/wokwi/rp2040js)), so tests can check
firmware without a board. UART0 is served over TCP, and a GDB stub is
available for debugging.

| Port | What |
|---|---|
| 4000 | UART0 (GP0/GP1), both directions. Output since boot is replayed on connect. |
| 3333 | GDB remote stub |

## Run

From the beedance module root:

```bash
docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
docker run -d --rm --name picosim -p 4000:4000 -p 3333:3333 beedance-picosim
go test -tags picosim ./embedded/picosim/
```

`docker logs picosim` shows the UART output, and `nc localhost 4000` gives
an interactive serial console.

## Other firmware

Build arguments pick what is built:

| Argument | Default | |
|---|---|---|
| `PKG` | `./embedded/example/raspberrypi` | Go package to build |
| `TARGET` | `pico` | TinyGo target: `pico` or `pico-w` |
| `TINYGO_VERSION` | `0.42.0` | |
| `RP2040JS_VERSION` | `1.4.0` | |

```bash
docker build -f embedded/picosim/Dockerfile --build-arg PKG=./path/to/firmware -t my-picosim .
```

The Dockerfile works from any Go module root, so other apiarytech modules can
use it the same way.

A prebuilt `.uf2`, `.hex` or `.bin` (for example a CI artifact) runs without
rebuilding:

```bash
docker run --rm -p 4000:4000 -v "$PWD/beedance-pico.uf2:/fw.uf2:ro" beedance-picosim /fw.uf2
```

Firmware has to print with `-serial=uart`. TinyGo's default for the Pico is
USB CDC, which picosim does not serve.

## Environment

| Variable | Default | |
|---|---|---|
| `UART_PORT` | `4000` | `0` disables |
| `UART_BACKLOG` | `65536` | Bytes of UART output replayed to each new client |
| `GDB_PORT` | `3333` | `0` disables |
| `LOG_LEVEL` | `error` | `warn` shows firmware access to peripherals the emulator lacks |
| `WIRES` | | Jumper wires, `OUT:IN,...`: `2:10` drives GP10 from GP2 |
| `ADC` | | Analog readings, `CHANNEL:VALUE,...` (12-bit): `0:2048` |

Bytes sent to UART0 enter its receive FIFO only as fast as the firmware
empties it, as on a real line, so lines longer than the 32-byte FIFO arrive
whole. For a request/reply protocol such as the hardware-in-the-loop rig
(`embedded/rig/pico`), set `UART_BACKLOG=0`, so that a new client is not
handed the replies to an earlier one.

## Limits

- RP2040 only. The Pico 2 (RP2350, `pico2`, `pico2-w`) is not emulated.
- The Pico W's CYW43 radio is not emulated, so there is no Wi-Fi.
- Some peripherals are partial. With `LOG_LEVEL=warn`, TinyGo's runtime
  shows reads of the ROSC random bit, which returns 0. The RNG seed is
  therefore fixed, which keeps runs repeatable.
- Timing is emulated, not cycle exact. Do not use it for real-time measurements.
- RAM is the real 264 KB, so out-of-memory failures match the hardware.

## Building behind a TLS-inspecting proxy

The firmware stage builds offline with the image's own Go. If go.mod asks for
a newer patch release, the container's copy of go.mod is lowered to match the
image's Go, so no toolchain download happens. A package with third-party
imports still downloads modules. Behind a proxy that re-signs TLS (for
example Cisco Umbrella), that download fails with
`x509: certificate signed by unknown authority` unless the proxy's CA is
added to the image.
