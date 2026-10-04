package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is stamped at link time (-X github.com/iamnikolie/oolog/cmd.version=…).
var version = "dev"

func buildVersion() string {
	v := version
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				v += " (" + s.Value[:12] + ")"
			}
		}
	}
	return v
}

var versionCmd = &cobra.Command{
	Use:         "version",
	Short:       "Show the oolog version",
	Args:        cobra.NoArgs,
	Annotations: map[string]string{"skipLoad": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "oolog version %s\n", buildVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
