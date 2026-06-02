//go:build !murlidev

package cli_test

import (
	"testing"

	murliCLI "github.com/murli-cli/murli-go/cli/v2"
	"github.com/urfave/cli/v2"
)

func TestV2DoctorNotAutoMountedByDefault(t *testing.T) {
	app := &cli.App{Name: "riffle", Usage: "test"}
	murliCLI.Wrap(app)
	for _, c := range app.Commands {
		if c.Name == "doctor" {
			t.Fatal("doctor must not be auto-mounted without -tags murlidev")
		}
	}
}
