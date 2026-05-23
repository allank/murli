package cli

import (
	"os"

	"github.com/allank/murli"
	"github.com/urfave/cli/v3"
)

// NewWriter returns a murli Writer configured from a urfave/cli v3 command.
func NewWriter(cmd *cli.Command) *murli.Writer {
	agentMode := cmd.Bool("agent")
	stdout := writerOrDefault(cmd.Root().Writer, os.Stdout)
	stderr := writerOrDefault(cmd.Root().ErrWriter, os.Stderr)
	return murli.NewWriter(stdout, stderr, agentMode)
}

