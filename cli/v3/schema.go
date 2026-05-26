package cli

import (
	"encoding/json"
	"io"

	"github.com/allank/murli"
	"github.com/urfave/cli/v3"
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
		Flags:            v3FlagSchemas(cmd.Flags, meta.FlagAnnotations),
		Subcommands:      v3SubcommandSchemas(cmd.Commands),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(schema)
}

func v3FlagSchemas(flags []cli.Flag, annotations map[string]murli.FlagAnnotation) []murli.FlagSchema {
	skipped := map[string]bool{"schema": true, "agent": true, "output": true, "protocol-version": true}
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
			Type:        v3FlagType(f),
			Default:     v3FlagDefault(f),
			Description: v3FlagUsage(f),
		}
		if ann, ok := annotations[name]; ok {
			murli.ApplyFlagAnnotation(&fs, ann)
		}
		list = append(list, fs)
	}
	return list
}

func v3FlagType(f cli.Flag) string {
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

func v3FlagDefault(f cli.Flag) any {
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

func v3FlagUsage(f cli.Flag) string {
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

// BuildV3DescribeTree recursively builds the full DescribeCommandSchema for a v3 command.
func BuildV3DescribeTree(cmd *cli.Command) murli.DescribeCommandSchema {
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
		Flags:            v3FlagSchemas(cmd.Flags, meta.FlagAnnotations),
	}
	for _, child := range cmd.Commands {
		if child.Hidden || child.Name == "describe" {
			continue
		}
		node.Subcommands = append(node.Subcommands, BuildV3DescribeTree(child))
	}
	return node
}

func v3SubcommandSchemas(cmds []*cli.Command) []murli.SubcommandSchema {
	var list []murli.SubcommandSchema
	for _, c := range cmds {
		if c.Hidden {
			continue
		}
		list = append(list, murli.SubcommandSchema{Name: c.Name, Summary: c.Usage})
	}
	return list
}
