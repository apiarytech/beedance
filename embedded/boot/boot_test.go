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

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// The layout here and in pico/layout.h are the same.
func TestLayoutMatchesC(t *testing.T) {
	src, err := os.ReadFile("pico/layout.h")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"XIP_BASE": XIPBase, "FLASH_SIZE": FlashSize, "SECTOR_SIZE": SectorSize, "PAGE_SIZE": PageSize,
		"BOOT_OFF": BootOff, "BOOT_SIZE": BootSize, "RUN_OFF": RunOff, "REGION_SIZE": RegionSize,
		"STATUS_OFF": StatusOff, "STATUS_SIZE": StatusSize, "STAGING_OFF": StagingOff,
		"BACKUP_OFF": BackupOff, "GOLDEN_OFF": GoldenOff, "DATA_OFF": DataOff,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`#define (\w+)\s+0x([0-9A-Fa-f]+)u`).FindAllStringSubmatch(string(src), -1) {
		v, _ := strconv.ParseInt(m[2], 16, 64)
		got[m[1]] = int(v)
	}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s: layout.h 0x%x, layout here 0x%x", name, got[name], v)
		}
	}
	// The regions follow one another without overlap.
	if RunOff+RegionSize != StatusOff || StatusOff+StatusSize != StagingOff ||
		StagingOff+RegionSize != BackupOff || BackupOff+RegionSize != GoldenOff || GoldenOff+RegionSize != DataOff {
		t.Error("regions overlap or leave gaps")
	}
}

func TestRecordRoundTrip(t *testing.T) {
	r := Record{Seq: 7, State: Trial, Flags: RolledBack | Restored, RunSize: 1, RunCRC: 2, NewSize: 3, NewCRC: 4,
		BackupSize: 5, BackupCRC: 6, GoldenSize: 7, GoldenCRC: 8}
	b := r.encode()
	got, ok := decode(b)
	if !ok || got != r {
		t.Fatalf("got %+v, %v", got, ok)
	}
	for i := range b {
		bad := append([]byte(nil), b...)
		bad[i] ^= 1
		if _, ok := decode(bad); ok {
			t.Fatalf("flip at %d accepted", i)
		}
	}
}

// statusFlash is flash whose bootloader has run once, an Idle record
// written as boot() writes it.
func statusFlash(t *testing.T) (*MemFlash, *Boot) {
	m := NewMemFlash()
	b := &Boot{Flash: m}
	if err := b.save(Record{State: Idle, RunSize: 256, GoldenSize: 256}, -1); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func TestUpdate(t *testing.T) {
	m, b := statusFlash(t)
	img := make([]byte, 3*SectorSize+123)
	for i := range img {
		img[i] = byte(i * 7)
	}
	if err := b.BeginUpdate(len(img)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(img); i += 333 {
		if err := b.WriteUpdate(img[i:min(i+333, len(img))]); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.CommitUpdate(); err != nil {
		t.Fatal(err)
	}
	r, err := b.Status()
	if err != nil || r.State != Pending || r.NewSize != uint32(len(img)) || r.Seq != 2 {
		t.Fatalf("status %+v, %v", r, err)
	}
	if string(m.Bytes[StagingOff:StagingOff+len(img)]) != string(img) {
		t.Fatal("staging slot does not hold the image")
	}
	if err := b.BeginUpdate(10); !errors.Is(err, ErrBusy) {
		t.Fatalf("pending: %v", err)
	}
	if err := b.Confirm(); !errors.Is(err, ErrNotOnTrial) {
		t.Fatalf("confirm pending: %v", err)
	}
}

func TestUpdateErrors(t *testing.T) {
	if err := (&Boot{Flash: NewMemFlash()}).BeginUpdate(10); !errors.Is(err, ErrNoStatus) {
		t.Fatalf("no status: %v", err)
	}
	_, b := statusFlash(t)
	if err := b.BeginUpdate(RegionSize + 1); !errors.Is(err, ErrSize) {
		t.Fatalf("too big: %v", err)
	}
	if err := b.WriteUpdate([]byte{1}); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("no update: %v", err)
	}
	b.BeginUpdate(10)
	b.WriteUpdate(make([]byte, 5))
	if err := b.CommitUpdate(); !errors.Is(err, ErrSize) {
		t.Fatalf("short: %v", err)
	}
	if err := b.WriteUpdate(make([]byte, 6)); !errors.Is(err, ErrSize) {
		t.Fatalf("past its size: %v", err)
	}
	if err := b.CommitUpdate(); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("after an error: %v", err)
	}
}

func TestConfirm(t *testing.T) {
	_, b := statusFlash(t)
	r, newest, _ := b.status()
	r.State = Trial
	b.save(r, newest)
	if !b.OnTrial() {
		t.Fatal("not on trial")
	}
	if err := b.Confirm(); err != nil {
		t.Fatal(err)
	}
	if r, _ := b.Status(); r.State != Idle || b.OnTrial() {
		t.Fatalf("after confirm: %+v", r)
	}
}
