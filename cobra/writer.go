package cobra

import (
	"github.com/murli-cli/murli-go"
	"github.com/spf13/cobra"
)

// NewWriter returns a murli Writer configured from a Cobra command's output streams,
// --agent, --output, --protocol-version, --force, --yes, and --dry-run flag values.
func NewWriter(cmd *cobra.Command) *murli.Writer {
	agentMode, _ := cmd.Flags().GetBool("agent")
	outputFlag, _ := cmd.Flags().GetString("output")
	protocolFlag, _ := cmd.Flags().GetString("protocol-version")

	// Force flags — present only on Mutating commands.
	forceFlag, _ := cmd.Flags().GetBool("force")
	yesFlag, _ := cmd.Flags().GetBool("yes")

	// Dry-run flag — present only on DryRunnable commands.
	dryRunFlag, _ := cmd.Flags().GetBool("dry-run")

	return murli.NewWriter(
		cmd.OutOrStdout(),
		cmd.ErrOrStderr(),
		agentMode,
		murli.WithOutputFormat(murli.OutputFormat(outputFlag)),
		murli.WithProtocolVersion(protocolFlag),
		murli.WithForce(forceFlag || yesFlag),
		murli.WithDryRun(dryRunFlag),
	)
}
