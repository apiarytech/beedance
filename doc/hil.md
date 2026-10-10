# Hardware in the loop

The `PROGRAM TEST_...` unit tests that `beedance -test` runs on a simulated
clock (software in the loop, see [Command line](cli.md#unit-tests)) can also
run against a plant: a model of it written in Go, or the real thing. Each
test's located variables are then I/O points. Before each scan the `%I`
inputs are read from the plant, and after it the `%Q` outputs are written
to it.

```iecst
PROGRAM TEST_Fill
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR
    level AT %IW0 : INT;    (* from the plant *)
    full  AT %IX0.1 : BOOL;
    valve AT %QX0.0 : BOOL; (* to the plant *)
    reached : BOOL;
END_VAR
IF level < 20 THEN valve := TRUE; END_IF;
IF level >= 80 THEN valve := FALSE; reached := TRUE; END_IF;
IF full THEN failures := failures + 1; message := 'overflow'; END_IF;
IF reached AND NOT valve THEN done := TRUE; END_IF;
END_PROGRAM
```

The same test runs in three ways:

| | The plant is | The clock is | Engines compared |
|---|---|---|---|
| Software in the loop | absent: inputs keep their initial values | simulated | yes |
| With a plant model | a `sil.IO` in Go | simulated, or the wall clock | with `Deterministic` |
| Hardware in the loop | a rig over the rig protocol, or beehive's I/O | the wall clock | no |

The evaluator is a simulator for debugging a program. The VM and the
transpiled Go are what run on a target. On real I/O each engine runs the
test in turn and is judged on its own run. The engines are not compared,
because the plant does not give the same inputs twice.

## From the command line

```bash
beedance -test -io tcp://rig.local:5000 -interval 10ms -engines vm tests.st
beedance -test -io serial:/dev/ttyACM0 -engines eval,vm,go tests.st
beedance -test -io serial:COM3?baud=9600 tests.st
```

`-io` connects to a rig at that address: over TCP, or over a serial port
(Linux, macOS and Windows; 8N1, 115200 baud unless `?baud=` says
otherwise). It implies `-realtime`: scans run `-interval` apart on the
wall clock, and timers read it. `-realtime` on its own runs the tests on
the wall clock without I/O.

Every engine runs against the rig. On the go engine, the I/O points are the
located variables the test program declares, exchanged through the
transpiled program's process image. `-go-timeout` bounds each scan, not the
whole test. A rig that does not answer a request within 10 seconds
(`Rig.Timeout`) fails the run instead of hanging it.

## The Pico rig

`embedded/rig/pico` is a ready-made rig for a Raspberry Pi Pico (or a Pico W,
Pico 2 or Pico 2 W). It speaks the rig protocol on its USB serial port. CI
builds it with the other Pico firmware (`beedance-rig-<target>.uf2`).

| Points | Pins |
|---|---|
| `%IX0.0` .. `%IX0.7` (BOOL) | digital inputs GP10 .. GP17, pulled down |
| `%QX0.0` .. `%QX0.7` (BOOL) | digital outputs GP2 .. GP9 |
| `%IW0` .. `%IW2` (INT, UINT, WORD, ...) | analog inputs ADC0 .. ADC2 (GP26 .. GP28), 0 .. 4095 |

```bash
tinygo flash -target=pico ./embedded/rig/pico
beedance -test -io serial:/dev/ttyACM0 embedded/rig/pico/bench.st   # with GP2 wired to GP10
```

`begin` refuses a point the rig does not have, or one of the wrong type, so
a test cannot drive a pin nobody wired. The outputs are low (the safe
state) at power-up, at `begin` and at `end`. They also go low when the host
says nothing for 2 seconds within a run, for example because it crashed or
the cable came out. The pin table is at the top of `main.go`; change it to
suit a bench.

The rig also runs on the emulated Pico. picosim's `WIRES` connects pins as
jumper wires would, and `ADC` sets analog readings. CI runs the rig's tests
and `bench.st` this way:

```bash
docker build -f embedded/picosim/Dockerfile --build-arg PKG=./embedded/rig/pico -t beedance-picosim-rig .
docker run -d --rm -p 4100:4000 -e UART_BACKLOG=0 -e WIRES=2:10 -e ADC=0:2048 beedance-picosim-rig
beedance -test -io tcp://localhost:4100 -interval 20ms embedded/rig/pico/bench.st
```

## The rig protocol

A rig is a program next to the I/O, on a Raspberry Pi, a PC with an I/O card,
or a microcontroller. It speaks JSON, one object per line, over TCP or a
serial port, and replies to each request in turn:

```text
→ {"op":"begin","test":"TEST_Fill","engine":"vm","points":[{"address":"%IW0","type":"INT"},{"address":"%IX0.1","type":"BOOL"},{"address":"%QX0.0","type":"BOOL"}]}
← {}
→ {"op":"read","t_ms":10}
← {"inputs":{"%IW0":512,"%IX0.1":false}}
→ {"op":"write","t_ms":10,"outputs":{"%QX0.0":true}}
← {}
→ {"op":"end"}
← {}
```

- **`begin`** starts a test's run on one engine and lists its I/O points
  with their declared types. A rig puts the plant into a known state here.
- **`read`** asks for the inputs of the next scan, by address. Only `%I`
  addresses are accepted. An input that is left out keeps its value.
- **`write`** gives the outputs after a scan.
- **`end`** ends the run, even one that failed. A rig makes the plant safe
  here. If the connection drops within a run, `sil.ServeRig` calls `End`
  too.
- A reply with `"error": "..."` fails that engine's run of the test, with
  the message.

Values are JSON: `true` and `false` for BOOL; numbers for the integer, REAL
and bit-string types; strings for STRING and for TIME (`"T#1.5s"`). `t_ms`
is the run's time in milliseconds since its first scan.

A rig in Go is `sil.ServeRig` around a `sil.IO` that reads and drives the
pins:

```go
ln, _ := net.Listen("tcp", ":5000")
for {
    conn, err := ln.Accept()
    if err != nil {
        log.Fatal(err)
    }
    sil.ServeRig(ctx, conn, pins) // pins implements sil.IO
    conn.Close()
}
```

## From Go

```go
results, err := sil.Run(ctx, source, sil.Options{
    IO:            plant,  // a sil.IO: a model, sil.OpenRig(ctx, "serial:/dev/ttyACM0"), or your own
    RealTime:      false,  // a model can run on the simulated clock
    Deterministic: true,   // the model gives each engine the same inputs: compare them
})
```

`sil.IO` has four methods: `Begin`, `Read`, `Write` and `End`. Values are
beedance objects, and `sil.ToObject` and `sil.FromObject` convert them from
and to plain Go values. The values of a run's I/O points are recorded in
each scan under their addresses (`%IW0`), next to the test's variables, so
`sil.WriteCSV` and `-csv` show what the plant did.

## In beehive

beehive runs the same tests on an edge node's I/O. `beehive-ctl -service logic
-node edge1 test-plant tests.st` binds `%I` and `%Q` to the tags at those
addresses, as configured programs are bound. It runs only while logic is
offline, keeps logic offline until it ends, and gives the output tags
their values from before the run back. Its engineering service's `test
-engines eval,vm` runs software in the loop, comparing the engines.
