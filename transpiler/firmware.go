/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

// This file generates, with Options.Firmware, what runs a program as
// firmware on a microcontroller and survives faults in memory, such as bit
// flips from radiation: New<Program>Firmware() returns a Firmware, which
// runs the program scan by scan, restarts it after a panic, keeps its RETAIN
// variables in two checksummed copies, feeds a watchdog and checks the
// firmware's code in flash. It is what embedded/harden's PLC does for the
// VM, written into the file: the file imports nothing of beedance, and the
// RETAIN variables are encoded by generated code, not reflection, for
// TinyGo.

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// firmwareNames are the names the firmware runtime declares in the file.
var firmwareNames = []string{
	"Firmware", "FirmwareStore", "FirmwareFault", "ErrFirmwareCode", "ErrFirmwareNoRetain",
	"firmwareProgram", "firmwareScrub", "retainReader", "retainU64", "retainStr", "retainBool",
	"retainParse", "retainMagic", "fwbinary", "fwcrc32", "fwerrors",
	// The methods a program gets.
	"retainLayout", "appendRetain", "loadRetain",
}

func init() {
	for _, n := range firmwareNames {
		reservedGoNames[n] = true
	}
	generatedPackages["fwbinary"] = "encoding/binary"
	generatedPackages["fwcrc32"] = "hash/crc32"
	generatedPackages["fwerrors"] = "errors"
}

// retainLayout is the fingerprint of a program's RETAIN variables, their
// names and types in pairs: the CRC-32 of each, its length first, as
// embedded/harden's Layout.
func retainLayout(namesAndTypes []string) uint32 {
	var b []byte
	for _, s := range namesAndTypes {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
		b = append(b, s...)
	}
	return crc32.ChecksumIEEE(b)
}

var goArrayType = regexp.MustCompile(`^\[(\d+)\](.+)$`)

// retainCode returns the statements that append expr, of Go type goType,
// to b, and that read it back from d; ok is false for a type it does not
// cover: a function block, a structure, a reference.
func retainCode(expr, goType string, depth int) (enc, dec string, ok bool) {
	if m := goArrayType.FindStringSubmatch(goType); m != nil {
		i := fmt.Sprintf("i%d", depth)
		e, d, ok := retainCode(expr+"["+i+"]", m[2], depth+1)
		if !ok {
			return "", "", false
		}
		return fmt.Sprintf("for %s := range %s {\n%s}\n", i, expr, e),
			fmt.Sprintf("for %s := range %s {\n%s}\n", i, expr, d), true
	}
	switch goType {
	case "iec.BOOL":
		return fmt.Sprintf("b = retainBool(b, bool(%s))\n", expr),
			fmt.Sprintf("%s = iec.BOOL(d.u8() != 0)\n", expr), true
	case "iec.SINT", "iec.INT", "iec.DINT", "iec.LINT", "iec.USINT", "iec.UINT", "iec.UDINT", "iec.ULINT",
		"iec.BYTE", "iec.WORD", "iec.DWORD", "iec.LWORD", "iec.CHAR", "iec.WCHAR", "iec.TIME":
		return fmt.Sprintf("b = retainU64(b, uint64(%s))\n", expr),
			fmt.Sprintf("%s = %s(d.u64())\n", expr, goType), true
	case "iec.REAL", "iec.LREAL":
		return fmt.Sprintf("b = retainU64(b, gomath.Float64bits(float64(%s)))\n", expr),
			fmt.Sprintf("%s = %s(gomath.Float64frombits(d.u64()))\n", expr, goType), true
	case "iec.STRING", "iec.WSTRING":
		return fmt.Sprintf("b = retainStr(b, string(%s))\n", expr),
			fmt.Sprintf("%s = %s(d.str())\n", expr, goType), true
	case "iec.DATE", "iec.TIME_OF_DAY", "iec.TOD", "iec.DT", "iec.DATE_AND_TIME":
		return fmt.Sprintf("b = retainU64(b, uint64(time.Time(%s).UnixNano()))\n", expr),
			fmt.Sprintf("%s = %s(time.Unix(0, int64(d.u64())).UTC())\n", expr, goType), true
	}
	return "", "", false
}

// transpileFirmware writes New<Program>Firmware() and the methods its
// Firmware needs of the program: its RETAIN variables' layout, encoding
// and decoding.
func (t *Transpiler) transpileFirmware(prog *ast.ProgramDeclaration, blocks [][]*ast.VarDeclStatement) {
	t.usesFirmware = true
	name := prog.Name.Value
	var layout []string
	var enc, dec strings.Builder
	for _, block := range blocks {
		for _, decl := range block {
			if !decl.IsRetain || decl.IsConstant || decl.Name == nil || decl.DataType == nil {
				continue
			}
			e, d, ok := retainCode("p."+decl.Name.Value, t.mapIecTypeToGo(decl.DataType), 0)
			if !ok {
				continue // kept at its initial value
			}
			layout = append(layout, decl.Name.Value, decl.DataType.String())
			enc.WriteString(e)
			dec.WriteString(d)
		}
	}
	t.write("\n// New%sFirmware returns %s as firmware: a Firmware that runs it scan by\n", name, name)
	t.write("// scan. Set its fields before Run.\n")
	t.write("func New%sFirmware() *Firmware[*%s] {\n", name, name)
	t.write("\treturn &Firmware[*%s]{New: New%s, Interval: 100 * time.Millisecond, RetainEvery: 1}\n}\n\n", name, name)
	t.write("func (p *%s) retainLayout() uint32 { return 0x%08x }\n\n", name, retainLayout(layout))
	t.write("func (p *%s) appendRetain(b []byte) []byte {\n%sreturn b\n}\n\n", name, enc.String())
	t.write("func (p *%s) loadRetain(d *retainReader) {\n%s}\n", name, dec.String())
}

// firmwareRuntime is written once into a file with Options.Firmware.
const firmwareRuntime = `
// FirmwareStore keeps the two copies of a program's RETAIN variables,
// numbered 0 and 1, where they survive a reset: flash, FRAM,
// battery-backed RAM. A copy is written whole, one at a time, so a reset
// during a write spoils at most the copy being written. It has the methods
// of beedance's harden.Store, so one driver serves both.
type FirmwareStore interface {
	// ReadCopy returns copy i as last written; nil and no error when it
	// has never been.
	ReadCopy(i int) ([]byte, error)
	// WriteCopy replaces copy i with data.
	WriteCopy(i int, data []byte) error
}

// Errors of Firmware.
var (
	// ErrFirmwareCode is the fault of a check of the code in flash that did
	// not match: Run returns it, as the code cannot be restored from
	// itself. Reset the chip, or let a bootloader start another image.
	ErrFirmwareCode = fwerrors.New("firmware: code in flash corrupt")
	// ErrFirmwareNoRetain is Firmware.RetainErr after a start with no
	// valid RETAIN variables saved: the program started cold.
	ErrFirmwareNoRetain = fwerrors.New("firmware: no valid retained data")
)

// FirmwareFault is the fault of a scan that panicked: an index out of
// range, a division by zero, memory gone wrong. Value is what it panicked
// with. On a target where TinyGo cannot recover a panic, the chip halts
// instead, and the watchdog resets it.
type FirmwareFault struct{ Value any }

// Error describes the panic.
func (f FirmwareFault) Error() string {
	switch v := f.Value.(type) {
	case error:
		return "firmware: scan panicked: " + v.Error()
	case string:
		return "firmware: scan panicked: " + v
	}
	return "firmware: scan panicked"
}

// firmwareProgram is what Firmware runs: a program of this file.
type firmwareProgram interface {
	Logic(now time.Time)
	retainLayout() uint32
	appendRetain(b []byte) []byte
	loadRetain(d *retainReader)
}

// Firmware runs a program as a PLC does, scan by scan, Interval apart, and
// survives faults: a scan that panics, or a check that fails, calls
// SafeState and restarts the program, its RETAIN variables loaded, up to
// Restarts times in a row. A scan's outputs reach the pins only through
// onScan, which runs after the scan and its checks passed.
type Firmware[P firmwareProgram] struct {
	// New makes the program with its initial values.
	New func() P
	// Interval is the time between the starts of scans.
	Interval time.Duration
	// Restarts is how many faults in a row Run restarts the program after
	// before it returns the fault; less than 0 for no bound; 0 returns the
	// first.
	Restarts int
	// SafeState, if not nil, is called with each fault, before the program
	// restarts or Run returns: drive the outputs to a safe state in it.
	SafeState func(err error)
	// Watchdog, if not nil, is called after each scan that passed its
	// checks: feed the chip's watchdog in it, so that a hang, or a panic
	// TinyGo cannot recover, resets the chip. Its timeout must be longer
	// than a write to flash of the RETAIN variables.
	Watchdog func()
	// RetainStore, if not nil, keeps the RETAIN variables: loaded at each
	// start, saved every RetainEvery scans when they changed. Elementary
	// types and arrays of them are kept; a function block or a structure
	// starts at its initial value.
	RetainStore FirmwareStore
	RetainEvery int
	// Code, if not nil, returns the firmware's code in flash as a slice
	// over it: on an RP2040, XIP flash from its start to the end of the
	// text section. ScrubBytes of it are checked per scan against
	// CodeChecksum, the CRC-32 (IEEE) a build step took of it; 0 takes it at
	// the start of Run, which finds only flips made after.
	Code         func() []byte
	CodeChecksum uint32
	ScrubBytes   int

	// Program is the instance that runs, made afresh at each start.
	Program P
	// LastScan is how long the last scan took, onScan aside.
	LastScan time.Duration
	// Faults counts the restarts since Run began.
	Faults int
	// RetainErr is the last error loading or saving the RETAIN variables.
	RetainErr error

	scrub  firmwareScrub
	seq    uint32
	newest int // 1 + the copy holding the newest data; 0 for none
	saved  []byte
	buf    []byte
}

// Run runs the program: scans of them (0: for ever). onScan, if not nil,
// is called after each scan that passed its checks, with the program: read
// the inputs and drive the outputs there, through the process image. Run
// returns the fault that stopped the program.
func (f *Firmware[P]) Run(scans int, onScan func(p P)) error {
	f.scrub = firmwareScrub{}
	if f.Code != nil {
		f.scrub.code, f.scrub.want = f.Code(), f.CodeChecksum
		if f.scrub.want == 0 {
			f.scrub.want = fwcrc32.ChecksumIEEE(f.scrub.code)
		}
	}
	n, inRow := 0, 0
	for {
		err := f.runFrom(&n, &inRow, scans, onScan)
		if err == nil {
			return nil
		}
		if f.SafeState != nil {
			f.SafeState(err)
		}
		if err == ErrFirmwareCode || (f.Restarts >= 0 && inRow >= f.Restarts) {
			return err
		}
		inRow++
		f.Faults++
	}
}

// runFrom starts the program and runs its scans from the nth until scans
// or a fault. A panic, in the program or in onScan, is a fault.
func (f *Firmware[P]) runFrom(n, inRow *int, scans int, onScan func(p P)) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = FirmwareFault{v}
		}
	}()
	p := f.start()
	next := time.Now()
	for scans == 0 || *n < scans {
		began := time.Now()
		p.Logic(began)
		f.LastScan = time.Since(began)
		if f.ScrubBytes > 0 && f.scrub.code != nil {
			if done, ok := f.scrub.step(f.ScrubBytes); done && !ok {
				return ErrFirmwareCode
			}
		}
		if onScan != nil {
			onScan(p)
		}
		*inRow = 0
		*n++
		if f.Watchdog != nil {
			f.Watchdog()
		}
		if f.RetainStore != nil && f.RetainEvery > 0 && *n%f.RetainEvery == 0 {
			if err := f.save(p); err != nil {
				f.RetainErr = err
			}
		}
		next = next.Add(f.Interval)
		if d := time.Until(next); d > 0 {
			time.Sleep(d)
		} else {
			next = time.Now() // overrun: start the next scan now
		}
	}
	return nil
}

// start makes the program afresh and loads its RETAIN variables. Data
// that does not fit leaves the program as New made it.
func (f *Firmware[P]) start() P {
	p := f.New()
	f.Program = p
	if f.RetainStore == nil {
		return p
	}
	payload, err := f.load(p.retainLayout())
	if err == nil {
		d := &retainReader{b: payload}
		p.loadRetain(d)
		if d.bad || len(d.b) != 0 {
			p = f.New()
			f.Program = p
			err = ErrFirmwareNoRetain
		}
	}
	f.RetainErr = err
	return p
}

// A copy of the RETAIN variables is: magic, layout, sequence number,
// payload length, payload, then the CRC-32 of all that, little-endian;
// embedded/harden's format with a magic of its own, as its payload is
// encoded otherwise.
const retainMagic = "BDRG"

// retainParse returns the sequence number and payload of a copy, ok if it
// is valid data of layout.
func retainParse(data []byte, layout uint32) (seq uint32, payload []byte, ok bool) {
	le := fwbinary.LittleEndian
	if len(data) < 20 || string(data[:4]) != retainMagic || le.Uint32(data[4:]) != layout {
		return 0, nil, false
	}
	n := int(le.Uint32(data[12:]))
	if n < 0 || n > len(data)-20 {
		return 0, nil, false
	}
	end := 16 + n
	if fwcrc32.ChecksumIEEE(data[:end]) != le.Uint32(data[end:]) {
		return 0, nil, false
	}
	return le.Uint32(data[8:]), data[16:end], true
}

// load returns the payload of the newer valid copy of layout.
func (f *Firmware[P]) load(layout uint32) ([]byte, error) {
	var best []byte
	var bestSeq uint32
	bestI := -1
	var readErr error
	for i := 0; i < 2; i++ {
		data, err := f.RetainStore.ReadCopy(i)
		if err != nil {
			readErr = err
			continue
		}
		// Newest by serial number arithmetic, so the sequence may wrap.
		if seq, payload, ok := retainParse(data, layout); ok && (bestI < 0 || int32(seq-bestSeq) > 0) {
			best, bestSeq, bestI = payload, seq, i
		}
	}
	if bestI < 0 {
		f.seq, f.newest, f.saved = 0, 0, nil
		if readErr != nil {
			return nil, readErr
		}
		return nil, ErrFirmwareNoRetain
	}
	f.seq, f.newest, f.saved = bestSeq, bestI+1, append(f.saved[:0], best...)
	return best, nil
}

// save writes the RETAIN variables over the older copy when they changed.
func (f *Firmware[P]) save(p P) error {
	payload := p.appendRetain(f.buf[:0])
	f.buf = payload
	if f.saved != nil && string(payload) == string(f.saved) {
		return nil
	}
	seq := f.seq + 1
	data := make([]byte, 0, 20+len(payload))
	data = append(data, retainMagic...)
	data = fwbinary.LittleEndian.AppendUint32(data, p.retainLayout())
	data = fwbinary.LittleEndian.AppendUint32(data, seq)
	data = fwbinary.LittleEndian.AppendUint32(data, uint32(len(payload)))
	data = append(data, payload...)
	data = fwbinary.LittleEndian.AppendUint32(data, fwcrc32.ChecksumIEEE(data))
	i := 0
	if f.newest == 1 {
		i = 1
	}
	if err := f.RetainStore.WriteCopy(i, data); err != nil {
		return err
	}
	f.seq, f.newest, f.saved = seq, i+1, append(f.saved[:0], payload...)
	if f.saved == nil {
		f.saved = []byte{}
	}
	return nil
}

// firmwareScrub checks the code in flash against its checksum a few bytes
// at a time.
type firmwareScrub struct {
	code []byte
	want uint32
	off  int
	crc  uint32
}

// step reads the next n bytes; done when a pass ended, ok when it matched.
func (s *firmwareScrub) step(n int) (done, ok bool) {
	k := min(n, len(s.code)-s.off)
	s.crc = fwcrc32.Update(s.crc, fwcrc32.IEEETable, s.code[s.off:s.off+k])
	if s.off += k; s.off < len(s.code) {
		return false, true
	}
	ok = s.crc == s.want
	s.off, s.crc = 0, 0
	return true, ok
}

// retainReader reads the values appendRetain wrote.
type retainReader struct {
	b   []byte
	bad bool
}

func (d *retainReader) next(n int) []byte {
	if d.bad || n < 0 || len(d.b) < n {
		d.bad = true
		return make([]byte, max(n, 8))
	}
	p := d.b[:n]
	d.b = d.b[n:]
	return p
}

func (d *retainReader) u8() byte    { return d.next(1)[0] }
func (d *retainReader) u64() uint64 { return fwbinary.LittleEndian.Uint64(d.next(8)) }
func (d *retainReader) str() string {
	return string(d.next(int(fwbinary.LittleEndian.Uint32(d.next(4)))))
}

func retainU64(b []byte, v uint64) []byte { return fwbinary.LittleEndian.AppendUint64(b, v) }

func retainStr(b []byte, s string) []byte {
	return append(fwbinary.LittleEndian.AppendUint32(b, uint32(len(s))), s...)
}

func retainBool(b []byte, v bool) []byte {
	if v {
		return append(b, 1)
	}
	return append(b, 0)
}
`
