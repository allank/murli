//go:build murlidev

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/murli-cli/murli-go"
	murliCLI "github.com/murli-cli/murli-go/cli/v3"
	"github.com/urfave/cli/v3"
)

func TestV3DoctorCommand(t *testing.T) {
	app := &cli.Command{
		Name:  "riffle",
		Usage: "Riffle semantic search",
		Commands: []*cli.Command{
			{Name: "query", Usage: "Search", Action: func(ctx context.Context, c *cli.Command) error { return nil }},
		},
	}
	var buf bytes.Buffer
	app.Writer = &buf

	murliCLI.Wrap(app)

	err := app.Run(context.Background(), []string{"riffle", "doctor"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var env struct {
		Status string             `json:"status"`
		Result murli.DoctorReport `json:"result"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if env.Status != "ok" {
		t.Errorf("status: want %q, got %q", "ok", env.Status)
	}
	if len(env.Result.Checks) == 0 {
		t.Error("doctor result must have checks")
	}
}
