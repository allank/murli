package murli

// SchemaVersion is the murli envelope schema version.
// Frozen at "1.0" once v1.0 ships; incrementing requires a documented migration.
const SchemaVersion = "0.2"

// ToolVersion is the version of the CLI tool using murli.
// Set this at startup, typically from ldflags:
//
//	murli.ToolVersion = version // injected by -ldflags "-X main.version=1.2.3"
var ToolVersion = ""
