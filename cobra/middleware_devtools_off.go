//go:build !murlidev

package cobra

import gocobra "github.com/spf13/cobra"

func mountDevTools(*gocobra.Command) {}
