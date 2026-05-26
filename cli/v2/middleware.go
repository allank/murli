package cli

import (
	"fmt"
	"os"

	"github.com/allank/murli"
	"github.com/urfave/cli/v2"
)

// Run enables murli on app and calls app.Run(args).
func Run(app *cli.App, args []string) error {
	Wrap(app)
	if err := app.Run(args); err != nil {
		w := appWriter(app)
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
func Wrap(app *cli.App) {
	wrapCommands(app.Commands, app)
}

func wrapCommands(cmds []*cli.Command, app *cli.App) {
	for _, cmd := range cmds {
		cmd.Flags = append(cmd.Flags,
			&cli.BoolFlag{Name: "schema", Usage: "Output agent-optimized JSON schema"},
			&cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"},
			&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|yaml|text"},
			&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version for envelope shaping (0.1|0.2)"},
		)

		originalAction := cmd.Action
		currentCmd := cmd

		cmd.Action = func(ctx *cli.Context) error {
			if ctx.Bool("schema") {
				out := writerOrDefault(ctx.App.Writer, os.Stdout)
				return EmitSchema(currentCmd, out)
			}

			w := NewWriter(ctx)

			// Protocol-version validation.
			if pv := ctx.String("protocol-version"); pv != "" {
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
				runErr = originalAction(ctx)
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

		if len(cmd.Subcommands) > 0 {
			wrapCommands(cmd.Subcommands, app)
		}
	}
}

func appWriter(app *cli.App) *murli.Writer {
	stdout := writerOrDefault(app.Writer, os.Stdout)
	stderr := writerOrDefault(app.ErrWriter, os.Stderr)
	return murli.NewWriter(stdout, stderr, false)
}
