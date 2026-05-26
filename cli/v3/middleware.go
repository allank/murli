package cli

import (
	"context"
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
}

func wrapCommands(cmds []*cli.Command, root *cli.Command) {
	for _, cmd := range cmds {
		cmd.Flags = append(cmd.Flags,
			&cli.BoolFlag{Name: "schema", Usage: "Output agent-optimized JSON schema"},
			&cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"},
		)

		originalAction := cmd.Action
		currentCmd := cmd

		cmd.Action = func(ctx context.Context, c *cli.Command) error {
			if c.Bool("schema") {
				out := writerOrDefault(root.Writer, os.Stdout)
				return EmitSchema(currentCmd, out)
			}

			w := NewWriter(c)

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

func rootWriter(app *cli.Command) *murli.Writer {
	return murli.NewWriter(
		writerOrDefault(app.Writer, os.Stdout),
		writerOrDefault(app.ErrWriter, os.Stderr),
		false,
	)
}

func writerOrDefault(w io.Writer, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}
