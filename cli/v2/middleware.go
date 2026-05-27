package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

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
	// Register app-level murli flags if not already present.
	appFlagNames := map[string]bool{}
	for _, f := range app.Flags {
		if names := f.Names(); len(names) > 0 {
			appFlagNames[names[0]] = true
		}
	}
	if !appFlagNames["profile"] {
		app.Flags = append(app.Flags, &cli.StringFlag{Name: "profile", Usage: "Profile name to use for this invocation"})
	}
	if !appFlagNames["agent"] {
		app.Flags = append(app.Flags, &cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"})
	}

	wrapCommands(app.Commands, app)

	// Naming convention advisory: collect names, emit warnings if TTY.
	if isTTYWriter(writerOrDefault(app.Writer, os.Stdout)) {
		var cmdNames, flagNames []string
		for _, cmd := range app.Commands {
			collectV2Names(cmd, &cmdNames, &flagNames)
		}
		murli.CheckConventions(cmdNames, flagNames, writerOrDefault(app.ErrWriter, os.Stderr))
	}

	// Auto-mount describe command if not already present.
	for _, c := range app.Commands {
		if c.Name == "describe" {
			return // already mounted
		}
	}
	describeV2 := &cli.Command{
		Name:  "describe",
		Usage: "Print the full command tree and capabilities as a single JSON document",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|yaml|text"},
			&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version (0.1|0.2)"},
		},
		Action: func(ctx *cli.Context) error {
			stdout := writerOrDefault(ctx.App.Writer, os.Stdout)

			appStore, _ := murli.LoadProfileStore(ctx.App.Name)
			rootMeta := appMetadataFor(ctx.App)
			profileableNames := []string{}
			for flagName, ann := range rootMeta.FlagAnnotations {
				if ann.Profileable {
					profileableNames = append(profileableNames, flagName)
				}
			}
			sort.Strings(profileableNames)
			profilesInfo := &murli.ProfilesInfo{ProfileableFlags: profileableNames}
			if appStore != nil && len(appStore.Names()) > 0 {
				profilesInfo.Available = appStore.Names()
				profilesInfo.Default = appStore.Default
			}

			out := murli.DescribeOutput{
				Name:          ctx.App.Name,
				Summary:       ctx.App.Usage,
				SchemaVersion: murli.SchemaVersion,
				ToolVersion:   murli.ToolVersion,
				Capabilities:  murli.DefaultCapabilities(),
				Conventions:   murli.ConventionalVocabulary(),
				Profiles:      profilesInfo,
			}
			for _, cmd := range app.Commands {
				if cmd.Hidden || cmd.Name == "describe" {
					continue
				}
				out.Commands = append(out.Commands, BuildV2DescribeTree(cmd))
			}
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			_ = enc.Encode(out)
			return nil
		},
	}
	app.Commands = append(app.Commands, describeV2)

	// Auto-mount profile subcommand group if not already present.
	for _, c := range app.Commands {
		if c.Name == "profile" {
			return
		}
	}
	app.Commands = append(app.Commands, buildV2ProfileGroup(app))
}

func wrapCommands(cmds []*cli.Command, app *cli.App) {
	for _, cmd := range cmds {
		// Skip commands already wrapped by murli.
		alreadyWrapped := false
		for _, f := range cmd.Flags {
			if names := f.Names(); len(names) > 0 && names[0] == "schema" {
				alreadyWrapped = true
				break
			}
		}
		if alreadyWrapped {
			if len(cmd.Subcommands) > 0 {
				wrapCommands(cmd.Subcommands, app)
			}
			continue
		}

		cmd.Flags = append(cmd.Flags,
			&cli.BoolFlag{Name: "schema", Usage: "Output agent-optimized JSON schema"},
			&cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"},
			&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|yaml|text"},
			&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version for envelope shaping (0.1|0.2)"},
		)

		// Register --force and --yes on mutating commands.
		meta := metadataFor(cmd)
		if meta.Mutating {
			cmd.Flags = append(cmd.Flags,
				&cli.BoolFlag{Name: "force", Usage: "Bypass the non-interactive mutation guard"},
				&cli.BoolFlag{Name: "yes", Usage: "Bypass the non-interactive mutation guard"},
			)
		}
		// Register --dry-run on DryRunnable commands.
		if meta.DryRunnable {
			cmd.Flags = append(cmd.Flags,
				&cli.BoolFlag{Name: "dry-run", Usage: "Preview the operation without executing it"},
			)
		}

		originalAction := cmd.Action
		currentCmd := cmd

		cmd.Action = func(ctx *cli.Context) error {
			applyV2Profile(ctx) // apply stored profile values before anything else

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

			// Output format validation.
			if outFmt := ctx.String("output"); outFmt != "" {
				valid := false
				for _, v := range murli.ValidOutputFormats {
					if outFmt == v {
						valid = true
						break
					}
				}
				if !valid {
					w.WriteError(&murli.AgentError{
						Code:        murli.ExitUserError,
						ErrorType:   "invalid_output_format",
						Message:     fmt.Sprintf("unknown --output value %q", outFmt),
						Suggestion:  "Use --output json, ndjson, yaml, or text",
						Recoverable: true,
						ValidValues: murli.ValidOutputFormats,
					})
					return nil
				}
			}

			// Non-interactive guard.
			if meta := metadataFor(currentCmd); meta.Mutating && !w.IsTTY() && !w.IsForced() {
				w.WriteError(&murli.AgentError{
					Code:        murli.ExitUserError,
					ErrorType:   "confirmation_required",
					Message:     "This command mutates state and requires explicit confirmation.",
					Suggestion:  "Pass --force or --yes to proceed without a TTY.",
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
				} else if errors.Is(runErr, context.Canceled) {
					w.WriteError(&murli.AgentError{
						Code:        murli.ExitCancelled,
						ErrorType:   "cancelled",
						Message:     runErr.Error(),
						Recoverable: false,
					})
				} else if errors.Is(runErr, context.DeadlineExceeded) {
					w.WriteError(&murli.AgentError{
						Code:        murli.ExitTimeout,
						ErrorType:   "timeout",
						Message:     runErr.Error(),
						Recoverable: true,
					})
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

// isTTYWriter reports whether w is a character device (TTY).
func isTTYWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, _ := f.Stat()
		return stat != nil && (stat.Mode()&os.ModeCharDevice) != 0
	}
	return false
}

// collectV2Names gathers all command names and flag names recursively from cmd.
func collectV2Names(cmd *cli.Command, cmds, flags *[]string) {
	*cmds = append(*cmds, cmd.Name)
	for _, f := range cmd.Flags {
		if names := f.Names(); len(names) > 0 {
			*flags = append(*flags, names[0])
		}
	}
	for _, sub := range cmd.Subcommands {
		collectV2Names(sub, cmds, flags)
	}
}

func appWriter(app *cli.App) *murli.Writer {
	stdout := writerOrDefault(app.Writer, os.Stdout)
	stderr := writerOrDefault(app.ErrWriter, os.Stderr)
	return murli.NewWriter(stdout, stderr, false)
}

// applyV2Profile reads the active profile and applies stored flag values to root
// context flags that were not explicitly set. If --profile was explicitly set but
// the profile does not exist, a not_found error is written.
func applyV2Profile(ctx *cli.Context) {
	// Find the app-level root context: the last context in the lineage that has
	// a non-nil App. urfave/cli v2 appends a phantom parentContext with no App
	// as the final element, so we must skip it.
	lineage := ctx.Lineage()
	var rootCtx *cli.Context
	for i := len(lineage) - 1; i >= 0; i-- {
		if lineage[i].App != nil {
			rootCtx = lineage[i]
			break
		}
	}
	if rootCtx == nil {
		return
	}
	store, err := murli.LoadProfileStore(ctx.App.Name)
	if err != nil {
		return
	}
	explicitProfile := rootCtx.String("profile")
	profileName := explicitProfile
	if profileName == "" {
		profileName = store.Default
	}
	if profileName == "" {
		return
	}
	profile, ok := store.Get(profileName)
	if !ok {
		if explicitProfile != "" {
			w := NewWriter(ctx)
			w.WriteError(&murli.AgentError{
				Code:        murli.ExitNotFound,
				ErrorType:   "not_found",
				Message:     fmt.Sprintf("profile %q not found", explicitProfile),
				Suggestion:  "Run 'profile list' to see available profiles.",
				Recoverable: false,
			})
		}
		return
	}
	for flagName, value := range profile.Flags {
		if !rootCtx.IsSet(flagName) {
			_ = rootCtx.Set(flagName, value)
		}
	}
}

// v2FlagStringValue returns the string representation of a named app flag's current value.
func v2FlagStringValue(rootCtx *cli.Context, app *cli.App, name string) string {
	for _, f := range app.Flags {
		names := f.Names()
		if len(names) == 0 || names[0] != name {
			continue
		}
		switch f.(type) {
		case *cli.BoolFlag:
			return strconv.FormatBool(rootCtx.Bool(name))
		case *cli.IntFlag:
			return strconv.Itoa(rootCtx.Int(name))
		case *cli.Float64Flag:
			return strconv.FormatFloat(rootCtx.Float64(name), 'f', -1, 64)
		default:
			return rootCtx.String(name)
		}
	}
	return rootCtx.String(name)
}

// buildV2ProfileGroup builds the `profile` command group for urfave/cli v2.
func buildV2ProfileGroup(app *cli.App) *cli.Command {
	profileCmd := &cli.Command{
		Name:  "profile",
		Usage: "Manage saved flag profiles",
		Subcommands: []*cli.Command{
			{
				Name:      "save",
				Usage:     "Save current profileable flags as a named profile",
				ArgsUsage: "<name>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() < 1 {
						return fmt.Errorf("profile save requires a name argument")
					}
					name := ctx.Args().First()
					w := NewWriter(ctx)
					lineage := ctx.Lineage()
					var rootCtx *cli.Context
					for i := len(lineage) - 1; i >= 0; i-- {
						if lineage[i].App != nil {
							rootCtx = lineage[i]
							break
						}
					}
					if rootCtx == nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "could not find root context"})
						return nil
					}
					rootMeta := appMetadataFor(app)
					flags := make(map[string]string)
					for flagName, ann := range rootMeta.FlagAnnotations {
						if ann.Profileable && rootCtx.IsSet(flagName) {
							flags[flagName] = v2FlagStringValue(rootCtx, app, flagName)
						}
					}
					if len(flags) == 0 {
						w.WriteError(&murli.AgentError{
							Code:        murli.ExitUserError,
							ErrorType:   "user_error",
							Message:     "no profileable flags were set",
							Suggestion:  "Pass at least one profileable flag when running profile save. Use --schema to see which flags are profileable.",
							Recoverable: true,
						})
						return nil
					}
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to load profile store: " + err.Error()})
						return nil
					}
					store.Set(name, murli.Profile{Flags: flags})
					if err := store.Save(app.Name); err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to save profile store: " + err.Error()})
						return nil
					}
					w.WriteSuccess(
						fmt.Sprintf("Profile %q saved.", name),
						map[string]any{"profile": name, "flags": flags},
					)
					return nil
				},
			},
			{
				Name:      "use",
				Usage:     "Set a profile as the default",
				ArgsUsage: "<name>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() < 1 {
						return fmt.Errorf("profile use requires a name argument")
					}
					name := ctx.Args().First()
					w := NewWriter(ctx)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to load profile store: " + err.Error()})
						return nil
					}
					if err := store.SetDefault(name); err != nil {
						w.WriteError(&murli.AgentError{
							Code:        murli.ExitNotFound,
							ErrorType:   "not_found",
							Message:     fmt.Sprintf("profile %q not found", name),
							Suggestion:  "Run 'profile list' to see available profiles.",
							Recoverable: false,
						})
						return nil
					}
					if err := store.Save(app.Name); err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to save profile store: " + err.Error()})
						return nil
					}
					w.WriteSuccess(
						fmt.Sprintf("Profile %q is now the default.", name),
						map[string]any{"default": name},
					)
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "List all saved profiles",
				Action: func(ctx *cli.Context) error {
					w := NewWriter(ctx)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to load profile store: " + err.Error()})
						return nil
					}
					names := store.Names()
					payload := map[string]any{"profiles": names}
					if store.Default != "" {
						payload["default"] = store.Default
					}
					lines := make([]string, 0, len(names))
					for _, n := range names {
						if n == store.Default {
							lines = append(lines, "  "+n+" *")
						} else {
							lines = append(lines, "  "+n)
						}
					}
					humanText := strings.Join(lines, "\n")
					if humanText == "" {
						humanText = "(no profiles saved)"
					}
					w.WriteSuccess(humanText, payload)
					return nil
				},
			},
			{
				Name:      "show",
				Usage:     "Show the flag values in a profile",
				ArgsUsage: "<name>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() < 1 {
						return fmt.Errorf("profile show requires a name argument")
					}
					name := ctx.Args().First()
					w := NewWriter(ctx)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to load profile store: " + err.Error()})
						return nil
					}
					profile, ok := store.Get(name)
					if !ok {
						w.WriteError(&murli.AgentError{
							Code:        murli.ExitNotFound,
							ErrorType:   "not_found",
							Message:     fmt.Sprintf("profile %q not found", name),
							Suggestion:  "Run 'profile list' to see available profiles.",
							Recoverable: false,
						})
						return nil
					}
					w.WriteSuccess(
						fmt.Sprintf("Profile %q: %v", name, profile.Flags),
						map[string]any{"name": name, "flags": profile.Flags},
					)
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a saved profile",
				ArgsUsage: "<name>",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() < 1 {
						return fmt.Errorf("profile delete requires a name argument")
					}
					name := ctx.Args().First()
					w := NewWriter(ctx)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to load profile store: " + err.Error()})
						return nil
					}
					if _, ok := store.Get(name); !ok {
						w.WriteError(&murli.AgentError{
							Code:        murli.ExitNotFound,
							ErrorType:   "not_found",
							Message:     fmt.Sprintf("profile %q not found", name),
							Suggestion:  "Run 'profile list' to see available profiles.",
							Recoverable: false,
						})
						return nil
					}
					store.Delete(name)
					if err := store.Save(app.Name); err != nil {
						w.WriteError(&murli.AgentError{Code: murli.ExitToolError, ErrorType: "tool_error", Message: "failed to save profile store: " + err.Error()})
						return nil
					}
					w.WriteSuccess(
						fmt.Sprintf("Profile %q deleted.", name),
						map[string]any{"deleted": name},
					)
					return nil
				},
			},
		},
	}
	return profileCmd
}
