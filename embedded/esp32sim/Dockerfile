# syntax=docker/dockerfile:1
#
# Emulated ESP32 (Xtensa). Builds a TinyGo package as ESP32 firmware and runs
# it under Espressif's QEMU, with UART0 on tcp :4000. Upstream QEMU has no
# ESP32 machine; Espressif's fork does (https://github.com/espressif/qemu).
#
# Build from the beedance module root:
#   docker build -f embedded/esp32sim/Dockerfile -t beedance-esp32sim .
#   docker run --rm -p 4000:4000 beedance-esp32sim
#
# The CPU starts when a client connects to :4000, so no output is missed.

ARG TINYGO_VERSION=0.42.0

FROM tinygo/tinygo:${TINYGO_VERSION} AS firmware
ARG PKG=./embedded/example/raspberrypi
USER root
# Build with the Go that ships in the image (TinyGo 0.42.0 has Go 1.27.0)
# rather than downloading the newer patch release go.mod may ask for. Only the
# container's copy of go.mod is lowered, and only when it is newer.
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY . .
RUN sed -i 's/\r$//' go.mod; \
    have=$(cd / && go env GOVERSION | sed 's/^go//'); \
    want=$(sed -n -E 's/^go ([0-9.]+)$/\1/p' go.mod); \
    if [ "$(printf '%s\n%s\n' "$have" "$want" | sort -V | tail -1)" != "$have" ]; then \
      sed -i -E -e "s/^go [0-9.]+$/go $have/" -e '/^toolchain /d' go.mod; \
    fi
# esp32-qemu is TinyGo's ESP32 target for this emulator. A .img output is a
# whole 4 MB flash image with the firmware at 0x1000, where the ESP32 ROM
# loads it; a .bin would start at 0 and not boot.
RUN mkdir -p /out && tinygo build -target=esp32-qemu -size short -o /out/firmware.img ${PKG}

FROM debian:bookworm-slim
ARG QEMU_TAG=esp-develop-9.2.2-20260417
ARG QEMU_VERSION=esp_develop_9.2.2_20260417
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
      libglib2.0-0 libpixman-1-0 libslirp0 libsdl2-2.0-0 libgcrypt20 \
 && curl -fsSL "https://github.com/espressif/qemu/releases/download/${QEMU_TAG}/qemu-xtensa-softmmu-${QEMU_VERSION}-x86_64-linux-gnu.tar.xz" \
      | tar -xJ -C /opt \
 && apt-get purge -y curl xz-utils && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*
COPY --from=firmware /out/firmware.img /firmware/firmware.img
RUN useradd -r -m esp32sim
USER esp32sim
EXPOSE 4000
ENTRYPOINT ["/opt/qemu/bin/qemu-system-xtensa", "-machine", "esp32", "-nographic", "-monitor", "none", \
            "-serial", "tcp::4000,server=on,wait=on", \
            "-drive", "file=/firmware/firmware.img,if=mtd,format=raw,snapshot=on"]
