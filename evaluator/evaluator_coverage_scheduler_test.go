package evaluator

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
)

const schedulerConfig = `
PROGRAM Prog
	VAR_OUTPUT COUNT : INT; END_VAR
	COUNT := COUNT + 1;
END_PROGRAM
CONFIGURATION Cell
	RESOURCE R1 ON CPU
		TASK Slow(INTERVAL := T#1h, PRIORITY := 5);
		TASK Fast(INTERVAL := T#10ms, PRIORITY := 1);
		TASK Ev(SINGLE := trig, PRIORITY := 3);
		PROGRAM P1 WITH Fast : Prog (COUNT => total);
		PROGRAM P2 WITH Ev : Prog;
		PROGRAM P3 : Prog;
	END_RESOURCE
END_CONFIGURATION
`

func TestSchedulerCycles(t *testing.T) {
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, schedulerConfig, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	s, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	// Tasks are ordered by priority; a program without a task runs in a
	// background task of the lowest priority.
	names := []string{}
	for _, task := range s.Tasks {
		names = append(names, task.Name)
	}
	if got := strings.Join(names, ","); got != "Fast,Ev,Slow,BACKGROUND" {
		t.Fatalf("task order = %s, want Fast,Ev,Slow,BACKGROUND", got)
	}

	count := func(instance string) string {
		v, _ := programInstanceEnv(t, env, "R1", instance).Get("COUNT")
		return v.Inspect()
	}
	env.Set("trig", FALSE)
	start := time.Now()

	// First cycle: the periodic tasks are due; the event has not fired.
	runSchedulerCycle(s, env, start)
	if count("P1") != "1" || count("P2") != "0" {
		t.Fatalf("after cycle 1: P1=%s P2=%s", count("P1"), count("P2"))
	}
	// The output mapping copies COUNT to total.
	if total, _ := env.Get("total"); total == nil || total.Inspect() != "1" {
		t.Fatalf("total = %v, want 1", total)
	}

	// Before Fast's interval has passed, nothing runs.
	runSchedulerCycle(s, env, start.Add(5*time.Millisecond))
	if count("P1") != "1" {
		t.Fatalf("P1 ran before its interval: %s", count("P1"))
	}

	// A rising edge of the trigger runs the event task once.
	env.Set("trig", TRUE)
	runSchedulerCycle(s, env, start.Add(20*time.Millisecond))
	runSchedulerCycle(s, env, start.Add(40*time.Millisecond))
	if count("P1") != "3" || count("P2") != "1" || count("P3") != "4" {
		t.Fatalf("after cycles 3-4: P1=%s P2=%s P3=%s", count("P1"), count("P2"), count("P3"))
	}
}

func TestNewSchedulerWithoutResource(t *testing.T) {
	if _, err := NewScheduler(object.NewEnvironment()); err == nil || !strings.Contains(err.Message, "no resource found in configuration") {
		t.Fatalf("expected a missing-resource error, got %v", err)
	}
}

// RunScheduler runs scan cycles until its context is done.
func TestRunScheduler(t *testing.T) {
	env := object.NewEnvironment()
	if result := testEvalWithEnv(t, schedulerConfig, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	s, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	env.Set("trig", FALSE)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	RunScheduler(ctx, s, env, time.Millisecond)
	count, _ := programInstanceEnv(t, env, "R1", "P1").Get("COUNT")
	if count.Inspect() == "0" {
		t.Error("the scheduler ran no cycles")
	}
}

// A task with neither INTERVAL nor SINGLE runs on every scan, and tasks of
// the same priority run in the order of their names.
func TestSchedulerEveryScanTasks(t *testing.T) {
	env := object.NewEnvironment()
	config := `
PROGRAM Prog
	VAR_OUTPUT COUNT : INT; END_VAR
	COUNT := COUNT + 1;
END_PROGRAM
CONFIGURATION Cell
	RESOURCE R1 ON CPU
		TASK Beta(PRIORITY := 2);
		TASK Alpha(PRIORITY := 2);
		PROGRAM B1 WITH Beta : Prog;
		PROGRAM Free : Prog;
	END_RESOURCE
END_CONFIGURATION
`
	if result := testEvalWithEnv(t, config, env); isError(result) {
		t.Fatalf("configuration evaluation failed: %s", result.Inspect())
	}
	s, err := NewScheduler(env)
	if err != nil {
		t.Fatalf("NewScheduler failed: %s", err.Inspect())
	}
	names := []string{}
	for _, task := range s.Tasks {
		names = append(names, fmt.Sprintf("%s:%d", task.Name, task.Priority))
	}
	if got := strings.Join(names, ","); got != "Alpha:2,Beta:2,BACKGROUND:3" {
		t.Fatalf("tasks = %s", got)
	}
	start := time.Now()
	runSchedulerCycle(s, env, start)
	runSchedulerCycle(s, env, start)
	for _, instance := range []string{"B1", "Free"} {
		if v, _ := programInstanceEnv(t, env, "R1", instance).Get("COUNT"); v.Inspect() != "2" {
			t.Errorf("%s ran %s times in 2 scans, want 2", instance, v.Inspect())
		}
	}
}
