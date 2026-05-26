package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Logger writes diagnostic messages to stderr.
// TTY mode: human-readable plain text.
// Agent mode: one NDJSON object per line {ts, level, msg}; consecutive duplicates
// are collapsed into a single entry with a "repeated" count.
type Logger struct {
	writer     io.Writer
	lastLine   string
	dupCount   int
	isTTY      bool
	isProgress bool
}

// NewLogger initializes a Logger writing to writer.
func NewLogger(writer io.Writer, isTTY bool) *Logger {
	return &Logger{writer: writer, isTTY: isTTY}
}

// Log writes a message. In TTY mode: plain text. In agent mode: NDJSON with deduplication.
func (l *Logger) Log(line string) {
	if l.isTTY {
		fmt.Fprintln(l.writer, line)
		return
	}
	if line == l.lastLine && !l.isProgress {
		l.dupCount++
		return
	}
	l.Flush()
	l.lastLine = line
	l.dupCount = 0
	l.isProgress = false
}

// LogProgress writes a progress message.
// TTY: overwrites current line with carriage return.
// Agent: NDJSON with level "progress" and deduplication.
func (l *Logger) LogProgress(line string) {
	if l.isTTY {
		fmt.Fprintf(l.writer, "\r\033[K%s", line)
		return
	}
	if line == l.lastLine && l.isProgress {
		l.dupCount++
		return
	}
	l.Flush()
	l.lastLine = line
	l.dupCount = 0
	l.isProgress = true
}

// Flush writes any deferred/deduplicated log entry.
func (l *Logger) Flush() {
	if l.isTTY || l.lastLine == "" {
		return
	}
	level := "info"
	if l.isProgress {
		level = "progress"
	}
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"level": level,
		"msg":   l.lastLine,
	}
	if l.dupCount > 0 {
		entry["repeated"] = l.dupCount
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintf(l.writer, "%s\n", data)
	l.lastLine = ""
	l.dupCount = 0
	l.isProgress = false
}
