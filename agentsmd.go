package murli

import (
	"fmt"
	"strings"
)

// FormatAgentsMD generates an AGENTS.md stub from a DescribeOutput.
// The stub gives agents a quick orientation: tool name and purpose, how to
// introspect further, and a per-command summary with --schema references.
// Intended to be pasted as a starting point into the repository's AGENTS.md.
func FormatAgentsMD(out DescribeOutput) string {
	var sb strings.Builder
	fmt.Fprintln(&sb, "# AGENTS.md")
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "> Auto-generated from `%s describe`. Edit to add project context.\n", out.Name)
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "## Tool: %s\n", out.Name)
	fmt.Fprintln(&sb)
	if out.Summary != "" {
		fmt.Fprintln(&sb, out.Summary)
		fmt.Fprintln(&sb)
	}
	fmt.Fprintln(&sb, "## Introspection")
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "```bash")
	fmt.Fprintf(&sb, "%s describe        # full JSON schema\n", out.Name)
	fmt.Fprintf(&sb, "%s --help          # human-readable help\n", out.Name)
	fmt.Fprintln(&sb, "```")
	fmt.Fprintln(&sb)
	fmt.Fprintln(&sb, "## Schema Version")
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "`%s`\n", out.SchemaVersion)
	if len(out.Commands) > 0 {
		fmt.Fprintln(&sb)
		fmt.Fprintln(&sb, "## Commands")
		for _, cmd := range out.Commands {
			fmt.Fprintln(&sb)
			fmt.Fprintf(&sb, "### %s\n", cmd.Name)
			fmt.Fprintln(&sb)
			if cmd.AgentDescription != "" {
				fmt.Fprintln(&sb, cmd.AgentDescription)
				fmt.Fprintln(&sb)
			} else if cmd.Summary != "" {
				fmt.Fprintln(&sb, cmd.Summary)
				fmt.Fprintln(&sb)
			}
			fmt.Fprintln(&sb, "```bash")
			fmt.Fprintf(&sb, "%s %s --schema    # JSON schema for this command\n", out.Name, cmd.Name)
			fmt.Fprintln(&sb, "```")
			if len(cmd.Subcommands) > 0 {
				fmt.Fprintln(&sb)
				names := make([]string, 0, len(cmd.Subcommands))
				for _, sub := range cmd.Subcommands {
					names = append(names, "`"+sub.Name+"`")
				}
				fmt.Fprintf(&sb, "**Subcommands:** %s\n", strings.Join(names, ", "))
			}
		}
	}
	return sb.String()
}
