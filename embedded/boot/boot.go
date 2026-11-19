/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Package boot is the app's side of the Pico bootloader (pico/): it writes
// a new firmware image to flash for the bootloader to install at the next
// reset, confirms an update on trial, and reads the status the bootloader
// keeps.
//
// The bootloader copies a new image into the run slot, keeps the one before
// as a backup, and starts the new one on trial: a reset before the app
// calls Confirm rolls it back. A run slot that fails its CRC at boot is
// restored from the backup, or from the golden image, the app's first,
// copied at first boot and never overwritten.
//
// Flash is TinyGo's machine.Flash on a Pico built with pico/pico-boot.json,
// whose linker script starts it at the status sectors (StatusOff).
package boot

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// The layout of the Pico's 2 MB of flash under the bootloader, as offsets
// from the start of flash (pico/layout.h).
const (
	XIPBase    = 0x10000000
	FlashSize  = 0x00200000
	SectorSize = 0x00001000
	PageSize   = 0x00000100
	BootOff    = 0x00000000
	BootSize   = 0x00008000
	RunOff     = 0x00008000
	RegionSize = 0x00070000
	StatusOff  = 0x00078000
	StatusSize = 0x00002000
	StagingOff = 0x0007A000
	BackupOff  = 0x000EA000
	GoldenOff  = 0x0015A000
	DataOff    = 0x001CA000
	DataSize   = FlashSize - DataOff
)

// State is where the bootloader is with the image in the run slot.
type State uint32

// The states of Record.State.
const (
	Idle          State = iota // the run slot holds a kept image
	Pending                    // the staging slot holds an update to install
	Install                    // the run slot backed up; the update being copied in
	Trial                      // the update runs, not yet confirmed
	RestoreBackup              // the backup being copied into the run slot
	RestoreGolden              // the golden image being copied into the run slot
)

// The bits of Record.Flags.
const (
	RolledBack = 1 << iota // the last update was rolled back
	Restored               // the run slot was found corrupt and restored
	Rejected               // the last update did not match its CRC
	NoFallback             // an image failed with nothing sound to go back to
)

// Record is the status the bootloader keeps, in two sectors, each a copy
// written whole; the newer valid one counts.
type Record struct {
	Seq, Flags            uint32
	State                 State
	RunSize, RunCRC       uint32 // the image in the run slot
	NewSize, NewCRC       uint32 // the update in the staging slot
	BackupSize, BackupCRC uint32
	GoldenSize, GoldenCRC uint32
}

const (
	magic      = 0x53424442 // "BDBS"
	recordSize = 13 * 4
)

func (r *Record) encode() []byte {
	le := binary.LittleEndian
	b := make([]byte, 0, recordSize)
	for _, v := range []uint32{magic, r.Seq, uint32(r.State), r.Flags, r.RunSize, r.RunCRC, r.NewSize,
		r.NewCRC, r.BackupSize, r.BackupCRC, r.GoldenSize, r.GoldenCRC} {
		b = le.AppendUint32(b, v)
	}
	return le.AppendUint32(b, crc32.ChecksumIEEE(b))
}

func decode(b []byte) (Record, bool) {
	le := binary.LittleEndian
	if len(b) < recordSize || le.Uint32(b) != magic || le.Uint32(b[recordSize-4:]) != crc32.ChecksumIEEE(b[:recordSize-4]) {
		return Record{}, false
	}
	w := func(i int) uint32 { return le.Uint32(b[4*i:]) }
	return Record{Seq: w(1), State: State(w(2)), Flags: w(3), RunSize: w(4), RunCRC: w(5), NewSize: w(6), NewCRC: w(7),
		BackupSize: w(8), BackupCRC: w(9), GoldenSize: w(10), GoldenCRC: w(11)}, true
}

// Flash is the flash from the status sectors (StatusOff) to its end, as
// TinyGo's machine.Flash gives it: offsets are from StatusOff, a write is
// into erased flash, an erase is of whole sectors (blocks).
type Flash interface {
	ReadAt(p []byte, off int64) (int, error)
	WriteAt(p []byte, off int64) (int, error)
	EraseBlocks(start, length int64) error
}

// Errors of Boot.
var (
	// ErrNoStatus is Status's error before the bootloader has run once.
	ErrNoStatus = errors.New("boot: no status: the bootloader has not run")
	// ErrBusy is BeginUpdate's error while an update waits to be installed
	// or is on trial (Confirm it first).
	ErrBusy = errors.New("boot: an update is pending or on trial")
	// ErrNoUpdate is the error of WriteUpdate and CommitUpdate without
	// BeginUpdate.
	ErrNoUpdate = errors.New("boot: no update begun")
	// ErrSize is the error of an update larger than a slot, of a write past
	// its size and of a commit before all of it is written.
	ErrSize = errors.New("boot: update size wrong")
	// ErrNotOnTrial is Confirm's error when no update is on trial.
	ErrNotOnTrial = errors.New("boot: no update on trial")
)

// Boot is the app's side of the bootloader.
type Boot struct {
	Flash Flash

	upd *update
}

type update struct {
	size, written int
	page          []byte // a page not yet written whole
}

// rel is an offset from the start of flash as an offset into Flash.
func rel(off int) int64 { return int64(off - StatusOff) }

// status returns the newer valid record and the sector it is in.
func (b *Boot) status() (Record, int, error) {
	var best Record
	newest := -1
	for i := range 2 {
		p := make([]byte, recordSize)
		if _, err := b.Flash.ReadAt(p, rel(StatusOff+i*SectorSize)); err != nil {
			return Record{}, -1, err
		}
		if r, ok := decode(p); ok && (newest < 0 || int32(r.Seq-best.Seq) > 0) {
			best, newest = r, i
		}
	}
	if newest < 0 {
		return Record{}, -1, ErrNoStatus
	}
	return best, newest, nil
}

// Status returns the status the bootloader keeps.
func (b *Boot) Status() (Record, error) {
	r, _, err := b.status()
	return r, err
}

// save writes r, a sequence number on, over the older status copy.
func (b *Boot) save(r Record, newest int) error {
	i := 0
	if newest == 0 {
		i = 1
	}
	r.Seq++
	page := make([]byte, PageSize)
	for k := range page {
		page[k] = 0xFF
	}
	copy(page, r.encode())
	off := rel(StatusOff + i*SectorSize)
	if err := b.Flash.EraseBlocks(off/SectorSize, 1); err != nil {
		return err
	}
	_, err := b.Flash.WriteAt(page, off)
	return err
}

// OnTrial reports whether the app runs as an update on trial: it should
// Confirm once it has shown it works, or the next reset rolls it back.
func (b *Boot) OnTrial() bool {
	r, err := b.Status()
	return err == nil && r.State == Trial
}

// Confirm keeps the update on trial, so that a reset no longer rolls it
// back.
func (b *Boot) Confirm() error {
	r, newest, err := b.status()
	if err != nil {
		return err
	}
	if r.State != Trial {
		return ErrNotOnTrial
	}
	r.State = Idle
	return b.save(r, newest)
}

// BeginUpdate starts writing a new firmware image of size bytes, built
// for the run slot (pico/pico-boot.json), to the staging slot.
func (b *Boot) BeginUpdate(size int) error {
	r, _, err := b.status()
	if err != nil {
		return err
	}
	if r.State != Idle {
		return ErrBusy
	}
	if size <= 0 || size > RegionSize {
		return ErrSize
	}
	b.upd = &update{size: size}
	return nil
}

// WriteUpdate writes the next part of the image. Flash is written a page
// at a time and erased a sector at a time as the writes reach it: parts of
// a sector or less keep each write's pause short. An error ends the update.
func (b *Boot) WriteUpdate(part []byte) error {
	u := b.upd
	if u == nil {
		return ErrNoUpdate
	}
	if u.written+len(u.page)+len(part) > u.size {
		b.upd = nil
		return ErrSize
	}
	u.page = append(u.page, part...)
	for len(u.page) >= PageSize {
		if err := b.writePage(u.page[:PageSize]); err != nil {
			b.upd = nil
			return err
		}
		u.page = u.page[PageSize:]
	}
	u.page = append([]byte(nil), u.page...)
	return nil
}

// writePage writes the next page of the update, erasing its sector first
// when it starts one.
func (b *Boot) writePage(p []byte) error {
	u := b.upd
	off := rel(StagingOff + u.written)
	if u.written%SectorSize == 0 {
		if err := b.Flash.EraseBlocks(off/SectorSize, 1); err != nil {
			return err
		}
	}
	if _, err := b.Flash.WriteAt(p, off); err != nil {
		return err
	}
	u.written += len(p)
	return nil
}

// CommitUpdate checks the image as flash holds it and marks it for the
// bootloader to install at the next reset. Reset the chip when the outputs
// are safe: the install takes a few seconds, the copy of the run slot to
// the backup and of the update into the run slot.
func (b *Boot) CommitUpdate() error {
	u := b.upd
	if u == nil {
		return ErrNoUpdate
	}
	if u.written+len(u.page) != u.size {
		return ErrSize
	}
	b.upd = nil
	if len(u.page) > 0 {
		page := make([]byte, PageSize)
		for k := range page {
			page[k] = 0xFF
		}
		copy(page, u.page)
		b.upd = u
		err := b.writePage(page)
		b.upd = nil
		if err != nil {
			return err
		}
	}
	crc, err := b.crc(StagingOff, u.size)
	if err != nil {
		return err
	}
	r, newest, err := b.status()
	if err != nil {
		return err
	}
	r.State, r.NewSize, r.NewCRC = Pending, uint32(u.size), crc
	return b.save(r, newest)
}

// AbortUpdate ends an update before its commit.
func (b *Boot) AbortUpdate() { b.upd = nil }

// crc returns the CRC-32 of size bytes of flash at off, read a sector at
// a time.
func (b *Boot) crc(off, size int) (uint32, error) {
	var crc uint32
	p := make([]byte, SectorSize)
	for done := 0; done < size; done += SectorSize {
		n := min(SectorSize, size-done)
		if _, err := b.Flash.ReadAt(p[:n], rel(off+done)); err != nil {
			return 0, err
		}
		crc = crc32.Update(crc, crc32.IEEETable, p[:n])
	}
	return crc, nil
}

// MemFlash is a Flash in memory, as NOR flash behaves: an erase sets bytes
// to 0xFF and a write only clears bits. Bytes is all of flash, from 0.
type MemFlash struct {
	Bytes []byte
}

// NewMemFlash returns an erased MemFlash of FlashSize bytes.
func NewMemFlash() *MemFlash {
	m := &MemFlash{Bytes: make([]byte, FlashSize)}
	for i := range m.Bytes {
		m.Bytes[i] = 0xFF
	}
	return m
}

func (m *MemFlash) span(off int64, n int) ([]byte, error) {
	a := int64(StatusOff) + off
	if off < 0 || a+int64(n) > int64(len(m.Bytes)) {
		return nil, errors.New("boot: flash access out of range")
	}
	return m.Bytes[a : a+int64(n)], nil
}

// ReadAt reads from off, an offset from StatusOff.
func (m *MemFlash) ReadAt(p []byte, off int64) (int, error) {
	s, err := m.span(off, len(p))
	if err != nil {
		return 0, err
	}
	return copy(p, s), nil
}

// WriteAt programs p at off: it clears the bits p has clear.
func (m *MemFlash) WriteAt(p []byte, off int64) (int, error) {
	s, err := m.span(off, len(p))
	if err != nil {
		return 0, err
	}
	for i, v := range p {
		s[i] &= v
	}
	return len(p), nil
}

// EraseBlocks erases length sectors from sector start.
func (m *MemFlash) EraseBlocks(start, length int64) error {
	s, err := m.span(start*SectorSize, int(length*SectorSize))
	if err != nil {
		return err
	}
	for i := range s {
		s[i] = 0xFF
	}
	return nil
}
