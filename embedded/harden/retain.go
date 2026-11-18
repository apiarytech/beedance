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
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/apiarytech/beedance/object"
)

// Store keeps the two copies of the RETAIN variables, numbered 0 and 1,
// where they survive a reset: flash, FRAM, battery-backed RAM. A copy is
// written whole, and only one at a time, so a reset during a write spoils
// at most the copy being written.
type Store interface {
	// ReadCopy returns copy i as it was last written; nil and no error
	// when it has never been.
	ReadCopy(i int) ([]byte, error)
	// WriteCopy replaces copy i with data.
	WriteCopy(i int, data []byte) error
}

// MemStore is a Store in memory, for tests and for RAM that a reset keeps.
type MemStore struct {
	Copies [2][]byte
}

// ReadCopy returns copy i.
func (m *MemStore) ReadCopy(i int) ([]byte, error) { return m.Copies[i], nil }

// WriteCopy replaces copy i with a copy of data.
func (m *MemStore) WriteCopy(i int, data []byte) error {
	m.Copies[i] = append([]byte(nil), data...)
	return nil
}

// ErrNoRetain is Load's error when neither copy holds valid data of this
// program's RETAIN variables: the program starts cold.
var ErrNoRetain = errors.New("harden: no valid retained data")

// A copy is: magic, layout, sequence number, payload length, payload,
// then the CRC-32 of all that, little-endian.
var magic = [4]byte{'B', 'D', 'R', '1'}

const headerSize = 16

// Layout returns the fingerprint of a program's RETAIN variables: their
// names and types, in pairs, in slot order. Data saved under another
// layout is not loaded, so a program whose RETAIN variables changed starts
// cold instead of reading one variable's value into another.
func Layout(namesAndTypes ...string) uint32 {
	var b []byte
	for _, s := range namesAndTypes {
		b = appendString(b, s)
	}
	return crc32.ChecksumIEEE(b)
}

// Retain saves the RETAIN variables of a program, in the global slots
// Slots, to a Store and loads them back.
type Retain struct {
	store  Store
	slots  []int
	layout uint32
	seq    uint32
	last   int // the copy holding the newest data; -1 for none
	saved  []byte
}

// NewRetain returns a Retain of the variables in global slots slots, with
// fingerprint layout (see Layout).
func NewRetain(store Store, slots []int, layout uint32) *Retain {
	return &Retain{store: store, slots: slots, layout: layout, last: -1}
}

// payload encodes the variables' values. A variable holding a value the
// encoding does not wholly cover, such as a standard function block with
// state of its own, is skipped whole: half its state, loaded, would not
// agree with the other half.
func (r *Retain) payload(globals []object.Object) []byte {
	var b []byte
	for _, s := range r.slots {
		start := len(b)
		var ok bool
		if b, ok = appendValue(b, globals[s]); !ok {
			b = append(b[:start], tagSkip)
		}
	}
	return b
}

// parse returns the sequence number and payload of a copy, ok if it is
// valid data of this layout.
func (r *Retain) parse(data []byte) (seq uint32, payload []byte, ok bool) {
	if len(data) < headerSize+4 || !bytes.Equal(data[:4], magic[:]) {
		return 0, nil, false
	}
	le := binary.LittleEndian
	if le.Uint32(data[4:]) != r.layout {
		return 0, nil, false
	}
	n := int(le.Uint32(data[12:]))
	if n < 0 || n > len(data)-headerSize-4 {
		return 0, nil, false
	}
	end := headerSize + n
	if crc32.ChecksumIEEE(data[:end]) != le.Uint32(data[end:]) {
		return 0, nil, false
	}
	return le.Uint32(data[8:]), data[headerSize:end], true
}

// Load reads the newer valid copy into globals, which hold the values the
// program's initialization gave the variables. When neither copy is
// valid, or the newer valid one does not fit the variables, globals are
// left as they are and the error says why; a copy that is spoilt is
// passed over for the other.
func (r *Retain) Load(globals []object.Object) error {
	type cp struct {
		i       int
		seq     uint32
		payload []byte
	}
	var found []cp
	var readErr error
	for i := range 2 {
		data, err := r.store.ReadCopy(i)
		if err != nil {
			readErr = err
			continue
		}
		if seq, p, ok := r.parse(data); ok {
			found = append(found, cp{i, seq, p})
		}
	}
	// Newest first, by serial number arithmetic, so the sequence may wrap.
	if len(found) == 2 && int32(found[1].seq-found[0].seq) > 0 {
		found[0], found[1] = found[1], found[0]
	}
	for _, c := range found {
		d := &decoder{b: c.payload}
		vals := make([]object.Object, len(r.slots))
		ok := true
		for k, s := range r.slots {
			if vals[k], ok = d.value(globals[s]); !ok {
				break
			}
		}
		if !ok || d.err || len(d.b) != 0 {
			continue
		}
		for k, s := range r.slots {
			globals[s] = vals[k]
		}
		r.seq, r.last = c.seq, c.i
		r.saved = append([]byte(nil), c.payload...)
		return nil
	}
	if readErr != nil {
		return readErr
	}
	return ErrNoRetain
}

// Save writes the variables' values over the older copy, keeping the
// newer, when they differ from what was last loaded or saved. So that
// flash wears less, a caller saves every few scans, not every one.
func (r *Retain) Save(globals []object.Object) error {
	p := r.payload(globals)
	if r.saved != nil && bytes.Equal(p, r.saved) {
		return nil
	}
	seq := r.seq + 1
	data := make([]byte, 0, headerSize+len(p)+4)
	data = append(data, magic[:]...)
	data = appendU32(data, r.layout)
	data = appendU32(data, seq)
	data = appendU32(data, uint32(len(p)))
	data = append(data, p...)
	data = appendU32(data, crc32.ChecksumIEEE(data))
	i := 0
	if r.last == 0 {
		i = 1
	}
	if err := r.store.WriteCopy(i, data); err != nil {
		return err
	}
	r.seq, r.last, r.saved = seq, i, p
	return nil
}
