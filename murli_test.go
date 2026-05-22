package murli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

// TestSchemaGeneration verifies standard metadata extraction, argument parsing, type conversion, and subcommand listing.
func TestSchemaGeneration(t *testing.T) {
	root := &cobra.Command{
		Use:   "demo <target> [option]",
		Short: "A lovely demonstration CLI command",
	}
	root.Flags().Int("limit", 10, "Page limit")
	root.Flags().Bool("verbose", false, "Enable verbose logs")

	sub := &cobra.Command{
		Use:   "subcmd",
		Short: "Child command",
	}
	root.AddCommand(sub)

	Annotate(root, Metadata{
		AgentDescription: "Use this command to demonstrate schema generation capabilities.",
		WhenToUse:        "When needing to verify the schema generation engine is healthy.",
		Idempotent:       true,
		Arguments: []ArgumentMetadata{
			{
				Name:        "target",
				Type:        "string",
				Description: "The focal element under inspection",
			},
		},
		Returns: &ReturnSchema{
			Type:        "json",
			Description: "Operation metadata and timestamps",
			Shape: map[string]any{
				"id":        "string",
				"timestamp": "int64",
			},
		},
		Examples: []string{
			"demo server-123 --limit 5",
		},
	})

	// Capture output
	buf := &bytes.Buffer{}
	root.SetOut(buf)

	err := EmitSchema(root)
	if err != nil {
		t.Fatalf("failed to emit schema: %v", err)
	}

	var schema CommandSchema
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse schema JSON: %v", err)
	}

	// Assertions
	if schema.Name != "demo" {
		t.Errorf("expected name 'demo', got %q", schema.Name)
	}
	if schema.Summary != "A lovely demonstration CLI command" {
		t.Errorf("incorrect summary: %q", schema.Summary)
	}
	if schema.AgentDescription != "Use this command to demonstrate schema generation capabilities." {
		t.Errorf("incorrect agent description")
	}
	if !schema.Idempotent {
		t.Errorf("expected idempotent = true")
	}

	// Arguments check (verify target enriched, option default type string)
	if len(schema.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(schema.Arguments))
	}
	if schema.Arguments[0].Name != "target" || schema.Arguments[0].Required != true || schema.Arguments[0].Description != "The focal element under inspection" {
		t.Errorf("failed target argument enrichment: %+v", schema.Arguments[0])
	}
	if schema.Arguments[1].Name != "option" || schema.Arguments[1].Required != false || schema.Arguments[1].Description != "" {
		t.Errorf("failed option argument parsing: %+v", schema.Arguments[1])
	}

	// Flags check (verify default values typed as int/bool, not string)
	var flagLimit, flagVerbose bool
	for _, f := range schema.Flags {
		if f.Name == "limit" {
			flagLimit = true
			if f.Type != "int" || f.Default.(float64) != 10 { // JSON numbers parse as float64
				t.Errorf("limit flag mapped incorrectly: %+v", f)
			}
		}
		if f.Name == "verbose" {
			flagVerbose = true
			if f.Type != "bool" || f.Default.(bool) != false {
				t.Errorf("verbose flag mapped incorrectly: %+v", f)
			}
		}
	}
	if !flagLimit || !flagVerbose {
		t.Errorf("missing flags in schema")
	}

	// Returns check
	if schema.Returns == nil || schema.Returns.Type != "json" || schema.Returns.Shape["id"] != "string" {
		t.Errorf("incorrect return schema: %+v", schema.Returns)
	}

	// Subcommand check
	if len(schema.Subcommands) != 1 || schema.Subcommands[0].Name != "subcmd" {
		t.Errorf("subcommand mapping failed: %+v", schema.Subcommands)
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
		want := "Loading config (repeated 1 times progress)\nConnecting db\n"
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})
}

// TestMiddlewareInterception tests error handling wrapping and flags checking.
func TestMiddlewareInterception(t *testing.T) {
	var capturedExitCode int
	ExitFunc = func(code int) {
		capturedExitCode = code
	}
	defer func() {
		ExitFunc = func(code int) { /* restore or default */ }
	}()

	t.Run("Standard command error wrapping", func(t *testing.T) {
		capturedExitCode = -999
		cmd := &cobra.Command{
			Use: "failcmd",
			RunE: func(c *cobra.Command, args []string) error {
				return errors.New("underlying socket closed")
			},
		}

		Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)

		// Call execution
		_ = cmd.Execute()

		if capturedExitCode != ExitToolError {
			t.Errorf("expected exit code %d, got %d", ExitToolError, capturedExitCode)
		}

		var resp AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse wrapped error: %v", err)
		}

		if resp.Code != ExitToolError || resp.ErrorType != "execution_error" || resp.Message != "underlying socket closed" {
			t.Errorf("mismatched wrapped error fields: %+v", resp)
		}
	})

	t.Run("AgentError interception", func(t *testing.T) {
		capturedExitCode = -999
		cmd := &cobra.Command{
			Use: "customfail",
			RunE: func(c *cobra.Command, args []string) error {
				return &AgentError{
					Code:        ExitUserError,
					ErrorType:   "file_not_found",
					Message:     "config.json is missing",
					Suggestion:  "Create config.json under project root.",
					Recoverable: true,
				}
			},
		}

		Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)

		_ = cmd.Execute()

		if capturedExitCode != ExitUserError {
			t.Errorf("expected exit code %d, got %d", ExitUserError, capturedExitCode)
		}

		var resp AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if resp.ErrorType != "file_not_found" || resp.Suggestion != "Create config.json under project root." {
			t.Errorf("mismatched fields: %+v", resp)
		}
	})

	t.Run("Flag parsing error interception", func(t *testing.T) {
		capturedExitCode = -999
		cmd := &cobra.Command{
			Use: "flagtest",
			RunE: func(c *cobra.Command, args []string) error {
				return nil
			},
		}
		cmd.Flags().Int("num", 0, "A number")

		Enable(cmd)
		bufErr := &bytes.Buffer{}
		cmd.SetErr(bufErr)

		// Set bad arguments to trigger flag parsing failure
		cmd.SetArgs([]string{"--num", "not-a-number"})
		_ = cmd.Execute()

		if capturedExitCode != ExitUserError {
			t.Errorf("expected exit code %d, got %d", ExitUserError, capturedExitCode)
		}

		var resp AgentError
		if err := json.Unmarshal(bufErr.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if resp.ErrorType != "flag_error" || !strings.Contains(resp.Message, "not-a-number") {
			t.Errorf("incorrect flag error wrap: %+v", resp)
		}
	})
}
