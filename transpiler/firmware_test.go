/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/lexer"
	"github.com/apiarytech/beedance/parser"
	"github.com/apiarytech/beedance/sil"
	"github.com/apiarytech/beedance/transpiler"
)

// firmwareProgram has RETAIN variables of several types, a counter that
// starts over at each start, and an input that, when set, makes the scan
// panic (an index out of range).
const firmwareProgram = `
PROGRAM Main
VAR RETAIN
	count : DINT;
	total : LREAL;
	name : STRING;
	flags : ARRAY[1..3] OF BOOL;
	when : TIME;
	tm : TON;
END_VAR
VAR
	fresh : DINT;
	idx : INT := 1;
	arr : ARRAY[1..2] OF INT;
	bad AT %IX0.0 : BOOL;
	ok AT %QX0.0 : BOOL;
END_VAR
count := count + 1;
fresh := fresh + 1;
total := total + 0.5;
name := 'run';
flags[2] := TRUE;
when := T#3s;
tm(IN := TRUE, PT := T#1s);
IF bad THEN
	arr[idx + 5] := 1;
END_IF;
ok := TRUE;
END_PROGRAM
`

func transpileFirmware(t *testing.T, src string) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	out, err := transpiler.GoFileWith(prog, transpiler.Options{Firmware: true, CompactImage: true})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// The generated file has a Firmware for the program, and the RETAIN
// variables it can keep in its encoding: not the TON.
func TestFirmwareGenerated(t *testing.T) {
	src := transpileFirmware(t, firmwareProgram)
	for _, want := range []string{
		"func NewMainFirmware() *Firmware[*Main] {",
		"func NewMain() *Main {",
		"b = retainU64(b, uint64(p.count))",
		"p.name = iec.STRING(d.str())",
		"type Firmware[P firmwareProgram] struct {",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in the generated file", want)
		}
	}
	if strings.Contains(src, "p.tm") && strings.Contains(src[strings.Index(src, "func (p *Main) appendRetain"):], "p.tm\n") {
		t.Error("the TON is encoded")
	}
	if strings.Contains(src, `"github.com/apiarytech/beedance`) {
		t.Error("the generated file imports beedance")
	}
	// Without the option, nothing of it.
	p := parser.New(lexer.New(firmwareProgram))
	plain, err := transpiler.GoFileWith(p.ParseProgram(), transpiler.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "Firmware") {
		t.Error("a file without Options.Firmware has a Firmware")
	}
}

// runFirmware builds src, with main, against royaljelly and runs it. Set
// ROYALJELLY_DIR to build against a local royaljelly.
func runFirmware(t *testing.T, src, main string) string {
	t.Helper()
	dir := t.TempDir()
	directive, require := sil.GoDirective, "require github.com/apiarytech/royaljelly "+sil.RoyaljellyVersion+"\n"
	if local := os.Getenv("ROYALJELLY_DIR"); local != "" {
		b, err := os.ReadFile(filepath.Join(local, "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		if m := regexp.MustCompile(`(?m)^go (\S+)`).FindSubmatch(b); m != nil {
			directive = string(m[1])
		}
		require += "replace github.com/apiarytech/royaljelly => " + filepath.ToSlash(local) + "\n"
	} else if err := sil.GoEngineAvailable(); err != nil {
		t.Skip(err)
	}
	files := map[string]string{
		"go.mod":      "module beedance.test/firmware\n\ngo " + directive + "\n\n" + require,
		"program.go":  src,
		"firmware.go": main,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	for _, args := range [][]string{{"mod", "tidy"}, {"run", "."}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		if args[0] == "run" {
			return string(out)
		}
	}
	return ""
}

// A panicking scan calls SafeState and restarts the program, which goes on
// from its RETAIN variables; a second start, as after a reset, does too.
// A flip in the code in flash stops it, and with Restarts 0 the first
// fault does.
func TestFirmwareRuns(t *testing.T) {
	got := runFirmware(t, transpileFirmware(t, firmwareProgram), `package main

import (
	"fmt"
	"time"

	"github.com/apiarytech/royaljelly/iec"
)

type memStore struct{ copies [2][]byte }

func (m *memStore) ReadCopy(i int) ([]byte, error) { return m.copies[i], nil }

func (m *memStore) WriteCopy(i int, d []byte) error {
	m.copies[i] = append([]byte(nil), d...)
	return nil
}

func setBad(v bool) { processImage.Write(func(img *processImageData) { img.I.B[0] = iec.BOOL(v) }) }

func main() {
	store := &memStore{}
	code := []byte("the firmware's code, as flash holds it")
	fw := NewMainFirmware()
	fw.Interval = time.Millisecond
	fw.Restarts = 3
	fw.RetainStore = store
	fw.Code = func() []byte { return code }
	fw.ScrubBytes = 8
	var safe []string
	fw.SafeState = func(err error) { safe = append(safe, err.Error()); setBad(false) }
	dog, n := 0, 0
	fw.Watchdog = func() { dog++ }
	err := fw.Run(10, func(p *Main) {
		if n++; n == 4 {
			setBad(true)
		}
	})
	p := fw.Program
	fmt.Printf("err=%v faults=%d safe=%d dog=%d count=%d fresh=%d\n", err, fw.Faults, len(safe), dog, p.count, p.fresh)
	fmt.Printf("safe: %q\n", safe)

	fw = NewMainFirmware()
	fw.Interval = time.Millisecond
	fw.RetainStore = store
	err = fw.Run(2, nil)
	p = fw.Program
	fmt.Printf("restart err=%v retainErr=%v count=%d fresh=%d total=%v name=%s flags=%v when=%v\n",
		err, fw.RetainErr, p.count, p.fresh, p.total, p.name, p.flags, p.when)

	// The newer copy spoilt in storage: the older, of count 11, loads.
	newest := 0
	if store.copies[1][8] > store.copies[0][8] {
		newest = 1
	}
	store.copies[newest][20] ^= 1
	fw = NewMainFirmware()
	fw.Interval = time.Millisecond
	fw.RetainStore = store
	fw.Run(1, nil)
	fmt.Printf("spoilt count=%d retainErr=%v\n", fw.Program.count, fw.RetainErr)

	fw = NewMainFirmware()
	fw.Interval = time.Millisecond
	fw.Restarts = -1
	fw.Code = func() []byte { return code }
	fw.ScrubBytes = 1 << 20
	n = 0
	err = fw.Run(0, func(p *Main) {
		if n++; n == 2 {
			code[3] ^= 0x10
		}
	})
	fmt.Printf("code err=%v scans=%d\n", err == ErrFirmwareCode, n)

	fw = NewMainFirmware()
	fw.Interval = time.Millisecond
	err = fw.Run(0, func(p *Main) { setBad(true) })
	_, isFault := err.(FirmwareFault)
	fmt.Printf("restarts 0: fault=%v faults=%d\n", isFault, fw.Faults)
}
`)
	for _, want := range []string{
		"err=<nil> faults=1 safe=1 dog=10 count=10 fresh=6\n",
		"restart err=<nil> retainErr=<nil> count=12 fresh=2 total=6 name=run flags=[false true false] when=3000000000\n",
		"spoilt count=12 retainErr=<nil>\n",
		"code err=true scans=2\n",
		"restarts 0: fault=true faults=0\n",
		"scan panicked",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}
