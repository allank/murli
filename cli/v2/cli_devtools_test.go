//go:build murlidev

package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/allank/murli"
	murliCLI "github.com/allank/murli/cli/v2"
	"github.com/urfave/cli/v2"
)

func TestV2DoctorCommand(t *testing.T) {
	app := &cli.App{
		Name:  "riffle",
		Usage: "Riffle semantic search",
		Commands: []*cli.Command{
			{Name: "query", Usage: "Search", Action: func(ctx *cli.Context) error { return nil }},
		},
	}
	var buf bytes.Buffer
	app.Writer = &buf

	murliCLI.Wrap(app)

	err := app.Run([]string{"riffle", "doctor"})
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
