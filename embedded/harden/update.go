/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package harden

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// ImageStore is two slots of flash, numbered 0 and 1, that hold programs
// sent to a PLC. A slot holds an encoded image (EncodeImage), then marks
// the PLC writes after it: committed, started, kept.
type ImageStore interface {
	// ImageSize is the bytes a slot holds.
	ImageSize() int
	// ReadImage returns slot i, at least as far as it was written; nil
	// when nothing was.
	ReadImage(i int) ([]byte, error)
	// WriteImage writes p at off in slot i. The writes to a slot come at
	// increasing offsets, each into flash not written since the slot was
	// started over; a write at offset 0 starts it over. The store erases
	// flash as the writes reach it, a sector at a time, so that each write
	// costs at most one erase.
	WriteImage(i, off int, p []byte) error
}

// MemImageStore is an ImageStore in memory, for tests.
type MemImageStore struct {
	Size  int
	Slots [2][]byte
}

// ImageSize returns m.Size.
func (m *MemImageStore) ImageSize() int { return m.Size }

// ReadImage returns slot i.
func (m *MemImageStore) ReadImage(i int) ([]byte, error) { return m.Slots[i], nil }

// WriteImage writes p at off in slot i, the bytes skipped over erased
// (0xFF).
func (m *MemImageStore) WriteImage(i, off int, p []byte) error {
	if off == 0 {
		m.Slots[i] = nil
	}
	if off < len(m.Slots[i]) || off+len(p) > m.Size {
		return errors.New("harden: write out of order or past the slot")
	}
	for len(m.Slots[i]) < off {
		m.Slots[i] = append(m.Slots[i], 0xFF)
	}
	m.Slots[i] = append(m.Slots[i], p...)
	return nil
}

// Marks written after a slot's image: a trailer of the sequence number,
// the golden image's checksum and a CRC when the update is committed, then markStarted when it first
// starts, then markKept when its trial ends.
const (
	markStarted uint32 = 0x54524953 // "SIRT"
	markKept    uint32 = 0x5045454B // "KEEP"
	trailerSize        = 12
)

// slot is what a slot of Images holds.
type slot struct {
	i       int
	data    []byte
	n       int // length of the image
	seq     uint32
	started bool
	kept    bool
}

func trailerCRC(data []byte, n int, seq, golden uint32) uint32 {
	return crc32.ChecksumIEEE(appendU32(appendU32(append([]byte(nil), data[n-4:n]...), seq), golden))
}

// readSlot reads slot i; ok when it holds an image committed under this
// firmware's golden image. Firmware flashed anew, with another golden
// image, so runs its own program, not one sent to the firmware before.
func (p *PLC) readSlot(i int) (s slot, ok bool) {
	data, err := p.Images.ReadImage(i)
	if err != nil {
		return slot{}, false
	}
	n := imageLen(data)
	if n == 0 || len(data) < n+trailerSize {
		return slot{}, false
	}
	le := binary.LittleEndian
	s = slot{i: i, data: data[:n], n: n, seq: le.Uint32(data[n:])}
	golden := le.Uint32(data[n+4:])
	if golden != p.GoldenChecksum || le.Uint32(data[n+8:]) != trailerCRC(data, n, s.seq, golden) {
		return slot{}, false
	}
	mark := func(off int, m uint32) bool { return len(data) >= off+4 && le.Uint32(data[off:]) == m }
	s.started = mark(n+trailerSize, markStarted)
	s.kept = mark(n+trailerSize+4, markKept)
	return s, true
}

// slots returns the committed slots, newest first.
func (p *PLC) slots() []slot {
	var out []slot
	for i := range 2 {
		if s, ok := p.readSlot(i); ok {
			out = append(out, s)
		}
	}
	if len(out) == 2 && int32(out[1].seq-out[0].seq) > 0 {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

// mark writes m at off in slot i.
func (p *PLC) mark(i, off int, m uint32) bool {
	if err := p.Images.WriteImage(i, off, appendU32(nil, m)); err != nil {
		p.SlotErr = err
		return false
	}
	return true
}

// clear spoils slot i's image, so that it never runs again.
func (p *PLC) clear(i int) {
	if err := p.Images.WriteImage(i, 0, make([]byte, 4)); err != nil {
		p.SlotErr = err
	}
}

// boot chooses the image to run: the newest slot's, or the golden image
// when no slot holds a sound program. A slot started but not kept was on
// trial when the chip reset: it is cleared, and the one before runs.
func (p *PLC) boot() {
	p.trial = 0
	p.use(-1, p.Golden())
	if p.Images == nil {
		return
	}
	for _, s := range p.slots() {
		if s.started && !s.kept {
			p.clear(s.i)
			continue
		}
		im, err := DecodeImage(s.data)
		if err != nil {
			p.clear(s.i)
			continue
		}
		if !s.kept {
			if !p.mark(s.i, s.n+trailerSize, markStarted) {
				continue
			}
			p.trial = max(p.TrialScans, 1)
		}
		p.use(s.i, im)
		return
	}
}

// start runs the committed update, on trial.
func (p *PLC) start() {
	im, i := *p.next, p.nextSlot
	p.next = nil
	s, ok := p.readSlot(i)
	if !ok || !p.mark(i, s.n+trailerSize, markStarted) {
		p.clear(i)
		return
	}
	p.use(i, im)
	p.trial = max(p.TrialScans, 1)
}

// confirm keeps the program whose trial ended.
func (p *PLC) confirm() {
	if s, ok := p.readSlot(p.active); ok {
		p.mark(p.active, s.n+trailerSize+4, markKept)
	}
}

// rollback clears the program on trial, which faulted with err, and runs
// the one before it.
func (p *PLC) rollback(err error) {
	p.Rollbacks++
	p.RollbackErr = err
	p.clear(p.active)
	p.boot()
}

// update is an update being written.
type update struct {
	slot, size, written int
}

// spare returns the slot an update goes to: not the one that runs; when
// the golden image runs, an empty slot, or else the older.
func (p *PLC) spare() int {
	if p.active >= 0 {
		return 1 - p.active
	}
	s := p.slots()
	switch len(s) {
	case 0:
		return 0
	case 1:
		return 1 - s[0].i
	}
	return s[1].i
}

// BeginUpdate starts an update: size bytes of an encoded image
// (EncodeImage), to come by WriteUpdate. It goes to the slot the running
// program is not in. The program runs on until CommitUpdate, unless
// HoldDuringUpdate holds it.
func (p *PLC) BeginUpdate(size int) error {
	switch {
	case p.Images == nil:
		return ErrNoImages
	case p.upd != nil || p.next != nil || p.trial > 0:
		return ErrUpdateBusy
	case size < 12 || size+trailerSize+8 > p.Images.ImageSize():
		return ErrUpdateSize
	}
	p.upd = &update{slot: p.spare(), size: size}
	if p.holding() && p.SafeState != nil {
		p.SafeState(ErrUpdating)
	}
	return nil
}

// WriteUpdate writes the next part of the update to flash. Parts as small
// as a sector keep each write's pause short. An error ends the update.
func (p *PLC) WriteUpdate(part []byte) error {
	u := p.upd
	if u == nil {
		return ErrNoUpdate
	}
	if u.written+len(part) > u.size {
		p.upd = nil
		return ErrUpdateSize
	}
	if err := p.Images.WriteImage(u.slot, u.written, part); err != nil {
		p.upd = nil
		return err
	}
	u.written += len(part)
	return nil
}

// CommitUpdate checks the update as flash holds it, and that it is a
// program for this firmware, then marks it committed. The program
// switches to it at the end of the scan, on trial for TrialScans scans.
// An error but ErrUpdateSize, for a commit before the last part, ends the
// update, and the program runs on.
func (p *PLC) CommitUpdate() error {
	u := p.upd
	if u == nil {
		return ErrNoUpdate
	}
	if u.written != u.size {
		return ErrUpdateSize
	}
	p.upd = nil
	data, err := p.Images.ReadImage(u.slot)
	if err != nil {
		return err
	}
	if len(data) < u.size || imageLen(data) != u.size {
		return ErrImageFormat
	}
	im, err := DecodeImage(data[:u.size])
	if err != nil {
		return err
	}
	var seq uint32
	for _, s := range p.slots() {
		if s.i != u.slot && int32(s.seq-seq) > 0 {
			seq = s.seq
		}
	}
	seq++
	trailer := appendU32(appendU32(appendU32(nil, seq), p.GoldenChecksum), trailerCRC(data, u.size, seq, p.GoldenChecksum))
	if err := p.Images.WriteImage(u.slot, u.size, trailer); err != nil {
		return err
	}
	p.next, p.nextSlot = &im, u.slot
	return nil
}

// AbortUpdate ends an update. What was written is never run.
func (p *PLC) AbortUpdate() { p.upd = nil }
