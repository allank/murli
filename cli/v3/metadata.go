package cli

import (
	"github.com/allank/murli"
	"github.com/urfave/cli/v3"
)

const metadataKey = "murli"

// Annotate stores murli Metadata in the command's Metadata map.
func Annotate(cmd *cli.Command, meta murli.Metadata) {
	if cmd.Metadata == nil {
		cmd.Metadata = make(map[string]any)
	}
	cmd.Metadata[metadataKey] = meta
}

// metadataFor retrieves stored metadata, returning zero-value if absent.
func metadataFor(cmd *cli.Command) murli.Metadata {
	if cmd.Metadata == nil {
		return murli.Metadata{}
	}
	if m, ok := cmd.Metadata[metadataKey].(murli.Metadata); ok {
		return m
	}
	return murli.Metadata{}
}
