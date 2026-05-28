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

// buildV3AppDescribeOutput builds the DescribeOutput for app.
// Called by both the describe command and the doctor command.
func buildV3AppDescribeOutput(app *cli.Command) murli.DescribeOutput {
	appStore, _ := murli.LoadProfileStore(app.Name)
	rootMeta := metadataFor(app)
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
		Name:          app.Name,
		Summary:       app.Usage,
		SchemaVersion: murli.SchemaVersion,
		ToolVersion:   murli.ToolVersion,
		Capabilities:  murli.DefaultCapabilities(),
		Profiles:      profilesInfo,
	}
	for _, cmd := range app.Commands {
		if cmd.Hidden || cmd.Name == "describe" {
			continue
		}
		out.Commands = append(out.Commands, BuildV3DescribeTree(cmd))
	}
	return out
}

// Wrap injects --schema and --agent flags and wraps all command Actions.
func Wrap(app *cli.Command) {
	// Guard against double-wrapping on repeated Wrap() calls.
	if app.Metadata == nil {
		app.Metadata = make(map[string]any)
	}
	if app.Metadata["murli_enabled"] == true {
		return
	}
	app.Metadata["murli_enabled"] = true

	// Suppress urfave/cli's built-in error printing so murli controls all stderr output.
	app.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}

	// Register --profile and --agent root flags if not already present.
	hasProfile := false
	hasAgent := false
	for _, f := range app.Flags {
		names := f.Names()
		if len(names) == 0 {
			continue
		}
		switch names[0] {
		case "profile":
			hasProfile = true
		case "agent":
			hasAgent = true
		}
	}
	if !hasProfile {
		app.Flags = append(app.Flags, &cli.StringFlag{Name: "profile", Usage: "Profile name to use for this invocation"})
	}
	if !hasAgent {
		app.Flags = append(app.Flags, &cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"})
	}

	wrapCommands(app.Commands, app)

	// Naming convention advisory: collect names, emit warnings if TTY.
	if isTTYWriter(writerOrDefault(app.Writer, os.Stdout)) {
		var cmdNames, flagNames []string
		for _, cmd := range app.Commands {
			collectV3Names(cmd, &cmdNames, &flagNames)
		}
		murli.CheckConventions(cmdNames, flagNames, writerOrDefault(app.ErrWriter, os.Stderr))
	}

	// Auto-mount describe command if not already present.
	for _, c := range app.Commands {
		if c.Name == "describe" {
			// describe already mounted by caller — skip describe mounting but still mount profile below.
			// goto rather than return so both subcommands are mounted independently.
			goto mountProfile
		}
	}
	{
		describeV3 := &cli.Command{
			Name:  "describe",
			Usage: "Print the full command tree and capabilities as a single JSON document",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|text"},
				&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version (0.2)"},
				&cli.BoolFlag{Name: "agents-md", Usage: "Generate an AGENTS.md stub instead of JSON"},
			},
			Action: func(ctx context.Context, c *cli.Command) error {
				out := buildV3AppDescribeOutput(app)

				stdout := writerOrDefault(app.Writer, os.Stdout)
				if c.Bool("agents-md") {
					fmt.Fprint(stdout, murli.FormatAgentsMD(out))
					return nil
				}

				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				_ = enc.Encode(out)
				return nil
			},
		}
		app.Commands = append(app.Commands, describeV3)
	}

mountProfile:
	// Auto-mount profile subcommand group if not already present.
	profileAlreadyMounted := false
	for _, c := range app.Commands {
		if c.Name == "profile" {
			profileAlreadyMounted = true
			break
		}
	}
	if !profileAlreadyMounted {
		app.Commands = append(app.Commands, buildV3ProfileGroup(app))
	}

	// Auto-mount doctor command if not already present.
	for _, c := range app.Commands {
		if c.Name == "doctor" {
			return
		}
	}
	app.Commands = append(app.Commands, buildV3DoctorCmd(app))
}

// writeDoctorTTY writes a human-readable doctor report to w.
func writeDoctorTTY(w io.Writer, report murli.DoctorReport) {
	for _, c := range report.Checks {
		var icon string
		switch c.Status {
		case "pass":
			icon = "✓"
		case "warn":
			icon = "⚠"
		case "fail":
			icon = "✗"
		default:
			icon = "?"
		}
		if c.Message != "" {
			fmt.Fprintf(w, "%s %s: %s\n", icon, c.Name, c.Message)
		} else {
			fmt.Fprintf(w, "%s %s\n", icon, c.Name)
		}
	}
	fmt.Fprintf(w, "\n%d passed, %d warnings, %d failed\n",
		report.Passed, report.Warnings, report.Failed)
}

func buildV3DoctorCmd(app *cli.Command) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Run murli integration self-checks",
		Action: func(ctx context.Context, c *cli.Command) error {
			out := buildV3AppDescribeOutput(app)
			report := murli.RunDoctor(out)

			stdout := writerOrDefault(app.Writer, os.Stdout)
			stderr := writerOrDefault(app.ErrWriter, os.Stderr)
			// doctor has no --agent flag registered on it; access root's flag.
			agentMode := c.Root().Bool("agent")
			w := murli.NewWriter(stdout, stderr, agentMode)

			if w.IsTTY() {
				writeDoctorTTY(stdout, report)
				return nil
			}
			summary := "all checks passed"
			if report.Failed > 0 {
				summary = fmt.Sprintf("%d check(s) failed", report.Failed)
			} else if report.Warnings > 0 {
				summary = fmt.Sprintf("%d warning(s)", report.Warnings)
			}
			w.WriteSuccess(summary, report)
			return nil
		},
	}
}

func wrapCommands(cmds []*cli.Command, root *cli.Command) {
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
			if len(cmd.Commands) > 0 {
				wrapCommands(cmd.Commands, root)
			}
			continue
		}

		cmd.Flags = append(cmd.Flags,
			&cli.BoolFlag{Name: "schema", Usage: "Output agent-optimized JSON schema"},
			&cli.BoolFlag{Name: "agent", Usage: "Force agent-optimized JSON mode"},
			&cli.StringFlag{Name: "output", Usage: "Output format: json|ndjson|text"},
			&cli.StringFlag{Name: "protocol-version", Usage: "Protocol version (0.2)"},
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

		cmd.Action = func(ctx context.Context, c *cli.Command) error {
			if stopped := applyV3Profile(c); stopped {
				return nil // not_found error already written
			}

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
						Suggestion:  "Use --protocol-version 0.2",
						Recoverable: true,
						ValidValues: murli.ValidProtocolVersions,
					})
					return nil
				}
			}

			// Output format validation.
			if outFmt := c.String("output"); outFmt != "" {
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
						Suggestion:  "Use --output json, ndjson, or text",
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
				runErr = originalAction(ctx, c)
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

// applyV3Profile reads the active profile and applies stored flag values to root
// flags that were not explicitly set.
// Returns true if execution should stop (not_found error written for explicit missing profile).
func applyV3Profile(c *cli.Command) (stopped bool) {
	root := c.Root()
	store, err := murli.LoadProfileStore(root.Name)
	if err != nil {
		return false
	}
	explicitProfile := root.String("profile")
	profileName := explicitProfile
	if profileName == "" {
		profileName = store.Default
	}
	if profileName == "" {
		return false
	}
	profile, ok := store.Get(profileName)
	if !ok {
		if explicitProfile != "" {
			w := NewWriter(c)
			w.WriteError(&murli.AgentError{
				Code:        murli.ExitNotFound,
				ErrorType:   "not_found",
				Message:     fmt.Sprintf("profile %q not found", explicitProfile),
				Suggestion:  "Run 'profile list' to see available profiles.",
				Recoverable: false,
			})
			return true
		}
		return false
	}
	for flagName, value := range profile.Flags {
		if !root.IsSet(flagName) {
			_ = root.Set(flagName, value)
		}
	}
	return false
}

// v3FlagStringValue returns the string representation of a named root flag's current value.
func v3FlagStringValue(root *cli.Command, name string) string {
	for _, f := range root.Flags {
		names := f.Names()
		if len(names) == 0 || names[0] != name {
			continue
		}
		switch f.(type) {
		case *cli.BoolFlag:
			return strconv.FormatBool(root.Bool(name))
		case *cli.IntFlag:
			return strconv.Itoa(root.Int(name))
		case *cli.Float64Flag:
			return strconv.FormatFloat(root.Float64(name), 'f', -1, 64)
		default:
			return root.String(name)
		}
	}
	return root.String(name)
}

// buildV3ProfileGroup builds the `profile` command group for urfave/cli v3.
func buildV3ProfileGroup(app *cli.Command) *cli.Command {
	return &cli.Command{
		Name:  "profile",
		Usage: "Manage saved flag profiles",
		Commands: []*cli.Command{
			{
				Name:      "save",
				Usage:     "Save current profileable flags as a named profile",
				ArgsUsage: "<name>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() < 1 {
						return fmt.Errorf("profile save requires a name argument")
					}
					name := c.Args().First()
					w := NewWriter(c)
					root := c.Root()
					rootMeta := metadataFor(app)
					flags := make(map[string]string)
					for flagName, ann := range rootMeta.FlagAnnotations {
						if ann.Profileable && root.IsSet(flagName) {
							flags[flagName] = v3FlagStringValue(root, flagName)
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
						w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
						return nil
					}
					store.Set(name, murli.Profile{Flags: flags})
					if err := store.Save(app.Name); err != nil {
						w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
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
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() < 1 {
						return fmt.Errorf("profile use requires a name argument")
					}
					name := c.Args().First()
					w := NewWriter(c)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
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
						w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
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
				Action: func(ctx context.Context, c *cli.Command) error {
					w := NewWriter(c)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
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
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() < 1 {
						return fmt.Errorf("profile show requires a name argument")
					}
					name := c.Args().First()
					w := NewWriter(c)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
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
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() < 1 {
						return fmt.Errorf("profile delete requires a name argument")
					}
					name := c.Args().First()
					w := NewWriter(c)
					store, err := murli.LoadProfileStore(app.Name)
					if err != nil {
						w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
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
						w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
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
}
