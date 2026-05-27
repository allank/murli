package cli

import (
	"io"
	"os"

	"github.com/allank/murli"
	"github.com/urfave/cli/v2"
)

// NewWriter returns a murli Writer configured from a urfave/cli v2 context.
func NewWriter(ctx *cli.Context) *murli.Writer {
	agentMode := ctx.Bool("agent")
	outputFlag := ctx.String("output")
	protocolFlag := ctx.String("protocol-version")

	// Force flags — present only on Mutating commands; default false if not registered.
	forceFlag := ctx.Bool("force")
	yesFlag := ctx.Bool("yes")

	// Dry-run flag — present only on DryRunnable commands; default false if not registered.
	dryRunFlag := ctx.Bool("dry-run")

	stdout := writerOrDefault(ctx.App.Writer, os.Stdout)
	stderr := writerOrDefault(ctx.App.ErrWriter, os.Stderr)
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
