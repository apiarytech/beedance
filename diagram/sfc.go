/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package diagram

// This file holds the model of a Sequential Function Chart: its steps, the
// actions each step drives and the transitions between them. Package
// plcopen reads it from a graphical <SFC> body, ChartOf from the text form
// the parser reads, and Text writes it as that text form, which every
// engine runs. SFCSVG (sfc_svg.go) draws it.

import (
	"fmt"
	"strings"

	"github.com/apiarytech/beedance/ast"
)

// Chart is a Sequential Function Chart.
type Chart struct {
	Steps       []Step
	Transitions []Transition
	// Actions are the action bodies the chart defines, as Structured Text.
	// A step may also drive an action that has no body here: a BOOL
	// variable of the POU, TRUE while the action is active.
	Actions []Action
}

// Step is a step and the actions it drives.
type Step struct {
	Name    string
	Initial bool
	Actions []Association
}

// Association links a step to an action under a qualifier.
type Association struct {
	Action string
	// Qualifier is N, S, R, P, D, L, SD, DS or SL; "" is N.
	Qualifier string
	// Duration is the time of a timed qualifier, such as T#2s.
	Duration string
}

// Transition clears From (every step active) into To, when Condition, a
// BOOL expression, is TRUE. Several From steps are a simultaneous
// convergence, several To steps a simultaneous divergence.
type Transition struct {
	From, To  []string
	Condition string
	// Name is the transition's name, if it has one; drawings show it.
	Name string
}

// Action is a named action body.
type Action struct {
	Name string
	// Body is Structured Text statements, one or more lines.
	Body string
}

// Text writes the chart in the text form: its actions, then its steps,
// then its transitions, each line indented by indent. The chart must have
// one initial step, as beedance runs one.
func (c *Chart) Text(indent string) (string, error) {
	if err := c.check(); err != nil {
		return "", err
	}
	var b strings.Builder
	line := func(format string, args ...any) {
		b.WriteString(indent)
		fmt.Fprintf(&b, format, args...)
		b.WriteString("\n")
	}
	for _, a := range c.Actions {
		line("ACTION %s:", a.Name)
		for _, l := range strings.Split(strings.TrimRight(a.Body, "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				line("\t%s", l)
			}
		}
		line("END_ACTION")
		b.WriteString("\n")
	}
	for _, s := range c.Steps {
		kw := "STEP"
		if s.Initial {
			kw = "INITIAL_STEP"
		}
		if len(s.Actions) == 0 {
			line("%s %s: END_STEP", kw, s.Name)
			continue
		}
		line("%s %s:", kw, s.Name)
		for _, a := range s.Actions {
			q := a.Qualifier
			if q == "" {
				q = "N"
			}
			if a.Duration != "" {
				line("\t%s(%s, %s);", a.Action, q, a.Duration)
			} else {
				line("\t%s(%s);", a.Action, q)
			}
		}
		line("END_STEP")
	}
	if len(c.Transitions) > 0 {
		b.WriteString("\n")
	}
	for _, t := range c.Transitions {
		line("TRANSITION FROM %s TO %s := %s; END_TRANSITION", stepList(t.From), stepList(t.To), t.Condition)
	}
	return b.String(), nil
}

// check reports what the text form cannot hold or no engine can run.
func (c *Chart) check() error {
	steps := map[string]bool{}
	initial := ""
	for _, s := range c.Steps {
		if !isIdent(s.Name) {
			return fmt.Errorf("step name %q is not an identifier", s.Name)
		}
		key := strings.ToUpper(s.Name)
		if steps[key] {
			return fmt.Errorf("two steps are named %s", s.Name)
		}
		steps[key] = true
		if s.Initial {
			if initial != "" {
				return fmt.Errorf("steps %s and %s are both initial; beedance runs a chart from one initial step", initial, s.Name)
			}
			initial = s.Name
		}
		for _, a := range s.Actions {
			if !isIdent(a.Action) {
				return fmt.Errorf("step %s: action name %q is not an identifier", s.Name, a.Action)
			}
		}
	}
	if initial == "" {
		return fmt.Errorf("the chart has no initial step")
	}
	for _, t := range c.Transitions {
		if len(t.From) == 0 || len(t.To) == 0 {
			return fmt.Errorf("transition %s does not join steps", t.label())
		}
		for _, n := range append(append([]string{}, t.From...), t.To...) {
			if !steps[strings.ToUpper(n)] {
				return fmt.Errorf("transition %s: no step %s", t.label(), n)
			}
		}
		if strings.TrimSpace(t.Condition) == "" {
			return fmt.Errorf("transition %s has no condition", t.label())
		}
	}
	return nil
}

func (t Transition) label() string {
	if t.Name != "" {
		return t.Name
	}
	return "from " + strings.Join(t.From, ", ") + " to " + strings.Join(t.To, ", ")
}

func stepList(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return "(" + strings.Join(names, ", ") + ")"
}

func isIdent(s string) bool {
	if s == "" || !isLetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isLetter(s[i]) && !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// ChartOf reads a chart from an SFC body the parser read, to draw it.
// Action bodies are left out: a drawing names actions, it does not list
// their statements.
func ChartOf(sfc *ast.SFCProgram) *Chart {
	c := &Chart{}
	for _, el := range sfc.Elements {
		switch e := el.(type) {
		case *ast.ActionStatement:
			if e != nil && e.Name != nil {
				c.Actions = append(c.Actions, Action{Name: e.Name.Value})
			}
		case *ast.StepStatement:
			if e == nil || e.Name == nil {
				continue
			}
			s := Step{Name: e.Name.Value, Initial: e.IsInitial}
			for _, a := range e.Actions {
				if a == nil || a.ActionName == nil {
					continue
				}
				as := Association{Action: a.ActionName.Value}
				if a.Qualifier != nil {
					as.Qualifier = a.Qualifier.Value
				}
				if a.Duration != nil {
					as.Duration = a.Duration.String()
				}
				s.Actions = append(s.Actions, as)
			}
			c.Steps = append(c.Steps, s)
		case *ast.TransitionStatement:
			if e == nil {
				continue
			}
			t := Transition{}
			for _, id := range e.From {
				t.From = append(t.From, id.Value)
			}
			for _, id := range e.To {
				t.To = append(t.To, id.Value)
			}
			if e.Condition != nil {
				t.Condition = unwrap(e.Condition.String())
			}
			c.Transitions = append(c.Transitions, t)
		}
	}
	return c
}

// unwrap drops the parentheses the parser's String puts around a whole
// expression.
func unwrap(s string) string {
	for len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		depth := 0
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '(':
				depth++
			case ')':
				depth--
			case '\'', '"':
				if j := strings.IndexByte(s[i+1:], s[i]); j >= 0 {
					i += j + 1
				}
			}
			if depth == 0 && i < len(s)-1 {
				return s // the first parenthesis closes before the end
			}
		}
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}
