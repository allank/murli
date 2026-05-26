package cobra

import (
	"github.com/allank/murli"
	"github.com/spf13/cobra"
)

// NewWriter returns a murli Writer configured from a Cobra command's output streams,
// --agent, --output, and --protocol-version flag values.
func NewWriter(cmd *cobra.Command) *murli.Writer {
	agentMode, _ := cmd.Flags().GetBool("agent")
	outputFlag, _ := cmd.Flags().GetString("output")
	protocolFlag, _ := cmd.Flags().GetString("protocol-version")
	return murli.NewWriter(
		cmd.OutOrStdout(),
		cmd.ErrOrStderr(),
		agentMode,
		murli.WithOutputFormat(murli.OutputFormat(outputFlag)),
		murli.WithProtocolVersion(protocolFlag),
	)
}
