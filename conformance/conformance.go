// Package conformance provides a test suite that verifies a murli-enabled CLI binary
// satisfies the murli 1.0 contract. Import it in your CLI's test suite to get
// continuous contract compliance checks.
//
// Usage:
//
//	func TestConformance(t *testing.T) {
//	    suite := conformance.NewSuite("/path/to/your/binary")
//	    suite.Check(t)
//	}
package conformance

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

// Suite runs conformance checks against a murli-enabled CLI binary.
type Suite struct {
	// BinaryPath is the path to the CLI binary to test.
	BinaryPath string
}

// NewSuite creates a conformance test suite for the binary at path.
func NewSuite(binaryPath string) *Suite {
	return &Suite{BinaryPath: binaryPath}
}

// Check runs all generic conformance checks. Call from your TestConformance function.
func (s *Suite) Check(t *testing.T) {
	t.Helper()
	t.Run("describe", s.CheckDescribe)
}

// CheckDescribe verifies that `binary describe` returns valid JSON satisfying the
// murli DescribeOutput contract: schema_version, name, and capabilities are present.
func (s *Suite) CheckDescribe(t *testing.T) {
	t.Helper()
	stdout, _, _ := s.run("describe")

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("describe output is not valid JSON: %v\nraw: %s", err, stdout)
	}

	// schema_version must be present and non-empty.
	sv, _ := got["schema_version"].(string)
	if sv == "" {
		t.Error("describe output must have schema_version")
	}

	// name must be present.
	name, _ := got["name"].(string)
	if name == "" {
		t.Error("describe output must have name")
	}

	// capabilities must be a non-nil object.
	caps, ok := got["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("describe output must have a capabilities object")
	}

	// capabilities.output_formats must be a non-empty array.
	fmts, _ := caps["output_formats"].([]any)
	if len(fmts) == 0 {
		t.Error("capabilities.output_formats must be a non-empty array")
	}
}

// run executes the binary with args and returns stdout, stderr, and the exit code.
func (s *Suite) run(args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command(s.BinaryPath, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}
