package cobra

import (
	"github.com/allank/murli"
	"github.com/spf13/cobra"
)

// NewWriter returns a murli Writer configured from a Cobra command's output streams
// and --agent flag state.
func NewWriter(cmd *cobra.Command) *murli.Writer {
	agentMode, _ := cmd.Flags().GetBool("agent")
	return murli.NewWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), agentMode)
}
