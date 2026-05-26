package cobra

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/allank/murli"
	gocobra "github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// EmitSchema extracts command metadata and flag schemas, prints JSON to Stdout, and returns nil.
func EmitSchema(cmd *gocobra.Command) error {
	var meta murli.Metadata
	if cmd.Annotations != nil {
		if raw, ok := cmd.Annotations["agentcobra"]; ok {
			_ = json.Unmarshal([]byte(raw), &meta)
		}
	}

	schema := murli.CommandSchema{
		Name:             cmd.Name(),
		Summary:          cmd.Short,
		WhenToUse:        meta.WhenToUse,
		AgentDescription: meta.AgentDescription,
		Idempotent:       meta.Idempotent,
		Mutating:         meta.Mutating,
		Returns:          meta.Returns,
		Examples:         meta.Examples,
		Arguments:        mergeArguments(cmd.Use, meta.Arguments),
		Flags:            getFlagSchemas(cmd, meta.FlagAnnotations),
		Subcommands:      getSubcommandSchemas(cmd),
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(schema)
}

func mergeArguments(use string, customArgs []murli.ArgumentMetadata) []murli.ArgumentMetadata {
	tokens := strings.Fields(use)
	if len(tokens) <= 1 {
		return customArgs
	}

	var parsed []murli.ArgumentMetadata
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
			name = token[1 : len(token)-1]
		} else {
			required = true
		}
		name = strings.TrimSuffix(name, "...")
		parsed = append(parsed, murli.ArgumentMetadata{Name: name, Type: "string", Required: required})
	}

	customMap := make(map[string]murli.ArgumentMetadata)
	for _, arg := range customArgs {
		customMap[arg.Name] = arg
	}

	var final []murli.ArgumentMetadata
	for _, p := range parsed {
		if c, ok := customMap[p.Name]; ok {
			if c.Type != "" {
				p.Type = c.Type
			}
			if c.Description != "" {
				p.Description = c.Description
			}
			if c.Required {
				p.Required = true
			}
		}
		final = append(final, p)
	}
	return final
}

func getFlagSchemas(cmd *gocobra.Command, annotations map[string]murli.FlagAnnotation) []murli.FlagSchema {
	var list []murli.FlagSchema
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "schema" || f.Name == "agent" || f.Name == "output" || f.Name == "protocol-version" {
			return
		}
		t := f.Value.Type()
		var defVal any = f.DefValue
		switch t {
		case "int", "int8", "int16", "int32", "int64":
			if v, err := strconv.ParseInt(f.DefValue, 10, 64); err == nil {
				defVal = v
			}
		case "uint", "uint8", "uint16", "uint32", "uint64":
			if v, err := strconv.ParseUint(f.DefValue, 10, 64); err == nil {
				defVal = v
			}
		case "bool":
			if v, err := strconv.ParseBool(f.DefValue); err == nil {
				defVal = v
			}
		case "float32", "float64":
			if v, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
				defVal = v
			}
		}
		fs := murli.FlagSchema{Name: f.Name, Type: t, Default: defVal, Description: f.Usage}
		if ann, ok := annotations[f.Name]; ok {
			murli.ApplyFlagAnnotation(&fs, ann)
		}
		list = append(list, fs)
	})
	return list
}

func getSubcommandSchemas(cmd *gocobra.Command) []murli.SubcommandSchema {
	var list []murli.SubcommandSchema
	for _, sub := range cmd.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		list = append(list, murli.SubcommandSchema{Name: sub.Name(), Summary: sub.Short})
	}
	return list
}
