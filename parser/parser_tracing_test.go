package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

// captureOutput redirects stdout to a buffer for the duration of a function call
// and returns the captured output as a string.
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestTracingFunctions(t *testing.T) {
	// Save initial state and schedule restoration.
	originalTraceLevel := traceLevel
	originalEnableTracing := enableTracing
	t.Cleanup(func() {
		traceLevel = originalTraceLevel
		enableTracing = originalEnableTracing
	})

	// --- Test when tracing is disabled ---
	t.Run("TracingDisabled", func(t *testing.T) {
		traceLevel = 0
		SetTracing(false)

		if identLevel() != "" {
			t.Errorf("identLevel() should be empty when tracing is disabled, got %q", identLevel())
		}
		incIdent()
		if traceLevel != 0 {
			t.Errorf("incIdent() should not change traceLevel when tracing is disabled, got %d", traceLevel)
		}
		decIdent()
		if traceLevel != 0 {
			t.Errorf("decIdent() should not change traceLevel when tracing is disabled, got %d", traceLevel)
		}
		output := captureOutput(func() {
			tracePrint("hello")
		})
		if output != "" {
			t.Errorf("tracePrint() should not print when tracing is disabled, got %q", output)
		}
	})

	// --- Test when tracing is enabled ---
	t.Run("TracingEnabled", func(t *testing.T) {
		SetTracing(true)
		traceLevel = 0 // Reset level for this sub-test

		// Test incIdent and decIdent
		incIdent()
		if traceLevel != 1 {
			t.Errorf("incIdent() failed. want=1, got=%d", traceLevel)
		}
		incIdent()
		if traceLevel != 2 {
			t.Errorf("incIdent() failed. want=2, got=%d", traceLevel)
		}
		decIdent()
		if traceLevel != 1 {
			t.Errorf("decIdent() failed. want=1, got=%d", traceLevel)
		}
		decIdent()
		if traceLevel != 0 {
			t.Errorf("decIdent() failed. want=0, got=%d", traceLevel)
		}

		// Test identLevel
		traceLevel = 1
		if identLevel() != "\t" {
			t.Errorf("identLevel() at level 1 wrong. want=\"\\t\", got=%q", identLevel())
		}
		traceLevel = 2
		if identLevel() != "\t\t" {
			t.Errorf("identLevel() at level 2 wrong. want=\"\\t\\t\", got=%q", identLevel())
		}

		// Test tracePrint
		traceLevel = 3 // Set a known level for printing
		output := captureOutput(func() {
			tracePrint("test message")
		})
		expectedOutput := fmt.Sprintf("%s%s\n", "\t\t\t", "test message")
		if output != expectedOutput {
			t.Errorf("tracePrint() output wrong.\nwant=%q\ngot=%q", expectedOutput, output)
		}
	})

	// --- Test trace and untrace ---
	t.Run("TraceAndUntrace", func(t *testing.T) {
		SetTracing(true)
		traceLevel = 0

		output := captureOutput(func() {
			func() {
				defer untrace(trace("my function"))
				tracePrint("inside function")
			}()
		})

		expected := "BEGIN my function\n\tinside function\nEND my function\n"
		if output != expected {
			t.Errorf("trace/untrace output wrong.\nwant:\n%q\ngot:\n%q", expected, output)
		}

		if traceLevel != 0 {
			t.Errorf("traceLevel should be 0 after trace/untrace block, got %d", traceLevel)
		}
	})
}
