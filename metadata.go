package murli

import "encoding/json"

// Metadata provides LLM-specific parameters to supplement standard CLI command definitions.
type Metadata struct {
	// AgentDescription is a detailed description of the command's scope.
	AgentDescription string `json:"agent_description"`

	// WhenToUse specifies when an agent should select this command.
	WhenToUse string `json:"when_to_use"`

	// Idempotent specifies if the command is safe to re-run on failure.
	Idempotent bool `json:"idempotent"`

	// Mutating marks commands that write, delete, or otherwise change state.
	// When true and the output is not a TTY, the adapter rejects the command with a
	// confirmation_required error to prevent accidental mutation in non-interactive mode.
	// A bypass flag (--force / --yes) will be added in a future release.
	Mutating bool `json:"mutating,omitempty"`

	// Arguments defines explicit positional argument documentation.
	Arguments []ArgumentMetadata `json:"arguments,omitempty"`

	// Returns defines the expected output format on success.
	Returns *ReturnSchema `json:"returns,omitempty"`

	// Examples contains concrete shell invocations for agent in-context learning.
	// Changed from []string to []Example in v0.3.
	Examples []Example `json:"examples,omitempty"`

	// FlagAnnotations provides extended metadata keyed by flag name, for fields that
	// cannot be auto-detected from the CLI framework (env var binding, enum values, etc.).
	FlagAnnotations map[string]FlagAnnotation `json:"flag_annotations,omitempty"`

	// DryRunnable marks commands that support --dry-run preview mode.
	// When true, murli auto-registers a --dry-run flag on this command.
	// Engineers must check IsDryRun() in their action and call WritePlan() if true.
	DryRunnable bool `json:"dry_runnable,omitempty"`

	// Destructive marks commands whose effects cannot be undone
	// (e.g. delete, overwrite, truncate).
	Destructive bool `json:"destructive,omitempty"`

	// Reversible marks commands that can be undone if something goes wrong
	// (e.g. the command creates a backup before modifying).
	Reversible bool `json:"reversible,omitempty"`
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
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Shape       map[string]any  `json:"shape,omitempty"`
	// OutputSchema carries a raw JSON Schema (draft-2020-12) blob when the engineer
	// provides it via Annotate(). murli serialises it as-is into --schema and describe output.
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}
