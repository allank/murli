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
		if resp.SchemaVersion != SchemaVersion {
			t.Errorf("SchemaVersion: want %q, got %q", SchemaVersion, resp.SchemaVersion)
		}
		// ToolVersion is "" (default), so with omitempty it should NOT appear in JSON
		if resp.ToolVersion != "" {
			t.Errorf("ToolVersion: want empty, got %q", resp.ToolVersion)
		}
	})
}

func TestExtendedExitCodes(t *testing.T) {
	if ExitTimeout != 4 {
		t.Errorf("ExitTimeout: want 4, got %d", ExitTimeout)
	}
	if ExitNotFound != 5 {
		t.Errorf("ExitNotFound: want 5, got %d", ExitNotFound)
	}
	if ExitPermission != 6 {
		t.Errorf("ExitPermission: want 6, got %d", ExitPermission)
	}
	if ExitConflict != 7 {
		t.Errorf("ExitConflict: want 7, got %d", ExitConflict)
	}
	if ExitRateLimited != 8 {
		t.Errorf("ExitRateLimited: want 8, got %d", ExitRateLimited)
	}
	if ExitCancelled != 9 {
		t.Errorf("ExitCancelled: want 9, got %d", ExitCancelled)
	}
}

func TestNewUserError(t *testing.T) {
	err := NewUserError("invalid value", "provide a number between 1 and 10")
	if err.Code != ExitUserError {
		t.Errorf("Code: want %d, got %d", ExitUserError, err.Code)
	}
	if err.Message != "invalid value" {
		t.Errorf("Message: want %q, got %q", "invalid value", err.Message)
	}
	if err.Suggestion != "provide a number between 1 and 10" {
		t.Errorf("Suggestion: want %q, got %q", "provide a number between 1 and 10", err.Suggestion)
	}
	if !err.Recoverable {
		t.Error("NewUserError must be Recoverable = true")
	}
	if err.ErrorType == "" {
		t.Error("NewUserError must set ErrorType")
	}
}

func TestNewToolError(t *testing.T) {
	err := NewToolError("database connection refused")
	if err.Code != ExitToolError {
		t.Errorf("Code: want %d, got %d", ExitToolError, err.Code)
	}
	if err.Message != "database connection refused" {
		t.Errorf("Message: want %q, got %q", "database connection refused", err.Message)
	}
	if err.Recoverable {
		t.Error("NewToolError must be Recoverable = false")
	}
	if err.ErrorType == "" {
		t.Error("NewToolError must set ErrorType")
	}
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

func TestVersionInSuccessEnvelope(t *testing.T) {
	ToolVersion = "1.2.3"
	defer func() { ToolVersion = "" }()

	buf := &bytes.Buffer{}
	w := &Writer{stdout: buf, isTTY: false}
	w.WriteSuccess("done", map[string]any{"key": "val"})

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, buf.String())
	}
	if resp["schema_version"] != SchemaVersion {
		t.Errorf("schema_version: want %q, got %v", SchemaVersion, resp["schema_version"])
	}
	if resp["tool_version"] != "1.2.3" {
		t.Errorf("tool_version: want %q, got %v", "1.2.3", resp["tool_version"])
	}
}

func TestVersionInErrorEnvelope(t *testing.T) {
	ToolVersion = "2.0.0"
	defer func() { ToolVersion = "" }()

	var capturedCode int
	ExitFunc = func(code int) { capturedCode = code }
	defer func() { ExitFunc = func(code int) {} }()

	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: false}
	w.WriteError(NewToolError("disk full"))

	_ = capturedCode
	var got AgentError
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, buf.String())
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version: want %q, got %q", SchemaVersion, got.SchemaVersion)
	}
	if got.ToolVersion != "2.0.0" {
		t.Errorf("tool_version: want %q, got %q", "2.0.0", got.ToolVersion)
	}
}

func TestAgentErrorExtendedFields(t *testing.T) {
	var capturedCode int
	ExitFunc = func(code int) { capturedCode = code }
	defer func() { ExitFunc = func(code int) {} }()

	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: false}

	err := &AgentError{
		Code:         ExitRateLimited,
		ErrorType:    "rate_limited",
		Message:      "API quota exceeded",
		Suggestion:   "Wait and retry.",
		Recoverable:  true,
		RetryAfterMs: 5000,
		DocURL:       "https://example.com/rate-limits",
		ValidValues:  []string{"us-east-1", "us-west-2"},
		Field:        "region",
	}

	w.WriteError(err)

	if capturedCode != ExitRateLimited {
		t.Errorf("exit code: want %d, got %d", ExitRateLimited, capturedCode)
	}

	var got AgentError
	if parseErr := json.Unmarshal(buf.Bytes(), &got); parseErr != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", parseErr, buf.String())
	}
	if got.RetryAfterMs != 5000 {
		t.Errorf("RetryAfterMs: want 5000, got %d", got.RetryAfterMs)
	}
	if got.DocURL != "https://example.com/rate-limits" {
		t.Errorf("DocURL: want %q, got %q", "https://example.com/rate-limits", got.DocURL)
	}
	if len(got.ValidValues) != 2 || got.ValidValues[0] != "us-east-1" {
		t.Errorf("ValidValues: want [us-east-1 us-west-2], got %v", got.ValidValues)
	}
	if got.Field != "region" {
		t.Errorf("Field: want %q, got %q", "region", got.Field)
	}
}
