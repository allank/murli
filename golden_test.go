package murli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate golden files")

func TestGoldenSuccessEnvelope(t *testing.T) {
	orig := ToolVersion
	ToolVersion = ""
	defer func() { ToolVersion = orig }()

	buf := &bytes.Buffer{}
	w := NewWriter(buf, &bytes.Buffer{}, false)
	w.WriteSuccess("done", map[string]any{"key": "value", "count": 1})
	compareGolden(t, "success_envelope.json", buf.Bytes())
}

func TestGoldenPlanEnvelope(t *testing.T) {
	orig := ToolVersion
	ToolVersion = ""
	defer func() { ToolVersion = orig }()

	buf := &bytes.Buffer{}
	w := NewWriter(buf, &bytes.Buffer{}, false)
	w.WritePlan("Would delete 3 files", map[string]any{"files": []string{"a", "b", "c"}, "count": 3})
	compareGolden(t, "plan_envelope.json", buf.Bytes())
}

func TestGoldenErrorEnvelope(t *testing.T) {
	orig := ToolVersion
	origExit := ExitFunc
	ToolVersion = ""
	ExitFunc = func(int) {}
	defer func() {
		ToolVersion = orig
		ExitFunc = origExit
	}()

	buf := &bytes.Buffer{}
	w := NewWriter(&bytes.Buffer{}, buf, false)
	w.WriteError(&AgentError{
		Code:        ExitUserError,
		ErrorType:   "invalid_input",
		Message:     "the --region flag is required",
		Suggestion:  "Pass --region us-east-1",
		Recoverable: true,
	})
	compareGolden(t, "error_envelope.json", buf.Bytes())
}

// compareGolden compares got to the file at testdata/golden/<name>.
// With -update, it writes got to the file instead.
func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s missing — run with -update to generate: %v", name, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("golden mismatch for %s:\nwant:\n%s\ngot:\n%s", name, want, got)
	}
}
