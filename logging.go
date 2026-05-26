package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

// ansiRe matches ANSI CSI escape sequences (e.g. \x1b[32m, \x1b[0m).
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI CSI escape sequences from s.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// Logger writes diagnostic messages to stderr.
// TTY mode: human-readable plain text.
// Agent mode: one NDJSON object per line {ts, level, msg}; consecutive duplicates
// are collapsed into a single entry with a "repeated" count.
type Logger struct {
	writer     io.Writer
	lastLine   string
	loggedAt   time.Time
	dupCount   int
	isTTY      bool
	isProgress bool
}

// NewLogger initializes a Logger writing to writer.
func NewLogger(writer io.Writer, isTTY bool) *Logger {
	return &Logger{writer: writer, isTTY: isTTY}
}

// Log writes a message. In TTY mode: plain text. In agent mode: NDJSON with deduplication.
// ANSI escape codes are stripped in agent mode.
func (l *Logger) Log(line string) {
	if l.isTTY {
		fmt.Fprintln(l.writer, line)
		return
	}
	// Strip ANSI codes in agent mode before deduplication and storage.
	line = stripANSI(line)
	if line == l.lastLine && !l.isProgress {
		l.dupCount++
		return
	}
	l.Flush()
	l.lastLine = line
	l.loggedAt = time.Now().UTC()
	l.dupCount = 0
	l.isProgress = false
}

// LogProgress writes a progress message.
// TTY: overwrites current line with carriage return.
// Agent: NDJSON with level "progress" and deduplication. ANSI codes stripped.
func (l *Logger) LogProgress(line string) {
	if l.isTTY {
		fmt.Fprintf(l.writer, "\r\033[K%s", line)
		return
	}
	// Strip ANSI codes in agent mode.
	line = stripANSI(line)
	if line == l.lastLine && l.isProgress {
		l.dupCount++
		return
	}
	l.Flush()
	l.lastLine = line
	l.loggedAt = time.Now().UTC()
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
		"ts":    l.loggedAt.Format(time.RFC3339Nano),
		"level": level,
		"msg":   l.lastLine,
	}
	if l.dupCount > 0 {
		entry["repeated"] = l.dupCount
	}
	enc := json.NewEncoder(l.writer)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(entry)
	l.lastLine = ""
	l.dupCount = 0
	l.isProgress = false
}
