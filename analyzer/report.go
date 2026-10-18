/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package analyzer

// The checks made once every body has been walked: what is never used,
// what is written from more than one place, what calls itself.

import (
	"fmt"
	"sort"
	"strings"
)

func (a *analysis) afterWalk() {
	a.unusedVariables()
	a.multipleWrites()
	a.recursion()
	a.instanceCalls()
}

func declPos(v *variable) (int, int) {
	if v.decl != nil && v.decl.Name != nil {
		return v.decl.Name.Token.Row, v.decl.Name.Token.Column
	}
	return 0, 0
}

func (a *analysis) unusedVariables() {
	for _, p := range a.pous {
		for _, v := range p.order {
			name := up(v.name)
			if name == "EN" || name == "ENO" {
				continue
			}
			row, col := declPos(v)
			switch v.section {
			case "VAR_INPUT":
				if v.reads == 0 {
					a.report("unused-input", Info, row, col, p.name, fmt.Sprintf("input %s is never read", v.name))
				}
			case "VAR_IN_OUT":
				if v.reads == 0 && v.writes == 0 {
					a.report("unused-input", Info, row, col, p.name, fmt.Sprintf("in-out %s is never used", v.name))
				}
			case "VAR_OUTPUT":
				if v.writes == 0 {
					a.report("unset-output", Info, row, col, p.name, fmt.Sprintf("output %s is never set", v.name))
				}
			default:
				switch {
				case v.reads == 0 && v.writes == 0:
					a.report("unused-variable", Warning, row, col, p.name, fmt.Sprintf("%s is declared but never used", v.name))
				case v.reads == 0 && v.located == "" &&
					(v.section == "VAR_TEMP" || p.kind == "FUNCTION" || p.kind == "METHOD" || p.kind == "PROPERTY"):
					a.report("unread-variable", Info, row, col, p.name, fmt.Sprintf("%s is written but never read", v.name))
				}
			}
			if v.section != "VAR_EXTERNAL" {
				if _, ok := a.globals[name]; ok {
					a.report("shadowed-global", Warning, row, col, p.name, fmt.Sprintf("%s hides the global variable of the same name", v.name))
				}
			}
			if v.section == "VAR_EXTERNAL" && p.kind == "FUNCTION_BLOCK" {
				a.report("external-in-fb", Info, row, col, p.name,
					fmt.Sprintf("function block %s uses the global %s: pass it as an input or in-out instead", p.name, v.name))
			}
		}
		if (p.kind == "FUNCTION" || p.kind == "METHOD") && p.result != "" && !p.set {
			a.report("unset-output", Info, p.row, p.col, p.name, fmt.Sprintf("%s never assigns its result", p.name))
		}
	}
	for _, g := range a.gorder {
		if g.reads == 0 && g.writes == 0 {
			row, col := declPos(g)
			a.report("unused-variable", Warning, row, col, "", fmt.Sprintf("global %s is declared but never used", g.name))
		}
	}
}

func (a *analysis) multipleWrites() {
	addrs := make([]string, 0, len(a.outputWrites))
	for addr := range a.outputWrites {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	for _, addr := range addrs {
		sites := a.outputWrites[addr]
		if len(sites) < 2 {
			continue
		}
		f := sites[0]
		for _, s := range sites[1:] {
			a.report("multiple-output-writes", Warning, s.row, s.col, s.pou,
				fmt.Sprintf("output %%%s is written in %d places (first at row %d, column %d in %s): write it once a scan",
					addr, len(sites), f.row, f.col, f.pou))
		}
	}
	for _, g := range a.gorder {
		progs := a.globalWriters[g]
		if len(progs) < 2 {
			continue
		}
		sites := make([]site, 0, len(progs))
		for _, s := range progs {
			sites = append(sites, s)
		}
		sort.Slice(sites, func(i, j int) bool {
			if sites[i].row != sites[j].row {
				return sites[i].row < sites[j].row
			}
			return sites[i].col < sites[j].col
		})
		names := make([]string, len(sites))
		for i, s := range sites {
			names[i] = s.pou
		}
		for _, s := range sites[1:] {
			a.report("global-writers", Warning, s.row, s.col, s.pou,
				fmt.Sprintf("global %s is written by %d PROGRAMs (%s): let one own it", g.name, len(sites), strings.Join(names, ", ")))
		}
	}
}

// recursion reports each POU that calls itself, directly or through
// others, at its call that starts the cycle.
func (a *analysis) recursion() {
	for _, p := range a.pous {
		keys := make([]string, 0, len(p.calls))
		for k := range p.calls {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			path, ok := a.pathTo(k, p.key)
			if !ok {
				continue
			}
			at := p.calls[k]
			msg := p.name + " calls itself"
			if len(path) > 1 {
				names := make([]string, 0, len(path)-1)
				for _, step := range path[:len(path)-1] {
					names = append(names, a.byKey[step].name)
				}
				msg += " through " + strings.Join(names, ", ")
			}
			a.report("recursion", Warning, at[0], at[1], p.name, msg+": a PLC's stack is small and fixed")
			break
		}
	}
}

// pathTo finds the calls from POU from to POU to: the keys after from,
// ending with to.
func (a *analysis) pathTo(from, to string) ([]string, bool) {
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		if k == to {
			var path []string
			for cur := k; cur != ""; cur = prev[cur] {
				path = append([]string{cur}, path...)
			}
			return path, true
		}
		p := a.byKey[k]
		if p == nil {
			continue
		}
		next := make([]string, 0, len(p.calls))
		for n := range p.calls {
			next = append(next, n)
		}
		sort.Strings(next)
		for _, n := range next {
			if _, seen := prev[n]; !seen {
				prev[n] = k
				queue = append(queue, n)
			}
		}
	}
	return nil, false
}

func (a *analysis) instanceCalls() {
	for _, p := range a.pous {
		for _, v := range p.order {
			sites := p.instanceCalls[v]
			if len(sites) < 2 {
				continue
			}
			for _, s := range sites[1:] {
				a.report("instance-calls", Warning, s[0], s[1], p.name,
					fmt.Sprintf("instance %s is called %d times in %s (first at row %d): its timers and edges see each call",
						v.name, len(sites), p.name, sites[0][0]))
			}
		}
		// Instances of the function block a method or accessor belongs to.
		for v, sites := range p.instanceCalls {
			if v.pou == p || len(sites) < 2 {
				continue
			}
			for _, s := range sites[1:] {
				a.report("instance-calls", Warning, s[0], s[1], p.name,
					fmt.Sprintf("instance %s is called %d times in %s (first at row %d): its timers and edges see each call",
						v.name, len(sites), p.name, sites[0][0]))
			}
		}
	}
}
