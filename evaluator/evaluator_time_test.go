package evaluator

import (
	"testing"
	"time"

	"github.com/apiarytech/beedance/object"
)

func TestTimeDateLiteralEvaluation(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		// TIME literals
		{"T#5s;", 5 * time.Second},
		{"TIME#1h_30m;", 90 * time.Minute},
		{"T#-10s_500ms;", -10*time.Second - 500*time.Millisecond},
		{"T#1.5s;", 1500 * time.Millisecond},

		// DATE literals
		{"D#2026-05-21;", time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)},
		{"DATE#2026-05-21;", time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)},

		// TIME_OF_DAY literals
		{"TOD#14:30:00;", time.Date(0, 1, 1, 14, 30, 0, 0, time.UTC)},
		{"TIME_OF_DAY#14:30:00.123;", time.Date(0, 1, 1, 14, 30, 0, 123000000, time.UTC)},

		// DATE_AND_TIME literals
		{"DT#2026-05-21-14:30:00;", time.Date(2026, 5, 21, 14, 30, 0, 0, time.UTC)},
		{"DATE_AND_TIME#2026-05-21-14:30:00.5;", time.Date(2026, 5, 21, 14, 30, 0, 500000000, time.UTC)},

		// Error cases
		{"T#5z;", "could not parse TIME literal"},
		{"D#2026/05/21;", "could not parse DATE literal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(t, tt.input)

			switch expected := tt.expected.(type) {
			case time.Duration:
				testTimeObject(t, evaluated, tt.input, expected)
			case time.Time:
				switch obj := evaluated.(type) {
				case *object.Date:
					if !obj.Value.Equal(expected) {
						t.Errorf("wrong date. want=%v, got=%v", expected, obj.Value)
					}
				case *object.TimeOfDay:
					// Compare only the time part for TOD
					if obj.Value.Format("15:04:05.999") != expected.Format("15:04:05.999") {
						t.Errorf("wrong time of day. want=%v, got=%v", expected, obj.Value)
					}
				case *object.DateAndTime:
					if !obj.Value.Equal(expected) {
						t.Errorf("wrong date and time. want=%v, got=%v", expected, obj.Value)
					}
				default:
					t.Fatalf("unhandled time type. got=%T", evaluated)
				}
			case string:
				testErrorObjectContains(t, evaluated, expected)
			default:
				t.Fatalf("unhandled expected type: %T", tt.expected)
			}
		})
	}
}
