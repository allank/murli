package murli

// Metadata provides LLM-specific parameters to supplement standard CLI command definitions.
type Metadata struct {
	// AgentDescription is a detailed description of the command's scope.
	AgentDescription string `json:"agent_description"`

	// WhenToUse specifies when an agent should select this command.
	WhenToUse string `json:"when_to_use"`

	// Idempotent specifies if the command is safe to re-run on failure.
	Idempotent bool `json:"idempotent"`

	// Arguments defines explicit positional argument documentation.
	Arguments []ArgumentMetadata `json:"arguments,omitempty"`

	// Returns defines the expected output format on success.
	Returns *ReturnSchema `json:"returns,omitempty"`

	// Examples contains concrete shell invocations for agent in-context learning.
	Examples []string `json:"examples,omitempty"`
}

// ArgumentMetadata documents a single positional argument.
type ArgumentMetadata struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// ReturnSchema describes the shape of successful command output.
type ReturnSchema struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Shape       map[string]any `json:"shape,omitempty"`
}
