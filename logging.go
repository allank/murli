package murli

import (
	"fmt"
	"io"
)

// Logger implements a deferred token-efficient logging mechanism.
type Logger struct {
	writer       io.Writer
	lastLine     string
	dupCount     int
	isTTY        bool
	isProgress   bool
}

// NewLogger initializes a new Logger writing to the specified writer.
func NewLogger(writer io.Writer, isTTY bool) *Logger {
	return &Logger{
		writer: writer,
		isTTY:  isTTY,
	}
}

// Log writes a message, deduplicating consecutive duplicates in Agent mode.
func (l *Logger) Log(line string) {
	if l.isTTY {
		// In TTY mode, print immediately
		fmt.Fprintln(l.writer, line)
		return
	}

	// In Agent mode, deduplicate consecutive identical lines
	if line == l.lastLine && !l.isProgress {
		l.dupCount++
		return
	}

	l.Flush()

	l.lastLine = line
	l.dupCount = 0
	l.isProgress = false
}

// LogProgress writes a progress update.
// In TTY mode, it overwrites the current line using standard terminal escape codes.
// In Agent mode, it filters duplicates to reduce token footprint.
func (l *Logger) LogProgress(line string) {
	if l.isTTY {
		// Clear current line and write with carriage return
		fmt.Fprintf(l.writer, "\r\033[K%s", line)
		return
	}

	// In Agent mode, filter consecutive identical progress messages
	if line == l.lastLine && l.isProgress {
		l.dupCount++
		return
	}

	l.Flush()

	l.lastLine = line
	l.dupCount = 0
	l.isProgress = true
}

// Flush writes any deferred deduplicated logs to the output stream.
func (l *Logger) Flush() {
	if l.isTTY || l.lastLine == "" {
		return
	}

	if l.dupCount > 0 {
		noun := "times"
		if l.dupCount == 1 {
			noun = "time"
		}
		if l.isProgress {
			fmt.Fprintf(l.writer, "%s (repeated %d %s, progress)\n", l.lastLine, l.dupCount, noun)
		} else {
			fmt.Fprintf(l.writer, "%s (repeated %d %s)\n", l.lastLine, l.dupCount, noun)
		}
	} else {
		fmt.Fprintln(l.writer, l.lastLine)
	}

	l.lastLine = ""
	l.dupCount = 0
	l.isProgress = false
}
