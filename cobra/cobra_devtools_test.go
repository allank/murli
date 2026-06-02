//go:build murlidev

package cobra_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/murli-cli/murli-go"
	murliCobra "github.com/murli-cli/murli-go/cobra"
	"github.com/spf13/cobra"
)

func TestDoctorCommand(t *testing.T) {
	root := &cobra.Command{Use: "riffle", Short: "Riffle semantic search"}
	queryCmd := &cobra.Command{Use: "query <text>", Short: "Semantic query"}
	root.AddCommand(queryCmd)
	murliCobra.Annotate(queryCmd, murli.Metadata{AgentDescription: "Searches the index.", Idempotent: true})
	murliCobra.Enable(root)

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"doctor", "--agent"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var env struct {
		Status string             `json:"status"`
		Result murli.DoctorReport `json:"result"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal doctor output: %v\nraw: %s", err, buf.String())
	}
	if env.Status != "ok" {
		t.Errorf("status: want %q, got %q", "ok", env.Status)
	}
	if len(env.Result.Checks) == 0 {
		t.Error("doctor must return at least one check")
	}
	if env.Result.Failed > 0 {
		t.Errorf("doctor failed with %d failures:", env.Result.Failed)
		for _, c := range env.Result.Checks {
			if c.Status == "fail" {
				t.Logf("  FAIL %s: %s", c.Name, c.Message)
			}
		}
	}
}
