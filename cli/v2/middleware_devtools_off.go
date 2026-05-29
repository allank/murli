//go:build !murlidev

package cli

import "github.com/urfave/cli/v2"

func mountDevTools(*cli.App) {}
