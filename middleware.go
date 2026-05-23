package murli

import (
	"github.com/spf13/cobra"
)

// Execute is a drop-in replacement for rootCmd.Execute() that automatically enables murli and handles top-level routing/argument/unknown command errors gracefully.
func Execute(rootCmd *cobra.Command) error {
	Enable(rootCmd)
	err := rootCmd.Execute()
	if err != nil {
		w := NewWriter(rootCmd)
		w.WriteError(&AgentError{
			Code:        ExitUserError,
			ErrorType:   "command_error",
			Message:     err.Error(),
			Suggestion:  "Check command usage with --schema or --help.",
			Recoverable: true,
		})
	}
	return err
}

// Enable injects persistent global flags and wraps all command Run/RunE handlers recursive.
func Enable(rootCmd *cobra.Command) {
	// Register persistent flags if not already present
	if rootCmd.PersistentFlags().Lookup("schema") == nil {
		rootCmd.PersistentFlags().Bool("schema", false, "Output agent-optimized JSON schema")
	}
	if rootCmd.PersistentFlags().Lookup("agent") == nil {
		rootCmd.PersistentFlags().Bool("agent", false, "Force agent-optimized JSON mode")
	}

	// Wrap execution tree recursively
	wrapCommands(rootCmd)
}

// wrapCommands intercepts execution, flag parsing, and standard errors recursively.
func wrapCommands(cmd *cobra.Command) {
	originalRunE := cmd.RunE
	originalRun := cmd.Run

	// Silence default Cobra formatting to prevent double-printing or raw output contamination in Agent mode
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	// Custom flag error interception
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		w := NewWriter(c)
		w.WriteError(&AgentError{
			Code:        ExitUserError,
			ErrorType:   "flag_error",
			Message:     err.Error(),
			Suggestion:  "Check command usage with --schema or --help.",
			Recoverable: true,
		})
		return nil
	})

	// Wrap positional arguments validator to bypass validation when querying the schema
	if cmd.Args != nil {
		originalArgsVal := cmd.Args
		cmd.Args = func(c *cobra.Command, args []string) error {
			schemaFlag, _ := c.Flags().GetBool("schema")
			if schemaFlag {
				return nil
			}
			return originalArgsVal(c, args)
		}
	}

	// Wrap execution handler
	cmd.RunE = func(c *cobra.Command, args []string) error {
		// Intercept schema queries immediately
		schemaFlag, _ := c.Flags().GetBool("schema")
		if schemaFlag {
			return EmitSchema(c)
		}

		// Perform native execution
		var runErr error
		if originalRunE != nil {
			runErr = originalRunE(c, args)
		} else if originalRun != nil {
			originalRun(c, args)
		} else {
			// If no execution runner is defined (like basic parent namespaces), trigger Help
			return c.Help()
		}

		// Clean up any remaining telemetry logs
		w := NewWriter(c)
		w.Flush()

		// Intercept execution errors
		if runErr != nil {
			if w.IsTTY() {
				return runErr
			}
			if agentErr, ok := runErr.(*AgentError); ok {
				w.WriteError(agentErr)
			} else {
				w.WriteError(&AgentError{
					Code:        ExitToolError,
					ErrorType:   "execution_error",
					Message:     runErr.Error(),
					Recoverable: false,
				})
			}
			return nil // Prevent Cobra from printing it again
		}

		return nil
	}

	// Recursively apply to all subcommands
	for _, child := range cmd.Commands() {
		wrapCommands(child)
	}
}
