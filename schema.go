package murli

// CommandSchema is the JSON payload emitted by --schema.
type CommandSchema struct {
	Name             string             `json:"name"`
	Summary          string             `json:"summary"`
	WhenToUse        string             `json:"when_to_use,omitempty"`
	AgentDescription string             `json:"agent_description,omitempty"`
	Idempotent       bool               `json:"idempotent"`
	Arguments        []ArgumentMetadata `json:"arguments,omitempty"`
	Flags            []FlagSchema       `json:"flags,omitempty"`
	Returns          *ReturnSchema      `json:"returns,omitempty"`
	Examples         []string           `json:"examples,omitempty"`
	Subcommands      []SubcommandSchema `json:"subcommands,omitempty"`
}

// FlagSchema represents a single CLI flag in the JSON schema.
type FlagSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// SubcommandSchema represents a registered subcommand.
type SubcommandSchema struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}
