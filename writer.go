package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Writer handles dynamic output routing based on terminal presence and agent flags.
type Writer struct {
	stdout io.Writer
	stderr io.Writer
	isTTY  bool
	force  bool
	logger *Logger
}

// NewWriter returns a configured output writer for the current execution context.
func NewWriter(cmd *cobra.Command) *Writer {
	forceAgent, _ := cmd.Flags().GetBool("agent")

	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	// Pure-Go, zero-dependency TTY check
	stdoutIsTTY := isTerminal(stdout)

	w := &Writer{
		stdout: stdout,
		stderr: stderr,
		isTTY:  stdoutIsTTY && !forceAgent,
		force:  forceAgent,
	}
	w.logger = NewLogger(stderr, w.isTTY)
	return w
}

// IsTTY returns true if the writer is in TTY (Human Pretty) mode.
func (w *Writer) IsTTY() bool {
	return w.isTTY
}

// Log writes a message to the logging stream, leveraging deduplication.
func (w *Writer) Log(msg string) {
	w.logger.Log(msg)
}

// Progress writes a progress update, using overwrites in TTY and token collapsing in Agent mode.
func (w *Writer) Progress(msg string) {
	w.logger.LogProgress(msg)
}

// Flush flushes any deferred duplicate logs.
func (w *Writer) Flush() {
	w.logger.Flush()
}

// WriteSuccess writes standard output. If in TTY mode, writes human text; otherwise writes structured JSON.
func (w *Writer) WriteSuccess(humanText string, jsonPayload any) {
	if w.isTTY {
		fmt.Fprintln(w.stdout, humanText)
	} else {
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{
			"status": "ok",
			"result": jsonPayload,
		})
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
