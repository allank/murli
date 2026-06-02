package cli

import (
	"io"
	"os"

	"github.com/murli-cli/murli-go"
	"github.com/urfave/cli/v3"
)

// NewWriter returns a murli Writer configured from a urfave/cli v3 command.
func NewWriter(cmd *cli.Command) *murli.Writer {
	agentMode := cmd.Bool("agent")
	outputFlag := cmd.String("output")
	protocolFlag := cmd.String("protocol-version")

	// Force flags — present only on Mutating commands; default false if not registered.
	forceFlag := cmd.Bool("force")
	yesFlag := cmd.Bool("yes")

	// Dry-run flag — present only on DryRunnable commands; default false if not registered.
	dryRunFlag := cmd.Bool("dry-run")

	stdout := writerOrDefault(cmd.Root().Writer, os.Stdout)
	stderr := writerOrDefault(cmd.Root().ErrWriter, os.Stderr)
	return murli.NewWriter(
		stdout,
		stderr,
		agentMode,
		murli.WithOutputFormat(murli.OutputFormat(outputFlag)),
		murli.WithProtocolVersion(protocolFlag),
		murli.WithForce(forceFlag || yesFlag),
		murli.WithDryRun(dryRunFlag),
	)
}

func writerOrDefault(w io.Writer, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}
