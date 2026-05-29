//go:build murlidev

package cobra

import (
	"fmt"
	"io"
	"os"

	"github.com/allank/murli"
	gocobra "github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func isTTYWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, _ := f.Stat()
		return stat != nil && (stat.Mode()&os.ModeCharDevice) != 0
	}
	return false
}

func collectNames(cmd *gocobra.Command, cmds, flags *[]string) {
	*cmds = append(*cmds, cmd.Name())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		*flags = append(*flags, f.Name)
	})
	for _, child := range cmd.Commands() {
		collectNames(child, cmds, flags)
	}
}

func mountDevTools(rootCmd *gocobra.Command) {
	if isTTYWriter(rootCmd.OutOrStdout()) {
		var cmdNames, flagNames []string
		collectNames(rootCmd, &cmdNames, &flagNames)
		murli.CheckConventions(cmdNames, flagNames, rootCmd.ErrOrStderr())
	}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "doctor" {
			return
		}
	}
	rootCmd.AddCommand(buildCobraDoctorCmd(rootCmd))
}

func buildCobraDoctorCmd(rootCmd *gocobra.Command) *gocobra.Command {
	return &gocobra.Command{
		Use:   "doctor",
		Short: "Run murli integration self-checks",
		RunE: func(cmd *gocobra.Command, args []string) error {
			out := buildCobraDescribeOutput(rootCmd)
			report := murli.RunDoctor(out)

			w := NewWriter(cmd)
			if w.IsTTY() {
				murli.WriteDoctorTTY(cmd.OutOrStdout(), report)
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
