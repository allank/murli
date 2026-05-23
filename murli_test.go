package murli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestSuccessWriter verifies success outputs in TTY (human) vs non-TTY (agent) modes.
func TestSuccessWriter(t *testing.T) {
	t.Run("TTY mode success", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := &Writer{
			stdout: buf,
			isTTY:  true,
		}

		w.WriteSuccess("All tasks completed successfully", map[string]any{"count": 42})

		got := buf.String()
		want := "All tasks completed successfully\n"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("Agent mode success", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := &Writer{
			stdout: buf,
			isTTY:  false,
		}

		w.WriteSuccess("All tasks completed successfully", map[string]any{"count": 42})

		var resp struct {
			Status string         `json:"status"`
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", resp.Status)
		}
		if resp.Result["count"].(float64) != 42 {
			t.Errorf("expected result count 42, got %v", resp.Result["count"])
		}
	})
}

// TestErrorWriter verifies structured error serialization and exit code triggering.
func TestErrorWriter(t *testing.T) {
	// Temporarily capture exit code
	var capturedExitCode int
	ExitFunc = func(code int) {
		capturedExitCode = code
	}
	defer func() {
		ExitFunc = func(code int) { /* restore or default */ }
	}()

	errPayload := &AgentError{
		Code:        ExitUserError,
		ErrorType:   "invalid_parameter",
		Message:     "The provided value is out of bounds",
		Suggestion:  "Ensure the value is between 1 and 10",
		Recoverable: true,
	}

	t.Run("TTY mode error", func(t *testing.T) {
		capturedExitCode = -999
		buf := &bytes.Buffer{}
		w := &Writer{
			stderr: buf,
			isTTY:  true,
		}

		w.WriteError(errPayload)

		if capturedExitCode != ExitUserError {
			t.Errorf("expected exit code %d, got %d", ExitUserError, capturedExitCode)
		}

		got := buf.String()
		if !strings.Contains(got, "Error: The provided value is out of bounds") {
			t.Errorf("missing error message in %q", got)
		}
		if !strings.Contains(got, "Hint:  Ensure the value is between 1 and 10") {
			t.Errorf("missing hint message in %q", got)
		}
	})

	t.Run("Agent mode error", func(t *testing.T) {
		capturedExitCode = -999
		buf := &bytes.Buffer{}
		w := &Writer{
			stderr: buf,
			isTTY:  false,
		}

		w.WriteError(errPayload)

		if capturedExitCode != ExitUserError {
			t.Errorf("expected exit code %d, got %d", ExitUserError, capturedExitCode)
		}

		var resp AgentError
		if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse error JSON: %v", err)
		}

		if resp.Code != ExitUserError || resp.ErrorType != "invalid_parameter" || resp.Message != "The provided value is out of bounds" || !resp.Recoverable {
			t.Errorf("mismatched structured error payload: %+v", resp)
		}
	})
}

// TestLogDeduplication validates logging collapsing and overwriting.
func TestLogDeduplication(t *testing.T) {
	t.Run("TTY mode overwrites", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger(buf, true)

		l.LogProgress("Task A: 10%")
		l.LogProgress("Task A: 20%")

		got := buf.String()
		// TTY uses carriage returns and clear-line codes
		if !strings.Contains(got, "\r\033[KTask A: 10%") || !strings.Contains(got, "\r\033[KTask A: 20%") {
			t.Errorf("missing TTY overwrite sequences: %q", got)
		}
	})

	t.Run("Agent mode log collapsing", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger(buf, false)

		l.Log("Hello")
		l.Log("Hello")
		l.Log("Hello")
		l.Log("World")
		l.Flush()

		got := buf.String()
		want := "Hello (repeated 2 times)\nWorld\n"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("Agent mode progress collapsing", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger(buf, false)

		l.LogProgress("Loading config")
		l.LogProgress("Loading config")
		l.LogProgress("Connecting db")
		l.Flush()

		got := buf.String()
		want := "Loading config (repeated 1 time, progress)\nConnecting db\n"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})
}
