package murli

import (
	"strings"
	"testing"
)

func TestFormatAgentsMD_BasicStructure(t *testing.T) {
	out := DescribeOutput{
		Name:          "riffle",
		Summary:       "Riffle semantic search tool",
		SchemaVersion: "1.0",
		Commands: []DescribeCommandSchema{
			{Name: "query", Summary: "Search the index"},
			{Name: "delete", Summary: "Delete an entry", AgentDescription: "Deletes a single entry by ID."},
		},
	}
	md := FormatAgentsMD(out)

	checks := []struct {
		desc string
		want string
	}{
		{"has title", "# AGENTS.md"},
		{"has tool name", "## Tool: riffle"},
		{"has summary", "Riffle semantic search tool"},
		{"has introspection section", "## Introspection"},
		{"has describe invocation", "riffle describe"},
		{"has schema_version", "## Schema Version"},
		{"has version value", "`1.0`"},
		{"has commands section", "## Commands"},
		{"has query command", "### query"},
		{"has query summary", "Search the index"},
		{"has delete command", "### delete"},
		{"has delete agent_description", "Deletes a single entry by ID."},
		{"has schema reference", "riffle query --schema"},
	}
	for _, c := range checks {
		t.Run(c.desc, func(t *testing.T) {
			if !strings.Contains(md, c.want) {
				t.Errorf("expected output to contain %q\nfull output:\n%s", c.want, md)
			}
		})
	}
}

func TestFormatAgentsMD_NoCommands(t *testing.T) {
	out := DescribeOutput{
		Name:          "mytool",
		Summary:       "A tool",
		SchemaVersion: "1.0",
	}
	md := FormatAgentsMD(out)
	if !strings.Contains(md, "# AGENTS.md") {
		t.Error("must have title even with no commands")
	}
	if strings.Contains(md, "## Commands") {
		t.Error("commands section must be omitted when there are no commands")
	}
}

func TestFormatAgentsMD_SubcommandsListed(t *testing.T) {
	out := DescribeOutput{
		Name:          "mytool",
		SchemaVersion: "1.0",
		Commands: []DescribeCommandSchema{
			{
				Name:    "repo",
				Summary: "Manage repos",
				Subcommands: []DescribeCommandSchema{
					{Name: "clone", Summary: "Clone a repo"},
					{Name: "list", Summary: "List repos"},
				},
			},
		},
	}
	md := FormatAgentsMD(out)
	if !strings.Contains(md, "`clone`") || !strings.Contains(md, "`list`") {
		t.Errorf("subcommand names must appear in output:\n%s", md)
	}
}
