package murli

// SchemaVersion is the murli envelope schema version.
// Frozen at "1.0"; incrementing requires a documented migration.
const SchemaVersion = "1.0"

// ToolVersion is the version of the CLI tool using murli.
// Set this in your main() using a version variable injected at build time:
//
//	murli.ToolVersion = version // where `version` is set via -ldflags "-X main.version=1.2.3"
//
// Or inject directly at build time:
//
//	-ldflags "-X github.com/allank/murli.ToolVersion=1.2.3"
var ToolVersion = ""
