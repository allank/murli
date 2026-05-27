package murli

// SafetyBlock summarises the safety properties of a command for agent consumption.
// Assembled from Metadata fields by the adapter at schema/describe emit time.
type SafetyBlock struct {
	ReadOnly    bool `json:"read_only"`
	Idempotent  bool `json:"idempotent"`
	Destructive bool `json:"destructive,omitempty"`
	DryRunnable bool `json:"dry_run_supported,omitempty"`
}

// CommandSchema is the JSON payload emitted by --schema.
type CommandSchema struct {
	Name             string             `json:"name"`
	Summary          string             `json:"summary"`
	WhenToUse        string             `json:"when_to_use,omitempty"`
	AgentDescription string             `json:"agent_description,omitempty"`
	Idempotent       bool               `json:"idempotent"`
	Mutating         bool               `json:"mutating,omitempty"`
	Arguments        []ArgumentMetadata `json:"arguments,omitempty"`
	Flags            []FlagSchema       `json:"flags,omitempty"`
	Returns          *ReturnSchema      `json:"returns,omitempty"`
	Examples         []Example          `json:"examples,omitempty"`
	Subcommands      []SubcommandSchema `json:"subcommands,omitempty"`
	Safety           SafetyBlock        `json:"safety"`
}

// FlagSchema represents a single CLI flag in the JSON schema.
// Basic fields (Name, Type, Default, Description) are auto-populated by the adapter.
// Extended fields (Env, Sensitive, Persistent, MutuallyExclusiveWith, Enum, Pattern, Profileable)
// are populated from Metadata.FlagAnnotations[flagName] by the adapter schema emitters
// (see cobra/schema.go, cli/v2/schema.go, cli/v3/schema.go).
type FlagSchema struct {
	Name                  string   `json:"name"`
	Type                  string   `json:"type"`
	Default               any      `json:"default"`
	Description           string   `json:"description"`
	Env                   string   `json:"env,omitempty"`
	Sensitive             bool     `json:"sensitive,omitempty"`
	Persistent            bool     `json:"persistent,omitempty"`
	MutuallyExclusiveWith []string `json:"mutually_exclusive_with,omitempty"`
	Enum                  []string `json:"enum,omitempty"`
	Pattern               string   `json:"pattern,omitempty"`
	Profileable           bool     `json:"profileable,omitempty"`
}

// SubcommandSchema represents a registered subcommand in --schema output (brief listing).
type SubcommandSchema struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// Example is a concrete usage example for a command, used in Metadata.Examples.
type Example struct {
	Command          string `json:"command"`
	Description      string `json:"description,omitempty"`
	ExpectedExitCode int    `json:"expected_exit_code,omitempty"` // 0 = success; omitted
}

// FlagAnnotation provides extended metadata for a single flag, keyed by flag name
// in Metadata.FlagAnnotations. None of these fields can be auto-detected from the
// CLI framework; all must be supplied by the engineer via Annotate().
type FlagAnnotation struct {
	Env                   string   `json:"env,omitempty"`
	Sensitive             bool     `json:"sensitive,omitempty"`
	Persistent            bool     `json:"persistent,omitempty"`
	MutuallyExclusiveWith []string `json:"mutually_exclusive_with,omitempty"`
	Enum                  []string `json:"enum,omitempty"`
	Pattern               string   `json:"pattern,omitempty"`
	Profileable           bool     `json:"profileable,omitempty"`
}

// ApplyFlagAnnotation merges a FlagAnnotation onto a FlagSchema in place.
// Boolean fields (Sensitive, Persistent, Profileable) are one-way: they can be set to true
// but not cleared to false. This is intentional — ApplyFlagAnnotation is applied
// to a zero-value FlagSchema, so false is the default and only needs to be set once.
// Slice fields (MutuallyExclusiveWith, Enum) are deep-copied to prevent aliasing.
func ApplyFlagAnnotation(fs *FlagSchema, ann FlagAnnotation) {
	if ann.Env != "" {
		fs.Env = ann.Env
	}
	if ann.Sensitive {
		fs.Sensitive = true
	}
	if ann.Persistent {
		fs.Persistent = true
	}
	if len(ann.MutuallyExclusiveWith) > 0 {
		fs.MutuallyExclusiveWith = append([]string(nil), ann.MutuallyExclusiveWith...)
	}
	if len(ann.Enum) > 0 {
		fs.Enum = append([]string(nil), ann.Enum...)
	}
	if ann.Pattern != "" {
		fs.Pattern = ann.Pattern
	}
	if ann.Profileable {
		fs.Profileable = true
	}
}

// DescribeCommandSchema is the full recursive schema used in describe output.
// Unlike CommandSchema (for --schema), Subcommands here carry full recursive data.
type DescribeCommandSchema struct {
	Name             string                  `json:"name"`
	Summary          string                  `json:"summary"`
	WhenToUse        string                  `json:"when_to_use,omitempty"`
	AgentDescription string                  `json:"agent_description,omitempty"`
	Idempotent       bool                    `json:"idempotent"`
	Mutating         bool                    `json:"mutating,omitempty"`
	Arguments        []ArgumentMetadata      `json:"arguments,omitempty"`
	Flags            []FlagSchema            `json:"flags,omitempty"`
	Returns          *ReturnSchema           `json:"returns,omitempty"`
	Examples         []Example               `json:"examples,omitempty"`
	Subcommands      []DescribeCommandSchema  `json:"subcommands,omitempty"`
	Safety           SafetyBlock             `json:"safety"`
}

// DescribeOutput is emitted by the auto-mounted `describe` subcommand.
// Commands lists the root-level commands of the tool. Each DescribeCommandSchema
// within Commands has its own Subcommands field for nested commands, creating
// a recursive tree. The top-level field is named "commands" (not "subcommands")
// to distinguish root commands from nested subcommands.
type DescribeOutput struct {
	Name          string                  `json:"name"`
	Summary       string                  `json:"summary"`
	SchemaVersion string                  `json:"schema_version"`
	ToolVersion   string                  `json:"tool_version,omitempty"`
	Capabilities  Capabilities            `json:"capabilities"`
	Profiles      *ProfilesInfo           `json:"profiles,omitempty"`
	Commands      []DescribeCommandSchema  `json:"commands,omitempty"`
}

// Capabilities describes what the binary supports, auto-populated by murli in describe output.
type Capabilities struct {
	Streaming       bool     `json:"streaming"`
	DryRun          bool     `json:"dry_run"`
	OutputFormats   []string `json:"output_formats,omitempty"`
	SchemaVersion   string   `json:"schema_version"`
	ToolVersion     string   `json:"tool_version,omitempty"`
	ProtocolVersion string   `json:"protocol_version,omitempty"`
	Profiles        bool     `json:"profiles"`
}

// ProfilesInfo describes the profile system state for agent consumption.
// Included in DescribeOutput when the tool supports profiles.
type ProfilesInfo struct {
	Available        []string `json:"available,omitempty"`
	Default          string   `json:"default,omitempty"`
	ProfileableFlags []string `json:"profileable_flags"` // always present; empty array when no flags are profileable
}

// DefaultCapabilities returns the Capabilities block reflecting the current murli build.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		Streaming:     true,
		DryRun:        false,
		OutputFormats: []string{"json", "ndjson", "text"},
		SchemaVersion: SchemaVersion,
		ToolVersion:   ToolVersion,
		Profiles:      true,
	}
}
