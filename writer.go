package murli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// OutputFormat controls the serialization format of WriteSuccess.
type OutputFormat string

const (
	// OutputFormatDefault is unset — format is determined by TTY detection.
	OutputFormatDefault OutputFormat = ""
	// OutputFormatJSON writes a pretty-printed JSON envelope to stdout.
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatNDJSON writes a minified single-line JSON envelope to stdout.
	OutputFormatNDJSON OutputFormat = "ndjson"
	// OutputFormatYAML writes a YAML-encoded envelope to stdout.
	OutputFormatYAML OutputFormat = "yaml"
	// OutputFormatText writes human-readable plain text to stdout (same as TTY mode).
	OutputFormatText OutputFormat = "text"
)

// ValidOutputFormats lists the accepted --output values.
var ValidOutputFormats = []string{"json", "ndjson", "yaml", "text"}

// ValidProtocolVersions lists the accepted --protocol-version values.
var ValidProtocolVersions = []string{"0.1", "0.2"}

// WriterOption is a functional option for NewWriter.
type WriterOption func(*Writer)

// WithOutputFormat sets the explicit output format, overriding TTY auto-detection.
func WithOutputFormat(f OutputFormat) WriterOption {
	return func(w *Writer) {
		w.outputFormat = f
	}
}

// WithProtocolVersion sets the protocol version for envelope shaping.
// Valid values: "0.1", "0.2". Empty string defaults to "0.2" (current).
func WithProtocolVersion(v string) WriterOption {
	return func(w *Writer) {
		w.protocolVersion = v
	}
}

// WithForce sets the force flag, activating bypass of the non-interactive mutation guard.
// Set to true when --force or --yes is present on the command line.
func WithForce(force bool) WriterOption {
	return func(w *Writer) {
		w.force = force
	}
}

// WithDryRun sets the dry-run flag.
// Set to true when --dry-run is present on the command line.
func WithDryRun(dryRun bool) WriterOption {
	return func(w *Writer) {
		w.dryRun = dryRun
	}
}

// Writer handles dynamic output routing based on terminal presence, agent flags,
// explicit output format, and negotiated protocol version.
type Writer struct {
	mu              sync.Mutex
	stdout          io.Writer
	stderr          io.Writer
	isTTY           bool
	outputFormat    OutputFormat
	protocolVersion string
	force           bool // set via WithForce; backs --force/--yes bypass of the mutation guard
	dryRun          bool // set via WithDryRun; backs --dry-run flag
	logger          *Logger
}

// NewWriter returns a configured output writer.
// Set agentMode true to force JSON output regardless of TTY state.
// Pass WriterOption values to set OutputFormat or ProtocolVersion.
func NewWriter(stdout, stderr io.Writer, agentMode bool, opts ...WriterOption) *Writer {
	isTTY := isTerminal(stdout) && !agentMode
	w := &Writer{
		stdout: stdout,
		stderr: stderr,
		isTTY:  isTTY,
	}
	for _, opt := range opts {
		opt(w)
	}
	// Explicit format overrides isTTY routing.
	switch w.outputFormat {
	case OutputFormatText:
		w.isTTY = true
	case OutputFormatJSON, OutputFormatNDJSON, OutputFormatYAML:
		w.isTTY = false
	}
	w.logger = NewLogger(stderr, w.isTTY)
	return w
}

// IsTTY returns true if the writer is in human (TTY) mode.
func (w *Writer) IsTTY() bool {
	return w.isTTY
}

// IsForced returns true if --force or --yes was passed on the command line.
// Engineers may use this to suppress their own confirmation prompts in TTY mode.
// The non-interactive mutation guard bypass is automatic when IsForced is true.
func (w *Writer) IsForced() bool {
	return w.force
}

// IsDryRun returns true if --dry-run was passed on the command line.
// Engineers should check this at the start of their action and call WritePlan() if true.
func (w *Writer) IsDryRun() bool {
	return w.dryRun
}

// Format returns the explicit output format (may be OutputFormatDefault).
func (w *Writer) Format() OutputFormat {
	return w.outputFormat
}

// ProtocolVersion returns the negotiated protocol version (empty string = "0.2").
func (w *Writer) ProtocolVersion() string {
	if w.protocolVersion == "" {
		return "0.2"
	}
	return w.protocolVersion
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

// WriteSuccess writes to stdout. Format depends on outputFormat and isTTY:
//   - TTY or OutputFormatText: humanText plain line
//   - OutputFormatNDJSON: single minified JSON line
//   - OutputFormatYAML: YAML-encoded envelope
//   - OutputFormatJSON or default agent mode: pretty-printed JSON envelope
func (w *Writer) WriteSuccess(humanText string, jsonPayload any) {
	switch {
	case w.outputFormat == OutputFormatText || (w.outputFormat == OutputFormatDefault && w.isTTY):
		fmt.Fprintln(w.stdout, humanText)

	case w.outputFormat == OutputFormatNDJSON:
		envelope := w.buildSuccessEnvelope(jsonPayload)
		data, err := json.Marshal(envelope)
		if err != nil {
			return
		}
		fmt.Fprintf(w.stdout, "%s\n", data)

	case w.outputFormat == OutputFormatYAML:
		envelope := w.buildSuccessEnvelope(jsonPayload)
		enc := yaml.NewEncoder(w.stdout)
		enc.SetIndent(2)
		_ = enc.Encode(envelope)

	default: // OutputFormatJSON or default agent mode
		envelope := w.buildSuccessEnvelope(jsonPayload)
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(envelope)
	}
}

// buildSuccessEnvelope constructs the success envelope map,
// respecting the negotiated protocol version.
func (w *Writer) buildSuccessEnvelope(jsonPayload any) map[string]any {
	envelope := map[string]any{
		"status": "ok",
		"result": jsonPayload,
	}
	if w.ProtocolVersion() != "0.1" {
		envelope["schema_version"] = SchemaVersion
		if ToolVersion != "" {
			envelope["tool_version"] = ToolVersion
		}
	}
	return envelope
}

// WritePlan writes a dry-run plan to stdout. Format depends on outputFormat and isTTY:
//   - TTY or OutputFormatText: humanText plain line
//   - OutputFormatNDJSON: single minified JSON line with "status": "plan"
//   - OutputFormatYAML: YAML-encoded plan envelope
//   - OutputFormatJSON or default agent mode: pretty-printed JSON with "status": "plan"
func (w *Writer) WritePlan(humanText string, plan any) {
	switch {
	case w.outputFormat == OutputFormatText || (w.outputFormat == OutputFormatDefault && w.isTTY):
		fmt.Fprintln(w.stdout, humanText)

	case w.outputFormat == OutputFormatNDJSON:
		envelope := w.buildPlanEnvelope(plan)
		data, err := json.Marshal(envelope)
		if err != nil {
			return
		}
		fmt.Fprintf(w.stdout, "%s\n", data)

	case w.outputFormat == OutputFormatYAML:
		envelope := w.buildPlanEnvelope(plan)
		enc := yaml.NewEncoder(w.stdout)
		enc.SetIndent(2)
		_ = enc.Encode(envelope)

	default: // OutputFormatJSON or default agent mode
		envelope := w.buildPlanEnvelope(plan)
		enc := json.NewEncoder(w.stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(envelope)
	}
}

// buildPlanEnvelope constructs the plan envelope map,
// respecting the negotiated protocol version.
func (w *Writer) buildPlanEnvelope(plan any) map[string]any {
	envelope := map[string]any{
		"status": "plan",
		"result": plan,
	}
	if w.ProtocolVersion() != "0.1" {
		envelope["schema_version"] = SchemaVersion
		if ToolVersion != "" {
			envelope["tool_version"] = ToolVersion
		}
	}
	return envelope
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
// Agent mode: minified JSON on one line (consistent with WriteEvent).
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
