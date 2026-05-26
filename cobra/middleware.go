package cobra

import (
	"encoding/json"
	"fmt"

	"github.com/allank/murli"
	gocobra "github.com/spf13/cobra"
)

// Execute is a drop-in replacement for rootCmd.Execute().
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
	if rootCmd.PersistentFlags().Lookup("output") == nil {
		rootCmd.PersistentFlags().String("output", "", "Output format: json|ndjson|yaml|text")
	}
	if rootCmd.PersistentFlags().Lookup("protocol-version") == nil {
		rootCmd.PersistentFlags().String("protocol-version", "", "Protocol version for envelope shaping (0.1|0.2)")
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

		w := NewWriter(c)

		// Protocol-version validation.
		if pv, _ := c.Flags().GetString("protocol-version"); pv != "" {
			valid := false
			for _, v := range murli.ValidProtocolVersions {
				if pv == v {
					valid = true
					break
				}
			}
			if !valid {
				w.WriteError(&murli.AgentError{
					Code:        murli.ExitUserError,
					ErrorType:   "invalid_protocol_version",
					Message:     fmt.Sprintf("unknown --protocol-version %q", pv),
					Suggestion:  "Use --protocol-version 0.1 or --protocol-version 0.2",
					Recoverable: true,
					ValidValues: murli.ValidProtocolVersions,
				})
				return nil
			}
		}

		// Non-interactive guard: mutating commands must not block waiting for input.
		if meta := cobraMetadata(c); meta.Mutating && !w.IsTTY() {
			w.WriteError(&murli.AgentError{
				Code:        murli.ExitUserError,
				ErrorType:   "confirmation_required",
				Message:     "This command mutates state and requires explicit confirmation.",
				Suggestion:  "Mutation requires confirmation. Use a TTY (interactive terminal) to run this command, or wait for --force support in a future release.",
				Recoverable: true,
			})
			return nil
		}

		var runErr error
		if originalRunE != nil {
			runErr = originalRunE(c, args)
		} else if originalRun != nil {
			originalRun(c, args)
		} else {
			return c.Help()
		}

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

// cobraMetadata extracts murli.Metadata from a command's annotations map.
func cobraMetadata(cmd *gocobra.Command) murli.Metadata {
	if cmd.Annotations == nil {
		return murli.Metadata{}
	}
	raw, ok := cmd.Annotations["agentcobra"]
	if !ok {
		return murli.Metadata{}
	}
	var meta murli.Metadata
	_ = json.Unmarshal([]byte(raw), &meta)
	return meta
}
