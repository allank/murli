//go:build murlidev

package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/allank/murli"
	"github.com/urfave/cli/v3"
)

func isTTYWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, _ := f.Stat()
		return stat != nil && (stat.Mode()&os.ModeCharDevice) != 0
	}
	return false
}

func collectV3Names(cmd *cli.Command, cmds, flags *[]string) {
	*cmds = append(*cmds, cmd.Name)
	for _, f := range cmd.Flags {
		if names := f.Names(); len(names) > 0 {
			*flags = append(*flags, names[0])
		}
	}
	for _, sub := range cmd.Commands {
		collectV3Names(sub, cmds, flags)
	}
}

func mountDevTools(app *cli.Command) {
	if isTTYWriter(writerOrDefault(app.Writer, os.Stdout)) {
		var cmdNames, flagNames []string
		for _, cmd := range app.Commands {
			collectV3Names(cmd, &cmdNames, &flagNames)
		}
		murli.CheckConventions(cmdNames, flagNames, writerOrDefault(app.ErrWriter, os.Stderr))
	}
	for _, c := range app.Commands {
		if c.Name == "doctor" {
			return
		}
	}
	app.Commands = append(app.Commands, buildV3DoctorCmd(app))
}

func buildV3DoctorCmd(app *cli.Command) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Run murli integration self-checks",
		Action: func(ctx context.Context, c *cli.Command) error {
			out := buildV3AppDescribeOutput(app)
			report := murli.RunDoctor(out)

			stdout := writerOrDefault(app.Writer, os.Stdout)
			stderr := writerOrDefault(app.ErrWriter, os.Stderr)
			agentMode := c.Root().Bool("agent")
			w := murli.NewWriter(stdout, stderr, agentMode)

			if w.IsTTY() {
				murli.WriteDoctorTTY(stdout, report)
				return nil
			}
			summary := "all checks passed"
			if report.Failed > 0 {
				summary = fmt.Sprintf("%d check(s) failed", report.Failed)
				w.WritePlan(summary, report)
			} else if report.Warnings > 0 {
				summary = fmt.Sprintf("%d warning(s)", report.Warnings)
				w.WriteSuccess(summary, report)
			} else {
				w.WriteSuccess(summary, report)
			}
			return nil
		},
	}
}
