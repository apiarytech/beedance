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
	"errors"
	"time"

	"github.com/apiarytech/beedance/object"
	"github.com/apiarytech/beedance/stdlib"
	"github.com/apiarytech/beedance/vm"
)

// PLC runs a program as a PLC does, scan by scan, and survives faults in
// memory: it checks the image and TRUE and FALSE after each scan, restarts
// the program after a fault, keeps the RETAIN variables, and takes a new
// program in two slots of flash, on trial, rolling back to the one before
// when it faults.
//
// The program a firmware is built with is its golden image: never
// overwritten, it is what runs when no slot holds a sound program.
//
// Its fields are set before Run. The methods that change the program
// (BeginUpdate, WriteUpdate, CommitUpdate, AbortUpdate) are called from
// onScan or OnHold, between scans, never from another goroutine.
type PLC struct {
	// Golden makes the golden image afresh, from flash: at start, and to
	// restore it when it is found corrupt. GoldenChecksum is its checksum.
	Golden         func() Image
	GoldenChecksum uint32
	// Interval is the time between the starts of scans.
	Interval time.Duration
	// VMOptions size the VMs' stacks.
	VMOptions []vm.Option

	// Restarts is how many faults in a row Run restarts the program after
	// before it returns the fault; less than 0 for no bound; 0 returns the
	// first. A fault of a program on trial rolls it back instead.
	Restarts int
	// ScrubBytes is how many bytes of the image each scan checks against
	// its checksum; 0 checks it only after a fault.
	ScrubBytes int
	// SafeState, if not nil, is called with each fault, before the program
	// restarts or Run returns, and with ErrUpdating when an update holds
	// the program: drive the outputs to a safe state in it.
	SafeState func(err error)
	// Watchdog, if not nil, is called after each scan that passed its
	// checks, and each Interval while an update holds the program: feed the
	// chip's watchdog in it. Its timeout must be longer than the longest
	// write to flash, a sector's erase: tens of milliseconds, more on some
	// chips.
	Watchdog func()
	// RetainStore, if not nil, keeps the RETAIN variables: loaded at each
	// start, saved every RetainEvery scans when they changed, but not
	// while a new program is on trial.
	RetainStore Store
	RetainEvery int

	// Images, if not nil, holds two programs sent to the PLC
	// (BeginUpdate); nil runs the golden image and refuses updates.
	Images ImageStore
	// TrialScans is how many scans a new program runs on trial. A fault in
	// them, or a reset before they end, rolls it back; after them it is
	// kept. Less than 1 is 1.
	TrialScans int
	// HoldDuringUpdate stops the scans while an update is written, the
	// outputs in their safe state (SafeState(ErrUpdating)), so that no
	// scan runs late for a write to flash. It holds only with OnHold set.
	HoldDuringUpdate bool
	// OnHold is called each Interval while an update holds the program,
	// in place of a scan and onScan: write the update's next part in it.
	OnHold func()

	// LastScan is how long the last scan took to run, onScan aside.
	LastScan time.Duration
	// Faults counts the restarts and rollbacks since Run began.
	Faults int
	// Rollbacks counts the new programs rolled back; RollbackErr is the
	// fault that rolled back the last.
	Rollbacks   int
	RollbackErr error
	// RetainErr is the last error loading or saving the RETAIN variables;
	// ErrNoRetain after a start with none saved.
	RetainErr error
	// SlotErr is the last error marking or clearing a slot in Images.
	SlotErr error

	image    Image
	checksum uint32
	active   int
	trial    int
	upd      *update
	next     *Image
	nextSlot int
	builtins map[int]*object.Builtin
}

// Errors of the PLC and its updates.
var (
	// ErrUpdating is what SafeState is called with when an update holds
	// the program.
	ErrUpdating = errors.New("harden: program held for an update")
	// ErrNoImages is BeginUpdate's error when the PLC has no Images.
	ErrNoImages = errors.New("harden: no program slots")
	// ErrUpdateBusy is BeginUpdate's error while an update is written,
	// waits to start, or is on trial.
	ErrUpdateBusy = errors.New("harden: an update is under way")
	// ErrNoUpdate is the error of WriteUpdate and CommitUpdate without
	// BeginUpdate.
	ErrNoUpdate = errors.New("harden: no update begun")
	// ErrUpdateSize is the error of an update that does not fit a slot, of
	// a write past its size, and of a commit before all of it is written.
	ErrUpdateSize = errors.New("harden: update size wrong")
)

// errSwitch ends a run to start a committed update.
var errSwitch = errors.New("harden: switch program")

// Image returns the image the PLC runs: its bytecode is what the VMs run.
func (p *PLC) Image() Image { return p.image }

// Active returns the slot of Images whose program runs; -1 for the golden
// image.
func (p *PLC) Active() int { return p.active }

// OnTrial reports whether the program that runs is on trial.
func (p *PLC) OnTrial() bool { return p.trial > 0 }

// Run runs the program: its initialization, then scans Interval apart,
// scans of them (0: for ever). onScan, if not nil, is called after each
// scan that passed its checks, with the VM, whose IO() holds the located
// variables by address. Run returns the fault that stopped the program.
func (p *PLC) Run(scans int, onScan func(machine *vm.VM)) error {
	object.FinalizeBuiltins()
	start := time.Now()
	p.builtins = stdlib.ClockBuiltins(func() time.Duration { return time.Since(start) })
	p.boot()
	n, inRow := 0, 0
	for {
		err := p.runFrom(&n, &inRow, scans, onScan)
		if err == nil {
			return nil
		}
		if err == errSwitch {
			p.start()
			continue
		}
		if p.SafeState != nil {
			p.SafeState(err)
		}
		RepairSingletons()
		if p.trial > 0 {
			p.Faults++
			p.rollback(err)
			continue
		}
		if p.Restarts >= 0 && inRow >= p.Restarts {
			return err
		}
		inRow++
		p.Faults++
		if err := p.reload(); err != nil {
			if p.SafeState != nil {
				p.SafeState(err)
			}
			return err
		}
	}
}

// runFrom starts the image, initialization and all, and runs its scans
// from the nth until scans, a fault or a switch to an update.
func (p *PLC) runFrom(n, inRow *int, scans int, onScan func(machine *vm.VM)) error {
	im := p.image
	globals := make([]object.Object, im.Globals)
	initVM := vm.NewWithGlobalsStore(im.Init, globals, p.VMOptions...)
	for idx, b := range p.builtins {
		initVM.SetBuiltin(idx, b)
	}
	if err := initVM.Run(); err != nil {
		return err
	}
	var retain *Retain
	if p.RetainStore != nil {
		retain = NewRetain(p.RetainStore, im.RetainSlots, im.RetainLayout)
		p.RetainErr = retain.Load(globals)
	}
	// The initialization's VM is dropped before the scans' is made, so a
	// microcontroller's heap holds one VM's stack and frames, not two.
	io := initVM.IO()
	initVM = nil
	scanVM := vm.NewWithGlobalsStore(im.Scan, globals, p.VMOptions...)
	for idx, b := range p.builtins {
		scanVM.SetBuiltin(idx, b)
	}
	for addr, v := range io {
		scanVM.IO()[addr] = v
	}
	var scrub *Scrubber
	if p.ScrubBytes > 0 {
		scrub = NewScrubber(p.checksum, im.Init, im.Scan)
	}
	next := time.Now()
	for scans == 0 || *n < scans {
		if p.next != nil {
			return errSwitch
		}
		if p.holding() {
			p.OnHold()
			if p.Watchdog != nil {
				p.Watchdog()
			}
			next = p.wait(next)
			continue
		}
		scanVM.Reset()
		began := time.Now()
		err := scanVM.Run()
		p.LastScan = time.Since(began)
		if err != nil {
			return err
		}
		// The checks come before onScan, which drives the outputs.
		if RepairSingletons() {
			return ErrSingletons
		}
		if scrub != nil {
			if done, ok := scrub.Step(p.ScrubBytes); done && !ok {
				return ErrImage
			}
		}
		if onScan != nil {
			onScan(scanVM)
		}
		*inRow = 0
		*n++
		if p.trial > 0 {
			if p.trial--; p.trial == 0 {
				p.confirm()
			}
		}
		if p.Watchdog != nil {
			p.Watchdog()
		}
		if retain != nil && p.trial == 0 && p.RetainEvery > 0 && *n%p.RetainEvery == 0 {
			if err := retain.Save(globals); err != nil {
				p.RetainErr = err
			}
		}
		next = p.wait(next)
	}
	return nil
}

// wait sleeps until the next scan is due, Interval after next.
func (p *PLC) wait(next time.Time) time.Time {
	next = next.Add(p.Interval)
	if d := time.Until(next); d > 0 {
		time.Sleep(d)
		return next
	}
	return time.Now() // overrun: start the next scan now
}

// holding reports whether an update holds the program.
func (p *PLC) holding() bool {
	return p.upd != nil && p.HoldDuringUpdate && p.OnHold != nil
}

// use makes im, from slot (-1: the golden image), the image to run.
func (p *PLC) use(slot int, im Image) {
	p.active, p.image = slot, im
	if slot < 0 {
		p.checksum = p.GoldenChecksum
	} else {
		p.checksum = im.Checksum()
	}
}

// reload checks the image after a fault and, when it does not match its
// checksum, makes it afresh from flash. It returns ErrImage when the
// golden image still does not match; a slot that no longer decodes is
// cleared, and the program before it runs.
func (p *PLC) reload() error {
	if p.image.Checksum() == p.checksum {
		return nil
	}
	if p.active < 0 {
		p.use(-1, p.Golden())
		if p.image.Checksum() != p.checksum {
			return ErrImage
		}
		return nil
	}
	data, err := p.Images.ReadImage(p.active)
	if err == nil {
		var im Image
		if im, err = DecodeImage(data); err == nil {
			p.use(p.active, im)
			return nil
		}
	}
	p.clear(p.active)
	p.boot()
	return nil
}
