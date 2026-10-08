# syntax=docker/dockerfile:1
#
# Emulated Raspberry Pi Pico (RP2040). Builds a TinyGo package as firmware and
# runs it under rp2040js, with UART0 on tcp :4000 and a GDB stub on tcp :3333.
#
# Build from the beedance module root:
#   docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
#   docker run --rm -p 4000:4000 -p 3333:3333 beedance-picosim

ARG TINYGO_VERSION=0.42.0
ARG NODE_VERSION=22

FROM tinygo/tinygo:${TINYGO_VERSION} AS firmware
# pico2/pico2-w (RP2350) are not emulated. pico-w builds, but its CYW43 radio
# is not emulated.
ARG TARGET=pico
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
# -serial=uart sends fmt output to UART0 (GP0/GP1) instead of USB CDC.
RUN mkdir -p /out && tinygo build -target=${TARGET} -serial=uart -size short -o /out/firmware.hex ${PKG}

FROM node:${NODE_VERSION}-alpine
ARG RP2040JS_VERSION=1.4.0
WORKDIR /picosim
COPY embedded/picosim/package.json ./
RUN npm install --omit=dev --no-audit --no-fund rp2040js@${RP2040JS_VERSION}
# RP2040 bootrom B1, as published with rp2040js (plain JS despite the .ts name).
ADD --chmod=644 https://raw.githubusercontent.com/wokwi/rp2040js/v${RP2040JS_VERSION}/demo/bootrom.ts bootrom.mjs
COPY embedded/picosim/run.mjs ./
COPY --from=firmware /out/firmware.hex /firmware/firmware.hex
USER node
EXPOSE 4000 3333
ENTRYPOINT ["node", "run.mjs"]
CMD ["/firmware/firmware.hex"]
