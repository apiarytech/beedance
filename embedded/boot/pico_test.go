/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package boot

// The bootloader's tests: the same scenarios on its core compiled for this
// computer (pico/host.c, with a C compiler) and on the bootloader built for
// the RP2040 and run on rp2040js (with clang, lld and Node.js, when
// RP2040JS_ESM names rp2040js's dist/esm and RP2040JS_BOOTROM a module
// exporting bootromB1).

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// runner boots flash once, the power cut after failAfter erases and
// programs (-1: never); it returns what the bootloader did and the flash.
type runner func(t *testing.T, flash []byte, failAfter int) (string, []byte)

func run(t *testing.T, cmd *exec.Cmd, file string, flash []byte) (string, []byte) {
	t.Helper()
	if err := os.WriteFile(file, flash, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", cmd.Path, err, out)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out)), after
}

func hostRunner(t *testing.T) runner {
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "host")
	if out, err := exec.Command(cc, "-O2", "-Wall", "-Werror", "-o", bin, "pico/core.c", "pico/host.c").CombinedOutput(); err != nil {
		t.Skipf("no C compiler for the host build: %v\n%s", err, out)
	}
	return func(t *testing.T, flash []byte, failAfter int) (string, []byte) {
		file := filepath.Join(dir, "flash.bin")
		return run(t, exec.Command(bin, file, strconv.Itoa(failAfter)), file, flash)
	}
}

// thumb builds a program for the RP2040 with clang and lld.
func thumb(t *testing.T, out string, args ...string) []byte {
	t.Helper()
	flags := []string{"--target=thumbv6m-none-eabi", "-mcpu=cortex-m0plus", "-mthumb", "-Os", "-ffreestanding",
		"-fno-builtin", "-nostdlib", "-fuse-ld=lld", "-o", out + ".elf"}
	if b, err := exec.Command("clang", append(flags, args...)...).CombinedOutput(); err != nil {
		t.Skipf("no clang for thumbv6m with lld: %v\n%s", err, b)
	}
	if b, err := exec.Command("llvm-objcopy", "-O", "binary", out+".elf", out+".bin").CombinedOutput(); err != nil {
		t.Skipf("no llvm-objcopy: %v\n%s", err, b)
	}
	img, err := os.ReadFile(out + ".bin")
	if err != nil {
		t.Fatal(err)
	}
	return img
}

var bootloaderImage []byte

func emulatorRunner(t *testing.T) runner {
	esm, rom := os.Getenv("RP2040JS_ESM"), os.Getenv("RP2040JS_BOOTROM")
	if esm == "" || rom == "" {
		t.Skip("RP2040JS_ESM and RP2040JS_BOOTROM not set")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("no node")
	}
	dir := t.TempDir()
	if bootloaderImage == nil {
		bootloaderImage = thumb(t, filepath.Join(dir, "bootloader"), "-Wl,-T,pico/bootloader.ld", "-Wl,--gc-sections",
			"pico/pico-boot-stage2.S", "pico/core.c", "pico/rp2040.c")
	}
	emulate, _ := filepath.Abs("pico/emulate.mjs")
	return func(t *testing.T, flash []byte, failAfter int) (string, []byte) {
		file := filepath.Join(dir, "flash.bin")
		f := append([]byte(nil), flash...)
		copy(f[BootOff:], bootloaderImage)
		out, after := run(t, exec.Command("node", emulate, esm, rom, file, strconv.Itoa(failAfter)), file, f)
		// "jump 8000 app N": the app is checked by its image, as with host.c.
		if strings.HasPrefix(out, "jump") {
			out = strings.Join(strings.Fields(out)[:2], " ")
		}
		return out, after
	}
}

// apps are test apps of different sizes, built for the RP2040 when clang
// can, or else stand-ins with a valid vector table.
func apps(t *testing.T) [4][]byte {
	var a [4][]byte
	dir := t.TempDir()
	for id := 1; id <= 3; id++ {
		out := filepath.Join(dir, fmt.Sprint("app", id))
		flags := []string{"--target=thumbv6m-none-eabi", "-mcpu=cortex-m0plus", "-mthumb", "-Os", "-ffreestanding",
			"-fno-builtin", "-nostdlib", "-fuse-ld=lld", "-o", out + ".elf", fmt.Sprintf("-DAPP_ID=%d", id),
			"-Wl,-T,pico/testapp.ld", "pico/testapp.c"}
		if exec.Command("clang", flags...).Run() == nil && exec.Command("llvm-objcopy", "-O", "binary", out+".elf", out+".bin").Run() == nil {
			a[id], _ = os.ReadFile(out + ".bin")
			continue
		}
		img := make([]byte, id*5000)
		copy(img, []byte{0x00, 0x00, 0x04, 0x20, 0x09, 0x81, 0x00, 0x10}) // SP 0x20040000, reset 0x10008109
		for i := 8; i < len(img); i++ {
			img[i] = byte(i * id)
		}
		a[id] = img
	}
	return a
}

// flashWith is erased flash holding app at the run slot.
func flashWith(app []byte) []byte {
	m := NewMemFlash()
	copy(m.Bytes[RunOff:], app)
	return m.Bytes
}

// runs reports whether flash's run slot holds app, as its record says.
func runs(flash, app []byte) bool {
	r, err := (&Boot{Flash: &MemFlash{Bytes: flash}}).Status()
	if err != nil || r.RunSize == 0 || int(r.RunSize) > RegionSize {
		return false
	}
	slot := flash[RunOff : RunOff+int(r.RunSize)]
	return crc32.ChecksumIEEE(slot) == r.RunCRC && string(slot[:len(app)]) == string(app)
}

func status(t *testing.T, flash []byte) Record {
	r, err := (&Boot{Flash: &MemFlash{Bytes: flash}}).Status()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// send writes app to flash as an update and commits it.
func send(t *testing.T, flash, app []byte) {
	b := &Boot{Flash: &MemFlash{Bytes: flash}}
	if err := b.BeginUpdate(len(app)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(app); i += 300 {
		if err := b.WriteUpdate(app[i:min(i+300, len(app))]); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.CommitUpdate(); err != nil {
		t.Fatal(err)
	}
}

func confirm(t *testing.T, flash []byte) {
	if err := (&Boot{Flash: &MemFlash{Bytes: flash}}).Confirm(); err != nil {
		t.Fatal(err)
	}
}

// scenario is the life of a Pico in flight: first boot, updates kept and
// rolled back, a corrupt run slot restored from the backup and then from
// the golden image, a spoilt update refused, nothing to run.
func scenario(t *testing.T, boot runner) {
	app := apps(t)
	step := func(name string, flash []byte, want string) []byte {
		t.Helper()
		got, after := boot(t, flash, -1)
		if got != want {
			t.Fatalf("%s: %q, want %q", name, got, want)
		}
		return after
	}

	f := step("first boot", flashWith(app[1]), "jump 8000")
	if r := status(t, f); r.State != Idle || r.GoldenSize == 0 || r.GoldenCRC != r.RunCRC || !runs(f, app[1]) {
		t.Fatalf("first boot: %+v", r)
	}

	send(t, f, app[2])
	f = step("update", f, "jump 8000")
	if r := status(t, f); r.State != Trial || !runs(f, app[2]) || r.BackupSize == 0 {
		t.Fatalf("update: %+v", r)
	}
	f = step("reset on trial", f, "jump 8000")
	if r := status(t, f); r.State != Idle || r.Flags&RolledBack == 0 || !runs(f, app[1]) {
		t.Fatalf("rollback: %+v", r)
	}

	send(t, f, app[2])
	f = step("update again", f, "jump 8000")
	confirm(t, f)
	f = step("kept", f, "jump 8000")
	if r := status(t, f); r.State != Idle || r.Flags != 0 || !runs(f, app[2]) {
		t.Fatalf("kept: %+v", r)
	}
	send(t, f, app[3])
	f = step("third", f, "jump 8000")
	confirm(t, f)
	f = step("third kept", f, "jump 8000")

	f[RunOff+100] ^= 0x10 // a flip in the run slot: restored from the backup, app 2
	f = step("corrupt run slot", f, "jump 8000")
	if r := status(t, f); r.Flags&Restored == 0 || !runs(f, app[2]) {
		t.Fatalf("restore from backup: %+v", r)
	}
	f[RunOff+100] ^= 0x10
	f[BackupOff+100] ^= 0x10 // and in the backup: the golden image, app 1
	f = step("corrupt backup too", f, "jump 8000")
	if r := status(t, f); !runs(f, app[1]) {
		t.Fatalf("restore from golden: %+v", r)
	}

	send(t, f, app[3])
	f[StagingOff+50] ^= 1 // spoilt after the commit
	f = step("spoilt update", f, "jump 8000")
	if r := status(t, f); r.Flags&Rejected == 0 || r.State != Idle || !runs(f, app[1]) {
		t.Fatalf("spoilt update: %+v", r)
	}

	step("no app", NewMemFlash().Bytes, "halt")
}

// powerCuts cuts the power at each erase and program of a boot from flash,
// then boots again until an app starts: it must be one of want, whole, as
// its record says, whatever the cut.
func powerCuts(t *testing.T, boot runner, flash []byte, every int, want ...[]byte) {
	for n := 0; ; n += every {
		got, f := boot(t, append([]byte(nil), flash...), n)
		if got != "power lost" {
			if n == 0 {
				t.Fatalf("boot does no flash writes: %s", got)
			}
			return
		}
		for i := 0; i < 5 && got != "jump 8000"; i++ {
			got, f = boot(t, f, -1)
		}
		ok := false
		for _, app := range want {
			ok = ok || runs(f, app)
		}
		if got != "jump 8000" || !ok {
			t.Fatalf("power cut after %d writes: %s, status %+v", n, got, status(t, f))
		}
	}
}

func powerScenario(t *testing.T, boot runner, every int) {
	app := apps(t)
	t.Run("first boot", func(t *testing.T) { powerCuts(t, boot, flashWith(app[1]), every, app[1]) })

	_, f := boot(t, flashWith(app[1]), -1)
	send(t, f, app[2])
	t.Run("install", func(t *testing.T) { powerCuts(t, boot, f, every, app[1], app[2]) })

	_, onTrial := boot(t, append([]byte(nil), f...), -1)
	t.Run("rollback", func(t *testing.T) { powerCuts(t, boot, onTrial, every, app[1]) })

	confirm(t, onTrial)
	onTrial[RunOff+10] ^= 4
	t.Run("restore", func(t *testing.T) { powerCuts(t, boot, onTrial, every, app[1]) })
}

func TestBootloaderHost(t *testing.T)     { scenario(t, hostRunner(t)) }
func TestBootloaderEmulated(t *testing.T) { scenario(t, emulatorRunner(t)) }

func TestBootloaderPowerCutsHost(t *testing.T) { powerScenario(t, hostRunner(t), 1) }

func TestBootloaderPowerCutsEmulated(t *testing.T) {
	every := 1
	if testing.Short() {
		every = 5
	}
	powerScenario(t, emulatorRunner(t), every)
}
