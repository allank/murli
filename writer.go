package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Writer handles dynamic output routing based on terminal presence and agent flags.
type Writer struct {
	stdout io.Writer
	stderr io.Writer
	isTTY  bool
	force  bool
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
