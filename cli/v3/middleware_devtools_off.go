//go:build !murlidev

package cli

import "github.com/urfave/cli/v3"

func mountDevTools(*cli.Command) {}
