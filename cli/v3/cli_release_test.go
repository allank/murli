//go:build !murlidev

package cli_test

import (
	"testing"

	murliCLI "github.com/allank/murli/cli/v3"
	"github.com/urfave/cli/v3"
)

func TestV3DoctorNotAutoMountedByDefault(t *testing.T) {
	app := &cli.Command{Name: "riffle", Usage: "test"}
	murliCLI.Wrap(app)
	for _, c := range app.Commands {
		if c.Name == "doctor" {
			t.Fatal("doctor must not be auto-mounted without -tags murlidev")
		}
	}
}
