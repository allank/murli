package cli

import (
	"github.com/allank/murli"
	"github.com/urfave/cli/v2"
)

// commandMeta stores per-command metadata. *cli.Command has no Metadata field in v2;
// only *cli.App does. We use a pointer-keyed map as the storage mechanism.
var commandMeta = make(map[*cli.Command]murli.Metadata)

// Annotate registers murli Metadata for a urfave/cli v2 command.
func Annotate(cmd *cli.Command, meta murli.Metadata) {
	commandMeta[cmd] = meta
}

// metadataFor retrieves registered metadata for a command, returning zero-value if absent.
func metadataFor(cmd *cli.Command) murli.Metadata {
	return commandMeta[cmd]
}

// appMeta stores metadata for *cli.App roots, used to identify profileable flags.
// Unlike commands (which use commandMeta), apps have no natural home for metadata in v2.
var appMeta = make(map[*cli.App]murli.Metadata)

// AnnotateApp registers murli Metadata for the root cli.App.
// Use this to mark root-level app flags (those on app.Flags) as Profileable.
func AnnotateApp(app *cli.App, meta murli.Metadata) {
	appMeta[app] = meta
}

// appMetadataFor retrieves registered metadata for an app, returning zero-value if absent.
func appMetadataFor(app *cli.App) murli.Metadata {
	return appMeta[app]
}
