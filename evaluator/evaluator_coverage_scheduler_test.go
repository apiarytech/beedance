package evaluator

import (
	"strings"
	"testing"
	"time"

	"beedance/object"
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
	// Tasks are ordered by priority; a program without a task is not scheduled.
	names := []string{}
	for _, task := range s.Tasks {
		names = append(names, task.Name)
	}
	if got := strings.Join(names, ","); got != "Fast,Ev,Slow" {
		t.Fatalf("task order = %s, want Fast,Ev,Slow", got)
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
	if count("P1") != "3" || count("P2") != "1" || count("P3") != "0" {
		t.Fatalf("after cycles 3-4: P1=%s P2=%s P3=%s", count("P1"), count("P2"), count("P3"))
	}
}

func TestNewSchedulerWithoutResource(t *testing.T) {
	if _, err := NewScheduler(object.NewEnvironment()); err == nil || !strings.Contains(err.Message, "no resource found in configuration") {
		t.Fatalf("expected a missing-resource error, got %v", err)
	}
}
