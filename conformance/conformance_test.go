package conformance_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/allank/murli/conformance"
)

// TestConformanceSuiteAgainstDemo builds the demo binary and runs conformance checks.
// This validates the conformance package itself and the demo integration.
func TestConformanceSuiteAgainstDemo(t *testing.T) {
	binaryPath := buildDemoBinary(t)
	suite := conformance.NewSuite(binaryPath)
	suite.Check(t)
}

func buildDemoBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "riffle")
	cmd := exec.Command("go", "build", "-o", binaryPath, "github.com/allank/murli/demo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build demo binary: %v\n%s", err, out)
	}
	return binaryPath
}
