package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Writer handles dynamic output routing based on terminal presence and agent flags.
type Writer struct {
	mu     sync.Mutex
	stdout io.Writer
	stderr io.Writer
	isTTY  bool
	force  bool   // reserved: will back --force/--yes bypass of the non-interactive guard (v0.4)
	logger *Logger
}

// NewWriter returns a configured output writer.
// Set agentMode true to force JSON output regardless of TTY state.
func NewWriter(stdout, stderr io.Writer, agentMode bool) *Writer {
	isTTY := isTerminal(stdout) && !agentMode
	w := &Writer{
		stdout: stdout,
		stderr: stderr,
		isTTY:  isTTY,
		force:  agentMode,
	}
	w.logger = NewLogger(stderr, w.isTTY)
	return w
}

// IsTTY returns true if the writer is in human (TTY) mode.
func (w *Writer) IsTTY() bool {
	return w.isTTY
}

// Log writes a message to stderr, deduplicating consecutive duplicates in agent mode.
func (w *Writer) Log(msg string) {
	w.logger.Log(msg)
}

// Progress writes a progress update; overwrites current line in TTY, collapses in agent mode.
func (w *Writer) Progress(msg string) {
	w.logger.LogProgress(msg)
}

// Flush flushes any deferred deduplicated logs.
func (w *Writer) Flush() {
	w.logger.Flush()
}

// WriteSuccess writes to stdout. TTY mode writes humanText; agent mode writes a
// JSON envelope with status, schema_version, tool_version (if set), and result.
func (w *Writer) WriteSuccess(humanText string, jsonPayload any) {
	if w.isTTY {
		fmt.Fprintln(w.stdout, humanText)
	} else {
		envelope := map[string]any{
			"status":         "ok",
			"schema_version": SchemaVersion,
			"result":         jsonPayload,
		}
		if ToolVersion != "" {
			envelope["tool_version"] = ToolVersion
		}
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(envelope)
	}
}

// WriteEvent writes a single minified JSON object to stdout on one line.
// It is safe to call WriteEvent concurrently from multiple goroutines.
// However, WriteSuccess and WriteError are not mutex-protected — call them only
// after all WriteEvent goroutines have completed (e.g., after a sync.WaitGroup.Wait()).
// In TTY mode WriteEvent is a no-op (events are machine-only).
func (w *Writer) WriteEvent(v any) {
	if w.isTTY {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(w.stdout, "%s\n", data)
}

// ProgressEvent carries typed progress state for long-running operations.
// All fields are optional — populate what is meaningful for the operation.
type ProgressEvent struct {
	Stage   string  `json:"stage,omitempty"`
	Current int     `json:"current,omitempty"`
	Total   int     `json:"total,omitempty"`
	Percent float64 `json:"percent,omitempty"`
	EtaMs   int64   `json:"eta_ms,omitempty"`
	Message string  `json:"message,omitempty"`
}

// WriteProgress emits a structured progress event to stderr.
// Agent mode: minified JSON on one line (via json.Marshal, consistent with WriteEvent).
// TTY mode: formatted human-readable line with carriage return to overwrite.
func (w *Writer) WriteProgress(evt ProgressEvent) {
	if w.isTTY {
		line := evt.Message
		if evt.Stage != "" {
			line = "[" + evt.Stage + "] " + line
		}
		if evt.Total > 0 {
			line += fmt.Sprintf(" (%d/%d", evt.Current, evt.Total)
			if evt.Percent > 0 {
				line += fmt.Sprintf(", %.0f%%", evt.Percent)
			}
			line += ")"
		}
		fmt.Fprintf(w.stderr, "\r\033[K%s", line)
		return
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	fmt.Fprintf(w.stderr, "%s\n", data)
}

// isTerminal returns true if the given writer is a character device (TTY).
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil {
			return (stat.Mode() & os.ModeCharDevice) != 0
		}
	}
	return false
}
