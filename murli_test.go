package murli

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
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

	t.Run("Agent mode omits tool_version when unset", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := &Writer{stdout: buf, isTTY: false}
		w.WriteSuccess("done", nil)

		var env map[string]any
		if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
			t.Fatalf("JSON parse: %v", err)
		}
		if _, present := env["tool_version"]; present {
			t.Errorf("tool_version must be absent when ToolVersion is empty, got: %v", env["tool_version"])
		}
		if env["schema_version"] != SchemaVersion {
			t.Errorf("schema_version must always be present, got: %v", env["schema_version"])
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
		if resp.SchemaVersion != SchemaVersion {
			t.Errorf("SchemaVersion: want %q, got %q", SchemaVersion, resp.SchemaVersion)
		}
		// ToolVersion is "" (default), so with omitempty it should NOT appear in JSON
		if resp.ToolVersion != "" {
			t.Errorf("ToolVersion: want empty, got %q", resp.ToolVersion)
		}
	})
}

func TestExtendedExitCodes(t *testing.T) {
	if ExitTimeout != 4 {
		t.Errorf("ExitTimeout: want 4, got %d", ExitTimeout)
	}
	if ExitNotFound != 5 {
		t.Errorf("ExitNotFound: want 5, got %d", ExitNotFound)
	}
	if ExitPermission != 6 {
		t.Errorf("ExitPermission: want 6, got %d", ExitPermission)
	}
	if ExitConflict != 7 {
		t.Errorf("ExitConflict: want 7, got %d", ExitConflict)
	}
	if ExitRateLimited != 8 {
		t.Errorf("ExitRateLimited: want 8, got %d", ExitRateLimited)
	}
	if ExitCancelled != 9 {
		t.Errorf("ExitCancelled: want 9, got %d", ExitCancelled)
	}
}

func TestNewUserError(t *testing.T) {
	err := NewUserError("invalid value", "provide a number between 1 and 10")
	if err.Code != ExitUserError {
		t.Errorf("Code: want %d, got %d", ExitUserError, err.Code)
	}
	if err.Message != "invalid value" {
		t.Errorf("Message: want %q, got %q", "invalid value", err.Message)
	}
	if err.Suggestion != "provide a number between 1 and 10" {
		t.Errorf("Suggestion: want %q, got %q", "provide a number between 1 and 10", err.Suggestion)
	}
	if !err.Recoverable {
		t.Error("NewUserError must be Recoverable = true")
	}
	if err.ErrorType == "" {
		t.Error("NewUserError must set ErrorType")
	}
}

func TestNewToolError(t *testing.T) {
	err := NewToolError("database connection refused")
	if err.Code != ExitToolError {
		t.Errorf("Code: want %d, got %d", ExitToolError, err.Code)
	}
	if err.Message != "database connection refused" {
		t.Errorf("Message: want %q, got %q", "database connection refused", err.Message)
	}
	if err.Recoverable {
		t.Error("NewToolError must be Recoverable = false")
	}
	if err.ErrorType == "" {
		t.Error("NewToolError must set ErrorType")
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
		if !strings.Contains(got, "\r\033[KTask A: 10%") || !strings.Contains(got, "\r\033[KTask A: 20%") {
			t.Errorf("missing TTY overwrite sequences: %q", got)
		}
	})

	t.Run("Agent mode log deduplication emits NDJSON", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger(buf, false)
		l.Log("Hello")
		l.Log("Hello")
		l.Log("Hello")
		l.Log("World")
		l.Flush()

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 log lines, got %d:\n%s", len(lines), buf.String())
		}

		var first, second map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
			t.Fatalf("line 0 not valid JSON: %v — %q", err, lines[0])
		}
		if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
			t.Fatalf("line 1 not valid JSON: %v — %q", err, lines[1])
		}
		if first["msg"] != "Hello" {
			t.Errorf("first msg: want %q, got %v", "Hello", first["msg"])
		}
		rep, ok := first["repeated"].(float64)
		if !ok {
			t.Fatal("repeated field missing or wrong type in first entry")
		}
		if rep != 2 {
			t.Errorf("repeated: want 2, got %v", rep)
		}
		if first["level"] != "info" {
			t.Errorf("level: want %q, got %v", "info", first["level"])
		}
		if second["msg"] != "World" {
			t.Errorf("second msg: want %q, got %v", "World", second["msg"])
		}
	})

	t.Run("Agent mode progress deduplication emits NDJSON", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := NewLogger(buf, false)
		l.LogProgress("Loading config")
		l.LogProgress("Loading config")
		l.LogProgress("Connecting db")
		l.Flush()

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 log lines, got %d:\n%s", len(lines), buf.String())
		}

		var first map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
			t.Fatalf("line 0 not valid JSON: %v — %q", err, lines[0])
		}
		if first["msg"] != "Loading config" {
			t.Errorf("first msg: want %q, got %v", "Loading config", first["msg"])
		}
		rep, ok := first["repeated"].(float64)
		if !ok {
			t.Fatal("repeated field missing or wrong type in first entry")
		}
		if rep != 1 {
			t.Errorf("repeated: want 1, got %v", rep)
		}
		if first["level"] != "progress" {
			t.Errorf("level: want %q, got %v", "progress", first["level"])
		}
		var second map[string]any
		if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
			t.Fatalf("line 1 not valid JSON: %v — %q", err, lines[1])
		}
		if second["msg"] != "Connecting db" {
			t.Errorf("second msg: want %q, got %v", "Connecting db", second["msg"])
		}
		if second["level"] != "progress" {
			t.Errorf("second level: want %q, got %v", "progress", second["level"])
		}
		if _, present := second["repeated"]; present {
			t.Error("second entry must not have a 'repeated' field")
		}
	})
}

func TestLoggerTimestampFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	l := NewLogger(buf, false)
	l.Log("hello world")
	l.Flush()

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("not valid JSON: %v — %q", err, buf.String())
	}
	ts, ok := entry["ts"].(string)
	if !ok || ts == "" {
		t.Errorf("ts field missing or not a string: %v", entry["ts"])
	}
	_, err1 := time.Parse(time.RFC3339, ts)
	_, err2 := time.Parse(time.RFC3339Nano, ts)
	if err1 != nil && err2 != nil {
		t.Errorf("ts %q is not RFC3339 or RFC3339Nano: %v / %v", ts, err1, err2)
	}
}

func TestVersionInSuccessEnvelope(t *testing.T) {
	ToolVersion = "1.2.3"
	defer func() { ToolVersion = "" }()

	buf := &bytes.Buffer{}
	w := &Writer{stdout: buf, isTTY: false}
	w.WriteSuccess("done", map[string]any{"key": "val"})

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, buf.String())
	}
	if resp["schema_version"] != SchemaVersion {
		t.Errorf("schema_version: want %q, got %v", SchemaVersion, resp["schema_version"])
	}
	if resp["tool_version"] != "1.2.3" {
		t.Errorf("tool_version: want %q, got %v", "1.2.3", resp["tool_version"])
	}
}

func TestVersionInErrorEnvelope(t *testing.T) {
	ToolVersion = "2.0.0"
	defer func() { ToolVersion = "" }()

	var capturedCode int
	ExitFunc = func(code int) { capturedCode = code }
	defer func() { ExitFunc = func(code int) {} }()

	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: false}
	w.WriteError(NewToolError("disk full"))

	if capturedCode != ExitToolError {
		t.Errorf("exit code: want %d, got %d", ExitToolError, capturedCode)
	}
	var got AgentError
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, buf.String())
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version: want %q, got %q", SchemaVersion, got.SchemaVersion)
	}
	if got.ToolVersion != "2.0.0" {
		t.Errorf("tool_version: want %q, got %q", "2.0.0", got.ToolVersion)
	}
}

func TestWriteEvent(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &Writer{stdout: buf, isTTY: false}

	w.WriteEvent(map[string]any{"stage": "processing", "item": 1})
	w.WriteEvent(map[string]any{"stage": "processing", "item": 2})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 event lines, got %d:\n%s", len(lines), buf.String())
	}

	var evt map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &evt); err != nil {
		t.Fatalf("line 0 not valid JSON: %v — %q", err, lines[0])
	}
	if evt["stage"] != "processing" {
		t.Errorf("stage: want %q, got %v", "processing", evt["stage"])
	}
	if evt["item"].(float64) != 1 {
		t.Errorf("item: want 1, got %v", evt["item"])
	}
}

func TestWriteEventIsMinified(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &Writer{stdout: buf, isTTY: false}
	w.WriteEvent(map[string]any{"k": "v"})

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, "\n") {
		t.Error("WriteEvent output must be a single line (minified JSON)")
	}
	if strings.Contains(line, "  ") {
		t.Error("WriteEvent output must not contain indentation")
	}
}

func TestWriteEventConcurrentSafe(t *testing.T) {
	buf := &safeBuffer{}
	w := &Writer{stdout: buf, isTTY: false}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			w.WriteEvent(map[string]any{"n": n})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 event lines, got %d", len(lines))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d not valid JSON: %v — %q", i, err, line)
		}
	}
}

// safeBuffer is a bytes.Buffer protected by a mutex for concurrent test writes.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *safeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

func TestWriteProgressAgentMode(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: false}

	w.WriteProgress(ProgressEvent{
		Stage:   "indexing",
		Current: 42,
		Total:   100,
		Percent: 42.0,
		EtaMs:   3000,
		Message: "Indexing documents...",
	})

	line := strings.TrimSpace(buf.String())
	var evt map[string]any
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		t.Fatalf("not valid JSON: %v — %q", err, line)
	}
	if evt["stage"] != "indexing" {
		t.Errorf("stage: want %q, got %v", "indexing", evt["stage"])
	}
	if evt["current"].(float64) != 42 {
		t.Errorf("current: want 42, got %v", evt["current"])
	}
	if evt["total"].(float64) != 100 {
		t.Errorf("total: want 100, got %v", evt["total"])
	}
	if evt["percent"].(float64) != 42.0 {
		t.Errorf("percent: want 42.0, got %v", evt["percent"])
	}
	if evt["eta_ms"].(float64) != 3000 {
		t.Errorf("eta_ms: want 3000, got %v", evt["eta_ms"])
	}
	if evt["message"] != "Indexing documents..." {
		t.Errorf("message: want %q, got %v", "Indexing documents...", evt["message"])
	}
}

func TestWriteProgressTTYMode(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: true}

	w.WriteProgress(ProgressEvent{
		Stage:   "building",
		Current: 5,
		Total:   10,
		Message: "Compiling sources",
	})

	got := buf.String()
	if !strings.Contains(got, "Compiling sources") {
		t.Errorf("TTY progress missing message: %q", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Error("TTY progress must not emit JSON")
	}
}

func TestExampleType(t *testing.T) {
	ex := Example{
		Command:          "riffle query woodworking",
		Description:      "Find folders matching the woodworking topic",
		ExpectedExitCode: 0,
	}
	data, err := json.Marshal(ex)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if got["command"] != "riffle query woodworking" {
		t.Errorf("command: %v", got["command"])
	}
	if got["description"] != "Find folders matching the woodworking topic" {
		t.Errorf("description: %v", got["description"])
	}
	// expected_exit_code 0 is omitted (omitempty)
	if _, present := got["expected_exit_code"]; present {
		t.Errorf("expected_exit_code should be omitted when zero")
	}
}

func TestFlagAnnotationType(t *testing.T) {
	ann := FlagAnnotation{
		Env:                   "RIFFLE_TOP",
		Sensitive:             false,
		Persistent:            true,
		MutuallyExclusiveWith: []string{"all"},
		Enum:                  []string{"5", "10", "20"},
		Pattern:               `^\d+$`,
	}
	data, err := json.Marshal(ann)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if got["env"] != "RIFFLE_TOP" {
		t.Errorf("env: %v", got["env"])
	}
	if got["persistent"] != true {
		t.Errorf("persistent: %v", got["persistent"])
	}
	enums, ok := got["enum"].([]any)
	if !ok || len(enums) != 3 {
		t.Errorf("enum: %v", got["enum"])
	}
}

func TestApplyFlagAnnotation(t *testing.T) {
	fs := FlagSchema{Name: "top", Type: "int", Default: 5, Description: "Max results"}
	ann := FlagAnnotation{
		Env:                   "RIFFLE_TOP",
		Persistent:            true,
		MutuallyExclusiveWith: []string{"all"},
		Enum:                  []string{"5", "10"},
		Pattern:               `^\d+$`,
	}
	ApplyFlagAnnotation(&fs, ann)
	if fs.Env != "RIFFLE_TOP" {
		t.Errorf("Env: %q", fs.Env)
	}
	if !fs.Persistent {
		t.Error("Persistent should be true")
	}
	if len(fs.MutuallyExclusiveWith) != 1 || fs.MutuallyExclusiveWith[0] != "all" {
		t.Errorf("MutuallyExclusiveWith: %v", fs.MutuallyExclusiveWith)
	}
	if len(fs.Enum) != 2 {
		t.Errorf("Enum: %v", fs.Enum)
	}
	if fs.Pattern != `^\d+$` {
		t.Errorf("Pattern: %q", fs.Pattern)
	}
}

func TestApplyFlagAnnotationDeepCopy(t *testing.T) {
	fs := FlagSchema{Name: "x"}
	enum := []string{"a", "b"}
	ann := FlagAnnotation{Enum: enum}
	ApplyFlagAnnotation(&fs, ann)
	// Mutate original — must not affect fs.Enum
	enum[0] = "CHANGED"
	if fs.Enum[0] != "a" {
		t.Errorf("Enum deep copy violated: fs.Enum[0] = %q", fs.Enum[0])
	}
}

func TestDescribeOutputTypes(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		Summary:       "Riffle semantic search",
		SchemaVersion: "0.2",
		Capabilities: Capabilities{
			Streaming:     true,
			DryRun:        false,
			OutputFormats: []string{"json", "ndjson", "yaml", "text"},
			SchemaVersion: "0.2",
		},
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if got["name"] != "riffle" {
		t.Errorf("name: %v", got["name"])
	}
	caps, ok := got["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities missing")
	}
	if caps["streaming"] != true {
		t.Errorf("streaming: %v", caps["streaming"])
	}
	formats, ok := caps["output_formats"].([]any)
	if !ok || len(formats) != 4 {
		t.Errorf("output_formats: %v", caps["output_formats"])
	}
}

func TestDefaultCapabilities(t *testing.T) {
	caps := DefaultCapabilities()
	if !caps.Streaming {
		t.Error("Streaming should be true")
	}
	if caps.DryRun {
		t.Error("DryRun should be false")
	}
	if len(caps.OutputFormats) != 4 {
		t.Errorf("OutputFormats: %v", caps.OutputFormats)
	}
	if caps.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion: %q", caps.SchemaVersion)
	}
}

func TestReturnSchemaOutputSchema(t *testing.T) {
	rs := ReturnSchema{
		Type:         "json",
		Description:  "A result",
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}}}`),
	}
	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if got["output_schema"] == nil {
		t.Error("output_schema must be present")
	}
}

func TestAgentErrorExtendedFields(t *testing.T) {
	var capturedCode int
	ExitFunc = func(code int) { capturedCode = code }
	defer func() { ExitFunc = func(code int) {} }()

	buf := &bytes.Buffer{}
	w := &Writer{stderr: buf, isTTY: false}

	err := &AgentError{
		Code:         ExitRateLimited,
		ErrorType:    "rate_limited",
		Message:      "API quota exceeded",
		Suggestion:   "Wait and retry.",
		Recoverable:  true,
		RetryAfterMs: 5000,
		DocURL:       "https://example.com/rate-limits",
		ValidValues:  []string{"us-east-1", "us-west-2"},
		Field:        "region",
	}

	w.WriteError(err)

	if capturedCode != ExitRateLimited {
		t.Errorf("exit code: want %d, got %d", ExitRateLimited, capturedCode)
	}

	var got AgentError
	if parseErr := json.Unmarshal(buf.Bytes(), &got); parseErr != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", parseErr, buf.String())
	}
	if got.RetryAfterMs != 5000 {
		t.Errorf("RetryAfterMs: want 5000, got %d", got.RetryAfterMs)
	}
	if got.DocURL != "https://example.com/rate-limits" {
		t.Errorf("DocURL: want %q, got %q", "https://example.com/rate-limits", got.DocURL)
	}
	if len(got.ValidValues) != 2 || got.ValidValues[0] != "us-east-1" {
		t.Errorf("ValidValues: want [us-east-1 us-west-2], got %v", got.ValidValues)
	}
	if got.Field != "region" {
		t.Errorf("Field: want %q, got %q", "region", got.Field)
	}
}
