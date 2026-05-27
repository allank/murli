package cobra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/allank/murli"
	gocobra "github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		rootCmd.PersistentFlags().String("output", "", "Output format: json|ndjson|text")
	}
	if rootCmd.PersistentFlags().Lookup("protocol-version") == nil {
		rootCmd.PersistentFlags().String("protocol-version", "", "Protocol version (0.2)")
	}
	if rootCmd.PersistentFlags().Lookup("profile") == nil {
		rootCmd.PersistentFlags().String("profile", "", "Profile name to use for this invocation")
	}
	// Naming convention advisory: emit warnings in TTY mode only (developer feedback).
	if isTTYWriter(rootCmd.OutOrStdout()) {
		var cmdNames, flagNames []string
		collectNames(rootCmd, &cmdNames, &flagNames)
		murli.CheckConventions(cmdNames, flagNames, rootCmd.ErrOrStderr())
	}

	// Guard against double-wrapping on repeated Enable() calls.
	if rootCmd.Annotations == nil {
		rootCmd.Annotations = make(map[string]string)
	}
	if rootCmd.Annotations["murli_enabled"] == "1" {
		return
	}
	rootCmd.Annotations["murli_enabled"] = "1"

	wrapCommands(rootCmd)

	// Auto-mount describe command if not already present.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "describe" {
			return // already mounted
		}
	}
	describeCmd := &gocobra.Command{
		Use:   "describe",
		Short: "Print the full command tree and capabilities as a single JSON document",
		RunE: func(cmd *gocobra.Command, args []string) error {
			store, _ := murli.LoadProfileStore(rootCmd.Name()) // empty store on error — never fail describe

			// Collect profileable root flag names (always a non-nil slice so it serialises as []).
			rootMeta := cobraMetadata(rootCmd)
			profileableNames := []string{}
			for flagName, ann := range rootMeta.FlagAnnotations {
				if ann.Profileable {
					profileableNames = append(profileableNames, flagName)
				}
			}
			sort.Strings(profileableNames)

			profilesInfo := &murli.ProfilesInfo{
				ProfileableFlags: profileableNames,
			}
			if store != nil && len(store.Names()) > 0 {
				profilesInfo.Available = store.Names()
				profilesInfo.Default = store.Default
			}

			out := murli.DescribeOutput{
				Name:          rootCmd.Name(),
				Summary:       rootCmd.Short,
				SchemaVersion: murli.SchemaVersion,
				ToolVersion:   murli.ToolVersion,
				Capabilities:  murli.DefaultCapabilities(),
				Profiles:      profilesInfo,
			}
			for _, child := range rootCmd.Commands() {
				if child.Hidden || child.Name() == "help" || child.Name() == "describe" {
					continue
				}
				out.Commands = append(out.Commands, BuildDescribeTree(child))
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			_ = enc.Encode(out)
			return nil
		},
	}
	rootCmd.AddCommand(describeCmd)

	// Auto-mount profile subcommand group if not already present.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "profile" {
			return
		}
	}
	rootCmd.AddCommand(buildCobraProfileGroup(rootCmd))
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

	// Register --force and --yes on mutating commands.
	meta := cobraMetadata(cmd)
	if meta.Mutating {
		if cmd.Flags().Lookup("force") == nil {
			cmd.Flags().Bool("force", false, "Bypass the non-interactive mutation guard")
		}
		if cmd.Flags().Lookup("yes") == nil {
			cmd.Flags().Bool("yes", false, "Bypass the non-interactive mutation guard")
		}
	}
	// Register --dry-run on DryRunnable commands.
	if meta.DryRunnable {
		if cmd.Flags().Lookup("dry-run") == nil {
			cmd.Flags().Bool("dry-run", false, "Preview the operation without executing it")
		}
	}

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
		if stopped := applyCobraProfile(c); stopped {
			return nil // not_found error already written
		}

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
					Suggestion:  "Use --protocol-version 0.2",
					Recoverable: true,
					ValidValues: murli.ValidProtocolVersions,
				})
				return nil
			}
		}

		// Output format validation.
		if outFmt, _ := c.Flags().GetString("output"); outFmt != "" {
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

		// Non-interactive guard: mutating commands must not block waiting for input.
		if meta := cobraMetadata(c); meta.Mutating && !w.IsTTY() && !w.IsForced() {
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
			return nil
		}
		return nil
	}

	for _, child := range cmd.Commands() {
		wrapCommands(child)
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

// collectNames gathers all command names and flag names recursively from cmd.
func collectNames(cmd *gocobra.Command, cmds, flags *[]string) {
	*cmds = append(*cmds, cmd.Name())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		*flags = append(*flags, f.Name)
	})
	for _, child := range cmd.Commands() {
		collectNames(child, cmds, flags)
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

// applyCobraProfile reads the active profile (from --profile flag or store default)
// and applies stored flag values to root persistent flags that were not explicitly set.
// Returns true if execution should stop (not_found error written for explicit missing profile).
func applyCobraProfile(c *gocobra.Command) (stopped bool) {
	root := c.Root()
	store, err := murli.LoadProfileStore(root.Name())
	if err != nil {
		return false // silent — disk errors must not break normal operation
	}
	explicitProfile, _ := root.PersistentFlags().GetString("profile")
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
			// User explicitly requested a non-existent profile — surface as error.
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
		f := root.PersistentFlags().Lookup(flagName)
		if f != nil && !f.Changed {
			_ = f.Value.Set(value)
		}
	}
	return false
}

// collectCobraProfileableFlags returns the set of root persistent flag names
// marked Profileable: true in the root command's murli metadata.
func collectCobraProfileableFlags(root *gocobra.Command) map[string]bool {
	meta := cobraMetadata(root)
	profileable := make(map[string]bool)
	for flagName, ann := range meta.FlagAnnotations {
		if ann.Profileable {
			profileable[flagName] = true
		}
	}
	return profileable
}

// buildCobraProfileGroup builds the `profile` command group with save/use/list/show/delete subcommands.
func buildCobraProfileGroup(root *gocobra.Command) *gocobra.Command {
	profileCmd := &gocobra.Command{
		Use:   "profile",
		Short: "Manage saved flag profiles",
	}

	// profile save <name>
	profileCmd.AddCommand(&gocobra.Command{
		Use:   "save <name>",
		Short: "Save current profileable flags as a named profile",
		Args:  gocobra.ExactArgs(1),
		RunE: func(c *gocobra.Command, args []string) error {
			name := args[0]
			w := NewWriter(c)
			profileable := collectCobraProfileableFlags(root)
			flags := make(map[string]string)
			root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
				if f.Changed && profileable[f.Name] {
					flags[f.Name] = f.Value.String()
				}
			})
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
			store, err := murli.LoadProfileStore(root.Name())
			if err != nil {
				w.WriteError(murli.NewToolError("failed to load profile store: " + err.Error()))
				return nil
			}
			store.Set(name, murli.Profile{Flags: flags})
			if err := store.Save(root.Name()); err != nil {
				w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
				return nil
			}
			w.WriteSuccess(
				fmt.Sprintf("Profile %q saved.", name),
				map[string]any{"profile": name, "flags": flags},
			)
			return nil
		},
	})

	// profile use <name>
	profileCmd.AddCommand(&gocobra.Command{
		Use:   "use <name>",
		Short: "Set a profile as the default",
		Args:  gocobra.ExactArgs(1),
		RunE: func(c *gocobra.Command, args []string) error {
			name := args[0]
			w := NewWriter(c)
			store, err := murli.LoadProfileStore(root.Name())
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
			if err := store.Save(root.Name()); err != nil {
				w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
				return nil
			}
			w.WriteSuccess(
				fmt.Sprintf("Profile %q is now the default.", name),
				map[string]any{"default": name},
			)
			return nil
		},
	})

	// profile list
	profileCmd.AddCommand(&gocobra.Command{
		Use:   "list",
		Short: "List all saved profiles",
		RunE: func(c *gocobra.Command, args []string) error {
			w := NewWriter(c)
			store, err := murli.LoadProfileStore(root.Name())
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
	})

	// profile show <name>
	profileCmd.AddCommand(&gocobra.Command{
		Use:   "show <name>",
		Short: "Show the flag values in a profile",
		Args:  gocobra.ExactArgs(1),
		RunE: func(c *gocobra.Command, args []string) error {
			name := args[0]
			w := NewWriter(c)
			store, err := murli.LoadProfileStore(root.Name())
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
	})

	// profile delete <name>
	profileCmd.AddCommand(&gocobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved profile",
		Args:  gocobra.ExactArgs(1),
		RunE: func(c *gocobra.Command, args []string) error {
			name := args[0]
			w := NewWriter(c)
			store, err := murli.LoadProfileStore(root.Name())
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
			if err := store.Save(root.Name()); err != nil {
				w.WriteError(murli.NewToolError("failed to save profile store: " + err.Error()))
				return nil
			}
			w.WriteSuccess(
				fmt.Sprintf("Profile %q deleted.", name),
				map[string]any{"deleted": name},
			)
			return nil
		},
	})

	return profileCmd
}
