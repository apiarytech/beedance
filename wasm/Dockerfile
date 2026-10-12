# syntax=docker/dockerfile:1
#
# beedance's toolchain as WebAssembly (package wasm). Builds
# cmd/beedance-wasm twice, for WASI (GOOS=wasip1) and for the web (GOOS=js),
# into two images:
#
#   sil (default)  Node.js running the modules: software-in-the-loop tests on
#                  the evaluator and the VM, or the JSON request protocol
#   web            the playground page, served on :8080
#
# Build from the beedance module root:
#   docker build -f wasm/Dockerfile -t beedance-wasm .
#   docker build -f wasm/Dockerfile --target web -t beedance-wasm-web .
#
#   docker run --rm -i beedance-wasm test < tests.st
#   docker run --rm -v "$PWD:/work" beedance-wasm test lib.st tests.st
#   docker run --rm -i beedance-wasm serve < requests.jsonl
#   docker run --rm -p 8080:8080 beedance-wasm-web

ARG GO_VERSION=1.27.2
ARG NODE_VERSION=24

FROM golang:${GO_VERSION} AS build
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN set -e; mkdir -p /out/web; \
    GOOS=wasip1 GOARCH=wasm go build -trimpath -ldflags=-s -o /out/beedance-wasi.wasm ./wasm/cmd/beedance-wasm; \
    GOOS=js GOARCH=wasm go build -trimpath -ldflags=-s -o /out/web/beedance.wasm ./wasm/cmd/beedance-wasm; \
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /out/web/; \
    cp wasm/web/index.html wasm/web/app.js /out/web/; \
    ls -l /out /out/web

# The playground: static files. app.js fetches the module as bytes, so the
# MIME type is only a courtesy. httpd sends the gzipped copy of a file to a
# browser that takes gzip: the module is a quarter of the size.
FROM busybox:1.36 AS web
COPY --from=build /out/web /www
RUN echo ".wasm:application/wasm" > /etc/httpd.conf \
 && gzip -9 -k /www/beedance.wasm /www/wasm_exec.js
EXPOSE 8080
CMD ["httpd", "-f", "-v", "-p", "8080", "-h", "/www", "-c", "/etc/httpd.conf"]

# Both modules under Node.js, 24 or later: Node 20 and 22 at times crash
# (SIGSEGV) running the WASI module after a run that grew its memory. The
# entry point runs the WASI one, with /work as its working directory for
# FILE arguments; js.mjs runs the web one for hosts that want to check the
# module a page loads:
#   docker run --rm -i --entrypoint node beedance-wasm /beedance/js.mjs /beedance/wasm_exec.js /beedance/beedance.wasm
FROM node:${NODE_VERSION}-alpine AS sil
WORKDIR /beedance
COPY --from=build /out/beedance-wasi.wasm /out/web/beedance.wasm /out/web/wasm_exec.js ./
COPY wasm/node/wasi.mjs wasm/node/js.mjs ./
USER node
WORKDIR /work
ENTRYPOINT ["node", "--no-warnings", "/beedance/wasi.mjs", "/beedance/beedance-wasi.wasm"]
CMD ["test"]
