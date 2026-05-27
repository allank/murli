package cli

import (
	"encoding/json"
	"io"

	"github.com/allank/murli"
	"github.com/urfave/cli/v2"
)

// EmitSchema writes the agent-optimized JSON schema for a command to w.
func EmitSchema(cmd *cli.Command, w io.Writer) error {
	meta := metadataFor(cmd)

	schema := murli.CommandSchema{
		Name:             cmd.Name,
		Summary:          cmd.Usage,
		WhenToUse:        meta.WhenToUse,
		AgentDescription: meta.AgentDescription,
		Idempotent:       meta.Idempotent,
		Mutating:         meta.Mutating,
		Returns:          meta.Returns,
		Examples:         meta.Examples,
		Arguments:        meta.Arguments,
		Flags:            v2FlagSchemas(cmd.Flags, meta.FlagAnnotations),
		Subcommands:      v2SubcommandSchemas(cmd.Subcommands),
		Safety: murli.SafetyBlock{
			ReadOnly:    !meta.Mutating,
			Idempotent:  meta.Idempotent,
			Destructive: meta.Destructive,
			Reversible:  meta.Reversible,
			DryRunnable: meta.DryRunnable,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(schema)
}

func v2FlagSchemas(flags []cli.Flag, annotations map[string]murli.FlagAnnotation) []murli.FlagSchema {
	skipped := map[string]bool{
		"schema": true, "agent": true, "output": true, "protocol-version": true,
		"force": true, "yes": true, "dry-run": true, "profile": true,
	}
	var list []murli.FlagSchema
	for _, f := range flags {
		names := f.Names()
		if len(names) == 0 {
			continue
		}
		name := names[0]
		if skipped[name] {
			continue
		}
		fs := murli.FlagSchema{
			Name:        name,
			Type:        v2FlagType(f),
			Default:     v2FlagDefault(f),
			Description: v2FlagUsage(f),
		}
		if ann, ok := annotations[name]; ok {
			murli.ApplyFlagAnnotation(&fs, ann)
		}
		list = append(list, fs)
	}
	return list
}

func v2FlagType(f cli.Flag) string {
	switch f.(type) {
	case *cli.BoolFlag:
		return "bool"
	case *cli.IntFlag:
		return "int"
	case *cli.Int64Flag:
		return "int64"
	case *cli.Float64Flag:
		return "float64"
	case *cli.StringFlag:
		return "string"
	case *cli.StringSliceFlag:
		return "[]string"
	case *cli.IntSliceFlag:
		return "[]int"
	default:
		return "string"
	}
}

func v2FlagDefault(f cli.Flag) any {
	switch tf := f.(type) {
	case *cli.BoolFlag:
		return tf.Value
	case *cli.IntFlag:
		return tf.Value
	case *cli.Int64Flag:
		return tf.Value
	case *cli.Float64Flag:
		return tf.Value
	case *cli.StringFlag:
		return tf.Value
	case *cli.StringSliceFlag:
		return tf.Value
	case *cli.IntSliceFlag:
		return tf.Value
	default:
		return ""
	}
}

func v2FlagUsage(f cli.Flag) string {
	switch tf := f.(type) {
	case *cli.BoolFlag:
		return tf.Usage
	case *cli.IntFlag:
		return tf.Usage
	case *cli.Int64Flag:
		return tf.Usage
	case *cli.Float64Flag:
		return tf.Usage
	case *cli.StringFlag:
		return tf.Usage
	case *cli.StringSliceFlag:
		return tf.Usage
	case *cli.IntSliceFlag:
		return tf.Usage
	default:
		return ""
	}
}

// BuildV2DescribeTree recursively builds the full DescribeCommandSchema for a v2 command.
func BuildV2DescribeTree(cmd *cli.Command) murli.DescribeCommandSchema {
	meta := metadataFor(cmd)
	node := murli.DescribeCommandSchema{
		Name:             cmd.Name,
		Summary:          cmd.Usage,
		WhenToUse:        meta.WhenToUse,
		AgentDescription: meta.AgentDescription,
		Idempotent:       meta.Idempotent,
		Mutating:         meta.Mutating,
		Returns:          meta.Returns,
		Examples:         meta.Examples,
		Arguments:        meta.Arguments,
		Flags:            v2FlagSchemas(cmd.Flags, meta.FlagAnnotations),
		Safety: murli.SafetyBlock{
			ReadOnly:    !meta.Mutating,
			Idempotent:  meta.Idempotent,
			Destructive: meta.Destructive,
			Reversible:  meta.Reversible,
			DryRunnable: meta.DryRunnable,
		},
	}
	for _, child := range cmd.Subcommands {
		if child.Hidden || child.Name == "describe" {
			continue
		}
		node.Subcommands = append(node.Subcommands, BuildV2DescribeTree(child))
	}
	return node
}

func v2SubcommandSchemas(cmds []*cli.Command) []murli.SubcommandSchema {
	var list []murli.SubcommandSchema
	for _, c := range cmds {
		if c.Hidden {
			continue
		}
		list = append(list, murli.SubcommandSchema{Name: c.Name, Summary: c.Usage})
	}
	return list
}
