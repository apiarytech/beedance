package evaluator

import (
	"strings"
	"testing"

	"beedance/object"
)

// evalCase is an input and its expected result: the printed form of the
// value, or "ERROR: <part of the message>" for an error.
type evalCase struct {
	input string
	want  string
}

// checkEval evaluates each input in a fresh environment and compares the
// result with want.
func checkEval(t *testing.T, cases []evalCase) {
	t.Helper()
	for _, c := range cases {
		result := testEvalWithEnv(t, c.input, object.NewEnvironment())
		checkEvalResult(t, c.input, result, c.want)
	}
}

func checkEvalResult(t *testing.T, input string, result object.Object, want string) {
	t.Helper()
	if result == nil {
		t.Errorf("%s: got no result", input)
		return
	}
	got := result.Inspect()
	if err, isErr := result.(*object.Error); isErr {
		got = "ERROR: " + err.Message
	}
	if rest, wantErr := strings.CutPrefix(want, "ERROR:"); wantErr {
		if !strings.HasPrefix(got, "ERROR: ") || !strings.Contains(got, strings.TrimSpace(rest)) {
			t.Errorf("%s:\n  expected an error containing %q\n  got %s", input, strings.TrimSpace(rest), got)
		}
		return
	}
	if got != want {
		t.Errorf("%s:\n  expected %s\n  got      %s", input, want, got)
	}
}
