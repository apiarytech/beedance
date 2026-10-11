# syntax=docker/dockerfile:1
#
# Emulated ESP32, ESP32-C3 or ESP32-S3. Builds a TinyGo package as firmware
# and runs it under Espressif's QEMU, with UART0 on tcp :4000. Upstream QEMU
# has no ESP32 machines; Espressif's fork does (https://github.com/espressif/qemu).
#
# Build from the beedance module root:
#   docker build -f embedded/esp32sim/Dockerfile -t beedance-esp32sim .
#   docker build -f embedded/esp32sim/Dockerfile --build-arg CHIP=esp32c3 -t beedance-esp32sim-c3 .
#   docker run --rm -p 4000:4000 beedance-esp32sim
#
# The CPU starts when a client connects to :4000, so no output is missed.

ARG TINYGO_VERSION=0.42.0

FROM tinygo/tinygo:${TINYGO_VERSION} AS firmware
# esp32, esp32c3 or esp32s3.
ARG CHIP=esp32
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
# The ESP32 runs TinyGo's esp32-qemu target, a .img: a whole 4 MB flash image
# with the firmware at 0x1000, where the ROM loads it.
#
# The C3 and S3 ROMs cannot load a TinyGo image (its first segment is mapped
# from flash), so the app is built for ESP-IDF's second-stage bootloader with
# the targets in embedded/esp32sim/tinygo, and the flash image is assembled
# from the bootloader and partition table in embedded/esp32sim/boot and the
# app at 0x10000. Output goes to UART0, which QEMU serves.
RUN set -e; mkdir -p /out; d=embedded/esp32sim; \
    for f in $d/tinygo/*-idf.*; do sed 's/\r$//' "$f" > "$(tinygo env TINYGOROOT)/targets/$(basename "$f")"; done; \
    case "$CHIP" in \
      esp32) tinygo build -target=esp32-qemu -size short -o /out/flash.img ${PKG} ;; \
      esp32c3|esp32s3) \
        tinygo build -target=$CHIP-idf -size short -o /out/app.bin ${PKG}; \
        head -c 4194304 /dev/zero | tr '\0' '\377' > /out/flash.img; \
        dd if=$d/boot/$CHIP/bootloader.bin      of=/out/flash.img bs=1024 seek=0  conv=notrunc; \
        dd if=$d/boot/$CHIP/partition-table.bin of=/out/flash.img bs=1024 seek=32 conv=notrunc; \
        dd if=/out/app.bin                      of=/out/flash.img bs=1024 seek=64 conv=notrunc ;; \
      *) echo "CHIP must be esp32, esp32c3 or esp32s3, not $CHIP" >&2; exit 1 ;; \
    esac

FROM debian:bookworm-slim
ARG CHIP=esp32
ARG QEMU_TAG=esp-develop-9.2.2-20260417
ARG QEMU_VERSION=esp_develop_9.2.2_20260417
# The C3 is RISC-V; the ESP32 and S3 are Xtensa.
RUN set -e; case "$CHIP" in esp32c3) arch=riscv32 ;; *) arch=xtensa ;; esac; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
      libglib2.0-0 libpixman-1-0 libslirp0 libsdl2-2.0-0 libgcrypt20; \
    curl -fsSL "https://github.com/espressif/qemu/releases/download/${QEMU_TAG}/qemu-${arch}-softmmu-${QEMU_VERSION}-x86_64-linux-gnu.tar.xz" \
      | tar -xJ -C /opt; \
    apt-get purge -y curl xz-utils; apt-get autoremove -y; rm -rf /var/lib/apt/lists/*; \
    ln -s /opt/qemu/bin/qemu-system-$arch /opt/qemu/bin/qemu-system
COPY --from=firmware /out/flash.img /firmware/flash.img
RUN useradd -r -m esp32sim
USER esp32sim
ENV CHIP=${CHIP}
EXPOSE 4000
# -icount 3, as ESP-IDF's idf.py qemu runs the C3; it gives the C3's timers
# a steady clock.
ENTRYPOINT ["/bin/sh", "-c", "exec /opt/qemu/bin/qemu-system -machine $CHIP $([ \"$CHIP\" = esp32c3 ] && echo -icount 3) -nographic -monitor none -serial tcp::4000,server=on,wait=on -drive file=/firmware/flash.img,if=mtd,format=raw,snapshot=on \"$@\"", "esp32sim"]
