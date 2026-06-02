//go:build !murlidev

package cobra_test

import (
	"testing"

	murliCobra "github.com/murli-cli/murli-go/cobra"
	"github.com/spf13/cobra"
)

func TestDoctorNotAutoMountedByDefault(t *testing.T) {
	root := &cobra.Command{Use: "riffle", Short: "test"}
	murliCobra.Enable(root)
	for _, c := range root.Commands() {
		if c.Name() == "doctor" {
			t.Fatal("doctor must not be auto-mounted without -tags murlidev")
		}
	}
}
