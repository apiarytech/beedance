/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

// Command gdbprof is a sampling profiler for firmware on an emulated or real
// board with a GDB remote stub (picosim serves one on port 3333). It stops
// the CPU, reads pc and lr, lets it run on, and reports where the samples
// fell: by function, by function and caller, and by source line (from the
// ELF's DWARF).
//
//	go run ./gdbprof -elf firmware.elf -d 60s
//
// The caller is lr, which is only the caller while the sampled function has
// not called another: read it as a hint.
package main

import (
	"bufio"
	"debug/dwarf"
	"debug/elf"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "localhost:3333", "the GDB remote stub")
	file := flag.String("elf", "", "the firmware's ELF, built as the running firmware was")
	dur := flag.Duration("d", 20*time.Second, "how long to sample")
	gap := flag.Duration("gap", 2*time.Millisecond, "pause between samples")
	top := flag.Int("top", 30, "rows of each table")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "gdbprof: -elf is required")
		os.Exit(2)
	}
	f, err := elf.Open(*file)
	check(err)
	funcOf := funcTable(f)
	lineOf := lineTable(f)

	conn, err := net.Dial("tcp", *addr)
	check(err)
	g := &stub{conn: conn, r: bufio.NewReader(conn)}

	byFunc, byCaller, byLine := map[string]int{}, map[string]int{}, map[string]int{}
	n := 0
	g.interrupt()
	for end := time.Now().Add(*dur); time.Now().Before(end); {
		pc, lr := g.reg(15), g.reg(14)&^1
		byFunc[funcOf(pc)]++
		byCaller[funcOf(pc)+"  <-  "+funcOf(lr)]++
		byLine[lineOf(pc)]++
		n++
		g.send("c")
		time.Sleep(*gap)
		g.interrupt()
	}
	g.send("c")
	fmt.Printf("%d samples\n", n)
	for _, t := range []struct {
		title string
		m     map[string]int
	}{{"function (pc)", byFunc}, {"function <- caller (pc <- lr)", byCaller}, {"source line", byLine}} {
		fmt.Printf("\n== %s\n", t.title)
		printTop(t.m, n, *top)
	}
}

// stub speaks the GDB remote serial protocol, enough to stop the CPU, read a
// register and continue.
type stub struct {
	conn net.Conn
	r    *bufio.Reader
}

func (s *stub) send(packet string) {
	sum := 0
	for i := 0; i < len(packet); i++ {
		sum += int(packet[i])
	}
	fmt.Fprintf(s.conn, "$%s#%02x", packet, sum&0xff)
}

// recv returns the next packet's payload, acknowledging it.
func (s *stub) recv() string {
	for {
		b, err := s.r.ReadByte()
		check(err)
		if b != '$' {
			continue // an acknowledgement
		}
		payload, err := s.r.ReadString('#')
		check(err)
		s.r.Discard(2) // the checksum
		s.conn.Write([]byte("+"))
		return strings.TrimSuffix(payload, "#")
	}
}

// interrupt stops the CPU and waits for the stop reply.
func (s *stub) interrupt() {
	s.conn.Write([]byte{3})
	s.recv()
}

// reg reads a 32-bit little-endian register: 15 is pc, 14 lr on ARM.
func (s *stub) reg(n int) uint64 {
	s.send(fmt.Sprintf("p%x", n))
	b, err := hex.DecodeString(s.recv())
	if err != nil || len(b) < 4 {
		return 0
	}
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24
}

// funcTable returns a function giving the name of the function holding an
// address.
func funcTable(f *elf.File) func(uint64) string {
	type span struct {
		lo, hi uint64
		name   string
	}
	syms, err := f.Symbols()
	check(err)
	var spans []span
	for _, s := range syms {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC && s.Size > 0 {
			lo := s.Value &^ 1 // the Thumb bit
			spans = append(spans, span{lo, lo + s.Size, s.Name})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo < spans[j].lo })
	return func(a uint64) string {
		i := sort.Search(len(spans), func(i int) bool { return spans[i].hi > a })
		if i < len(spans) && spans[i].lo <= a {
			return spans[i].name
		}
		return fmt.Sprintf("?%#x", a)
	}
}

// lineTable returns a function giving the file:line of an address. A file
// in beedance is named from the module root.
func lineTable(f *elf.File) func(uint64) string {
	type at struct {
		a   uint64
		pos string
	}
	var lines []at
	if d, err := f.DWARF(); err == nil {
		rd := d.Reader()
		for {
			e, err := rd.Next()
			if err != nil || e == nil {
				break
			}
			if lr, err := d.LineReader(e); err == nil && lr != nil {
				var le dwarf.LineEntry
				for lr.Next(&le) == nil {
					name := strings.ReplaceAll(le.File.Name, "\\", "/")
					if i := strings.LastIndex(name, "beedance/"); i >= 0 {
						name = name[i+len("beedance/"):]
					} else if i := strings.LastIndex(name, "/"); i >= 0 {
						name = name[i+1:]
					}
					lines = append(lines, at{le.Address, fmt.Sprintf("%s:%d", name, le.Line)})
				}
			}
			rd.SkipChildren()
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].a < lines[j].a })
	return func(a uint64) string {
		i := sort.Search(len(lines), func(i int) bool { return lines[i].a > a })
		if i == 0 {
			return "?"
		}
		return lines[i-1].pos
	}
}

func printTop(m map[string]int, n, top int) {
	type row struct {
		k string
		v int
	}
	rows := make([]row, 0, len(m))
	for k, v := range m {
		rows = append(rows, row{k, v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].v > rows[j].v || rows[i].v == rows[j].v && rows[i].k < rows[j].k })
	for i, r := range rows {
		if i == top {
			break
		}
		fmt.Printf("%5.1f%%  %s\n", 100*float64(r.v)/float64(max(n, 1)), r.k)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "gdbprof:", err)
		os.Exit(1)
	}
}
