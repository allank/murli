package murli

import (
	"encoding/json"
	"fmt"
	"os"
)

// Exit codes 0–3: table-stakes (present since v0.1).
const (
	ExitOK        = 0 // Success
	ExitUserError = 1 // Bad input or arguments
	ExitToolError = 2 // Environment/internal failure
	ExitPartial   = 3 // Some operations succeeded, some failed
)

// Exit codes 4–9: extended taxonomy (v0.2).
const (
	ExitTimeout     = 4 // Operation timed out; retry may succeed
	ExitNotFound    = 5 // Requested resource does not exist
	ExitPermission  = 6 // Caller lacks permission; not retryable without auth change
	ExitConflict    = 7 // State conflict (e.g. resource already exists)
	ExitRateLimited = 8 // Rate limit hit; retry after RetryAfterMs
	ExitCancelled   = 9 // Operation cancelled by signal or context
)

// ExitFunc is swapped out in tests to capture exit codes without terminating.
var ExitFunc = os.Exit

// AgentError is the structured error envelope written to stderr.
// Fields are serialised as JSON in agent mode; SchemaVersion and ToolVersion
// are auto-populated by WriteError — do not set them manually.
type AgentError struct {
	Code          int      `json:"code"`
	ErrorType     string   `json:"error"`
	Message       string   `json:"message"`
	Suggestion    string   `json:"suggestion,omitempty"`
	Recoverable   bool     `json:"recoverable"`
	ValidValues   []string `json:"valid_values,omitempty"`
	RetryAfterMs  int      `json:"retry_after_ms,omitempty"`
	DocURL        string   `json:"doc_url,omitempty"`
	Field         string   `json:"field,omitempty"`
	SchemaVersion string   `json:"schema_version,omitempty"`
	ToolVersion   string   `json:"tool_version,omitempty"`
}

// Error implements the standard Go error interface.
func (e *AgentError) Error() string {
	return e.Message
}

// NewUserError returns a recoverable AgentError for bad input.
// Use when the caller supplied invalid arguments or configuration.
func NewUserError(message, suggestion string) *AgentError {
	return &AgentError{
		Code:        ExitUserError,
		ErrorType:   "user_error",
		Message:     message,
		Suggestion:  suggestion,
		Recoverable: true,
	}
}

// NewToolError returns a non-recoverable AgentError for internal/environment failures.
// Use when the fault is in the environment (network, filesystem, dependency) not the caller.
func NewToolError(message string) *AgentError {
	return &AgentError{
		Code:        ExitToolError,
		ErrorType:   "tool_error",
		Message:     message,
		Recoverable: false,
	}
}

// WriteError writes the structured error to stderr and exits with the error's exit code.
// In TTY mode: human-readable. In agent mode: JSON envelope with schema_version and tool_version.
func (w *Writer) WriteError(err *AgentError) {
	if w.isTTY {
		fmt.Fprintf(w.stderr, "Error: %s\n", err.Message)
		if err.Suggestion != "" {
			fmt.Fprintf(w.stderr, "Hint:  %s\n", err.Suggestion)
		}
	} else {
		toWrite := *err // copy struct so we don't mutate the caller's
		if len(err.ValidValues) > 0 {
			toWrite.ValidValues = append([]string(nil), err.ValidValues...)
		}
		if w.ProtocolVersion() != "0.1" {
			toWrite.SchemaVersion = SchemaVersion
			toWrite.ToolVersion = ToolVersion
		}
		enc := json.NewEncoder(w.stderr)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(&toWrite)
	}
	ExitFunc(err.Code)
}
