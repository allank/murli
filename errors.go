package murli

import (
	"encoding/json"
	"fmt"
	"os"
)

// Exit codes mapping to standard Agent actions
const (
	ExitOK        = 0 // Successful execution
	ExitUserError = 1 // Bad input or argument configuration
	ExitToolError = 2 // Environment, network, or filesystem crash
	ExitPartial   = 3 // Some operations succeeded, some failed
)

// ExitFunc is a package-level variable to enable clean mock testing of os.Exit.
var ExitFunc = os.Exit

// AgentError represents a structured error returned to an LLM-based agent.
type AgentError struct {
	Code        int    `json:"code"`
	ErrorType   string `json:"error"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
	Recoverable bool   `json:"recoverable"`
}

// Error implements the standard Go error interface.
func (e *AgentError) Error() string {
	return e.Message
}

// WriteError writes the structured error to stderr and exits with the defined exit code.
func (w *Writer) WriteError(err *AgentError) {
	if w.isTTY {
		fmt.Fprintf(w.stderr, "Error: %s\n", err.Message)
		if err.Suggestion != "" {
			fmt.Fprintf(w.stderr, "Hint:  %s\n", err.Suggestion)
		}
	} else {
		enc := json.NewEncoder(w.stderr)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(err)
	}
	ExitFunc(err.Code)
}
