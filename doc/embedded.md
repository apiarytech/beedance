# Microcontrollers

beedance's core uses only the standard library, so parts of it build with
[TinyGo](https://tinygo.org) for microcontrollers.

## What runs under TinyGo

| Package | TinyGo (0.42) |
|---|---|
| `token`, `lexer`, `ast`, `code`, `object`, `parser`, `stdlib` | Tests pass (checked in CI) |
| `compiler`, `vm` | Tests do not pass yet: a nil pointer dereference under TinyGo |
| `evaluator` | Tests do not pass yet: `FIRST`, `LAST` and `REST` of an empty array |

The embedded VM example below builds for the Pico, since it uses only the
VM's execution path on fixed bytecode.

## Raspberry Pi Pico

`embedded/example/raspberrypi` runs a small bytecode program on the VM.
CI builds it as firmware for `pico`, `pico-w`, `pico2` and `pico2-w` and
uploads the `.uf2` files as artifacts.

```bash
tinygo build -target=pico -size short -o beedance-pico.uf2 ./embedded/example/raspberrypi/
tinygo flash -target=pico ./embedded/example/raspberrypi/
```

To run the firmware without a board, `embedded/picosim` builds it into a
Docker image that emulates the RP2040 and serves UART0 over TCP. See
[embedded/picosim/README.md](../embedded/picosim/README.md).

```bash
docker build -f embedded/picosim/Dockerfile -t beedance-picosim .
docker run -d --rm -p 4000:4000 beedance-picosim
go test -tags picosim ./embedded/picosim/
```

On a Raspberry Pi running Linux, the regular Go toolchain builds the whole
of beedance (ARMv6, ARMv7 and arm64 are tested in CI under QEMU).

## Generating a standalone VM program

`embedded.GenerateEmbeddedGo(w, bytecode)` writes a Go `main` package that
holds the bytecode and its constants as Go literals and runs them on the VM:

```go
comp := compiler.New()
err := comp.Compile(program)
var out bytes.Buffer
err = embedded.GenerateEmbeddedGo(&out, comp.Bytecode())
os.WriteFile("main.go", out.Bytes(), 0o644) // then: tinygo build -target=pico .
```

Constants of these kinds are supported: integers (`LInt`), reals
(`LReal`), strings, booleans, `TIME`, `DATE` and compiled functions. Other
constants fail generation. The generated program runs the bytecode once;
a scan loop, I/O and a clock are for the application to add.

The generated program makes its VM with `vm.WithGlobalsSize(n)`, where `n`
is `vm.GlobalsNeeded` of the bytecode. The default of `vm.GlobalsSize`
slots takes 512 KB on a 32-bit target, more than the RP2040's 264 KB of RAM.
Code that makes its own VM for a microcontroller should do the same.

## Not yet

- No binary bytecode format: programs are compiled from source, or
  embedded as Go literals as above.
- No board I/O mapping: located variables live in the VM's I/O image and
  the application moves them to and from pins.
