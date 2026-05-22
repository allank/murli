package murli

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandSchema represents the JSON-RPC or tool-calling schema emitted for an agent.
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

// EmitSchema extracts metadata and flag schemas for a command, prints it to Stdout in JSON, and returns nil.
func EmitSchema(cmd *cobra.Command) error {
	var meta Metadata

	// Fetch registered metadata from annotations
	if cmd.Annotations != nil {
		if rawMeta, exists := cmd.Annotations["agentcobra"]; exists {
			_ = json.Unmarshal([]byte(rawMeta), &meta)
		}
	}

	// Build the schema payload
	schema := CommandSchema{
		Name:             cmd.Name(),
		Summary:          cmd.Short,
		WhenToUse:        meta.WhenToUse,
		AgentDescription: meta.AgentDescription,
		Idempotent:       meta.Idempotent,
		Returns:          meta.Returns,
		Examples:         meta.Examples,
	}

	// 1. Parse positional arguments
	schema.Arguments = mergeArguments(cmd.Use, meta.Arguments)

	// 2. Map flags
	schema.Flags = getFlagSchemas(cmd)

	// 3. Collect subcommands
	schema.Subcommands = getSubcommandSchemas(cmd)

	// Format output with indent for premium legibility
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(schema)
}

// mergeArguments parses the Use string and merges custom argument metadata
func mergeArguments(use string, customArgs []ArgumentMetadata) []ArgumentMetadata {
	tokens := strings.Fields(use)
	if len(tokens) <= 1 {
		// No arguments parsed, fallback to custom metadata if provided
		return customArgs
	}

	var parsed []ArgumentMetadata
	for _, token := range tokens[1:] {
		lower := strings.ToLower(token)
		if lower == "[flags]" || lower == "[flag]" {
			continue
		}

		required := false
		name := token
		if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
			required = true
			name = token[1 : len(token)-1]
		} else if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") {
			required = false
			name = token[1 : len(token)-1]
		} else {
			required = true
		}

		name = strings.TrimSuffix(name, "...")
		parsed = append(parsed, ArgumentMetadata{
			Name:     name,
			Type:     "string",
			Required: required,
		})
	}

	// Map custom metadata for enrichment
	customMap := make(map[string]ArgumentMetadata)
	for _, arg := range customArgs {
		customMap[arg.Name] = arg
	}

	var finalArgs []ArgumentMetadata
	for _, pArg := range parsed {
		if cArg, ok := customMap[pArg.Name]; ok {
			merged := pArg
			if cArg.Type != "" {
				merged.Type = cArg.Type
			}
			if cArg.Description != "" {
				merged.Description = cArg.Description
			}
			// Let metadata override required field if explicitly set
			if cArg.Required {
				merged.Required = true
			}
			finalArgs = append(finalArgs, merged)
		} else {
			finalArgs = append(finalArgs, pArg)
		}
	}

	return finalArgs
}

// getFlagSchemas extracts and type-converts default values for command flags
func getFlagSchemas(cmd *cobra.Command) []FlagSchema {
	var list []FlagSchema
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Omit the global schema/agent flags from the agent schema itself
		if f.Name == "schema" || f.Name == "agent" {
			return
		}

		t := f.Value.Type()
		var defVal any = f.DefValue

		// Safe JSON-type casting for standard Cobra/pflag types
		switch t {
		case "int", "int8", "int16", "int32", "int64":
			if val, err := strconv.ParseInt(f.DefValue, 10, 64); err == nil {
				defVal = val
			}
		case "uint", "uint8", "uint16", "uint32", "uint64":
			if val, err := strconv.ParseUint(f.DefValue, 10, 64); err == nil {
				defVal = val
			}
		case "bool":
			if val, err := strconv.ParseBool(f.DefValue); err == nil {
				defVal = val
			}
		case "float32", "float64":
			if val, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
				defVal = val
			}
		}

		list = append(list, FlagSchema{
			Name:        f.Name,
			Type:        t,
			Default:     defVal,
			Description: f.Usage,
		})
	})
	return list
}

// getSubcommandSchemas gets schemas for all subcommands
func getSubcommandSchemas(cmd *cobra.Command) []SubcommandSchema {
	var list []SubcommandSchema
	for _, sub := range cmd.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		list = append(list, SubcommandSchema{
			Name:    sub.Name(),
			Summary: sub.Short,
		})
	}
	return list
}
