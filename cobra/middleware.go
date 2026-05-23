package cobra

import (
	"github.com/allank/murli"
	gocobra "github.com/spf13/cobra"
)

// Execute is a drop-in replacement for rootCmd.Execute() that enables murli and handles
// top-level errors.
func Execute(rootCmd *gocobra.Command) error {
	Enable(rootCmd)
	err := rootCmd.Execute()
	if err != nil {
		w := NewWriter(rootCmd)
		w.WriteError(&murli.AgentError{
			Code:        murli.ExitUserError,
			ErrorType:   "command_error",
			Message:     err.Error(),
			Suggestion:  "Check command usage with --schema or --help.",
			Recoverable: true,
		})
	}
	return err
}

// Enable injects --schema and --agent persistent flags and wraps all command RunE handlers.
func Enable(rootCmd *gocobra.Command) {
	if rootCmd.PersistentFlags().Lookup("schema") == nil {
		rootCmd.PersistentFlags().Bool("schema", false, "Output agent-optimized JSON schema")
	}
	if rootCmd.PersistentFlags().Lookup("agent") == nil {
		rootCmd.PersistentFlags().Bool("agent", false, "Force agent-optimized JSON mode")
	}
	wrapCommands(rootCmd)
}

func wrapCommands(cmd *gocobra.Command) {
	originalRunE := cmd.RunE
	originalRun := cmd.Run

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetFlagErrorFunc(func(c *gocobra.Command, err error) error {
		w := NewWriter(c)
		w.WriteError(&murli.AgentError{
			Code:        murli.ExitUserError,
			ErrorType:   "flag_error",
			Message:     err.Error(),
			Suggestion:  "Check command usage with --schema or --help.",
			Recoverable: true,
		})
		return nil
	})

	if cmd.Args != nil {
		orig := cmd.Args
		cmd.Args = func(c *gocobra.Command, args []string) error {
			if ok, _ := c.Flags().GetBool("schema"); ok {
				return nil
			}
			return orig(c, args)
		}
	}

	cmd.RunE = func(c *gocobra.Command, args []string) error {
		if ok, _ := c.Flags().GetBool("schema"); ok {
			return EmitSchema(c)
		}

		var runErr error
		if originalRunE != nil {
			runErr = originalRunE(c, args)
		} else if originalRun != nil {
			originalRun(c, args)
		} else {
			return c.Help()
		}

		w := NewWriter(c)
		w.Flush()

		if runErr != nil {
			if w.IsTTY() {
				return runErr
			}
			if agentErr, ok := runErr.(*murli.AgentError); ok {
				w.WriteError(agentErr)
			} else {
				w.WriteError(&murli.AgentError{
					Code:        murli.ExitToolError,
					ErrorType:   "execution_error",
					Message:     runErr.Error(),
					Recoverable: false,
				})
			}
			return nil
		}
		return nil
	}

	for _, child := range cmd.Commands() {
		wrapCommands(child)
	}
}
