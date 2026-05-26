package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/allank/murli"
	"github.com/urfave/cli/v3"
)

// Run enables murli on app and calls app.Run.
func Run(app *cli.Command, args []string) error {
	Wrap(app)
	if err := app.Run(context.Background(), args); err != nil {
		w := rootWriter(app)
		if agentErr, ok := err.(*murli.AgentError); ok {
			w.WriteError(agentErr)
		} else {
			w.WriteError(&murli.AgentError{
				Code:        murli.ExitUserError,
				ErrorType:   "command_error",
				Message:     err.Error(),
				Suggestion:  "Check command usage with --schema or --help.",
				Recoverable: true,
			})
		}
	}
	return nil
}

// Wrap injects --schema and --agent flags and wraps all command Actions.
func Wrap(app *cli.Command) {
	wrapCommands(app.Commands, app)

	// Naming convention advisory: collect names, emit warnings if TTY.
	if isTTYWriter(writerOrDefault(app.Writer, os.Stdout)) {
		var cmdNames, flagNames []string
		for _, cmd := range app.Commands {
			collectV3Names(cmd, &cmdNames, &flagNames)
		}
		murli.CheckConventions(cmdNames, flagNames, writerOrDefault(app.ErrWriter, os.Stderr))
	}
}

func wrapCommands(cmds []*cli.Command, root *cli.Command) {
	for _, cmd := range cmds {
		cmd.Flags = append(cmd.Flags,
			&cli.BoolFlag{Name: "schema", Usage: "Output agent-optimized JSON schema"},
			&cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"},
			&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|yaml|text"},
			&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version for envelope shaping (0.1|0.2)"},
		)

		originalAction := cmd.Action
		currentCmd := cmd

		cmd.Action = func(ctx context.Context, c *cli.Command) error {
			if c.Bool("schema") {
				out := writerOrDefault(root.Writer, os.Stdout)
				return EmitSchema(currentCmd, out)
			}

			w := NewWriter(c)

			// Protocol-version validation.
			if pv := c.String("protocol-version"); pv != "" {
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

			// Non-interactive guard.
			if meta := metadataFor(currentCmd); meta.Mutating && !w.IsTTY() {
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
			if originalAction != nil {
				runErr = originalAction(ctx, c)
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
			}
			return nil
		}

		if len(cmd.Commands) > 0 {
			wrapCommands(cmd.Commands, root)
		}
	}
}

// isTTYWriter reports whether w is a character device (TTY).
func isTTYWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, _ := f.Stat()
		return stat != nil && (stat.Mode()&os.ModeCharDevice) != 0
	}
	return false
}

// collectV3Names gathers all command names and flag names recursively from cmd.
func collectV3Names(cmd *cli.Command, cmds, flags *[]string) {
	*cmds = append(*cmds, cmd.Name)
	for _, f := range cmd.Flags {
		if names := f.Names(); len(names) > 0 {
			*flags = append(*flags, names[0])
		}
	}
	for _, sub := range cmd.Commands {
		collectV3Names(sub, cmds, flags)
	}
}

func rootWriter(app *cli.Command) *murli.Writer {
	return murli.NewWriter(
		writerOrDefault(app.Writer, os.Stdout),
		writerOrDefault(app.ErrWriter, os.Stderr),
		false,
	)
}

