package murli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// Metadata provides LLM-specific parameters to supplement standard Cobra command definitions.
type Metadata struct {
	// AgentDescription is a detailed description of the command's scope, including warnings of what NOT to do.
	AgentDescription string `json:"agent_description"`

	// WhenToUse specifies the exact triggering condition that instructs the agent when to select this command.
	WhenToUse string `json:"when_to_use"`

	// Idempotent specifies if the command is safe to re-run multiple times on failure.
	Idempotent bool `json:"idempotent"`

	// Arguments defines the explicit positional arguments for the command.
	Arguments []ArgumentMetadata `json:"arguments,omitempty"`

	// Returns defines the expected JSON key-value pairs or text formats emitted on success.
	Returns *ReturnSchema `json:"returns,omitempty"`

	// Examples contains concrete shell execution invocations for training/in-context learning.
	Examples []string `json:"examples,omitempty"`
}

// ArgumentMetadata represents custom documentation for a positional argument.
type ArgumentMetadata struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// ReturnSchema represents the dynamic shape of standard output.
type ReturnSchema struct {
	Type        string         `json:"type"`            // "json" or "text"
	Description string         `json:"description"`
	Shape       map[string]any `json:"shape,omitempty"` // For JSON returns, outlines keys and types
}

// Annotate serializes and binds metadata to an existing Cobra Command.
func Annotate(cmd *cobra.Command, meta Metadata) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	data, _ := json.Marshal(meta)
	cmd.Annotations["agentcobra"] = string(data)
}
