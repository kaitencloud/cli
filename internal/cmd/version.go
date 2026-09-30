package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "Print Kaiten CLI version information",
		Annotations: map[string]string{skipOutputValidationAnnotation: "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			version := Version
			commit := Commit
			date := Date

			if version == "dev" {
				if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
					version = info.Main.Version
				}
			}

			return writeMessage(cmd, fmt.Sprintf("Kaiten CLI %s (%s) built at %s", version, commit, date))
		},
	}
}
